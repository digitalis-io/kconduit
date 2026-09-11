package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/digitalis-io/kconduit/pkg/kafka"
)

// Row building for the four list-view tables.
//
// Each tab keeps its rows twice: m.<x>Rows holds everything fetched, and the
// bubbles table holds the filtered and sorted view of it. Typing in a filter or
// moving the sort column re-derives the second from the first, so neither costs
// a request to the cluster.
//
// Every function here takes a pointer receiver. On a value receiver the
// SetRows call lands on a copy and the table silently keeps its old rows.

type topicMetricsMsg struct {
	metrics map[string]kafka.TopicMetrics
	err     error
}

// fetchTopicMetrics measures message counts and disk usage. It is a separate
// command from fetchTopics because it is O(partitions) requests: the list
// paints first and the numbers fill in when they arrive.
func fetchTopicMetrics(client *kafka.Client) tea.Cmd {
	return func() tea.Msg {
		metrics, err := client.GetTopicMetrics()
		return topicMetricsMsg{metrics: metrics, err: err}
	}
}

// humanBytes renders a byte count at a readable scale.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for size := n / unit; size >= unit && exp < 4; size /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTP"[exp])
}

// topicColumns widens the topic name to whatever the table's width allows,
// because a truncated topic name is the one column that makes the row useless.
func topicColumns(tableWidth int) []table.Column {
	cols := make([]table.Column, len(topicsColumns))
	copy(cols, topicsColumns)

	fixed := 0
	for _, c := range cols[1:] {
		fixed += c.Width + 2 // bubbles pads each cell by one column either side
	}
	if name := tableWidth - fixed - 2; name > cols[0].Width {
		cols[0].Width = name
	}
	return cols
}

func (m *Model) rebuildTopicRows() {
	rows := make([]table.Row, 0, len(m.topics))
	for _, topic := range m.topics {
		messages, disk := "…", "…"
		if metric, ok := m.topicMetrics[topic.Name]; ok {
			messages = fmt.Sprintf("%d", metric.Messages)
			disk = humanBytes(metric.DiskBytes)
		}
		rows = append(rows, safeRow(
			topic.Name,
			fmt.Sprintf("%d", topic.Partitions),
			fmt.Sprintf("%d", topic.ReplicationFactor),
			messages,
			disk,
		))
	}
	m.topicRows = rows

	state := m.tabStates[TopicsTab]
	m.topicsTable.SetColumns(decorateColumns(topicColumns(m.topicsTable.Width()), state))
	m.topicsTable.SetRows(state.applyView(rows))
}

func (m *Model) rebuildBrokerRows() {
	rows := make([]table.Row, 0, len(m.brokers))
	for _, broker := range m.brokers {
		role := "Broker"
		if broker.IsController {
			role = "✅ Controller"
		}

		rack := broker.Rack
		if rack == "" {
			rack = "-"
		}

		version := broker.ApiVersions
		if version == "" {
			version = "Unknown"
		}

		logDirs := "-"
		if broker.LogDirCount > 0 {
			logDirs = fmt.Sprintf("%d", broker.LogDirCount)
		}

		rows = append(rows, safeRow(
			fmt.Sprintf("%d", broker.ID),
			broker.Host,
			fmt.Sprintf("%d", broker.Port),
			broker.Status,
			version,
			role,
			rack,
			logDirs,
		))
	}
	m.brokerRows = rows

	state := m.tabStates[BrokersTab]
	m.brokersTable.SetColumns(decorateColumns(brokersColumns, state))
	m.brokersTable.SetRows(state.applyView(rows))
}

func (m *Model) rebuildConsumerRows() {
	rows := make([]table.Row, 0, len(m.consumerGroups))
	for _, group := range m.consumerGroups {
		rows = append(rows, safeRow(
			group.GroupID,
			fmt.Sprintf("%d", group.NumMembers),
			fmt.Sprintf("%d", group.NumTopics),
			fmt.Sprintf("%d", group.ConsumerLag),
			group.Coordinator,
			group.State,
		))
	}
	m.consumerRows = rows

	state := m.tabStates[ConsumerGroupsTab]
	m.consumersTable.SetColumns(decorateColumns(consumersColumns, state))
	m.consumersTable.SetRows(state.applyView(rows))
}

func (m *Model) rebuildACLRows() {
	if m.aclTable == nil {
		return
	}

	rows := make([]table.Row, 0, len(m.acls))
	for _, acl := range m.acls {
		rows = append(rows, safeRow(
			acl.Principal,
			acl.ResourceType,
			acl.ResourceName,
			acl.PatternType,
			acl.Operation,
			acl.PermissionType,
			acl.Host,
		))
	}
	m.aclRows = rows

	state := m.tabStates[ACLsTab]
	m.aclTable.SetColumns(decorateColumns(aclColumns, state))
	m.aclTable.SetRows(state.applyView(rows))
}

// rebuildActiveTab re-applies the filter and sort of whichever tab is showing.
// Used after a keystroke that changed only the view, not the data.
func (m *Model) rebuildActiveTab() {
	switch m.activeTab {
	case BrokersTab:
		m.rebuildBrokerRows()
	case TopicsTab:
		m.rebuildTopicRows()
	case ConsumerGroupsTab:
		m.rebuildConsumerRows()
	case ACLsTab:
		m.rebuildACLRows()
	}
}

// activeTable returns the table the current tab draws, and whether there is one.
// The ACL table is built on first use, so it can legitimately be absent.
func (m *Model) activeTable() (*table.Model, bool) {
	switch m.activeTab {
	case BrokersTab:
		return &m.brokersTable, true
	case TopicsTab:
		if m.focusedPanel == 1 {
			return &m.configTable, true
		}
		return &m.topicsTable, true
	case ConsumerGroupsTab:
		return &m.consumersTable, true
	case ACLsTab:
		if m.aclTable == nil {
			return nil, false
		}
		return m.aclTable, true
	}
	return nil, false
}

// visibleRowCount is how many rows survive the current tab's filter.
//
// It reads the tab's own table rather than activeTable: in the Topics tab the
// focus may be on the configuration panel, and the filter still applies to the
// topic list behind it.
func (m *Model) visibleRowCount() int {
	switch m.activeTab {
	case BrokersTab:
		return len(m.brokersTable.Rows())
	case TopicsTab:
		return len(m.topicsTable.Rows())
	case ConsumerGroupsTab:
		return len(m.consumersTable.Rows())
	case ACLsTab:
		if m.aclTable != nil {
			return len(m.aclTable.Rows())
		}
	}
	return 0
}

// totalRowCount is how many rows the current tab has before filtering.
func (m *Model) totalRowCount() int {
	switch m.activeTab {
	case BrokersTab:
		return len(m.brokerRows)
	case TopicsTab:
		return len(m.topicRows)
	case ConsumerGroupsTab:
		return len(m.consumerRows)
	case ACLsTab:
		return len(m.aclRows)
	}
	return 0
}

// blurActiveTab releases focus from every table the current tab owns.
//
// Key dispatch routes on activeTab rather than on each table's Focused() flag,
// so a stale focus is harmless today. It is still cleared: focus state that
// disagrees with the active tab is the kind of thing that becomes a real bug
// the first time something renders from it.
func (m *Model) blurActiveTab() {
	switch m.activeTab {
	case BrokersTab:
		m.brokersTable.Blur()
	case TopicsTab:
		m.topicsTable.Blur()
		m.configTable.Blur()
	case ConsumerGroupsTab:
		m.consumersTable.Blur()
	case ACLsTab:
		if m.aclTable != nil {
			m.aclTable.Blur()
		}
	}
}
