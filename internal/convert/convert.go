// Package convert orchestrates the full pipeline: scan → probe → group →
// ask (via callback) → plan → run with progress reporting.
package convert

import (
	"context"
	"errors"
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
	// Each refers to a queue-item-wide question (asked once per queue item,
	// not per group): the video codec, loudness normalization and attachment
	// handling.
	AskPreset      bool
	AskNormalize   bool
	AskAttachments bool
	// Defaults holds config-provided answers (Preset, Normalize,
	// Attachments) for the fields that are not asked.
	Defaults plan.Answers
}

// AskFunc asks the user about one group and returns the answers. Returning an
// error aborts the conversion.
type AskFunc func(AskInput) (plan.Answers, error)

// FileTaskState is the lifecycle state of one file in a run.
type FileTaskState string

const (
	StatePending  FileTaskState = "pending"
	StateRunning  FileTaskState = "running"
	StateDone     FileTaskState = "done"
	StateFailed   FileTaskState = "failed"
	StateCanceled FileTaskState = "canceled"
	StateSkipped  FileTaskState = "skipped"
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
	Start    func(task *plan.Task)
	Progress func(task *plan.Task, stats ffmpegx.Stats)
	Done     func(task *plan.Task, err error)
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

	mu          sync.Mutex
	files       []*FileTask
	filesByPath map[string]*FileTask
	started     time.Time // when the first file started converting
	finished    time.Time // when every task reached a terminal state
}

// Snapshot returns a point-in-time copy of the file task states plus overall
// progress figures.
type Snapshot struct {
	Files   []FileTask
	Overall float64
	Done    int
	Failed  int
	Total   int
	// Started is the wall-clock time the first file started converting; zero
	// until then (the shared pool may hold tasks back after Run is called).
	Started time.Time
	// Finished is the wall-clock time every task reached a terminal state;
	// zero until the run completes.
	Finished time.Time
}

// Elapsed returns the wall time since the first file started converting, or
// the total run duration once finished. Zero before the run starts.
func (sn Snapshot) Elapsed() time.Duration {
	if sn.Started.IsZero() {
		return 0
	}
	if !sn.Finished.IsZero() {
		return sn.Finished.Sub(sn.Started)
	}
	return time.Since(sn.Started)
}

// ETA estimates the remaining time from overall progress as
// elapsed·(1−p)/p. It returns 0 when no sensible estimate exists: before the
// run starts, below 1% done (the estimate is wildly unstable there), or once
// the run has finished.
func (sn Snapshot) ETA() time.Duration {
	if sn.Started.IsZero() || !sn.Finished.IsZero() || sn.Overall < 0.01 || sn.Overall >= 1 {
		return 0
	}
	return time.Duration(float64(sn.Elapsed()) * (1 - sn.Overall) / sn.Overall)
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
	sn := Snapshot{Overall: s.overall(), Started: s.started, Finished: s.finished}
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

// Plan is the result of scanning and probing a source: groups of files with
// identical audio/subtitle layouts, ready to be asked about.
type Plan struct {
	Source string
	// SourceFolder is the source's own folder name when the source is a
	// folder ("" for a single-file source); BuildSession appends it to the
	// output root.
	SourceFolder string
	Skipped      []scan.Skipped
	Groups       []*group.Group
}

// Probe scans and probes the source (writing outputs to outDir) and groups
// the files.
func Probe(ctx context.Context, source, outDir string) (*Plan, error) {
	scanRes, err := scan.Scan(source, outDir)
	if err != nil {
		return nil, err
	}

	// Probe all files with a bounded worker set (not one goroutine per
	// file).
	medias := make([]*probe.Media, len(scanRes.Files))
	limit := pool.DefaultLimit()
	if limit > len(scanRes.Files) && len(scanRes.Files) > 0 {
		limit = len(scanRes.Files)
	}
	if limit < 1 {
		limit = 1
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < limit; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				m, err := probe.File(ctx, scanRes.Files[i])
				if err == nil {
					medias[i] = m
				}
			}
		}()
	}
	for i := range scanRes.Files {
		jobs <- i
	}
	close(jobs)
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
	return &Plan{Source: source, SourceFolder: scanRes.SourceFolder, Skipped: scanRes.Skipped, Groups: group.Files(probed)}, nil
}

// AskInputs builds the question list for the plan's groups, applying config
// defaults (questions already answered by the config have their Ask* flag
// cleared).
func (p *Plan) AskInputs(cfg *config.Config) []AskInput {
	if cfg == nil {
		cfg = &config.Config{}
	}
	// Attachments are only worth asking about when at least one file in the
	// plan actually carries embedded attachments (fonts, posters, ...).
	hasAttachments := false
	for _, g := range p.Groups {
		for _, m := range g.Files {
			if len(m.Attachments) > 0 {
				hasAttachments = true
				break
			}
		}
		if hasAttachments {
			break
		}
	}

	inputs := make([]AskInput, 0, len(p.Groups))
	for _, g := range p.Groups {
		if g.Audio == nil && len(g.Files) > 0 {
			g.Audio = g.Files[0].Audio
			g.Subtitles = g.Files[0].Subtitles
		}
		input := AskInput{
			Group:          g,
			AudioDefaults:  group.DefaultAudio(g),
			SubsDefaults:   group.DefaultSubs(g),
			PresetChoices:  cfg.PresetNames(),
			AskPreset:      cfg.DefaultPreset == "",
			AskNormalize:   cfg.Audio == nil || cfg.Audio.Normalize == nil,
			AskAttachments: cfg.Attachments == nil && hasAttachments,
		}
		input.Defaults = plan.Answers{
			Preset:      cfg.DefaultPresetName(),
			Normalize:   cfg.NormalizeDefault(),
			Attachments: cfg.Attachments == nil || *cfg.Attachments == "copy",
		}
		inputs = append(inputs, input)
	}
	return inputs
}

// BuildSession assembles the runnable session from per-group answers,
// writing outputs to outDir. The answers slice is index-aligned with the
// AskInputs result. Preset, Normalize and Attachments are queue-item-wide
// choices expected to be identical across groups; an empty Preset falls back
// to the configured default preset. Category, when set, is a configured
// output subfolder: outputs land in outDir/category. A folder source adds its
// own name as a subfolder: outDir[/category]/<source folder>.
func (p *Plan) BuildSession(cfg *config.Config, category string, answers []plan.Answers, outDir string) (*Session, error) {
	if cfg == nil {
		cfg = &config.Config{}
	}
	if len(answers) != len(p.Groups) {
		return nil, fmt.Errorf("%d answers for %d groups", len(answers), len(p.Groups))
	}
	if category != "" && !cfg.HasCategory(category) {
		return nil, fmt.Errorf("unknown category %q", category)
	}
	presets := plan.PresetsMap(cfg)
	sess := &Session{Source: p.Source, Skipped: p.Skipped, filesByPath: make(map[string]*FileTask)}
	defPreset := cfg.DefaultPresetName()
	outRoot := outDir
	if category != "" {
		outRoot = filepath.Join(outDir, category)
	}
	if p.SourceFolder != "" {
		outRoot = filepath.Join(outRoot, p.SourceFolder)
	}
	for gi, g := range p.Groups {
		if g.Audio == nil && len(g.Files) > 0 {
			g.Audio = g.Files[0].Audio
			g.Subtitles = g.Files[0].Subtitles
		}
		ans := answers[gi]
		name := defPreset
		if ans.Preset != "" {
			name = ans.Preset
		}
		for _, m := range g.Files {
			fileAns := ans
			resolvedAudio, rerr := resolveAudioIndexes(g.Audio, m.Audio, ans.Audio)
			if rerr != nil {
				return nil, rerr
			}
			fileAns.Audio = resolvedAudio
			resolvedSubs, rerr := resolveSubIndexes(g.Subtitles, m.Subtitles, ans.Subs)
			if rerr != nil {
				return nil, rerr
			}
			fileAns.Subs = resolvedSubs
			out := scan.OutputPathIn(outRoot, m.Path)
			task, err := plan.Build(m, name, fileAns, presets, cfg.Bitrate(), out)
			if err != nil {
				return nil, err
			}
			sess.Tasks = append(sess.Tasks, task)
			ft := &FileTask{
				Path:     m.Path,
				Output:   out,
				State:    StatePending,
				Duration: m.DurationOf(),
			}
			sess.files = append(sess.files, ft)
			sess.filesByPath[m.Path] = ft
		}
	}
	return sess, nil
}

// Prepare is the one-shot convenience for frontends that can ask
// synchronously: probe, ask about every group, build the session. The
// output folder is resolved from the config (FF_CONVERT_OUTPUT_DIR
// overriding, "converted" in the working directory as the fallback).
func Prepare(ctx context.Context, source string, cfg *config.Config, ask AskFunc) (*Session, error) {
	if cfg == nil {
		cfg = &config.Config{}
	}
	outDir := scan.ResolveOutputDir(cfg.OutputDir, scan.ConvertedDir)
	p, err := Probe(ctx, source, outDir)
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
	return p.BuildSession(cfg, "", answers, outDir)
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
			s.markStarted()
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
					if ctx.Err() != nil {
						f.State = StateCanceled
					} else {
						f.State = StateFailed
					}
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
	s.mu.Lock()
	s.finished = time.Now()
	s.mu.Unlock()

	var errs []error
	for _, f := range s.Files() {
		if f.State == StateFailed {
			errs = append(errs, fmt.Errorf("%s: %s", f.Path, f.Error))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// markStarted records the wall-clock time of the first actual task start.
func (s *Session) markStarted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started.IsZero() {
		s.started = time.Now()
	}
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
	return s.filesByPath[t.Input]
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

// resolveAudioIndexes maps a group's selected audio tracks (given either as
// the representative's absolute stream indexes or as list ordinals) to the
// corresponding absolute indexes in file m's own audio stream list.
func resolveAudioIndexes(rep, member []probe.Stream, selected []int) ([]int, error) {
	return resolveTrackIndexes(rep, member, selected, "audio")
}

// resolveSubIndexes does the same for subtitle tracks.
func resolveSubIndexes(rep, member []probe.Stream, selected []int) ([]int, error) {
	return resolveTrackIndexes(rep, member, selected, "subtitle")
}

func resolveTrackIndexes(rep, member []probe.Stream, selected []int, kind string) ([]int, error) {
	out := make([]int, 0, len(selected))
	for _, sel := range selected {
		ord := -1
		for i, s := range rep {
			if s.Index == sel {
				ord = i
				break
			}
		}
		if ord < 0 {
			// Fall back to treating the selection as a list ordinal.
			if sel >= 0 && sel < len(rep) {
				ord = sel
			}
		}
		if ord < 0 || ord >= len(member) {
			return nil, fmt.Errorf("no matching %s stream for selection %d", kind, sel)
		}
		out = append(out, member[ord].Index)
	}
	return out, nil
}
