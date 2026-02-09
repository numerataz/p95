package views

import (
	"fmt"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"

	"github.com/ninetyfive/sixtyseven/internal/tui/components"
	"github.com/ninetyfive/sixtyseven/internal/tui/messages"
	"github.com/ninetyfive/sixtyseven/internal/tui/styles"
	"github.com/ninetyfive/sixtyseven/pkg/client"
)

// ViewMode represents the current view mode
type ViewMode int

const (
	ViewModeList  ViewMode = iota // List of metrics with latest values
	ViewModeChart                 // Full-screen single chart
)

// MetricInfo holds metric display info
type MetricInfo struct {
	Name       string
	Latest     float64
	Previous   float64
	HasPrev    bool
	PointCount int
}

// RunDetailModel is the run detail view model
type RunDetailModel struct {
	client *client.Client
	width  int
	height int

	runID   uuid.UUID
	run     *client.Run
	loading bool
	err     error

	// View mode
	viewMode ViewMode

	// Metrics
	metricNames []string
	metricInfo  map[string]*MetricInfo
	charts      map[string]*components.Chart
	selectedIdx int

	// Continuations (resume markers)
	continuations []client.Continuation

	// List view scroll offset
	listOffset int

	// Polling-based updates
	lastUpdate time.Time

	// Refresh state
	refreshing    bool
	refreshMetric string
	spinnerFrame  int
}

// NewRunDetail creates a new run detail model
func NewRunDetail(c *client.Client) RunDetailModel {
	return RunDetailModel{
		client:     c,
		charts:     make(map[string]*components.Chart),
		metricInfo: make(map[string]*MetricInfo),
		viewMode:   ViewModeList,
	}
}

// SetSize sets the view dimensions
func (m RunDetailModel) SetSize(width, height int) RunDetailModel {
	m.width = width
	m.height = height

	// Resize charts for full-screen view
	chartHeight := height - 8
	if chartHeight < 10 {
		chartHeight = 10
	}
	for _, chart := range m.charts {
		chart.SetSize(width-4, chartHeight)
	}

	return m
}

// SetRunID sets the run ID to display
func (m RunDetailModel) SetRunID(runID uuid.UUID) RunDetailModel {
	m.runID = runID
	m.run = nil
	m.metricNames = nil
	m.charts = make(map[string]*components.Chart)
	m.metricInfo = make(map[string]*MetricInfo)
	m.continuations = nil
	m.selectedIdx = 0
	m.listOffset = 0
	m.viewMode = ViewModeList
	return m
}

// LoadRun returns a command to load the run
func (m RunDetailModel) LoadRun() tea.Cmd {
	return func() tea.Msg {
		run, err := m.client.GetRun(m.runID, true)
		if err != nil {
			return messages.ErrorMsg{Err: err}
		}

		names, err := m.client.GetMetricNames(m.runID)
		if err != nil {
			return messages.ErrorMsg{Err: err}
		}

		return messages.RunLoadedMsg{
			Run:         run,
			MetricNames: names,
		}
	}
}

// RefreshMetrics returns a command to refresh metrics
func (m RunDetailModel) RefreshMetrics() tea.Cmd {
	if m.run == nil || m.run.Status != "running" {
		return nil
	}

	return func() tea.Msg {
		latest, err := m.client.GetLatestMetrics(m.runID)
		if err != nil {
			return nil
		}
		return messages.LatestMetricsLoadedMsg{Metrics: latest}
	}
}

// LoadMetricSeries returns a command to load a metric series
func (m RunDetailModel) LoadMetricSeries(metricName string) tea.Cmd {
	return func() tea.Msg {
		points, err := m.client.GetMetricSeries(m.runID, metricName, m.width-10)
		if err != nil {
			return messages.ErrorMsg{Err: err}
		}
		return messages.MetricSeriesLoadedMsg{
			MetricName: metricName,
			Points:     points,
		}
	}
}

// LoadContinuations returns a command to load run continuations
func (m RunDetailModel) LoadContinuations() tea.Cmd {
	return func() tea.Msg {
		continuations, err := m.client.GetContinuations(m.runID)
		if err != nil {
			// Don't error out - continuations are optional
			return messages.ContinuationsLoadedMsg{Continuations: nil}
		}
		return messages.ContinuationsLoadedMsg{Continuations: continuations}
	}
}

// Update handles messages
func (m RunDetailModel) Update(msg tea.Msg) (RunDetailModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.viewMode {
		case ViewModeList:
			switch msg.String() {
			case "up", "k":
				if m.selectedIdx > 0 {
					m.selectedIdx--
					m.adjustListScroll()
				}
			case "down", "j":
				if m.selectedIdx < len(m.metricNames)-1 {
					m.selectedIdx++
					m.adjustListScroll()
				}
			case "enter":
				if len(m.metricNames) > 0 {
					m.viewMode = ViewModeChart
					name := m.metricNames[m.selectedIdx]
					m.refreshing = true
					m.refreshMetric = name
					cmds = append(cmds, m.LoadMetricSeries(name))
				}
			case "r":
				m.loading = true
				cmds = append(cmds, m.LoadRun())
			}
		case ViewModeChart:
			switch msg.String() {
			case "esc":
				m.viewMode = ViewModeList
				m.refreshing = false
			case "t":
				m.toggleChartRenderMode()
			case "left", "h":
				if m.selectedIdx > 0 {
					m.selectedIdx--
					name := m.metricNames[m.selectedIdx]
					m.refreshing = true
					m.refreshMetric = name
					cmds = append(cmds, m.LoadMetricSeries(name))
				}
			case "right", "l":
				if m.selectedIdx < len(m.metricNames)-1 {
					m.selectedIdx++
					name := m.metricNames[m.selectedIdx]
					m.refreshing = true
					m.refreshMetric = name
					cmds = append(cmds, m.LoadMetricSeries(name))
				}
			case "r":
				if len(m.metricNames) > 0 {
					name := m.metricNames[m.selectedIdx]
					m.refreshing = true
					m.refreshMetric = name
					cmds = append(cmds, m.LoadMetricSeries(name))
				}
			}
		}

	case messages.RunLoadedMsg:
		m.run = msg.Run
		m.metricNames = msg.MetricNames
		m.loading = false
		m.lastUpdate = time.Now()

		sort.Strings(m.metricNames)

		for _, name := range m.metricNames {
			if _, ok := m.charts[name]; !ok {
				chart := components.NewChart(name)
				chart.SetSize(m.width-4, m.height-8)
				// Apply any existing continuations to new chart
				if len(m.continuations) > 0 {
					var markers []components.ContinuationMarker
					for _, cont := range m.continuations {
						note := ""
						if cont.Note != nil {
							note = *cont.Note
						}
						markers = append(markers, components.ContinuationMarker{
							Step: cont.Step,
							Note: note,
						})
					}
					chart.SetContinuations(markers)
				}
				m.charts[name] = chart
			}
			if _, ok := m.metricInfo[name]; !ok {
				m.metricInfo[name] = &MetricInfo{Name: name}
			}
		}

		if m.run.LatestMetrics != nil {
			for name, value := range m.run.LatestMetrics {
				if info, ok := m.metricInfo[name]; ok {
					info.Latest = value
				}
			}
		}

		// Load continuations
		cmds = append(cmds, m.LoadContinuations())

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
		if m.refreshMetric == msg.MetricName {
			m.refreshing = false
			m.refreshMetric = ""
		}
		if chart, ok := m.charts[msg.MetricName]; ok {
			var points []components.DataPoint
			for _, p := range msg.Points {
				points = append(points, components.DataPoint{
					Step:  p.Step,
					Value: p.Value,
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

	case messages.ErrorMsg:
		m.err = msg.Err
		m.loading = false
		m.refreshing = false

	case messages.TickMsg:
		m.spinnerFrame = (m.spinnerFrame + 1) % 4

		if m.viewMode == ViewModeChart && m.run != nil && m.run.Status == "running" && !m.refreshing {
			if len(m.metricNames) > 0 {
				name := m.metricNames[m.selectedIdx]
				m.refreshing = true
				m.refreshMetric = name
				cmds = append(cmds, m.LoadMetricSeries(name))
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// adjustListScroll adjusts the list scroll offset to keep selection visible
func (m *RunDetailModel) adjustListScroll() {
	visibleItems := m.height - 12
	if visibleItems < 3 {
		visibleItems = 3
	}

	if m.selectedIdx < m.listOffset {
		m.listOffset = m.selectedIdx
	} else if m.selectedIdx >= m.listOffset+visibleItems {
		m.listOffset = m.selectedIdx - visibleItems + 1
	}
}

func (m RunDetailModel) toggleChartRenderMode() {
	for _, chart := range m.charts {
		chart.ToggleRenderMode()
	}
}

// View renders the run detail
func (m RunDetailModel) View() string {
	if m.loading {
		return styles.Label.Render("Loading run...")
	}

	if m.run == nil {
		return styles.Label.Render("Run not found")
	}

	switch m.viewMode {
	case ViewModeChart:
		return m.renderChartView()
	default:
		return m.renderListView()
	}
}

// renderListView renders the metric list view
func (m RunDetailModel) renderListView() string {
	var sb string

	statusStyle := styles.StatusStyle(m.run.Status)
	sb += styles.Title.Render(m.run.Name) + "  "
	sb += statusStyle.Render(m.run.Status) + "\n"

	infoStyle := styles.Label
	sb += infoStyle.Render(fmt.Sprintf("Started: %s", m.run.StartedAt.Format("Jan 02 15:04:05")))
	if m.run.DurationSeconds != nil {
		sb += infoStyle.Render(fmt.Sprintf("  Duration: %s", styles.FormatDuration(*m.run.DurationSeconds)))
	}
	sb += "\n"

	if len(m.run.Tags) > 0 {
		sb += infoStyle.Render("Tags: ")
		for _, tag := range m.run.Tags {
			sb += lipgloss.NewStyle().
				Background(styles.Primary).
				Foreground(lipgloss.Color("232")).
				Padding(0, 1).
				Render(tag) + " "
		}
		sb += "\n"
	}

	if len(m.continuations) > 0 {
		contStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
		sb += contStyle.Render(fmt.Sprintf("Continuations: %d (↻ resume points)", len(m.continuations))) + "\n"
	}

	sb += "\n"

	if len(m.metricNames) == 0 {
		sb += styles.Label.Render("No metrics logged yet")
	} else {
		sb += styles.Subtitle.Render("Metrics") + "\n\n"

		visibleItems := m.height - 12
		if visibleItems < 3 {
			visibleItems = 3
		}

		endIdx := m.listOffset + visibleItems
		if endIdx > len(m.metricNames) {
			endIdx = len(m.metricNames)
		}

		for i := m.listOffset; i < endIdx; i++ {
			name := m.metricNames[i]
			info := m.metricInfo[name]

			prefix := "  "
			if i == m.selectedIdx {
				prefix = "> "
			}

			valueStr := "---"
			if info != nil {
				valueStr = formatMetricValue(info.Latest)
			}

			trend := "  "
			if info != nil && info.HasPrev {
				if info.Latest > info.Previous {
					trend = lipgloss.NewStyle().Foreground(styles.Success).Render(" ↑")
				} else if info.Latest < info.Previous {
					trend = lipgloss.NewStyle().Foreground(styles.Error).Render(" ↓")
				}
			}

			nameWidth := m.width - 20
			if nameWidth < 20 {
				nameWidth = 20
			}
			nameDisplay := name
			if len(nameDisplay) > nameWidth {
				nameDisplay = nameDisplay[:nameWidth-3] + "..."
			}

			line := fmt.Sprintf("%s%-*s %12s%s", prefix, nameWidth, nameDisplay, valueStr, trend)

			if i == m.selectedIdx {
				sb += styles.SelectedItem.Render(line) + "\n"
			} else {
				sb += styles.Label.Render(line) + "\n"
			}
		}

		if len(m.metricNames) > visibleItems {
			sb += "\n" + styles.Label.Render(fmt.Sprintf("  %d of %d metrics", m.selectedIdx+1, len(m.metricNames)))
		}
	}

	sb += "\n" + styles.HelpKey.Render("enter") + styles.HelpDesc.Render(" view chart  ")
	sb += styles.HelpKey.Render("↑/↓") + styles.HelpDesc.Render(" navigate  ")
	sb += styles.HelpKey.Render("r") + styles.HelpDesc.Render(" refresh")

	if !m.lastUpdate.IsZero() {
		sb += "\n" + styles.Label.Render(fmt.Sprintf("Last update: %s", m.lastUpdate.Format("15:04:05")))
	}

	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Render(sb)
}

// renderChartView renders the full-screen chart view
func (m RunDetailModel) renderChartView() string {
	if len(m.metricNames) == 0 {
		return styles.Label.Render("No metrics")
	}

	name := m.metricNames[m.selectedIdx]
	chart := m.charts[name]

	var sb string

	spinnerFrames := []string{"⠋", "⠙", "⠹", "⠸"}

	sb += styles.Title.Render(name) + "  "
	sb += styles.Label.Render(fmt.Sprintf("(%d/%d)", m.selectedIdx+1, len(m.metricNames)))
	if m.refreshing {
		sb += "  " + styles.Subtitle.Render(spinnerFrames[m.spinnerFrame]+" loading...")
	} else if m.run != nil && m.run.Status == "running" {
		sb += "  " + styles.StatusRunning.Render("● live")
	}
	sb += "\n\n"

	if chart != nil {
		if chart.PointCount() == 0 && m.refreshing {
			sb += styles.Label.Render(spinnerFrames[m.spinnerFrame] + " Loading chart data...")
		} else if chart.PointCount() == 0 {
			sb += styles.Label.Render("No data points yet")
		} else {
			sb += chart.View()
		}
	} else {
		sb += styles.Label.Render("Chart not available")
	}

	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Render(sb)
}

// formatMetricValue formats a metric value for display
func formatMetricValue(v float64) string {
	absV := v
	if absV < 0 {
		absV = -absV
	}

	if absV == 0 {
		return "0"
	} else if absV >= 1000000 {
		return fmt.Sprintf("%.2fM", v/1000000)
	} else if absV >= 1000 {
		return fmt.Sprintf("%.2fK", v/1000)
	} else if absV >= 1 {
		return fmt.Sprintf("%.4f", v)
	} else if absV >= 0.0001 {
		return fmt.Sprintf("%.6f", v)
	} else {
		return fmt.Sprintf("%.2e", v)
	}
}

// InChartMode returns true if the view is showing a full-screen chart
func (m RunDetailModel) InChartMode() bool {
	return m.viewMode == ViewModeChart
}
