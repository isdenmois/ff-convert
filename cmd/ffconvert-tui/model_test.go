package main

import (
	"testing"

	"github.com/isden/ff-convert/internal/convert"
	"github.com/isden/ff-convert/internal/group"
	"github.com/isden/ff-convert/internal/plan"
	"github.com/isden/ff-convert/internal/probe"
)

// newTestModel builds a wizard model over n fake groups, seeded like the
// probedMsg handler does, with every queue-item-wide question asked unless
// overridden.
func newTestModel(t *testing.T, n int, askPreset, askNormalize, askAttachments bool) *model {
	t.Helper()
	audio := []probe.Stream{{Index: 1, Language: "eng"}}
	subs := []probe.Stream{{Index: 2, Language: "eng"}}
	m := &model{}
	for i := 0; i < n; i++ {
		m.asks = append(m.asks, convert.AskInput{
			Group: &group.Group{
				Files:     []*probe.Media{{Path: "x.mkv"}},
				Audio:     audio,
				Subtitles: subs,
			},
			AudioDefaults:  []int{1},
			SubsDefaults:   []int{2},
			PresetChoices:  []string{"copy", "av1"},
			AskPreset:      askPreset,
			AskNormalize:   askNormalize,
			AskAttachments: askAttachments,
		})
		m.asks[i].Defaults = plan.Answers{Preset: "copy", Normalize: true, Attachments: true}
	}
	m.reply = make([]plan.Answers, len(m.asks))
	for i, a := range m.asks {
		m.reply[i] = plan.Answers{
			Preset:      a.Defaults.Preset,
			Normalize:   a.Defaults.Normalize,
			Attachments: a.Defaults.Attachments,
		}
	}
	m.phase = phaseAsking
	m.enterGroup(0)
	return m
}

// TestQueueWideQuestionsAskedOnce walks the wizard with two groups and all
// questions asked: the normalize/attachments/preset steps must appear only
// after the last group, and their answers must be replicated into every
// group's reply.
func TestQueueWideQuestionsAskedOnce(t *testing.T) {
	m := newTestModel(t, 2, true, true, true)

	// Group 0: audio, subs — then straight to group 1.
	m.commitStep()
	if m.step != stepSubs {
		t.Fatalf("after audio: step = %d, want stepSubs", m.step)
	}
	m.commitStep()
	if m.groupIdx != 1 || m.step != stepAudio {
		t.Fatalf("after group 0 subs: group = %d step = %d, want group 1 stepAudio", m.groupIdx, m.step)
	}

	// Group 1: audio, subs — then the queue-item-wide questions.
	m.commitStep()
	m.commitStep()
	if m.step != stepNormalize {
		t.Fatalf("after last group subs: step = %d, want stepNormalize", m.step)
	}

	m.cursor = 1 // No — keep original loudness
	m.commitStep()
	if m.step != stepAttach {
		t.Fatalf("after normalize: step = %d, want stepAttach", m.step)
	}

	m.cursor = 1 // No — drop attachments
	m.commitStep()
	if m.step != stepPreset {
		t.Fatalf("after attach: step = %d, want stepPreset", m.step)
	}

	m.cursor = 1 // av1
	m.commitStep()
	if m.step != stepConfirm {
		t.Fatalf("after preset: step = %d, want stepConfirm", m.step)
	}

	for i, ans := range m.reply {
		if ans.Normalize {
			t.Errorf("reply[%d].Normalize = true, want false (replicated queue-wide choice)", i)
		}
		if ans.Attachments {
			t.Errorf("reply[%d].Attachments = true, want false (replicated queue-wide choice)", i)
		}
		if ans.Preset != "av1" {
			t.Errorf("reply[%d].Preset = %q, want %q (replicated queue-wide choice)", i, ans.Preset, "av1")
		}
	}
}

// TestConfigAnsweredQuestionsSkipped verifies that config-answered questions
// are never stepped into: after the last group's subs the wizard goes
// straight to confirm when nothing is asked.
func TestConfigAnsweredQuestionsSkipped(t *testing.T) {
	m := newTestModel(t, 2, false, false, false)

	m.commitStep() // group 0 audio
	m.commitStep() // group 0 subs → group 1
	m.commitStep() // group 1 audio
	m.commitStep() // group 1 subs → confirm
	if m.step != stepConfirm {
		t.Fatalf("step = %d, want stepConfirm", m.step)
	}
	for i, ans := range m.reply {
		if !ans.Normalize || !ans.Attachments || ans.Preset != "copy" {
			t.Errorf("reply[%d] = %+v, want seeded defaults", i, ans)
		}
	}
}
