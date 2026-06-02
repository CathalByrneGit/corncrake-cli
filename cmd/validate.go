package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/pflag"

	ehecs "github.com/CathalByrneGit/corncrake-sdk"
	"github.com/CathalByrneGit/corncrake-sdk/tenant"
	"github.com/CathalByrneGit/corncrake-cli/tui"
)

// RunValidate implements: corncrake-cli validate <file> --mapping <file> [flags]
func RunValidate(args []string) error {
	fs := pflag.NewFlagSet("validate", pflag.ContinueOnError)
	mappingFile := fs.String("mapping", "mapping.json", "Mapping file from 'corncrake-cli map'")
	holding     := fs.String("holding", "", "CSO holding number (required)")
	quarter     := fs.Int("quarter", 0, "Reporting quarter 1–4 (required)")
	year        := fs.Int("year", 0, "Reporting year e.g. 2026 (required)")
	returnType  := fs.String("return-type", "ORIGINAL", "ORIGINAL or AMENDED")
	interactive := fs.Bool("interactive", false, "Launch interactive TUI validation report")
	fs.Usage = func() {
		fmt.Print(`Usage: corncrake-cli validate <file.csv> [flags]

Validates a CSV against EHECS schema and statutory rules.
Exit code 0 = valid, 1 = errors found.

FLAGS
`)
		fs.PrintDefaults()
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
		tenant.SubmissionMeta{},
		*holding, *year, *quarter, *returnType,
	)
	if err != nil {
		return fmt.Errorf("reading CSV: %w", err)
	}

	result, err := ehecs.Validate(sub)
	if err != nil {
		return err
	}

	// ── Interactive TUI path ──────────────────────────────────────────────────
	if *interactive {
		model := tui.NewValidateModel(result, len(sub.Employees), file)
		p := tea.NewProgram(model, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}
		if !result.OK() {
			return fmt.Errorf("validation failed: %d error(s)",
				len(result.SchemaErrors)+len(result.LogicErrors))
		}
		return nil
	}

	// ── Non-interactive (existing behaviour) ──────────────────────────────────
	printHeader(fmt.Sprintf("Validating %s — %d employee(s) read", file, len(sub.Employees)))
	fmt.Println()
	printValidationResult(result, len(sub.Employees))

	if !result.OK() {
		return fmt.Errorf("validation failed: %d error(s)",
			len(result.SchemaErrors)+len(result.LogicErrors))
	}
	return nil
}
