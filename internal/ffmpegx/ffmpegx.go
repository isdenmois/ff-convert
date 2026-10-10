// Package ffmpegx runs ffmpeg with machine-readable progress parsing.
package ffmpegx

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// FFmpegPath can be overridden for tests.
var FFmpegPath = "ffmpeg"

// Stats is a progress snapshot for one running conversion.
type Stats struct {
	// OutTime is the encoded output timestamp.
	OutTime time.Duration
	// Speed is ffmpeg's processing speed (e.g. "1.25x"); empty until known.
	Speed string
	// Percent is OutTime/Duration clamped to [0,1]; 0 when duration unknown.
	Percent float64
	// Frame is the encoded frame count.
	Frame int64
}

// Options for Run.
type Options struct {
	Args     []string
	Duration time.Duration
	// OnProgress, when set, receives periodic progress updates.
	OnProgress func(Stats)
}

// Run executes ffmpeg with the given arguments. stdout is expected to carry
// the `-progress pipe:1` protocol; stderr's last lines are captured for error
// reporting.
func Run(ctx context.Context, opts Options) error {
	cmd := exec.CommandContext(ctx, FFmpegPath, opts.Args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start ffmpeg: %w", err)
	}

	errTail := &tail{max: 40}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		parseProgress(stdout, opts)
	}()
	go func() {
		defer wg.Done()
		errTail.readFrom(stderr)
	}()
	wg.Wait()

	runErr := cmd.Wait()
	if ctx.Err() != nil {
		return fmt.Errorf("ffmpeg canceled: %w", ctx.Err())
	}
	if runErr != nil {
		msg := strings.Join(errTail.lines(), "\n")
		if msg == "" {
			return fmt.Errorf("ffmpeg failed: %w", runErr)
		}
		return fmt.Errorf("ffmpeg failed: %s", msg)
	}
	return nil
}

func parseProgress(r io.Reader, opts Options) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var st Stats
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "out_time_us":
			if us, err := strconv.ParseInt(val, 10, 64); err == nil && us >= 0 {
				st.OutTime = time.Duration(us) * time.Microsecond
				st.recompute(opts.Duration)
			}
		case "out_time_ms":
			// Historical quirk: out_time_ms is actually microseconds.
			if st.OutTime == 0 {
				if us, err := strconv.ParseInt(val, 10, 64); err == nil && us >= 0 {
					st.OutTime = time.Duration(us) * time.Microsecond
					st.recompute(opts.Duration)
				}
			}
		case "speed":
			st.Speed = strings.TrimSuffix(val, "x") + "x"
		case "frame":
			if f, err := strconv.ParseInt(val, 10, 64); err == nil {
				st.Frame = f
			}
		case "progress":
			if opts.OnProgress != nil && (val == "continue" || val == "end") {
				s := st
				opts.OnProgress(s)
			}
		}
	}
}

func (s *Stats) recompute(total time.Duration) {
	if total > 0 {
		s.Percent = float64(s.OutTime) / float64(total)
		if s.Percent > 1 {
			s.Percent = 1
		}
		if s.Percent < 0 {
			s.Percent = 0
		}
	}
}

// tail keeps the last max lines written to it.
type tail struct {
	mu  sync.Mutex
	buf []string
	max int
}

func (t *tail) readFrom(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		t.mu.Lock()
		t.buf = append(t.buf, line)
		if len(t.buf) > t.max {
			t.buf = t.buf[len(t.buf)-t.max:]
		}
		t.mu.Unlock()
	}
}

func (t *tail) lines() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.buf...)
}

// Exists reports whether the ffmpeg binary is available in PATH.
func Exists() bool {
	_, err := exec.LookPath(FFmpegPath)
	return err == nil
}

// Verify quickly checks that ffmpeg and ffprobe exist.
func Verify(ffprobePath string) error {
	if !Exists() {
		return fmt.Errorf("ffmpeg not found in PATH")
	}
	if _, err := exec.LookPath(ffprobePath); err != nil {
		return fmt.Errorf("ffprobe not found in PATH")
	}
	return nil
}

// Discard is a no-op progress sink.
var Discard = func(Stats) {}

