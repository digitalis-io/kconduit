package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/digitalis-io/kconduit/pkg/kafka"
)

// GroupLagModel breaks a consumer group's lag down by partition.
//
// The Groups tab shows one total per group, which answers "is anything behind?"
// but not "behind on what?". This answers the second question: the partition,
// where the group has committed to, where the log ends, and which member owns
// it.
type GroupLagModel struct {
	client  *kafka.Client
	groupID string
	rows    []kafka.PartitionLag
	table   table.Model
	state   tableState
	loading bool
	err     error
	width   int
	height  int
}

var groupLagColumns = []table.Column{
	{Title: "Topic", Width: 34},
	{Title: "Partition", Width: 10},
	{Title: "Current", Width: 14},
	{Title: "Log End", Width: 14},
	{Title: "Lag", Width: 12},
	{Title: "Member", Width: 24},
}

type groupLagMsg struct {
	rows []kafka.PartitionLag
	err  error
}

func fetchGroupLag(client *kafka.Client, groupID string) tea.Cmd {
	return func() tea.Msg {
		rows, err := client.GetConsumerGroupLag(groupID)
		return groupLagMsg{rows: rows, err: err}
	}
}

func NewGroupLagModel(client *kafka.Client, groupID string) GroupLagModel {
	t := table.New(
		table.WithColumns(groupLagColumns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = tableHeaderStyle()
	s.Selected = tableSelectedStyle()
	t.SetStyles(s)

	// Lag is the reason to open this view, so it is what the rows are ordered
	// by: worst first.
	st := newTableState("filter topics…")
	st.sortCol = 4
	st.sortDesc = true

	return GroupLagModel{
		client:  client,
		groupID: groupID,
		table:   t,
		state:   st,
		loading: true,
	}
}

func (m GroupLagModel) Init() tea.Cmd {
	return fetchGroupLag(m.client, m.groupID)
}

func (m GroupLagModel) Update(msg tea.Msg) (GroupLagModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case groupLagMsg:
		m.loading = false
		m.err = msg.err
		m.rows = msg.rows
		m.rebuildRows()
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetWidth(msg.Width - 4)
		m.table.SetHeight(max(msg.Height-12, 3))
		return m, nil

	case tea.KeyMsg:
		if m.state.filtering {
			switch msg.String() {
			case "esc":
				m.state = m.state.stopFilter(true)
				m.rebuildRows()
				return m, nil
			case "enter":
				m.state = m.state.stopFilter(false)
				return m, nil
			}
			m.state.filter, cmd = m.state.filter.Update(msg)
			m.rebuildRows()
			return m, cmd
		}

		switch msg.String() {
		case "esc", "q":
			return m, func() tea.Msg { return SwitchToListViewMsg{} }
		case "r":
			m.loading = true
			return m, fetchGroupLag(m.client, m.groupID)
		case "/":
			m.state, cmd = m.state.startFilter()
			return m, cmd
		case "<":
			m.state = m.state.moveSort(-1, len(groupLagColumns))
			m.rebuildRows()
			return m, nil
		case ">":
			m.state = m.state.moveSort(1, len(groupLagColumns))
			m.rebuildRows()
			return m, nil
		case "g":
			m.table.GotoTop()
			return m, nil
		case "G":
			m.table.GotoBottom()
			return m, nil
		case "y":
			return m, copyRowCmd(m.table.SelectedRow())
		}
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// rebuildRows re-renders the table from m.rows through the filter and sort.
// It takes a pointer receiver: on a value receiver the new rows would be set on
// a copy and silently lost.
func (m *GroupLagModel) rebuildRows() {
	rows := make([]table.Row, 0, len(m.rows))
	for _, r := range m.rows {
		current := fmt.Sprintf("%d", r.Current)
		if r.Current < 0 {
			current = "-"
		}
		member := r.Member
		if member == "" {
			member = "-"
		}
		rows = append(rows, safeRow(
			r.Topic,
			fmt.Sprintf("%d", r.Partition),
			current,
			fmt.Sprintf("%d", r.LogEnd),
			fmt.Sprintf("%d", r.Lag),
			member,
		))
	}

	m.table.SetColumns(decorateColumns(groupLagColumns, m.state))
	m.table.SetRows(m.state.applyView(rows))
}

func (m GroupLagModel) View() string {
	title := appHeaderStyle.Render("Consumer group lag") + "  " +
		lipgloss.NewStyle().Foreground(theme.Accent).Render(m.groupID)

	var body string
	switch {
	case m.loading:
		body = labelStyle.Render("  Measuring lag across partitions…")
	case m.err != nil:
		body = errorStyle.Render("  " + m.err.Error())
	case len(m.rows) == 0:
		body = labelStyle.Render("  This group has no committed offsets.")
	default:
		body = panelStyle.Width(max(m.width-4, 20)).Render(m.table.View()) + "\n" +
			m.summary()
	}

	help := renderHelpBar(
		"/", "filter",
		"< >", "sort",
		"g G", "top/bottom",
		"y", "copy row",
		"r", "refresh",
		"esc", "back",
	)
	if m.state.filtering {
		help = m.state.filter.View()
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, "", body, "", help)
}

// summary totals the lag across the partitions currently shown, so a filtered
// view answers "how far behind is this topic?" rather than only the whole group.
func (m GroupLagModel) summary() string {
	var total, partitions int64
	for _, row := range m.table.Rows() {
		var lag int64
		if _, err := fmt.Sscanf(row[4], "%d", &lag); err == nil {
			total += lag
		}
		partitions++
	}

	label := "  Total lag "
	if m.state.active() {
		label = "  Total lag (filtered) "
	}

	style := successStyle
	if total > 0 {
		style = warningStyle
	}

	return labelStyle.Render(label) +
		style.Render(fmt.Sprintf("%d", total)) +
		labelStyle.Render(fmt.Sprintf("  across %d partitions", partitions))
}
