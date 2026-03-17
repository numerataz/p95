package views

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ninetyfive/p95/internal/tui/messages"
	"github.com/ninetyfive/p95/internal/tui/styles"
	"github.com/ninetyfive/p95/pkg/client"
)

// RunsListModel is the runs list view model
type RunsListModel struct {
	client client.API
	width  int
	height int

	teamSlug string
	appSlug  string

	runs        []client.Run
	selectedIdx int
	loading     bool
	err         error
}

// NewRunsList creates a new runs list model
func NewRunsList(c client.API) RunsListModel {
	return RunsListModel{
		client: c,
	}
}

// SetSize sets the view dimensions
func (m RunsListModel) SetSize(width, height int) RunsListModel {
	m.width = width
	m.height = height
	return m
}

// SetContext sets the team and app context
func (m RunsListModel) SetContext(teamSlug, appSlug string) RunsListModel {
	m.teamSlug = teamSlug
	m.appSlug = appSlug
	m.runs = nil
	m.selectedIdx = 0
	return m
}

// LoadRuns returns a command to load runs
func (m RunsListModel) LoadRuns() tea.Cmd {
	return func() tea.Msg {
		runs, err := m.client.GetRuns(m.teamSlug, m.appSlug)
		if err != nil {
			return messages.ErrorMsg{Err: err}
		}
		return messages.RunsLoadedMsg{Runs: runs}
	}
}

// Update handles messages
func (m RunsListModel) Update(msg tea.Msg) (RunsListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.selectedIdx > 0 {
				m.selectedIdx--
			}
		case "down", "j":
			if m.selectedIdx < len(m.runs)-1 {
				m.selectedIdx++
			}
		case "enter":
			if len(m.runs) > 0 {
				run := m.runs[m.selectedIdx]
				return m, func() tea.Msg {
					return messages.NavigateToRunMsg{RunID: run.ID}
				}
			}
		case "r":
			// Refresh
			m.loading = true
			return m, m.LoadRuns()
		}

	case messages.RunsLoadedMsg:
		m.runs = msg.Runs
		m.loading = false
		if m.selectedIdx >= len(m.runs) {
			m.selectedIdx = max(0, len(m.runs)-1)
		}

	case messages.ErrorMsg:
		m.err = msg.Err
		m.loading = false
	}

	return m, nil
}

// View renders the runs list
func (m RunsListModel) View() string {
	if m.loading {
		return styles.Label.Render("Loading runs...")
	}

	if len(m.runs) == 0 {
		return styles.Label.Render("No runs found. Start a training run to see it here.")
	}

	var sb string

	// Header
	sb += styles.Title.Render(fmt.Sprintf("Runs in %s/%s", m.teamSlug, m.appSlug)) + "\n\n"

	// Column headers
	headerStyle := styles.TableHeader
	header := fmt.Sprintf("  %-24s %-10s %-12s %-20s",
		"Name", "Status", "Duration", "Started")
	sb += headerStyle.Render(header) + "\n"

	// Runs
	for i, run := range m.runs {
		prefix := "  "
		style := styles.TableRow
		if i == m.selectedIdx {
			prefix = "▸ "
			style = styles.TableRowSelected
		}

		// Format duration
		duration := "-"
		if run.DurationSeconds != nil {
			duration = styles.FormatDuration(*run.DurationSeconds)
		} else if run.Status == "running" {
			duration = styles.FormatDuration(time.Since(run.StartedAt).Seconds())
		}

		// Format started time
		started := run.StartedAt.Format("Jan 02 15:04")

		// Status with color
		displayStatus := run.Status
		if run.IsInactive() {
			displayStatus = "inactive"
		}
		statusStyle := styles.StatusStyle(displayStatus)
		status := statusStyle.Render(displayStatus)

		line := fmt.Sprintf("%s%-24s %-10s %-12s %-20s",
			prefix,
			truncate(run.Name, 24),
			status,
			duration,
			started,
		)
		sb += style.Render(line) + "\n"
	}

	// Help
	sb += "\n" + styles.Label.Render("Press 'r' to refresh, 'enter' to view details")

	// Render in container
	box := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Render(sb)

	return box
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
