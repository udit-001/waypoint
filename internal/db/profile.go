package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Profile represents the user profile (singleton row).
type Profile struct {
	Name       string `db:"name" json:"name"`
	Email      string `db:"email" json:"email"`
	Phone      string `db:"phone" json:"phone"`
	Title      string `db:"title" json:"title"`
	Skills     string `db:"skills" json:"-"`     // raw JSON string from DB; emitted as array via MarshalJSON
	Experience string `db:"experience" json:"-"` // raw JSON string from DB; emitted as structured entries
	Education  string `db:"education" json:"-"`  // raw JSON string from DB; emitted as structured entries
	Industry   string `db:"industry" json:"industry"`

	// Curation brief — facts.
	CurrentLocation string `db:"current_location" json:"currentLocation"`
	Seniority       string `db:"seniority" json:"seniority"`

	// Curation brief — constraints.
	VisaSponsorship string `db:"visa_sponsorship" json:"visaSponsorship"`
	SalaryFloor     string `db:"salary_floor" json:"-"` // raw JSON string from DB; emitted as array via MarshalJSON

	// Curation brief — preferences (the brief). Array-valued ones store a
	// JSON array string and are emitted as arrays via MarshalJSON.
	Remote         string `db:"remote" json:"remote"`
	LocationPref   string `db:"location_preference" json:"-"`
	Companies      string `db:"companies" json:"-"`
	AvoidCompanies string `db:"avoid_companies" json:"-"`
	Keywords       string `db:"keywords" json:"-"`
	Dealbreakers   string `db:"dealbreakers" json:"-"`
}

// MarshalJSON ensures Skills is returned as a JSON array and Experience /
// Education as structured entries (upgraded from legacy flat strings on read).
func (p Profile) MarshalJSON() ([]byte, error) {
	type Alias Profile
	parseArray := func(s string) []string {
		if s == "" {
			return []string{}
		}
		var arr []string
		if err := json.Unmarshal([]byte(s), &arr); err != nil {
			return []string{}
		}
		return arr
	}
	parseFloors := func(s string) []SalaryFloorEntry {
		if s == "" {
			return []SalaryFloorEntry{}
		}
		var arr []SalaryFloorEntry
		if err := json.Unmarshal([]byte(s), &arr); err != nil {
			return []SalaryFloorEntry{}
		}
		return arr
	}
	return json.Marshal(&struct {
		*Alias
		Skills         []string           `json:"skills"`
		Experience     []ExperienceEntry  `json:"experience"`
		Education      []EducationEntry   `json:"education"`
		LocationPref   []string           `json:"locationPreference"`
		Companies      []string           `json:"companies"`
		AvoidCompanies []string           `json:"avoidCompanies"`
		Keywords       []string           `json:"keywords"`
		Dealbreakers   []string           `json:"dealbreakers"`
		SalaryFloor    []SalaryFloorEntry `json:"salaryFloor"`
	}{
		Alias:          (*Alias)(&p),
		Skills:         parseArray(p.Skills),
		Experience:     ParseExperienceEntries(p.Experience),
		Education:      ParseEducationEntries(p.Education),
		LocationPref:   parseArray(p.LocationPref),
		Companies:      parseArray(p.Companies),
		AvoidCompanies: parseArray(p.AvoidCompanies),
		Keywords:       parseArray(p.Keywords),
		Dealbreakers:   parseArray(p.Dealbreakers),
		SalaryFloor:    parseFloors(p.SalaryFloor),
	})
}

// Settings represents app settings (singleton row).
type Settings struct {
	Theme            string `db:"theme" json:"theme"`
	RemindersEnabled int    `db:"reminders_enabled" json:"remindersEnabled"`
	DefaultView      string `db:"default_view" json:"defaultView"`
	ItemsPerPage     int    `db:"items_per_page" json:"itemsPerPage"`

	// Scrapers permanently opted out of autopilot (WP-175). Stored as a
	// JSON array string of scraper ids; emitted as an array via MarshalJSON.
	AutopilotDisabledScrapers string `db:"autopilot_disabled_scrapers" json:"-"`

	// Autopilot settings.
	AutopilotEnabled      int    `db:"autopilot_enabled" json:"autopilotEnabled"`
	AutopilotCadence      int    `db:"autopilot_cadence" json:"autopilotCadence"` // hours; 0 = default (6)
	AutopilotProvider     string `db:"autopilot_provider" json:"autopilotProvider"`
	ZenAPIKey             string `db:"zen_api_key" json:"zenApiKey"`
	ZenModel              string `db:"zen_model" json:"zenModel"`
	ExaAPIKey             string `db:"exa_api_key" json:"exaApiKey"`
	DiscoveryIntervalDays int    `db:"discovery_interval_days" json:"discoveryIntervalDays"`
}

// MarshalJSON emits AutopilotDisabledScrapers as a JSON array (the same
// raw-string-to-array pattern the Profile brief preferences use).
func (s Settings) MarshalJSON() ([]byte, error) {
	type Alias Settings
	return json.Marshal(&struct {
		*Alias
		AutopilotDisabledScrapers []string `json:"autopilotDisabledScrapers"`
	}{
		Alias:                     (*Alias)(&s),
		AutopilotDisabledScrapers: ParseDisabledScrapers(s.AutopilotDisabledScrapers),
	})
}

// ParseDisabledScrapers decodes the stored autopilot_disabled_scrapers JSON
// array string. Empty or unparseable input yields an empty slice — callers
// treat the field as "no scrapers opted out".
func ParseDisabledScrapers(s string) []string {
	out := stringList(s)
	if out == nil {
		return []string{}
	}
	return out
}

// DisabledScrapersJSON encodes scraper ids into the canonical stored form
// (lowercase, trim, dedupe) — the same normalization the profile's list
// preferences apply.
func DisabledScrapersJSON(names []string) string {
	b, err := json.Marshal(normalizeListValues(names))
	if err != nil {
		return "[]"
	}
	return string(b)
}

// defaultSettings holds the Go-level defaults returned when no settings row
// exists in the database yet.
var defaultSettings = Settings{
	Theme:                 "light",
	RemindersEnabled:      1,
	DefaultView:           "dashboard",
	ItemsPerPage:          25,
	AutopilotCadence:      6, // default 6 hours
	DiscoveryIntervalDays: 30,
}

// GetProfile returns the user profile. If no profile row exists yet, it
// returns a zero-value Profile with a nil error; callers detect "not set up"
// by checking p.Name == "".
func (s *SQLiteStore) GetProfile() (Profile, error) {
	var p Profile
	err := s.Get(&p, `SELECT name, email, phone, title, skills, experience, education, industry, current_location, seniority, visa_sponsorship, salary_floor, remote, location_preference, companies, avoid_companies, keywords, dealbreakers FROM profile WHERE id = 1`)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Profile{}, nil
		}
		return Profile{}, err
	}
	return p, nil
}

// UpsertProfile inserts the profile row if it doesn't exist, then updates the
// provided fields. Only keys present in the updates map are changed.
func (s *SQLiteStore) UpsertProfile(updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	columnMap := map[string]string{
		"name":                "name",
		"email":               "email",
		"phone":               "phone",
		"title":               "title",
		"skills":              "skills",
		"experience":          "experience",
		"education":           "education",
		"industry":            "industry",
		"current_location":    "current_location",
		"seniority":           "seniority",
		"visa_sponsorship":    "visa_sponsorship",
		"salary_floor":        "salary_floor",
		"remote":              "remote",
		"location_preference": "location_preference",
		"companies":           "companies",
		"avoid_companies":     "avoid_companies",
		"keywords":            "keywords",
		"dealbreakers":        "dealbreakers",
	}
	var setClauses []string
	var args []any
	for key, col := range columnMap {
		if val, ok := updates[key]; ok {
			// Normalize the brief's list-valued preferences so the stored
			// value is the normalized match form (case-fold, trim, dedupe).
			if isListPrefKey(key) {
				s, _ := val.(string)
				val = normalizeListJSON(s)
			}
			setClauses = append(setClauses, col+" = ?")
			args = append(args, val)
		}
	}
	if len(setClauses) == 0 {
		return nil
	}
	if _, err := s.Exec(`INSERT INTO profile (id) VALUES (1) ON CONFLICT(id) DO NOTHING`); err != nil {
		return err
	}
	args = append(args, 1) // id = 1
	query := fmt.Sprintf("UPDATE profile SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	_, err := s.Exec(query, args...)
	return err
}

// isListPrefKey reports whether a profile key is a brief preference whose
// stored value is a normalized JSON list (case-fold, trim, dedupe).
func isListPrefKey(key string) bool {
	switch key {
	case "location_preference", "companies", "avoid_companies", "keywords", "dealbreakers":
		return true
	}
	return false
}

// ensureAutopilotSettingsColumns adds the migration-10/11 columns when
// missing. Idempotent — SQLite ADD COLUMN errors on duplicates, which we
// ignore. Called by both settings readers and writers: a fresh database
// must accept PATCH autopilot_enabled=1 before ANY settings read has
// ever run (the daemon ticker, the usual first reader, doesn't start
// while autopilot is off).
func ensureAutopilotSettingsColumns(s *SQLiteStore) {
	_, _ = s.Exec(`ALTER TABLE settings ADD COLUMN autopilot_enabled INTEGER NOT NULL DEFAULT 0`)
	_, _ = s.Exec(`ALTER TABLE settings ADD COLUMN autopilot_cadence INTEGER NOT NULL DEFAULT 6`)
	_, _ = s.Exec(`ALTER TABLE settings ADD COLUMN autopilot_provider TEXT NOT NULL DEFAULT ''`)
	_, _ = s.Exec(`ALTER TABLE settings ADD COLUMN zen_api_key TEXT DEFAULT ''`)
	_, _ = s.Exec(`ALTER TABLE settings ADD COLUMN zen_model TEXT DEFAULT ''`)
	_, _ = s.Exec(`ALTER TABLE settings ADD COLUMN exa_api_key TEXT DEFAULT ''`)
	_, _ = s.Exec(`ALTER TABLE settings ADD COLUMN discovery_interval_days INTEGER NOT NULL DEFAULT 30`)
	_, _ = s.Exec(`ALTER TABLE settings ADD COLUMN autopilot_disabled_scrapers TEXT NOT NULL DEFAULT ''`)
}

// GetSettings returns the app settings. If no settings row exists yet, it
// returns Go-level defaults with a nil error.
func (s *SQLiteStore) GetSettings() (Settings, error) {
	var st Settings
	ensureAutopilotSettingsColumns(s)

	err := s.Get(&st, `SELECT theme, reminders_enabled, default_view, items_per_page, autopilot_enabled, autopilot_cadence, autopilot_provider, zen_api_key, zen_model, exa_api_key, discovery_interval_days, autopilot_disabled_scrapers FROM settings WHERE id = 1`)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return defaultSettings, nil
		}
		return Settings{}, err
	}
	return st, nil
}

// UpsertSettings inserts the settings row if it doesn't exist, then updates the
// provided fields. Only keys present in the updates map are changed.
func (s *SQLiteStore) UpsertSettings(updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}

	ensureAutopilotSettingsColumns(s)

	columnMap := map[string]string{
		"theme":                   "theme",
		"reminders_enabled":       "reminders_enabled",
		"default_view":            "default_view",
		"items_per_page":          "items_per_page",
		"autopilot_enabled":       "autopilot_enabled",
		"autopilot_cadence":       "autopilot_cadence",
		"autopilot_provider":      "autopilot_provider",
		"zen_api_key":             "zen_api_key",
		"zen_model":               "zen_model",
		"exa_api_key":             "exa_api_key",
		"discovery_interval_days": "discovery_interval_days",

		"autopilot_disabled_scrapers": "autopilot_disabled_scrapers",
	}
	var setClauses []string
	var args []any
	for key, col := range columnMap {
		if val, ok := updates[key]; ok {
			// The disabled-scrapers list is stored in canonical form (the
			// same normalization profile list preferences get), so reads are
			// deterministic no matter which surface wrote the value.
			if key == "autopilot_disabled_scrapers" {
				s, _ := val.(string)
				val = normalizeListJSON(s)
			}
			setClauses = append(setClauses, col+" = ?")
			args = append(args, val)
		}
	}
	if len(setClauses) == 0 {
		return nil
	}
	if _, err := s.Exec(`INSERT INTO settings (id) VALUES (1) ON CONFLICT(id) DO NOTHING`); err != nil {
		return err
	}
	args = append(args, 1) // id = 1
	query := fmt.Sprintf("UPDATE settings SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	_, err := s.Exec(query, args...)
	return err
}
