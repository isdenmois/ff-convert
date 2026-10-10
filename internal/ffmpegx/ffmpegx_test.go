package ffmpegx

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParseProgress(t *testing.T) {
	input := "frame=10\nfps=0.0\nout_time_us=1000000\nspeed=1.25x\nprogress=continue\n" +
		"frame=20\nout_time_us=2000000\nspeed=2.00x\nprogress=end\n"
	var got []Stats
	_, err := runParse(input, 10*time.Second, &got)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("progress updates = %d, want 2", len(got))
	}
	if got[0].OutTime != time.Second || got[0].Speed != "1.25x" {
		t.Fatalf("first stats = %+v", got[0])
	}
	if got[0].Percent != 0.1 {
		t.Fatalf("first percent = %v", got[0].Percent)
	}
	if got[1].Percent != 0.2 {
		t.Fatalf("second percent = %v", got[1].Percent)
	}
}

func TestParseProgressQuirkMs(t *testing.T) {
	// Old ffmpeg builds emit out_time_ms in microseconds and no out_time_us.
	input := "out_time_ms=1500000\nprogress=continue\n"
	var got []Stats
	_, err := runParse(input, time.Second, &got)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].OutTime != 1500*time.Millisecond {
		t.Fatalf("stats = %+v", got)
	}
}

func runParse(input string, total time.Duration, out *[]Stats) (Stats, error) {
	var last Stats
	opts := Options{
		Duration: total,
		OnProgress: func(s Stats) {
			last = s
			*out = append(*out, s)
		},
	}
	r := strings.NewReader(input)
	parseProgress(r, opts)
	return last, nil
}

func TestRunReal(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "in.mkv")
	gen := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=duration=2:size=128x96:rate=25",
		"-f", "lavfi", "-i", "sine=duration=2:frequency=440",
		"-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac",
		src)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v\n%s", err, out)
	}

	outPath := filepath.Join(dir, "out.mkv")
	var mu sync.Mutex
	var maxPct float64
	err := Run(context.Background(), Options{
		Args: []string{"-hide_banner", "-loglevel", "error", "-y",
			"-i", src, "-c:v", "libx264", "-preset", "ultrafast",
			"-progress", "pipe:1", "-nostats", outPath},
		Duration: 2 * time.Second,
		OnProgress: func(s Stats) {
			mu.Lock()
			if s.Percent > maxPct {
				maxPct = s.Percent
			}
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("output missing: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if maxPct < 0.5 {
		t.Fatalf("max progress = %v, want > 0.5", maxPct)
	}
}

func TestRunErrorTail(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	err := Run(context.Background(), Options{
		Args: []string{"-hide_banner", "-loglevel", "error", "-y",
			"-i", "/nonexistent/file.mkv", "-c", "copy", "/tmp/never.mkv"},
	})
	if err == nil {
		t.Fatal("must fail for missing input")
	}
	if !strings.Contains(err.Error(), "ffmpeg failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}
