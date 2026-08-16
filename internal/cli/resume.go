package cli

import (
	"github.com/spf13/cobra"
	"github.com/udit-001/waypoint/internal/resume"
)

// resumeCmd groups resume-text commands. These commands are written for
// agent consumers: output is always JSON (no --json flag needed).
var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Extract and redact resume text for model use",
	Long: `Extract text from resume PDFs and redact contact information.

The extraction backends (pdf_oxide via a downloaded native library, or the
pdftotext fallback) and the redaction engine (email + phone) live behind
'waypoint resume'. Output is always JSON on stdout so agents can parse and
verify it directly.

NOTE: unlike other commands, --json is accepted as a no-op — resume output
is always JSON.`,
}

var resumeExtractFlags struct {
	noRedact bool
}

var resumeExtractCmd = &cobra.Command{
	Use:   "extract <file.pdf>",
	Short: "Extract PDF text with contact info redacted (default)",
	Long: `Extract text from a resume PDF, redacting email addresses and
phone numbers by default (fail-safe: forgetting --no-redact never leaks).

Redaction is deterministic: email via structural regex; phone via
libphonenumber-validated candidates. Detected spans are reported in the
JSON output so the caller can verify nothing leaked.

Use --no-redact only for the user's own eyes / debugging.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		res, err := resume.Extract(cmd.Context(), args[0], resume.Options{
			NoRedact: resumeExtractFlags.noRedact,
		})
		if err != nil {
			printJSON(struct {
				OK    bool   `json:"ok"`
				Error string `json:"error"`
			}{false, err.Error()})
			return err
		}
		printJSON(struct {
			OK bool `json:"ok"`
			resume.Result
		}{true, res})
		return nil
	},
}

var resumeDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Report which extraction backends are available",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		d := resume.Doctor()
		printJSON(struct {
			OK bool `json:"ok"`
			resume.DoctorReport
		}{true, d})
		return nil
	},
}

func init() {
	// --json is accepted for uniformity with the rest of the CLI (AGENTS.md
	// convention "all commands accept --json"); output is JSON regardless.
	resumeCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Output as JSON (resume commands are always JSON; accepted for uniformity)")
	resumeExtractCmd.Flags().BoolVar(&resumeExtractFlags.noRedact, "no-redact", false,
		"Output raw text (user's own eyes only; never before a model)")
	resumeCmd.AddCommand(resumeExtractCmd, resumeDoctorCmd)
	rootCmd.AddCommand(resumeCmd)
}
