package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// helpTitle names the window and, importantly, says how to close it again.
const helpTitle = "Keyboard Shortcuts  ·  ↑↓ PgUp/PgDn scroll  ·  esc closes"

// helpMargin is how much of the screen stays visible around the window, so it
// reads as something drawn over KConduit rather than a separate screen.
const helpMargin = 6

// helpBinding is one row of the reference. An empty Category repeats the
// category above it, which is how the rows are grouped without a separate
// nesting structure.
type helpBinding struct {
	Category string
	Key      string
	Desc     string
}

// helpBindings is the single description of every binding in the list view.
// The overlay renders it; new bindings belong here so the two cannot drift.
var helpBindings = []helpBinding{
	{"Navigation", "1 – 4", "Jump to Brokers, Topics, Groups, ACLs"},
	{"", "tab / shift+tab", "Next or previous tab (panel, in Topics)"},
	{"", "↑ / ↓", "Move the selection"},
	{"", "g / G", "First or last row"},
	{"", "esc", "Back / close"},
	{"", "q", "Quit"},

	{"Finding things", "/", "Filter the current table"},
	{"", "< / >", "Step the sort order: column, then direction"},
	{"", "y", "Copy the selection to the clipboard"},

	{"Data", "r", "Refresh the current tab"},
	{"", "ctrl+r", "Toggle auto-refresh (5s)"},
	{"", "enter", "Consume topic · group lag · broker detail"},
	{"", "p", "Produce a message to the selected topic"},

	{"Changing things", "C", "Create a topic (or an ACL, in the ACLs tab)"},
	{"", "e", "Edit the selected config value or ACL"},
	{"", "d", "Delete the selected topic or ACL"},

	{"Tools", "s", "Session manager"},
	{"", "a", "AI assistant"},
	{"", "? / F1", "This window"},
}

// helpOverlay is the open help, if any. Only the scroll position is state; the
// rows are rebuilt from helpBindings on every draw.
type helpOverlay struct {
	active bool
	scroll int
}

// helpLines lays the bindings out one per line with the columns aligned.
func helpLines() []string {
	catWidth, keyWidth := 0, 0
	for _, b := range helpBindings {
		catWidth = max(catWidth, lipgloss.Width(b.Category))
		keyWidth = max(keyWidth, lipgloss.Width(b.Key))
	}

	lines := make([]string, 0, len(helpBindings))
	for _, b := range helpBindings {
		cat := sectionTitleStyle.UnsetMarginBottom().Render(pad(b.Category, catWidth))
		if b.Category == "" {
			cat = strings.Repeat(" ", catWidth)
		}
		lines = append(lines,
			cat+"  "+helpKeyStyle.Render(pad(b.Key, keyWidth))+"  "+labelStyle.Render(b.Desc))
	}
	return lines
}

// pad right-pads s to width, measuring display width rather than bytes.
func pad(s string, width int) string {
	if gap := width - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

// visibleRows is how many binding rows fit, given the screen height.
func (h helpOverlay) visibleRows(screenHeight int) int {
	// Border (2), padding (2), title plus its blank line (2).
	return max(screenHeight-helpMargin-6, 3)
}

// clampScroll keeps the scroll position inside the list. Both the key handler
// and the renderer go through it, so the scrollbar cannot describe a window
// other than the one on screen.
func (h helpOverlay) clampScroll(screenHeight int) int {
	return min(max(h.scroll, 0), max(len(helpBindings)-h.visibleRows(screenHeight), 0))
}

// Update handles a key press while the help window is open. The second return
// value reports whether the window is still open.
func (h helpOverlay) Update(msg tea.KeyMsg, screenHeight int) (helpOverlay, bool) {
	rows := h.visibleRows(screenHeight)

	switch msg.String() {
	case "esc", "?", "q", "f1":
		h.active = false
		h.scroll = 0
		return h, false
	case "up", "k":
		h.scroll--
	case "down", "j":
		h.scroll++
	case "pgup":
		h.scroll -= rows
	case "pgdown", " ":
		h.scroll += rows
	case "home", "g":
		h.scroll = 0
	case "end", "G":
		h.scroll = len(helpBindings)
	}

	h.scroll = h.clampScroll(screenHeight)
	return h, true
}

// View renders the help window centred over the whole screen.
func (h helpOverlay) View(width, height int) string {
	lines := helpLines()
	rows := h.visibleRows(height)
	first := h.clampScroll(height)
	last := min(first+rows, len(lines))

	body := strings.Join(lines[first:last], "\n")
	if len(lines) > rows {
		body += "\n\n" + helpDescStyle.Render(scrollHint(first, last, len(lines)))
	}

	boxWidth := min(width-helpMargin, longestLine(lines)+4)
	box := overlayBoxStyle(boxWidth).Render(
		sectionTitleStyle.Render(helpTitle) + "\n" + body,
	)

	return renderOverlay(box, width, height)
}

// scrollHint says which slice of the list is on screen.
func scrollHint(first, last, total int) string {
	return fmt.Sprintf("showing %d–%d of %d", first+1, last, total)
}

// longestLine is the display width of the widest line.
func longestLine(lines []string) int {
	widest := 0
	for _, line := range lines {
		widest = max(widest, lipgloss.Width(line))
	}
	return widest
}
