package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CathalByrneGit/corncrake-cli/cmd"
	_ "github.com/CathalByrneGit/corncrake-sdk/tenants/cso"
)

// validCSV has slightly varied header names to test fuzzy matching end-to-end.
const validCSV = `ppsn,emp_ref,occ_code,emp_type,gross_pay,basic_salary,ot_pay,basic_hrs,ot_hours,er_prsi
1234567A,EMP001,4,FULL_TIME,15250.00,14000.00,1250.00,520.0,25.0,1967.25
9876543B,EMP002,2,PART_TIME,8000.00,8000.00,0.00,312.0,0,1032.00
`

func writeTemp(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMap_GeneratesMappingFile(t *testing.T) {
	dir := t.TempDir()
	csv  := writeTemp(t, dir, "payroll.csv", validCSV)
	out  := filepath.Join(dir, "mapping.json")

	err := cmd.RunMap([]string{csv, "--out", out})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal("mapping file not created:", err)
	}

	var mf struct {
		TenantID    string `json:"tenantId"`
		Assignments []struct {
			TargetField  string `json:"targetField"`
			SourceColumn string `json:"sourceColumn"`
		} `json:"assignments"`
	}
	if err := json.Unmarshal(data, &mf); err != nil {
		t.Fatal("invalid mapping JSON:", err)
	}

	if mf.TenantID != "cso-ehecs" {
		t.Errorf("unexpected tenantId: %s", mf.TenantID)
	}
	if len(mf.Assignments) == 0 {
		t.Error("expected assignments in mapping file")
	}

	// Verify PPSN was matched to "ppsn" column
	for _, a := range mf.Assignments {
		if a.TargetField == "PPSN" && a.SourceColumn != "ppsn" {
			t.Errorf("PPSN should map to 'ppsn', got %q", a.SourceColumn)
		}
	}
}

func TestMap_WithOverrides(t *testing.T) {
	dir := t.TempDir()
	csv := writeTemp(t, dir, "payroll.csv", validCSV)
	out := filepath.Join(dir, "mapping.json")

	err := cmd.RunMap([]string{csv, "--out", out, "--set", "GrossEarnings=gross_pay"})
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(out)
	var mf struct {
		Assignments []struct {
			TargetField  string `json:"targetField"`
			SourceColumn string `json:"sourceColumn"`
			AutoMatched  bool   `json:"autoMatched"`
		} `json:"assignments"`
	}
	json.Unmarshal(data, &mf)

	for _, a := range mf.Assignments {
		if a.TargetField == "GrossEarnings" {
			if a.AutoMatched {
				t.Error("overridden field should not be AutoMatched")
			}
			if a.SourceColumn != "gross_pay" {
				t.Errorf("expected gross_pay, got %s", a.SourceColumn)
			}
		}
	}
}

func TestValidate_PassesValidCSV(t *testing.T) {
	dir     := t.TempDir()
	csv     := writeTemp(t, dir, "payroll.csv", validCSV)
	mapping := filepath.Join(dir, "mapping.json")

	// First generate mapping
	cmd.RunMap([]string{csv, "--out", mapping})

	err := cmd.RunValidate([]string{
		csv, "--mapping", mapping,
		"--holding", "CSO123456",
		"--quarter", "1",
		"--year", "2026",
	})
	if err != nil {
		t.Errorf("expected valid CSV to pass, got: %v", err)
	}
}

func TestValidate_FailsInvalidCSV(t *testing.T) {
	dir := t.TempDir()
	badCSV := `ppsn,emp_ref,occ_code,emp_type,gross_pay,basic_salary,ot_pay,basic_hrs,ot_hours,er_prsi
BADPPSN,EMP001,4,FULL_TIME,100.00,100.00,500.00,520.0,0,10.00
`
	csv     := writeTemp(t, dir, "bad.csv", badCSV)
	mapping := filepath.Join(dir, "mapping.json")
	cmd.RunMap([]string{csv, "--out", mapping})

	err := cmd.RunValidate([]string{
		csv, "--mapping", mapping,
		"--holding", "CSO123456",
		"--quarter", "1",
		"--year", "2026",
	})
	if err == nil {
		t.Error("expected RunValidate to return an error for invalid CSV")
	}
}

func TestExport_ProducesXML(t *testing.T) {
	dir     := t.TempDir()
	csv     := writeTemp(t, dir, "payroll.csv", validCSV)
	mapping := filepath.Join(dir, "mapping.json")
	outXML  := filepath.Join(dir, "out.xml")

	cmd.RunMap([]string{csv, "--out", mapping})

	err := cmd.RunExport([]string{
		csv, "--mapping", mapping,
		"--holding", "CSO123456",
		"--quarter", "1",
		"--year", "2026",
		"--out", outXML,
	})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(outXML)
	if err != nil {
		t.Fatal("output XML not created:", err)
	}

	xmlStr := string(data)
	if !strings.Contains(xmlStr, `xmlns="http://www.cso.ie/ehecs/schema/v5"`) {
		t.Error("XML missing EHECS v5 namespace")
	}
	if !strings.Contains(xmlStr, "<PPSN>1234567A</PPSN>") {
		t.Error("XML missing PPSN")
	}
	if !strings.Contains(xmlStr, "CSO123456") {
		t.Error("XML missing holding number")
	}
}

func TestTenants_ListsCSO(t *testing.T) {
	// Just verify it doesn't error
	if err := cmd.RunTenants([]string{}); err != nil {
		t.Fatal(err)
	}
}

func TestSubmit_XMLOutput(t *testing.T) {
	dir     := t.TempDir()
	csv     := writeTemp(t, dir, "payroll.csv", validCSV)
	mapping := filepath.Join(dir, "mapping.json")
	outXML  := filepath.Join(dir, "submit_out.xml")

	cmd.RunMap([]string{csv, "--out", mapping})

	err := cmd.RunSubmit([]string{
		csv, "--mapping", mapping,
		"--holding", "CSO123456",
		"--quarter", "1",
		"--year", "2026",
		"--xml",
		"--xml-out", outXML,
	})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(outXML)
	if err != nil {
		t.Fatal("XML file not created:", err)
	}

	xml := string(data)
	if !strings.Contains(xml, `xmlns="http://www.cso.ie/ehecs/schema/v5"`) {
		t.Error("expected EHECS v5 namespace in output")
	}
	if !strings.Contains(xml, "CSO123456") {
		t.Error("expected holding number in XML")
	}
	if !strings.Contains(xml, "1234567A") {
		t.Error("expected PPSN in XML")
	}
}

func TestSubmit_XMLWithoutToken(t *testing.T) {
	dir     := t.TempDir()
	csv     := writeTemp(t, dir, "payroll.csv", validCSV)
	mapping := filepath.Join(dir, "mapping.json")
	cmd.RunMap([]string{csv, "--out", mapping})

	// --xml should not require a token
	err := cmd.RunSubmit([]string{
		csv, "--mapping", mapping,
		"--holding", "CSO123456",
		"--quarter", "1", "--year", "2026",
		"--xml", "--xml-out", "-",
	})
	if err != nil {
		t.Errorf("--xml should not require a token, got: %v", err)
	}
}

func TestSubmit_DryRunRequiresNoToken(t *testing.T) {
	dir     := t.TempDir()
	csv     := writeTemp(t, dir, "payroll.csv", validCSV)
	mapping := filepath.Join(dir, "mapping.json")
	cmd.RunMap([]string{csv, "--out", mapping})

	err := cmd.RunSubmit([]string{
		csv, "--mapping", mapping,
		"--holding", "CSO123456",
		"--quarter", "1", "--year", "2026",
		"--dry-run",
	})
	if err != nil {
		t.Errorf("--dry-run should not require a token, got: %v", err)
	}
}
