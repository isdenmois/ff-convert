package main

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/isden/ff-convert/internal/config"
	"github.com/isden/ff-convert/internal/convert"
	"github.com/isden/ff-convert/internal/plan"
	"github.com/isden/ff-convert/internal/pool"
	"github.com/isden/ff-convert/internal/probe"
)

type phase int

const (
	phaseProbing phase = iota
	phaseAsking
	phaseRunning
	phaseDone
	phaseError
)

type step int

const (
	stepAudio step = iota
	stepSubs
	stepPreset
	stepNormalize
	stepAttach
	stepConfirm
)

// messages

type probedMsg struct {
	plan  *convert.Plan
	asks  []convert.AskInput
	files int
	skips int
}

type errMsg struct{ err error }

type startedMsg struct{ sess *convert.Session }

type tickMsg struct{}

// model is the root bubbletea model.
type model struct {
	source string
	cfg    *config.Config
	pool   *pool.Pool

	phase   phase
	err     error
	spinner spinner.Model

	plan  *convert.Plan
	asks  []convert.AskInput
	reply []plan.Answers // index-aligned with asks

	groupIdx int
	step     step
	cursor   int
	checked  map[int]bool // stream index → checked (audio/subs steps)

	sess   *convert.Session
	ctx    context.Context
	cancel context.CancelFunc
}

func newModel(source string, cfg *config.Config, p *pool.Pool) model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return model{
		source:  source,
		cfg:     cfg,
		pool:    p,
		phase:   phaseProbing,
		spinner: sp,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.probeCmd())
}

func (m model) probeCmd() tea.Cmd {
	source, cfg := m.source, m.cfg
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		p, err := convert.Probe(ctx, source)
		if err != nil {
			return errMsg{err}
		}
		files := 0
		for _, g := range p.Groups {
			files += len(g.Files)
		}
		return probedMsg{plan: p, asks: p.AskInputs(cfg), files: files, skips: len(p.Skipped)}
	}
}

// currentAsk returns the ask input for the group being edited.
func (m *model) currentAsk() convert.AskInput { return m.ask(m.groupIdx) }
func (m *model) ask(i int) convert.AskInput   { return m.asks[i] }

// enterGroup initializes the wizard state for group gi.
func (m *model) enterGroup(gi int) {
	m.groupIdx = gi
	m.checked = map[int]bool{}
	a := m.ask(gi)
	for _, idx := range a.AudioDefaults {
		m.checked[idx] = true
	}
	m.cursor = 0
	m.step = stepAudio
}

// nextStep advances to the next applicable step, skipping questions that the
// config already answered, moving between groups and ending at confirm.
func (m *model) nextStep() {
	switch m.step {
	case stepAudio:
		m.step = stepSubs
		m.resetCursorForStep()
	case stepSubs:
		if m.ask(m.groupIdx).AskPreset {
			m.step = stepPreset
		} else {
			m.step = stepNormalize
		}
		m.resetCursorForStep()
	case stepPreset:
		m.step = stepNormalize
		m.resetCursorForStep()
	case stepNormalize:
		m.step = stepAttach
		m.resetCursorForStep()
	case stepAttach:
		if m.groupIdx+1 < len(m.asks) {
			m.enterGroup(m.groupIdx + 1)
		} else {
			m.step = stepConfirm
			m.cursor = 0
		}
	case stepConfirm:
		// handled by confirm()
	}
}

// resetCursorForStep positions the cursor on the sensible default for the
// new step.
func (m *model) resetCursorForStep() {
	m.cursor = 0
	a := m.currentAsk()
	switch m.step {
	case stepPreset:
		for i, name := range a.PresetChoices {
			if name == "av1" {
				m.cursor = i
				break
			}
		}
	case stepNormalize:
		m.cursor = 1 // default: No
	case stepAttach:
		m.cursor = 0 // default: Yes (copy)
	}
}

// commitStep stores the current step's answer and advances.
func (m *model) commitStep() tea.Cmd {
	a := m.currentAsk()
	ans := m.reply[m.groupIdx]
	switch m.step {
	case stepAudio:
		ans.Audio = sortedIndexes(m.checked, a.Group.Audio)
	case stepSubs:
		ans.Subs = sortedIndexes(m.checked, a.Group.Subtitles)
	case stepPreset:
		if m.cursor < len(a.PresetChoices) {
			ans.Preset = a.PresetChoices[m.cursor]
		}
	case stepNormalize:
		ans.Normalize = m.cursor == 0
	case stepAttach:
		ans.Attachments = m.cursor == 0
	case stepConfirm:
		return m.confirm()
	}
	if m.step != stepConfirm {
		m.reply[m.groupIdx] = ans
		m.nextStep()
	}
	return nil
}

// options returns the selectable entries for choice-style steps.
func (m *model) options() []string {
	a := m.currentAsk()
	switch m.step {
	case stepPreset:
		return a.PresetChoices
	case stepNormalize:
		return []string{"Yes — apply loudnorm (i=-13:lra=15:tp=-1)", "No — keep original loudness"}
	case stepAttach:
		return []string{"Yes — copy attachments (fonts, posters)", "No — drop attachments"}
	}
	return nil
}

func (m *model) confirm() tea.Cmd {
	sess, err := m.plan.BuildSession(m.cfg, m.reply)
	if err != nil {
		m.phase = phaseError
		m.err = err
		return nil
	}
	if err := sess.EnsureOutputDir(); err != nil {
		m.phase = phaseError
		m.err = err
		return nil
	}
	m.sess = sess
	m.phase = phaseRunning
	m.ctx, m.cancel = context.WithCancel(context.Background())

	sessRef, poolRef, ctxRef := sess, m.pool, m.ctx
	go func() {
		_ = sessRef.Run(ctxRef, poolRef, nil) // errors are recorded per-file
	}()
	return tea.Batch(tickCmd(), startedMsgCmd(sess))
}

func startedMsgCmd(sess *convert.Session) tea.Cmd {
	return func() tea.Msg { return startedMsg{sess: sess} }
}

func tickCmd() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case errMsg:
		m.phase = phaseError
		m.err = msg.err
		return m, nil

	case probedMsg:
		m.plan = msg.plan
		m.asks = msg.asks
		m.reply = make([]plan.Answers, len(msg.asks))
		for i, a := range msg.asks {
			m.reply[i] = plan.Answers{
				Preset:      a.Defaults.Preset,
				Normalize:   a.Defaults.Normalize,
				Attachments: a.Defaults.Attachments,
			}
		}
		if len(msg.asks) == 0 {
			m.phase = phaseDone // nothing to convert (all skipped)
			return m, tea.Quit
		}
		m.phase = phaseAsking
		m.enterGroup(0)
		return m, nil

	case startedMsg:
		return m, nil

	case tickMsg:
		if m.phase != phaseRunning {
			return m, nil
		}
		sn := m.sess.Snapshot()
		if sn.Done+sn.Failed >= sn.Total && sn.Total > 0 {
			m.phase = phaseDone
			return m, nil
		}
		return m, tickCmd()

	case spinner.TickMsg:
		if m.phase == phaseProbing {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		if m.phase == phaseRunning && m.cancel != nil {
			m.cancel()
			m.phase = phaseDone
			return m, nil
		}
		return m, tea.Quit
	case "q":
		if m.phase == phaseDone || m.phase == phaseError {
			return m, tea.Quit
		}
	}

	switch m.phase {
	case phaseAsking:
		return m.handleAskKey(msg)
	}
	return m, nil
}

func (m model) handleAskKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	listLen := len(m.options())
	if m.step == stepAudio {
		listLen = len(m.currentAsk().Group.Audio)
	} else if m.step == stepSubs {
		listLen = len(m.currentAsk().Group.Subtitles)
	}

	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < listLen-1 {
			m.cursor++
		}
	case " ", "space", "enter":
		if m.step == stepAudio || m.step == stepSubs {
			if msg.String() == "enter" {
				return m, m.commitStep()
			}
			var idx int
			if m.step == stepAudio {
				idx = m.currentAsk().Group.Audio[m.cursor].Index
			} else {
				idx = m.currentAsk().Group.Subtitles[m.cursor].Index
			}
			// At least one audio track must stay selected.
			if m.step == stepAudio && m.checked[idx] && len(m.checked) <= 1 {
				return m, nil
			}
			if m.checked[idx] {
				delete(m.checked, idx)
			} else {
				m.checked[idx] = true
			}
			return m, nil
		}
		return m, m.commitStep()
	case "y":
		if m.step == stepNormalize || m.step == stepAttach {
			m.cursor = 0
			return m, m.commitStep()
		}
	case "n":
		if m.step == stepNormalize || m.step == stepAttach {
			m.cursor = 1
			return m, m.commitStep()
		}
	case "tab":
		return m, m.commitStep()
	}
	return m, nil
}

// sortedIndexes returns the checked stream indexes in group track order.
func sortedIndexes(set map[int]bool, tracks []probe.Stream) (out []int) {
	for _, s := range tracks {
		if set[s.Index] {
			out = append(out, s.Index)
		}
	}
	return out
}
