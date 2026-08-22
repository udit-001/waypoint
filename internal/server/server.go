package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/udit-001/waypoint/internal/config"
	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/linkedin"
	"github.com/udit-001/waypoint/web"
)

// Config holds the server configuration.
type Config struct {
	Port      int
	DB        db.Store
	NoOpen    bool // don't auto-open browser
	Silent    bool // suppress terminal output (daemon mode)
	Autopilot bool // start autopilot ticker (daemon mode only)

	// LoadBoards reads boards.toml for GET /api/companies (WP-149).
	// Injected by the CLI (ADR 0001: CLI writes, web reads); nil means
	// no boards file — the endpoint serves an empty list.
	LoadBoards func() ([]config.BoardEntry, error)

	// WithBoards is the write side of the same seam (WP-154): run fn
	// against the loaded boards file and persist it when fn succeeds.
	// Backs POST /api/candidates/{id}/add. Nil = read-only web UI.
	WithBoards func(fn func(*config.BoardsFile) error) error
}

// newMux creates the HTTP mux with API routes, PWA routes, and static file
// serving. Extracted from Start for testability — tests can create a mux and
// make requests against it via httptest without binding to a port.
// newMux builds the mux with the default LinkedIn fetcher (hosted Exa MCP).
// Tests that need to stub the fetch use newMuxWithLinkedIn.
func newMux(store db.Store, staticFS fs.FS) http.Handler {
	return newMuxWithBoards(store, staticFS, linkedin.New(), nil, nil)
}

// newMuxWithLinkedIn builds the mux with an injected LinkedIn fetcher and
// no boards source.
func newMuxWithLinkedIn(store db.Store, staticFS fs.FS, li *linkedin.Fetcher) http.Handler {
	return newMuxWithBoards(store, staticFS, li, nil, nil)
}

// newMuxWithBoards builds the mux with both seams injected: the LinkedIn
// fetcher and the boards.toml loader backing GET /api/companies. A nil
// loadBoards means "no boards file" — the endpoint serves an empty list.
func newMuxWithBoards(store db.Store, staticFS fs.FS, li *linkedin.Fetcher, loadBoards func() ([]config.BoardEntry, error), withBoards func(func(*config.BoardsFile) error) error) http.Handler {
	mux := http.NewServeMux()

	// Read-only API
	mux.HandleFunc("GET /api/jobs", handleListJobs(store))
	mux.HandleFunc("GET /api/jobs/{id}", handleGetJob(store))
	mux.HandleFunc("GET /api/jobs/{id}/history", handleGetJobHistory(store))
	mux.HandleFunc("GET /api/stats", handleStats(store))
	mux.HandleFunc("GET /api/history", handleGetAllHistory(store))
	mux.HandleFunc("GET /api/categories", handleCategories(store))
	mux.HandleFunc("GET /api/artifacts", handleListArtifacts(store))
	mux.HandleFunc("GET /api/artifacts/{id}", handleGetArtifact(store))
	mux.HandleFunc("GET /api/search", handleSearch(store))

	// Profile & Settings
	mux.HandleFunc("GET /api/profile", handleGetProfile(store))
	mux.HandleFunc("GET /api/brief", handleGetBrief(store))
	mux.HandleFunc("PATCH /api/profile", handleUpdateProfile(store))
	mux.HandleFunc("POST /api/profile/import-linkedin", handleImportLinkedIn(store, li))
	mux.HandleFunc("GET /api/settings", handleGetSettings(store))
	mux.HandleFunc("PATCH /api/settings", handleUpdateSettings(store))

	mux.HandleFunc("GET /api/candidates", handleListCandidates(store))
	mux.HandleFunc("POST /api/candidates/{id}/add", handleAddCandidate(store, withBoards))
	mux.HandleFunc("POST /api/candidates/{id}/dismiss", handleDismissCandidate(store))

	// Companies — the boards.toml monitoring surface (WP-149).
	mux.HandleFunc("GET /api/companies", handleListCompanies(store, loadBoards))

	// Postings review queue
	mux.HandleFunc("GET /api/postings", handleListPostings(store))
	mux.HandleFunc("POST /api/postings/{url}/promote", handlePromotePosting(store))
	mux.HandleFunc("POST /api/postings/{url}/dismiss", handleDismissPosting(store))

	// Autopilot
	mux.HandleFunc("GET /api/autopilot", handleGetAutopilot(store))

	// Live-sync (SSE — tails the change_events table)
	mux.HandleFunc("GET /api/events", handleEvents(store))

	// PWA routes with proper cache headers.
	// sw.js must always revalidate (no-cache) or updates won't propagate.
	// Service-Worker-Allowed lets the SW control root scope.
	mux.HandleFunc("GET /sw.js", servePWAAsset(staticFS, "sw.js", "application/javascript", "no-cache", "/"))
	// Manifest can be cached for an hour.
	mux.HandleFunc("GET /manifest.json", servePWAAsset(staticFS, "manifest.json", "application/manifest+json", "public, max-age=3600", ""))
	// Offline page should always revalidate too.
	mux.HandleFunc("GET /offline.html", servePWAAsset(staticFS, "offline.html", "text/html; charset=utf-8", "no-cache", ""))

	mux.Handle("GET /", spaHandler(staticFS))

	return mux
}

// Start runs the HTTP server with the read-only API and embedded web UI.
func Start(cfg Config) error {
	staticFS, err := fs.Sub(web.Files, "dist")
	if err != nil {
		return fmt.Errorf("static subfs: %w", err)
	}

	mux := newMuxWithBoards(cfg.DB, staticFS, linkedin.New(), cfg.LoadBoards, cfg.WithBoards)

	addr := fmt.Sprintf("127.0.0.1:%d", cfg.Port)
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		// 60s (not 30s): the LinkedIn import route fetches through Exa MCP,
		// which can take ~10–50s for a profile page. WriteTimeout covers the
		// whole handler, so it must exceed the import timeout.
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Auto-open browser
	if !cfg.NoOpen && !cfg.Silent {
		url := fmt.Sprintf("http://%s", addr)
		if err := openBrowser(url); err != nil {
			log.Printf("  Open %s in your browser", url)
		}
	}

	if cfg.Silent {
		log.Printf("Waypoint server listening on http://127.0.0.1:%d", cfg.Port)
	} else {
		fmt.Printf("  Waypoint UI: http://127.0.0.1:%d\n", cfg.Port)
		fmt.Println("  Press Ctrl+C to stop")
		fmt.Println()
	}

	// Handle shutdown signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		server.Close()
	}()

	return server.ListenAndServe()
}

// spaHandler serves static files with SPA fallback to index.html.
func spaHandler(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "" || path == "/" {
			// Root: serve index.html
			fileServer.ServeHTTP(w, r)
			return
		}

		// Try to open the requested file
		cleanPath := path[1:] // strip leading /
		f, err := fsys.Open(cleanPath)
		if err != nil {
			// File doesn't exist → serve index.html (SPA fallback)
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		f.Close()

		// Check if it's a directory
		info, _ := fs.Stat(fsys, cleanPath)
		if info != nil && info.IsDir() {
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

// servePWAAsset returns a handler that serves a file from the embedded FS
// with custom Content-Type, Cache-Control, and optional Service-Worker-Allowed
// headers. This is used for PWA-critical files (sw.js, manifest, offline page)
// where the default FileServer headers are insufficient.
func servePWAAsset(fsys fs.FS, name, contentType, cacheControl, swScope string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", cacheControl)
		if swScope != "" {
			w.Header().Set("Service-Worker-Allowed", swScope)
		}
		w.Write(data)
	}
}

// openBrowser opens the default browser to the given URL.
func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default: // linux, freebsd, etc.
		cmd = "xdg-open"
		args = []string{url}
	}

	return exec.Command(cmd, args...).Start()
}

// --- JSON helpers ---

func jsonResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// --- API Handlers ---

func handleListJobs(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("status")
		category := r.URL.Query().Get("category")
		search := r.URL.Query().Get("search")

		jobs, err := db.ListJobs(store, db.ListOpts{
			Search:   search,
			Status:   status,
			Category: category,
		})

		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if jobs == nil {
			jobs = []db.Job{}
		}
		jsonResponse(w, jobs)
	}
}

func handleGetJob(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			jsonError(w, "invalid job id", http.StatusBadRequest)
			return
		}

		job, err := store.GetJob(id)
		if err != nil {
			jsonError(w, "job not found", http.StatusNotFound)
			return
		}
		jsonResponse(w, job)
	}
}

func handleGetJobHistory(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			jsonError(w, "invalid job id", http.StatusBadRequest)
			return
		}

		history, err := store.GetJobHistory(id)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if history == nil {
			history = []db.HistoryEntry{}
		}
		jsonResponse(w, history)
	}
}

func handleStats(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := store.GetStats()
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResponse(w, stats)
	}
}

func handleGetAllHistory(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		history, err := store.GetAllHistory()
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if history == nil {
			history = []db.HistoryEntry{}
		}
		jsonResponse(w, history)
	}
}

func handleGetProfile(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		profile, err := store.GetProfile()
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResponse(w, profile)
	}
}

func handleGetSettings(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := store.GetSettings()
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResponse(w, s)
	}
}

func handleCategories(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cats, err := store.GetCategories()
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if cats == nil {
			cats = []db.Category{}
		}
		jsonResponse(w, cats)
	}
}

func handleListArtifacts(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		search := r.URL.Query().Get("search")
		skill := r.URL.Query().Get("skill")
		jobStr := r.URL.Query().Get("job")
		all := r.URL.Query().Get("all") == "true"

		if search != "" {
			arts, err := store.SearchArtifacts(search)
			if err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if arts == nil {
				arts = []db.Artifact{}
			}
			jsonResponse(w, arts)
			return
		}

		var jobID int64
		if jobStr != "" {
			jobID, _ = strconv.ParseInt(jobStr, 10, 64)
		}

		arts, err := store.GetArtifacts(skill, jobID, all)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if arts == nil {
			arts = []db.Artifact{}
		}
		jsonResponse(w, arts)
	}
}

func handleGetArtifact(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			jsonError(w, "invalid artifact id", http.StatusBadRequest)
			return
		}

		art, err := store.GetArtifact(id)
		if err != nil {
			jsonError(w, "artifact not found", http.StatusNotFound)
			return
		}
		jsonResponse(w, art)
	}
}

func handleSearch(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q == "" {
			jsonError(w, "missing query parameter 'q'", http.StatusBadRequest)
			return
		}

		results, err := store.SearchAll(q)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if results == nil {
			results = []db.SearchResultItem{}
		}
		jsonResponse(w, results)
	}
}

// --- Postings review queue ---

func handleListPostings(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("status")

		postings, err := store.ListPostings(status)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if postings == nil {
			postings = []db.Posting{}
		}
		jsonResponse(w, postings)
	}
}

func handlePromotePosting(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rawURL, err := url.PathUnescape(r.PathValue("url"))
		if err != nil || rawURL == "" {
			jsonError(w, "invalid posting URL", http.StatusBadRequest)
			return
		}

		job, err := store.Promote(rawURL)
		if err != nil {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		jsonResponse(w, job)
	}
}

func handleDismissPosting(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rawURL, err := url.PathUnescape(r.PathValue("url"))
		if err != nil || rawURL == "" {
			jsonError(w, "invalid posting URL", http.StatusBadRequest)
			return
		}

		if err := store.SetPostingStatus(rawURL, db.StatusDismissed); err != nil {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		jsonResponse(w, map[string]string{"status": "dismissed", "url": rawURL})
	}
}

// --- Autopilot ---

func handleGetAutopilot(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		settings, err := store.GetSettings()
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		lastRun, hasRun, _ := store.GetLastRun()

		cadence := settings.AutopilotCadence
		if cadence <= 0 {
			cadence = 6
		}

		resp := map[string]any{
			"enabled":   settings.AutopilotEnabled == 1,
			"cadence":   cadence,
			"zenKeySet": settings.ZenAPIKey != "",
			"lastRun":   nil,
		}
		if hasRun {
			resp["lastRun"] = lastRun
		}

		jsonResponse(w, resp)
	}
}
