// corncrake-cli — CSO EHECS Payroll Formatter & Submission Tool
//
// A single static binary that maps, validates, formats, and submits
// EHECS payroll returns from any CSV or TSV source file.
//
// Usage:
//
//	corncrake-cli <command> [flags]
//
// Commands:
//
//	map       Auto-map source columns to EHECS fields and save a mapping file
//	validate  Validate a CSV against the EHECS schema and print a report
//	export    Convert a CSV to a schema-compliant XML file
//	submit    Submit a CSV directly to the EHECS REST API
//	tenants   List registered tenants
//
// Run corncrake-cli <command> --help for full flag documentation.
package main

import (
	"fmt"
	"os"

	// Register the CSO EHECS tenant on startup.
	_ "github.com/CathalByrneGit/corncrake-sdk/tenants/cso"

	"github.com/CathalByrneGit/corncrake-cli/cmd"
)

const version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	var err error
	switch command {
	case "map":
		err = cmd.RunMap(args)
	case "validate":
		err = cmd.RunValidate(args)
	case "export":
		err = cmd.RunExport(args)
	case "submit":
		err = cmd.RunSubmit(args)
	case "get":
		err = cmd.RunGet(args)
	case "tenants":
		err = cmd.RunTenants(args)
	case "version", "--version", "-v":
		fmt.Printf("corncrake-cli %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", command)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`corncrake-cli — CSO EHECS Payroll Formatter & Submission Tool

USAGE
  corncrake-cli <command> [flags]

COMMANDS
  map       Auto-map source columns to EHECS fields; save mapping file
  validate  Validate a CSV and print a human-readable report
  export    Convert CSV to schema-compliant XML (for manual upload)
  submit    Submit CSV directly to the EHECS REST API
  get       Fetch a submitted return from the API and save as XML
  tenants   List registered tenant schemes

Run corncrake-cli <command> --help for flag details.

EXAMPLES
  # Step 1: generate a mapping file from your CSV headers
  corncrake-cli map payroll_q1.csv --tenant cso-ehecs --out mapping.json

  # Step 2: validate before doing anything
  corncrake-cli validate payroll_q1.csv --mapping mapping.json \
    --holding CSO123456 --quarter 1 --year 2026

  # Step 3a: export to XML for manual upload
  corncrake-cli export payroll_q1.csv --mapping mapping.json \
    --holding CSO123456 --quarter 1 --year 2026 \
    --out EHECS_Q1_2026.xml

  # Step 3b: submit directly to the API
  corncrake-cli submit payroll_q1.csv --mapping mapping.json \
    --holding CSO123456 --quarter 1 --year 2026 \
    --token $EHECS_TOKEN
`)
}
