package jobs

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/isden/ff-convert/internal/plan"
	"github.com/isden/ff-convert/internal/pool"
	"github.com/isden/ff-convert/internal/scan"
)

func makeFixture(t *testing.T, dir, name string) string {
	t.Helper()
	out := filepath.Join(dir, name)
	cmd := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=duration=1:size=160x120:rate=10",
		"-f", "lavfi", "-i", "sine=duration=1:frequency=440",
		"-map", "0:v", "-map", "1:a",
		"-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac",
		"-metadata:s:a:0", "language=eng",
		out)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v\n%s", err, b)
	}
	return out
}

func waitState(t *testing.T, m *Manager, id string, want State) JobView {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		v, err := m.View(id)
		if err != nil {
			t.Fatal(err)
		}
		if v.State == want {
			return v
		}
		if v.State == StateFailed && want != StateFailed {
			t.Fatalf("job failed early: %s", v.Error)
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("job did not reach %s (last: %+v)", want, m.Views())
	return JobView{}
}

func TestJobLifecycle(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	dir := t.TempDir()
	outDir := t.TempDir()
	t.Setenv(scan.EnvOutputDir, outDir)
	makeFixture(t, dir, "a.mkv")
	makeFixture(t, dir, "b.mkv")

	m := NewManager(nil, pool.New(2))
	notifications := 0
	unsub := m.Subscribe(func() { notifications++ })
	defer unsub()

	v, err := m.Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	if v.State != StateProbing {
		t.Fatalf("state = %s", v.State)
	}
	q := waitState(t, m, v.ID, StateQuestions)
	if q.Questions == nil || len(q.Questions.Groups) != 1 {
		t.Fatalf("questions = %+v", q.Questions)
	}
	if len(q.Questions.Groups[0].AudioDefaults) != 1 {
		t.Fatalf("audio defaults = %+v", q.Questions.Groups[0])
	}
	if len(q.Questions.Groups[0].Tasks) != 2 || q.Questions.Groups[0].Tasks[0].Name != "a.mkv" {
		t.Fatalf("tasks = %+v", q.Questions.Groups[0].Tasks)
	}
	if !q.Questions.AskPreset || !q.Questions.AskNormalize || !q.Questions.AskAttachments {
		t.Fatal("all questions must be asked with nil config")
	}
	if q.Questions.DefaultPreset != "copy" {
		t.Fatalf("default preset = %q, want copy", q.Questions.DefaultPreset)
	}
	if !q.Questions.Defaults.Normalize {
		t.Fatal("normalization default must be on")
	}

	// Defaults + per-task copy presets: fast conversion.
	answers := []plan.Answers{{
		Audio:   q.Questions.Groups[0].AudioDefaults,
		Presets: []string{"copy", "copy"},
	}}
	if err := m.SubmitAnswers(v.ID, answers); err != nil {
		t.Fatal(err)
	}
	done := waitState(t, m, v.ID, StateDone)
	if done.Total != 2 || done.Done != 2 || done.Overall != 1 {
		t.Fatalf("done view = %+v", done)
	}
	for _, f := range done.Files {
		if _, err := os.Stat(f.Output); err != nil {
			t.Fatalf("output missing: %v", err)
		}
	}
	if notifications < 3 {
		t.Fatalf("notifications = %d, want >= 3", notifications)
	}

	// Re-adding the same folder must skip everything.
	v2, err := m.Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	done2 := waitState(t, m, v2.ID, StateDone)
	if len(done2.Skipped) != 2 {
		t.Fatalf("skipped = %+v", done2.Skipped)
	}

	// Submitting answers to a finished job must fail.
	if err := m.SubmitAnswers(v2.ID, answers); err == nil {
		t.Fatal("submitting to done job must fail")
	}

	// Remove.
	if err := m.Remove(v2.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := m.View(v2.ID); err == nil {
		t.Fatal("removed job must be gone")
	}
}
