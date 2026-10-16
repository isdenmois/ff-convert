package jobs

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/isden/ff-convert/internal/config"
	"github.com/isden/ff-convert/internal/plan"
	"github.com/isden/ff-convert/internal/pool"
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
	makeFixture(t, dir, "a.mkv")
	makeFixture(t, dir, "b.mkv")

	m := NewManager(nil, pool.New(2), outDir)
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
	if v.OutputDir != outDir {
		t.Fatalf("output dir = %q, want %q", v.OutputDir, outDir)
	}
	if v.OutputFolder != outDir {
		t.Fatalf("probing output folder = %q, want %q", v.OutputFolder, outDir)
	}
	q := waitState(t, m, v.ID, StateQuestions)
	if want := filepath.Join(outDir, filepath.Base(dir)); q.OutputFolder != want {
		t.Fatalf("questions output folder = %q, want %q", q.OutputFolder, want)
	}
	if q.Questions == nil || len(q.Questions.Groups) != 1 {
		t.Fatalf("questions = %+v", q.Questions)
	}
	if len(q.Questions.Groups[0].AudioDefaults) != 1 {
		t.Fatalf("audio defaults = %+v", q.Questions.Groups[0])
	}
	if len(q.Questions.Groups[0].Tasks) != 2 || q.Questions.Groups[0].Tasks[0].Name != "a.mkv" {
		t.Fatalf("tasks = %+v", q.Questions.Groups[0].Tasks)
	}
	if !q.Questions.AskPreset || !q.Questions.AskNormalize || q.Questions.AskAttachments {
		t.Fatal("all questions must be asked with nil config")
	}
	if q.Questions.DefaultPreset != "copy" {
		t.Fatalf("default preset = %q, want copy", q.Questions.DefaultPreset)
	}
	if !q.Questions.Defaults.Normalize {
		t.Fatal("normalization default must be on")
	}

	// Defaults + the copy preset: fast conversion.
	answers := []plan.Answers{{
		Audio:  q.Questions.Groups[0].AudioDefaults,
		Preset: "copy",
	}}
	if err := m.SubmitAnswers(v.ID, "", answers); err != nil {
		t.Fatal(err)
	}
	done := waitState(t, m, v.ID, StateDone)
	if done.Total != 2 || done.Done != 2 || done.Overall != 1 {
		t.Fatalf("done view = %+v", done)
	}
	if done.ElapsedMs <= 0 {
		t.Fatalf("done view elapsed_ms = %d, want > 0", done.ElapsedMs)
	}
	if done.EtaMs != 0 {
		t.Fatalf("done view eta_ms = %d, want 0", done.EtaMs)
	}
	want := filepath.Join(outDir, filepath.Base(dir))
	if done.OutputFolder != want {
		t.Fatalf("done output folder = %q, want %q", done.OutputFolder, want)
	}
	for _, f := range done.Files {
		if _, err := os.Stat(f.Output); err != nil {
			t.Fatalf("output missing: %v", err)
		}
		if filepath.Dir(f.Output) != want {
			t.Fatalf("file %s output %q not under %q", f.Name, f.Output, want)
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
	if err := m.SubmitAnswers(v2.ID, "", answers); err == nil {
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

// Jobs pin the output folder at creation: changing the manager's output dir
// must not affect jobs added earlier.
func TestOutputDirPinnedAtCreate(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(nil, pool.New(1), "/out/first")

	a, err := m.Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	if a.OutputDir != "/out/first" {
		t.Fatalf("job a output dir = %q", a.OutputDir)
	}

	m.SetOutputDir("/out/second")
	if got := m.OutputDir(); got != "/out/second" {
		t.Fatalf("manager output dir = %q, want /out/second", got)
	}

	va, err := m.View(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if va.OutputDir != "/out/first" {
		t.Fatalf("existing job must keep its pinned dir, got %q", va.OutputDir)
	}

	b, err := m.Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	if b.OutputDir != "/out/second" {
		t.Fatalf("new job must use the current dir, got %q", b.OutputDir)
	}
}

// The card's folder must include the category chosen with the answers, so it
// matches where the files are actually written.
func TestOutputFolderIncludesCategory(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	dir := t.TempDir()
	outDir := t.TempDir()
	makeFixture(t, dir, "a.mkv")

	m := NewManager(&config.Config{Categories: []string{"series"}}, pool.New(1), outDir)
	v, err := m.Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	q := waitState(t, m, v.ID, StateQuestions)
	if want := filepath.Join(outDir, filepath.Base(dir)); q.OutputFolder != want {
		t.Fatalf("questions output folder = %q, want %q", q.OutputFolder, want)
	}

	answers := []plan.Answers{{Audio: q.Questions.Groups[0].AudioDefaults, Preset: "copy"}}
	if err := m.SubmitAnswers(v.ID, "series", answers); err != nil {
		t.Fatal(err)
	}
	done := waitState(t, m, v.ID, StateDone)
	want := filepath.Join(outDir, "series", filepath.Base(dir))
	if done.OutputFolder != want {
		t.Fatalf("done output folder = %q, want %q", done.OutputFolder, want)
	}
	for _, f := range done.Files {
		if filepath.Dir(f.Output) != want {
			t.Fatalf("file %s output %q not under %q", f.Name, f.Output, want)
		}
	}
}
