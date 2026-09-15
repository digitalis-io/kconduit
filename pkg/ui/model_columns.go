package ui

import "github.com/charmbracelet/bubbles/table"

// Base column definitions for the list-view tables.
//
// They are package-level rather than built inside NewModel because sorting
// re-titles the active column on every redraw: decorateColumns copies from
// these, so the pristine titles are always available and a marker can never be
// appended twice.
var (
	topicsColumns = []table.Column{
		{Title: "Topic Name", Width: 30},
		{Title: "Parts", Width: 6},
		{Title: "RF", Width: 4},
		{Title: "Messages", Width: 12},
		{Title: "Disk", Width: 10},
	}

	brokersColumns = []table.Column{
		{Title: "ID", Width: 4},
		{Title: "Host", Width: 20},
		{Title: "Port", Width: 6},
		{Title: "Status", Width: 8},
		{Title: "Version", Width: 8},
		{Title: "Roles", Width: 20},
		{Title: "Rack", Width: 10},
		{Title: "Log Dirs", Width: 10},
	}

	consumersColumns = []table.Column{
		{Title: "Group ID", Width: 25},
		{Title: "Members", Width: 9},
		{Title: "Topics", Width: 8},
		{Title: "Lag", Width: 12},
		{Title: "Coordinator", Width: 13},
		{Title: "State", Width: 12},
	}

	aclColumns = []table.Column{
		{Title: "Principal", Width: 20},
		{Title: "Resource Type", Width: 15},
		{Title: "Resource", Width: 25},
		{Title: "Pattern", Width: 10},
		{Title: "Operation", Width: 15},
		{Title: "Permission", Width: 12},
		{Title: "Host", Width: 15},
	}
)

// columnsFor returns the base columns of a tab's table.
func columnsFor(tab TabView) []table.Column {
	switch tab {
	case BrokersTab:
		return brokersColumns
	case TopicsTab:
		return topicsColumns
	case ConsumerGroupsTab:
		return consumersColumns
	case ACLsTab:
		return aclColumns
	}
	return nil
}
