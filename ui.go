package fixpoint

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	baseStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#1E293B", Dark: "#E2E8F0"})

	breakpointHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#6366F1", Dark: "#A5B4FC"}).
				MarginBottom(1)

	sourceCardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#CBD5E1", Dark: "#334155"}).
			Background(lipgloss.AdaptiveColor{Light: "#F8FAFC", Dark: "#0F172A"}).
			Padding(0, 1, 1, 1)

	lineNumberStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#94A3B8", Dark: "#475569"}).
			Width(4).
			Align(lipgloss.Right)

	separatorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#CBD5E1", Dark: "#334155"})

	sourceLineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#334155", Dark: "#CBD5E1"})

	highlightedSourceStyle = lipgloss.NewStyle().
				Background(lipgloss.AdaptiveColor{Light: "#E0E7FF", Dark: "#1E1B4B"}).
				Foreground(lipgloss.AdaptiveColor{Light: "#3730A3", Dark: "#C7D2FE"}).
				Bold(true)

	aiPanelStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#6366F1", Dark: "#818CF8"}).
			Background(lipgloss.AdaptiveColor{Light: "#EEF2FF", Dark: "#1E1B4B"}).
			Foreground(lipgloss.AdaptiveColor{Light: "#1E293B", Dark: "#E2E8F0"}).
			Padding(1, 2).
			Width(80)

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#D97706", Dark: "#FBBF24"}).
			Bold(true)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}).
			Italic(true)

	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#6366F1", Dark: "#A5B4FC"}).
			Bold(true)
)

func RenderBreakpointHeader(reason string, threadID int) string {
	symbol := ""
	switch strings.ToLower(reason) {
	case "breakpoint":
		symbol = "breakpoint"
	case "exception", "panic", "error":
		symbol = "exception"
	case "step":
		symbol = "step"
	default:
		symbol = reason
	}
	return breakpointHeaderStyle.Render(fmt.Sprintf("  %s  threadId=%d", symbol, threadID))
}

func RenderSourceWindow(ctx *DebugContext) string {
	if ctx == nil || len(ctx.SourceSnippet) == 0 {
		return ""
	}

	var b strings.Builder
	for _, line := range ctx.SourceSnippet {
		lineNo := lineNumberStyle.Render(fmt.Sprintf("%d", line.LineNumber))
		sep := separatorStyle.Render("|")
		text := sourceLineStyle.Render(line.Text)

		if line.LineNumber == ctx.SourceLine {
			text = highlightedSourceStyle.Render(line.Text)
			b.WriteString(fmt.Sprintf("> %s %s %s\n", lineNo, sep, text))
		} else {
			b.WriteString(fmt.Sprintf("  %s %s %s\n", lineNo, sep, text))
		}
	}

	return sourceCardStyle.Render(strings.TrimRight(b.String(), "\n"))
}

func RenderAIResponseCard(analysis string) string {
	var b strings.Builder
	b.WriteString(breakpointHeaderStyle.Render("  analysis"))
	b.WriteString("\n")
	b.WriteString(aiPanelStyle.Render(strings.TrimSpace(analysis)))
	return b.String()
}

func RenderWarning(message string) string {
	return warningStyle.Render(message)
}

func RenderInfo(message string) string {
	return infoStyle.Render(message)
}

func RenderPrompt(message string) string {
	return promptStyle.Render(message)
}
