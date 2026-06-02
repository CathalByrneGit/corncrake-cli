package tui_test

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/CathalByrneGit/corncrake-cli/tui"
	ehecs "github.com/CathalByrneGit/corncrake-sdk"
	"github.com/CathalByrneGit/corncrake-sdk/mapper"
	"github.com/CathalByrneGit/corncrake-sdk/tenant"
	_ "github.com/CathalByrneGit/corncrake-sdk/tenants/cso"
)

const tenantID = "cso-ehecs"

// ── MapModel tests ────────────────────────────────────────────────────────────

func TestMapModel_InitialRender(t *testing.T) {
	plan, cfg := testPlan(t)
	m := tui.NewMapModel(plan, cfg, "mapping.json")
	view := m.View()

	if !strings.Contains(view, "EHECS Column Mapper") {
		t.Error("expected title in view")
	}
	if !strings.Contains(view, "PPSN") {
		t.Error("expected PPSN field in view")
	}
	if !strings.Contains(view, "GrossEarnings") {
		t.Error("expected GrossEarnings field in view")
	}
}

func TestMapModel_NavigationDown(t *testing.T) {
	plan, cfg := testPlan(t)
	m := tui.NewMapModel(plan, cfg, "mapping.json")

	// Send down key
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = updated.(tui.MapModel)

	// Cursor should have moved — view should still render without panic
	view := m.View()
	if view == "" {
		t.Error("expected non-empty view after navigation")
	}
}

func TestMapModel_QuitWithoutSaving(t *testing.T) {
	plan, cfg := testPlan(t)
	m := tui.NewMapModel(plan, cfg, "mapping.json")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	m = updated.(tui.MapModel)

	result := m.Result()
	if result.Saved {
		t.Error("expected Saved=false after quit")
	}
}

func TestMapModel_SaveConfirm(t *testing.T) {
	plan, cfg := testPlan(t)
	m := tui.NewMapModel(plan, cfg, "/tmp/test-mapping.json")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = updated.(tui.MapModel)

	result := m.Result()
	if !result.Saved {
		t.Error("expected Saved=true after 's'")
	}
	if result.OutFile != "/tmp/test-mapping.json" {
		t.Errorf("unexpected OutFile: %s", result.OutFile)
	}
}

func TestMapModel_OpenDropdown(t *testing.T) {
	plan, cfg := testPlan(t)
	m := tui.NewMapModel(plan, cfg, "mapping.json")

	// Press enter to open dropdown
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(tui.MapModel)

	view := m.View()
	if !strings.Contains(view, "Select source column") {
		t.Error("expected dropdown to open after enter")
	}
}

func TestMapModel_CloseDropdownWithEsc(t *testing.T) {
	plan, cfg := testPlan(t)
	m := tui.NewMapModel(plan, cfg, "mapping.json")

	// Open dropdown
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(tui.MapModel)

	// Close with esc
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(tui.MapModel)

	view := m.View()
	if strings.Contains(view, "Select source column") {
		t.Error("expected dropdown to close after esc")
	}
}

// ── ValidateModel tests ───────────────────────────────────────────────────────

func TestValidateModel_PassView(t *testing.T) {
	result := tenant.ValidationResult{}
	m := tui.NewValidateModel(result, testValidateSub(5), "payroll.csv")

	view := m.View()
	if !strings.Contains(view, "Validation Report") {
		t.Error("expected report title")
	}
	if !strings.Contains(view, "PASSED") {
		t.Error("expected PASSED badge for clean result")
	}
}

func TestValidateModel_ErrorView(t *testing.T) {
	result := tenant.ValidationResult{
		LogicErrors: []tenant.ValidationItem{{
			Code:    "OVERTIME_INCONSISTENCY",
			Field:   "employees[0].overtimeHours",
			Message: "OvertimePay > 0 but OvertimeHours = 0",
		}},
	}
	m := tui.NewValidateModel(result, testValidateSub(3), "payroll.csv")
	view := m.View()

	if !strings.Contains(view, "FAILED") {
		t.Error("expected FAILED badge")
	}
	if !strings.Contains(view, "OvertimePay") {
		t.Error("expected error message in view")
	}
}

func TestValidateModel_DetailPanel(t *testing.T) {
	result := tenant.ValidationResult{
		LogicErrors: []tenant.ValidationItem{{
			Code:    "OVERTIME_INCONSISTENCY",
			Field:   "employees[0].overtimeHours",
			Message: "test error",
		}},
	}
	m := tui.NewValidateModel(result, testValidateSub(1), "payroll.csv")

	// Press enter to open detail panel
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(tui.ValidateModel)

	view := m.View()
	// Should show statutory explanation
	if !strings.Contains(view, "Statutory rule") {
		t.Error("expected statutory explanation in detail panel")
	}
}

func TestValidateModel_ToggleErrors(t *testing.T) {
	result := tenant.ValidationResult{
		LogicErrors: []tenant.ValidationItem{{
			Code: "OVERTIME_INCONSISTENCY", Message: "test",
		}},
		Warnings: []tenant.ValidationItem{{
			Code: "ZERO_EARNINGS", Message: "zero earnings",
		}},
	}
	m := tui.NewValidateModel(result, testValidateSub(1), "payroll.csv")

	// Toggle errors off with 'e'
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m = updated.(tui.ValidateModel)

	view := m.View()
	if strings.Contains(view, "OVERTIME_INCONSISTENCY") {
		t.Error("expected errors to be hidden after toggle")
	}
}

func TestValidateModel_Quit(t *testing.T) {
	result := tenant.ValidationResult{}
	m := tui.NewValidateModel(result, testValidateSub(1), "payroll.csv")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Error("expected quit command after 'q'")
	}
}

// ── SubmitModel tests ─────────────────────────────────────────────────────────

func TestSubmitModel_ConfirmView(t *testing.T) {
	m := tui.NewSubmitModel("CSO123456", 1, 2026, "ORIGINAL", 42,
		"payroll.csv", "https://api.cso.ie/ehecs/v1", nil)

	view := m.View()
	if !strings.Contains(view, "CSO123456") {
		t.Error("expected holding number in view")
	}
	if !strings.Contains(view, "42") {
		t.Error("expected employee count in view")
	}
	if !strings.Contains(view, "Q1 2026") {
		t.Error("expected period in view")
	}
}

func TestSubmitModel_CancelWithQ(t *testing.T) {
	m := tui.NewSubmitModel("CSO123456", 1, 2026, "ORIGINAL", 5,
		"payroll.csv", "https://api.cso.ie/ehecs/v1", nil)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Error("expected quit command after 'q'")
	}
}

func TestSubmitModel_SuccessView(t *testing.T) {
	called := false
	submitFn := func() (*tenant.SubmitResult, error) {
		called = true
		return &tenant.SubmitResult{
			SubmissionID:  "test-sub-id",
			RunReference:  "RUN-001",
			Status:        "RECEIVED",
			EmployeeCount: 5,
		}, nil
	}
	m := tui.NewSubmitModel("CSO123456", 1, 2026, "ORIGINAL", 5,
		"payroll.csv", "https://api.cso.ie/ehecs/v1", submitFn)

	// Simulate the result coming back
	updated, _ := m.Update(tui.TestSubmitResultMsg(
		&tenant.SubmitResult{
			SubmissionID: "test-sub-id", RunReference: "RUN-001",
			Status: "RECEIVED", EmployeeCount: 5,
		}, nil))
	_ = called
	m = updated.(tui.SubmitModel)

	view := m.View()
	if !strings.Contains(view, "test-sub-id") {
		t.Error("expected submission ID in success view")
	}
}

// ── Test helpers ──────────────────────────────────────────────────────────────

func testPlan(t *testing.T) (*mapper.MappingPlan, *tenant.Config) {
	t.Helper()
	csv := `ppsn,emp_ref,occ_code,emp_type,gross_pay,basic_salary,basic_hrs,er_prsi
1234567A,EMP001,4,FULL_TIME,15250.00,14000.00,520.0,1967.25
`
	plan, err := ehecs.AutoMap(strings.NewReader(csv), tenantID)
	if err != nil {
		t.Fatal(err)
	}
	plan.SourceFile = "test.csv"
	cfg := tenant.Get(tenantID)
	return plan, cfg
}

// ── GetModel tests ────────────────────────────────────────────────────────────

func TestGetModel_SummaryTab(t *testing.T) {
	m := tui.NewGetModel(testGetResult())
	// Simulate a window size message so the viewport initialises
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(tui.GetModel)

	view := m.View()
	if !strings.Contains(view, "EHECS Submission Viewer") {
		t.Error("expected title in view")
	}
	if !strings.Contains(view, "CSO123456") {
		t.Error("expected holding number in summary tab")
	}
	if !strings.Contains(view, "RECEIVED") {
		t.Error("expected status in summary tab")
	}
}

func TestGetModel_EmployeesTab(t *testing.T) {
	m := tui.NewGetModel(testGetResult())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(tui.GetModel)

	// Switch to employees tab with "2"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	m = updated.(tui.GetModel)

	view := m.View()
	if !strings.Contains(view, "1234567A") {
		t.Error("expected PPSN in employees tab")
	}
	if !strings.Contains(view, "FULL_TIME") {
		t.Error("expected employment type in employees tab")
	}
}

func TestGetModel_XMLTab(t *testing.T) {
	m := tui.NewGetModel(testGetResult())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(tui.GetModel)

	// Switch to XML tab with "3"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	m = updated.(tui.GetModel)

	view := m.View()
	if !strings.Contains(view, "EHECSReturn") {
		t.Error("expected XML content in XML tab")
	}
	if !strings.Contains(view, "cso.ie/ehecs/schema") {
		t.Error("expected EHECS namespace in XML tab")
	}
}

func TestGetModel_TabCycle(t *testing.T) {
	m := tui.NewGetModel(testGetResult())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(tui.GetModel)

	// Tab key cycles through tabs
	for i := 0; i < 3; i++ {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = updated.(tui.GetModel)
	}
	view := m.View()
	// After 3 tabs we should be back at summary (tab 0)
	if !strings.Contains(view, "CSO123456") {
		t.Error("expected to cycle back to summary tab")
	}
}

func TestGetModel_Quit(t *testing.T) {
	m := tui.NewGetModel(testGetResult())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(tui.GetModel)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Error("expected quit command after 'q'")
	}
}

func testGetResult() *tenant.GetResult {
	sub := &tenant.Submission{
		TenantID:      tenantID,
		HoldingNumber: "CSO123456",
		TaxYear:       2026,
		Quarter:       1,
		ReturnType:    "ORIGINAL",
		Employees: []tenant.EmployeeRecord{{
			PPSN:           "1234567A",
			EmploymentID:   "EMP001",
			OccupationCode: 4,
			EmploymentType: "FULL_TIME",
			GrossEarnings:  15250.00,
			BasicPay:       14000.00,
			OvertimePay:    1250.00,
			BasicHours:     520.0,
			OvertimeHours:  25.0,
			EmployerPRSI:   1967.25,
		}},
	}
	return &tenant.GetResult{
		SubmissionID:  "550e8400-e29b-41d4-a716-446655440000",
		RunReference:  "RUN-2026-Q1-abc123",
		HoldingNumber: "CSO123456",
		TaxYear:       2026,
		Quarter:       1,
		ReturnType:    "ORIGINAL",
		Status:        "RECEIVED",
		ReceivedAt:    "2026-04-14T10:23:11Z",
		SoftwareUsed:  "corncrake-cli",
		EmployeeCount: 1,
		Submission:    sub,
	}
}

// testValidateSub creates a minimal Submission with n employees for validator TUI tests.
func testValidateSub(n int) *tenant.Submission {
	sub := &tenant.Submission{
		TenantID:      tenantID,
		HoldingNumber: "CSO123456",
		TaxYear:       2026,
		Quarter:       1,
		ReturnType:    "ORIGINAL",
	}
	for i := 0; i < n; i++ {
		sub.Employees = append(sub.Employees, tenant.EmployeeRecord{
			PPSN:           fmt.Sprintf("%07dA", i+1234567),
			EmploymentID:   fmt.Sprintf("EMP%03d", i+1),
			OccupationCode: 4, EmploymentType: "FULL_TIME",
			GrossEarnings: 15000, BasicPay: 15000, BasicHours: 520, EmployerPRSI: 1935,
		})
	}
	return sub
}
