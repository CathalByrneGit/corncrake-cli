// Package cmd implements the corncrake-cli subcommands.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/CathalByrneGit/corncrake-sdk/mapper"
	"github.com/CathalByrneGit/corncrake-sdk/tenant"
)

// ── Shared flag helpers ───────────────────────────────────────────────────────

// commonFlags holds flags shared across map/validate/export/submit.
type commonFlags struct {
	file       string // positional: input CSV
	tenantID   string
	mappingFile string
	holding    string
	quarter    int
	year       int
	returnType string
}

func (f *commonFlags) validate() error {
	if f.file == "" {
		return fmt.Errorf("input file is required (first positional argument)")
	}
	if f.holding == "" {
		return fmt.Errorf("--holding is required")
	}
	if f.quarter < 1 || f.quarter > 4 {
		return fmt.Errorf("--quarter must be 1–4")
	}
	if f.year < 2008 || f.year > 2099 {
		return fmt.Errorf("--year must be 2008–2099")
	}
	return nil
}

// ── Mapping file I/O ──────────────────────────────────────────────────────────

// mappingFileJSON is the on-disk format for a saved mapping plan.
type mappingFileJSON struct {
	TenantID    string              `json:"tenantId"`
	SourceFile  string              `json:"sourceFile"`
	Assignments []assignmentJSON    `json:"assignments"`
}

type assignmentJSON struct {
	SourceColumn string  `json:"sourceColumn"`
	TargetField  string  `json:"targetField"`
	Score        float64 `json:"score"`
	AutoMatched  bool    `json:"autoMatched"`
}

func saveMappingFile(plan *mapper.MappingPlan, path string) error {
	mf := mappingFileJSON{
		TenantID:   plan.TenantID,
		SourceFile: plan.SourceFile,
	}
	for _, a := range plan.Assignments {
		mf.Assignments = append(mf.Assignments, assignmentJSON{
			SourceColumn: a.SourceColumn,
			TargetField:  a.TargetField,
			Score:        a.Score,
			AutoMatched:  a.AutoMatched,
		})
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(mf)
}

func loadMappingFile(path string) (*mapper.MappingPlan, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening mapping file: %w", err)
	}
	defer f.Close()

	var mf mappingFileJSON
	if err := json.NewDecoder(f).Decode(&mf); err != nil {
		return nil, fmt.Errorf("parsing mapping file: %w", err)
	}

	plan := &mapper.MappingPlan{
		TenantID:   mf.TenantID,
		SourceFile: mf.SourceFile,
	}
	for _, a := range mf.Assignments {
		plan.Assignments = append(plan.Assignments, mapper.ColumnMapping{
			SourceColumn: a.SourceColumn,
			TargetField:  a.TargetField,
			Score:        a.Score,
			AutoMatched:  a.AutoMatched,
		})
	}
	// Reconstruct column list from assignments
	seen := map[string]bool{}
	for _, a := range plan.Assignments {
		if !seen[a.SourceColumn] {
			plan.Columns = append(plan.Columns, a.SourceColumn)
			seen[a.SourceColumn] = true
		}
	}
	return plan, nil
}

// ── Output helpers ────────────────────────────────────────────────────────────

const (
	colReset  = "\033[0m"
	colRed    = "\033[31m"
	colYellow = "\033[33m"
	colGreen  = "\033[32m"
	colCyan   = "\033[36m"
	colBold   = "\033[1m"
)

func colour(c, s string) string {
	if !isTerminal() {
		return s
	}
	return c + s + colReset
}

func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func printHeader(s string) {
	fmt.Println(colour(colBold+colCyan, s))
}

func printOK(s string) {
	fmt.Println(colour(colGreen, "  ✓ "+s))
}

func printWarn(s string) {
	fmt.Println(colour(colYellow, "  ⚠ "+s))
}

func printErr(s string) {
	fmt.Println(colour(colRed, "  ✗ "+s))
}

func printInfo(s string) {
	fmt.Println("  " + s)
}

// printValidationResult prints a structured report of validation results.
func printValidationResult(result tenant.ValidationResult, employeeCount int) {
	if len(result.SchemaErrors) > 0 {
		printHeader(fmt.Sprintf("Schema errors (%d)", len(result.SchemaErrors)))
		for _, e := range result.SchemaErrors {
			printErr(fmt.Sprintf("[%s] %s — %s", e.Code, e.Field, e.Message))
		}
	}
	if len(result.LogicErrors) > 0 {
		printHeader(fmt.Sprintf("Logic errors (%d)", len(result.LogicErrors)))
		for _, e := range result.LogicErrors {
			printErr(fmt.Sprintf("[%s] %s — %s", e.Code, e.Field, e.Message))
		}
	}
	if len(result.Warnings) > 0 {
		printHeader(fmt.Sprintf("Warnings (%d)", len(result.Warnings)))
		for _, w := range result.Warnings {
			printWarn(fmt.Sprintf("[%s] %s — %s", w.Code, w.Field, w.Message))
		}
	}

	fmt.Println()
	if result.OK() {
		printOK(fmt.Sprintf("Validation passed — %d employee record(s) ready", employeeCount))
	} else {
		total := len(result.SchemaErrors) + len(result.LogicErrors)
		printErr(fmt.Sprintf("Validation failed — %d error(s), %d warning(s)",
			total, len(result.Warnings)))
	}

	// Summary line matching the statutory rules reference
	fmt.Println()
	printInfo("Checks performed:")
	printInfo("  • PPSN format (7 digits + 1–2 uppercase letters)")
	printInfo("  • OccupationCode range (1–9, CSO SOC-2010)")
	printInfo("  • BasicHours quarterly cap (2,184 hrs = 168/wk × 13 weeks)")
	printInfo("  • OvertimePay > 0 requires OvertimeHours > 0")
	printInfo("  • GrossEarnings ≥ OvertimePay")
	printInfo("  Source: CSO Notes for Payroll Software Providers v4.0")
}

// parseOverrides parses a slice of "TargetField=SourceColumn" strings.
func parseOverrides(pairs []string) map[string]string {
	out := map[string]string{}
	for _, p := range pairs {
		parts := strings.SplitN(p, "=", 2)
		if len(parts) == 2 {
			out[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return out
}
