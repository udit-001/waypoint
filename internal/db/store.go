package db

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
	GetSettings() (Settings, error)
	UpsertSettings(updates map[string]any) error

	// Lifecycle
	RunMigrations(dbPath string) error
	Close() error
}
