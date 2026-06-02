package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/CathalByrneGit/corncrake-sdk/tenant"
)

// ValidateModel is the bubbletea model for the interactive validation report.
type ValidateModel struct {
	result        tenant.ValidationResult
	employeeCount int
	filename      string

	// All items flattened into one list for navigation
	items  []valItem
	cursor int

	// Filter state
	showErrors   bool
	showWarnings bool
	showPassing  bool

	// Detail panel
	detailOpen bool

	width  int
	height int
	done   bool
}

type valItemKind int

const (
	kindError valItemKind = iota
	kindWarn
	kindPass // synthetic pass items summarising passing checks
)

type valItem struct {
	kind    valItemKind
	code    string
	field   string
	message string
	detail  string // expanded statutory explanation
}

// explanations maps error/warning codes to human-readable statutory context.
var explanations = map[string]string{
	"OVERTIME_INCONSISTENCY": `Statutory rule (CSO Notes for Payroll Software Providers v4.0):

If OvertimePay is greater than zero, the employee must have worked overtime
hours in the quarter. A non-zero pay figure with zero hours suggests either:

  • The overtime hours column was not mapped correctly
  • The source system records overtime pay under a different column
  • The pay figure includes non-overtime components that should be in Allowances

Resolution: verify the OvertimeHours column mapping, or move the pay to
the Allowances field if it is not strictly overtime compensation.`,

	"EARNINGS_INCONSISTENCY": `Statutory rule (CSO Notes for Payroll Software Providers v4.0):

GrossEarnings must be greater than or equal to OvertimePay, because
GrossEarnings is defined as the total of BasicPay + OvertimePay + Allowances
+ Bonuses + ShiftPremiums.

A GrossEarnings value smaller than OvertimePay is arithmetically impossible
under the EHECS definition.

Resolution: check whether GrossEarnings in your source system is net of
deductions rather than gross. EHECS requires the pre-deduction gross figure.`,

	"HOURS_EXCEEDED": `Statutory rule (CSO Notes for Payroll Software Providers v4.0):

BasicHours is capped at 2,184 hours per quarter.
This derives from the maximum possible working week: 168 hours × 13 weeks.

Exceeding this cap typically means:
  • Hours are recorded in minutes rather than hours in the source system
  • The figure is annual hours rather than quarterly hours
  • The BasicHours column was matched to a cumulative YTD hours field

Resolution: divide by 60 if source is in minutes, or by 4 if annual.`,

	"INVALID_FORMAT": `Statutory rule (EHECS XML Schema v5):

The PPSN (Personal Public Service Number) must match the format:
  7 digits followed by 1 or 2 uppercase letters
  Examples: 1234567A  •  9876543WA

Common causes of format failures:
  • Leading/trailing whitespace in the source file
  • The field contains an internal employee ID rather than a PPSN
  • Test/placeholder data in the source system

Resolution: verify the PPSN column mapping. Use the --set flag to explicitly
assign the correct column: --set "PPSN=pps_number"`,

	"ZERO_EARNINGS": `Advisory (CSO Notes for Payroll Software Providers v4.0):

An employee with zero GrossEarnings is unusual but valid in specific cases:
  • Employee was on unpaid leave for the entire quarter
  • Employee joined the enterprise at the end of the quarter
  • Nil return for an employee who has since left

If this is not intentional, check that the GrossEarnings column is mapped
to the correct source field.`,

	"ZERO_PRSI_FULL_TIME": `Advisory (CSO Notes for Payroll Software Providers v4.0):

A full-time employee with zero EmployerPRSI is unusual. PRSI exemptions
exist but are uncommon for standard full-time employment.

Common causes:
  • EmployerPRSI column is unmapped or mapped to the wrong field
  • Employee is a proprietary director (Class S — no employer PRSI)
  • Employee is below the PRSI earnings threshold for the quarter

Verify the EmployerPRSI column mapping before submitting.`,
}

func explanationFor(code string) string {
	if e, ok := explanations[code]; ok {
		return e
	}
	return "No additional detail available for this check."
}

// NewValidateModel creates the validator TUI model from a validation result.
func NewValidateModel(result tenant.ValidationResult, employeeCount int, filename string) ValidateModel {
	m := ValidateModel{
		result:        result,
		employeeCount: employeeCount,
		filename:      filename,
		showErrors:    true,
		showWarnings:  true,
		showPassing:   true,
	}
	m.rebuildItems()
	return m
}

func (m *ValidateModel) rebuildItems() {
	m.items = nil

	if m.showErrors {
		for _, e := range m.result.SchemaErrors {
			m.items = append(m.items, valItem{
				kind: kindError, code: e.Code, field: e.Field,
				message: e.Message, detail: explanationFor(e.Code),
			})
		}
		for _, e := range m.result.LogicErrors {
			m.items = append(m.items, valItem{
				kind: kindError, code: e.Code, field: e.Field,
				message: e.Message, detail: explanationFor(e.Code),
			})
		}
	}

	if m.showWarnings {
		for _, w := range m.result.Warnings {
			m.items = append(m.items, valItem{
				kind: kindWarn, code: w.Code, field: w.Field,
				message: w.Message, detail: explanationFor(w.Code),
			})
		}
	}

	if m.showPassing && m.result.OK() {
		m.items = append(m.items, valItem{
			kind:    kindPass,
			code:    "ALL_CHECKS_PASSED",
			message: fmt.Sprintf("All validation checks passed — %d employee record(s) ready", m.employeeCount),
		})
	}
}

func (m ValidateModel) Init() tea.Cmd { return nil }

func (m ValidateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.done = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.detailOpen = false
			}

		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
				m.detailOpen = false
			}

		case "enter", " ":
			if len(m.items) > 0 {
				m.detailOpen = !m.detailOpen
			}

		case "e":
			m.showErrors = !m.showErrors
			m.rebuildItems()
			if m.cursor >= len(m.items) {
				m.cursor = max(0, len(m.items)-1)
			}

		case "w":
			m.showWarnings = !m.showWarnings
			m.rebuildItems()
			if m.cursor >= len(m.items) {
				m.cursor = max(0, len(m.items)-1)
			}

		case "esc":
			m.detailOpen = false
		}
	}
	return m, nil
}

func (m ValidateModel) View() string {
	if m.done {
		return ""
	}
	var b strings.Builder

	// Title bar
	status := "PASSED"
	statusStyle := StyleBadgeOK
	if !m.result.OK() {
		status = "FAILED"
		statusStyle = StyleBadgeError
	} else if len(m.result.Warnings) > 0 {
		status = "WARNINGS"
		statusStyle = StyleBadgeWarn
	}

	b.WriteString(StyleTitle.Render("  EHECS Validation Report  "))
	b.WriteString(statusStyle.Render(" " + status + " "))
	b.WriteString(StyleSubtitle.Render(fmt.Sprintf("  %s  ·  %d employees", m.filename, m.employeeCount)))
	b.WriteString("\n\n")

	// Summary counts
	errs := len(m.result.SchemaErrors) + len(m.result.LogicErrors)
	warns := len(m.result.Warnings)
	summary := fmt.Sprintf("  %s  %s  %s",
		StyleBadgeError.Render(fmt.Sprintf(" %d error(s) ", errs)),
		StyleBadgeWarn.Render(fmt.Sprintf(" %d warning(s) ", warns)),
		StyleBadgeOK.Render(fmt.Sprintf(" %d employee(s) ", m.employeeCount)),
	)
	b.WriteString(summary)
	b.WriteString("\n\n")

	if m.detailOpen && len(m.items) > 0 {
		b.WriteString(m.renderDetail())
	} else {
		b.WriteString(m.renderList())
	}

	b.WriteString("\n")
	b.WriteString(m.renderStatusBar())
	return b.String()
}

func (m ValidateModel) renderList() string {
	var b strings.Builder

	if len(m.items) == 0 {
		b.WriteString(StyleRowDim.Render("  No items match current filter."))
		return b.String()
	}

	for i, item := range m.items {
		badge := ""
		msg := ""
		switch item.kind {
		case kindError:
			badge = StyleBadgeError.Render(" FAIL ")
			msg = StyleScoreLow.Render(truncate(item.message, 70))
		case kindWarn:
			badge = StyleBadgeWarn.Render(" WARN ")
			msg = StyleScoreMid.Render(truncate(item.message, 70))
		case kindPass:
			badge = StyleBadgeOK.Render(" PASS ")
			msg = StyleScoreHigh.Render(item.message)
		}

		row := fmt.Sprintf("  %s  %s", badge, msg)
		if i == m.cursor {
			b.WriteString(StyleRowSelected.Render(row))
		} else {
			b.WriteString(row)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m ValidateModel) renderDetail() string {
	if m.cursor >= len(m.items) {
		return ""
	}
	item := m.items[m.cursor]

	var b strings.Builder
	w := m.width - 6
	if w < 40 {
		w = 40
	}

	badge := StyleBadgeError.Render(" FAIL ")
	if item.kind == kindWarn {
		badge = StyleBadgeWarn.Render(" WARN ")
	} else if item.kind == kindPass {
		badge = StyleBadgeOK.Render(" PASS ")
	}

	title := fmt.Sprintf("%s  %s", badge,
		lipgloss.NewStyle().Bold(true).Foreground(colLight).Render(item.code))

	content := strings.Builder{}
	content.WriteString(lipgloss.NewStyle().Foreground(colDim).Render("Field: "+item.field) + "\n\n")
	content.WriteString(lipgloss.NewStyle().Foreground(colLight).Render(item.message) + "\n\n")
	if item.detail != "" {
		content.WriteString(lipgloss.NewStyle().Foreground(colDim).Render(item.detail))
	}

	panel := StyleDetail.Width(w).Render(title + "\n\n" + content.String())
	b.WriteString(panel)
	return b.String()
}

func (m ValidateModel) renderStatusBar() string {
	errToggle := "e:errors"
	if !m.showErrors {
		errToggle = "e:errors(off)"
	}
	warnToggle := "w:warnings"
	if !m.showWarnings {
		warnToggle = "w:warnings(off)"
	}

	if m.detailOpen {
		return StyleStatusBar.Width(m.width).Render(
			KeyHint("↑↓", "navigate") +
				KeyHint("enter/esc", "close detail") +
				KeyHint("q", "quit"),
		)
	}
	return StyleStatusBar.Width(m.width).Render(
		KeyHint("↑↓", "navigate") +
			KeyHint("enter", "expand detail") +
			KeyHint(errToggle, "toggle") +
			KeyHint(warnToggle, "toggle") +
			KeyHint("q", "quit"),
	)
}
