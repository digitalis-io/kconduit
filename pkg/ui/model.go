package ui

import (
	"fmt"
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
	client           *kafka.Client
	topics           []kafka.TopicInfo
	brokers          []kafka.BrokerInfo
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
	selectedTopic    string
	activeTab        TabView
}

func NewModel(client *kafka.Client) Model {
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
		{Title: "Role", Width: 12},
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

	return Model{
		topicsTable:  topicsTable,
		brokersTable: brokersTable,
		client:       client,
		loading:      true,
		mode:         ListView,
		activeTab:    BrokersTab,
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

type topicConfigMsg struct {
	config *kafka.TopicConfig
	err    error
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
		case "tab", "right":
			// Move to next tab
			// First blur the current table
			switch m.activeTab {
			case BrokersTab:
				m.brokersTable.Blur()
				m.activeTab = TopicsTab
				m.topicsTable.Focus()
			case TopicsTab:
				m.topicsTable.Blur()
				m.activeTab = ConsumerGroupsTab
			case ConsumerGroupsTab:
				m.activeTab = ACLsTab
			case ACLsTab:
				m.activeTab = BrokersTab
				m.brokersTable.Focus()
			}
			// Trigger refresh when switching tabs
			return m, tea.Batch(fetchTopics(m.client), fetchBrokers(m.client))
		case "shift+tab", "left":
			// Move to previous tab
			// First blur the current table
			switch m.activeTab {
			case BrokersTab:
				m.brokersTable.Blur()
				m.activeTab = ACLsTab
			case TopicsTab:
				m.topicsTable.Blur()
				m.activeTab = BrokersTab
				m.brokersTable.Focus()
			case ConsumerGroupsTab:
				m.activeTab = TopicsTab
				m.topicsTable.Focus()
			case ACLsTab:
				m.activeTab = ConsumerGroupsTab
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
			return m, fetchTopics(m.client)
		case "3":
			// Switch to Consumer Groups tab
			if m.activeTab == BrokersTab {
				m.brokersTable.Blur()
			} else if m.activeTab == TopicsTab {
				m.topicsTable.Blur()
			}
			m.activeTab = ConsumerGroupsTab
			return m, nil // TODO: fetch consumer groups
		case "4":
			// Switch to ACLs tab
			if m.activeTab == BrokersTab {
				m.brokersTable.Blur()
			} else if m.activeTab == TopicsTab {
				m.topicsTable.Blur()
			}
			m.activeTab = ACLsTab
			return m, nil // TODO: fetch ACLs
		case "r":
			m.loading = true
			return m, tea.Batch(fetchTopics(m.client), fetchBrokers(m.client))
		case "c", "C":
			m.createTopicModel = NewCreateTopicModel(m.client)
			m.mode = CreateTopicView
			return m, m.createTopicModel.Init()
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
				role = "★ Controller"
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
	}

	// Update the active table based on current tab
	// This allows keyboard navigation to work properly
	switch m.activeTab {
	case BrokersTab:
		var cmd tea.Cmd
		m.brokersTable, cmd = m.brokersTable.Update(msg)
		cmds = append(cmds, cmd)
	case TopicsTab:
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
	case ConsumerGroupsTab, ACLsTab:
		// No table to update for these tabs yet
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

func (m Model) View() string {
	switch m.mode {
	case ProducerView:
		return m.producerModel.View()
	case ConsumerView:
		return m.consumerModel.View()
	case CreateTopicView:
		return m.createTopicModel.View()
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
		Width((m.width-10)/2).
		Height(m.height - 12)
	
	topicsView := leftPanel.Render(m.topicsTable.View())
	
	// Right panel: topic config
	rightPanel := borderStyle.
		Width((m.width-10)/2).
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

func (m Model) renderTopicConfig() string {
	if m.topicConfig == nil {
		return "No configuration available"
	}
	
	var sb strings.Builder
	
	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("229")).
		Underline(true)
	
	sb.WriteString(titleStyle.Render(fmt.Sprintf("Topic: %s", m.topicConfig.Name)))
	sb.WriteString("\n\n")
	
	// Basic info
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	sb.WriteString(infoStyle.Render(fmt.Sprintf("Partitions: %d\n", m.topicConfig.Partitions)))
	sb.WriteString(infoStyle.Render(fmt.Sprintf("Replication Factor: %d\n", m.topicConfig.ReplicationFactor)))
	sb.WriteString("\n")
	
	// Partition Details
	if len(m.topicConfig.PartitionDetails) > 0 {
		sectionStyle := titleStyle.Underline(false)
		sb.WriteString(sectionStyle.Render("Partition Details:"))
		sb.WriteString("\n")
		for _, p := range m.topicConfig.PartitionDetails {
			sb.WriteString(fmt.Sprintf("  P%d: Leader=%d, Replicas=%v, ISR=%v\n", 
				p.ID, p.Leader, p.Replicas, p.ISR))
		}
		sb.WriteString("\n")
	}
	
	// Configuration
	if len(m.topicConfig.Configs) > 0 {
		sectionStyle := titleStyle.Underline(false)
		sb.WriteString(sectionStyle.Render("Configuration:"))
		sb.WriteString("\n")
		
		// Show important configs first
		importantConfigs := []string{
			"retention.ms", "retention.bytes", "segment.ms", "segment.bytes",
			"cleanup.policy", "compression.type", "min.insync.replicas",
		}
		
		for _, key := range importantConfigs {
			if val, exists := m.topicConfig.Configs[key]; exists {
				sb.WriteString(fmt.Sprintf("  %s: %s\n", key, val))
			}
		}
		
		// Show remaining configs
		for key, val := range m.topicConfig.Configs {
			isImportant := false
			for _, imp := range importantConfigs {
				if key == imp {
					isImportant = true
					break
				}
			}
			if !isImportant && val != "" {
				sb.WriteString(fmt.Sprintf("  %s: %s\n", key, val))
			}
		}
	}
	
	return sb.String()
}

func (m Model) renderConsumerGroupsView() string {
	return "Consumer Groups view - Coming soon..."
}

func (m Model) renderACLsView() string {
	return "ACLs view - Coming soon..."
}

func (m Model) getHelpText() string {
	baseHelp := "Tab/→: Next | Shift+Tab/←: Previous | 1-4: Jump to tab | r: Refresh | q: Quit"
	
	switch m.activeTab {
	case TopicsTab:
		return baseHelp + " | Enter: Consume | P: Produce | C: Create Topic"
	default:
		return baseHelp
	}
}