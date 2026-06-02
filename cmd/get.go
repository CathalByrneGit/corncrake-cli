package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/pflag"

	ehecs "github.com/CathalByrneGit/corncrake-sdk"
	"github.com/CathalByrneGit/corncrake-sdk/client"
	"github.com/CathalByrneGit/corncrake-sdk/formatter"
	"github.com/CathalByrneGit/corncrake-sdk/tenant"
	"github.com/CathalByrneGit/corncrake-cli/tui"
)

// RunGet implements: corncrake-cli get [flags]
//
// Default behaviour: fetches a submission and prints a clean human-readable
// summary to the terminal — metadata block + employee table.
//
// --xml:         also export a schema-compliant XML file for backwards
//                compatibility with https://lodgedata.cso.ie or archiving.
// --interactive: launch the TUI viewer (three tabs: summary / employees / xml).
func RunGet(args []string) error {
	fs := pflag.NewFlagSet("get", pflag.ContinueOnError)

	submissionID := fs.String("submission", "", "Submission ID to retrieve (required)")
	holding      := fs.String("holding", "", "CSO holding number (required)")
	taxYear      := fs.Int("year", 0, "Tax year of the submission (required)")
	quarter      := fs.Int("quarter", 0, "Quarter 1–4 (required)")
	runRef       := fs.String("run", "", "Run reference (required)")
	token        := fs.String("token", "", "Bearer JWT token (or set EHECS_TOKEN)")
	apiURL       := fs.String("api-url", "", "Override API base URL (e.g. PIT environment)")
	softwareUsed := fs.String("software", "corncrake-cli", "Software identifier")
	softwareVer  := fs.String("software-version", "1.0.0", "Software version")
	interactive  := fs.Bool("interactive", false, "Launch interactive TUI viewer")
	asXML        := fs.Bool("xml", false, "Also export submission as schema-compliant XML file")
	xmlOut       := fs.String("xml-out", "", "XML output path (default: <submissionId>.xml, - for stdout)")

	fs.Usage = func() {
		fmt.Print(`Usage: corncrake-cli get [flags]

Fetches a previously submitted EHECS return from the API.

Default:       prints a human-readable summary — metadata + employee table.
--xml:         also export a schema-compliant XML file for backwards
               compatibility with https://lodgedata.cso.ie or archiving.
--interactive: launch the TUI viewer (summary / employees / XML tabs).

FLAGS
`)
		fs.PrintDefaults()
		fmt.Print(`
EXAMPLES
  # View a submission summary
  corncrake-cli get \
    --submission 550e8400-e29b-41d4-a716-446655440000 \
    --holding CSO123456 --year 2026 --quarter 1 \
    --run RUN-2026-Q1-abc12345 --token $EHECS_TOKEN

  # Summary + save XML (backwards compat with lodgedata.cso.ie)
  corncrake-cli get --submission <id> ... --token $EHECS_TOKEN --xml

  # XML to stdout (pipe into diff or a pager)
  corncrake-cli get --submission <id> ... --token $EHECS_TOKEN \
    --xml --xml-out -

  # Interactive TUI viewer
  corncrake-cli get --submission <id> ... --token $EHECS_TOKEN \
    --interactive
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	switch {
	case *submissionID == "":
		return fmt.Errorf("--submission is required")
	case *holding == "":
		return fmt.Errorf("--holding is required")
	case *taxYear == 0:
		return fmt.Errorf("--year is required")
	case *quarter < 1 || *quarter > 4:
		return fmt.Errorf("--quarter must be 1–4")
	case *runRef == "":
		return fmt.Errorf("--run is required")
	}

	tok := *token
	if tok == "" {
		tok = os.Getenv("EHECS_TOKEN")
	}
	if tok == "" {
		return fmt.Errorf("--token or EHECS_TOKEN environment variable is required")
	}

	opts := []client.Option{}
	if *apiURL != "" {
		opts = append(opts, client.WithBaseURL(*apiURL))
	}
	c, err := ehecs.NewClient("cso-ehecs", *softwareUsed, *softwareVer, opts...)
	if err != nil {
		return err
	}

	printInfo(fmt.Sprintf("Fetching submission %s…", *submissionID))
	fmt.Println()

	result, err := c.GetSubmission(context.Background(),
		*holding, *taxYear, *quarter, *runRef, *submissionID, tok)
	if err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}

	// ── Interactive TUI ───────────────────────────────────────────────────────
	if *interactive {
		model := tui.NewGetModel(result)
		p := tea.NewProgram(model, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}
		// Fall through to XML export if --xml was also requested
		if !*asXML {
			return nil
		}
	}

	// ── Default: human-readable summary ──────────────────────────────────────
	if !*interactive {
		printGetSummary(result)
	}

	// ── XML export — opt-in via --xml ─────────────────────────────────────────
	if *asXML {
		if err := exportXML(result, *xmlOut, *submissionID); err != nil {
			return err
		}
	}

	return nil
}

// printGetSummary renders a clean terminal summary of a retrieved submission.
func printGetSummary(r *tenant.GetResult) {
	printHeader(fmt.Sprintf("Submission  %s", r.SubmissionID))
	fmt.Println()

	// Metadata block — tabwriter aligns the colon-separated KV pairs
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	kv := func(k, v string) {
		fmt.Fprintf(tw, "  %s\t%s\n", colour(colCyan, k), v)
	}

	kv("Run reference",  r.RunReference)
	kv("Holding number", r.HoldingNumber)
	kv("Period",         fmt.Sprintf("Q%d %d", r.Quarter, r.TaxYear))
	kv("Return type",    r.ReturnType)
	kv("Status",         statusColour(r.Status))
	kv("Received at",    r.ReceivedAt)
	kv("Software",       r.SoftwareUsed)
	kv("Employees",      fmt.Sprintf("%d", r.EmployeeCount))
	tw.Flush()

	// Employee table
	if r.Submission != nil && len(r.Submission.Employees) > 0 {
		fmt.Println()
		printHeader("Employees")
		fmt.Println()

		etw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(etw, "  %s\t%s\t%s\t%s\t%s\t%s\n",
			colour(colCyan, "PPSN"),
			colour(colCyan, "EMP ID"),
			colour(colCyan, "TYPE"),
			colour(colCyan, "GROSS (€)"),
			colour(colCyan, "HOURS"),
			colour(colCyan, "PRSI (€)"),
		)
		sep := strings.Repeat("─", 9)
		fmt.Fprintf(etw, "  %s\t%s\t%s\t%s\t%s\t%s\n",
			sep, sep, strings.Repeat("─", 11), sep, sep, sep)

		var totalGross, totalHours, totalPRSI float64
		for _, emp := range r.Submission.Employees {
			fmt.Fprintf(etw, "  %s\t%s\t%s\t%.2f\t%.1f\t%.2f\n",
				emp.PPSN,
				truncateStr(emp.EmploymentID, 8),
				emp.EmploymentType,
				emp.GrossEarnings,
				emp.BasicHours,
				emp.EmployerPRSI,
			)
			totalGross += emp.GrossEarnings
			totalHours += emp.BasicHours
			totalPRSI  += emp.EmployerPRSI
		}
		fmt.Fprintf(etw, "  %s\t%s\t%s\t%s\t%s\t%s\n",
			sep, sep, strings.Repeat("─", 11), sep, sep, sep)
		fmt.Fprintf(etw, "  %s\t%s\t%s\t%.2f\t%.1f\t%.2f\n",
			colour(colCyan, fmt.Sprintf("TOTAL (%d)", len(r.Submission.Employees))),
			"", "", totalGross, totalHours, totalPRSI,
		)
		etw.Flush()
	}

	// Warnings
	if len(r.Warnings) > 0 {
		fmt.Println()
		printHeader(fmt.Sprintf("Warnings (%d)", len(r.Warnings)))
		for _, w := range r.Warnings {
			printWarn(fmt.Sprintf("[%s] %s — %s", w.Code, w.Field, w.Message))
		}
	}
}

// exportXML formats the submission as EHECS XML and writes it.
// dest == "" → <submissionID>.xml
// dest == "-" → stdout
func exportXML(r *tenant.GetResult, dest, submissionID string) error {
	if r.Submission == nil {
		return fmt.Errorf("no submission data available to export")
	}

	xmlBytes, err := ehecs.Format(r.Submission)
	if err != nil {
		return fmt.Errorf("formatting to XML: %w", err)
	}

	if dest == "" {
		ext := formatter.FileExtension("cso-ehecs")
		dest = fmt.Sprintf("%s.%s", submissionID, ext)
	}

	if dest == "-" {
		fmt.Print(string(xmlBytes))
		return nil
	}

	if err := os.WriteFile(dest, xmlBytes, 0644); err != nil {
		return fmt.Errorf("writing XML to %s: %w", dest, err)
	}

	fmt.Println()
	printOK(fmt.Sprintf("XML written to %s (%d bytes)", dest, len(xmlBytes)))
	printInfo("Compatible with https://lodgedata.cso.ie manual upload")
	return nil
}

// statusColour applies terminal colour to a submission status string.
func statusColour(s string) string {
	switch s {
	case "RECEIVED", "ACCEPTED":
		return colour(colGreen, s)
	case "REJECTED", "FAILED":
		return colour(colRed, s)
	default:
		return colour(colYellow, s)
	}
}


// truncateStr shortens a string with an ellipsis if it exceeds n runes.
func truncateStr(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
