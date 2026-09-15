package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// autoRefreshInterval is how often the active tab reloads while auto-refresh is
// on. Five seconds is short enough to watch lag move and long enough that a
// large cluster is not being re-listed constantly.
const autoRefreshInterval = 5 * time.Second

type autoRefreshTickMsg struct{}

func autoRefreshTick() tea.Cmd {
	return tea.Tick(autoRefreshInterval, func(time.Time) tea.Msg {
		return autoRefreshTickMsg{}
	})
}

// refreshActiveTab reloads only the data the current tab shows.
//
// It does not set m.loading: an automatic refresh must not replace the table
// with a spinner every five seconds, and a manual "r" sets the flag itself.
//
// Only the tab's primary list is fetched here. The things derived from it
// follow on their own: topic metrics and the selected topic's config from
// topicsMsg, cluster statistics from brokersMsg.
func (m Model) refreshActiveTab() tea.Cmd {
	switch m.activeTab {
	case ACLsTab:
		return fetchACLs(m.client)
	case ConsumerGroupsTab:
		return fetchConsumerGroups(m.client)
	case TopicsTab:
		return fetchTopics(m.client)
	default:
		return fetchBrokers(m.client)
	}
}

// updateFilterKey handles a key press while the filter input has focus.
//
// esc abandons the filter and restores the full list; enter keeps it and hands
// focus back to the table, so the arrow keys move the selection again.
func (m Model) updateFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	state := m.tabStates[m.activeTab]

	switch msg.String() {
	case "esc":
		m.tabStates[m.activeTab] = state.stopFilter(true)
		m.rebuildActiveTab()
		return m, nil
	case "enter":
		m.tabStates[m.activeTab] = state.stopFilter(false)
		return m, nil
	}

	var cmd tea.Cmd
	state.filter, cmd = state.filter.Update(msg)
	m.tabStates[m.activeTab] = state
	m.rebuildActiveTab()
	return m, cmd
}

// backToList dismisses a sub-view and returns to the list.
//
// reload is the data the list needs to pick up whatever the sub-view changed,
// and label is the spinner text while it loads — empty for a sub-view that
// changed nothing, so returning from, say, the session manager does not flash a
// spinner over a table that is already correct.
func (m Model) backToList(msg SwitchToListViewMsg, reload tea.Cmd, label string) (tea.Model, tea.Cmd) {
	m.mode = ListView

	if label != "" {
		m.loading = true
		m.loadingLabel = label
	}

	var cmds []tea.Cmd
	if reload != nil {
		cmds = append(cmds, reload)
	}
	if msg.Notice != "" {
		var cmd tea.Cmd
		m.toast, cmd = m.toast.show(msg.Level, msg.Notice)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

// handleClipboardMsg turns the outcome of a copy into a notification.
//
// Both the list view and the lag view offer "y", so both need this; it lives
// here once rather than in each of their update functions.
func (m Model) handleClipboardMsg(msg clipboardMsg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	if msg.err != nil {
		m.toast, cmd = m.toast.show(toastError, "Copy failed: "+msg.err.Error())
	} else {
		m.toast, cmd = m.toast.show(toastSuccess, "Copied "+msg.what+" to the clipboard")
	}
	return m, cmd
}
