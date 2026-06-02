package cmd

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/pflag"

	"github.com/CathalByrneGit/corncrake-cli/tui"
	ehecs "github.com/CathalByrneGit/corncrake-sdk"
	"github.com/CathalByrneGit/corncrake-sdk/client"
	"github.com/CathalByrneGit/corncrake-sdk/formatter"
	"github.com/CathalByrneGit/corncrake-sdk/tenant"
)

// RunSubmit implements: corncrake-cli submit <file> --mapping <file> [flags]
//
// Default:  validates and POSTs to the EHECS REST API.
// --xml:    validate and produce a schema-compliant XML file instead —
//
//	no token required, compatible with https://lodgedata.cso.ie
//	manual upload and local archiving.
//
// --dry-run: validate only, no submission and no XML written.
func RunSubmit(args []string) error {
	fs := pflag.NewFlagSet("submit", pflag.ContinueOnError)
	mappingFile := fs.String("mapping", "mapping.json", "Mapping file from 'corncrake-cli map'")
	holding := fs.String("holding", "", "CSO holding number (required)")
	quarter := fs.Int("quarter", 0, "Reporting quarter 1–4 (required)")
	year := fs.Int("year", 0, "Reporting year e.g. 2026 (required)")
	returnType := fs.String("return-type", "ORIGINAL", "ORIGINAL or AMENDED")
	token := fs.String("token", "", "Bearer JWT (or EHECS_TOKEN env var) — not required with --xml")
	apiURL := fs.String("api-url", "", "Override API base URL (e.g. for PIT environment)")
	softwareUsed := fs.String("software", "corncrake-cli", "Software identifier")
	softwareVer := fs.String("software-version", "1.0.0", "Software version")
	dryRun := fs.Bool("dry-run", false, "Validate only — no API call, no XML written")
	interactive := fs.Bool("interactive", false, "Launch interactive TUI submit screen")
	asXML := fs.Bool("xml", false, "Produce XML file instead of submitting to the API")
	xmlOut := fs.String("xml-out", "", "XML output path (default: EHECS_Q<q>_<year>.xml, - for stdout)")

	fs.Usage = func() {
		fmt.Print(`Usage: corncrake-cli submit <file.csv> [flags]

Validates a CSV and either submits it to the EHECS REST API (default)
or produces a schema-compliant XML file (--xml).

--xml is the backwards-compatible option: validates exactly as the API
would, then writes EHECS v5 XML for manual upload to lodgedata.cso.ie
or for local archiving. No token required.

FLAGS
`)
		fs.PrintDefaults()
		fmt.Print(`
EXAMPLES
  # Submit to API
  corncrake-cli submit payroll.csv --mapping mapping.json \
    --holding CSO123456 --quarter 1 --year 2026 --token $EHECS_TOKEN

  # Produce XML instead (manual upload or archive)
  corncrake-cli submit payroll.csv --mapping mapping.json \
    --holding CSO123456 --quarter 1 --year 2026 --xml

  # XML to a specific path
  corncrake-cli submit payroll.csv --mapping mapping.json \
    --holding CSO123456 --quarter 1 --year 2026 \
    --xml --xml-out /reports/EHECS_Q1_2026.xml

  # XML to stdout (pipe into diff or pager)
  corncrake-cli submit payroll.csv --mapping mapping.json \
    --holding CSO123456 --quarter 1 --year 2026 --xml --xml-out -

  # Validate only — no output produced
  corncrake-cli submit payroll.csv --mapping mapping.json \
    --holding CSO123456 --quarter 1 --year 2026 --dry-run
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("input file is required")
	}
	file := fs.Arg(0)

	if *holding == "" || *quarter == 0 || *year == 0 {
		return fmt.Errorf("--holding, --quarter and --year are required")
	}

	// Token only required when actually submitting to the API
	tok := *token
	if tok == "" {
		tok = os.Getenv("EHECS_TOKEN")
	}
	if tok == "" && !*dryRun && !*asXML {
		return fmt.Errorf("--token or EHECS_TOKEN is required (not needed with --xml or --dry-run)")
	}

	plan, err := loadMappingFile(*mappingFile)
	if err != nil {
		return err
	}

	f, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("opening %s: %w", file, err)
	}
	defer f.Close()

	sub, err := ehecs.ReadSubmission(f, plan,
		tenant.SubmissionMeta{SoftwareUsed: *softwareUsed, SoftwareVer: *softwareVer},
		*holding, *year, *quarter, *returnType,
	)
	if err != nil {
		return fmt.Errorf("reading CSV: %w", err)
	}

	// Validation runs in every path — including --xml and --dry-run
	result, err := ehecs.Validate(sub)
	if err != nil {
		return err
	}
	if !result.OK() {
		printValidationResult(result, len(sub.Employees))
		return fmt.Errorf("validation failed — fix errors before proceeding")
	}
	if len(result.Warnings) > 0 {
		printValidationResult(result, len(sub.Employees))
		fmt.Println()
	}

	// ── Dry run — stop here ───────────────────────────────────────────────────
	if *dryRun {
		printOK(fmt.Sprintf("Dry run — %d employee(s) validated, no output produced", len(sub.Employees)))
		return nil
	}

	// ── XML output path ───────────────────────────────────────────────────────
	if *asXML {
		return writeSubmitXML(sub, *xmlOut, *quarter, *year)
	}

	// ── API submission path ───────────────────────────────────────────────────
	opts := []client.Option{}
	resolvedURL := "https://api.cso.ie/ehecs/v1"
	if *apiURL != "" {
		opts = append(opts, client.WithBaseURL(*apiURL))
		resolvedURL = *apiURL
	}
	c, err := ehecs.NewClient(plan.TenantID, *softwareUsed, *softwareVer, opts...)
	if err != nil {
		return err
	}

	if *interactive {
		submitFn := func() (*tenant.SubmitResult, error) {
			return c.Submit(context.Background(), sub, tok)
		}
		model := tui.NewSubmitModel(
			*holding, *quarter, *year, *returnType,
			len(sub.Employees), file, resolvedURL, submitFn,
		)
		p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}
		return nil
	}

	printInfo(fmt.Sprintf("Submitting %d employee records to %s…", len(sub.Employees), resolvedURL))
	res, err := c.Submit(context.Background(), sub, tok)
	if err != nil {
		return fmt.Errorf("submission failed: %w", err)
	}
	fmt.Println()
	printOK("Submission accepted")
	printInfo(fmt.Sprintf("  Submission ID : %s", res.SubmissionID))
	printInfo(fmt.Sprintf("  Run reference : %s", res.RunReference))
	printInfo(fmt.Sprintf("  Status        : %s", res.Status))
	printInfo(fmt.Sprintf("  Employees     : %d", res.EmployeeCount))
	if len(res.Warnings) > 0 {
		fmt.Println()
		for _, w := range res.Warnings {
			printWarn(fmt.Sprintf("  [%s] %s", w.Code, w.Message))
		}
	}
	return nil
}

// writeSubmitXML formats a submission as EHECS v5 XML and writes it.
// dest == ""  → EHECS_Q<q>_<year>.xml
// dest == "-" → stdout
func writeSubmitXML(sub *tenant.Submission, dest string, quarter, year int) error {
	xmlBytes, err := ehecs.Format(sub)
	if err != nil {
		return fmt.Errorf("formatting to XML: %w", err)
	}

	if dest == "" {
		ext := formatter.FileExtension(sub.TenantID)
		dest = fmt.Sprintf("EHECS_Q%d_%d.%s", quarter, year, ext)
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
	printInfo(fmt.Sprintf("  Employees     : %d", len(sub.Employees)))
	printInfo(fmt.Sprintf("  Namespace     : http://www.cso.ie/ehecs/schema/v5"))
	printInfo(fmt.Sprintf("  Upload at     : https://lodgedata.cso.ie"))
	return nil
}
