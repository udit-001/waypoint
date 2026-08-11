package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
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
		fmt.Printf("  Greeting Style: %s\n", displayVal(p.GreetingStyle))
		fmt.Printf("  Sign-Off:       %s\n", displayVal(p.SignOff))

		// Parse JSON array fields for display
		fmt.Printf("  Skills:         %s\n", displayJSONList(p.Skills))
		fmt.Printf("  Education:      %s\n", displayJSONList(p.Education))
		fmt.Printf("  Experience:     %s\n", displayJSONList(p.Experience))

		fmt.Println()
		return nil
	},
}

// --- set ---

var profileSetFlags struct {
	name          string
	email         string
	phone         string
	title         string
	skills        string
	experience    string
	education     string
	industry      string
	greetingStyle string
	signOff       string
}

var profileSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Update profile fields",
	Long: `Update one or more profile fields. Only the flags you provide are changed.

Skills, experience, and education take a comma-separated list (shell-safe)
or a JSON array:
  --skills "Go,React,Python"
  --education "BS Computer Science - MIT"
  --experience "5 years backend development"

Greeting style options: formal, casual, creative

Examples:
  waypoint profile set --name "Jane Doe" --title "Senior Engineer"
  waypoint profile set --skills "Go,React,AWS"
  waypoint profile set --email "jane@example.com" --phone "+1-555-0123"
  waypoint profile set --greeting-style casual --sign-off "Cheers"`,
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
		// List fields share one input format: comma-separated or a JSON array.
		for _, f := range []struct{ flag, key string }{
			{profileSetFlags.skills, "skills"},
			{profileSetFlags.experience, "experience"},
			{profileSetFlags.education, "education"},
		} {
			if f.flag == "" {
				continue
			}
			normalized, err := parseListInput(f.flag)
			if err != nil {
				return fmt.Errorf("%s: %w", f.key, err)
			}
			updates[f.key] = normalized
		}
		if profileSetFlags.industry != "" {
			updates["industry"] = profileSetFlags.industry
		}
		if profileSetFlags.greetingStyle != "" {
			valid := map[string]bool{"formal": true, "casual": true, "creative": true}
			if !valid[profileSetFlags.greetingStyle] {
				return fmt.Errorf("greeting-style must be one of: formal, casual, creative")
			}
			updates["greeting_style"] = profileSetFlags.greetingStyle
		}
		if profileSetFlags.signOff != "" {
			updates["sign_off"] = profileSetFlags.signOff
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
			case "greeting_style":
				fmt.Println("    Greeting Style: updated")
			case "sign_off":
				fmt.Println("    Sign-Off:       updated")
			}
		}
		fmt.Println()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(profileCmd)
	profileCmd.AddCommand(profileShowCmd)
	profileCmd.AddCommand(profileSetCmd)

	profileSetCmd.Flags().StringVar(&profileSetFlags.name, "name", "", "Full name")
	profileSetCmd.Flags().StringVar(&profileSetFlags.email, "email", "", "Email address")
	profileSetCmd.Flags().StringVar(&profileSetFlags.phone, "phone", "", "Phone number")
	profileSetCmd.Flags().StringVar(&profileSetFlags.title, "title", "", "Professional title")
	profileSetCmd.Flags().StringVar(&profileSetFlags.skills, "skills", "", "Skills as JSON array")
	profileSetCmd.Flags().StringVar(&profileSetFlags.experience, "experience", "", "Experience as JSON array")
	profileSetCmd.Flags().StringVar(&profileSetFlags.education, "education", "", "Education as JSON array")
	profileSetCmd.Flags().StringVar(&profileSetFlags.industry, "industry", "", "Target industry")
	profileSetCmd.Flags().StringVar(&profileSetFlags.greetingStyle, "greeting-style", "", "Greeting style (formal, casual, creative)")
	profileSetCmd.Flags().StringVar(&profileSetFlags.signOff, "sign-off", "", "Email sign-off")
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
