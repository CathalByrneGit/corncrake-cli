package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/CathalByrneGit/corncrake-sdk/tenant"
)

// ValidateModel is the bubbletea model for the interactive validation report.
// Tab 1: validation results. Tab 2: data table preview.
type ValidateModel struct {
	result        tenant.ValidationResult
	submission    *tenant.Submission
	employeeCount int
	filename      string

	// Tab navigation: 0 = validation, 1 = data table
	activeTab int

	// Validation tab state
	items        []valItem
	cursor       int
	showErrors   bool
	showWarnings bool
	showPassing  bool
	detailOpen   bool

	// Data table tab state
	dataTable  table.Model
	tableReady bool

	width  int
	height int
	done   bool
}

type valItemKind int

const (
	kindError valItemKind = iota
	kindWarn
	kindPass
)

type valItem struct {
	kind    valItemKind
	code    string
	field   string
	message string
	detail  string
}

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

// NewValidateModel creates the validator TUI model.
// Pass the full Submission so the data table tab can render employee rows.
func NewValidateModel(result tenant.ValidationResult, sub *tenant.Submission, filename string) ValidateModel {
	m := ValidateModel{
		result:        result,
		submission:    sub,
		employeeCount: len(sub.Employees),
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
			m.items = append(m.items, valItem{kind: kindError, code: e.Code, field: e.Field, message: e.Message, detail: explanationFor(e.Code)})
		}
		for _, e := range m.result.LogicErrors {
			m.items = append(m.items, valItem{kind: kindError, code: e.Code, field: e.Field, message: e.Message, detail: explanationFor(e.Code)})
		}
	}
	if m.showWarnings {
		for _, w := range m.result.Warnings {
			m.items = append(m.items, valItem{kind: kindWarn, code: w.Code, field: w.Field, message: w.Message, detail: explanationFor(w.Code)})
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

// buildDataTable constructs the bubbles table from the submission employees.
func (m *ValidateModel) buildDataTable() {
	if m.submission == nil || len(m.submission.Employees) == 0 {
		return
	}

	w := m.width
	if w < 60 {
		w = 120
	}

	cols := []table.Column{
		{Title: "PPSN", Width: 12},
		{Title: "Emp ID", Width: 8},
		{Title: "Type", Width: 11},
		{Title: "Gross (€)", Width: 10},
		{Title: "Basic (€)", Width: 10},
		{Title: "OT Pay (€)", Width: 10},
		{Title: "Hrs", Width: 7},
		{Title: "OT Hrs", Width: 7},
		{Title: "PRSI (€)", Width: 10},
	}

	rows := make([]table.Row, len(m.submission.Employees))
	for i, emp := range m.submission.Employees {
		rows[i] = table.Row{
			emp.PPSN,
			truncate(emp.EmploymentID, 6),
			emp.EmploymentType,
			fmt.Sprintf("%.2f", emp.GrossEarnings),
			fmt.Sprintf("%.2f", emp.BasicPay),
			fmt.Sprintf("%.2f", emp.OvertimePay),
			fmt.Sprintf("%.1f", emp.BasicHours),
			fmt.Sprintf("%.1f", emp.OvertimeHours),
			fmt.Sprintf("%.2f", emp.EmployerPRSI),
		}
	}

	t := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(max(5, m.height-8)),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colNavy).
		BorderBottom(true).
		Bold(true).
		Foreground(colTeal)
	s.Selected = s.Selected.
		Foreground(colWhite).
		Background(lipgloss.Color("#1e3a5f")).
		Bold(false)
	t.SetStyles(s)

	m.dataTable = t
	m.tableReady = true
}

func (m ValidateModel) Init() tea.Cmd { return nil }

func (m ValidateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Always invalidate so the table rebuilds at the new size
		m.tableReady = false
		if m.activeTab == 1 {
			m.buildDataTable()
		}

	case tea.KeyMsg:
		// Global keys
		switch msg.String() {
		case "q", "ctrl+c":
			m.done = true
			return m, tea.Quit
		case "tab":
			m.activeTab = (m.activeTab + 1) % 2
			if m.activeTab == 1 {
				if !m.tableReady {
					m.buildDataTable()
				}
				m.dataTable.Focus()
			} else {
				m.dataTable.Blur()
			}
			return m, nil
		case "1":
			m.activeTab = 0
			if m.tableReady {
				m.dataTable.Blur()
			}
			return m, nil
		case "2":
			m.activeTab = 1
			if !m.tableReady {
				m.buildDataTable()
			}
			m.dataTable.Focus()
			return m, nil
		}

		// Tab-specific keys
		if m.activeTab == 1 {
			var cmd tea.Cmd
			m.dataTable, cmd = m.dataTable.Update(msg)
			return m, cmd
		}

		// Validation tab keys
		switch msg.String() {
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
	if m.width > 0 && m.width < 60 {
		return "\n  Terminal too narrow — widen the window.\n"
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

	b.WriteString(StyleTitle.Render("  Validation Report  "))
	b.WriteString(statusStyle.Render(" " + status + " "))
	b.WriteString(StyleSubtitle.Render(fmt.Sprintf("  %s  ·  %d employees", m.filename, m.employeeCount)))
	b.WriteString("\n")

	// Tab bar
	tabs := []string{"1 Validation", "2 Data preview"}
	for i, tab := range tabs {
		if i == m.activeTab {
			b.WriteString(lipgloss.NewStyle().
				Background(colTeal).Foreground(colWhite).
				Bold(true).Padding(0, 2).Render(tab))
		} else {
			b.WriteString(lipgloss.NewStyle().
				Foreground(colDim).Padding(0, 2).Render(tab))
		}
		b.WriteString(" ")
	}
	b.WriteString("\n")
	b.WriteString(StyleRowDim.Render(strings.Repeat("─", m.width)))
	b.WriteString("\n")

	if m.activeTab == 1 {
		b.WriteString(m.renderDataTable())
	} else {
		b.WriteString(m.renderValidation())
	}

	b.WriteString("\n")
	b.WriteString(m.renderStatusBar())
	return b.String()
}

func (m ValidateModel) renderValidation() string {
	var b strings.Builder

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
	return b.String()
}

func (m ValidateModel) renderDataTable() string {
	if m.submission == nil || len(m.submission.Employees) == 0 {
		return StyleRowDim.Render("\n  No employee data available.\n")
	}
	if !m.tableReady || m.activeTab != 1 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(m.dataTable.View())
	return b.String()
}

func (m ValidateModel) renderList() string {
	var b strings.Builder
	if len(m.items) == 0 {
		b.WriteString(StyleRowDim.Render("  No items match current filter."))
		return b.String()
	}
	for i, item := range m.items {
		badge, msg := "", ""
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
	if m.activeTab == 1 {
		return StyleStatusBar.Width(m.width).Render(
			KeyHint("↑↓", "navigate") +
				KeyHint("tab/1/2", "switch tabs") +
				KeyHint("q", "quit"),
		)
	}

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
				KeyHint("tab/2", "data preview") +
				KeyHint("q", "quit"),
		)
	}
	return StyleStatusBar.Width(m.width).Render(
		KeyHint("↑↓", "navigate") +
			KeyHint("enter", "expand detail") +
			KeyHint(errToggle, "toggle") +
			KeyHint(warnToggle, "toggle") +
			KeyHint("tab/2", "data preview") +
			KeyHint("q", "quit"),
	)
}
