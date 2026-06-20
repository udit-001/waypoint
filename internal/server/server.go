package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"

	"github.com/SwatiBio/waypoint/internal/db"
	"github.com/SwatiBio/waypoint/web"
)

// Config holds the server configuration.
type Config struct {
	Port   int
	DB     *db.Store
	NoOpen bool // don't auto-open browser
	Silent bool // suppress terminal output (daemon mode)
}

// Start runs the HTTP server with the read-only API and embedded web UI.
func Start(cfg Config) error {
	mux := http.NewServeMux()

	// Read-only API
	mux.HandleFunc("GET /api/jobs", handleListJobs(cfg.DB))
	mux.HandleFunc("GET /api/jobs/{id}", handleGetJob(cfg.DB))
	mux.HandleFunc("GET /api/jobs/{id}/history", handleGetJobHistory(cfg.DB))
	mux.HandleFunc("GET /api/stats", handleStats(cfg.DB))
	mux.HandleFunc("GET /api/history", handleGetAllHistory(cfg.DB))
	mux.HandleFunc("GET /api/categories", handleCategories(cfg.DB))

	// Profile & Settings
	mux.HandleFunc("GET /api/profile", handleGetProfile(cfg.DB))
	mux.HandleFunc("GET /api/settings", handleGetSettings(cfg.DB))

	// Embedded static UI
	staticFS, err := fs.Sub(web.Files, ".")
	if err != nil {
		return fmt.Errorf("static subfs: %w", err)
	}
	mux.Handle("GET /", spaHandler(staticFS))

	addr := fmt.Sprintf("127.0.0.1:%d", cfg.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
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
		_ = os.Remove(pidFilePath())
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

func pidFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "server.pid"
	}
	return filepath.Join(home, ".job-tracker", "server.pid")
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

func handleListJobs(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("status")
		category := r.URL.Query().Get("category")
		search := r.URL.Query().Get("search")

		var jobs []db.Job
		var err error

		switch {
		case search != "":
			jobs, err = store.SearchJobs(search)
		case status != "" || category != "":
			jobs, err = store.FilterJobs(status, category)
		default:
			jobs, err = store.GetJobs()
		}

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

func handleGetJob(store *db.Store) http.HandlerFunc {
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

func handleGetJobHistory(store *db.Store) http.HandlerFunc {
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

func handleStats(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jobs, err := store.GetJobs()
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		byStatus := make(map[string]int)
		byCategory := make(map[string]int)
		for _, j := range jobs {
			byStatus[j.Status]++
			if j.CategoryName != "" {
				byCategory[j.CategoryName]++
			}
		}

		jsonResponse(w, map[string]any{
			"total":      len(jobs),
			"byStatus":   byStatus,
			"byCategory": byCategory,
		})
	}
}

func handleGetAllHistory(store *db.Store) http.HandlerFunc {
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

func handleGetProfile(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		profile, err := store.GetProfile()
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResponse(w, profile)
	}
}

func handleGetSettings(store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := store.GetSettings()
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResponse(w, s)
	}
}

func handleCategories(store *db.Store) http.HandlerFunc {
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
