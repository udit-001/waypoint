package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SwatiBio/waypoint/internal/db"
	"github.com/SwatiBio/waypoint/internal/version"
	"github.com/spf13/cobra"
)

func defaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "jobtracker.db"
	}
	return filepath.Join(home, ".waypoint", "waypoint.db")
}

var (
	store     *db.Store
	storePath string
	jsonOut   bool
)

var rootCmd = &cobra.Command{
	Use:     "waypoint",
	Short:   "Track job applications from the command line",
	Version: version.Version,
	Long: `A CLI tool to manage and track your job applications.

Data is stored in a local SQLite database. Use 'waypoint init'
to create one, then add, list, update, and delete your job entries.

Most commands support --json for machine-readable output.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip DB connection for non-DB commands
		if cmd.Name() == "init" || cmd.Name() == "help" || cmd.Name() == "completion" || cmd.Name() == "version" {
			return nil
		}
		var err error
		store, err = db.Open(storePath)
		if err != nil {
			return fmt.Errorf("could not open database at %s: %w", storePath, err)
		}
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if store != nil {
			return store.Close()
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&storePath, "db", defaultDBPath(), "Path to SQLite database")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Output as JSON")
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// printJSON outputs a value as formatted JSON.
func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

// formatError returns a user-friendly error message.
func formatError(msg string, err error) error {
	if err != nil {
		return fmt.Errorf("%s: %w", msg, err)
	}
	return fmt.Errorf("%s", msg)
}

// truncate shortens a string for display.
func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}

// formatTable formats rows with aligned columns.
func formatTable(header []string, rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}

	// Calculate column widths
	colWidths := make([]int, len(header))
	for i, h := range header {
		colWidths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	// Cap widths at 40 for readability
	for i := range colWidths {
		if colWidths[i] > 40 {
			colWidths[i] = 40
		}
	}

	var b strings.Builder

	// Header
	for i, h := range header {
		if i > 0 {
			b.WriteString("  ")
		}
		fmt.Fprintf(&b, "%-*s", colWidths[i], h)
	}
	b.WriteString("\n")

	// Separator
	sepCount := 0
	for _, w := range colWidths {
		sepCount += w
	}
	b.WriteString(strings.Repeat("─", sepCount+2*(len(header)-1)))
	b.WriteString("\n")

	// Rows
	for _, row := range rows {
		for i, cell := range row {
			if i > 0 {
				b.WriteString("  ")
			}
			display := cell
			if len(display) > colWidths[i] {
				display = display[:colWidths[i]-3] + "..."
			}
			fmt.Fprintf(&b, "%-*s", colWidths[i], display)
		}
		b.WriteString("\n")
	}

	return b.String()
}
