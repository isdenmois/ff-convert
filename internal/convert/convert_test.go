package convert

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/isden/ff-convert/internal/config"
	"github.com/isden/ff-convert/internal/plan"
	"github.com/isden/ff-convert/internal/pool"
	"github.com/isden/ff-convert/internal/probe"
	"github.com/isden/ff-convert/internal/scan"
)

// makeFixture generates a small mkv with eng 5.1 + rus stereo audio and an
// eng subtitle.
func makeFixture(t *testing.T, dir, name string) string {
	t.Helper()
	srt := filepath.Join(dir, "sub.srt")
	if err := os.WriteFile(srt, []byte("1\n00:00:00,000 --> 00:00:01,000\nhi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, name)
	cmd := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=duration=2:size=160x120:rate=10",
		"-f", "lavfi", "-i", "anoisesrc=duration=2:colour=pink:seed=1,pan=5.1|c0=c0|c1=c0|c2=c0|c3=c0|c4=c0|c5=c0",
		"-f", "lavfi", "-i", "anoisesrc=duration=2:colour=white:seed=2,pan=stereo|c0=c0|c1=c0",
		"-i", srt,
		"-map", "0:v", "-map", "1:a", "-map", "2:a", "-map", "3:s",
		"-c:v", "libx264", "-preset", "ultrafast",
		"-c:a", "aac", "-metadata:s:a:0", "language=eng", "-metadata:s:a:1", "language=rus",
		"-c:s", "srt", "-metadata:s:s:0", "language=eng",
		out)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture %s: %v\n%s", name, err, b)
	}
	return out
}

func TestPrepareAndRunEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	dir := t.TempDir()
	outDir := t.TempDir()
	t.Setenv(scan.EnvOutputDir, outDir)
	makeFixture(t, dir, "a.mkv")
	makeFixture(t, dir, "b.mkv")

	// A custom preset backed by ffmpeg's built-in mpeg4 encoder (always
	// available) to exercise a per-task codec different from copy.
	cfg := &config.Config{
		Presets: map[string]config.Preset{
			"mpeg4": {Codec: "mpeg4", Args: []string{"-q:v", "5"}},
		},
	}

	var asked int
	ask := func(in AskInput) (plan.Answers, error) {
		asked++
		if len(in.Group.Files) != 2 {
			t.Errorf("group should contain both files, got %d", len(in.Group.Files))
		}
		if len(in.AudioDefaults) != 1 || in.AudioDefaults[0] != 1 {
			t.Errorf("audio defaults = %v, want [1] (eng 5.1)", in.AudioDefaults)
		}
		if len(in.SubsDefaults) != 1 {
			t.Errorf("sub defaults = %v", in.SubsDefaults)
		}
		if !in.AskPreset || !in.AskNormalize || !in.AskAttachments {
			t.Error("all questions must be asked with unset config")
		}
		// Defaults must preselect copy and normalization-on per task.
		if len(in.Defaults.Presets) != 2 || in.Defaults.Presets[0] != "copy" || in.Defaults.Presets[1] != "copy" {
			t.Errorf("preset defaults = %v, want [copy copy]", in.Defaults.Presets)
		}
		if !in.Defaults.Normalize {
			t.Error("normalization default must be on")
		}
		return plan.Answers{
			Audio:       in.AudioDefaults,
			Subs:        in.SubsDefaults,
			Presets:     []string{"copy", "mpeg4"}, // per-task codecs
			Normalize:   false,
			Attachments: false,
		}, nil
	}

	sess, err := Prepare(context.Background(), dir, cfg, ask)
	if err != nil {
		t.Fatal(err)
	}
	if asked != 1 {
		t.Fatalf("ask called %d times, want 1 (both files in one group)", asked)
	}
	if len(sess.Tasks) != 2 {
		t.Fatalf("tasks = %d", len(sess.Tasks))
	}
	if err := sess.EnsureOutputDir(); err != nil {
		t.Fatal(err)
	}
	if err := sess.Run(context.Background(), pool.New(2), nil); err != nil {
		t.Fatal(err)
	}
	for _, f := range sess.Files() {
		if f.State != StateDone {
			t.Fatalf("file %s state=%s err=%s", f.Path, f.State, f.Error)
		}
		if _, err := os.Stat(f.Output); err != nil {
			t.Fatalf("output missing: %v", err)
		}
		if filepath.Dir(f.Output) != outDir {
			t.Fatalf("output %q must be in the configured output dir", f.Output)
		}
	}
	if got := sess.Snapshot().Overall; got != 1 {
		t.Fatalf("overall = %v", got)
	}

	// Output must contain the selected eng audio as opus stereo and the sub.
	m, err := probe.File(context.Background(), filepath.Join(outDir, "a.mkv"))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Audio) != 1 || m.Audio[0].Codec != "opus" || m.Audio[0].Channels != 2 || m.Audio[0].Language != "eng" {
		t.Fatalf("output audio = %+v", m.Audio)
	}
	if len(m.Subtitles) != 1 || m.Subtitles[0].Language != "eng" {
		t.Fatalf("output subs = %+v", m.Subtitles)
	}
	if m.Video[0].Codec != "h264" {
		t.Fatalf("a.mkv video should be copied h264, got %s", m.Video[0].Codec)
	}

	// The second task was given its own codec: mpeg4 instead of copy.
	m, err = probe.File(context.Background(), filepath.Join(outDir, "b.mkv"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Video[0].Codec != "mpeg4" {
		t.Fatalf("b.mkv video should be re-encoded mpeg4, got %s", m.Video[0].Codec)
	}
}
