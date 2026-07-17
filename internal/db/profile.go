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
	Name          string `db:"name" json:"name"`
	Email         string `db:"email" json:"email"`
	Phone         string `db:"phone" json:"phone"`
	Title         string `db:"title" json:"title"`
	Skills        string `db:"skills" json:"-"`     // raw JSON string from DB; emitted as array via MarshalJSON
	Experience    string `db:"experience" json:"-"` // raw JSON string from DB; emitted as array via MarshalJSON
	Education     string `db:"education" json:"-"`  // raw JSON string from DB; emitted as array via MarshalJSON
	Industry      string `db:"industry" json:"industry"`
	GreetingStyle string `db:"greeting_style" json:"greetingStyle"`
	SignOff       string `db:"sign_off" json:"signOff"`
}

// MarshalJSON ensures Skills, Experience, and Education are returned as
// proper JSON arrays instead of escaped strings.
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
	return json.Marshal(&struct {
		*Alias
		Skills     []string `json:"skills"`
		Experience []string `json:"experience"`
		Education  []string `json:"education"`
	}{
		Alias:      (*Alias)(&p),
		Skills:     parseArray(p.Skills),
		Experience: parseArray(p.Experience),
		Education:  parseArray(p.Education),
	})
}

// Settings represents app settings (singleton row).
type Settings struct {
	Theme            string `db:"theme" json:"theme"`
	RemindersEnabled int    `db:"reminders_enabled" json:"remindersEnabled"`
	DefaultView      string `db:"default_view" json:"defaultView"`
	ItemsPerPage     int    `db:"items_per_page" json:"itemsPerPage"`
}

// defaultSettings holds the Go-level defaults returned when no settings row
// exists in the database yet.
var defaultSettings = Settings{
	Theme:            "light",
	RemindersEnabled: 1,
	DefaultView:      "dashboard",
	ItemsPerPage:     25,
}

// GetProfile returns the user profile. If no profile row exists yet, it
// returns a zero-value Profile with a nil error; callers detect "not set up"
// by checking p.Name == "".
func (s *SQLiteStore) GetProfile() (Profile, error) {
	var p Profile
	err := s.Get(&p, `SELECT name, email, phone, title, skills, experience, education, industry, greeting_style, sign_off FROM profile WHERE id = 1`)
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
		"name":           "name",
		"email":          "email",
		"phone":          "phone",
		"title":          "title",
		"skills":         "skills",
		"experience":     "experience",
		"education":      "education",
		"industry":       "industry",
		"greeting_style": "greeting_style",
		"sign_off":       "sign_off",
	}
	var setClauses []string
	var args []any
	for key, col := range columnMap {
		if val, ok := updates[key]; ok {
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

// GetSettings returns the app settings. If no settings row exists yet, it
// returns Go-level defaults with a nil error.
func (s *SQLiteStore) GetSettings() (Settings, error) {
	var st Settings
	err := s.Get(&st, `SELECT theme, reminders_enabled, default_view, items_per_page FROM settings WHERE id = 1`)
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
	columnMap := map[string]string{
		"theme":             "theme",
		"reminders_enabled": "reminders_enabled",
		"default_view":      "default_view",
		"items_per_page":    "items_per_page",
	}
	var setClauses []string
	var args []any
	for key, col := range columnMap {
		if val, ok := updates[key]; ok {
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
