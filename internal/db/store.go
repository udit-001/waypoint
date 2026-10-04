package db

import (
	"github.com/udit-001/waypoint/internal/scraper"
)

// Store is the persistence interface. The concrete implementation
// (SQLiteStore) wraps sqlx + SQLite. Tests use FakeStore (in-memory maps).
// Define tests against this interface, not the concrete type.
type Store interface {
	// Jobs
	GetJobs() ([]Job, error)
	GetJob(id int64) (Job, error)
	InsertJob(j Job) (Job, error)
	UpdateJobFields(id int64, updates map[string]any) error
	DeleteJob(id int64) error
	SearchJobs(query string, status, category string) ([]Job, error)
	FilterJobs(status, category string) ([]Job, error)
	JobCount() (int, error)
	JobExists(url string) (bool, error)

	// History
	AddHistory(jobID int64, action, from, to string) error
	GetJobHistory(jobID int64) ([]HistoryEntry, error)
	GetAllHistory() ([]HistoryEntry, error)

	// Categories
	GetCategories() ([]Category, error)
	GetCategoriesWithCounts() ([]CategoryWithCount, error)
	GetCategoryByID(id int64) (Category, error)
	AddCategory(name string) (Category, error)
	DeleteCategory(id int64) error
	RenameCategory(id int64, newName string) error
	HasCategory(name string) (bool, error)
	CategoryIDByName(name string) (int64, error)
	CategoryJobCount(id int64) (int, error)

	// Stats
	GetStats() (Stats, error)

	// Artifacts
	GetArtifacts(skillID string, jobID int64, includeArchived bool) ([]Artifact, error)
	GetArtifact(id int64) (Artifact, error)
	AddArtifact(a Artifact) (Artifact, error)
	UpdateArtifact(id int64, updates map[string]any) (Artifact, error)
	ArchiveArtifact(id int64) error
	DeleteArtifact(id int64) error
	SearchArtifacts(query string) ([]Artifact, error)
	SearchAll(query string) ([]SearchResultItem, error)

	// Profile & Settings
	GetProfile() (Profile, error)
	UpsertProfile(updates map[string]any) error
	GetBrief() (Brief, error)
	GetSettings() (Settings, error)
	UpsertSettings(updates map[string]any) error

	// Lifecycle
	RunMigrations(dbPath string) error
	Close() error

	// Postings ledger — the world's scraped data, one row per URL,
	// awaiting review (promote or dismiss).
	HasPosting(url string) (bool, error)
	AddPostings(results []scraper.Result) error
	ListPostings(status string) ([]Posting, error)
	CountPostings(status string) (int, error)
	GetPosting(url string) (Posting, bool, error)
	SetPostingStatus(url, status string) error
	PrunePostings(days int) (int, error)
	EnrichPosting(url, desc string, meta map[string]string) error
	MigratePostings(entries []Posting) (int, error)

	// Promote — moves a posting into the applications (jobs) table.
	Promote(url string) (Job, error)

	// Live-sync — coarse mutation notifications consumed by SSE. The
	// table (not an in-memory broker) is the bus: writers may live in
	// another process (CLI autopilot run) than the server.
	AddChangeEvent(kind string) error
	ChangesSince(cursor int64, limit int) ([]ChangeEvent, error)

	// Autopilot run log. One record per cycle: the verdict plus the
	// per-source and per-stage evidence stored under it.
	AddRunLog(entry RunLog) (int64, error)
	UpdateRunLog(id int64, entry RunLog) error
	GetLastRun() (RunLog, bool, error)
	ListRunLogs(limit int) ([]RunLog, error)

	// Company discovery candidates — the discovery ledger (WP-150).
	SaveCandidates(cands []CompanyCandidate) error
	Candidates(status string) ([]CompanyCandidate, error)
	SetCandidateStatus(id int64, status string) error

	// Board sweep state + per-company new counts (WP-149) — the raw
	// material of the Companies page's trust strip.
	SetBoardSweepState(board string, st BoardSweepState) error
	GetBoardSweepStates() (map[string]BoardSweepState, error)
	NewPostingCounts() (map[string]int, error)

	// Facet cache (WP-152) — the expanded facet list keyed by brief
	// hash, so repeat discovery runs skip the LLM expansion.
	SaveDiscoveryFacets(hash string, facets []string) error
	DiscoveryFacets(hash string) ([]string, bool, error)

	// Last-discovery trigger state (WP-153).
	// Autopilot scheduling: one-shot "run now" requests.
	RequestAutopilotRun() error
	ConsumeAutopilotRunRequest() (bool, error)
	SaveDiscoveryLastRun(briefHash, atRFC3339 string) error
	DiscoveryLastRun() (hash, at string, has bool, err error)
}
