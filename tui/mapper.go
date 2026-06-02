package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/CathalByrneGit/corncrake-sdk/mapper"
	"github.com/CathalByrneGit/corncrake-sdk/tenant"
)

// MapResult is returned when the mapper TUI exits.
type MapResult struct {
	Plan    *mapper.MappingPlan
	Saved   bool   // true if user confirmed with 's'
	OutFile string // path where mapping was saved
}

// MapModel is the bubbletea model for the interactive column mapper.
type MapModel struct {
	// Input
	plan    *mapper.MappingPlan
	cfg     *tenant.Config
	outFile string

	// Layout
	width  int
	height int

	// Navigation
	cursor   int  // which field row is selected
	inDrop   bool // dropdown is open
	dropIdx  int  // selected item in dropdown

	// Dropdown candidates for the current row — sorted by score desc
	dropItems []dropItem

	// Result
	done   bool
	result MapResult
}

type dropItem struct {
	col   string
	score float64
}

// NewMapModel creates the mapper TUI model.
func NewMapModel(plan *mapper.MappingPlan, cfg *tenant.Config, outFile string) MapModel {
	return MapModel{
		plan:    plan,
		cfg:     cfg,
		outFile: outFile,
		cursor:  0,
	}
}

func (m MapModel) Init() tea.Cmd { return nil }

func (m MapModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if m.inDrop {
			return m.updateDropdown(msg)
		}
		return m.updateTable(msg)
	}
	return m, nil
}

func (m MapModel) updateTable(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	fields := m.cfg.FieldSchema
	switch msg.String() {
	case "q", "ctrl+c":
		m.done = true
		m.result = MapResult{Plan: m.plan, Saved: false}
		return m, tea.Quit

	case "s":
		m.done = true
		m.result = MapResult{Plan: m.plan, Saved: true, OutFile: m.outFile}
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < len(fields)-1 {
			m.cursor++
		}

	case "enter", " ":
		// Open dropdown for this field
		m.inDrop = true
		m.dropIdx = 0
		m.dropItems = m.candidatesFor(fields[m.cursor].ID)

	case "d":
		// Clear assignment for current field
		m.removeAssignment(fields[m.cursor].ID)
	}
	return m, nil
}

func (m MapModel) updateDropdown(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.inDrop = false

	case "up", "k":
		if m.dropIdx > 0 {
			m.dropIdx--
		}

	case "down", "j":
		if m.dropIdx < len(m.dropItems) {
			m.dropIdx++
		}

	case "enter", " ":
		field := m.cfg.FieldSchema[m.cursor].ID
		if m.dropIdx == 0 {
			// Index 0 = unmapped — clear the assignment
			m.removeAssignment(field)
		} else if m.dropIdx <= len(m.dropItems) {
			// Index 1..N maps to dropItems[0..N-1]
			chosen := m.dropItems[m.dropIdx-1]
			m.setAssignment(field, chosen.col, chosen.score)
		}
		m.inDrop = false
	}
	return m, nil
}

func (m MapModel) View() string {
	if m.done {
		return ""
	}

	var b strings.Builder

	// Title bar
	title := StyleTitle.Render("  EHECS Column Mapper")
	subtitle := StyleSubtitle.Render(fmt.Sprintf("  %s  ·  %s", m.cfg.Name, m.outFile))
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, title, subtitle))
	b.WriteString("\n\n")

	if m.inDrop {
		b.WriteString(m.renderDropdown())
	} else {
		b.WriteString(m.renderTable())
	}

	// Status bar
	b.WriteString("\n")
	b.WriteString(m.renderStatusBar())

	return b.String()
}

func (m MapModel) renderTable() string {
	var b strings.Builder

	// Header
	header := lipgloss.NewStyle().Foreground(colTeal).Bold(true).Render(
		fmt.Sprintf("  %-22s  %-28s  %s  %s",
			"EHECS FIELD", "SOURCE COLUMN", "MATCH", "REQ"),
	)
	b.WriteString(header)
	b.WriteString("\n")
	b.WriteString(StyleRowDim.Render("  " + strings.Repeat("─", 72)))
	b.WriteString("\n")

	for i, field := range m.cfg.FieldSchema {
		assignment := m.assignmentFor(field.ID)

		// Field label
		label := field.ID
		req := "   "
		if field.Required {
			req = StyleFieldRequired.Render("req")
		}

		// Source column
		srcCol := StyleRowDim.Render("(unmapped)")
		scoreBar := "         "
		if assignment != nil {
			srcCol = lipgloss.NewStyle().Foreground(colLight).Render(
				fmt.Sprintf("%-28s", truncate(assignment.SourceColumn, 26)))
			scoreBar = ScoreBar(assignment.Score, 8) + fmt.Sprintf(" %2.0f%%", assignment.Score*100)
		}

		row := fmt.Sprintf("  %-22s  %s  %s  %s",
			truncate(label, 20), srcCol, scoreBar, req)

		if i == m.cursor {
			b.WriteString(StyleRowSelected.Render(row))
		} else if assignment == nil && field.Required {
			b.WriteString(StyleScoreLow.Render(row))
		} else {
			b.WriteString(StyleRowNormal.Render(row))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m MapModel) renderDropdown() string {
	field := m.cfg.FieldSchema[m.cursor]
	var b strings.Builder

	b.WriteString(lipgloss.NewStyle().
		Foreground(colTeal).Bold(true).
		Render(fmt.Sprintf("  Select source column for: %s", field.Label)))
	b.WriteString("\n")
	b.WriteString(StyleRowDim.Render("  " + strings.Repeat("─", 50)))
	b.WriteString("\n")

	// Show (unmapped) as first option
	unmapRow := "  (unmapped — clear assignment)"
	if m.dropIdx == 0 {
		b.WriteString(StyleDropItemSelected.Render(unmapRow))
	} else {
		b.WriteString(StyleDropItem.Render(unmapRow))
	}
	b.WriteString("\n")

	for i, item := range m.dropItems {
		idx := i + 1 // offset for unmapped option
		bar := ScoreBar(item.score, 8)
		row := fmt.Sprintf("  %-30s %s %2.0f%%", truncate(item.col, 28), bar, item.score*100)
		if idx == m.dropIdx {
			b.WriteString(StyleDropItemSelected.Render(row))
		} else {
			b.WriteString(StyleDropItem.Render(row))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (m MapModel) renderStatusBar() string {
	if m.inDrop {
		return StyleStatusBar.Width(m.width).Render(
			KeyHint("↑↓", "navigate") +
				KeyHint("enter", "select") +
				KeyHint("esc", "cancel"),
		)
	}
	return StyleStatusBar.Width(m.width).Render(
		KeyHint("↑↓", "navigate") +
			KeyHint("enter", "edit") +
			KeyHint("d", "clear") +
			KeyHint("s", "save & exit") +
			KeyHint("q", "quit without saving"),
	)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (m MapModel) assignmentFor(fieldID string) *mapper.ColumnMapping {
	for i := range m.plan.Assignments {
		if m.plan.Assignments[i].TargetField == fieldID {
			return &m.plan.Assignments[i]
		}
	}
	return nil
}

func (m *MapModel) setAssignment(fieldID, col string, score float64) {
	for i := range m.plan.Assignments {
		if m.plan.Assignments[i].TargetField == fieldID {
			m.plan.Assignments[i].SourceColumn = col
			m.plan.Assignments[i].Score = score
			m.plan.Assignments[i].AutoMatched = false
			return
		}
	}
	m.plan.Assignments = append(m.plan.Assignments, mapper.ColumnMapping{
		TargetField:  fieldID,
		SourceColumn: col,
		Score:        score,
		AutoMatched:  false,
	})
}

func (m *MapModel) removeAssignment(fieldID string) {
	result := m.plan.Assignments[:0]
	for _, a := range m.plan.Assignments {
		if a.TargetField != fieldID {
			result = append(result, a)
		}
	}
	m.plan.Assignments = result
}

// candidatesFor returns all source columns ranked by Jaro-Winkler score
// against the given field, for population of the dropdown.
func (m MapModel) candidatesFor(fieldID string) []dropItem {
	field := findFieldByID(m.cfg.FieldSchema, fieldID)
	items := make([]dropItem, 0, len(m.plan.Columns))
	for _, col := range m.plan.Columns {
		score := jaroWinklerScore(strings.ToLower(fieldID), strings.ToLower(col))
		if field != nil {
			for _, alias := range field.Aliases {
				s := jaroWinklerScore(strings.ToLower(alias), strings.ToLower(col))
				if s > score {
					score = s
				}
			}
		}
		items = append(items, dropItem{col: col, score: score})
	}
	// Sort by score descending
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j].score > items[j-1].score; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
	return items
}

func findFieldByID(fields []tenant.FieldDef, id string) *tenant.FieldDef {
	for i := range fields {
		if fields[i].ID == id {
			return &fields[i]
		}
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// jaroWinklerScore is a lightweight copy used for dropdown ranking.
// The full implementation lives in the SDK mapper package.
func jaroWinklerScore(s, t string) float64 {
	if s == t {
		return 1
	}
	sl, tl := len([]rune(s)), len([]rune(t))
	if sl == 0 || tl == 0 {
		return 0
	}
	maxDist := max(sl, tl)/2 - 1
	if maxDist < 0 {
		maxDist = 0
	}
	sm := make([]bool, sl)
	tm := make([]bool, tl)
	sr, tr := []rune(s), []rune(t)
	matches := 0
	for i := range sr {
		lo := max(0, i-maxDist)
		hi := min(tl-1, i+maxDist)
		for j := lo; j <= hi; j++ {
			if !tm[j] && sr[i] == tr[j] {
				sm[i], tm[j] = true, true
				matches++
				break
			}
		}
	}
	if matches == 0 {
		return 0
	}
	t2 := 0
	k := 0
	for i := range sr {
		if !sm[i] {
			continue
		}
		for !tm[k] {
			k++
		}
		if sr[i] != tr[k] {
			t2++
		}
		k++
	}
	jaro := (float64(matches)/float64(sl) +
		float64(matches)/float64(tl) +
		float64(matches-t2/2)/float64(matches)) / 3
	prefix := 0
	for i := 0; i < min(min(sl, tl), 4); i++ {
		if sr[i] == tr[i] {
			prefix++
		} else {
			break
		}
	}
	return jaro + float64(prefix)*0.1*(1-jaro)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Result returns the final state after the TUI exits.
// Call this on the model returned by tea.Program.Run().
func (m MapModel) Result() MapResult {
	return m.result
}