package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"

	"github.com/isden/ff-convert/internal/convert"
	"github.com/isden/ff-convert/internal/probe"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("213")).
			MarginBottom(1)

	groupStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205"))

	checkedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)

	errStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	okStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("42"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)

const barWidth = 28

var progressBar = progress.New(progress.WithWidth(barWidth), progress.WithSolidFill("213"))

func (m model) View() string {
	var b strings.Builder
	switch m.phase {
	case phaseProbing:
		b.WriteString(titleStyle.Render("ff-convert"))
		b.WriteString("\n\n")
		b.WriteString(m.spinner.View() + " scanning and probing " + m.source + " …\n\n")
		b.WriteString(helpStyle.Render("ctrl+c to abort"))

	case phaseAsking:
		b.WriteString(m.viewAsk())

	case phaseRunning:
		b.WriteString(m.viewRunning())

	case phaseDone:
		b.WriteString(m.viewDone())

	case phaseError:
		b.WriteString(titleStyle.Render("ff-convert — error"))
		b.WriteString("\n\n")
		b.WriteString(errStyle.Render(m.err.Error()))
		b.WriteString(helpStyle.Render("\n\nq to quit"))
	}
	return b.String()
}

func (m model) viewAsk() string {
	var b strings.Builder
	a := m.currentAsk()
	b.WriteString(titleStyle.Render("ff-convert — conversion setup"))
	b.WriteString(fmt.Sprintf("\n%s %s\n\n", groupStyle.Render("group"), groupStyle.Render(fmt.Sprintf("%d/%d", m.groupIdx+1, len(m.asks)))))

	switch m.step {
	case stepAudio, stepSubs:
		var tracks []string
		kind := "audio tracks"
		if m.step == stepAudio {
			for _, s := range a.Group.Audio {
				tracks = append(tracks, s.Label())
			}
		} else {
			kind = "subtitle tracks"
			for _, s := range a.Group.Subtitles {
				tracks = append(tracks, s.Label())
			}
		}
		if m.step == stepSubs && len(tracks) == 0 {
			b.WriteString("no subtitle tracks — nothing to select\n")
		}
		b.WriteString(fmt.Sprintf("Which %s to keep?\n\n", kind))
		for i, label := range tracks {
			idx := i
			if m.step == stepAudio {
				idx = a.Group.Audio[i].Index
			} else {
				idx = a.Group.Subtitles[i].Index
			}
			box := "[ ]"
			if m.checked[idx] {
				box = checkedStyle.Render("[x]")
			}
			cursor := "  "
			if m.cursor == i {
				cursor = cursorStyle.Render("❯ ")
			}
			b.WriteString(fmt.Sprintf("%s %s %s\n", cursor, box, label))
		}
		b.WriteString(helpStyle.Render("\n↑/↓ move · space toggle · enter continue"))

	case stepPreset, stepNormalize, stepAttach:
		var title string
		switch m.step {
		case stepPreset:
			files := a.Group.Files
			name := filepath.Base(files[m.taskIdx].Path)
			title = fmt.Sprintf("Video output preset — %s (file %d/%d)", name, m.taskIdx+1, len(files))
		case stepNormalize:
			title = "Normalize audio loudness?"
		case stepAttach:
			title = "Keep embedded attachments (fonts, posters)?"
		}
		b.WriteString(title + "\n\n")
		for i, opt := range m.options() {
			cursor := "  "
			if m.cursor == i {
				cursor = cursorStyle.Render("❯ ")
			}
			b.WriteString(cursor + " " + opt + "\n")
		}
		b.WriteString(helpStyle.Render("\n↑/↓ move · enter/y/n select"))

	case stepConfirm:
		b.WriteString(m.viewConfirm())
		b.WriteString(helpStyle.Render("\nenter start conversion · ctrl+c abort"))
	}
	return b.String()
}

func (m model) viewConfirm() string {
	var b strings.Builder
	b.WriteString("Ready to convert:\n\n")
	for gi, a := range m.asks {
		ans := m.reply[gi]
		b.WriteString(fmt.Sprintf("%s  %s\n", groupStyle.Render(fmt.Sprintf("group %d/%d:", gi+1, len(m.asks))), dimStyle.Render(fmt.Sprintf("%d file(s)", len(a.Group.Files)))))
		b.WriteString("  audio:       " + joinLabels(a.Group.Audio, ans.Audio) + "\n")
		if len(a.Group.Subtitles) == 0 {
			b.WriteString("  subtitles:   (none)\n")
		} else {
			b.WriteString("  subtitles:   " + joinLabels(a.Group.Subtitles, ans.Subs) + "\n")
		}
		b.WriteString(fmt.Sprintf("  normalize:   %s\n", yesNo(ans.Normalize)))
		b.WriteString(fmt.Sprintf("  attachments: %s\n", yesNo(ans.Attachments)))
		for fi, f := range a.Group.Files {
			preset := ""
			if fi < len(ans.Presets) {
				preset = ans.Presets[fi]
			}
			b.WriteString(fmt.Sprintf("  %-12s%s → %s\n", "file:", filepath.Base(f.Path), preset))
		}
		if gi < len(m.asks)-1 {
			b.WriteString("\n")
		}
	}
	if n := len(m.plan.Skipped); n > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf("\n(%d file(s) skipped: already converted)", n)))
	}
	return b.String()
}

func joinLabels(all []probe.Stream, selected []int) string {
	sel := make(map[int]bool, len(selected))
	for _, idx := range selected {
		sel[idx] = true
	}
	var parts []string
	for _, s := range all {
		if sel[s.Index] {
			parts = append(parts, s.Label())
		}
	}
	if len(parts) == 0 {
		return "(none)"
	}
	return strings.Join(parts, " | ")
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func (m model) viewRunning() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("ff-convert — converting"))
	sn := m.sess.Snapshot()

	bar := progressBar.ViewAs(sn.Overall)
	b.WriteString(fmt.Sprintf("\noverall %s %3.0f%%   %d/%d done", bar, sn.Overall*100, sn.Done, sn.Total))
	if sn.Failed > 0 {
		b.WriteString(errStyle.Render(fmt.Sprintf("   %d failed", sn.Failed)))
	}
	b.WriteString("\n\n")

	for _, f := range sn.Files {
		name := filepath.Base(f.Path)
		switch f.State {
		case convert.StateDone:
			b.WriteString(okStyle.Render("  ✓ ") + name + "\n")
		case convert.StateFailed:
			b.WriteString(errStyle.Render("  ✗ ") + name + "\n")
		case convert.StateRunning:
			bar := progressBar.ViewAs(f.Percent)
			speed := f.Speed
			if speed == "" {
				speed = "…"
			}
			b.WriteString(fmt.Sprintf("  ▶ %s %s  %s\n", name, bar, dimStyle.Render(speed)))
		case convert.StateCanceled:
			b.WriteString(dimStyle.Render("  − " + name + " (canceled)"))
		default:
			b.WriteString(dimStyle.Render("  · " + name))
		}
		b.WriteString("\n")
	}
	b.WriteString(helpStyle.Render("\nctrl+c cancel remaining"))
	return b.String()
}

func (m model) viewDone() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("ff-convert — finished"))
	if m.sess == nil {
		b.WriteString("\nnothing to convert (all files already have outputs)\n")
		return b.String()
	}
	sn := m.sess.Snapshot()
	b.WriteString(fmt.Sprintf("\n%d/%d files converted", sn.Done, sn.Total))
	if sn.Failed > 0 {
		b.WriteString(errStyle.Render(fmt.Sprintf(", %d failed:\n", sn.Failed)))
		for _, f := range sn.Files {
			if f.State == convert.StateFailed {
				b.WriteString(errStyle.Render(fmt.Sprintf("  ✗ %s\n    %s\n", filepath.Base(f.Path), f.Error)))
			}
		}
	} else {
		b.WriteString("\n")
	}
	for _, s := range m.plan.Skipped {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  − skipped (exists): %s\n", filepath.Base(s.Path))))
	}
	b.WriteString(helpStyle.Render("\nq to quit"))
	return b.String()
}
