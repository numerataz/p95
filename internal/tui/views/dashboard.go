package views

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"sixtyseven/internal/tui/messages"
	"sixtyseven/internal/tui/styles"
	"sixtyseven/pkg/client"
)

// FocusPanel represents which panel has focus
type FocusPanel int

const (
	FocusTeams FocusPanel = iota
	FocusApps
)

// DashboardModel is the dashboard view model
type DashboardModel struct {
	client *client.Client
	width  int
	height int

	teams       []client.Team
	selectedIdx int
	loading     bool
	err         error

	// Apps for selected team
	apps   []client.App
	appIdx int

	// Focus state
	focus FocusPanel
}

// NewDashboard creates a new dashboard model
func NewDashboard(c *client.Client) DashboardModel {
	return DashboardModel{
		client: c,
		focus:  FocusTeams,
	}
}

// Init initializes the dashboard
func (m DashboardModel) Init() tea.Cmd {
	return m.loadTeams
}

// SetSize sets the view dimensions
func (m DashboardModel) SetSize(width, height int) DashboardModel {
	m.width = width
	m.height = height
	return m
}

// loadTeams loads the teams from the API
func (m DashboardModel) loadTeams() tea.Msg {
	teams, err := m.client.GetTeams()
	if err != nil {
		return messages.ErrorMsg{Err: err}
	}
	return messages.TeamsLoadedMsg{Teams: teams}
}

// loadApps loads apps for a team
func (m DashboardModel) loadApps(teamSlug string) tea.Cmd {
	return func() tea.Msg {
		apps, err := m.client.GetApps(teamSlug)
		if err != nil {
			return messages.ErrorMsg{Err: err}
		}
		return messages.AppsLoadedMsg{Apps: apps}
	}
}

// Update handles messages
func (m DashboardModel) Update(msg tea.Msg) (DashboardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.focus == FocusTeams {
				if m.selectedIdx > 0 {
					m.selectedIdx--
					m.appIdx = 0
					if len(m.teams) > 0 {
						return m, m.loadApps(m.teams[m.selectedIdx].Slug)
					}
				}
			} else if m.focus == FocusApps {
				if m.appIdx > 0 {
					m.appIdx--
				}
			}
		case "down", "j":
			if m.focus == FocusTeams {
				if m.selectedIdx < len(m.teams)-1 {
					m.selectedIdx++
					m.appIdx = 0
					if len(m.teams) > 0 {
						return m, m.loadApps(m.teams[m.selectedIdx].Slug)
					}
				}
			} else if m.focus == FocusApps {
				if m.appIdx < len(m.apps)-1 {
					m.appIdx++
				}
			}
		case "left", "h":
			if m.focus == FocusApps {
				m.focus = FocusTeams
			}
		case "right", "l", "tab":
			if m.focus == FocusTeams && len(m.apps) > 0 {
				m.focus = FocusApps
			}
		case "enter":
			if m.focus == FocusApps && len(m.apps) > 0 {
				team := m.teams[m.selectedIdx]
				app := m.apps[m.appIdx]
				return m, func() tea.Msg {
					return messages.NavigateToAppMsg{
						TeamSlug: team.Slug,
						AppSlug:  app.Slug,
					}
				}
			} else if m.focus == FocusTeams && len(m.apps) > 0 {
				m.focus = FocusApps
			}
		}

	case messages.TeamsLoadedMsg:
		m.teams = msg.Teams
		m.loading = false
		// Auto-load apps for first team
		if len(m.teams) > 0 {
			return m, m.loadApps(m.teams[0].Slug)
		}

	case messages.AppsLoadedMsg:
		m.apps = msg.Apps
		m.loading = false

	case messages.ErrorMsg:
		m.err = msg.Err
		m.loading = false
	}

	return m, nil
}

// View renders the dashboard
func (m DashboardModel) View() string {
	if m.loading {
		return styles.Label.Render("Loading...")
	}

	if len(m.teams) == 0 {
		return styles.Label.Render("No teams found. Create a team to get started.")
	}

	// Calculate panel widths
	panelWidth := (m.width - 4) / 2
	if panelWidth < 20 {
		panelWidth = 20
	}

	// Render both panels
	teamsPanel := m.renderTeamsPanel(panelWidth)
	appsPanel := m.renderAppsPanel(panelWidth)

	// Join panels horizontally
	content := lipgloss.JoinHorizontal(lipgloss.Top, teamsPanel, "  ", appsPanel)

	// Help text
	help := "\n" + styles.HelpKey.Render("←/→") + styles.HelpDesc.Render(" switch panels  ")
	help += styles.HelpKey.Render("↑/↓") + styles.HelpDesc.Render(" navigate  ")
	help += styles.HelpKey.Render("enter") + styles.HelpDesc.Render(" select")

	return content + help
}

// renderTeamsPanel renders the teams panel
func (m DashboardModel) renderTeamsPanel(width int) string {
	// Build content
	var content string
	for i, team := range m.teams {
		prefix := "  "
		if i == m.selectedIdx {
			prefix = "> "
		}

		line := fmt.Sprintf("%s%-*s", prefix, width-4, team.Name)

		if i == m.selectedIdx && m.focus == FocusTeams {
			content += styles.SelectedItem.Render(line) + "\n"
		} else if i == m.selectedIdx {
			content += styles.Value.Render(line) + "\n"
		} else {
			content += styles.Label.Render(line) + "\n"
		}
	}

	// Create border style based on focus
	borderColor := styles.Muted
	if m.focus == FocusTeams {
		borderColor = styles.Primary
	}

	return renderPanelBox(width, 0, borderColor, "Teams", content)
}

// renderAppsPanel renders the apps panel
func (m DashboardModel) renderAppsPanel(width int) string {
	// Build content
	var content string

	title := "Apps"
	if len(m.teams) > 0 {
		team := m.teams[m.selectedIdx]
		title = fmt.Sprintf("Apps in %s", team.Name)
	}

	if len(m.apps) == 0 {
		content += styles.Label.Render("No apps in this team")
	} else {
		for i, app := range m.apps {
			prefix := "  "
			if i == m.appIdx {
				prefix = "> "
			}

			runInfo := fmt.Sprintf("(%d runs)", app.RunCount)
			nameWidth := width - len(runInfo) - 6
			if nameWidth < 10 {
				nameWidth = 10
			}
			line := fmt.Sprintf("%s%-*s %s", prefix, nameWidth, app.Name, runInfo)

			if i == m.appIdx && m.focus == FocusApps {
				content += styles.SelectedItem.Render(line) + "\n"
			} else if i == m.appIdx {
				content += styles.Value.Render(line) + "\n"
			} else {
				content += styles.Label.Render(line) + "\n"
			}
		}
	}

	// Create border style based on focus
	borderColor := styles.Muted
	if m.focus == FocusApps {
		borderColor = styles.Primary
	}

	return renderPanelBox(width, 0, borderColor, title, content)
}
