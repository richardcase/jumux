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
	footerStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	errStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

func statusStyle(status string) lipgloss.Style {
	switch status {
	case "dirty":
		return dirtyStyle
	case "clean":
		return cleanStyle
	}
	return unknownStyle
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render(truncate("agentmux", m.width)))
	b.WriteString("\n\n")

	if len(m.items) == 0 {
		b.WriteString(unknownStyle.Render(truncate("no agents", m.width)))
		b.WriteString("\n")
	}
	for i, it := range m.items {
		marker := " "
		if it.Activity {
			marker = activityStyle.Render("!")
		}
		label := truncate(it.Label, m.width-len("  ! dirty  "))
		line := marker + " " + label + "  " + statusStyle(it.Status).Render(it.Status)
		if i == m.cursor {
			line = selectedStyle.Render("▸") + line[1:]
		}
		b.WriteString(truncateANSIAware(line, m.width))
		b.WriteString("\n")
	}

	footer := "j/k move · ⏎ jump · q quit"
	if m.err != nil {
		footer = errStyle.Render(truncate(m.err.Error(), m.width))
	} else {
		footer = footerStyle.Render(truncate(footer, m.width))
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
