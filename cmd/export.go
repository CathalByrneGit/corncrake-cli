package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"

	ehecs "github.com/CathalByrneGit/corncrake-sdk"
	"github.com/CathalByrneGit/corncrake-sdk/formatter"
	"github.com/CathalByrneGit/corncrake-sdk/tenant"
)

// RunExport implements: corncrake-cli export <file> --mapping <file> [flags]
//
// Validates a CSV and writes a schema-compliant output file.
// For CSO EHECS this is an XML file suitable for upload to lodgedata.cso.ie.
func RunExport(args []string) error {
	fs := pflag.NewFlagSet("export", pflag.ContinueOnError)
	mappingFile := fs.String("mapping", "mapping.json", "Mapping file from 'corncrake-cli map'")
	holding     := fs.String("holding", "", "CSO holding number (required)")
	quarter     := fs.Int("quarter", 0, "Reporting quarter 1–4 (required)")
	year        := fs.Int("year", 0, "Reporting year e.g. 2026 (required)")
	returnType  := fs.String("return-type", "ORIGINAL", "ORIGINAL or AMENDED")
	outFile     := fs.String("out", "", "Output file path (default: EHECS_Q<q>_<year>.xml)")
	force       := fs.Bool("force", false, "Write output even if validation warnings exist")
	fs.Usage = func() {
		fmt.Print(`Usage: corncrake-cli export <file.csv> [flags]

Validates a CSV and writes a schema-compliant XML file for the EHECS survey.
The output file can be uploaded manually to https://lodgedata.cso.ie

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

	printHeader(fmt.Sprintf("Validating %s — %d employee(s)", file, len(sub.Employees)))
	fmt.Println()

	result, err := ehecs.Validate(sub)
	if err != nil {
		return err
	}

	printValidationResult(result, len(sub.Employees))

	if !result.OK() {
		return fmt.Errorf("validation failed — fix errors before exporting")
	}
	if len(result.Warnings) > 0 && !*force {
		fmt.Println()
		printWarn("Warnings found. Use --force to export anyway.")
	}

	// Determine output path
	out := *outFile
	if out == "" {
		ext := formatter.FileExtension(plan.TenantID)
		out = fmt.Sprintf("EHECS_Q%d_%d.%s", *quarter, *year, ext)
	}

	data, err := ehecs.Format(sub)
	if err != nil {
		return fmt.Errorf("formatting: %w", err)
	}

	if err := os.WriteFile(out, data, 0644); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	fmt.Println()
	printOK(fmt.Sprintf("Exported %d employee records to %s", len(sub.Employees), out))
	printInfo(fmt.Sprintf("File size: %d bytes", len(data)))
	printInfo(fmt.Sprintf("Upload at: https://lodgedata.cso.ie"))

	return nil
}
