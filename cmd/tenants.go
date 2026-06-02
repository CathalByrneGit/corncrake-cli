package cmd

import (
	"fmt"
	"sort"

	"github.com/spf13/pflag"

	"github.com/CathalByrneGit/corncrake-sdk/tenant"
)

// RunTenants implements: corncrake-cli tenants
// Lists all registered tenant schemes and their field schemas.
func RunTenants(args []string) error {
	fs := pflag.NewFlagSet("tenants", pflag.ContinueOnError)
	verbose := fs.Bool("verbose", false, "Show field schema for each tenant")
	fs.Usage = func() {
		fmt.Print(`Usage: corncrake-cli tenants [flags]

Lists all registered statistical reporting schemes (tenants).

FLAGS
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	ids := tenant.List()
	sort.Strings(ids)

	if len(ids) == 0 {
		fmt.Println("No tenants registered.")
		return nil
	}

	printHeader("Registered tenants")
	fmt.Println()

	for _, id := range ids {
		cfg := tenant.Get(id)
		if cfg == nil {
			continue
		}
		fmt.Printf("  %s\n", colour(colBold, id))
		printInfo(fmt.Sprintf("Name:    %s", cfg.Name))
		printInfo(fmt.Sprintf("Version: %s", cfg.Version))
		printInfo(fmt.Sprintf("API:     %s", cfg.BaseURL))

		if *verbose {
			fmt.Println()
			printInfo("Fields:")
			for _, f := range cfg.FieldSchema {
				req := ""
				if f.Required {
					req = colour(colRed, " [required]")
				}
				fmt.Printf("    %-20s %s%s\n", f.ID, f.Label, req)
			}
		}
		fmt.Println()
	}

	printInfo("To see field schema: corncrake-cli tenants --verbose")
	return nil
}
