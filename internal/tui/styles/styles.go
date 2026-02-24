package styles

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors - warm orange and white palette
	Primary   = lipgloss.Color("208") // Bright orange
	Secondary = lipgloss.Color("252") // Bright gray (more legible)
	Accent    = lipgloss.Color("215") // Soft peach orange
	Success   = lipgloss.Color("114") // Brighter green
	Warning   = lipgloss.Color("214") // Golden orange
	Error     = lipgloss.Color("203") // Brighter red
	Muted     = lipgloss.Color("245") // Medium gray
	Highlight = lipgloss.Color("223") // Warm cream
	White     = lipgloss.Color("255") // Pure white

	// Series colors for multi-run comparison
	Series1 = lipgloss.Color("208") // Orange (primary)
	Series2 = lipgloss.Color("114") // Green
	Series3 = lipgloss.Color("75")  // Blue
	Series4 = lipgloss.Color("213") // Magenta/Pink
	Series5 = lipgloss.Color("222") // Yellow

	SeriesColors = []lipgloss.Color{Series1, Series2, Series3, Series4, Series5}

	// Text styles
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(White)

	Subtitle = lipgloss.NewStyle().
			Foreground(Primary)

	Label = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "0", Dark: "252"})

	Value = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "0", Dark: "255"})

	StatusRunning = lipgloss.NewStyle().
			Foreground(Success).
			Bold(true)

	StatusCompleted = lipgloss.NewStyle().
			Foreground(Secondary).
			Bold(true)

	StatusFailed = lipgloss.NewStyle().
			Foreground(Error).
			Bold(true)

	StatusCanceled = lipgloss.NewStyle().
			Foreground(Warning).
			Bold(true)

	StatusInactive = lipgloss.NewStyle().
			Foreground(lipgloss.Color("178")) // Amber - stale/no-data running

	// Box styles
	Box = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.AdaptiveColor{Light: "0", Dark: "245"}).
		Padding(0, 1)

	SelectedBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Primary).
			Padding(0, 1)

	// Header
	Header = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("232")). // Dark text on orange
		Background(Primary).
		Padding(0, 1)

	// Status bar
	StatusBar = lipgloss.NewStyle().
			Foreground(Muted).
			Padding(0, 1)

	// Help
	HelpKey = lipgloss.NewStyle().
		Foreground(Primary).
		Bold(true)

	HelpDesc = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "0", Dark: "245"})

	// Table
	TableHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(White).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(Primary)

	TableRow = lipgloss.NewStyle().
			Foreground(White)

	TableRowSelected = lipgloss.NewStyle().
				Foreground(lipgloss.Color("232")). // Dark text
				Background(Primary).
				Bold(true)

	// List item styles
	SelectedItem = lipgloss.NewStyle().
			Foreground(lipgloss.Color("232")). // Dark/black text
			Background(Primary).               // Orange background
			Bold(true)

	// Chart
	ChartLine = lipgloss.NewStyle().
			Foreground(Primary)

	ChartAxis = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "0", Dark: "245"})

	ChartTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Primary)
)

// StatusStyle returns the appropriate style for a run status
func StatusStyle(status string) lipgloss.Style {
	switch status {
	case "running":
		return StatusRunning
	case "inactive":
		return StatusInactive
	case "completed":
		return StatusCompleted
	case "failed":
		return StatusFailed
	case "canceled", "aborted":
		return StatusCanceled
	default:
		return Label
	}
}

// FormatDuration formats a duration in seconds to a human-readable string
func FormatDuration(seconds float64) string {
	if seconds < 60 {
		return fmt.Sprintf("%.1fs", seconds)
	} else if seconds < 3600 {
		return fmt.Sprintf("%.1fm", seconds/60)
	}
	return fmt.Sprintf("%.1fh", seconds/3600)
}
