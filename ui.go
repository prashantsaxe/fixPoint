package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	breakpointHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#005F87", Dark: "#7DD3FC"})

	lineNumberStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"})

	sourceLineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#111827", Dark: "#E5E7EB"})

	highlightedSourceStyle = lipgloss.NewStyle().
				Background(lipgloss.AdaptiveColor{Light: "#DBEAFE", Dark: "#1E3A8A"}).
				Foreground(lipgloss.AdaptiveColor{Light: "#111827", Dark: "#F9FAFB"}).
				Bold(true)

	aiCardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#4F46E5", Dark: "#818CF8"}).
			Background(lipgloss.AdaptiveColor{Light: "#EEF2FF", Dark: "#1F2340"}).
			Foreground(lipgloss.AdaptiveColor{Light: "#111827", Dark: "#E5E7EB"}).
			Padding(1, 2)

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#9A3412", Dark: "#FDBA74"}).
			Bold(true)
)

func RenderBreakpointHeader(reason string, threadID int) string {
	return breakpointHeaderStyle.Render(fmt.Sprintf("🎯 Breakpoint Hit! Reason: %s | ThreadId: %d", reason, threadID))
}

func RenderSourceWindow(ctx *DebugContext) string {
	if ctx == nil || len(ctx.SourceSnippet) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(breakpointHeaderStyle.Render("Source Window"))
	b.WriteString("\n")

	for _, line := range ctx.SourceSnippet {
		lineNo := lineNumberStyle.Render(fmt.Sprintf("%4d", line.LineNumber))
		prefix := "  "
		text := sourceLineStyle.Render(line.Text)

		if line.LineNumber == ctx.SourceLine {
			prefix = "> "
			text = highlightedSourceStyle.Render(line.Text)
		}

		b.WriteString(fmt.Sprintf("%s%s | %s\n", prefix, lineNo, text))
	}

	return strings.TrimRight(b.String(), "\n")
}

func RenderAIResponseCard(analysis string) string {
	title := breakpointHeaderStyle.Render("🤖 FixPoint AI Analysis")
	body := aiCardStyle.Render(strings.TrimSpace(analysis))
	return title + "\n" + body
}

func RenderWarning(message string) string {
	return warningStyle.Render("⚠ " + message)
}
