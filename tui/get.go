package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/CathalByrneGit/corncrake-sdk/formatter"
	"github.com/CathalByrneGit/corncrake-sdk/tenant"
)

// GetModel is the bubbletea model for the submission viewer.
// It shows a metadata panel on the left and a scrollable XML preview on the right.
type GetModel struct {
	result *tenant.GetResult

	// XML content — generated once on init
	xmlContent string
	xmlErr     error

	// Tab navigation: 0 = summary, 1 = employees, 2 = xml
	activeTab int

	// Viewport for scrollable content
	vp      viewport.Model
	vpReady bool

	width  int
	height int
	done   bool
}

// NewGetModel creates the submission viewer TUI model.
func NewGetModel(result *tenant.GetResult) GetModel {
	// Pre-generate XML — cheap, done once
	var xmlContent string
	var xmlErr error
	if result.Submission != nil {
		b, err := formatter.Format(result.Submission)
		if err != nil {
			xmlErr = err
		} else {
			xmlContent = string(b)
		}
	}
	return GetModel{
		result:     result,
		xmlContent: xmlContent,
		xmlErr:     xmlErr,
		activeTab:  0,
	}
}

func (m GetModel) Init() tea.Cmd { return nil }

func (m GetModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		headerH := 6  // title + tabs + divider
		footerH := 2  // status bar
		contentH := m.height - headerH - footerH
		if contentH < 5 {
			contentH = 5
		}
		if !m.vpReady {
			m.vp = viewport.New(m.width-4, contentH)
			m.vpReady = true
		} else {
			m.vp.Width = m.width - 4
			m.vp.Height = contentH
		}
		m.vp.SetContent(m.activeContent())

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.done = true
			return m, tea.Quit

		case "1":
			m.activeTab = 0
			m.vp.SetContent(m.activeContent())
			m.vp.GotoTop()

		case "2":
			m.activeTab = 1
			m.vp.SetContent(m.activeContent())
			m.vp.GotoTop()

		case "3":
			m.activeTab = 2
			m.vp.SetContent(m.activeContent())
			m.vp.GotoTop()

		case "tab":
			m.activeTab = (m.activeTab + 1) % 3
			m.vp.SetContent(m.activeContent())
			m.vp.GotoTop()

		default:
			if m.vpReady {
				var cmd tea.Cmd
				m.vp, cmd = m.vp.Update(msg)
				return m, cmd
			}
		}
	}
	return m, nil
}

func (m GetModel) View() string {
	if m.done || !m.vpReady {
		return ""
	}

	var b strings.Builder

	// Title bar
	statusBadge := StyleBadgeOK.Render(" " + m.result.Status + " ")
	b.WriteString(StyleTitle.Render("  EHECS Submission Viewer  "))
	b.WriteString(statusBadge)
	b.WriteString(StyleSubtitle.Render(fmt.Sprintf(
		"  %s  ·  Q%d %d  ·  %d employees",
		m.result.SubmissionID[:8]+"…",
		m.result.Quarter,
		m.result.TaxYear,
		m.result.EmployeeCount,
	)))
	b.WriteString("\n")

	// Tab bar
	tabs := []string{"1 Summary", "2 Employees", "3 XML"}
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

	// Scrollable content area
	b.WriteString(m.vp.View())
	b.WriteString("\n")

	// Status bar
	b.WriteString(StyleStatusBar.Width(m.width).Render(
		KeyHint("1/2/3", "tabs") +
			KeyHint("tab", "next tab") +
			KeyHint("↑↓/pgup/pgdn", "scroll") +
			KeyHint("q", "quit"),
	))

	return b.String()
}

// activeContent returns the rendered content for the current tab.
func (m GetModel) activeContent() string {
	switch m.activeTab {
	case 0:
		return m.renderSummary()
	case 1:
		return m.renderEmployees()
	case 2:
		return m.renderXML()
	}
	return ""
}

func (m GetModel) renderSummary() string {
	var b strings.Builder
	r := m.result

	b.WriteString("\n")
	b.WriteString(renderSectionHeader("Submission Details"))
	b.WriteString(renderDetailKV("Submission ID", r.SubmissionID))
	b.WriteString(renderDetailKV("Run Reference", r.RunReference))
	b.WriteString(renderDetailKV("Holding Number", r.HoldingNumber))
	b.WriteString(renderDetailKV("Period", fmt.Sprintf("Q%d %d", r.Quarter, r.TaxYear)))
	b.WriteString(renderDetailKV("Return Type", r.ReturnType))
	b.WriteString(renderDetailKV("Status", r.Status))
	b.WriteString(renderDetailKV("Received At", r.ReceivedAt))
	b.WriteString(renderDetailKV("Software Used", r.SoftwareUsed))
	b.WriteString(renderDetailKV("Software Version", r.SoftwareVersion))
	b.WriteString(renderDetailKV("Employee Count", fmt.Sprintf("%d", r.EmployeeCount)))

	if len(r.Warnings) > 0 {
		b.WriteString("\n")
		b.WriteString(renderSectionHeader(fmt.Sprintf("Warnings (%d)", len(r.Warnings))))
		for _, w := range r.Warnings {
			b.WriteString(StyleBadgeWarn.Render(" WARN "))
			b.WriteString("  ")
			b.WriteString(StyleRowDim.Render(fmt.Sprintf("[%s] %s", w.Code, w.Message)))
			b.WriteString("\n")
		}
	} else {
		b.WriteString("\n")
		b.WriteString("  ")
		b.WriteString(StyleBadgeOK.Render(" NO WARNINGS "))
		b.WriteString("\n")
	}

	return b.String()
}

func (m GetModel) renderEmployees() string {
	if m.result.Submission == nil || len(m.result.Submission.Employees) == 0 {
		return StyleRowDim.Render("\n  No employee records available.\n")
	}

	var b strings.Builder
	b.WriteString("\n")

	// Header row
	b.WriteString(lipgloss.NewStyle().Foreground(colTeal).Bold(true).Render(
		fmt.Sprintf("  %-14s  %-10s  %-12s  %-10s  %-10s  %s",
			"PPSN", "EMP ID", "TYPE", "GROSS (€)", "HOURS", "PRSI (€)")))
	b.WriteString("\n")
	b.WriteString(StyleRowDim.Render("  " + strings.Repeat("─", 74)))
	b.WriteString("\n")

	for i, emp := range m.result.Submission.Employees {
		row := fmt.Sprintf("  %-14s  %-10s  %-12s  %10.2f  %10.1f  %10.2f",
			emp.PPSN,
			truncate(emp.EmploymentID, 8),
			emp.EmploymentType,
			emp.GrossEarnings,
			emp.BasicHours,
			emp.EmployerPRSI,
		)
		if i%2 == 0 {
			b.WriteString(StyleRowNormal.Render(row))
		} else {
			b.WriteString(StyleRowDim.Render(row))
		}
		b.WriteString("\n")
	}

	// Totals row
	var totalGross, totalHours, totalPRSI float64
	for _, emp := range m.result.Submission.Employees {
		totalGross += emp.GrossEarnings
		totalHours += emp.BasicHours
		totalPRSI += emp.EmployerPRSI
	}
	b.WriteString(StyleRowDim.Render("  " + strings.Repeat("─", 74)))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(colTeal).Render(
		fmt.Sprintf("  %-14s  %-10s  %-12s  %10.2f  %10.1f  %10.2f",
			fmt.Sprintf("TOTAL (%d)", len(m.result.Submission.Employees)),
			"", "", totalGross, totalHours, totalPRSI)))
	b.WriteString("\n")

	return b.String()
}

func (m GetModel) renderXML() string {
	if m.xmlErr != nil {
		return StyleScoreLow.Render(fmt.Sprintf("\n  Error generating XML: %v\n", m.xmlErr))
	}
	if m.xmlContent == "" {
		return StyleRowDim.Render("\n  No XML content available.\n")
	}

	var b strings.Builder
	b.WriteString("\n")

	// Syntax-colour the XML lines minimally
	for _, line := range strings.Split(m.xmlContent, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "<!--"):
			b.WriteString(StyleRowDim.Render(line))
		case strings.HasPrefix(trimmed, "</"):
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#38bdf8")).Render(line))
		case strings.HasPrefix(trimmed, "<"):
			b.WriteString(lipgloss.NewStyle().Foreground(colLight).Render(line))
		default:
			b.WriteString(lipgloss.NewStyle().Foreground(colTeal).Render(line))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// ── Shared render helpers ─────────────────────────────────────────────────────

func renderSectionHeader(s string) string {
	return lipgloss.NewStyle().
		Foreground(colTeal).Bold(true).
		Render("  "+s) + "\n" +
		StyleRowDim.Render("  "+strings.Repeat("─", len(s)+2)) + "\n"
}

func renderDetailKV(k, v string) string {
	return lipgloss.NewStyle().Foreground(colDim).Render(fmt.Sprintf("  %-20s", k)) +
		lipgloss.NewStyle().Foreground(colLight).Render(v) + "\n"
}
