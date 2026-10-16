// Package jobs manages the conversion queue shared by the web and desktop
// frontends: jobs are created from a path, probed asynchronously, enriched
// with user answers, and executed through the shared worker pool.
package jobs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/isden/ff-convert/internal/config"
	"github.com/isden/ff-convert/internal/convert"
	"github.com/isden/ff-convert/internal/plan"
	"github.com/isden/ff-convert/internal/pool"
	"github.com/isden/ff-convert/internal/probe"
)

// State is the lifecycle state of a job.
type State string

const (
	StateProbing   State = "probing"
	StateQuestions State = "questions"
	StateQueued    State = "queued"
	StateRunning   State = "running"
	StateDone      State = "done"
	StateFailed    State = "failed"
	StateCanceled  State = "canceled"
)

// Terminal reports whether the state is final.
func (s State) Terminal() bool {
	switch s {
	case StateDone, StateFailed, StateCanceled:
		return true
	}
	return false
}

// StreamView is a UI-friendly track description.
type StreamView struct {
	Index    int    `json:"index"`
	Label    string `json:"label"`
	Language string `json:"language"`
	Title    string `json:"title,omitempty"`
	Codec    string `json:"codec,omitempty"`
	Channels int    `json:"channels,omitempty"`
	Layout   string `json:"layout,omitempty"`
}

func streamViews(ss []probe.Stream) []StreamView {
	out := make([]StreamView, 0, len(ss))
	for _, s := range ss {
		out = append(out, StreamView{
			Index: s.Index, Label: s.Label(), Language: s.Language,
			Title: s.Title, Codec: s.Codec, Channels: s.Channels, Layout: s.Layout,
		})
	}
	return out
}

// TaskView describes one convertible file (task) inside a group.
type TaskView struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

// GroupView describes one question group.
type GroupView struct {
	Key           string       `json:"key"`
	FileCount     int          `json:"file_count"`
	Files         []string     `json:"files"`
	Tasks         []TaskView   `json:"tasks"`
	Audio         []StreamView `json:"audio"`
	Subs          []StreamView `json:"subs"`
	AudioDefaults []int        `json:"audio_defaults"`
	SubsDefaults  []int        `json:"subs_defaults"`
}

// QuestionsView is everything the UI needs to ask about a job.
type QuestionsView struct {
	Groups         []GroupView `json:"groups"`
	PresetChoices  []string    `json:"preset_choices"`
	AskPreset      bool        `json:"ask_preset"`
	AskNormalize   bool        `json:"ask_normalize"`
	AskAttachments bool        `json:"ask_attachments"`
	// DefaultPreset is the preselected video codec for the whole queue item
	// (config default_preset, "copy" when unset).
	DefaultPreset string `json:"default_preset"`
	// Categories are the selectable output subfolders. They are a web-only
	// feature: the web API injects them into the snapshots it sends; the
	// shared manager leaves the field nil so the desktop never sees them.
	Categories []string     `json:"categories,omitempty"`
	Defaults   plan.Answers `json:"defaults"`
	// SourceFolder is the subfolder a folder source contributes to the
	// output root ("" for single-file sources); the answers modal composes
	// its output preview with it.
	SourceFolder string `json:"source_folder,omitempty"`
}

// FileView is a UI-friendly file progress record.
type FileView struct {
	Path    string  `json:"path"`
	Name    string  `json:"name"`
	Output  string  `json:"output"`
	State   string  `json:"state"`
	Percent float64 `json:"percent"`
	Speed   string  `json:"speed,omitempty"`
	Error   string  `json:"error,omitempty"`
}

// SkippedView describes a skipped file.
type SkippedView struct {
	Path   string `json:"path"`
	Name   string `json:"name"`
	Output string `json:"output"`
}

// JobView is the complete UI snapshot of a job.
type JobView struct {
	ID   string `json:"id"`
	Path string `json:"path"`
	Name string `json:"name"`
	// OutputDir is the output folder pinned when the job was created; later
	// changes to the Manager's output dir do not affect it.
	OutputDir string `json:"output_dir,omitempty"`
	// OutputFolder is the folder this job's converted files are written to:
	// OutputDir plus the category chosen with the answers and, for folder
	// sources, the source's own folder name. Equal to OutputDir while the
	// source has not been scanned yet.
	OutputFolder string         `json:"output_folder,omitempty"`
	State        State          `json:"state"`
	CreatedAt    time.Time      `json:"created_at"`
	Questions    *QuestionsView `json:"questions,omitempty"`
	Files        []FileView     `json:"files,omitempty"`
	Skipped      []SkippedView  `json:"skipped,omitempty"`
	Overall      float64        `json:"overall"`
	Done         int            `json:"done"`
	Failed       int            `json:"failed"`
	Total        int            `json:"total"`
	// ElapsedMs is the conversion wall time so far — the total run duration
	// once the job finishes; omitted until the first file starts converting.
	ElapsedMs int64 `json:"elapsed_ms,omitempty"`
	// EtaMs is the estimated remaining conversion time; omitted when no
	// sensible estimate exists (see convert.Snapshot.ETA).
	EtaMs int64  `json:"eta_ms,omitempty"`
	Error string `json:"error,omitempty"`
}

type job struct {
	id   string
	path string
	// outDir is the output folder pinned at creation time (see JobView).
	outDir string
	// category is the output subfolder chosen with the answers ("" = the
	// output root); jobs created without a category keep the root.
	category  string
	created   time.Time
	state     State
	err       string
	plan      *convert.Plan
	questions *QuestionsView
	answers   []plan.Answers
	sess      *convert.Session
	ctx       context.Context
	cancel    context.CancelFunc
	canceled  bool
}

// Manager owns the job list and drives job execution.
type Manager struct {
	cfg  *config.Config
	pool *pool.Pool
	// outDir is the current output folder; jobs pin it at creation.
	outDir string

	mu        sync.Mutex
	jobs      []*job
	nextID    int
	listeners map[int]func()
	listenID  int
}

// NewManager creates a manager. The pool is shared: all jobs' tasks compete
// for its worker slots. outDir is the initial output folder (FF_CONVERT_
// OUTPUT_DIR / config output_dir / frontend fallback, already resolved by
// the caller).
func NewManager(cfg *config.Config, p *pool.Pool, outDir string) *Manager {
	if cfg == nil {
		cfg = &config.Config{}
	}
	if p == nil {
		p = pool.NewDefault()
	}
	return &Manager{cfg: cfg, pool: p, outDir: outDir, listeners: map[int]func(){}}
}

// OutputDir returns the current output folder applied to newly created jobs.
func (m *Manager) OutputDir() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.outDir
}

// SetOutputDir changes the output folder for jobs created from now on;
// existing jobs keep the folder pinned at their creation.
func (m *Manager) SetOutputDir(dir string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.outDir = dir
}

// Subscribe registers a change notification callback. The returned function
// unsubscribes. Callbacks must not call back into the Manager.
func (m *Manager) Subscribe(fn func()) (cancel func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listenID++
	id := m.listenID
	m.listeners[id] = fn
	return func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		delete(m.listeners, id)
	}
}

func (m *Manager) notify() {
	m.mu.Lock()
	fns := make([]func(), 0, len(m.listeners))
	for _, fn := range m.listeners {
		fns = append(fns, fn)
	}
	m.mu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

// Create registers a new job for the given file/folder path and starts
// probing asynchronously. The job pins the manager's current output folder.
func (m *Manager) Create(path string) (JobView, error) {
	if _, err := os.Stat(path); err != nil {
		return JobView{}, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return JobView{}, err
	}

	m.mu.Lock()
	m.nextID++
	j := &job{
		id:      strconv.Itoa(m.nextID),
		path:    abs,
		outDir:  m.outDir,
		created: time.Now(),
		state:   StateProbing,
	}
	j.ctx, j.cancel = context.WithCancel(context.Background())
	m.jobs = append(m.jobs, j)
	m.mu.Unlock()
	go m.probeJob(j)

	m.notify()
	return m.view(j), nil
}

func (m *Manager) probeJob(j *job) {
	p, err := convert.Probe(j.ctx, j.path, j.outDir)
	m.mu.Lock()
	if j.state != StateProbing { // canceled meanwhile
		m.mu.Unlock()
		return
	}
	if err != nil {
		j.state = StateFailed
		j.err = err.Error()
		m.mu.Unlock()
		m.notify()
		return
	}
	j.plan = p
	if len(p.Groups) == 0 {
		j.state = StateDone // everything skipped
		m.mu.Unlock()
		m.notify()
		return
	}
	inputs := p.AskInputs(m.cfg)

	askAttachments := false
	if len(inputs) > 0 {
		askAttachments = inputs[0].AskAttachments
	}

	qv := &QuestionsView{
		PresetChoices:  m.cfg.PresetNames(),
		AskPreset:      m.cfg.DefaultPreset == "",
		AskNormalize:   m.cfg.Audio == nil || m.cfg.Audio.Normalize == nil,
		AskAttachments: askAttachments,
		DefaultPreset:  m.cfg.DefaultPresetName(),
		SourceFolder:   p.SourceFolder,
	}
	for i, in := range inputs {
		g := GroupView{
			Key:           fmt.Sprintf("g%d", i+1),
			FileCount:     len(in.Group.Files),
			Audio:         streamViews(in.Group.Audio),
			Subs:          streamViews(in.Group.Subtitles),
			AudioDefaults: in.AudioDefaults,
			SubsDefaults:  in.SubsDefaults,
		}
		for _, f := range in.Group.Files {
			name := filepath.Base(f.Path)
			g.Files = append(g.Files, name)
			g.Tasks = append(g.Tasks, TaskView{Path: f.Path, Name: name})
		}
		qv.Groups = append(qv.Groups, g)
	}
	qv.Defaults = inputs[0].Defaults
	j.questions = qv
	j.state = StateQuestions
	m.mu.Unlock()
	m.notify()
}

// SubmitAnswers stores the per-group answers and enqueues the job for
// conversion. answers is index-aligned with the questions' groups. A
// non-empty category picks a configured output subfolder; "" keeps the
// output root.
func (m *Manager) SubmitAnswers(id string, category string, answers []plan.Answers) error {
	m.mu.Lock()
	j := m.byID(id)
	if j == nil {
		m.mu.Unlock()
		return fmt.Errorf("job %s not found", id)
	}
	if j.state != StateQuestions {
		m.mu.Unlock()
		return fmt.Errorf("job %s is in state %s, not accepting answers", id, j.state)
	}
	if len(answers) != len(j.questions.Groups) {
		m.mu.Unlock()
		return fmt.Errorf("%d answer groups for %d question groups", len(answers), len(j.questions.Groups))
	}
	sess, err := j.plan.BuildSession(m.cfg, category, answers, j.outDir)
	if err != nil {
		m.mu.Unlock()
		return err
	}
	j.answers = answers
	j.category = category
	j.sess = sess
	j.state = StateQueued
	m.mu.Unlock()
	go m.runJob(j)

	m.notify()
	return nil
}

func (m *Manager) runJob(j *job) {
	if err := j.sess.EnsureOutputDir(); err != nil {
		m.mu.Lock()
		j.state = StateFailed
		j.err = err.Error()
		m.mu.Unlock()
		m.notify()
		return
	}
	m.mu.Lock()
	j.state = StateRunning
	m.mu.Unlock()
	m.notify()

	_ = j.sess.Run(j.ctx, m.pool, convert.ReporterFuncs{})

	m.mu.Lock()
	switch {
	case j.canceled:
		j.state = StateCanceled
	default:
		sn := j.sess.Snapshot()
		j.state = StateDone
		if sn.Failed > 0 {
			j.state = StateFailed
			j.err = fmt.Sprintf("%d of %d file(s) failed", sn.Failed, sn.Total)
		}
	}
	m.mu.Unlock()
	m.notify()
}

// Cancel cancels a running or queued job (or one awaiting answers).
func (m *Manager) Cancel(id string) error {
	m.mu.Lock()
	j := m.byID(id)
	if j == nil {
		m.mu.Unlock()
		return fmt.Errorf("job %s not found", id)
	}
	if j.state.Terminal() {
		m.mu.Unlock()
		return fmt.Errorf("job %s already finished", id)
	}
	j.canceled = true
	j.cancel()
	if j.state == StateProbing || j.state == StateQuestions || j.state == StateQueued {
		j.state = StateCanceled
	}
	m.mu.Unlock()
	m.notify()
	return nil
}

// Remove deletes a job from the list, canceling it first if needed.
func (m *Manager) Remove(id string) error {
	if err := func() error {
		m.mu.Lock()
		j := m.byID(id)
		if j == nil {
			m.mu.Unlock()
			return fmt.Errorf("job %s not found", id)
		}
		if !j.state.Terminal() {
			j.canceled = true
			j.cancel()
		}
		for i, x := range m.jobs {
			if x == j {
				m.jobs = append(m.jobs[:i], m.jobs[i+1:]...)
				break
			}
		}
		m.mu.Unlock()
		return nil
	}(); err != nil {
		return err
	}
	m.notify()
	return nil
}

// View returns the snapshot of one job.
func (m *Manager) View(id string) (JobView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j := m.byID(id)
	if j == nil {
		return JobView{}, fmt.Errorf("job %s not found", id)
	}
	return m.view(j), nil
}

// Views returns snapshots of all jobs in creation order.
func (m *Manager) Views() []JobView {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]JobView, 0, len(m.jobs))
	for _, j := range m.jobs {
		out = append(out, m.view(j))
	}
	return out
}

// byID requires m.mu held.
func (m *Manager) byID(id string) *job {
	for _, j := range m.jobs {
		if j.id == id {
			return j
		}
	}
	return nil
}

// view requires m.mu held.
func (m *Manager) view(j *job) JobView {
	sourceFolder := ""
	if j.plan != nil {
		sourceFolder = j.plan.SourceFolder
	}
	v := JobView{
		ID:           j.id,
		Path:         j.path,
		Name:         filepath.Base(j.path),
		OutputDir:    j.outDir,
		OutputFolder: convert.OutputRoot(j.outDir, j.category, sourceFolder),
		State:        j.state,
		CreatedAt:    j.created,
		Error:        j.err,
		Questions:    j.questions,
	}
	if j.plan != nil {
		for _, s := range j.plan.Skipped {
			v.Skipped = append(v.Skipped, SkippedView{Path: s.Path, Name: filepath.Base(s.Path), Output: s.Output})
		}
	}
	if j.sess != nil {
		sn := j.sess.Snapshot()
		v.Overall = sn.Overall
		v.Done, v.Failed, v.Total = sn.Done, sn.Failed, sn.Total
		v.ElapsedMs = sn.Elapsed().Milliseconds()
		v.EtaMs = sn.ETA().Milliseconds()
		for _, f := range sn.Files {
			v.Files = append(v.Files, FileView{
				Path: f.Path, Name: filepath.Base(f.Path), Output: f.Output,
				State: string(f.State), Percent: f.Percent, Speed: f.Speed, Error: f.Error,
			})
		}
	}
	return v
}

// Config exposes the active configuration (for UI choices).
func (m *Manager) Config() *config.Config { return m.cfg }
