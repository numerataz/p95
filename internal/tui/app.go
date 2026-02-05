package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"sixtyseven/internal/tui/messages"
	"sixtyseven/internal/tui/styles"
	"sixtyseven/internal/tui/views"
	"sixtyseven/pkg/client"
)

// App is the main TUI application model
type App struct {
	client *client.Client
	width  int
	height int

	// Main view (unified lazygit-style layout)
	main views.MainModel

	// Error state
	err     error
	loading bool
}

// New creates a new TUI application
func New(apiClient *client.Client) App {
	zone.NewGlobal()
	return App{
		client: apiClient,
		main:   views.NewMain(apiClient),
	}
}

// Init initializes the application
func (a App) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		a.main.Init(),
		tickCmd(),
	)
}

// tickCmd returns a command that sends tick messages for periodic updates
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
		return messages.TickMsg{}
	})
}

// Update handles messages
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return a, tea.Quit
		case "?":
			// Show help (could be implemented)
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		// Propagate to main view (account for header and status bar lines)
		a.main = a.main.SetSize(msg.Width, msg.Height-2)

	case messages.ErrorMsg:
		a.err = msg.Err

	case messages.LoadingMsg:
		a.loading = msg.Loading

	case messages.TickMsg:
		cmds = append(cmds, tickCmd())
	}

	// Update main view
	var cmd tea.Cmd
	a.main, cmd = a.main.Update(msg)
	cmds = append(cmds, cmd)

	return a, tea.Batch(cmds...)
}

// View renders the application
func (a App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	// Header
	header := a.renderHeader()

	// Content
	content := a.main.View()

	// Status bar
	statusBar := a.renderStatusBar()

	// Error display
	if a.err != nil {
		errorBox := lipgloss.NewStyle().
			Foreground(styles.Error).
			Render(fmt.Sprintf("Error: %s", a.err.Error()))
		content = errorBox + "\n" + content
	}

	return zone.Scan(lipgloss.JoinVertical(
		lipgloss.Top,
		header,
		content,
		statusBar,
	))
}

// renderHeader renders the application header
func (a App) renderHeader() string {
	title := styles.Header.Render(" sixtyseven ")

	header := lipgloss.JoinHorizontal(
		lipgloss.Left,
		title,
	)

	// Pad to full width
	return lipgloss.NewStyle().
		Width(a.width).
		Render(header)
}

// renderStatusBar renders the status bar
func (a App) renderStatusBar() string {
	help := []string{
		styles.HelpKey.Render("q") + styles.HelpDesc.Render(" quit"),
		styles.HelpKey.Render("r") + styles.HelpDesc.Render(" refresh"),
		styles.HelpKey.Render("t") + styles.HelpDesc.Render(" switch style"),
		styles.HelpKey.Render("space") + styles.HelpDesc.Render(" compare run"),
		styles.HelpKey.Render("c") + styles.HelpDesc.Render(" clear compare"),
	}

	helpText := strings.Join(help, "  ")

	loadingIndicator := ""
	if a.loading {
		loadingIndicator = styles.Label.Render(" Loading...")
	}

	return lipgloss.NewStyle().
		Width(a.width).
		Foreground(styles.Muted).
		Render(helpText + loadingIndicator)
}
