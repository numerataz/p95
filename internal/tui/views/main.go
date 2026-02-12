package views

import (
	"fmt"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	zone "github.com/lrstanley/bubblezone"

	"github.com/ninetyfive/p95/internal/tui/components"
	"github.com/ninetyfive/p95/internal/tui/messages"
	"github.com/ninetyfive/p95/internal/tui/styles"
	"github.com/ninetyfive/p95/pkg/client"
)

// Zone ID prefixes for mouse tracking
const (
	zoneTeamPrefix    = "team-"
	zoneProjectPrefix = "project-"
	zoneRunPrefix     = "run-"
	zoneMetricPrefix  = "metric-"
)

// Panel represents which panel has focus
type Panel int

const (
	PanelTeams Panel = iota
	PanelProjects
	PanelRuns
	PanelGraph
)

// MainModel is the unified main view model with lazygit-style layout
type MainModel struct {
	client *client.Client
	width  int
	height int

	// Zone prefix for unique zone IDs
	zoneID string

	// Focus state
	focus Panel

	// Teams data
	teams       []client.Team
	teamIdx     int
	teamsLoaded bool

	// Projects (Apps) data
	apps       []client.App
	appIdx     int
	appsLoaded bool

	// Runs data
	runs       []client.Run
	runIdx     int
	runsLoaded bool

	// Selected run and metrics
	selectedRun *client.Run
	metricNames []string
	metricIdx   int
	charts      map[string]*components.Chart
	metricInfo  map[string]*MetricInfo

	// Loading states
	loadingTeams    bool
	loadingApps     bool
	loadingRuns     bool
	loadingRun      bool
	loadingMetric   bool
	refreshingChart bool

	// Scroll offsets for lists
	teamsOffset   int
	appsOffset    int
	runsOffset    int
	metricsOffset int

	// Error state
	err error

	// Last update time
	lastUpdate   time.Time
	spinnerFrame int
	tickCount    int // For periodic refreshes

	// Comparison state
	comparedRuns     map[uuid.UUID]bool                   // Set of runs being compared
	comparedRunNames map[uuid.UUID]string                 // Run ID -> name mapping
	comparedSeries   map[uuid.UUID][]components.DataPoint // Run ID -> data points for current metric
	comparedRunOrder []uuid.UUID                          // Stable order for colors/legend

	// Continuations (resume markers)
	continuations []client.Continuation
}

// NewMain creates a new main model
func NewMain(c *client.Client) MainModel {
	return MainModel{
		client:           c,
		zoneID:           zone.NewPrefix(),
		focus:            PanelTeams,
		charts:           make(map[string]*components.Chart),
		metricInfo:       make(map[string]*MetricInfo),
		comparedRuns:     make(map[uuid.UUID]bool),
		comparedRunNames: make(map[uuid.UUID]string),
		comparedSeries:   make(map[uuid.UUID][]components.DataPoint),
		comparedRunOrder: nil,
	}
}

// Init initializes the main view
func (m MainModel) Init() tea.Cmd {
	return m.loadTeams
}

// SetSize sets the view dimensions
func (m MainModel) SetSize(width, height int) MainModel {
	m.width = width
	m.height = height

	// Update chart sizes
	// Panel uses: 2 (borders) + 1 (header line) + 1 (tabs) = 4 lines overhead
	// Width: 2 (borders) + 2 (padding) = 4 chars overhead
	graphWidth, graphHeight := m.graphPanelSize()
	for _, chart := range m.charts {
		chart.SetSize(graphWidth-4, graphHeight-4)
	}

	return m
}

// leftPanelWidth returns the width for left panels
func (m MainModel) leftPanelWidth() int {
	// Left panel is about 25% of width, min 24, max 40
	w := m.width / 4
	if w < 24 {
		w = 24
	}
	if w > 40 {
		w = 40
	}
	return w
}

// graphPanelSize returns the width and height for the graph panel
func (m MainModel) graphPanelSize() (int, int) {
	leftW := m.leftPanelWidth()
	graphW := m.width - leftW - 3
	graphH := m.contentHeight()
	return graphW, graphH
}

func (m MainModel) panelHeight() int {
	return m.contentHeight() / 3
}

func (m MainModel) contentHeight() int {
	if m.height <= 1 {
		return m.height
	}
	return m.height - 1
}

// loadTeams loads teams from the API
func (m MainModel) loadTeams() tea.Msg {
	teams, err := m.client.GetTeams()
	if err != nil {
		return messages.ErrorMsg{Err: err}
	}
	return messages.TeamsLoadedMsg{Teams: teams}
}

// loadApps loads apps for the selected team
func (m MainModel) loadApps() tea.Cmd {
	if len(m.teams) == 0 {
		return nil
	}
	team := m.teams[m.teamIdx]
	return func() tea.Msg {
		apps, err := m.client.GetApps(team.Slug)
		if err != nil {
			return messages.ErrorMsg{Err: err}
		}
		return messages.AppsLoadedMsg{Apps: apps}
	}
}

// loadRuns loads runs for the selected app
func (m MainModel) loadRuns() tea.Cmd {
	if len(m.teams) == 0 || len(m.apps) == 0 {
		return nil
	}
	team := m.teams[m.teamIdx]
	app := m.apps[m.appIdx]
	return func() tea.Msg {
		runs, err := m.client.GetRuns(team.Slug, app.Slug)
		if err != nil {
			return messages.ErrorMsg{Err: err}
		}
		return messages.RunsLoadedMsg{Runs: runs}
	}
}

// loadAppsSilent loads apps/projects without showing loading state (for background refresh)
func (m MainModel) loadAppsSilent() tea.Cmd {
	if len(m.teams) == 0 {
		return nil
	}
	team := m.teams[m.teamIdx]
	return func() tea.Msg {
		apps, err := m.client.GetApps(team.Slug)
		if err != nil {
			return nil // Silently ignore errors on background refresh
		}
		return messages.AppsRefreshedMsg{Apps: apps}
	}
}

// loadRunsSilent loads runs without showing loading state (for background refresh)
func (m MainModel) loadRunsSilent() tea.Cmd {
	if len(m.teams) == 0 || len(m.apps) == 0 {
		return nil
	}
	team := m.teams[m.teamIdx]
	app := m.apps[m.appIdx]
	return func() tea.Msg {
		runs, err := m.client.GetRuns(team.Slug, app.Slug)
		if err != nil {
			return nil // Silently ignore errors on background refresh
		}
		return messages.RunsRefreshedMsg{Runs: runs}
	}
}

// loadRun loads detailed run info
func (m MainModel) loadRun() tea.Cmd {
	if len(m.runs) == 0 {
		return nil
	}
	run := m.runs[m.runIdx]
	return func() tea.Msg {
		fullRun, err := m.client.GetRun(run.ID, true)
		if err != nil {
			return messages.ErrorMsg{Err: err}
		}
		names, err := m.client.GetMetricNames(run.ID)
		if err != nil {
			return messages.ErrorMsg{Err: err}
		}
		return messages.RunLoadedMsg{
			Run:         fullRun,
			MetricNames: names,
		}
	}
}

// loadMetricSeries loads metric series data for the chart
func (m MainModel) loadMetricSeries() tea.Cmd {
	if len(m.metricNames) == 0 || m.selectedRun == nil {
		return nil
	}
	name := m.metricNames[m.metricIdx]
	runID := m.selectedRun.ID
	graphW, _ := m.graphPanelSize()
	return func() tea.Msg {
		points, err := m.client.GetMetricSeries(runID, name, graphW-10)
		if err != nil {
			return messages.ErrorMsg{Err: err}
		}
		return messages.MetricSeriesLoadedMsg{
			MetricName: name,
			Points:     points,
		}
	}
}

// loadContinuations loads continuations for the selected run
func (m MainModel) loadContinuations() tea.Cmd {
	if m.selectedRun == nil {
		return nil
	}
	runID := m.selectedRun.ID
	return func() tea.Msg {
		continuations, err := m.client.GetContinuations(runID)
		if err != nil {
			// Don't error out - continuations are optional
			return messages.ContinuationsLoadedMsg{Continuations: nil}
		}
		return messages.ContinuationsLoadedMsg{Continuations: continuations}
	}
}

// Update handles messages
func (m MainModel) Update(msg tea.Msg) (MainModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			// Cycle through panels
			m.focus = (m.focus + 1) % 4
		case "shift+tab":
			// Cycle backwards
			m.focus = (m.focus + 3) % 4
		case "1":
			m.focus = PanelTeams
		case "2":
			m.focus = PanelProjects
		case "3":
			m.focus = PanelRuns
		case "4":
			m.focus = PanelGraph
		case "up":
			cmds = append(cmds, m.handleUp()...)
		case "down":
			cmds = append(cmds, m.handleDown()...)
		case "left":
			// Reserved for within-panel navigation
		case "right":
			// Reserved for within-panel navigation
		case "enter":
			cmds = append(cmds, m.handleEnter()...)
		case "r":
			cmds = append(cmds, m.handleRefresh()...)
		case "t":
			m.toggleChartRenderMode()
		case "x":
			m.toggleXAxisMode()
		case "y":
			m.toggleYAxisScale()
		case "f":
			// Cycle highlight through compared runs (bring to front)
			if m.focus == PanelGraph && len(m.comparedRuns) > 0 {
				m.cycleHighlight()
			}
		case " ":
			if m.focus == PanelRuns && len(m.runs) > 0 {
				cmds = append(cmds, m.handleToggleComparison()...)
			}
		case "c":
			if m.focus == PanelRuns || m.focus == PanelGraph {
				m.clearComparison()
				cmds = append(cmds, m.rebuildComparisonChart()...)
			}
		}

	case tea.MouseMsg:
		if msg.Action != tea.MouseActionRelease || msg.Button != tea.MouseButtonLeft {
			break
		}
		cmds = append(cmds, m.handleMouseClick(msg)...)

	case messages.TeamsLoadedMsg:
		m.teams = msg.Teams
		m.teamsLoaded = true
		m.loadingTeams = false
		m.teamIdx = 0
		if len(m.teams) > 0 {
			m.loadingApps = true
			cmds = append(cmds, m.loadApps())
		}

	case messages.AppsLoadedMsg:
		m.apps = msg.Apps
		m.appsLoaded = true
		m.loadingApps = false
		m.appIdx = 0
		if len(m.apps) > 0 {
			m.loadingRuns = true
			cmds = append(cmds, m.loadRuns())
		}

	case messages.AppsRefreshedMsg:
		// Silent refresh - preserve selection if possible
		if msg.Apps == nil {
			break
		}
		oldSelectedSlug := ""
		if len(m.apps) > 0 && m.appIdx < len(m.apps) {
			oldSelectedSlug = m.apps[m.appIdx].Slug
		}

		m.apps = msg.Apps

		// Try to maintain selection
		newIdx := 0
		for i, app := range m.apps {
			if app.Slug == oldSelectedSlug {
				newIdx = i
				break
			}
		}
		m.appIdx = newIdx

	case messages.RunsLoadedMsg:
		m.runs = msg.Runs
		m.runsLoaded = true
		m.loadingRuns = false
		m.runIdx = 0
		// Clear comparison when runs list is reloaded
		m.clearComparison()
		if len(m.runs) > 0 {
			m.loadingRun = true
			cmds = append(cmds, m.loadRun())
		}

	case messages.RunsRefreshedMsg:
		// Silent refresh - preserve selection if possible
		if msg.Runs == nil {
			break
		}
		oldSelectedID := ""
		if m.selectedRun != nil {
			oldSelectedID = m.selectedRun.ID.String()
		} else if len(m.runs) > 0 && m.runIdx < len(m.runs) {
			oldSelectedID = m.runs[m.runIdx].ID.String()
		}

		m.runs = msg.Runs

		// Build set of current run IDs for comparison cleanup
		currentRunIDs := make(map[uuid.UUID]bool)
		for _, run := range m.runs {
			currentRunIDs[run.ID] = true
		}

		// Remove compared runs that no longer exist
		for runID := range m.comparedRuns {
			if !currentRunIDs[runID] {
				delete(m.comparedRuns, runID)
				delete(m.comparedRunNames, runID)
				delete(m.comparedSeries, runID)
				m.removeComparedRunOrder(runID)
			}
		}

		// Try to maintain selection and update selectedRun status
		newIdx := 0
		for i, run := range m.runs {
			if run.ID.String() == oldSelectedID {
				newIdx = i
				// Update the selected run's status from the refreshed data
				if m.selectedRun != nil {
					m.selectedRun.Status = run.Status
					m.selectedRun.EndedAt = run.EndedAt
					m.selectedRun.DurationSeconds = run.DurationSeconds
				}
				break
			}
		}
		m.runIdx = newIdx

		// If we have new runs at the top, show indicator (runs are sorted newest first)
		m.lastUpdate = time.Now()

	case messages.RunLoadedMsg:
		m.selectedRun = msg.Run
		m.metricNames = msg.MetricNames
		m.loadingRun = false
		m.metricIdx = 0
		m.lastUpdate = time.Now()
		m.continuations = nil // Reset continuations for new run

		sort.Strings(m.metricNames)

		// Initialize charts and metric info
		graphW, graphH := m.graphPanelSize()
		for _, name := range m.metricNames {
			if _, ok := m.charts[name]; !ok {
				chart := components.NewChart(name)
				chart.SetSize(graphW-4, graphH-4)
				m.charts[name] = chart
			}
			if _, ok := m.metricInfo[name]; !ok {
				m.metricInfo[name] = &MetricInfo{Name: name}
			}
		}

		// Update latest metrics
		if m.selectedRun.LatestMetrics != nil {
			for name, value := range m.selectedRun.LatestMetrics {
				if info, ok := m.metricInfo[name]; ok {
					info.Latest = value
				}
			}
		}

		// Load continuations
		cmds = append(cmds, m.loadContinuations())

		// Load first metric chart
		if len(m.metricNames) > 0 {
			m.loadingMetric = true
			cmds = append(cmds, m.loadMetricSeries())
		}

	case messages.ContinuationsLoadedMsg:
		m.continuations = msg.Continuations
		// Update all charts with continuation markers
		for _, chart := range m.charts {
			var markers []components.ContinuationMarker
			for _, cont := range m.continuations {
				note := ""
				if cont.Note != nil {
					note = *cont.Note
				}
				markers = append(markers, components.ContinuationMarker{
					Step:      cont.Step,
					Timestamp: cont.Timestamp.UnixMilli(),
					Note:      note,
				})
			}
			chart.SetContinuations(markers)
		}

	case messages.MetricSeriesLoadedMsg:
		m.loadingMetric = false
		m.refreshingChart = false
		if chart, ok := m.charts[msg.MetricName]; ok {
			var points []components.DataPoint
			for _, p := range msg.Points {
				var ts int64
				// Only use timestamp if it's a valid time (not zero)
				if !p.Time.IsZero() {
					ts = p.Time.UnixMilli()
				}
				points = append(points, components.DataPoint{
					Step:      p.Step,
					Value:     p.Value,
					Timestamp: ts,
				})
			}
			chart.SetData(points)
			if info, ok := m.metricInfo[msg.MetricName]; ok {
				info.PointCount = len(points)
				if len(points) > 0 {
					info.Latest = points[len(points)-1].Value
					if len(points) > 1 {
						info.Previous = points[len(points)-2].Value
						info.HasPrev = true
					}
				}
			}
		}
		m.lastUpdate = time.Now()

	case messages.LatestMetricsLoadedMsg:
		m.lastUpdate = time.Now()
		for name, value := range msg.Metrics {
			if chart, ok := m.charts[name]; ok {
				chart.AddPoint(int64(chart.PointCount()), value)
			}
			if info, ok := m.metricInfo[name]; ok {
				info.Previous = info.Latest
				info.HasPrev = true
				info.Latest = value
				info.PointCount++
			}
		}

	case messages.ComparisonSeriesLoadedMsg:
		if len(m.metricNames) > 0 && msg.MetricName == m.metricNames[m.metricIdx] {
			var points []components.DataPoint
			for _, p := range msg.Points {
				var ts int64
				if !p.Time.IsZero() {
					ts = p.Time.UnixMilli()
				}
				points = append(points, components.DataPoint{
					Step:      p.Step,
					Value:     p.Value,
					Timestamp: ts,
				})
			}
			m.comparedSeries[msg.RunID] = points
			m.rebuildChartSeries()
		}

	case messages.ErrorMsg:
		m.err = msg.Err
		m.loadingTeams = false
		m.loadingApps = false
		m.loadingRuns = false
		m.loadingRun = false
		m.loadingMetric = false

	case messages.TickMsg:
		m.spinnerFrame = (m.spinnerFrame + 1) % 4
		m.tickCount++

		// Auto-refresh projects list every 5 ticks (10 seconds) to catch new projects
		if m.tickCount%5 == 0 && len(m.teams) > 0 && !m.loadingApps {
			cmds = append(cmds, m.loadAppsSilent())
		}

		// Auto-refresh runs list every 3 ticks (6 seconds) to catch new runs
		if m.tickCount%3 == 0 && len(m.apps) > 0 && !m.loadingRuns {
			cmds = append(cmds, m.loadRunsSilent())
		}

		// Auto-refresh chart if run is running
		if m.selectedRun != nil && m.selectedRun.Status == "running" && !m.refreshingChart {
			m.refreshingChart = true
			cmds = append(cmds, m.loadMetricSeries())
		}

		// Auto-refresh comparison series for running runs
		if len(m.comparedRuns) > 0 {
			for _, run := range m.runs {
				if m.comparedRuns[run.ID] && run.Status == "running" {
					cmds = append(cmds, m.loadComparisonSeries(run.ID))
				}
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// handleUp handles up key press
func (m *MainModel) handleUp() []tea.Cmd {
	var cmds []tea.Cmd
	switch m.focus {
	case PanelTeams:
		if m.teamIdx > 0 {
			m.teamIdx--
			m.adjustScroll(&m.teamsOffset, m.teamIdx, m.panelHeight()-4)
			// Load apps for new team
			m.apps = nil
			m.runs = nil
			m.selectedRun = nil
			m.metricNames = nil
			m.loadingApps = true
			cmds = append(cmds, m.loadApps())
		}
	case PanelProjects:
		if m.appIdx > 0 {
			m.appIdx--
			m.adjustScroll(&m.appsOffset, m.appIdx, m.panelHeight()-4)
			// Load runs for new app
			m.runs = nil
			m.selectedRun = nil
			m.metricNames = nil
			m.loadingRuns = true
			cmds = append(cmds, m.loadRuns())
		}
	case PanelRuns:
		if m.runIdx > 0 {
			m.runIdx--
			m.adjustScroll(&m.runsOffset, m.runIdx, m.panelHeight()-4)
			// Load run details
			m.selectedRun = nil
			m.metricNames = nil
			m.loadingRun = true
			cmds = append(cmds, m.loadRun())
		}
	case PanelGraph:
		if m.metricIdx > 0 {
			m.metricIdx--
			m.adjustScroll(&m.metricsOffset, m.metricIdx, 10)
			m.loadingMetric = true
			cmds = append(cmds, m.loadMetricSeries())
			// Clear and reload all comparison series for new metric
			m.comparedSeries = make(map[uuid.UUID][]components.DataPoint)
			m.rebuildChartSeries() // Clear chart series immediately
			cmds = append(cmds, m.loadAllComparisonSeries()...)
		}
	}
	return cmds
}

// handleDown handles down key press
func (m *MainModel) handleDown() []tea.Cmd {
	var cmds []tea.Cmd
	switch m.focus {
	case PanelTeams:
		if m.teamIdx < len(m.teams)-1 {
			m.teamIdx++
			m.adjustScroll(&m.teamsOffset, m.teamIdx, m.panelHeight()-4)
			m.apps = nil
			m.runs = nil
			m.selectedRun = nil
			m.metricNames = nil
			m.loadingApps = true
			cmds = append(cmds, m.loadApps())
		}
	case PanelProjects:
		if m.appIdx < len(m.apps)-1 {
			m.appIdx++
			m.adjustScroll(&m.appsOffset, m.appIdx, m.panelHeight()-4)
			m.runs = nil
			m.selectedRun = nil
			m.metricNames = nil
			m.loadingRuns = true
			cmds = append(cmds, m.loadRuns())
		}
	case PanelRuns:
		if m.runIdx < len(m.runs)-1 {
			m.runIdx++
			m.adjustScroll(&m.runsOffset, m.runIdx, m.panelHeight()-4)
			m.selectedRun = nil
			m.metricNames = nil
			m.loadingRun = true
			cmds = append(cmds, m.loadRun())
		}
	case PanelGraph:
		if m.metricIdx < len(m.metricNames)-1 {
			m.metricIdx++
			m.adjustScroll(&m.metricsOffset, m.metricIdx, 10)
			m.loadingMetric = true
			cmds = append(cmds, m.loadMetricSeries())
			// Clear and reload all comparison series for new metric
			m.comparedSeries = make(map[uuid.UUID][]components.DataPoint)
			m.rebuildChartSeries() // Clear chart series immediately
			cmds = append(cmds, m.loadAllComparisonSeries()...)
		}
	}
	return cmds
}

// handleLeft handles left key press (within-panel navigation)
func (m *MainModel) handleLeft() []tea.Cmd {
	return nil
}

// handleRight handles right key press (within-panel navigation)
func (m *MainModel) handleRight() []tea.Cmd {
	return nil
}

// handleEnter handles enter key press
func (m *MainModel) handleEnter() []tea.Cmd {
	var cmds []tea.Cmd
	switch m.focus {
	case PanelTeams:
		// Move to projects panel
		if len(m.apps) > 0 {
			m.focus = PanelProjects
		}
	case PanelProjects:
		// Move to runs panel
		if len(m.runs) > 0 {
			m.focus = PanelRuns
		}
	case PanelRuns:
		// Move to graph panel
		if len(m.metricNames) > 0 {
			m.focus = PanelGraph
		}
	case PanelGraph:
		// Potentially expand chart (future feature)
	}
	return cmds
}

// handleRefresh handles refresh key press
func (m *MainModel) handleRefresh() []tea.Cmd {
	var cmds []tea.Cmd
	switch m.focus {
	case PanelTeams:
		m.loadingTeams = true
		cmds = append(cmds, m.loadTeams)
	case PanelProjects:
		if len(m.teams) > 0 {
			m.loadingApps = true
			cmds = append(cmds, m.loadApps())
		}
	case PanelRuns:
		if len(m.apps) > 0 {
			m.loadingRuns = true
			cmds = append(cmds, m.loadRuns())
		}
	case PanelGraph:
		if len(m.metricNames) > 0 {
			m.loadingMetric = true
			cmds = append(cmds, m.loadMetricSeries())
		}
	}
	return cmds
}

func (m MainModel) toggleChartRenderMode() {
	for _, chart := range m.charts {
		chart.ToggleRenderMode()
	}
}

func (m MainModel) toggleXAxisMode() {
	for _, chart := range m.charts {
		chart.ToggleXAxisMode()
	}
}

func (m MainModel) toggleYAxisScale() {
	for _, chart := range m.charts {
		chart.ToggleYAxisScale()
	}
}

func (m MainModel) cycleHighlight() {
	if len(m.metricNames) == 0 {
		return
	}
	name := m.metricNames[m.metricIdx]
	if chart, ok := m.charts[name]; ok {
		chart.CycleHighlight()
	}
}

// handleToggleComparison toggles the current run in/out of comparison
func (m *MainModel) handleToggleComparison() []tea.Cmd {
	if len(m.runs) == 0 {
		return nil
	}
	run := m.runs[m.runIdx]
	if m.comparedRuns[run.ID] {
		delete(m.comparedRuns, run.ID)
		delete(m.comparedRunNames, run.ID)
		delete(m.comparedSeries, run.ID)
		m.removeComparedRunOrder(run.ID)
		return m.rebuildComparisonChart()
	} else if len(m.comparedRuns) < 5 {
		m.comparedRuns[run.ID] = true
		m.comparedRunNames[run.ID] = run.Name
		m.comparedRunOrder = append(m.comparedRunOrder, run.ID)
		// Load metric series for this run
		return []tea.Cmd{m.loadComparisonSeries(run.ID)}
	}
	return nil
}

// clearComparison removes all runs from comparison
func (m *MainModel) clearComparison() {
	m.comparedRuns = make(map[uuid.UUID]bool)
	m.comparedRunNames = make(map[uuid.UUID]string)
	m.comparedSeries = make(map[uuid.UUID][]components.DataPoint)
	m.comparedRunOrder = nil
}

// loadComparisonSeries loads metric data for a compared run
func (m *MainModel) loadComparisonSeries(runID uuid.UUID) tea.Cmd {
	if len(m.metricNames) == 0 {
		return nil
	}
	metricName := m.metricNames[m.metricIdx]
	runName := m.comparedRunNames[runID]
	graphW, _ := m.graphPanelSize()
	return func() tea.Msg {
		points, err := m.client.GetMetricSeries(runID, metricName, graphW-10)
		if err != nil {
			return nil // Silently fail
		}
		return messages.ComparisonSeriesLoadedMsg{
			RunID:      runID,
			RunName:    runName,
			MetricName: metricName,
			Points:     points,
		}
	}
}

// loadAllComparisonSeries loads metric data for all compared runs
func (m *MainModel) loadAllComparisonSeries() []tea.Cmd {
	var cmds []tea.Cmd
	for runID := range m.comparedRuns {
		cmds = append(cmds, m.loadComparisonSeries(runID))
	}
	return cmds
}

// rebuildComparisonChart rebuilds the chart with all compared series
func (m *MainModel) rebuildComparisonChart() []tea.Cmd {
	m.rebuildChartSeries()
	return nil
}

// rebuildChartSeries rebuilds the chart series from comparison data
func (m *MainModel) rebuildChartSeries() {
	if len(m.metricNames) == 0 {
		return
	}
	name := m.metricNames[m.metricIdx]
	chart, ok := m.charts[name]
	if !ok {
		return
	}

	// Clear existing series
	chart.ClearSeries()

	// If no runs are being compared, show nothing (selected run data is in chart.data)
	if len(m.comparedRuns) == 0 {
		return
	}

	// Build series from comparison data in stable order
	for i, runID := range m.comparedRunOrder {
		points, ok := m.comparedSeries[runID]
		if !ok || len(points) == 0 {
			continue
		}
		runName := m.comparedRunNames[runID]
		color := styles.SeriesColors[i%len(styles.SeriesColors)]
		chart.AddSeries(runName, color, points)
	}
}

func (m *MainModel) removeComparedRunOrder(runID uuid.UUID) {
	if len(m.comparedRunOrder) == 0 {
		return
	}
	for i, id := range m.comparedRunOrder {
		if id == runID {
			m.comparedRunOrder = append(m.comparedRunOrder[:i], m.comparedRunOrder[i+1:]...)
			return
		}
	}
}

func (m *MainModel) handleMouseClick(msg tea.MouseMsg) []tea.Cmd {
	var cmds []tea.Cmd

	// Check team items
	for i := range m.teams {
		if zone.Get(fmt.Sprintf("%s%s%d", m.zoneID, zoneTeamPrefix, i)).InBounds(msg) {
			if i != m.teamIdx {
				m.teamIdx = i
				m.adjustScroll(&m.teamsOffset, m.teamIdx, m.panelHeight()-4)
				m.apps = nil
				m.runs = nil
				m.selectedRun = nil
				m.metricNames = nil
				m.loadingApps = true
				cmds = append(cmds, m.loadApps())
			}
			m.focus = PanelTeams
			return cmds
		}
	}

	// Check project items
	for i := range m.apps {
		if zone.Get(fmt.Sprintf("%s%s%d", m.zoneID, zoneProjectPrefix, i)).InBounds(msg) {
			if i != m.appIdx {
				m.appIdx = i
				m.adjustScroll(&m.appsOffset, m.appIdx, m.panelHeight()-4)
				m.runs = nil
				m.selectedRun = nil
				m.metricNames = nil
				m.loadingRuns = true
				cmds = append(cmds, m.loadRuns())
			}
			m.focus = PanelProjects
			return cmds
		}
	}

	// Check run items
	for i := range m.runs {
		if zone.Get(fmt.Sprintf("%s%s%d", m.zoneID, zoneRunPrefix, i)).InBounds(msg) {
			if i != m.runIdx {
				m.runIdx = i
				m.adjustScroll(&m.runsOffset, m.runIdx, m.panelHeight()-4)
				m.selectedRun = nil
				m.metricNames = nil
				m.loadingRun = true
				cmds = append(cmds, m.loadRun())
			}
			m.focus = PanelRuns
			return cmds
		}
	}

	// Check metric tabs
	for i := range m.metricNames {
		if zone.Get(fmt.Sprintf("%s%s%d", m.zoneID, zoneMetricPrefix, i)).InBounds(msg) {
			if i != m.metricIdx {
				m.metricIdx = i
				m.loadingMetric = true
				cmds = append(cmds, m.loadMetricSeries())
				// Clear and reload all comparison series for new metric
				m.comparedSeries = make(map[uuid.UUID][]components.DataPoint)
				m.rebuildChartSeries() // Clear chart series immediately
				cmds = append(cmds, m.loadAllComparisonSeries()...)
			}
			m.focus = PanelGraph
			return cmds
		}
	}

	return cmds
}

func (m *MainModel) adjustScroll(offset *int, idx int, visibleItems int) {
	if visibleItems < 1 {
		visibleItems = 1
	}
	if idx < *offset {
		*offset = idx
	} else if idx >= *offset+visibleItems {
		*offset = idx - visibleItems + 1
	}
}

// View renders the main view
func (m MainModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Calculate sizes
	leftWidth := m.leftPanelWidth()
	graphW, graphH := m.graphPanelSize()

	// Reserve 1 line for help bar
	availableHeight := m.contentHeight()

	// Distribute height evenly among 3 panels, giving remainder to runs panel
	baseH := availableHeight / 3
	remainder := availableHeight % 3
	teamsH := baseH
	projectsH := baseH
	runsH := baseH + remainder // Give extra rows to runs panel (most used)

	// Render left panels
	teamsPanel := m.renderTeamsPanel(leftWidth, teamsH)
	projectsPanel := m.renderProjectsPanel(leftWidth, projectsH)
	runsPanel := m.renderRunsPanel(leftWidth, runsH)

	leftPanels := lipgloss.JoinVertical(lipgloss.Left, teamsPanel, projectsPanel, runsPanel)

	// Render right panel (graph)
	graphPanel := m.renderGraphPanel(graphW, graphH)

	// Join horizontally
	content := lipgloss.JoinHorizontal(lipgloss.Top, leftPanels, " ", graphPanel)

	// Add help bar
	helpBar := m.renderHelpBar()
	content = lipgloss.JoinVertical(lipgloss.Left, content, helpBar)

	return content
}

// renderHelpBar renders the keyboard shortcuts help bar
func (m MainModel) renderHelpBar() string {
	var help string

	// Basic navigation
	help += styles.HelpKey.Render("tab") + styles.HelpDesc.Render(" panel  ")
	help += styles.HelpKey.Render("↑↓") + styles.HelpDesc.Render(" navigate  ")

	// Chart controls
	help += styles.HelpKey.Render("t") + styles.HelpDesc.Render(" chart mode  ")
	help += styles.HelpKey.Render("x") + styles.HelpDesc.Render(" x-axis  ")
	help += styles.HelpKey.Render("y") + styles.HelpDesc.Render(" y-scale  ")

	// Comparison controls (show when in runs panel or graph panel)
	if m.focus == PanelRuns || m.focus == PanelGraph {
		help += styles.HelpKey.Render("space") + styles.HelpDesc.Render(" compare  ")
		if len(m.comparedRuns) > 0 {
			help += styles.HelpKey.Render("c") + styles.HelpDesc.Render(" clear  ")
			help += styles.HelpKey.Render("f") + styles.HelpDesc.Render(" highlight  ")
		}
	}

	help += styles.HelpKey.Render("r") + styles.HelpDesc.Render(" refresh")

	return help
}

func (m MainModel) renderTeamsPanel(width, height int) string {
	contentHeight := height - 2

	var content string
	if m.loadingTeams {
		content += styles.Label.Render("Loading...")
	} else if len(m.teams) == 0 {
		content += styles.Label.Render("No teams")
	} else {
		visibleItems := contentHeight - 1
		endIdx := m.teamsOffset + visibleItems
		if endIdx > len(m.teams) {
			endIdx = len(m.teams)
		}

		for i := m.teamsOffset; i < endIdx; i++ {
			team := m.teams[i]
			prefix := "  "
			if i == m.teamIdx {
				prefix = "> "
			}

			name := truncateStr(team.Name, width-6)
			line := fmt.Sprintf("%s%s", prefix, name)

			var styledLine string
			if i == m.teamIdx && m.focus == PanelTeams {
				styledLine = styles.SelectedItem.Render(padRight(line, width-4))
			} else if i == m.teamIdx {
				styledLine = styles.Value.Render(line)
			} else {
				styledLine = styles.Label.Render(line)
			}
			content += zone.Mark(fmt.Sprintf("%s%s%d", m.zoneID, zoneTeamPrefix, i), styledLine) + "\n"
		}
	}

	borderColor := styles.Muted
	if m.focus == PanelTeams {
		borderColor = styles.Primary
	}

	return renderPanelBox(width, height, borderColor, "1 Teams", content)
}

func (m MainModel) renderProjectsPanel(width, height int) string {
	contentHeight := height - 2

	var content string
	if m.loadingApps {
		content += styles.Label.Render("Loading...")
	} else if len(m.apps) == 0 {
		content += styles.Label.Render("No projects")
	} else {
		visibleItems := contentHeight - 1
		endIdx := m.appsOffset + visibleItems
		if endIdx > len(m.apps) {
			endIdx = len(m.apps)
		}

		for i := m.appsOffset; i < endIdx; i++ {
			app := m.apps[i]
			prefix := "  "
			if i == m.appIdx {
				prefix = "> "
			}

			name := truncateStr(app.Name, width-10)
			runCount := fmt.Sprintf("(%d)", app.RunCount)
			line := fmt.Sprintf("%s%s %s", prefix, name, runCount)

			var styledLine string
			if i == m.appIdx && m.focus == PanelProjects {
				styledLine = styles.SelectedItem.Render(padRight(line, width-4))
			} else if i == m.appIdx {
				styledLine = styles.Value.Render(line)
			} else {
				styledLine = styles.Label.Render(line)
			}
			content += zone.Mark(fmt.Sprintf("%s%s%d", m.zoneID, zoneProjectPrefix, i), styledLine) + "\n"
		}
	}

	borderColor := styles.Muted
	if m.focus == PanelProjects {
		borderColor = styles.Primary
	}

	return renderPanelBox(width, height, borderColor, "2 Projects", content)
}

func (m MainModel) renderRunsPanel(width, height int) string {
	contentHeight := height - 2

	var content string
	if m.loadingRuns {
		content += styles.Label.Render("Loading...")
	} else if len(m.runs) == 0 {
		content += styles.Label.Render("No runs")
	} else {
		visibleItems := contentHeight - 1
		endIdx := m.runsOffset + visibleItems
		if endIdx > len(m.runs) {
			endIdx = len(m.runs)
		}

		for i := m.runsOffset; i < endIdx; i++ {
			run := m.runs[i]

			// Build prefix with selection and comparison indicators
			selChar := " "
			if i == m.runIdx {
				selChar = ">"
			}
			compChar := " "
			if m.comparedRuns[run.ID] {
				compChar = "✓"
			}
			prefix := selChar + compChar

			// Status indicator
			statusChar := "○"
			var statusStyle lipgloss.Style
			switch run.Status {
			case "running":
				statusChar = "●"
				statusStyle = styles.StatusRunning
			case "completed":
				statusChar = "●"
				statusStyle = styles.StatusCompleted
			case "failed":
				statusChar = "●"
				statusStyle = styles.StatusFailed
			case "canceled", "aborted":
				statusChar = "●"
				statusStyle = styles.StatusCanceled
			default:
				statusStyle = styles.Label
			}

			name := truncateStr(run.Name, width-11)

			var styledLine string
			if i == m.runIdx && m.focus == PanelRuns {
				// When selected, style the whole line uniformly (no nested styles)
				line := fmt.Sprintf("%s %s %s", prefix, statusChar, name)
				styledLine = styles.SelectedItem.Render(padRight(line, width-4))
			} else if i == m.runIdx {
				// Selected but not focused - show status color
				line := fmt.Sprintf("%s %s %s", prefix, statusStyle.Render(statusChar), name)
				styledLine = styles.Value.Render(line)
			} else {
				// Not selected - show status color
				line := fmt.Sprintf("%s %s %s", prefix, statusStyle.Render(statusChar), name)
				styledLine = styles.Label.Render(line)
			}
			content += zone.Mark(fmt.Sprintf("%s%s%d", m.zoneID, zoneRunPrefix, i), styledLine) + "\n"
		}
	}

	borderColor := styles.Muted
	if m.focus == PanelRuns {
		borderColor = styles.Primary
	}

	return renderPanelBox(width, height, borderColor, "3 Runs", content)
}

// renderGraphPanel renders the graph panel
func (m MainModel) renderGraphPanel(width, height int) string {
	var content string

	// Header line: run name + status + update time
	if m.selectedRun != nil {
		content += styles.Label.Render(m.selectedRun.Name)
		if m.selectedRun.Status == "running" {
			content += "  " + styles.StatusRunning.Render("● live")
		}
		if !m.lastUpdate.IsZero() {
			content += "  " + lipgloss.NewStyle().Foreground(styles.Muted).Render(m.lastUpdate.Format("15:04:05"))
		}
		content += "\n"
	}

	// Metrics tabs (clickable)
	if len(m.metricNames) > 0 {
		// Calculate available width for tabs (panel width minus borders and padding)
		tabsWidth := width - 6
		var tabs []string
		currentWidth := 0

		for i, name := range m.metricNames {
			// Truncate long metric names
			displayName := name
			if len(displayName) > 15 {
				displayName = displayName[:12] + "..."
			}

			var tab string
			tabWidth := len(displayName) + 3 // space + name + space + separator

			// Check if we have room for this tab
			if currentWidth+tabWidth > tabsWidth && len(tabs) > 0 {
				// Add "..." indicator and stop
				tabs = append(tabs, styles.Label.Render("..."))
				break
			}

			if i == m.metricIdx {
				// Selected tab
				tab = zone.Mark(
					fmt.Sprintf("%s%s%d", m.zoneID, zoneMetricPrefix, i),
					styles.SelectedItem.Render(" "+displayName+" "),
				)
			} else {
				// Unselected tab
				tab = zone.Mark(
					fmt.Sprintf("%s%s%d", m.zoneID, zoneMetricPrefix, i),
					styles.Label.Render(" "+displayName+" "),
				)
			}
			tabs = append(tabs, tab)
			currentWidth += tabWidth
		}

		content += lipgloss.JoinHorizontal(lipgloss.Left, tabs...)
		content += "\n"
	}

	// Chart
	spinnerFrames := []string{"⠋", "⠙", "⠹", "⠸"}

	if m.loadingRun || m.loadingMetric {
		content += styles.Label.Render(spinnerFrames[m.spinnerFrame] + " Loading...")
	} else if m.selectedRun == nil {
		content += styles.Label.Render("Select a run to view metrics")
	} else if len(m.metricNames) == 0 {
		content += styles.Label.Render("No metrics logged for this run")
	} else {
		name := m.metricNames[m.metricIdx]
		if chart, ok := m.charts[name]; ok {
			if chart.PointCount() == 0 {
				content += styles.Label.Render("No data points yet")
			} else {
				content += chart.View()
			}
		}
	}

	borderColor := styles.Muted
	if m.focus == PanelGraph {
		borderColor = styles.Primary
	}

	return renderPanelBox(width, height, borderColor, "4 Graph", content)
}

// Helper functions

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func padRight(s string, width int) string {
	visibleWidth := lipgloss.Width(s)
	if visibleWidth >= width {
		return s
	}
	return s + fmt.Sprintf("%*s", width-visibleWidth, "")
}

// GetSelectedRunID returns the selected run ID if any
func (m MainModel) GetSelectedRunID() *uuid.UUID {
	if m.selectedRun != nil {
		return &m.selectedRun.ID
	}
	return nil
}
