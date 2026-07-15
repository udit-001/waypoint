package db

import (
	"fmt"
	"strings"
	"time"
)

// FakeStore is an in-memory implementation of Store for tests.
// No SQLite, no files — just maps. Use NewFakeStore() to construct.
//
// Compile-time check: FakeStore must satisfy Store.
var _ Store = (*FakeStore)(nil)

type FakeStore struct {
	Jobs       map[int64]Job
	Categories map[int64]Category
	Artifacts  map[int64]Artifact
	History    []HistoryEntry
	Profile    Profile
	Settings   Settings

	nextJobID  int64
	nextCatID  int64
	nextArtID  int64
	nextHistID int64
}

func NewFakeStore() *FakeStore {
	return &FakeStore{
		Jobs:       make(map[int64]Job),
		Categories: make(map[int64]Category),
		Artifacts:  make(map[int64]Artifact),
		History:    []HistoryEntry{},
		Settings:   defaultSettings,
	}
}

func (f *FakeStore) categoryName(id int64) string {
	if cat, ok := f.Categories[id]; ok {
		return cat.Name
	}
	return ""
}

// --- Jobs ---

func (f *FakeStore) GetJobs() ([]Job, error) {
	var jobs []Job
	for _, j := range f.Jobs {
		j.CategoryName = f.categoryName(j.CategoryID)
		jobs = append(jobs, j)
	}
	// Sort by ID descending
	for i := 0; i < len(jobs); i++ {
		for k := i + 1; k < len(jobs); k++ {
			if jobs[i].ID < jobs[k].ID {
				jobs[i], jobs[k] = jobs[k], jobs[i]
			}
		}
	}
	return jobs, nil
}

func (f *FakeStore) GetJob(id int64) (Job, error) {
	j, ok := f.Jobs[id]
	if !ok {
		return Job{}, fmt.Errorf("job %d not found", id)
	}
	j.CategoryName = f.categoryName(j.CategoryID)
	return j, nil
}

func (f *FakeStore) InsertJob(j Job) (Job, error) {
	f.nextJobID++
	j.ID = f.nextJobID
	f.Jobs[j.ID] = j
	return j, nil
}

func (f *FakeStore) UpdateJobFields(id int64, updates map[string]any) error {
	j, ok := f.Jobs[id]
	if !ok {
		return fmt.Errorf("job %d not found", id)
	}
	if v, ok := updates["company"]; ok {
		j.Company = fmt.Sprint(v)
	}
	if v, ok := updates["position"]; ok {
		j.Position = fmt.Sprint(v)
	}
	if v, ok := updates["status"]; ok {
		j.Status = fmt.Sprint(v)
	}
	if v, ok := updates["category_id"]; ok {
		if id, ok := v.(int64); ok {
			j.CategoryID = id
		}
	}
	if v, ok := updates["salary"]; ok {
		j.Salary = fmt.Sprint(v)
	}
	if v, ok := updates["location"]; ok {
		j.Location = fmt.Sprint(v)
	}
	if v, ok := updates["contact"]; ok {
		j.Contact = fmt.Sprint(v)
	}
	if v, ok := updates["url"]; ok {
		j.URL = fmt.Sprint(v)
	}
	if v, ok := updates["date"]; ok {
		j.Date = fmt.Sprint(v)
	}
	if v, ok := updates["applied_date"]; ok {
		j.AppliedDate = fmt.Sprint(v)
	}
	if v, ok := updates["notes"]; ok {
		j.Notes = fmt.Sprint(v)
	}
	if v, ok := updates["reminder_date"]; ok {
		s := fmt.Sprint(v)
		if s == "" {
			j.ReminderDate = nil
		} else {
			j.ReminderDate = &s
		}
	}
	if v, ok := updates["updated_at"]; ok {
		j.UpdatedAt = fmt.Sprint(v)
	}
	f.Jobs[id] = j
	return nil
}

func (f *FakeStore) DeleteJob(id int64) error {
	if _, ok := f.Jobs[id]; !ok {
		return fmt.Errorf("job %d not found", id)
	}
	delete(f.Jobs, id)
	var filtered []HistoryEntry
	for _, h := range f.History {
		if h.JobID != id {
			filtered = append(filtered, h)
		}
	}
	f.History = filtered
	return nil
}

func (f *FakeStore) SearchJobs(query string, status, category string) ([]Job, error) {
	q := strings.ToLower(query)
	var jobs []Job
	for _, j := range f.Jobs {
		if !strings.Contains(strings.ToLower(j.Company), q) &&
			!strings.Contains(strings.ToLower(j.Position), q) &&
			!strings.Contains(strings.ToLower(j.Notes), q) &&
			!strings.Contains(strings.ToLower(j.Location), q) &&
			!strings.Contains(strings.ToLower(j.Contact), q) {
			continue
		}
		if status != "" && j.Status != status {
			continue
		}
		if category != "" && f.categoryName(j.CategoryID) != category {
			continue
		}
		j.CategoryName = f.categoryName(j.CategoryID)
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (f *FakeStore) FilterJobs(status, category string) ([]Job, error) {
	var jobs []Job
	for _, j := range f.Jobs {
		if status != "" && j.Status != status {
			continue
		}
		if category != "" && f.categoryName(j.CategoryID) != category {
			continue
		}
		j.CategoryName = f.categoryName(j.CategoryID)
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (f *FakeStore) JobCount() (int, error) {
	return len(f.Jobs), nil
}

func (f *FakeStore) JobExists(url string) (bool, error) {
	for _, j := range f.Jobs {
		if j.URL == url {
			return true, nil
		}
	}
	return false, nil
}

// --- History ---

func (f *FakeStore) AddHistory(jobID int64, action, from, to string) error {
	f.nextHistID++
	f.History = append(f.History, HistoryEntry{
		ID:        f.nextHistID,
		JobID:     jobID,
		Action:    action,
		From:      from,
		To:        to,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
	return nil
}

func (f *FakeStore) GetJobHistory(jobID int64) ([]HistoryEntry, error) {
	var result []HistoryEntry
	for i := len(f.History) - 1; i >= 0; i-- {
		if f.History[i].JobID == jobID {
			result = append(result, f.History[i])
		}
	}
	return result, nil
}

func (f *FakeStore) GetAllHistory() ([]HistoryEntry, error) {
	result := make([]HistoryEntry, len(f.History))
	for i, h := range f.History {
		result[len(f.History)-1-i] = h
	}
	return result, nil
}

// --- Categories ---

func (f *FakeStore) GetCategories() ([]Category, error) {
	var cats []Category
	for _, c := range f.Categories {
		cats = append(cats, c)
	}
	return cats, nil
}

func (f *FakeStore) GetCategoriesWithCounts() ([]CategoryWithCount, error) {
	counts := make(map[int64]int)
	for _, j := range f.Jobs {
		if j.CategoryID != 0 {
			counts[j.CategoryID]++
		}
	}
	var result []CategoryWithCount
	for _, c := range f.Categories {
		result = append(result, CategoryWithCount{
			ID:       c.ID,
			Name:     c.Name,
			JobCount: counts[c.ID],
		})
	}
	return result, nil
}

func (f *FakeStore) GetCategoryByID(id int64) (Category, error) {
	c, ok := f.Categories[id]
	if !ok {
		return Category{}, fmt.Errorf("category %d not found", id)
	}
	return c, nil
}

func (f *FakeStore) AddCategory(name string) (Category, error) {
	f.nextCatID++
	c := Category{ID: f.nextCatID, Name: name}
	f.Categories[c.ID] = c
	return c, nil
}

func (f *FakeStore) DeleteCategory(id int64) error {
	if _, ok := f.Categories[id]; !ok {
		return fmt.Errorf("category id %d not found", id)
	}
	for jid, j := range f.Jobs {
		if j.CategoryID == id {
			j.CategoryID = 0
			f.Jobs[jid] = j
		}
	}
	delete(f.Categories, id)
	return nil
}

func (f *FakeStore) RenameCategory(id int64, newName string) error {
	c, ok := f.Categories[id]
	if !ok {
		return fmt.Errorf("category id %d not found", id)
	}
	c.Name = newName
	f.Categories[id] = c
	return nil
}

func (f *FakeStore) HasCategory(name string) (bool, error) {
	for _, c := range f.Categories {
		if c.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (f *FakeStore) CategoryIDByName(name string) (int64, error) {
	for _, c := range f.Categories {
		if c.Name == name {
			return c.ID, nil
		}
	}
	return 0, nil
}

func (f *FakeStore) CategoryJobCount(id int64) (int, error) {
	count := 0
	for _, j := range f.Jobs {
		if j.CategoryID == id {
			count++
		}
	}
	return count, nil
}

// --- Stats ---

func (f *FakeStore) GetStats() (Stats, error) {
	byStatus := make(map[string]int)
	byCategory := make(map[string]int)
	for _, j := range f.Jobs {
		byStatus[j.Status]++
		if name := f.categoryName(j.CategoryID); name != "" {
			byCategory[name]++
		}
	}
	return Stats{
		Total:      len(f.Jobs),
		ByStatus:   byStatus,
		ByCategory: byCategory,
	}, nil
}

// --- Artifacts ---

func (f *FakeStore) GetArtifacts(skillID string, jobID int64, includeArchived bool) ([]Artifact, error) {
	var arts []Artifact
	for _, a := range f.Artifacts {
		if skillID != "" && a.SkillID != skillID {
			continue
		}
		if jobID > 0 && (a.JobID == nil || *a.JobID != jobID) {
			continue
		}
		if !includeArchived && a.Archived {
			continue
		}
		arts = append(arts, a)
	}
	return arts, nil
}

func (f *FakeStore) GetArtifact(id int64) (Artifact, error) {
	a, ok := f.Artifacts[id]
	if !ok {
		return Artifact{}, fmt.Errorf("artifact %d not found", id)
	}
	return a, nil
}

func (f *FakeStore) AddArtifact(a Artifact) (Artifact, error) {
	f.nextArtID++
	a.ID = f.nextArtID
	now := time.Now().UTC().Format(time.RFC3339)
	if a.Options == "" {
		a.Options = "{}"
	}
	if a.Variants == "" {
		a.Variants = "[]"
	}
	a.CreatedAt = now
	a.UpdatedAt = now
	f.Artifacts[a.ID] = a
	return a, nil
}

func (f *FakeStore) UpdateArtifact(id int64, updates map[string]any) (Artifact, error) {
	a, ok := f.Artifacts[id]
	if !ok {
		return Artifact{}, fmt.Errorf("artifact %d not found", id)
	}
	if v, ok := updates["title"]; ok {
		a.Title = fmt.Sprint(v)
	}
	if v, ok := updates["options"]; ok {
		a.Options = fmt.Sprint(v)
	}
	if v, ok := updates["variants"]; ok {
		a.Variants = fmt.Sprint(v)
	}
	a.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	f.Artifacts[id] = a
	return a, nil
}

func (f *FakeStore) ArchiveArtifact(id int64) error {
	a, ok := f.Artifacts[id]
	if !ok {
		return fmt.Errorf("artifact %d not found", id)
	}
	a.Archived = true
	a.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	f.Artifacts[id] = a
	return nil
}

func (f *FakeStore) DeleteArtifact(id int64) error {
	if _, ok := f.Artifacts[id]; !ok {
		return fmt.Errorf("artifact %d not found", id)
	}
	delete(f.Artifacts, id)
	return nil
}

func (f *FakeStore) SearchArtifacts(query string) ([]Artifact, error) {
	q := strings.ToLower(query)
	var arts []Artifact
	for _, a := range f.Artifacts {
		if strings.Contains(strings.ToLower(a.Title), q) ||
			strings.Contains(strings.ToLower(a.SkillID), q) {
			arts = append(arts, a)
		}
	}
	return arts, nil
}

func (f *FakeStore) SearchAll(query string) ([]SearchResultItem, error) {
	q := strings.ToLower(query)
	var results []SearchResultItem
	for _, j := range f.Jobs {
		if strings.Contains(strings.ToLower(j.Company), q) ||
			strings.Contains(strings.ToLower(j.Position), q) {
			results = append(results, SearchResultItem{
				Type:  "job",
				ID:    j.ID,
				Title: j.Company + " — " + j.Position,
				Sub:   j.Status,
				Match: f.categoryName(j.CategoryID),
			})
		}
	}
	for _, a := range f.Artifacts {
		if strings.Contains(strings.ToLower(a.Title), q) {
			results = append(results, SearchResultItem{
				Type:  "artifact",
				ID:    a.ID,
				Title: a.Title,
				Sub:   a.SkillID,
				Match: "artifact",
			})
		}
	}
	return results, nil
}

// --- Profile & Settings ---

func (f *FakeStore) GetProfile() (Profile, error) {
	return f.Profile, nil
}

func (f *FakeStore) UpsertProfile(updates map[string]any) error {
	if v, ok := updates["name"]; ok {
		f.Profile.Name = fmt.Sprint(v)
	}
	if v, ok := updates["email"]; ok {
		f.Profile.Email = fmt.Sprint(v)
	}
	if v, ok := updates["phone"]; ok {
		f.Profile.Phone = fmt.Sprint(v)
	}
	if v, ok := updates["title"]; ok {
		f.Profile.Title = fmt.Sprint(v)
	}
	if v, ok := updates["industry"]; ok {
		f.Profile.Industry = fmt.Sprint(v)
	}
	if v, ok := updates["greeting_style"]; ok {
		f.Profile.GreetingStyle = fmt.Sprint(v)
	}
	if v, ok := updates["sign_off"]; ok {
		f.Profile.SignOff = fmt.Sprint(v)
	}
	return nil
}

func (f *FakeStore) GetSettings() (Settings, error) {
	return f.Settings, nil
}

func (f *FakeStore) UpsertSettings(updates map[string]any) error {
	if v, ok := updates["theme"]; ok {
		f.Settings.Theme = fmt.Sprint(v)
	}
	if v, ok := updates["default_view"]; ok {
		f.Settings.DefaultView = fmt.Sprint(v)
	}
	if v, ok := updates["items_per_page"]; ok {
		if n, ok := v.(int); ok {
			f.Settings.ItemsPerPage = n
		}
	}
	return nil
}

// --- Lifecycle ---

func (f *FakeStore) RunMigrations(dbPath string) error { return nil }
func (f *FakeStore) Close() error                      { return nil }
