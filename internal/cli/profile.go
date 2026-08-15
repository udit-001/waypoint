package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
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
  waypoint profile set --file profile.json
  waypoint profile schema
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
	file string
}

var profileSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Update profile fields from a document",
	Long: `Update profile fields from a JSON document with patch semantics: only
keys present in the document are changed; every other field stays untouched.

To clear a field, include it with an empty value — "" for a scalar
({"email": ""}), [] for a list ({"skills": []}).

A list value is a bare array (replace the whole list) or one op object:
  {"skills": {"append": ["Kotlin"]}}   {"skills": {"remove": ["Java"]}}

The document is the same shape 'profile show --json' emits — run
'waypoint profile schema' for the empty template. Unknown keys are rejected,
so a typo never silently drops an edit. The file is read directly, so the
shell never interprets its contents — the cross-shell-safe way to write
structured data (experience/education entries, salary floors, lists).

Examples:
  waypoint profile set --file profile.json
  waypoint profile schema                          # see the writable shape
  waypoint profile show --json | waypoint profile set --file -   # stdin`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		raw, err := readDocInput(profileSetFlags.file)
		if err != nil {
			return formatError("failed to read profile document", err)
		}
		var doc map[string]json.RawMessage
		if err := json.Unmarshal(raw, &doc); err != nil {
			return formatError("invalid profile document — must be a JSON object", err)
		}

		// Current experience gates manual seniority (placeholder until a
		// resume seed arrives). Validation lives behind the db seam, so the
		// CLI and the web route can never drift apart.
		exp, err := store.GetProfile()
		if err != nil {
			return formatError("failed to get profile", err)
		}
		updates, err := db.NormalizeProfileDocument(doc, exp)
		if err != nil {
			return err
		}

		if err := store.UpsertProfile(updates); err != nil {
			return formatError("failed to update profile", err)
		}

		if jsonOut {
			p, err := store.GetProfile()
			if err != nil {
				return formatError("failed to get profile", err)
			}
			printJSON(p)
			return nil
		}

		ops := db.ProfileDocOps(doc)
		keys := make([]string, 0, len(updates))
		for k := range updates {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Println()
		fmt.Printf("  ✓ Profile updated\n")
		for _, k := range keys {
			fmt.Printf("    %s: %s\n", profileKeyLabel(k), docOpVerb(ops[k]))
		}
		fmt.Println()
		return nil
	},
}

// profileKeyLabel renders a store key (the updates map) as a human label for
// `set` output.
func profileKeyLabel(key string) string {
	labels := map[string]string{
		"name":                "Name",
		"email":               "Email",
		"phone":               "Phone",
		"title":               "Title",
		"skills":              "Skills",
		"experience":          "Experience",
		"education":           "Education",
		"industry":            "Industry",
		"current_location":    "Current Location",
		"seniority":           "Seniority",
		"visa_sponsorship":    "Visa Sponsorship",
		"salary_floor":        "Salary Floor",
		"remote":              "Remote",
		"location_preference": "Location",
		"companies":           "Companies",
		"avoid_companies":     "Avoid",
		"keywords":            "Keywords",
		"dealbreakers":        "Dealbreakers",
	}
	if l, ok := labels[key]; ok {
		return l
	}
	words := strings.Split(strings.ReplaceAll(key, "_", " "), " ")
	for i, w := range words {
		if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// docOpVerb maps a list op label to a past-tense verb for 'profile set' output.
func docOpVerb(op string) string {
	switch op {
	case "append":
		return "appended"
	case "remove":
		return "removed"
	case "replace":
		return "replaced"
	case "clear":
		return "cleared"
	default:
		return "updated"
	}
}

// readDocInput reads the patch document: a file path, or stdin when the value
// is "-". The file is read directly — no shell interpretation of its contents.
func readDocInput(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}

// --- schema ---

var profileSchemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Show the profile document schema (empty template)",
	Long: `Print the profile document schema as an empty template — the writable
surface for 'waypoint profile set --file'. Fill in values and pass the file
back; keys absent from your document stay unchanged. The template is a valid
empty document — every value is what 'set' accepts unchanged.

To clear a field, write an empty value: "" for a scalar, [] for a list.
A list accepts a bare array (replace) or one op object {append|remove}.

Entry shapes: experience is [{"title","company","start","end","description"}],
education [{"institution","degree","start","end","description"}]; dates are
YYYY-MM (or YYYY), empty end means present, title/institution required.
salaryFloor is [{region, amount}] — region required, amount a positive number.

Example:
  waypoint profile schema`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(db.ProfileSchemaTemplate())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(profileCmd)
	profileCmd.AddCommand(profileShowCmd)
	profileCmd.AddCommand(profileBriefCmd)
	profileCmd.AddCommand(profileSetCmd)
	profileCmd.AddCommand(profileSchemaCmd)

	profileSetCmd.Flags().StringVar(&profileSetFlags.file, "file", "", "Profile document path (JSON object; '-' reads stdin)")
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
