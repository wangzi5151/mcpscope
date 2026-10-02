// Package tui implements the mcpscope terminal interface: browse an MCP
// server's tools, resources and prompts, call tools with JSON arguments,
// and review call history.
package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C6FF7")).
			Padding(0, 1)

	tabActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#7C6FF7")).
			Padding(0, 2)

	tabInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#8A8A93")).
				Padding(0, 2)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#3A3A44")).
			Padding(0, 1)

	panelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#B8B8C0"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6E6E78"))

	errStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Bold(true)

	okStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#51CF66")).
		Bold(true)

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8A8A93"))

	keyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD43B")).
			Bold(true)
)

// keyHelp renders a "key action" hint pair.
func keyHelp(key, action string) string {
	return keyStyle.Render(key) + " " + helpStyle.Render(action)
}
