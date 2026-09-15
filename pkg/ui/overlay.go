package ui

import "github.com/charmbracelet/lipgloss"

// renderOverlay centres box over the full terminal area.
//
// Bubble Tea has no compositor, so an overlay is really just a full-screen
// string with the box placed in the middle of it. lipgloss.Place understands
// ANSI sequences, which byte-slicing the background does not, so the background
// is dropped rather than risking a torn escape sequence mid-line.
func renderOverlay(box string, width, height int) string {
	if width <= 0 || height <= 0 {
		return box
	}
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

// overlayBoxStyle is the frame used by every overlay (help, toasts, detail
// panes) so they read as the same kind of thing.
func overlayBoxStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(theme.Primary).
		Padding(1, 2).
		Width(width)
}
