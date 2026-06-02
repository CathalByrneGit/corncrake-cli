// Package tui implements the interactive terminal UI for corncrake-cli.
// All three screens (mapper, validator, submit confirm) share these styles.
package tui

import "github.com/charmbracelet/lipgloss"

// Colour palette — CSO brand adjacent: deep navy + teal accent.
var (
	colNavy   = lipgloss.Color("#1a3c5e")
	colTeal   = lipgloss.Color("#0ea5a0")
	colGreen  = lipgloss.Color("#2e9e6b")
	colYellow = lipgloss.Color("#e8a020")
	colRed    = lipgloss.Color("#c0392b")
	colGrey   = lipgloss.Color("#64748b")
	colLight  = lipgloss.Color("#e2e8f0")
	colWhite  = lipgloss.Color("#ffffff")
	colDim    = lipgloss.Color("#94a3b8")
)

// ── Structural styles ─────────────────────────────────────────────────────────

var (
	// TitleBar — top strip with app name and screen title
	StyleTitle = lipgloss.NewStyle().
			Background(colNavy).
			Foreground(colWhite).
			Bold(true).
			Padding(0, 2)

	StyleSubtitle = lipgloss.NewStyle().
			Background(colNavy).
			Foreground(colTeal).
			Padding(0, 2)

	// StatusBar — bottom strip with key bindings
	StyleStatusBar = lipgloss.NewStyle().
			Background(colNavy).
			Foreground(colDim).
			Padding(0, 1)

	StyleStatusKey = lipgloss.NewStyle().
			Background(colTeal).
			Foreground(colWhite).
			Bold(true).
			Padding(0, 1)

	// Panel styles
	StylePanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colNavy).
			Padding(0, 1)

	StylePanelFocused = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colTeal).
				Padding(0, 1)

	// Detail pane
	StyleDetail = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colGrey).
			Padding(1, 2)
)

// ── Row / cell styles ─────────────────────────────────────────────────────────

var (
	StyleRowSelected = lipgloss.NewStyle().
				Background(lipgloss.Color("#1e3a5f")).
				Foreground(colWhite)

	StyleRowNormal = lipgloss.NewStyle().
			Foreground(colLight)

	StyleRowDim = lipgloss.NewStyle().
			Foreground(colDim)

	// Field label in mapping table
	StyleFieldLabel = lipgloss.NewStyle().
			Foreground(colLight).
			Width(22)

	StyleFieldRequired = lipgloss.NewStyle().
				Foreground(colRed).
				Bold(true)

	StyleFieldOptional = lipgloss.NewStyle().
				Foreground(colDim)

	// Score bar colours
	StyleScoreHigh = lipgloss.NewStyle().Foreground(colGreen)
	StyleScoreMid  = lipgloss.NewStyle().Foreground(colYellow)
	StyleScoreLow  = lipgloss.NewStyle().Foreground(colRed)

	// Validation severity badges
	StyleBadgeError = lipgloss.NewStyle().
			Background(colRed).
			Foreground(colWhite).
			Bold(true).
			Padding(0, 1)

	StyleBadgeWarn = lipgloss.NewStyle().
			Background(colYellow).
			Foreground(lipgloss.Color("#1a1a1a")).
			Bold(true).
			Padding(0, 1)

	StyleBadgeOK = lipgloss.NewStyle().
			Background(colGreen).
			Foreground(colWhite).
			Bold(true).
			Padding(0, 1)

	StyleBadgeInfo = lipgloss.NewStyle().
			Background(lipgloss.Color("#1d6fa4")).
			Foreground(colWhite).
			Padding(0, 1)

	// Dropdown item
	StyleDropItem = lipgloss.NewStyle().
			Foreground(colLight).
			PaddingLeft(2)

	StyleDropItemSelected = lipgloss.NewStyle().
				Background(colTeal).
				Foreground(colWhite).
				PaddingLeft(2)

	// Spinner / progress
	StyleSpinner = lipgloss.NewStyle().Foreground(colTeal)
)

// ── Helper renderers ──────────────────────────────────────────────────────────

// ScoreBar renders a compact horizontal bar for a 0.0–1.0 score.
func ScoreBar(score float64, width int) string {
	filled := int(score * float64(width))
	bar := ""
	for i := 0; i < width; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	style := StyleScoreLow
	if score >= 0.85 {
		style = StyleScoreHigh
	} else if score >= 0.65 {
		style = StyleScoreMid
	}
	return style.Render(bar)
}

// KeyHint renders a single key binding hint for the status bar.
func KeyHint(key, desc string) string {
	return StyleStatusKey.Render(key) + StyleStatusBar.Render(" "+desc+"  ")
}
