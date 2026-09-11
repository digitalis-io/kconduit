package ui

import (
	"errors"
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/digitalis-io/kconduit/pkg/logger"
)

// clipboardMsg reports the outcome of a copy so the caller can toast it.
type clipboardMsg struct {
	what string
	err  error
}

// copyTextCmd puts text on the system clipboard. what names the thing copied
// and is used in the notification, e.g. "topic name".
func copyTextCmd(text, what string) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(text) == "" {
			return clipboardMsg{what: what, err: errNothingToCopy}
		}
		err := clipboard.WriteAll(text)
		if err != nil {
			logger.Get().WithError(err).Debug("Failed to write to clipboard")
		}
		return clipboardMsg{what: what, err: err}
	}
}

// copyRowCmd copies a whole table row as tab-separated fields, which pastes
// straight into a spreadsheet or a ticket.
func copyRowCmd(row []string) tea.Cmd {
	return copyTextCmd(strings.Join(row, "\t"), "row")
}

// errNothingToCopy is returned when there is no selection to copy.
var errNothingToCopy = errors.New("nothing selected to copy")
