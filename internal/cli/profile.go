package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/udit-001/waypoint/internal/db"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage your user profile",
	Long: `View and update your user profile.

The profile stores your name, contact info, professional title,
skills, experience, education, and email preferences. It's used
by the AI generation skills to personalize content.

Examples:
  waypoint profile show
  waypoint profile set --name "Jane Doe" --title "Senior Engineer"
  waypoint profile set --skills '["Go","React","Python"]'
  waypoint profile show --json`,
}

// --- show ---

var profileShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display your profile",
	Long: `Show all profile fields.

Examples:
  waypoint profile show
  waypoint profile show --json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := store.GetProfile()
		if err != nil {
			return formatError("failed to get profile", err)
		}

		if jsonOut {
			printJSON(p)
			return nil
		}

		fmt.Println()
		fmt.Printf("  Name:           %s\n", displayVal(p.Name))
		fmt.Printf("  Email:          %s\n", displayVal(p.Email))
		fmt.Printf("  Phone:          %s\n", displayVal(p.Phone))
		fmt.Printf("  Title:          %s\n", displayVal(p.Title))
		fmt.Printf("  Industry:       %s\n", displayVal(p.Industry))

		// Parse JSON array fields for display
		fmt.Printf("  Skills:         %s\n", displayJSONList(p.Skills))
		fmt.Printf("  Education:      %s\n", displayEducation(p.Education))
		fmt.Printf("  Experience:     %s\n", displayExperience(p.Experience))

		fmt.Println()
		return nil
	},
}

// --- brief ---

var profileBriefCmd = &cobra.Command{
	Use:   "brief",
	Short: "Show the job-search curation brief",
	Long: `Display the curation brief the agent uses to search for jobs:
what is settled (facts, constraints) and what is still open (preferences).

The agent reads the frontier from --json: 'open' lists the preferences still
to answer and 'complete' is true when they are all set. Facts never gate
completion — a missing fact means a seed (resume / public profile) has not
arrived, not that the user must be interviewed.

Examples:
  waypoint profile brief
  waypoint profile brief --json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		b, err := store.GetBrief()
		if err != nil {
			return formatError("failed to get brief", err)
		}

		if jsonOut {
			printJSON(b)
			return nil
		}

		fmt.Println()
		fmt.Printf("  Facts:\n")
		fmt.Printf("    Title:           %s\n", displayVal(b.Facts.Title))
		fmt.Printf("    Seniority:       %s\n", displayVal(b.Facts.Seniority))
		fmt.Printf("    Current location: %s\n", displayVal(b.Facts.CurrentLocation))
		fmt.Printf("    Skills:          %s\n", joinList(b.Facts.Skills))
		fmt.Printf("  Constraints:\n")
		fmt.Printf("    Visa sponsorship:%s\n", displayVal(b.Constraints.VisaSponsorship))
		fmt.Printf("    Salary floor:    %s\n", salaryFloorDisplay(b.Constraints.SalaryFloor))
		fmt.Printf("  Preferences:\n")
		fmt.Printf("    Remote:          %s\n", displayVal(b.Preferences.Remote))
		fmt.Printf("    Location:        %s\n", joinList(b.Preferences.LocationPref))
		fmt.Printf("    Companies:       %s\n", joinList(b.Preferences.Companies))
		fmt.Printf("    Avoid:           %s\n", joinList(b.Preferences.AvoidCompanies))
		fmt.Printf("    Keywords:        %s\n", joinList(b.Preferences.Keywords))
		fmt.Printf("    Dealbreakers:    %s\n", joinList(b.Preferences.Dealbreakers))
		fmt.Printf("\n")
		fmt.Printf("  Open: %s\n", joinList(b.Open))
		if b.Complete {
			fmt.Printf("  Status: complete — brief is ready to search on.\n")
		} else {
			fmt.Printf("  Status: incomplete — %d preference(s) still open.\n", len(b.Open))
		}
		fmt.Println()
		return nil
	},
}

// joinList renders a string slice comma-separated, or a dash if empty.
func joinList(items []string) string {
	if len(items) == 0 {
		return "-"
	}
	return strings.Join(items, ", ")
}

// salaryFloorDisplay renders salary-floor entries with their derived currency,
// e.g. "INR 100000 (IN)" or a dash when none.
func salaryFloorDisplay(entries []db.SalaryFloorEntry) string {
	if len(entries) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Currency != "" {
			parts = append(parts, fmt.Sprintf("%s %d (%s)", e.Currency, e.Amount, e.Region))
		} else {
			parts = append(parts, fmt.Sprintf("%d (%s)", e.Amount, e.Region))
		}
	}
	return strings.Join(parts, ", ")
}

// --- set ---

var profileSetFlags struct {
	name       string
	email      string
	phone      string
	title      string
	skills     string
	experience string
	education  string
	industry   string

	// Curation brief.
	currentLocation string
	seniority       string
	visaSponsorship string
	salaryFloor     string
	remote          string
	locationPref    string
	companies       string
	avoidCompanies  string
	keywords        string
	dealbreakers    string
}

var profileSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Update profile fields",
	Long: `Update one or more profile fields. Only the flags you provide are changed.

Skills take a comma-separated list (shell-safe) or a JSON array:
  --skills "Go,React,Python"

Experience and education are structured — a JSON array of objects, dates as
YYYY-MM (or YYYY), empty end means present:
  --experience '[{"title":"Senior SWE","company":"Acme","start":"2021-03","end":"2023-06"}]'
  --education '[{"institution":"MIT","degree":"BS CS","start":"2015","end":"2019"}]'

Examples:
  waypoint profile set --name "Jane Doe" --title "Senior Engineer"
  waypoint profile set --skills "Go,React,AWS"
  waypoint profile set --email "jane@example.com" --phone "+1-555-0123"`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		updates := make(map[string]any)

		if profileSetFlags.name != "" {
			updates["name"] = profileSetFlags.name
		}
		if profileSetFlags.email != "" {
			updates["email"] = profileSetFlags.email
		}
		if profileSetFlags.phone != "" {
			updates["phone"] = profileSetFlags.phone
		}
		if profileSetFlags.title != "" {
			updates["title"] = profileSetFlags.title
		}
		// Skills keep the shared list input: comma-separated or a JSON array.
		if profileSetFlags.skills != "" {
			normalized, err := parseListInput(profileSetFlags.skills)
			if err != nil {
				return fmt.Errorf("skills: %w", err)
			}
			updates["skills"] = normalized
		}
		// Experience / education are structured: a JSON array of objects only.
		if profileSetFlags.experience != "" {
			serialized, err := parseStructuredInput(profileSetFlags.experience, db.ExperienceToJSON, func(e db.ExperienceEntry) error {
				if strings.TrimSpace(e.Title) == "" {
					return fmt.Errorf("title is required")
				}
				if err := db.ValidatePartialDate(e.Start); err != nil {
					return err
				}
				return db.ValidatePartialDate(e.End)
			})
			if err != nil {
				return fmt.Errorf("experience: %w", err)
			}
			updates["experience"] = serialized
		}
		if profileSetFlags.education != "" {
			serialized, err := parseStructuredInput(profileSetFlags.education, db.EducationToJSON, func(e db.EducationEntry) error {
				if strings.TrimSpace(e.Institution) == "" {
					return fmt.Errorf("institution is required")
				}
				if err := db.ValidatePartialDate(e.Start); err != nil {
					return err
				}
				return db.ValidatePartialDate(e.End)
			})
			if err != nil {
				return fmt.Errorf("education: %w", err)
			}
			updates["education"] = serialized
		}
		if profileSetFlags.industry != "" {
			updates["industry"] = profileSetFlags.industry
		}

		// Curation brief. Each flag uses cmd.Flags().Changed so an explicit
		// empty value clears the field (set → open) rather than being ignored.
		// Flag names equal the profile key verbatim (WP-103).
		setScalar := func(flagName, key, val string) {
			if cmd.Flags().Changed(flagName) {
				updates[key] = val
			}
		}
		setList := func(flagName, key, val string) error {
			if !cmd.Flags().Changed(flagName) {
				return nil
			}
			if val == "" {
				updates[key] = "[]"
				return nil
			}
			normalized, err := parseListInput(val)
			if err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
			updates[key] = normalized
			return nil
		}

		setScalar("current-location", "current_location", profileSetFlags.currentLocation)

		// Seniority is a derived fact: once experience carries a year signal,
		// the level is derived, not manually assignable. Manual set is a
		// placeholder for when experience has not arrived yet (no resume seed).
		if cmd.Flags().Changed("seniority") {
			exp, _ := store.GetProfile()
			if derived := db.DeriveSeniority(exp.Experience); derived != "" {
				return fmt.Errorf("seniority derives from experience as %q — correct experience instead, or clear it first", derived)
			}
			updates["seniority"] = profileSetFlags.seniority
		}
		setScalar("visa-sponsorship", "visa_sponsorship", profileSetFlags.visaSponsorship)
		setScalar("remote", "remote", profileSetFlags.remote)

		if cmd.Flags().Changed("salary-floor") {
			if profileSetFlags.salaryFloor == "" {
				updates["salary_floor"] = "[]"
			} else {
				// Amount-only entries default their region to the effective
				// current location: the --current-location flag if set this
				// run, otherwise the profile's stored value. Region is the
				// user's decision; currency is derived from it (never asked).
				defaultRegion := ""
				if cmd.Flags().Changed("current-location") {
					defaultRegion = profileSetFlags.currentLocation
				} else {
					if p, err := store.GetProfile(); err == nil {
						defaultRegion = p.CurrentLocation
					}
				}
				floors, err := db.ParseSalaryFloor(profileSetFlags.salaryFloor, defaultRegion)
				if err != nil {
					return formatError("invalid salary floor", err)
				}
				serialized, err := db.SalaryFloorToJSON(floors)
				if err != nil {
					return formatError("failed to encode salary floor", err)
				}
				updates["salary_floor"] = serialized
			}
		}

		for _, f := range []struct{ flag, key, val string }{
			{"location-preference", "location_preference", profileSetFlags.locationPref},
			{"companies", "companies", profileSetFlags.companies},
			{"avoid-companies", "avoid_companies", profileSetFlags.avoidCompanies},
			{"keywords", "keywords", profileSetFlags.keywords},
			{"dealbreakers", "dealbreakers", profileSetFlags.dealbreakers},
		} {
			if err := setList(f.flag, f.key, f.val); err != nil {
				return err
			}
		}

		if len(updates) == 0 {
			return fmt.Errorf("no fields to update — use --flags to specify changes")
		}

		if err := store.UpsertProfile(updates); err != nil {
			return formatError("failed to update profile", err)
		}

		if jsonOut {
			p, _ := store.GetProfile()
			printJSON(p)
			return nil
		}

		fmt.Println()
		fmt.Printf("  ✓ Profile updated\n")
		for key := range updates {
			switch key {
			case "name":
				fmt.Println("    Name:           updated")
			case "email":
				fmt.Println("    Email:          updated")
			case "phone":
				fmt.Println("    Phone:          updated")
			case "title":
				fmt.Println("    Title:          updated")
			case "skills":
				fmt.Println("    Skills:         updated")
			case "experience":
				fmt.Println("    Experience:     updated")
			case "education":
				fmt.Println("    Education:      updated")
			case "industry":
				fmt.Println("    Industry:       updated")
			case "current_location":
				fmt.Println("    Current Location: updated")
			case "seniority":
				fmt.Println("    Seniority:      updated")
			case "visa_sponsorship":
				fmt.Println("    Visa Sponsorship: updated")
			case "salary_floor":
				fmt.Println("    Salary Floor:   updated")
			case "remote":
				fmt.Println("    Remote:         updated")
			case "location_preference":
				fmt.Println("    Location:       updated")
			case "companies":
				fmt.Println("    Companies:      updated")
			case "avoid_companies":
				fmt.Println("    Avoid:          updated")
			case "keywords":
				fmt.Println("    Keywords:       updated")
			case "dealbreakers":
				fmt.Println("    Dealbreakers:   updated")
			}
		}
		fmt.Println()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(profileCmd)
	profileCmd.AddCommand(profileShowCmd)
	profileCmd.AddCommand(profileBriefCmd)
	profileCmd.AddCommand(profileSetCmd)

	profileSetCmd.Flags().StringVar(&profileSetFlags.name, "name", "", "Full name")
	profileSetCmd.Flags().StringVar(&profileSetFlags.email, "email", "", "Email address")
	profileSetCmd.Flags().StringVar(&profileSetFlags.phone, "phone", "", "Phone number")
	profileSetCmd.Flags().StringVar(&profileSetFlags.title, "title", "", "Professional title")
	profileSetCmd.Flags().StringVar(&profileSetFlags.skills, "skills", "", "Skills as JSON array")
	profileSetCmd.Flags().StringVar(&profileSetFlags.experience, "experience", "", "Experience as JSON array of {title, company, start, end} objects")
	profileSetCmd.Flags().StringVar(&profileSetFlags.education, "education", "", "Education as JSON array of {institution, degree, start, end} objects")
	profileSetCmd.Flags().StringVar(&profileSetFlags.industry, "industry", "", "Target industry")

	// Curation brief.
	profileSetCmd.Flags().StringVar(&profileSetFlags.currentLocation, "current-location", "", "Where you live now (seeds salary/remote defaults)")
	profileSetCmd.Flags().StringVar(&profileSetFlags.seniority, "seniority", "", "Seniority level (junior|mid|senior); derived from experience when unset")
	profileSetCmd.Flags().StringVar(&profileSetFlags.visaSponsorship, "visa-sponsorship", "", "Visa sponsorship required (yes|no)")
	profileSetCmd.Flags().StringVar(&profileSetFlags.salaryFloor, "salary-floor", "", "Salary floor as region:amount, e.g. \"IN:100000,GB:30000\"")
	profileSetCmd.Flags().StringVar(&profileSetFlags.remote, "remote", "", "Workplace type (remote|hybrid|onsite)")
	profileSetCmd.Flags().StringVar(&profileSetFlags.locationPref, "location-preference", "", "Location preference as comma list or JSON array")
	profileSetCmd.Flags().StringVar(&profileSetFlags.companies, "companies", "", "Target companies as comma list or JSON array")
	profileSetCmd.Flags().StringVar(&profileSetFlags.avoidCompanies, "avoid-companies", "", "Companies to avoid as comma list or JSON array")
	profileSetCmd.Flags().StringVar(&profileSetFlags.keywords, "keywords", "", "Must-have keywords as comma list or JSON array")
	profileSetCmd.Flags().StringVar(&profileSetFlags.dealbreakers, "dealbreakers", "", "Must-not-have terms as comma list or JSON array")
}

// displayVal returns the value or a dash if empty.
func displayVal(s string) string {
	if s == "" || s == "[]" {
		return "-"
	}
	return s
}

// displayJSONList parses a JSON array string for display.
func displayJSONList(s string) string {
	if s == "" || s == "[]" {
		return "-"
	}
	var items []string
	if err := json.Unmarshal([]byte(s), &items); err != nil {
		return s
	}
	result := ""
	for i, item := range items {
		if i > 0 {
			result += ", "
		}
		result += item
	}
	return result
}

// parseListInput normalizes a list value for profile array fields.
// It accepts either a JSON array ("[\"Go\",\"React\"]") or a plain
// comma-separated list ("Go,React"), returning the JSON array form
// that the profile stores. The comma form is shell-friendly — no
// nested quotes — so it works in PowerShell as well as bash.
func parseListInput(s string) (string, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return "", fmt.Errorf("value cannot be empty")
	}
	if strings.HasPrefix(trimmed, "[") {
		var v any
		if err := json.Unmarshal([]byte(trimmed), &v); err != nil {
			return "", fmt.Errorf("not a valid JSON array, e.g. [\"Go\",\"React\"] or a comma list: Go,React")
		}
		if _, ok := v.([]any); !ok {
			return "", fmt.Errorf("value must be a JSON array, e.g. [\"Go\",\"React\"] or a comma list: Go,React")
		}
		return trimmed, nil
	}
	parts := strings.Split(trimmed, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return "", fmt.Errorf("value cannot be empty")
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// parseStructuredInput parses a JSON array of structured entries for the
// experience/education flags, validates each via validate, and serializes to
// the stored JSON-array string.
func parseStructuredInput[T any](raw string, toJSON func([]T) (string, error), validate func(T) error) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("value cannot be empty")
	}
	var entries []T
	if err := json.Unmarshal([]byte(trimmed), &entries); err != nil {
		return "", fmt.Errorf("must be a JSON array of objects, e.g. '[{\"title\":\"SWE\",\"start\":\"2021-03\"}]'")
	}
	for i, e := range entries {
		if err := validate(e); err != nil {
			return "", fmt.Errorf("entry %d: %w", i+1, err)
		}
	}
	return toJSON(entries)
}

// displayExperience renders structured experience entries for `profile show`.
func displayExperience(s string) string {
	entries := db.ParseExperienceEntries(s)
	if len(entries) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		at := strings.TrimSpace(e.Company)
		name := strings.TrimSpace(e.Title)
		if name == "" {
			name = at
			at = ""
		}
		dates := strings.TrimSpace(e.Start)
		if dates != "" {
			if e.End != "" {
				dates += " – " + e.End
			} else {
				dates += " – present"
			}
		}
		var sb strings.Builder
		sb.WriteString(name)
		if at != "" {
			sb.WriteString(" @ " + at)
		}
		if dates != "" {
			sb.WriteString(" (" + dates + ")")
		}
		parts = append(parts, sb.String())
	}
	return strings.Join(parts, "; ")
}

// displayEducation renders structured education entries for `profile show`.
func displayEducation(s string) string {
	entries := db.ParseEducationEntries(s)
	if len(entries) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		deg := strings.TrimSpace(e.Degree)
		inst := strings.TrimSpace(e.Institution)
		if deg == "" {
			deg = inst
			inst = ""
		}
		dates := strings.TrimSpace(e.Start)
		if dates != "" {
			if e.End != "" {
				dates += " – " + e.End
			} else {
				dates += " – present"
			}
		}
		var sb strings.Builder
		sb.WriteString(deg)
		if inst != "" {
			sb.WriteString(", " + inst)
		}
		if dates != "" {
			sb.WriteString(" (" + dates + ")")
		}
		parts = append(parts, sb.String())
	}
	return strings.Join(parts, "; ")
}
