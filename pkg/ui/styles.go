package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Theme defines the color palette and reusable styles used across the TUI.
// Centralizing these ensures visual consistency.
var theme = struct {
	// Brand colors
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Accent    lipgloss.Color

	// Semantic colors
	Success lipgloss.Color
	Error   lipgloss.Color
	Warning lipgloss.Color
	Info    lipgloss.Color

	// Neutral tones
	Text      lipgloss.Color
	SubText   lipgloss.Color
	Muted     lipgloss.Color
	Border    lipgloss.Color
	DimBorder lipgloss.Color
	BgDark    lipgloss.Color
}{
	Primary:   lipgloss.Color("205"), // magenta/pink
	Secondary: lipgloss.Color("57"),  // purple
	Accent:    lipgloss.Color("86"),  // teal/cyan

	Success: lipgloss.Color("78"),  // green
	Error:   lipgloss.Color("203"), // red
	Warning: lipgloss.Color("214"), // orange
	Info:    lipgloss.Color("75"),  // blue

	Text:      lipgloss.Color("252"), // bright white
	SubText:   lipgloss.Color("245"), // light gray
	Muted:     lipgloss.Color("240"), // dim gray
	Border:    lipgloss.Color("238"), // subtle border
	DimBorder: lipgloss.Color("236"), // very dim
	BgDark:    lipgloss.Color("235"), // dark background
}

// --- Reusable styles ---

// appHeaderStyle renders the top-level application title bar.
var appHeaderStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(theme.Primary).
	PaddingLeft(1)

// activeTabStyle renders a selected tab with visual emphasis.
var activeTabStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("229")).
	Background(theme.Secondary).
	Padding(0, 2).
	BorderStyle(lipgloss.Border{
		Top:         "─",
		Bottom:      " ",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "┘",
		BottomRight: "└",
	}).
	BorderForeground(theme.Primary).
	BorderTop(true).
	BorderLeft(true).
	BorderRight(true).
	BorderBottom(true)

// inactiveTabStyle renders an unselected tab.
var inactiveTabStyle = lipgloss.NewStyle().
	Foreground(theme.Muted).
	Padding(0, 2).
	BorderStyle(lipgloss.Border{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "┴",
		BottomRight: "┴",
	}).
	BorderForeground(theme.Border).
	BorderTop(true).
	BorderLeft(true).
	BorderRight(true).
	BorderBottom(true)

// tabGapStyle fills the gap beneath inactive tabs so the tab bar
// has a continuous bottom border.
var tabGapStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.Border{
		Bottom: "─",
	}).
	BorderForeground(theme.Border).
	BorderBottom(true)

// panelStyle renders main content panels with a rounded border.
var panelStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(theme.Border)

// activePanelStyle renders a focused panel with a highlighted border.
var activePanelStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(theme.Primary)

// sectionTitleStyle renders section headings inside panels.
var sectionTitleStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(theme.Primary).
	MarginBottom(1)

// labelStyle renders dim labels for key-value displays.
var labelStyle = lipgloss.NewStyle().
	Foreground(theme.SubText)

// valueStyle renders bright values alongside labels.
var valueStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(theme.Text)

// successStyle renders positive status indicators.
var successStyle = lipgloss.NewStyle().
	Foreground(theme.Success)

// errorStyle renders error messages and negative indicators.
var errorStyle = lipgloss.NewStyle().
	Foreground(theme.Error)

// warningStyle renders warning text.
var warningStyle = lipgloss.NewStyle().
	Foreground(theme.Warning)

// helpKeyStyle renders a key binding in the help bar.
var helpKeyStyle = lipgloss.NewStyle().
	Foreground(theme.Accent).
	Bold(true)

// helpDescStyle renders the description next to a key in the help bar.
var helpDescStyle = lipgloss.NewStyle().
	Foreground(theme.Muted)

// helpSepStyle renders the separator between help items.
var helpSepStyle = lipgloss.NewStyle().
	Foreground(theme.DimBorder)

// dialogBoxStyle renders centered modal dialogs.
var dialogBoxStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(theme.Primary).
	Padding(1, 3).
	Width(54)

// statusBarStyle renders the bottom status bar.
var statusBarStyle = lipgloss.NewStyle().
	Foreground(theme.SubText).
	Background(theme.BgDark).
	Padding(0, 1)

// statusBarModeStyle renders the current mode label in the status bar.
var statusBarModeStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("229")).
	Background(theme.Secondary).
	Bold(true).
	Padding(0, 1)

// tableHeaderStyle returns the standard table header style.
func tableHeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(theme.Border).
		BorderBottom(true).
		Bold(true).
		Foreground(theme.SubText)
}

// tableSelectedStyle returns the standard row-selected style.
func tableSelectedStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("229")).
		Background(theme.Secondary).
		Bold(false)
}

// renderHelpBar formats key bindings into a styled help line.
// items should alternate: key, description, key, description, ...
func renderHelpBar(items ...string) string {
	var parts []string
	sep := helpSepStyle.Render(" │ ")
	for i := 0; i+1 < len(items); i += 2 {
		parts = append(parts, helpKeyStyle.Render(items[i])+" "+helpDescStyle.Render(items[i+1]))
	}
	var sb strings.Builder
	for i, p := range parts {
		if i > 0 {
			sb.WriteString(sep)
		}
		sb.WriteString(p)
	}
	return sb.String()
}
