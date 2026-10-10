// Command ffconvert-web is the web frontend of ff-convert: a chi JSON API
// plus an embedded Vue SPA, with WebSocket progress streaming.
//
// Environment:
//
//	FF_CONVERT_LIBRARY_DIR   root folder browsable/convertible via the UI (required)
//	FF_CONVERT_PORT          listen port (default 8080)
//	FF_CONVERT_CONFIG        config file override
//	FF_CONVERT_CONCURRENCY   worker pool size (default: CPU count / 2)
package main

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/isden/ff-convert/internal/config"
	"github.com/isden/ff-convert/internal/ffmpegx"
	"github.com/isden/ff-convert/internal/jobs"
	"github.com/isden/ff-convert/internal/plan"
	"github.com/isden/ff-convert/internal/pool"
	"github.com/isden/ff-convert/internal/probe"
	"github.com/isden/ff-convert/internal/scan"
	webui "github.com/isden/ff-convert/web"
)

func main() {
	logger := log.New(os.Stderr)

	libDir := os.Getenv("FF_CONVERT_LIBRARY_DIR")
	if libDir == "" {
		logger.Fatal("FF_CONVERT_LIBRARY_DIR is not set")
	}
	abs, err := filepath.Abs(libDir)
	if err != nil {
		logger.Fatal("invalid library dir", "err", err)
	}
	if _, err := os.Stat(abs); err != nil {
		logger.Fatal("library dir not accessible", "err", err)
	}
	port := envOr("FF_CONVERT_PORT", "8080")

	if err := ffmpegx.Verify(probe.FFprobePath); err != nil {
		logger.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("config", err)
	}
	p := pool.NewDefault()
	logger.Info("starting ff-convert web",
		"library", abs, "port", port, "pool", p.Limit(),
		"config", map[string]any{"exists": cfg.Exists, "path": cfg.Path})

	mgr := jobs.NewManager(cfg, p)
	srv := &server{mgr: mgr, lib: abs, log: logger}

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.NoCache)

	r.Get("/api/library", srv.handleLibrary)
	r.Post("/api/jobs", srv.handleCreateJob)
	r.Get("/api/jobs", srv.handleJobs)
	r.Get("/api/jobs/{id}", srv.handleJobGet)
	r.Post("/api/jobs/{id}/answers", srv.handleAnswers)
	r.Post("/api/jobs/{id}/cancel", srv.handleCancel)
	r.Delete("/api/jobs/{id}", srv.handleRemove)
	r.Get("/api/ws", srv.handleWS)

	spa, err := fs.Sub(webui.Dist(), "dist")
	if err != nil {
		logger.Fatal("embedded spa", err)
	}
	r.Handle("/*", spaHandler(spa))

	httpSrv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}
	logger.Info("listening", "addr", "http://localhost:"+port)
	if err := httpSrv.ListenAndServe(); err != nil {
		logger.Fatal("server", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

type server struct {
	mgr *jobs.Manager
	lib string
	log *log.Logger
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// resolveLibPath resolves a UI-supplied relative path inside the library,
// rejecting escapes.
func (s *server) resolveLibPath(rel string) (string, error) {
	clean := filepath.Clean("/" + rel) // forces the path to be absolute-relative
	p := filepath.Join(s.lib, clean)
	r, err := filepath.Rel(s.lib, p)
	if err != nil || strings.HasPrefix(r, "..") {
		return "", os.ErrPermission
	}
	return p, nil
}

type libraryEntry struct {
	Name string `json:"name"`
	Path string `json:"path"` // library-relative path, for the API
	Dir  bool   `json:"dir"`
}

func (s *server) handleLibrary(w http.ResponseWriter, r *http.Request) {
	sub := r.URL.Query().Get("path")
	dir, err := s.resolveLibPath(sub)
	if err != nil {
		writeErr(w, http.StatusForbidden, err)
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	// The converted output folder is an implementation detail, hide it.
	outDir := filepath.Base(scan.OutputDir())
	var out []libraryEntry
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || name == outDir {
			continue
		}
		isDir := e.IsDir()
		if !isDir && !scan.IsVideoFile(name) {
			continue
		}
		rel := name
		if sub != "" {
			rel = strings.TrimPrefix(filepath.Join(sub, name), "/")
		}
		out = append(out, libraryEntry{Name: name, Path: rel, Dir: isDir})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Dir != out[j].Dir {
			return out[i].Dir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	writeJSON(w, http.StatusOK, out)
}

func (s *server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	p, err := s.resolveLibPath(body.Path)
	if err != nil {
		writeErr(w, http.StatusForbidden, err)
		return
	}
	v, err := s.mgr.Create(p)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusAccepted, v)
}

func (s *server) handleJobs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.mgr.Views())
}

func (s *server) handleJobGet(w http.ResponseWriter, r *http.Request) {
	v, err := s.mgr.View(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *server) handleAnswers(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Answers []plan.Answers `json:"answers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.mgr.SubmitAnswers(id, body.Answers); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	v, err := s.mgr.View(id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *server) handleCancel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.mgr.Cancel(id); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	v, err := s.mgr.View(id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *server) handleRemove(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.mgr.Remove(id); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"removed": true})
}

// handleWS streams full job snapshots: immediately on connect, then on any
// manager change and at least every 500ms while jobs are in flight.
func (s *server) handleWS(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"}, // LAN tool, no auth by design
	})
	if err != nil {
		return
	}
	defer c.CloseNow()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	notify := make(chan struct{}, 1)
	unsub := s.mgr.Subscribe(func() {
		select {
		case notify <- struct{}{}:
		default:
		}
	})
	defer unsub()

	c.CloseRead(ctx) // client→server messages are ignored

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	send := func() bool {
		data, err := json.Marshal(s.mgr.Views())
		if err != nil {
			return false
		}
		wctx, wcancel := context.WithTimeout(ctx, 5*time.Second)
		defer wcancel()
		return c.Write(wctx, websocket.MessageText, data) == nil
	}

	if !send() {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-notify:
			if !send() {
				return
			}
		case <-ticker.C:
			if !send() {
				return
			}
		}
	}
}

func spaHandler(spa fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(spa))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(spa, path); err != nil {
			// SPA fallback: serve index.html for client-side routes.
			r2 := new(http.Request)
			*r2 = *r
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
