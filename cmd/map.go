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

// RunMap implements: corncrake-cli map <file> [flags]
func RunMap(args []string) error {
	fs := pflag.NewFlagSet("map", pflag.ContinueOnError)
	tenantID    := fs.String("tenant", "cso-ehecs", "Tenant scheme ID")
	outFile     := fs.String("out", "mapping.json", "Output mapping file path")
	overrides   := fs.StringArray("set", nil, "Override: TargetField=SourceColumn (repeatable)")
	interactive := fs.Bool("interactive", false, "Launch interactive TUI mapper")
	fs.Usage = func() {
		fmt.Print(`Usage: corncrake-cli map <file.csv> [flags]

Auto-maps source CSV columns to EHECS fields using Jaro-Winkler similarity.
Saves a mapping.json you can review and pass to validate/export/submit.

FLAGS
`)
		fs.PrintDefaults()
		fmt.Print(`
EXAMPLES
  corncrake-cli map payroll.csv
  corncrake-cli map payroll.csv --interactive
  corncrake-cli map payroll.csv --set "GrossEarnings=total_gross_pay"
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

	cfg := tenant.Get(*tenantID)
	if cfg == nil {
		return fmt.Errorf("unknown tenant %q — run 'corncrake-cli tenants' to list available tenants", *tenantID)
	}

	f, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("opening %s: %w", file, err)
	}
	defer f.Close()

	plan, err := ehecs.AutoMap(f, *tenantID)
	if err != nil {
		return err
	}
	plan.SourceFile = file

	if len(*overrides) > 0 {
		ehecs.ApplyOverrides(plan, parseOverrides(*overrides))
	}

	// ── Interactive TUI path ──────────────────────────────────────────────────
	if *interactive {
		model := tui.NewMapModel(plan, cfg, *outFile)
		p := tea.NewProgram(model, tea.WithAltScreen())
		final, err := p.Run()
		if err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}
		result := final.(tui.MapModel).Result()
		if !result.Saved {
			printWarn("Mapping not saved (quit without saving).")
			return nil
		}
		plan = result.Plan
		if err := saveMappingFile(plan, result.OutFile); err != nil {
			return fmt.Errorf("saving mapping file: %w", err)
		}
		printOK(fmt.Sprintf("Mapping saved to %s", result.OutFile))
		return nil
	}

	// ── Non-interactive (existing behaviour) ──────────────────────────────────
	printHeader(fmt.Sprintf("Column mapping — %s", *tenantID))
	fmt.Println()

	for _, field := range cfg.FieldSchema {
		var assigned *string
		var score float64
		var auto bool
		for _, a := range plan.Assignments {
			if a.TargetField == field.ID {
				assigned = &a.SourceColumn
				score = a.Score
				auto = a.AutoMatched
				break
			}
		}

		required := ""
		if field.Required {
			required = colour(colRed, " [required]")
		}

		if assigned != nil {
			matchType := "manual"
			if auto {
				matchType = fmt.Sprintf("auto %.0f%%", score*100)
			}
			printOK(fmt.Sprintf("%-20s ← %-25s (%s)%s",
				field.ID, *assigned, matchType, required))
		} else {
			if field.Required {
				printErr(fmt.Sprintf("%-20s ← %-25s%s", field.ID, "(unmapped)", required))
			} else {
				printInfo(fmt.Sprintf("  %-20s ← (unmapped — optional)", field.ID))
			}
		}
	}

	fmt.Println()
	if err := saveMappingFile(plan, *outFile); err != nil {
		return fmt.Errorf("saving mapping file: %w", err)
	}
	printOK(fmt.Sprintf("Mapping saved to %s", *outFile))

	unmapped := plan.Unmapped()
	for _, u := range unmapped {
		if fd := findField(cfg.FieldSchema, u); fd != nil && fd.Required {
			fmt.Println()
			printWarn("Some required fields are unmapped. Use --interactive or --set to fix.")
			break
		}
	}
	return nil
}

func findField(fields []tenant.FieldDef, id string) *tenant.FieldDef {
	for i := range fields {
		if fields[i].ID == id {
			return &fields[i]
		}
	}
	return nil
}
