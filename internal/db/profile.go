package db

import (
	"fmt"
	"strings"
)

// Profile represents the user profile (singleton row).
type Profile struct {
	Name          string `db:"name" json:"name"`
	Email         string `db:"email" json:"email"`
	Phone         string `db:"phone" json:"phone"`
	Title         string `db:"title" json:"title"`
	Skills        string `db:"skills" json:"skills"`
	Experience    string `db:"experience" json:"experience"`
	Education     string `db:"education" json:"education"`
	Industry      string `db:"industry" json:"industry"`
	GreetingStyle string `db:"greeting_style" json:"greetingStyle"`
	SignOff       string `db:"sign_off" json:"signOff"`
}

// Settings represents app settings (singleton row).
type Settings struct {
	Theme             string `db:"theme" json:"theme"`
	RemindersEnabled  int    `db:"reminders_enabled" json:"remindersEnabled"`
	DefaultView       string `db:"default_view" json:"defaultView"`
	ItemsPerPage      int    `db:"items_per_page" json:"itemsPerPage"`
}

// GetProfile returns the user profile.
func (s *Store) GetProfile() (Profile, error) {
	var p Profile
	err := s.Get(&p, `SELECT name, email, phone, title, skills, experience, education, industry, greeting_style, sign_off FROM profile WHERE id = 1`)
	return p, err
}

// UpdateProfile updates profile fields. Only non-empty fields in the map are changed.
func (s *Store) UpdateProfile(updates map[string]any) error {
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
	args = append(args, 1) // id = 1
	query := fmt.Sprintf("UPDATE profile SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	_, err := s.Exec(query, args...)
	return err
}

// GetSettings returns the app settings.
func (s *Store) GetSettings() (Settings, error) {
	var st Settings
	err := s.Get(&st, `SELECT theme, reminders_enabled, default_view, items_per_page FROM settings WHERE id = 1`)
	return st, err
}
