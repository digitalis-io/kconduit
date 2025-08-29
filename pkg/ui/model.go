package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/axonops/kconduit/pkg/kafka"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

type ViewMode int

const (
	ListView ViewMode = iota
	ProducerView
	ConsumerView
	CreateTopicView
	EditConfigView
	AIAssistantView
	DeleteTopicView
	CreateACLView
)

type TabView int

const (
	BrokersTab TabView = iota
	TopicsTab
	ConsumerGroupsTab
	ACLsTab
)

type Model struct {
	topicsTable      table.Model
	brokersTable     table.Model
	configTable      table.Model
	consumersTable   table.Model
	aclTable         *table.Model
	client           *kafka.Client
	topics           []kafka.TopicInfo
	brokers          []kafka.BrokerInfo
	consumerGroups   []kafka.ConsumerGroupInfo
	acls             []kafka.ACL
	topicConfig      *kafka.TopicConfig
	err              error
	loading          bool
	loadingConfig    bool
	width            int
	height           int
	mode             ViewMode
	producerModel    ProducerModel
	consumerModel    ConsumerModel
	createTopicModel CreateTopicModel
	createACLModel   CreateACLModel
	editConfigModel  *EditConfigModel
	aiAssistantModel AIAssistantModel
	deleteTopicModel DeleteTopicModel
	selectedTopic    string
	activeTab        TabView
	focusedPanel     int // 0: topics list, 1: config table (when in Topics tab)
	aiEngine         string
	aiModel          string
}

func NewModel(client *kafka.Client, aiEngine string, aiModel string) Model {
	// Topics table
	topicsColumns := []table.Column{
		{Title: "Topic Name", Width: 30},
		{Title: "Parts", Width: 8},
		{Title: "RF", Width: 4},
	}

	topicsTable := table.New(
		table.WithColumns(topicsColumns),
		table.WithFocused(false),
		table.WithHeight(10),
	)

	// Brokers table with more detailed columns
	brokersColumns := []table.Column{
		{Title: "ID", Width: 4},
		{Title: "Host", Width: 20},
		{Title: "Port", Width: 6},
		{Title: "Status", Width: 8},
		{Title: "Version", Width: 8},
		{Title: "Roles", Width: 20},
		{Title: "Rack", Width: 10},
		{Title: "Log Dirs", Width: 10},
	}

	brokersTable := table.New(
		table.WithColumns(brokersColumns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	// Set styles for both tables
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)

	topicsTable.SetStyles(s)
	brokersTable.SetStyles(s)

	// Config table for topic configuration display
	configColumns := []table.Column{
		{Title: "Configuration", Width: 40},
		{Title: "Value", Width: 45},
	}

	configTable := table.New(
		table.WithColumns(configColumns),
		table.WithFocused(false),
		table.WithHeight(30), // Will be dynamically adjusted
	)

	// Custom styles for config table with colors
	configStyles := table.DefaultStyles()
	configStyles.Header = configStyles.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true).
		Foreground(lipgloss.Color("213")) // Purple for headers
	configStyles.Cell = lipgloss.NewStyle().
		Foreground(lipgloss.Color("87")) // Cyan for keys
	configStyles.Selected = configStyles.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)

	configTable.SetStyles(configStyles)

	// Consumers table for consumer groups
	consumersColumns := []table.Column{
		{Title: "Group ID", Width: 25},
		{Title: "Members", Width: 8},
		{Title: "Topics", Width: 7},
		{Title: "Lag", Width: 10},
		{Title: "Coordinator", Width: 12},
		{Title: "State", Width: 10},
	}

	consumersTable := table.New(
		table.WithColumns(consumersColumns),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	consumersTable.SetStyles(s)

	return Model{
		topicsTable:    topicsTable,
		brokersTable:   brokersTable,
		configTable:    configTable,
		consumersTable: consumersTable,
		client:         client,
		loading:        true,
		mode:           ListView,
		activeTab:      BrokersTab,
		aiEngine:       aiEngine,
		aiModel:        aiModel,
	}
}

type tickMsg struct{}

type topicsMsg struct {
	topics []kafka.TopicInfo
	err    error
}

type brokersMsg struct {
	brokers []kafka.BrokerInfo
	err     error
}

type consumerGroupsMsg struct {
	groups []kafka.ConsumerGroupInfo
	err    error
}

type topicConfigMsg struct {
	config *kafka.TopicConfig
	err    error
}

type aclsMsg struct {
	acls []kafka.ACL
	err  error
}

type ViewChangedMsg struct {
	View TabView
}

func fetchTopics(client *kafka.Client) tea.Cmd {
	return func() tea.Msg {
		topics, err := client.GetTopicDetails()
		return topicsMsg{topics: topics, err: err}
	}
}

func fetchBrokers(client *kafka.Client) tea.Cmd {
	return func() tea.Msg {
		brokers, err := client.GetBrokers()
		return brokersMsg{brokers: brokers, err: err}
	}
}

func fetchConsumerGroups(client *kafka.Client) tea.Cmd {
	return func() tea.Msg {
		groups, err := client.GetConsumerGroups()
		return consumerGroupsMsg{groups: groups, err: err}
	}
}

func fetchACLs(client *kafka.Client) tea.Cmd {
	return func() tea.Msg {
		acls, err := client.ListACLs()
		return aclsMsg{acls: acls, err: err}
	}
}

func fetchTopicConfig(client *kafka.Client, topicName string) tea.Cmd {
	return func() tea.Msg {
		config, err := client.GetTopicConfig(topicName)
		return topicConfigMsg{config: config, err: err}
	}
}

func (m Model) Init() tea.Cmd {
	// Add a small delay to allow connection to establish
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case ProducerView:
		return m.updateProducerView(msg)
	case ConsumerView:
		return m.updateConsumerView(msg)
	case CreateTopicView:
		return m.updateCreateTopicView(msg)
	case EditConfigView:
		return m.updateEditConfigView(msg)
	case AIAssistantView:
		return m.updateAIAssistantView(msg)
	case DeleteTopicView:
		return m.updateDeleteTopicView(msg)
	case CreateACLView:
		return m.updateCreateACLView(msg)
	default:
		return m.updateListView(msg)
	}
}

func (m Model) updateListView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tickMsg:
		// Initial load after connection established
		return m, tea.Batch(fetchTopics(m.client), fetchBrokers(m.client))

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			// In Topics tab, switch between topics list and config table
			if m.activeTab == TopicsTab && m.topicConfig != nil {
				if m.focusedPanel == 0 {
					// Switch from topics list to config table
					m.topicsTable.Blur()
					m.configTable.Focus()
					m.focusedPanel = 1
				} else {
					// Switch from config table to topics list
					m.configTable.Blur()
					m.topicsTable.Focus()
					m.focusedPanel = 0
				}
				return m, nil
			}
			// Otherwise move to next tab
			switch m.activeTab {
			case BrokersTab:
				m.brokersTable.Blur()
				m.activeTab = TopicsTab
				m.topicsTable.Focus()
				m.focusedPanel = 0
			case TopicsTab:
				m.topicsTable.Blur()
				m.configTable.Blur()
				m.activeTab = ConsumerGroupsTab
				m.consumersTable.Focus()
				return m, fetchConsumerGroups(m.client)
			case ConsumerGroupsTab:
				m.consumersTable.Blur()
				m.activeTab = ACLsTab
				return m, fetchACLs(m.client)
			case ACLsTab:
				m.activeTab = BrokersTab
				m.brokersTable.Focus()
				return m, fetchBrokers(m.client)
			}
			// Trigger refresh when switching tabs
			return m, tea.Batch(fetchTopics(m.client), fetchBrokers(m.client))
		case "shift+tab":
			// In Topics tab, switch between config table and topics list (reverse)
			if m.activeTab == TopicsTab && m.topicConfig != nil {
				if m.focusedPanel == 1 {
					// Switch from config table to topics list
					m.configTable.Blur()
					m.topicsTable.Focus()
					m.focusedPanel = 0
				} else {
					// Switch from topics list to config table
					m.topicsTable.Blur()
					m.configTable.Focus()
					m.focusedPanel = 1
				}
				return m, nil
			}
			// Otherwise move to previous tab
			switch m.activeTab {
			case BrokersTab:
				m.brokersTable.Blur()
				m.activeTab = ACLsTab
			case TopicsTab:
				m.topicsTable.Blur()
				m.configTable.Blur()
				m.activeTab = BrokersTab
				m.brokersTable.Focus()
				m.focusedPanel = 0
			case ConsumerGroupsTab:
				m.consumersTable.Blur()
				m.activeTab = TopicsTab
				m.topicsTable.Focus()
				m.focusedPanel = 0
				return m, fetchTopics(m.client)
			case ACLsTab:
				m.activeTab = ConsumerGroupsTab
				m.consumersTable.Focus()
				return m, fetchConsumerGroups(m.client)
			}
			// Trigger refresh when switching tabs
			return m, tea.Batch(fetchTopics(m.client), fetchBrokers(m.client))
		case "1":
			// Switch to Brokers tab
			if m.activeTab == TopicsTab {
				m.topicsTable.Blur()
			}
			m.activeTab = BrokersTab
			m.brokersTable.Focus()
			return m, fetchBrokers(m.client)
		case "2":
			// Switch to Topics tab
			if m.activeTab == BrokersTab {
				m.brokersTable.Blur()
			}
			m.activeTab = TopicsTab
			m.topicsTable.Focus()
			m.configTable.Blur()
			m.focusedPanel = 0
			return m, fetchTopics(m.client)
		case "3":
			// Switch to Consumer Groups tab
			if m.activeTab == BrokersTab {
				m.brokersTable.Blur()
			} else if m.activeTab == TopicsTab {
				m.topicsTable.Blur()
				m.configTable.Blur()
			}
			m.activeTab = ConsumerGroupsTab
			m.consumersTable.Focus()
			return m, fetchConsumerGroups(m.client)
		case "4":
			// Switch to ACLs tab
			if m.activeTab == BrokersTab {
				m.brokersTable.Blur()
			} else if m.activeTab == TopicsTab {
				m.topicsTable.Blur()
			}
			m.activeTab = ACLsTab
			return m, fetchACLs(m.client)
		case "r", "R":
			m.loading = true
			return m, tea.Batch(fetchTopics(m.client), fetchBrokers(m.client))
		case "C":
			if m.activeTab == ACLsTab {
				// Create ACL
				m.createACLModel = NewCreateACLModel(m.client)
				m.mode = CreateACLView
				return m, m.createACLModel.Init()
			} else {
				// Create Topic
				m.createTopicModel = NewCreateTopicModel(m.client)
				m.mode = CreateTopicView
				return m, m.createTopicModel.Init()
			}
		case "A", "a":
			// Open AI Assistant
			m.aiAssistantModel = NewAIAssistantModel(m.client, m.aiEngine, m.aiModel)
			m.mode = AIAssistantView
			return m, m.aiAssistantModel.Init()
		case "D", "d":
			// Delete topic - only available in Topics tab
			if m.activeTab == TopicsTab && len(m.topics) > 0 && !m.loading && m.err == nil {
				selectedRow := m.topicsTable.SelectedRow()
				if len(selectedRow) > 0 {
					m.selectedTopic = selectedRow[0]
					m.deleteTopicModel = NewDeleteTopicModel(m.client, m.selectedTopic)
					m.mode = DeleteTopicView
					return m, m.deleteTopicModel.Init()
				}
			}
		case "p", "P":
			if m.activeTab == TopicsTab && len(m.topics) > 0 && !m.loading && m.err == nil {
				selectedRow := m.topicsTable.SelectedRow()
				if len(selectedRow) > 0 {
					m.selectedTopic = selectedRow[0]
					m.producerModel = NewProducerModel(m.selectedTopic, m.client)
					m.mode = ProducerView
					return m, m.producerModel.Init()
				}
			}
		case "e", "E":
			// Edit config value
			if m.activeTab == TopicsTab && m.focusedPanel == 1 && m.topicConfig != nil {
				// Get the selected config row
				selectedRow := m.configTable.SelectedRow()
				if len(selectedRow) == 2 && selectedRow[0] != "" && selectedRow[0] != "No configuration available" {
					// The key is now directly in the first column (no leading spaces)
					configKey := selectedRow[0]

					// Get the actual raw value from the config map
					if rawValue, exists := m.topicConfig.Configs[configKey]; exists {
						m.editConfigModel = NewEditConfigModel(m.client, m.selectedTopic, configKey, rawValue)
						m.mode = EditConfigView
						return m, m.editConfigModel.Init()
					}
				}
			}
		case "enter":
			if m.activeTab == TopicsTab && len(m.topics) > 0 && !m.loading && m.err == nil {
				selectedRow := m.topicsTable.SelectedRow()
				if len(selectedRow) > 0 {
					m.selectedTopic = selectedRow[0]
					m.consumerModel = NewConsumerModel(m.selectedTopic, m.client)
					m.mode = ConsumerView
					return m, m.consumerModel.Init()
				}
			}
		}

	case topicsMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.topics = msg.topics
		m.err = nil

		rows := make([]table.Row, len(m.topics))
		for i, topic := range m.topics {
			rows[i] = table.Row{
				topic.Name,
				fmt.Sprintf("%d", topic.Partitions),
				fmt.Sprintf("%d", topic.ReplicationFactor),
			}
		}
		m.topicsTable.SetRows(rows)

		// If we have topics and we're on the topics tab, select the first one
		if len(m.topics) > 0 && m.activeTab == TopicsTab {
			// Make sure topics table is focused
			if !m.topicsTable.Focused() {
				m.topicsTable.Focus()
			}
			selectedRow := m.topicsTable.SelectedRow()
			if len(selectedRow) > 0 {
				topicName := selectedRow[0]
				m.selectedTopic = topicName
				return m, fetchTopicConfig(m.client, topicName)
			}
		}

	case topicConfigMsg:
		m.loadingConfig = false
		if msg.err == nil {
			m.topicConfig = msg.config
			// Update config table with the configuration
			m.updateConfigTable()
		}

	case brokersMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.brokers = msg.brokers
		m.err = nil

		rows := make([]table.Row, len(m.brokers))
		for i, broker := range m.brokers {
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

			rows[i] = table.Row{
				fmt.Sprintf("%d", broker.ID),
				broker.Host,
				fmt.Sprintf("%d", broker.Port),
				broker.Status,
				version,
				role,
				rack,
				logDirs,
			}
		}
		m.brokersTable.SetRows(rows)

	case consumerGroupsMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.consumerGroups = msg.groups
		m.err = nil

		rows := make([]table.Row, len(m.consumerGroups))
		for i, group := range m.consumerGroups {
			lag := fmt.Sprintf("%d", group.ConsumerLag)
			if group.ConsumerLag == 0 {
				lag = "0"
			}

			rows[i] = table.Row{
				group.GroupID,
				fmt.Sprintf("%d", group.NumMembers),
				fmt.Sprintf("%d", group.NumTopics),
				lag,
				group.Coordinator,
				group.State,
			}
		}
		m.consumersTable.SetRows(rows)
		
	case aclsMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.acls = msg.acls
		m.err = nil
		
		// Create ACL table if not already created
		if m.aclTable == nil {
			aclColumns := []table.Column{
				{Title: "Principal", Width: 20},
				{Title: "Resource Type", Width: 15},
				{Title: "Resource", Width: 25},
				{Title: "Pattern", Width: 10},
				{Title: "Operation", Width: 15},
				{Title: "Permission", Width: 10},
				{Title: "Host", Width: 15},
			}
			t := table.New(
				table.WithColumns(aclColumns),
				table.WithFocused(true),
				table.WithHeight(10),
			)
			
			s := table.DefaultStyles()
			s.Header = s.Header.
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("240")).
				BorderBottom(true).
				Bold(false)
			s.Selected = s.Selected.
				Foreground(lipgloss.Color("229")).
				Background(lipgloss.Color("57")).
				Bold(false)
			t.SetStyles(s)
			m.aclTable = &t
		}
		
		rows := make([]table.Row, len(m.acls))
		for i, acl := range m.acls {
			rows[i] = table.Row{
				acl.Principal,
				acl.ResourceType,
				acl.ResourceName,
				acl.PatternType,
				acl.Operation,
				acl.PermissionType,
				acl.Host,
			}
		}
		m.aclTable.SetRows(rows)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Full width for single table view with tabs
		tableHeight := msg.Height - 10 // Account for header and footer

		// Brokers get full width
		m.brokersTable.SetHeight(tableHeight)
		m.brokersTable.SetWidth(msg.Width - 4)

		// Topics table gets half width (for split view)
		m.topicsTable.SetHeight(tableHeight)
		m.topicsTable.SetWidth((msg.Width - 10) / 2)

		// Consumers table gets full width
		m.consumersTable.SetHeight(tableHeight)
		m.consumersTable.SetWidth(msg.Width - 4)
	}

	// Update the active table based on current tab
	// This needs to happen for all messages, not just after key handling
	switch m.activeTab {
	case BrokersTab:
		var cmd tea.Cmd
		m.brokersTable, cmd = m.brokersTable.Update(msg)
		cmds = append(cmds, cmd)
	case TopicsTab:
		// Always update the topics table to handle initial selection
		if m.focusedPanel == 0 {
			// Topics list is focused - it processes all events
			var cmd tea.Cmd
			oldRow := m.topicsTable.SelectedRow()
			m.topicsTable, cmd = m.topicsTable.Update(msg)
			newRow := m.topicsTable.SelectedRow()

			// Check if selection changed
			if len(oldRow) > 0 && len(newRow) > 0 && oldRow[0] != newRow[0] {
				m.selectedTopic = newRow[0]
				m.loadingConfig = true
				cmds = append(cmds, cmd, fetchTopicConfig(m.client, newRow[0]))
			} else {
				cmds = append(cmds, cmd)
			}
		} else {
			// Config table is focused - it processes all events
			var cmd tea.Cmd
			m.configTable, cmd = m.configTable.Update(msg)
			cmds = append(cmds, cmd)
		}
	case ConsumerGroupsTab:
		var cmd tea.Cmd
		m.consumersTable, cmd = m.consumersTable.Update(msg)
		cmds = append(cmds, cmd)
	case ACLsTab:
		if m.aclTable != nil {
			var cmd tea.Cmd
			*m.aclTable, cmd = m.aclTable.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) updateProducerView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case SwitchToListViewMsg:
		m.mode = ListView
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	m.producerModel, cmd = m.producerModel.Update(msg)
	return m, cmd
}

func (m Model) updateConsumerView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case SwitchToListViewMsg:
		m.mode = ListView
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	m.consumerModel, cmd = m.consumerModel.Update(msg)
	return m, cmd
}

func (m Model) updateCreateTopicView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case SwitchToListViewMsg:
		m.mode = ListView
		m.loading = true
		return m, fetchTopics(m.client)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	m.createTopicModel, cmd = m.createTopicModel.Update(msg)
	return m, cmd
}

func (m Model) updateCreateACLView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case ViewChangedMsg:
		if msg.View == ACLsTab {
			m.mode = ListView
			m.activeTab = ACLsTab
			m.loading = true
			return m, fetchACLs(m.client)
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	updatedModel, cmd := m.createACLModel.Update(msg)
	m.createACLModel = updatedModel.(CreateACLModel)
	return m, cmd
}

func (m Model) updateEditConfigView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case SwitchToListViewMsg:
		m.mode = ListView
		// Refresh the topic config to show any changes
		return m, fetchTopicConfig(m.client, m.selectedTopic)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	updatedModel, cmd := m.editConfigModel.Update(msg)
	if editModel, ok := updatedModel.(*EditConfigModel); ok {
		m.editConfigModel = editModel
	}
	return m, cmd
}

func (m Model) updateAIAssistantView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case SwitchToListViewMsg:
		m.mode = ListView
		m.loading = true
		return m, fetchTopics(m.client)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	updatedModel, cmd := m.aiAssistantModel.Update(msg)
	if aiModel, ok := updatedModel.(AIAssistantModel); ok {
		m.aiAssistantModel = aiModel
	}
	return m, cmd
}

func (m Model) updateDeleteTopicView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case SwitchToListViewMsg:
		m.mode = ListView
		m.loading = true
		return m, fetchTopics(m.client)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	updatedModel, cmd := m.deleteTopicModel.Update(msg)
	m.deleteTopicModel = updatedModel

	return m, cmd
}

func (m Model) View() string {
	switch m.mode {
	case ProducerView:
		return m.producerModel.View()
	case ConsumerView:
		return m.consumerModel.View()
	case CreateTopicView:
		return m.createTopicModel.View()
	case CreateACLView:
		return m.createACLModel.View()
	case EditConfigView:
		return m.editConfigModel.View()
	case AIAssistantView:
		return m.aiAssistantModel.View()
	case DeleteTopicView:
		return m.deleteTopicModel.View()
	default:
		return m.listView()
	}
}

func (m Model) listView() string {
	var sb strings.Builder

	// Render tab bar
	tabBar := m.renderTabBar()
	sb.WriteString(tabBar)
	sb.WriteString("\n\n")

	if m.loading {
		sb.WriteString("Loading...")
		return sb.String()
	}

	if m.err != nil {
		sb.WriteString(fmt.Sprintf("Error: %v\n", m.err))
		sb.WriteString("\nPress 'r' to retry or 'q' to quit")
		return sb.String()
	}

	// Render content based on active tab
	var content string
	switch m.activeTab {
	case BrokersTab:
		content = m.renderBrokersView()
	case TopicsTab:
		content = m.renderTopicsView()
	case ConsumerGroupsTab:
		content = m.renderConsumerGroupsView()
	case ACLsTab:
		content = m.renderACLsView()
	}

	sb.WriteString(content)
	sb.WriteString("\n\n")

	// Footer with context-sensitive help
	help := m.getHelpText()
	sb.WriteString(help)

	return sb.String()
}

func (m Model) renderTabBar() string {
	tabs := []string{"Brokers", "Topics", "Consumer Groups", "ACLs"}

	activeTabStyle := lipgloss.NewStyle().
		Bold(true).
		Background(lipgloss.Color("57")).
		Foreground(lipgloss.Color("229")).
		Padding(0, 2)

	inactiveTabStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Padding(0, 2)

	var renderedTabs []string
	for i, tab := range tabs {
		prefix := fmt.Sprintf("[%d] ", i+1)
		if TabView(i) == m.activeTab {
			renderedTabs = append(renderedTabs, activeTabStyle.Render(prefix+tab))
		} else {
			renderedTabs = append(renderedTabs, inactiveTabStyle.Render(prefix+tab))
		}
	}

	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)

	// Add title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("229"))

	title := titleStyle.Render("🚀 KConduit - Kafka Management")

	return lipgloss.JoinVertical(lipgloss.Left, title, tabBar)
}

func (m Model) renderBrokersView() string {
	if len(m.brokers) == 0 {
		return "No brokers found."
	}

	borderStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240"))

	return borderStyle.Render(m.brokersTable.View())
}

func (m Model) renderTopicsView() string {
	if len(m.topics) == 0 {
		return "No topics found."
	}

	borderStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240"))

	// Left panel: topics list
	leftPanel := borderStyle.
		Width((m.width - 10) / 2).
		Height(m.height - 12)

	topicsView := leftPanel.Render(m.topicsTable.View())

	// Right panel: topic config
	rightPanel := borderStyle.
		Width((m.width - 10) / 2).
		Height(m.height - 12).
		Padding(1)

	var configView string
	if m.loadingConfig {
		configView = rightPanel.Render("Loading configuration...")
	} else if m.topicConfig != nil {
		configView = rightPanel.Render(m.renderTopicConfig())
	} else {
		configView = rightPanel.Render("Select a topic to view its configuration")
	}

	// Join panels horizontally
	return lipgloss.JoinHorizontal(lipgloss.Top, topicsView, " ", configView)
}

// updateConfigTable populates the config table with topic configuration
func (m *Model) updateConfigTable() {
	if m.topicConfig == nil || m.topicConfig.Configs == nil {
		m.configTable.SetRows([]table.Row{})
		return
	}

	var rows []table.Row

	// Get sorted keys from all configs
	keys := make([]string, 0, len(m.topicConfig.Configs))
	for k := range m.topicConfig.Configs {
		// Skip internal/system configs
		if strings.HasPrefix(k, "confluent.") || strings.HasPrefix(k, "leader.") || strings.HasPrefix(k, "follower.") {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Add all configs sorted alphabetically
	for _, key := range keys {
		val := m.topicConfig.Configs[key]
		formattedVal := m.formatConfigValue(key, val)
		rows = append(rows, table.Row{key, formattedVal})
	}

	// If no configs, show message
	if len(rows) == 0 {
		rows = append(rows, table.Row{"No configuration available", ""})
	}

	m.configTable.SetRows(rows)

	// Use available height for better visibility
	// Account for header (title + tabs), footer, borders, and config header
	availableHeight := m.height - 18 // More conservative to ensure everything fits
	if availableHeight < 10 {
		availableHeight = 10
	}
	if availableHeight > 35 { // Cap max height to prevent overflow
		availableHeight = 35
	}
	m.configTable.SetHeight(availableHeight)

	// Ensure the table has a valid cursor position
	if len(rows) > 0 && m.configTable.Cursor() >= len(rows) {
		m.configTable.SetCursor(0)
	}
}

func (m Model) renderTopicConfig() string {
	if m.topicConfig == nil {
		return "No configuration available"
	}

	var sb strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("229"))

	sb.WriteString(titleStyle.Render(fmt.Sprintf("📁 %s", m.topicConfig.Name)))
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", 45))
	sb.WriteString("\n\n")

	// Basic info in a compact format
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	sb.WriteString(infoStyle.Render(fmt.Sprintf("Partitions: %d | Replication: %d",
		m.topicConfig.Partitions, m.topicConfig.ReplicationFactor)))
	sb.WriteString("\n\n")

	// Render the Bubble Tea table
	sb.WriteString(m.configTable.View())

	return sb.String()
}

// formatConfigValue formats config values to be human-readable
func (m Model) formatConfigValue(key, value string) string {
	// Convert milliseconds to human readable
	if strings.HasSuffix(key, ".ms") {
		if ms, err := strconv.ParseInt(value, 10, 64); err == nil {
			if ms == -1 {
				return "unlimited"
			}
			hours := ms / 3600000
			if hours >= 24 {
				days := hours / 24
				return fmt.Sprintf("%dd", days)
			}
			if hours > 0 {
				return fmt.Sprintf("%dh", hours)
			}
			minutes := ms / 60000
			if minutes > 0 {
				return fmt.Sprintf("%dm", minutes)
			}
			return fmt.Sprintf("%dms", ms)
		}
	}

	// Convert bytes to human readable
	if strings.HasSuffix(key, ".bytes") {
		if bytes, err := strconv.ParseInt(value, 10, 64); err == nil {
			if bytes == -1 {
				return "unlimited"
			}
			if bytes >= 1073741824 {
				return fmt.Sprintf("%.1fGB", float64(bytes)/1073741824)
			}
			if bytes >= 1048576 {
				return fmt.Sprintf("%.1fMB", float64(bytes)/1048576)
			}
			if bytes >= 1024 {
				return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
			}
			return fmt.Sprintf("%dB", bytes)
		}
	}

	// Truncate long values
	if len(value) > 12 {
		return value[:10] + ".."
	}

	return value
}

func (m Model) renderConsumerGroupsView() string {
	if m.loading {
		return "\n  Loading consumer groups..."
	}

	if m.err != nil {
		return fmt.Sprintf("\n  Error: %v", m.err)
	}

	if len(m.consumerGroups) == 0 {
		return "\n  No consumer groups found"
	}

	return lipgloss.JoinVertical(
		lipgloss.Top,
		m.consumersTable.View(),
	)
}

func (m Model) renderACLsView() string {
	var sb strings.Builder
	
	// Title with icon
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205"))
	
	sb.WriteString(titleStyle.Render("🔐 Access Control Lists (ACLs)"))
	sb.WriteString("\n\n")
	
	// Render ACL table
	if m.aclTable != nil {
		sb.WriteString(m.aclTable.View())
	} else {
		sb.WriteString("Loading ACLs...")
	}
	
	// Error display
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			MarginTop(1)
		sb.WriteString("\n\n" + errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}
	
	return sb.String()
}

func (m Model) getHelpText() string {
	baseHelp := "→/←: Switch tabs | 1-4: Jump to tab | r: Refresh | A: AI Assistant | q: Quit"

	switch m.activeTab {
	case TopicsTab:
		if m.topicConfig != nil {
			if m.focusedPanel == 1 {
				return baseHelp + " | Tab: Switch panel | e: Edit Config | Enter: Consume | P: Produce | D: Delete Topic"
			}
			return baseHelp + " | Tab: Switch panel | Enter: Consume | P: Produce | C: Create Topic | D: Delete Topic"
		}
		return baseHelp + " | Enter: Consume | P: Produce | C: Create Topic | D: Delete Topic"
	case ACLsTab:
		return baseHelp + " | C: Create ACL"
	default:
		return baseHelp
	}
}
