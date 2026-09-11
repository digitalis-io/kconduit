package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// toastDuration is how long a notification stays on screen. Long enough to read
// a topic name, short enough not to sit over the table.
const toastDuration = 4 * time.Second

type toastLevel int

const (
	toastInfo toastLevel = iota
	toastSuccess
	toastError
)

// toast is a transient one-line notification shown above the status bar.
//
// Each toast carries a sequence number. A later toast replacing an earlier one
// bumps the number, so the earlier one's expiry tick arrives with a stale
// number and is ignored rather than clearing the new message.
type toast struct {
	message string
	level   toastLevel
	seq     int
}

// toastExpiredMsg is delivered when a toast's lifetime is up.
type toastExpiredMsg struct{ seq int }

// show replaces the current toast and returns the command that expires it.
func (t toast) show(level toastLevel, message string) (toast, tea.Cmd) {
	next := toast{message: message, level: level, seq: t.seq + 1}
	return next, tea.Tick(toastDuration, func(time.Time) tea.Msg {
		return toastExpiredMsg{seq: next.seq}
	})
}

// expire clears the toast if the tick belongs to the message on screen.
func (t toast) expire(msg toastExpiredMsg) toast {
	if msg.seq == t.seq {
		t.message = ""
	}
	return t
}

// visible reports whether there is anything to draw.
func (t toast) visible() bool { return t.message != "" }

// View renders the toast as a full-width bar.
func (t toast) View(width int) string {
	if !t.visible() {
		return ""
	}

	icon, colour := "•", theme.Info
	switch t.level {
	case toastSuccess:
		icon, colour = "✓", theme.Success
	case toastError:
		icon, colour = "✗", theme.Error
	case toastInfo:
		icon, colour = "•", theme.Info
	}

	return lipgloss.NewStyle().
		Foreground(colour).
		Background(theme.BgDark).
		Bold(true).
		Padding(0, 1).
		Width(max(width, 1)).
		MaxHeight(1).
		Render(icon + "  " + t.message)
}
