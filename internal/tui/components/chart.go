package components

import (
	"fmt"
	"math"
	"strings"

	"github.com/ninetyfive/sixtyseven/internal/tui/styles"

	"github.com/NimbleMarkets/ntcharts/canvas"
	"github.com/NimbleMarkets/ntcharts/canvas/graph"
	"github.com/NimbleMarkets/ntcharts/linechart"
	"github.com/charmbracelet/lipgloss"
)

// RenderMode controls how chart data is rendered.
type RenderMode int

const (
	RenderModeBraillePoints RenderMode = iota
	RenderModeLinechart
)

const (
	defaultXAxisStep = 2
	defaultYAxisStep = 2
)

// XAxisMode controls what the X axis displays
type XAxisMode int

const (
	XAxisModeStep XAxisMode = iota
	XAxisModeRelativeTime
)

// YAxisScale controls the Y axis scaling
type YAxisScale int

const (
	YAxisScaleLinear YAxisScale = iota
	YAxisScaleLog
)

// DataPoint represents a single data point
type DataPoint struct {
	Step      int64
	Value     float64
	Timestamp int64 // Unix timestamp in milliseconds
}

// DataSeries represents a named data series with its own color
type DataSeries struct {
	Label  string
	Color  lipgloss.Color
	Points []DataPoint
}

// ContinuationMarker represents a point where the run was resumed
type ContinuationMarker struct {
	Step      int64
	Timestamp int64 // Unix milliseconds
	Note      string
}

// Chart renders a simple line chart in the terminal
type Chart struct {
	name   string
	width  int
	height int
	data   []DataPoint // Legacy single series support
	series []DataSeries

	// Continuation markers (vertical lines)
	continuations []ContinuationMarker

	// Display options
	showAxes       bool
	color          lipgloss.Color
	mode           RenderMode
	xAxisMode      XAxisMode
	yAxisScale     YAxisScale
	highlightIndex int // Index of series to highlight (-1 = none)
}

// NewChart creates a new chart
func NewChart(name string) *Chart {
	return &Chart{
		name:           name,
		width:          60,
		height:         10,
		showAxes:       true,
		color:          styles.Primary,
		mode:           RenderModeBraillePoints,
		xAxisMode:      XAxisModeStep,
		yAxisScale:     YAxisScaleLinear,
		highlightIndex: -1,
		data:           []DataPoint{},
		series:         []DataSeries{},
	}
}

// SetSize sets the chart dimensions
func (c *Chart) SetSize(width, height int) {
	if width > 10 {
		c.width = width
	}
	if height > 3 {
		c.height = height
	}
}

// SetData sets the chart data points (legacy single series)
func (c *Chart) SetData(points []DataPoint) {
	c.data = points
}

// SetSeries sets multiple data series
func (c *Chart) SetSeries(series []DataSeries) {
	c.series = series
}

// AddSeries adds a new data series
func (c *Chart) AddSeries(label string, color lipgloss.Color, points []DataPoint) {
	c.series = append(c.series, DataSeries{
		Label:  label,
		Color:  color,
		Points: points,
	})
}

// ClearSeries removes all data series
func (c *Chart) ClearSeries() {
	c.series = []DataSeries{}
}

// SetContinuations sets the continuation markers
func (c *Chart) SetContinuations(continuations []ContinuationMarker) {
	c.continuations = continuations
}

// AddContinuation adds a continuation marker
func (c *Chart) AddContinuation(step int64, note string) {
	c.continuations = append(c.continuations, ContinuationMarker{Step: step, Note: note})
}

// ClearContinuations removes all continuation markers
func (c *Chart) ClearContinuations() {
	c.continuations = []ContinuationMarker{}
}

// GetContinuations returns the continuation markers
func (c *Chart) GetContinuations() []ContinuationMarker {
	return c.continuations
}

// AddPoint adds a data point to the chart (legacy single series)
func (c *Chart) AddPoint(step int64, value float64) {
	c.data = append(c.data, DataPoint{Step: step, Value: value})

	// Keep only last N points for display
	maxPoints := c.width - 10
	if maxPoints < 10 {
		maxPoints = 10
	}
	if len(c.data) > maxPoints {
		c.data = c.data[len(c.data)-maxPoints:]
	}
}

// Clear clears all data points
func (c *Chart) Clear() {
	c.data = []DataPoint{}
	c.series = []DataSeries{}
}

// SetRenderMode sets how the chart is rendered.
func (c *Chart) SetRenderMode(mode RenderMode) {
	c.mode = mode
}

// ToggleRenderMode switches between braille points and linechart rendering.
func (c *Chart) ToggleRenderMode() {
	if c.mode == RenderModeBraillePoints {
		c.mode = RenderModeLinechart
	} else {
		c.mode = RenderModeBraillePoints
	}
}

// ToggleXAxisMode switches between step and relative time mode.
func (c *Chart) ToggleXAxisMode() {
	if c.xAxisMode == XAxisModeStep {
		c.xAxisMode = XAxisModeRelativeTime
	} else {
		c.xAxisMode = XAxisModeStep
	}
}

// SetXAxisMode sets the X axis mode.
func (c *Chart) SetXAxisMode(mode XAxisMode) {
	c.xAxisMode = mode
}

// GetXAxisMode returns the current X axis mode.
func (c *Chart) GetXAxisMode() XAxisMode {
	return c.xAxisMode
}

// ToggleYAxisScale switches between linear and log scale.
func (c *Chart) ToggleYAxisScale() {
	if c.yAxisScale == YAxisScaleLinear {
		c.yAxisScale = YAxisScaleLog
	} else {
		c.yAxisScale = YAxisScaleLinear
	}
}

// SetYAxisScale sets the Y axis scale.
func (c *Chart) SetYAxisScale(scale YAxisScale) {
	c.yAxisScale = scale
}

// GetYAxisScale returns the current Y axis scale.
func (c *Chart) GetYAxisScale() YAxisScale {
	return c.yAxisScale
}

// SetHighlightIndex sets which series to highlight (bring to front).
// Pass -1 to clear highlighting.
func (c *Chart) SetHighlightIndex(index int) {
	c.highlightIndex = index
}

// GetHighlightIndex returns the current highlighted series index.
func (c *Chart) GetHighlightIndex() int {
	return c.highlightIndex
}

// CycleHighlight cycles through series to highlight.
// Returns the new highlight index.
func (c *Chart) CycleHighlight() int {
	if len(c.series) == 0 {
		c.highlightIndex = -1
		return -1
	}
	c.highlightIndex++
	if c.highlightIndex >= len(c.series) {
		c.highlightIndex = -1
	}
	return c.highlightIndex
}

// HasMultipleSeries returns true if there are multiple series to display
func (c *Chart) HasMultipleSeries() bool {
	return len(c.series) > 1
}

// View renders the chart
func (c *Chart) View() string {
	// If we have multi-series data, use that; otherwise use legacy single series
	if len(c.series) > 0 {
		return c.viewMultiSeries()
	}
	return c.viewSingleSeries()
}

// viewSingleSeries renders the chart with a single data series using ntcharts
func (c *Chart) viewSingleSeries() string {
	if c.mode == RenderModeLinechart {
		return c.viewSingleSeriesLinechart()
	}
	return c.viewSingleSeriesBraillePoints()
}

func (c *Chart) viewSingleSeriesBraillePoints() string {
	if len(c.data) == 0 {
		return styles.ChartTitle.Render(c.name) + "\n" +
			styles.Label.Render("  No data")
	}

	// Find min/max for scaling
	var minVal, maxVal float64
	var minX, maxX float64
	var minTime, maxTime int64
	hasTimestamps := false

	for i, p := range c.data {
		val := p.Value
		if c.yAxisScale == YAxisScaleLog && val > 0 {
			val = applyLogScale(val)
		}

		if i == 0 {
			minVal, maxVal = val, val
			minX, maxX = float64(p.Step), float64(p.Step)
			minTime, maxTime = p.Timestamp, p.Timestamp
		} else {
			if val < minVal {
				minVal = val
			}
			if val > maxVal {
				maxVal = val
			}
			if float64(p.Step) < minX {
				minX = float64(p.Step)
			}
			if float64(p.Step) > maxX {
				maxX = float64(p.Step)
			}
			if p.Timestamp < minTime {
				minTime = p.Timestamp
			}
			if p.Timestamp > maxTime {
				maxTime = p.Timestamp
			}
		}
		if p.Timestamp > 0 {
			hasTimestamps = true
		}
	}

	// Determine X axis range based on mode
	useRelativeTime := c.xAxisMode == XAxisModeRelativeTime && hasTimestamps
	if useRelativeTime {
		minX = 0
		maxX = float64(maxTime-minTime) / 1000.0
	}

	// Add padding to range
	valRange := maxVal - minVal
	if valRange == 0 {
		valRange = 1
		minVal -= 0.5
		maxVal += 0.5
	}

	xRange := maxX - minX
	if xRange == 0 {
		xRange = 1
	}

	// Calculate chart dimensions - ntcharts handles axes internally
	chartHeight := c.height - 1 // Just leave room for our title line
	if chartHeight < 3 {
		chartHeight = 3
	}
	chartWidth := c.width
	if chartWidth < 10 {
		chartWidth = 10
	}

	// Create ntcharts canvas + braille grid
	cv := canvas.New(chartWidth, chartHeight)
	origin, graphWidth, graphHeight := graphSizeAndOrigin(
		chartWidth,
		chartHeight,
		minVal,
		maxVal,
		defaultXAxisStep,
		defaultYAxisStep,
	)
	grid := graph.NewBrailleGrid(graphWidth, graphHeight,
		minX, maxX,
		minVal, maxVal,
	)

	// Draw a braille dot for each point
	lineStyle := lipgloss.NewStyle().Foreground(c.color)
	for i := 0; i < len(c.data); i++ {
		pt := c.data[i]
		var x float64
		if useRelativeTime {
			x = float64(pt.Timestamp-minTime) / 1000.0
		} else {
			x = float64(pt.Step)
		}
		y := pt.Value
		if c.yAxisScale == YAxisScaleLog && y > 0 {
			y = applyLogScale(y)
		}
		p := canvas.Float64Point{X: x, Y: y}
		grid.Set(grid.GridPoint(p))
	}

	graphTopLeft := canvas.Point{X: origin.X + 1, Y: 0}
	graph.DrawBraillePatterns(&cv, graphTopLeft, grid.BraillePatterns(), lineStyle)

	// Draw continuation markers as vertical dashed lines
	c.drawContinuationMarkers(&cv, origin, graphWidth, graphHeight, minX, maxX, useRelativeTime, minTime)

	c.drawAxesAndLabelsWithOptions(
		&cv,
		origin,
		graphWidth,
		graphHeight,
		minX,
		maxX,
		minVal,
		maxVal,
		defaultXAxisStep,
		defaultYAxisStep,
		useRelativeTime,
	)

	// Build output
	var sb strings.Builder

	// Title with latest value and mode indicators
	latest := c.data[len(c.data)-1].Value
	titleLine := styles.ChartTitle.Render(c.name) + " " +
		styles.Value.Render(fmt.Sprintf("%.4f", latest))
	if c.yAxisScale == YAxisScaleLog {
		titleLine += " " + styles.Label.Render("[log]")
	}
	if useRelativeTime {
		titleLine += " " + styles.Label.Render("[time]")
	}
	if len(c.continuations) > 0 {
		titleLine += " " + styles.Label.Render(fmt.Sprintf("[%d cont]", len(c.continuations)))
	}
	sb.WriteString(titleLine + "\n")

	// Chart
	sb.WriteString(cv.View())

	return sb.String()
}

func (c *Chart) viewSingleSeriesLinechart() string {
	if len(c.data) == 0 {
		return styles.ChartTitle.Render(c.name) + "\n" +
			styles.Label.Render("  No data")
	}

	// Find min/max for scaling
	var minVal, maxVal float64
	var minX, maxX float64
	var minTime, maxTime int64
	hasTimestamps := false

	for i, p := range c.data {
		val := p.Value
		if c.yAxisScale == YAxisScaleLog && val > 0 {
			val = applyLogScale(val)
		}

		if i == 0 {
			minVal, maxVal = val, val
			minX, maxX = float64(p.Step), float64(p.Step)
			minTime, maxTime = p.Timestamp, p.Timestamp
		} else {
			if val < minVal {
				minVal = val
			}
			if val > maxVal {
				maxVal = val
			}
			if float64(p.Step) < minX {
				minX = float64(p.Step)
			}
			if float64(p.Step) > maxX {
				maxX = float64(p.Step)
			}
			if p.Timestamp < minTime {
				minTime = p.Timestamp
			}
			if p.Timestamp > maxTime {
				maxTime = p.Timestamp
			}
		}
		if p.Timestamp > 0 {
			hasTimestamps = true
		}
	}

	// Determine X axis range based on mode
	useRelativeTime := c.xAxisMode == XAxisModeRelativeTime && hasTimestamps
	if useRelativeTime {
		minX = 0
		maxX = float64(maxTime-minTime) / 1000.0
	}

	// Add padding to range
	valRange := maxVal - minVal
	if valRange == 0 {
		valRange = 1
		minVal -= 0.5
		maxVal += 0.5
	}

	xRange := maxX - minX
	if xRange == 0 {
		xRange = 1
	}

	// Calculate chart dimensions
	chartHeight := c.height - 1
	if chartHeight < 3 {
		chartHeight = 3
	}
	chartWidth := c.width
	if chartWidth < 10 {
		chartWidth = 10
	}

	// Create formatters based on settings with deduplication
	yDedup := &dedupingFormatter{
		format: func(v float64) string {
			if c.yAxisScale == YAxisScaleLog {
				return formatLogYLabel(v)
			}
			return formatYLabel(v)
		},
	}

	xDedup := &dedupingFormatter{
		format: func(v float64) string {
			if useRelativeTime {
				return formatTimeLabel(v)
			}
			return fmt.Sprintf("%.0f", v)
		},
	}

	// Create ntcharts linechart
	lc := linechart.New(chartWidth, chartHeight,
		minX, maxX,
		minVal, maxVal,
		linechart.WithStyles(styles.ChartAxis, styles.Label, lipgloss.NewStyle().Foreground(c.color)),
		linechart.WithXYSteps(defaultXAxisStep, defaultYAxisStep),
		linechart.WithYLabelFormatter(yDedup.Format),
		linechart.WithXLabelFormatter(xDedup.Format),
	)

	// Draw lines between consecutive points with style
	lineStyle := lipgloss.NewStyle().Foreground(c.color)
	for i := 1; i < len(c.data); i++ {
		var x1, x2, y1, y2 float64
		if useRelativeTime {
			x1 = float64(c.data[i-1].Timestamp-minTime) / 1000.0
			x2 = float64(c.data[i].Timestamp-minTime) / 1000.0
		} else {
			x1 = float64(c.data[i-1].Step)
			x2 = float64(c.data[i].Step)
		}
		y1 = c.data[i-1].Value
		y2 = c.data[i].Value
		if c.yAxisScale == YAxisScaleLog {
			if y1 > 0 {
				y1 = applyLogScale(y1)
			}
			if y2 > 0 {
				y2 = applyLogScale(y2)
			}
		}
		p1 := canvas.Float64Point{X: x1, Y: y1}
		p2 := canvas.Float64Point{X: x2, Y: y2}
		lc.DrawBrailleLineWithStyle(p1, p2, lineStyle)
	}

	// Draw continuation markers as vertical dashed lines
	c.drawContinuationMarkers(&lc.Canvas, lc.Origin(), lc.GraphWidth(), lc.GraphHeight(), minX, maxX, useRelativeTime, minTime)

	// Draw axes
	lc.DrawXYAxisAndLabel()

	// Build output
	var sb strings.Builder

	// Title with latest value and mode indicators
	latest := c.data[len(c.data)-1].Value
	titleLine := styles.ChartTitle.Render(c.name) + " " +
		styles.Value.Render(fmt.Sprintf("%.4f", latest))
	if c.yAxisScale == YAxisScaleLog {
		titleLine += " " + styles.Label.Render("[log]")
	}
	if useRelativeTime {
		titleLine += " " + styles.Label.Render("[time]")
	}
	if len(c.continuations) > 0 {
		titleLine += " " + styles.Label.Render(fmt.Sprintf("[%d cont]", len(c.continuations)))
	}
	sb.WriteString(titleLine + "\n")

	// Chart
	sb.WriteString(lc.View())

	return sb.String()
}

// viewMultiSeries renders the chart with multiple data series using ntcharts
func (c *Chart) viewMultiSeries() string {
	if c.mode == RenderModeLinechart {
		return c.viewMultiSeriesLinechart()
	}
	return c.viewMultiSeriesBraillePoints()
}

func (c *Chart) viewMultiSeriesBraillePoints() string {
	// Reorder series if there's a highlight
	series := c.getOrderedSeries()

	// Find global min/max across all series
	var minVal, maxVal float64
	var minX, maxX float64
	var minTime, maxTime int64
	hasData := false
	hasTimestamps := false

	for _, s := range series {
		for _, p := range s.Points {
			val := p.Value
			if c.yAxisScale == YAxisScaleLog && val > 0 {
				val = applyLogScale(val)
			}

			if !hasData {
				minVal, maxVal = val, val
				minX, maxX = float64(p.Step), float64(p.Step)
				minTime, maxTime = p.Timestamp, p.Timestamp
				hasData = true
			} else {
				if val < minVal {
					minVal = val
				}
				if val > maxVal {
					maxVal = val
				}
				if float64(p.Step) < minX {
					minX = float64(p.Step)
				}
				if float64(p.Step) > maxX {
					maxX = float64(p.Step)
				}
				if p.Timestamp < minTime {
					minTime = p.Timestamp
				}
				if p.Timestamp > maxTime {
					maxTime = p.Timestamp
				}
			}
			if p.Timestamp > 0 {
				hasTimestamps = true
			}
		}
	}

	if !hasData {
		return styles.ChartTitle.Render(c.name) + "\n" +
			styles.Label.Render("  No data")
	}

	// Determine X axis range based on mode
	useRelativeTime := c.xAxisMode == XAxisModeRelativeTime && hasTimestamps
	if useRelativeTime {
		// Convert to relative seconds from first timestamp
		minX = 0
		maxX = float64(maxTime-minTime) / 1000.0 // Convert ms to seconds
	}

	// Add padding to range
	valRange := maxVal - minVal
	if valRange == 0 {
		valRange = 1
		minVal -= 0.5
		maxVal += 0.5
	}

	xRange := maxX - minX
	if xRange == 0 {
		xRange = 1
	}

	// Calculate chart dimensions - ntcharts handles axes internally
	// Reserve: 1 line for title, 1 line for legend
	chartHeight := c.height - 2
	if chartHeight < 3 {
		chartHeight = 3
	}
	chartWidth := c.width
	if chartWidth < 10 {
		chartWidth = 10
	}

	// Create ntcharts canvas
	cv := canvas.New(chartWidth, chartHeight)
	origin, graphWidth, graphHeight := graphSizeAndOrigin(
		chartWidth,
		chartHeight,
		minVal,
		maxVal,
		defaultXAxisStep,
		defaultYAxisStep,
	)

	// Draw each series with its color
	for _, s := range series {
		if len(s.Points) == 0 {
			continue
		}

		seriesStyle := lipgloss.NewStyle().Foreground(s.Color)
		grid := graph.NewBrailleGrid(graphWidth, graphHeight,
			minX, maxX,
			minVal, maxVal,
		)

		// Draw a braille dot for each point
		for i := 0; i < len(s.Points); i++ {
			pt := s.Points[i]
			var x float64
			if useRelativeTime {
				x = float64(pt.Timestamp-minTime) / 1000.0
			} else {
				x = float64(pt.Step)
			}
			y := pt.Value
			if c.yAxisScale == YAxisScaleLog && y > 0 {
				y = applyLogScale(y)
			}
			p := canvas.Float64Point{X: x, Y: y}
			grid.Set(grid.GridPoint(p))
		}

		graphTopLeft := canvas.Point{X: origin.X + 1, Y: 0}
		graph.DrawBraillePatterns(&cv, graphTopLeft, grid.BraillePatterns(), seriesStyle)
	}

	// Draw continuation markers as vertical dashed lines
	c.drawContinuationMarkers(&cv, origin, graphWidth, graphHeight, minX, maxX, useRelativeTime, minTime)

	c.drawAxesAndLabelsWithOptions(
		&cv,
		origin,
		graphWidth,
		graphHeight,
		minX,
		maxX,
		minVal,
		maxVal,
		defaultXAxisStep,
		defaultYAxisStep,
		useRelativeTime,
	)

	// Build output
	var sb strings.Builder

	// Title with mode indicators
	titleLine := styles.ChartTitle.Render(c.name)
	if c.yAxisScale == YAxisScaleLog {
		titleLine += " " + styles.Label.Render("[log]")
	}
	if useRelativeTime {
		titleLine += " " + styles.Label.Render("[time]")
	}
	if len(c.continuations) > 0 {
		titleLine += " " + styles.Label.Render(fmt.Sprintf("[%d cont]", len(c.continuations)))
	}
	sb.WriteString(titleLine + "\n")

	// Chart
	sb.WriteString(cv.View())
	sb.WriteString("\n")

	// Legend with highlight indicator
	var legendParts []string
	legendWidth := 0
	for i, s := range c.series {
		label := s.Label
		maxLabelLen := 15
		if len(label) > maxLabelLen {
			label = label[:maxLabelLen-3] + "..."
		}
		part := "━ " + label
		if legendWidth+len(part)+2 > chartWidth {
			break
		}
		lineStyle := lipgloss.NewStyle().Foreground(s.Color)
		legendItem := lineStyle.Render("━") + " " + label
		if i == c.highlightIndex {
			legendItem = "[" + legendItem + "]"
		}
		legendParts = append(legendParts, legendItem)
		legendWidth += len(part) + 2
	}
	if len(legendParts) > 0 {
		legend := strings.Join(legendParts, "  ")
		sb.WriteString(legend)
	}

	return sb.String()
}

// getOrderedSeries returns series in the correct order for drawing.
// The highlighted series (if any) is moved to the end so it's drawn on top.
func (c *Chart) getOrderedSeries() []DataSeries {
	if c.highlightIndex < 0 || c.highlightIndex >= len(c.series) {
		return c.series
	}

	// Create a new slice with highlighted series at the end
	result := make([]DataSeries, 0, len(c.series))
	for i, s := range c.series {
		if i != c.highlightIndex {
			result = append(result, s)
		}
	}
	result = append(result, c.series[c.highlightIndex])
	return result
}

func (c *Chart) viewMultiSeriesLinechart() string {
	// Reorder series if there's a highlight
	series := c.getOrderedSeries()

	// Find global min/max across all series
	var minVal, maxVal float64
	var minX, maxX float64
	var minTime, maxTime int64
	hasData := false
	hasTimestamps := false

	for _, s := range series {
		for _, p := range s.Points {
			val := p.Value
			if c.yAxisScale == YAxisScaleLog && val > 0 {
				val = applyLogScale(val)
			}

			if !hasData {
				minVal, maxVal = val, val
				minX, maxX = float64(p.Step), float64(p.Step)
				minTime, maxTime = p.Timestamp, p.Timestamp
				hasData = true
			} else {
				if val < minVal {
					minVal = val
				}
				if val > maxVal {
					maxVal = val
				}
				if float64(p.Step) < minX {
					minX = float64(p.Step)
				}
				if float64(p.Step) > maxX {
					maxX = float64(p.Step)
				}
				if p.Timestamp < minTime {
					minTime = p.Timestamp
				}
				if p.Timestamp > maxTime {
					maxTime = p.Timestamp
				}
			}
			if p.Timestamp > 0 {
				hasTimestamps = true
			}
		}
	}

	if !hasData {
		return styles.ChartTitle.Render(c.name) + "\n" +
			styles.Label.Render("  No data")
	}

	// Determine X axis range based on mode
	useRelativeTime := c.xAxisMode == XAxisModeRelativeTime && hasTimestamps
	if useRelativeTime {
		minX = 0
		maxX = float64(maxTime-minTime) / 1000.0
	}

	// Add padding to range
	valRange := maxVal - minVal
	if valRange == 0 {
		valRange = 1
		minVal -= 0.5
		maxVal += 0.5
	}

	xRange := maxX - minX
	if xRange == 0 {
		xRange = 1
	}

	// Calculate chart dimensions
	chartHeight := c.height - 2
	if chartHeight < 3 {
		chartHeight = 3
	}
	chartWidth := c.width
	if chartWidth < 10 {
		chartWidth = 10
	}

	// Create formatters based on settings with deduplication
	yDedup := &dedupingFormatter{
		format: func(v float64) string {
			if c.yAxisScale == YAxisScaleLog {
				return formatLogYLabel(v)
			}
			return formatYLabel(v)
		},
	}

	xDedup := &dedupingFormatter{
		format: func(v float64) string {
			if useRelativeTime {
				return formatTimeLabel(v)
			}
			return fmt.Sprintf("%.0f", v)
		},
	}

	// Create ntcharts linechart
	lc := linechart.New(chartWidth, chartHeight,
		minX, maxX,
		minVal, maxVal,
		linechart.WithStyles(styles.ChartAxis, styles.Label, styles.ChartLine),
		linechart.WithXYSteps(defaultXAxisStep, defaultYAxisStep),
		linechart.WithYLabelFormatter(yDedup.Format),
		linechart.WithXLabelFormatter(xDedup.Format),
	)

	// Draw each series with its color
	for _, s := range series {
		if len(s.Points) < 2 {
			continue
		}

		seriesStyle := lipgloss.NewStyle().Foreground(s.Color)

		// Draw lines between consecutive points
		for i := 1; i < len(s.Points); i++ {
			var x1, x2, y1, y2 float64
			if useRelativeTime {
				x1 = float64(s.Points[i-1].Timestamp-minTime) / 1000.0
				x2 = float64(s.Points[i].Timestamp-minTime) / 1000.0
			} else {
				x1 = float64(s.Points[i-1].Step)
				x2 = float64(s.Points[i].Step)
			}
			y1 = s.Points[i-1].Value
			y2 = s.Points[i].Value
			if c.yAxisScale == YAxisScaleLog {
				if y1 > 0 {
					y1 = applyLogScale(y1)
				}
				if y2 > 0 {
					y2 = applyLogScale(y2)
				}
			}
			p1 := canvas.Float64Point{X: x1, Y: y1}
			p2 := canvas.Float64Point{X: x2, Y: y2}
			lc.DrawBrailleLineWithStyle(p1, p2, seriesStyle)
		}
	}

	// Draw continuation markers as vertical dashed lines
	c.drawContinuationMarkers(&lc.Canvas, lc.Origin(), lc.GraphWidth(), lc.GraphHeight(), minX, maxX, useRelativeTime, minTime)

	// Draw axes
	lc.DrawXYAxisAndLabel()

	// Build output
	var sb strings.Builder

	// Title with mode indicators
	titleLine := styles.ChartTitle.Render(c.name)
	if c.yAxisScale == YAxisScaleLog {
		titleLine += " " + styles.Label.Render("[log]")
	}
	if useRelativeTime {
		titleLine += " " + styles.Label.Render("[time]")
	}
	if len(c.continuations) > 0 {
		titleLine += " " + styles.Label.Render(fmt.Sprintf("[%d cont]", len(c.continuations)))
	}
	sb.WriteString(titleLine + "\n")

	// Chart
	sb.WriteString(lc.View())
	sb.WriteString("\n")

	// Legend with highlight indicator
	var legendParts []string
	legendWidth := 0
	for i, s := range c.series {
		label := s.Label
		maxLabelLen := 15
		if len(label) > maxLabelLen {
			label = label[:maxLabelLen-3] + "..."
		}
		part := "━ " + label
		if legendWidth+len(part)+2 > chartWidth {
			break
		}
		lineStyle := lipgloss.NewStyle().Foreground(s.Color)
		legendItem := lineStyle.Render("━") + " " + label
		if i == c.highlightIndex {
			legendItem = "[" + legendItem + "]"
		}
		legendParts = append(legendParts, legendItem)
		legendWidth += len(part) + 2
	}
	if len(legendParts) > 0 {
		legend := strings.Join(legendParts, "  ")
		sb.WriteString(legend)
	}

	return sb.String()
}

// formatYLabel formats a Y-axis label to fit within 7 characters
func formatYLabel(v float64) string {
	yLabel := fmt.Sprintf("%7.2f", v)
	if math.Abs(v) >= 10000 {
		yLabel = fmt.Sprintf("%7.0f", v)
	} else if math.Abs(v) >= 1000 {
		yLabel = fmt.Sprintf("%7.1f", v)
	} else if math.Abs(v) < 0.01 && v != 0 {
		yLabel = fmt.Sprintf("%7.1e", v)
	}
	if len(yLabel) > 7 {
		yLabel = yLabel[:7]
	}
	return yLabel
}

// formatTimeLabel formats a relative time value (in seconds) for X-axis
func formatTimeLabel(seconds float64) string {
	if seconds < 60 {
		return fmt.Sprintf("%.0fs", seconds)
	} else if seconds < 3600 {
		return fmt.Sprintf("%.1fm", seconds/60)
	}
	return fmt.Sprintf("%.1fh", seconds/3600)
}

// dedupingFormatter creates a formatter that skips duplicate labels
type dedupingFormatter struct {
	lastLabel string
	format    func(float64) string
}

func (d *dedupingFormatter) Format(_ int, v float64) string {
	label := d.format(v)
	if label == d.lastLabel {
		return "" // Return empty to skip duplicate
	}
	d.lastLabel = label
	return label
}

// applyLogScale transforms a value to log scale (log10), handling zero/negative values
func applyLogScale(v float64) float64 {
	if v <= 0 {
		return 0 // Handle non-positive values
	}
	return math.Log10(v)
}

// inverseLogScale transforms a log-scale value back to linear
func inverseLogScale(v float64) float64 {
	return math.Pow(10, v)
}

// formatLogYLabel formats a Y-axis label for log scale display
func formatLogYLabel(logVal float64) string {
	// Convert from log scale back to actual value for display
	actualVal := inverseLogScale(logVal)
	return formatYLabel(actualVal)
}

func graphSizeAndOrigin(w, h int, minY, maxY float64, xStep, yStep int) (canvas.Point, int, int) {
	origin := canvas.Point{X: 0, Y: h - 1}
	graphWidth := w
	graphHeight := h
	if xStep > 0 {
		origin.Y -= 1
		graphHeight -= 2
	}
	if yStep > 0 {
		var lastVal string
		valueLen := 0
		rangeSz := maxY - minY
		increment := rangeSz / float64(graphHeight)
		for i := 0; i <= graphHeight; {
			v := minY + (increment * float64(i))
			s := formatYLabel(v)
			if lastVal != s {
				if len(s) > valueLen {
					valueLen = len(s)
				}
				lastVal = s
			}
			i += yStep
		}
		origin.X += valueLen
		graphWidth -= (valueLen + 1)
	}
	return origin, graphWidth, graphHeight
}

func drawAxesAndLabels(
	cv *canvas.Model,
	origin canvas.Point,
	graphWidth int,
	graphHeight int,
	minStep int64,
	maxStep int64,
	minVal float64,
	maxVal float64,
	xStep int,
	yStep int,
) {
	drawX := xStep > 0
	drawY := yStep > 0
	if drawX && drawY {
		graph.DrawXYAxis(cv, origin, styles.ChartAxis)
	} else {
		if drawY {
			graph.DrawVerticalLineUp(cv, origin, styles.ChartAxis)
		}
		if drawX {
			graph.DrawHorizonalLineRight(cv, origin, styles.ChartAxis)
		}
	}
	if drawY {
		drawYLabels(cv, origin, graphHeight, minVal, maxVal, yStep)
	}
	if drawX {
		drawXLabels(cv, origin, graphWidth, minStep, maxStep, xStep)
	}
}

func drawYLabels(cv *canvas.Model, origin canvas.Point, graphHeight int, minVal, maxVal float64, step int) {
	if step <= 0 {
		return
	}
	var lastVal string
	rangeSz := maxVal - minVal
	increment := rangeSz / float64(graphHeight)
	for i := 0; i <= graphHeight; {
		v := minVal + (increment * float64(i))
		s := formatYLabel(v)
		if lastVal != s {
			cv.SetStringWithStyle(canvas.Point{X: origin.X - len(s), Y: origin.Y - i}, s, styles.Label)
			lastVal = s
		}
		i += step
	}
}

func drawXLabels(cv *canvas.Model, origin canvas.Point, graphWidth int, minStep, maxStep int64, step int) {
	if step <= 0 {
		return
	}
	var lastVal string
	rangeSz := float64(maxStep - minStep)
	increment := rangeSz / float64(graphWidth)
	for i := 0; i < graphWidth; {
		if c := cv.Cell(canvas.Point{X: origin.X + i - 1, Y: origin.Y + 1}); c.Rune == 0 {
			v := float64(minStep) + (increment * float64(i))
			s := fmt.Sprintf("%.0f", v)
			sLen := len(s) + origin.X + i
			if (s != lastVal) && (sLen <= cv.Width()) {
				cv.SetStringWithStyle(canvas.Point{X: origin.X + i, Y: origin.Y + 1}, s, styles.Label)
				lastVal = s
			}
		}
		i += step
	}
}

// drawAxesAndLabelsWithOptions draws axes with support for time-based X axis and log Y axis
func (c *Chart) drawAxesAndLabelsWithOptions(
	cv *canvas.Model,
	origin canvas.Point,
	graphWidth int,
	graphHeight int,
	minX float64,
	maxX float64,
	minVal float64,
	maxVal float64,
	xStep int,
	yStep int,
	useRelativeTime bool,
) {
	drawX := xStep > 0
	drawY := yStep > 0
	if drawX && drawY {
		graph.DrawXYAxis(cv, origin, styles.ChartAxis)
	} else {
		if drawY {
			graph.DrawVerticalLineUp(cv, origin, styles.ChartAxis)
		}
		if drawX {
			graph.DrawHorizonalLineRight(cv, origin, styles.ChartAxis)
		}
	}
	if drawY {
		c.drawYLabelsWithOptions(cv, origin, graphHeight, minVal, maxVal, yStep)
	}
	if drawX {
		c.drawXLabelsWithOptions(cv, origin, graphWidth, minX, maxX, xStep, useRelativeTime)
	}
}

// drawYLabelsWithOptions draws Y axis labels with log scale support
func (c *Chart) drawYLabelsWithOptions(cv *canvas.Model, origin canvas.Point, graphHeight int, minVal, maxVal float64, step int) {
	if step <= 0 {
		return
	}
	var lastVal string
	rangeSz := maxVal - minVal
	increment := rangeSz / float64(graphHeight)
	for i := 0; i <= graphHeight; {
		v := minVal + (increment * float64(i))
		var s string
		if c.yAxisScale == YAxisScaleLog {
			s = formatLogYLabel(v)
		} else {
			s = formatYLabel(v)
		}
		if lastVal != s {
			cv.SetStringWithStyle(canvas.Point{X: origin.X - len(s), Y: origin.Y - i}, s, styles.Label)
			lastVal = s
		}
		i += step
	}
}

// drawXLabelsWithOptions draws X axis labels with time format support
func (c *Chart) drawXLabelsWithOptions(cv *canvas.Model, origin canvas.Point, graphWidth int, minX, maxX float64, step int, useRelativeTime bool) {
	if step <= 0 {
		return
	}
	var lastVal string
	rangeSz := maxX - minX
	increment := rangeSz / float64(graphWidth)
	for i := 0; i < graphWidth; {
		if cell := cv.Cell(canvas.Point{X: origin.X + i - 1, Y: origin.Y + 1}); cell.Rune == 0 {
			v := minX + (increment * float64(i))
			var s string
			if useRelativeTime {
				s = formatTimeLabel(v)
			} else {
				s = fmt.Sprintf("%.0f", v)
			}
			sLen := len(s) + origin.X + i
			if (s != lastVal) && (sLen <= cv.Width()) {
				cv.SetStringWithStyle(canvas.Point{X: origin.X + i, Y: origin.Y + 1}, s, styles.Label)
				lastVal = s
			}
		}
		i += step
	}
}

// drawContinuationMarkers draws vertical dashed lines at continuation points
func (c *Chart) drawContinuationMarkers(cv *canvas.Model, origin canvas.Point, graphWidth, graphHeight int, minX, maxX float64, useRelativeTime bool, minTime int64) {
	if len(c.continuations) == 0 {
		return
	}

	markerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244")) // Gray color
	xRange := maxX - minX
	if xRange == 0 {
		return
	}

	for _, cont := range c.continuations {
		// Calculate x position for this continuation
		var xPos float64
		if useRelativeTime && cont.Timestamp > 0 {
			// In relative time mode, use timestamp converted to seconds from start
			xPos = float64(cont.Timestamp-minTime) / 1000.0
		} else {
			// In step mode, use the step value
			xPos = float64(cont.Step)
		}
		if xPos < minX || xPos > maxX {
			continue // Skip if outside visible range
		}

		// Convert to screen coordinates
		screenX := origin.X + 1 + int(float64(graphWidth)*(xPos-minX)/xRange)
		if screenX <= origin.X || screenX >= origin.X+graphWidth+1 {
			continue
		}

		// Draw dashed vertical line (alternating characters)
		for y := 0; y < graphHeight; y++ {
			screenY := origin.Y - y - 1
			if screenY < 0 {
				continue
			}
			// Dashed pattern: draw every other character
			if y%2 == 0 {
				cv.SetStringWithStyle(canvas.Point{X: screenX, Y: screenY}, "┆", markerStyle)
			}
		}

		// Draw a marker at the top
		if origin.Y-graphHeight >= 0 {
			cv.SetStringWithStyle(canvas.Point{X: screenX, Y: origin.Y - graphHeight}, "↻", markerStyle)
		}
	}
}

// Name returns the chart name
func (c *Chart) Name() string {
	return c.name
}

// LatestValue returns the latest value, or 0 if no data
func (c *Chart) LatestValue() float64 {
	if len(c.data) == 0 {
		return 0
	}
	return c.data[len(c.data)-1].Value
}

// PointCount returns the number of data points
func (c *Chart) PointCount() int {
	return len(c.data)
}
