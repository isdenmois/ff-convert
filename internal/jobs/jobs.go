// Package jobs manages the conversion queue shared by the web and desktop
// frontends: jobs are created from a path, probed asynchronously, enriched
// with user answers, and executed through the shared worker pool.
package jobs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/isden/ff-convert/internal/config"
	"github.com/isden/ff-convert/internal/convert"
	"github.com/isden/ff-convert/internal/plan"
	"github.com/isden/ff-convert/internal/pool"
	"github.com/isden/ff-convert/internal/probe"
	"github.com/isden/ff-convert/internal/scan"
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
	// DefaultPreset is the preselected per-task video codec (config
	// default_preset, "copy" when unset).
	DefaultPreset string       `json:"default_preset"`
	Defaults      plan.Answers `json:"defaults"`
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
	ID        string         `json:"id"`
	Path      string         `json:"path"`
	Name      string         `json:"name"`
	State     State          `json:"state"`
	CreatedAt time.Time      `json:"created_at"`
	Questions *QuestionsView `json:"questions,omitempty"`
	Files     []FileView     `json:"files,omitempty"`
	Skipped   []SkippedView  `json:"skipped,omitempty"`
	Overall   float64        `json:"overall"`
	Done      int            `json:"done"`
	Failed    int            `json:"failed"`
	Total     int            `json:"total"`
	Error     string         `json:"error,omitempty"`
}

type job struct {
	id        string
	path      string
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

	mu        sync.Mutex
	jobs      []*job
	nextID    int
	listeners map[int]func()
	listenID  int
}

// NewManager creates a manager. The pool is shared: all jobs' tasks compete
// for its worker slots.
func NewManager(cfg *config.Config, p *pool.Pool) *Manager {
	if cfg == nil {
		cfg = &config.Config{}
	}
	if p == nil {
		p = pool.NewDefault()
	}
	return &Manager{cfg: cfg, pool: p, listeners: map[int]func(){}}
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
// probing asynchronously.
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
	p, err := convert.Probe(j.ctx, j.path)
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
	qv := &QuestionsView{
		PresetChoices:  m.cfg.PresetNames(),
		AskPreset:      m.cfg.DefaultPreset == "",
		AskNormalize:   m.cfg.Audio == nil || m.cfg.Audio.Normalize == nil,
		AskAttachments: m.cfg.Attachments == nil,
		DefaultPreset:  m.cfg.DefaultPresetName(),
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
// conversion. answers is index-aligned with the questions' groups.
func (m *Manager) SubmitAnswers(id string, answers []plan.Answers) error {
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
	sess, err := j.plan.BuildSession(m.cfg, answers)
	if err != nil {
		m.mu.Unlock()
		return err
	}
	j.answers = answers
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

	_ = j.sess.Run(j.ctx, m.pool, convert.ReporterFuncs{
		Start: func(*plan.Task) {
			m.mu.Lock()
			if j.state == StateRunning {
				m.mu.Unlock()
				return
			}
			j.state = StateRunning
			m.mu.Unlock()
			m.notify()
		},
	})

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
	v := JobView{
		ID:        j.id,
		Path:      j.path,
		Name:      filepath.Base(j.path),
		State:     j.state,
		CreatedAt: j.created,
		Error:     j.err,
		Questions: j.questions,
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

// SortedVideoExts exposes the scanned extensions (informational).
func SortedVideoExts() []string {
	out := make([]string, 0, len(scan.VideoExts))
	for e := range scan.VideoExts {
		out = append(out, e)
	}
	sort.Strings(out)
	return out
}
