package views

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"sixtyseven/internal/tui/styles"
)

func renderPanelBox(width, height int, borderColor lipgloss.Color, title string, content string) string {
	if width < 2 {
		return content
	}

	border := lipgloss.RoundedBorder()
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)

	innerWidth := width - 2
	innerStyle := lipgloss.NewStyle().
		Width(innerWidth).
		Padding(0, 1)

	if height > 0 {
		innerHeight := height - 2
		if innerHeight < 0 {
			innerHeight = 0
		}
		innerStyle = innerStyle.Height(innerHeight)
	}

	inner := innerStyle.Render(content)

	lines := strings.Split(inner, "\n")
	innerHeight := len(lines)

	rawTitle := ""
	if title != "" {
		rawTitle = " " + title + " "
		rawTitle = truncateToWidth(rawTitle, innerWidth)
	}

	topLine := ""
	if rawTitle == "" {
		topLine = borderStyle.Render(border.TopLeft) +
			borderStyle.Render(strings.Repeat(border.Top, innerWidth)) +
			borderStyle.Render(border.TopRight)
	} else {
		titleWidth := lipgloss.Width(rawTitle)
		fillWidth := innerWidth - titleWidth
		if fillWidth < 0 {
			fillWidth = 0
		}
		topLine = borderStyle.Render(border.TopLeft) +
			styles.Subtitle.Render(rawTitle) +
			borderStyle.Render(strings.Repeat(border.Top, fillWidth)) +
			borderStyle.Render(border.TopRight)
	}

	bottomLine := borderStyle.Render(border.BottomLeft) +
		borderStyle.Render(strings.Repeat(border.Bottom, innerWidth)) +
		borderStyle.Render(border.BottomRight)

	out := make([]string, 0, innerHeight+2)
	out = append(out, topLine)
	for _, line := range lines {
		out = append(out, borderStyle.Render(border.Left)+line+borderStyle.Render(border.Right))
	}
	out = append(out, bottomLine)

	return strings.Join(out, "\n")
}

func truncateToWidth(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	runes := []rune(s)
	if len(runes) > maxLen-3 {
		runes = runes[:maxLen-3]
	}
	return string(runes) + "..."
}
