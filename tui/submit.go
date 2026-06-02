package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/CathalByrneGit/corncrake-sdk/tenant"
)

// SubmitFunc is the function that performs the actual submission.
// It runs in a goroutine and sends a submitResultMsg when complete.
type SubmitFunc func() (*tenant.SubmitResult, error)

// SubmitModel is the bubbletea model for the submit confirmation screen.
type SubmitModel struct {
	// Submission details shown in the summary
	holdingNumber string
	quarter       int
	year          int
	returnType    string
	employeeCount int
	filename      string
	apiURL        string

	// Workflow state
	state       submitState
	spinner     spinner.Model
	submitFn    SubmitFunc
	result      *tenant.SubmitResult
	submitErr   error

	width  int
	height int
}

type submitState int

const (
	stateConfirm    submitState = iota // waiting for user to press Enter or q
	stateSubmitting                    // POST in flight — spinner shown
	stateSuccess                       // received 201
	stateError                         // received error
)

// Messages sent back to the Update loop
type submitResultMsg struct {
	result *tenant.SubmitResult
	err    error
}

// NewSubmitModel creates the submit confirmation TUI model.
func NewSubmitModel(
	holdingNumber string, quarter, year int, returnType string,
	employeeCount int, filename, apiURL string,
	submitFn SubmitFunc,
) SubmitModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = StyleSpinner

	return SubmitModel{
		holdingNumber: holdingNumber,
		quarter:       quarter,
		year:          year,
		returnType:    returnType,
		employeeCount: employeeCount,
		filename:      filename,
		apiURL:        apiURL,
		state:         stateConfirm,
		spinner:       sp,
		submitFn:      submitFn,
	}
}

func (m SubmitModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m SubmitModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch m.state {
		case stateConfirm:
			switch msg.String() {
			case "enter", "y":
				m.state = stateSubmitting
				return m, tea.Batch(m.spinner.Tick, m.doSubmit())
			case "q", "n", "ctrl+c", "esc":
				return m, tea.Quit
			}
		case stateSuccess, stateError:
			return m, tea.Quit
		}

	case submitResultMsg:
		if msg.err != nil {
			m.state = stateError
			m.submitErr = msg.err
		} else {
			m.state = stateSuccess
			m.result = msg.result
		}

	case spinner.TickMsg:
		if m.state == stateSubmitting {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m SubmitModel) doSubmit() tea.Cmd {
	return func() tea.Msg {
		result, err := m.submitFn()
		return submitResultMsg{result: result, err: err}
	}
}

func (m SubmitModel) View() string {
	var b strings.Builder

	b.WriteString(StyleTitle.Render("  EHECS Submission"))
	b.WriteString("\n\n")

	switch m.state {
	case stateConfirm:
		b.WriteString(m.renderSummary())
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().
			Foreground(colTeal).Bold(true).
			Render("  Submit this return?"))
		b.WriteString("\n\n")
		b.WriteString(StyleStatusBar.Width(m.width).Render(
			KeyHint("enter/y", "confirm & submit") +
				KeyHint("q/n", "cancel"),
		))

	case stateSubmitting:
		b.WriteString(m.renderSummary())
		b.WriteString("\n  ")
		b.WriteString(m.spinner.View())
		b.WriteString(StyleRowNormal.Render("  Submitting to " + m.apiURL + " …"))

	case stateSuccess:
		b.WriteString(m.renderSummary())
		b.WriteString("\n")
		b.WriteString(StyleBadgeOK.Render("  SUBMITTED  "))
		b.WriteString("\n\n")
		if m.result != nil {
			b.WriteString(renderKV("Submission ID", m.result.SubmissionID))
			b.WriteString(renderKV("Run reference", m.result.RunReference))
			b.WriteString(renderKV("Status", m.result.Status))
			b.WriteString(renderKV("Employees", fmt.Sprintf("%d", m.result.EmployeeCount)))
			if len(m.result.Warnings) > 0 {
				b.WriteString("\n")
				b.WriteString(StyleBadgeWarn.Render(
					fmt.Sprintf("  %d server warning(s) — run 'corncrake-cli validate' for details", len(m.result.Warnings))))
			}
		}
		b.WriteString("\n\n")
		b.WriteString(StyleStatusBar.Width(m.width).Render(KeyHint("any key", "exit")))

	case stateError:
		b.WriteString(StyleBadgeError.Render("  SUBMISSION FAILED  "))
		b.WriteString("\n\n")
		if m.submitErr != nil {
			b.WriteString(StyleScoreLow.Render("  " + m.submitErr.Error()))
		}
		b.WriteString("\n\n")
		b.WriteString(StyleRowDim.Render("  Check your token, network connection, and API URL."))
		b.WriteString("\n")
		b.WriteString(StyleStatusBar.Width(m.width).Render(KeyHint("any key", "exit")))
	}

	return b.String()
}

func (m SubmitModel) renderSummary() string {
	w := 52
	content := strings.Builder{}
	content.WriteString(renderKV("File", m.filename))
	content.WriteString(renderKV("Holding number", m.holdingNumber))
	content.WriteString(renderKV("Period", fmt.Sprintf("Q%d %d", m.quarter, m.year)))
	content.WriteString(renderKV("Return type", m.returnType))
	content.WriteString(renderKV("Employees", fmt.Sprintf("%d", m.employeeCount)))
	content.WriteString(renderKV("API", m.apiURL))
	return StylePanel.Width(w).Render(content.String())
}

func renderKV(k, v string) string {
	return lipgloss.NewStyle().Foreground(colDim).Render(fmt.Sprintf("  %-18s", k)) +
		lipgloss.NewStyle().Foreground(colLight).Render(v) + "\n"
}

// TestSubmitResultMsg is a test helper that creates a submitResultMsg for use
// in unit tests without going through the real HTTP path.
func TestSubmitResultMsg(result *tenant.SubmitResult, err error) tea.Msg {
	return submitResultMsg{result: result, err: err}
}
