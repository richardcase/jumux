package sidebar

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle   = lipgloss.NewStyle().Bold(true)
	selectedStyle = lipgloss.NewStyle().Reverse(true)
	dirtyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow
	cleanStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // green
	unknownStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8")) // dim
	activityStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("5")) // magenta
	workingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("6")) // cyan
	footerStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	errStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// agentIcon renders the agent-liveness column: an animated spinner while the
// agent works, ? while it waits for input, ✓ when done, · with no data.
func agentIcon(agent string, frame int) string {
	switch agent {
	case "working":
		return workingStyle.Render(spinnerFrames[frame%len(spinnerFrames)])
	case "waiting":
		return dirtyStyle.Render("?")
	case "done":
		return cleanStyle.Render("✓")
	}
	return unknownStyle.Render("·")
}

// jjIcon renders the working-copy column: ✓ clean, ● dirty, ? unknown.
func jjIcon(status string) string {
	switch status {
	case "clean":
		return cleanStyle.Render("✓")
	case "dirty":
		return dirtyStyle.Render("●")
	}
	return unknownStyle.Render("?")
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render(truncate("jumux", m.width)))
	b.WriteString("\n\n")

	if len(m.items) == 0 {
		b.WriteString(unknownStyle.Render(truncate("no agents", m.width)))
		b.WriteString("\n")
	}
	// Row: cursor(1) activity(1) sp(1) agent(1) sp(1) label gap(≥2) jj(1).
	const rowOverhead = 8
	for i, it := range m.items {
		cursor := " "
		if i == m.cursor {
			cursor = selectedStyle.Render("▸")
		}
		marker := " "
		if it.Activity {
			marker = activityStyle.Render("!")
		}
		label := truncate(it.Label, m.width-rowOverhead)
		pad := max(0, m.width-rowOverhead-len([]rune(label)))
		line := cursor + marker + " " + agentIcon(it.Agent, m.frame) + " " +
			label + strings.Repeat(" ", pad+2) + jjIcon(it.Status)
		b.WriteString(truncateANSIAware(line, m.width))
		b.WriteString("\n")
	}

	var footer string
	switch {
	case m.confirming:
		footer = errStyle.Render(truncate("remove '"+m.pendingRemove.Label+"'? y/n", m.width))
	case m.err != nil:
		footer = errStyle.Render(truncate(m.err.Error(), m.width))
	default:
		footer = footerStyle.Render(truncate("j/k move · ⏎ jump · d remove · q quit", m.width))
	}
	b.WriteString("\n")
	b.WriteString(footer)
	return b.String()
}

// truncate hard-limits a plain (unstyled) string to width columns.
func truncate(s string, width int) string {
	if width <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	return string(r[:width-1]) + "…"
}

// truncateANSIAware limits a styled line to width columns without cutting
// inside escape sequences.
func truncateANSIAware(s string, width int) string {
	if width <= 0 || lipgloss.Width(s) <= width {
		return s
	}
	// Drop trailing runes until it fits; escape sequences have zero width so
	// lipgloss.Width converges.
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r)) > width {
		r = r[:len(r)-1]
	}
	return string(r)
}
