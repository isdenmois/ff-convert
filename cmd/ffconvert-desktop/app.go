package main

import (
	"context"
	"log"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/isden/ff-convert/internal/config"
	"github.com/isden/ff-convert/internal/ffmpegx"
	"github.com/isden/ff-convert/internal/jobs"
	"github.com/isden/ff-convert/internal/plan"
	"github.com/isden/ff-convert/internal/pool"
	"github.com/isden/ff-convert/internal/probe"
)

// App is the Wails binding target; its exported methods are called from the
// frontend.
type App struct {
	ctx context.Context
	mgr *jobs.Manager
}

// NewApp builds the app with the shared conversion core.
func NewApp() *App {
	if err := ffmpegx.Verify(probe.FFprobePath); err != nil {
		log.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("config: ", err)
	}
	return &App{mgr: jobs.NewManager(cfg, pool.NewDefault())}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	go func() {
		ticker := time.NewTicker(400 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.emit()
			}
		}
	}()
	// Push state changes immediately on top of the periodic tick.
	a.mgr.Subscribe(func() { a.emit() })
}

func (a *App) emit() {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "jobs", a.mgr.Views())
}

// PickFile opens the native file dialog and returns the chosen path ("" when
// canceled).
func (a *App) PickFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Choose a movie file",
	})
}

// PickFolder opens the native folder dialog and returns the chosen path (""
// when canceled).
func (a *App) PickFolder() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Choose a folder with movie files",
	})
}

// AddJob registers a new conversion job for the picked path.
func (a *App) AddJob(path string) (jobs.JobView, error) {
	if path == "" {
		return jobs.JobView{}, nil
	}
	return a.mgr.Create(path)
}

// SubmitAnswers stores the user's per-group answers and enqueues the job.
func (a *App) SubmitAnswers(id string, answers []plan.Answers) error {
	return a.mgr.SubmitAnswers(id, answers)
}

// CancelJob cancels a pending or running job.
func (a *App) CancelJob(id string) error { return a.mgr.Cancel(id) }

// RemoveJob removes a job from the queue (canceling it first if needed).
func (a *App) RemoveJob(id string) error { return a.mgr.Remove(id) }

// Snapshot returns all job views for the initial UI state.
func (a *App) Snapshot() []jobs.JobView { return a.mgr.Views() }
