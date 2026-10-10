// Package convert orchestrates the full pipeline: scan → probe → group →
// ask (via callback) → plan → run with progress reporting.
package convert

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/isden/ff-convert/internal/config"
	"github.com/isden/ff-convert/internal/ffmpegx"
	"github.com/isden/ff-convert/internal/group"
	"github.com/isden/ff-convert/internal/plan"
	"github.com/isden/ff-convert/internal/pool"
	"github.com/isden/ff-convert/internal/probe"
	"github.com/isden/ff-convert/internal/scan"
)

// AskInput carries everything a UI needs to ask about one group.
type AskInput struct {
	Group *group.Group
	// AudioDefaults and SubsDefaults are the pre-selected stream indexes
	// (English, with the no-English fallback applied).
	AudioDefaults []int
	SubsDefaults  []int
	// PresetChoices are the selectable preset names.
	PresetChoices []string
	// AskPreset/AskNormalize/AskAttachments are false when the config
	// pre-answers the question (the answer is then taken from Defaults).
	AskPreset      bool
	AskNormalize   bool
	AskAttachments bool
	// Defaults holds config-provided answers (Preset name, Normalize,
	// Attachments) for the fields that are not asked.
	Defaults plan.Answers
}

// AskFunc asks the user about one group and returns the answers. Returning an
// error aborts the conversion.
type AskFunc func(AskInput) (plan.Answers, error)

// FileTaskState is the lifecycle state of one file in a run.
type FileTaskState string

const (
	StatePending   FileTaskState = "pending"
	StateRunning   FileTaskState = "running"
	StateDone      FileTaskState = "done"
	StateFailed    FileTaskState = "failed"
	StateCanceled  FileTaskState = "canceled"
	StateSkipped   FileTaskState = "skipped"
)

// FileTask tracks one file's progress within a run.
type FileTask struct {
	Path     string        `json:"path"`
	Output   string        `json:"output"`
	State    FileTaskState `json:"state"`
	Percent  float64       `json:"percent"`
	Speed    string        `json:"speed"`
	OutTime  time.Duration `json:"-"`
	Duration time.Duration `json:"-"`
	Error    string        `json:"error,omitempty"`
}

// Reporter receives progress updates for a run. All callbacks are invoked
// from goroutines; implementations must be safe for concurrent use.
type Reporter interface {
	TaskStart(task *plan.Task)
	TaskProgress(task *plan.Task, stats ffmpegx.Stats)
	TaskDone(task *plan.Task, err error)
}

// ReporterFuncs adapts functions to Reporter.
type ReporterFuncs struct {
	Start     func(task *plan.Task)
	Progress  func(task *plan.Task, stats ffmpegx.Stats)
	Done      func(task *plan.Task, err error)
}

func (r ReporterFuncs) TaskStart(t *plan.Task) {
	if r.Start != nil {
		r.Start(t)
	}
}

func (r ReporterFuncs) TaskProgress(t *plan.Task, s ffmpegx.Stats) {
	if r.Progress != nil {
		r.Progress(t, s)
	}
}

func (r ReporterFuncs) TaskDone(t *plan.Task, err error) {
	if r.Done != nil {
		r.Done(t, err)
	}
}

// Session is a prepared conversion run. It is safe for concurrent use:
// Run mutates file states internally while Snapshot can be read from a UI.
type Session struct {
	Source  string
	Tasks   []*plan.Task
	Skipped []scan.Skipped

	mu    sync.Mutex
	files []*FileTask
}

// Snapshot returns a point-in-time copy of the file task states plus overall
// progress figures.
type Snapshot struct {
	Files   []FileTask
	Overall float64
	Done    int
	Failed  int
	Total   int
}

// Files returns a copy of the current file task states.
func (s *Session) Files() []FileTask {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]FileTask, 0, len(s.files))
	for _, f := range s.files {
		out = append(out, *f)
	}
	return out
}

// Overall computes overall progress as sum(out_time)/sum(duration), counting
// finished files as fully processed.
func (s *Session) overall() float64 {
	var out, total time.Duration
	for _, f := range s.files {
		total += f.Duration
		if f.State == StateDone {
			out += f.Duration
		} else {
			out += f.OutTime
		}
	}
	if total == 0 {
		return 0
	}
	p := float64(out) / float64(total)
	if p > 1 {
		p = 1
	}
	return p
}

// Snapshot returns a copy of all file states with overall figures.
func (s *Session) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	sn := Snapshot{Overall: s.overall()}
	for _, f := range s.files {
		sn.Files = append(sn.Files, *f)
		switch f.State {
		case StateDone:
			sn.Done++
		case StateFailed:
			sn.Failed++
		}
	}
	sn.Total = len(s.files)
	return sn
}

// OverallCount returns (done, failed, total) file counts.
func (s *Session) OverallCount() (done, failed, total int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range s.files {
		if f.State == StateDone {
			done++
		}
		if f.State == StateFailed {
			failed++
		}
	}
	return done, failed, len(s.files)
}

// Plan is the result of scanning and probing a source: groups of files with
// identical audio/subtitle layouts, ready to be asked about.
type Plan struct {
	Source  string
	Skipped []scan.Skipped
	Groups  []*group.Group
}

// Probe scans and probes the source and groups the files.
func Probe(ctx context.Context, source string) (*Plan, error) {
	scanRes, err := scan.Scan(source)
	if err != nil {
		return nil, err
	}

	// Probe all files in parallel.
	medias := make([]*probe.Media, len(scanRes.Files))
	var wg sync.WaitGroup
	sem := make(chan struct{}, pool.DefaultLimit())
	for i, f := range scanRes.Files {
		wg.Add(1)
		go func(i int, f string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			m, err := probe.File(ctx, f)
			if err == nil {
				medias[i] = m
			}
		}(i, f)
	}
	wg.Wait()

	probed := make([]*probe.Media, 0, len(medias))
	var failed []string
	for i := range medias {
		if medias[i] == nil {
			failed = append(failed, scanRes.Files[i])
			continue
		}
		probed = append(probed, medias[i])
	}
	if len(probed) == 0 && len(scanRes.Files) > 0 {
		return nil, fmt.Errorf("ffprobe failed for all %d files (first: %s)", len(failed), failed[0])
	}
	return &Plan{Source: source, Skipped: scanRes.Skipped, Groups: group.Files(probed)}, nil
}

// AskInputs builds the question list for the plan's groups, applying config
// defaults (questions already answered by the config have their Ask* flag
// cleared).
func (p *Plan) AskInputs(cfg *config.Config) []AskInput {
	if cfg == nil {
		cfg = &config.Config{}
	}
	inputs := make([]AskInput, 0, len(p.Groups))
	for _, g := range p.Groups {
		input := AskInput{
			Group:          g,
			AudioDefaults:  group.DefaultAudio(g),
			SubsDefaults:   group.DefaultSubs(g),
			PresetChoices:  cfg.PresetNames(),
			AskPreset:      cfg.DefaultPreset == "",
			AskNormalize:   cfg.Audio == nil || cfg.Audio.Normalize == nil,
			AskAttachments: cfg.Attachments == nil,
		}
		input.Defaults = plan.Answers{
			Preset:      cfg.DefaultPreset,
			Normalize:   cfg.Audio != nil && cfg.Audio.Normalize != nil && *cfg.Audio.Normalize,
			Attachments: cfg.Attachments == nil || *cfg.Attachments == "copy",
		}
		inputs = append(inputs, input)
	}
	return inputs
}

// BuildSession assembles the runnable session from per-group answers. The
// answers slice is index-aligned with the AskInputs result.
func (p *Plan) BuildSession(cfg *config.Config, answers []plan.Answers) (*Session, error) {
	if cfg == nil {
		cfg = &config.Config{}
	}
	if len(answers) != len(p.Groups) {
		return nil, fmt.Errorf("%d answers for %d groups", len(answers), len(p.Groups))
	}
	presets := plan.PresetsMap(cfg)
	sess := &Session{Source: p.Source, Skipped: p.Skipped}
	for gi, g := range p.Groups {
		ans := answers[gi]
		for _, m := range g.Files {
			out := scan.OutputPath(m.Path)
			task, err := plan.Build(m, ans, presets, cfg.Bitrate(), out)
			if err != nil {
				return nil, err
			}
			sess.Tasks = append(sess.Tasks, task)
			sess.files = append(sess.files, &FileTask{
				Path:     m.Path,
				Output:   out,
				State:    StatePending,
				Duration: m.DurationOf(),
			})
		}
	}
	return sess, nil
}

// Prepare is the one-shot convenience for frontends that can ask
// synchronously: probe, ask about every group, build the session.
func Prepare(ctx context.Context, source string, cfg *config.Config, ask AskFunc) (*Session, error) {
	p, err := Probe(ctx, source)
	if err != nil {
		return nil, err
	}
	inputs := p.AskInputs(cfg)
	answers := make([]plan.Answers, len(inputs))
	for i, in := range inputs {
		ans, err := resolveAnswers(in, ask)
		if err != nil {
			return nil, err
		}
		answers[i] = ans
	}
	return p.BuildSession(cfg, answers)
}

// resolveAnswers merges config defaults with the interactive ask: only the
// questions flagged Ask* are posed; everything else comes from Defaults.
func resolveAnswers(in AskInput, ask AskFunc) (plan.Answers, error) {
	ans := plan.Answers{
		Audio:       append([]int(nil), in.AudioDefaults...),
		Subs:        append([]int(nil), in.SubsDefaults...),
		Preset:      in.Defaults.Preset,
		Normalize:   in.Defaults.Normalize,
		Attachments: in.Defaults.Attachments,
	}
	if !in.AskPreset && !in.AskNormalize && !in.AskAttachments {
		return ans, nil // fully determined by config
	}
	if ask == nil {
		return plan.Answers{}, fmt.Errorf("interactive answers required but no ask callback provided")
	}
	user, err := ask(in)
	if err != nil {
		return plan.Answers{}, err
	}
	if user.Audio != nil {
		ans.Audio = user.Audio
	}
	if user.Subs != nil {
		ans.Subs = user.Subs
	}
	if in.AskPreset && user.Preset != "" {
		ans.Preset = user.Preset
	}
	if in.AskNormalize {
		ans.Normalize = user.Normalize
	}
	if in.AskAttachments {
		ans.Attachments = user.Attachments
	}
	return ans, nil
}

// Run executes all session tasks through the pool. It returns when every
// task has finished (or the context is canceled). The reporter is optional.
func (s *Session) Run(ctx context.Context, p *pool.Pool, r Reporter) error {
	if p == nil {
		p = pool.NewDefault()
	}

	var wg sync.WaitGroup
	for _, task := range s.Tasks {
		wg.Add(1)
		t := task
		p.Go(func() {
			defer wg.Done()
			if ctx.Err() != nil {
				s.update(t, func(f *FileTask) { f.State = StateCanceled })
				return
			}
			s.update(t, func(f *FileTask) { f.State = StateRunning })
			if r != nil {
				r.TaskStart(t)
			}
			err := ffmpegx.Run(ctx, ffmpegx.Options{
				Args:     t.Args,
				Duration: t.Duration,
				OnProgress: func(st ffmpegx.Stats) {
					s.update(t, func(f *FileTask) {
						f.Percent = st.Percent
						f.Speed = st.Speed
						f.OutTime = st.OutTime
					})
					if r != nil {
						r.TaskProgress(t, st)
					}
				},
			})
			s.update(t, func(f *FileTask) {
				if err != nil {
					f.State = StateFailed
					f.Error = err.Error()
					os.Remove(t.Tmp)
				} else if err := os.Rename(t.Tmp, t.Output); err != nil {
					f.State = StateFailed
					f.Error = err.Error()
				} else {
					f.State = StateDone
					f.Percent = 1
				}
			})
			if r != nil {
				r.TaskDone(t, err)
			}
		})
	}
	wg.Wait()

	for _, f := range s.Files() {
		if f.State == StateFailed {
			return fmt.Errorf("%s", f.Error)
		}
	}
	return nil
}

// update mutates the FileTask belonging to a plan.Task under the session lock.
func (s *Session) update(t *plan.Task, fn func(*FileTask)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f := s.taskFile(t); f != nil {
		fn(f)
	}
}

// taskFile must be called with s.mu held.
func (s *Session) taskFile(t *plan.Task) *FileTask {
	for _, f := range s.files {
		if f.Path == t.Input {
			return f
		}
	}
	return nil
}

// FileTaskFor returns a copy of the file state for a task input path.
func (s *Session) FileTaskFor(path string) (FileTask, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range s.files {
		if f.Path == path {
			return *f, true
		}
	}
	return FileTask{}, false
}

// EnsureOutputDir creates the converted/ directory for the session's tasks.
func (s *Session) EnsureOutputDir() error {
	seen := make(map[string]bool)
	for _, t := range s.Tasks {
		dir := filepath.Dir(t.Output)
		if seen[dir] {
			continue
		}
		seen[dir] = true
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}
