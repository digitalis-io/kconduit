package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/digitalis-io/kconduit/pkg/kafka"
)

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
	EditACLView
	DeleteACLView
	SessionManagerView
	GroupLagView
)

type TabView int

const (
	BrokersTab TabView = iota
	TopicsTab
	ConsumerGroupsTab
	ACLsTab
)

type Model struct {
	topicsTable         table.Model
	brokersTable        table.Model
	configTable         table.Model
	consumersTable      table.Model
	aclTable            *table.Model
	client              *kafka.Client
	topics              []kafka.TopicInfo
	brokers             []kafka.BrokerInfo
	consumerGroups      []kafka.ConsumerGroupInfo
	acls                []kafka.ACL
	topicConfig         *kafka.TopicConfig
	clusterStats        *kafka.ClusterStats
	err                 error
	loading             bool
	loadingConfig       bool
	width               int
	height              int
	mode                ViewMode
	producerModel       ProducerModel
	consumerModel       ConsumerModel
	createTopicModel    CreateTopicModel
	createACLModel      *CreateACLHuhModel
	editACLModel        EditACLHuhModel
	deleteACLModel      *DeleteACLModel
	editConfigModel     *EditConfigModel
	aiAssistantModel    AIAssistantModel
	deleteTopicModel    DeleteTopicModel
	sessionManagerModel SessionManagerModel
	spinner             spinner.Model
	selectedTopic       string
	activeTab           TabView
	focusedPanel        int // 0: topics list, 1: config table (when in Topics tab)
	aiEngine            string
	aiModel             string
	activeSession       string

	// View state layered on top of the fetched data.
	help          helpOverlay
	toast         toast
	tabStates     [4]tableState
	groupLagModel GroupLagModel
	brokerDetail  bool
	autoRefresh   bool
	loadingLabel  string

	// Unfiltered, unsorted rows for each tab. The tables hold the filtered and
	// sorted view of these, so changing a filter or a sort column never needs
	// another round trip to the cluster.
	topicRows    []table.Row
	brokerRows   []table.Row
	consumerRows []table.Row
	aclRows      []table.Row

	// topicMetrics arrives after the topic list, keyed by topic name.
	// metricsInFlight stops a second measurement starting while one is running:
	// auto-refresh ticks every five seconds, and on a large cluster a single
	// pass takes longer than that.
	topicMetrics    map[string]kafka.TopicMetrics
	metricsInFlight bool
}

func NewModel(client *kafka.Client, aiEngine string, aiModel string, activeSession string) Model {
	topicsTable := table.New(
		table.WithColumns(topicsColumns),
		table.WithFocused(false),
		table.WithHeight(10),
	)

	brokersTable := table.New(
		table.WithColumns(brokersColumns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	// Set styles for both tables
	s := table.DefaultStyles()
	s.Header = tableHeaderStyle()
	s.Selected = tableSelectedStyle()

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
	configStyles.Header = tableHeaderStyle().Foreground(theme.Primary)
	configStyles.Cell = lipgloss.NewStyle().Foreground(theme.Accent)
	configStyles.Selected = tableSelectedStyle()

	configTable.SetStyles(configStyles)

	consumersTable := table.New(
		table.WithColumns(consumersColumns),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	consumersTable.SetStyles(s)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(theme.Primary)

	return Model{
		topicsTable:    topicsTable,
		brokersTable:   brokersTable,
		configTable:    configTable,
		consumersTable: consumersTable,
		client:         client,
		spinner:        sp,
		loading:        true,
		loadingLabel:   "Connecting to the Kafka cluster…",
		mode:           ListView,
		activeTab:      BrokersTab,
		aiEngine:       aiEngine,
		aiModel:        aiModel,
		activeSession:  activeSession,
		tabStates: [4]tableState{
			BrokersTab:        newTableState("filter brokers…"),
			TopicsTab:         newTableState("filter topics…"),
			ConsumerGroupsTab: newTableState("filter groups…"),
			ACLsTab:           newTableState("filter ACLs…"),
		},
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

type clusterStatsMsg struct {
	stats *kafka.ClusterStats
	err   error
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

// ViewChangedMsg returns from a sub-view to a particular tab of the list.
// Notice, when set, is toasted over the list once it is showing again.
type ViewChangedMsg struct {
	View   TabView
	Notice string
	Level  toastLevel
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

func fetchClusterStats(client *kafka.Client) tea.Cmd {
	return func() tea.Msg {
		stats, err := client.GetClusterStats()
		return clusterStatsMsg{stats: stats, err: err}
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
	// Start spinner and add a small delay to allow connection to establish
	return tea.Batch(
		m.spinner.Tick,
		tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
			return tickMsg{}
		}),
	)
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
	case EditACLView:
		return m.updateEditACLView(msg)
	case DeleteACLView:
		return m.updateDeleteACLView(msg)
	case SessionManagerView:
		return m.updateSessionManagerView(msg)
	case GroupLagView:
		return m.updateGroupLagView(msg)
	default:
		return m.updateListView(msg)
	}
}

func (m Model) updateListView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Always update spinner
	var spinnerCmd tea.Cmd
	m.spinner, spinnerCmd = m.spinner.Update(msg)
	if spinnerCmd != nil {
		cmds = append(cmds, spinnerCmd)
	}

	switch msg := msg.(type) {
	case tickMsg:
		// Initial load after connection established
		return m, tea.Batch(fetchTopics(m.client), fetchBrokers(m.client), m.spinner.Tick)

	case tea.KeyMsg:
		// The help window is drawn over the list, so it takes every key while
		// it is open rather than letting them fall through to the table
		// underneath.
		if m.help.active {
			m.help, _ = m.help.Update(msg, m.height)
			return m, nil
		}

		// A focused filter input likewise swallows keys: while it has focus,
		// "d" is a letter being typed, not the delete binding.
		if m.tabStates[m.activeTab].filtering {
			return m.updateFilterKey(msg)
		}

		switch s := msg.String(); s {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?", "f1":
			m.help.active = true
			m.help.scroll = 0
			return m, nil
		case "/":
			state, cmd := m.tabStates[m.activeTab].startFilter()
			m.tabStates[m.activeTab] = state
			return m, cmd
		case "esc":
			// One key that undoes whatever narrowing is in effect.
			if m.brokerDetail {
				m.brokerDetail = false
				return m, nil
			}
			if m.tabStates[m.activeTab].active() {
				m.tabStates[m.activeTab] = m.tabStates[m.activeTab].stopFilter(true)
				m.rebuildActiveTab()
			}
			return m, nil
		case "<", ">":
			delta := 1
			if s == "<" {
				delta = -1
			}
			m.tabStates[m.activeTab] = m.tabStates[m.activeTab].
				moveSort(delta, len(columnsFor(m.activeTab)))
			m.rebuildActiveTab()
			return m, nil
		case "g":
			if t, ok := m.activeTable(); ok {
				t.GotoTop()
			}
			return m, nil
		case "G":
			if t, ok := m.activeTable(); ok {
				t.GotoBottom()
			}
			return m, nil
		case "y":
			// On the config table the value on its own is what anyone wants to
			// paste; elsewhere the whole row is the useful unit.
			if m.activeTab == TopicsTab && m.focusedPanel == 1 {
				if row := m.configTable.SelectedRow(); len(row) == 2 {
					return m, copyTextCmd(row[1], "config value")
				}
				return m, nil
			}
			if t, ok := m.activeTable(); ok {
				return m, copyRowCmd(t.SelectedRow())
			}
			return m, nil
		case "ctrl+r":
			m.autoRefresh = !m.autoRefresh
			if m.autoRefresh {
				var cmd tea.Cmd
				m.toast, cmd = m.toast.show(toastInfo, "Auto-refresh on (every 5s)")
				return m, tea.Batch(cmd, autoRefreshTick())
			}
			var cmd tea.Cmd
			m.toast, cmd = m.toast.show(toastInfo, "Auto-refresh off")
			return m, cmd
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
			m.blurActiveTab()
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
			m.blurActiveTab()
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
			m.blurActiveTab()
			m.activeTab = BrokersTab
			m.brokersTable.Focus()
			return m, fetchBrokers(m.client)
		case "2":
			m.blurActiveTab()
			m.activeTab = TopicsTab
			m.topicsTable.Focus()
			m.focusedPanel = 0
			return m, fetchTopics(m.client)
		case "3":
			m.blurActiveTab()
			m.activeTab = ConsumerGroupsTab
			m.consumersTable.Focus()
			return m, fetchConsumerGroups(m.client)
		case "4":
			m.blurActiveTab()
			m.activeTab = ACLsTab
			if m.aclTable != nil {
				m.aclTable.Focus()
			}
			return m, fetchACLs(m.client)
		case "r", "R":
			m.loading = true
			switch m.activeTab {
			case ACLsTab:
				m.loadingLabel = "Loading ACLs…"
				return m, fetchACLs(m.client)
			case ConsumerGroupsTab:
				m.loadingLabel = "Measuring consumer group lag…"
				return m, fetchConsumerGroups(m.client)
			default:
				m.loadingLabel = "Loading topics and brokers…"
				return m, tea.Batch(fetchTopics(m.client), fetchBrokers(m.client))
			}
		case "C":
			if m.activeTab == ACLsTab {
				// Create ACL
				m.createACLModel = NewCreateACLHuhModel(m.client, m.width, m.height)
				m.mode = CreateACLView
				return m, m.createACLModel.Init()
			} else {
				// Create Topic
				m.createTopicModel = NewCreateTopicModel(m.client, m.width, m.height)
				m.mode = CreateTopicView
				return m, m.createTopicModel.Init()
			}
		case "s", "S":
			// Open session manager
			m.sessionManagerModel = NewSessionManagerModel(m.activeSession)
			m.mode = SessionManagerView
			return m, m.sessionManagerModel.Init()
		case "A", "a":
			// Open AI Assistant
			m.aiAssistantModel = NewAIAssistantModel(m.client, m.aiEngine, m.aiModel)
			m.mode = AIAssistantView
			return m, m.aiAssistantModel.Init()
		case "D", "d":
			// Delete topic or ACL depending on active tab
			if m.activeTab == TopicsTab && len(m.topics) > 0 && !m.loading && m.err == nil {
				selectedRow := m.topicsTable.SelectedRow()
				if len(selectedRow) > 0 {
					m.selectedTopic = selectedRow[0]
					m.deleteTopicModel = NewDeleteTopicModel(m.client, m.selectedTopic)
					m.mode = DeleteTopicView
					return m, m.deleteTopicModel.Init()
				}
			} else if m.activeTab == ACLsTab && m.aclTable != nil && len(m.acls) > 0 && !m.loading && m.err == nil {
				// Delete ACL
				selectedRow := m.aclTable.SelectedRow()
				if len(selectedRow) >= 7 {
					// Create ACL from selected row data - matching table column order
					selectedACL := kafka.ACL{
						Principal:      selectedRow[0], // Principal
						ResourceType:   selectedRow[1], // Resource Type
						ResourceName:   selectedRow[2], // Resource
						PatternType:    selectedRow[3], // Pattern
						Operation:      selectedRow[4], // Operation
						PermissionType: selectedRow[5], // Permission
						Host:           selectedRow[6], // Host
					}
					m.deleteACLModel = NewDeleteACLModel(m.client, selectedACL, m.width, m.height)
					m.mode = DeleteACLView
					return m, m.deleteACLModel.Init()
				}
			}
			return m, nil
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
			return m, nil
		case "e", "E":
			// Edit config value or ACL
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
			} else if m.activeTab == ACLsTab && m.aclTable != nil && len(m.acls) > 0 {
				// Edit ACL
				selectedRow := m.aclTable.SelectedRow()
				if len(selectedRow) >= 7 {
					// Reconstruct the ACL from the selected row
					selectedACL := kafka.ACL{
						Principal:      selectedRow[0],
						ResourceType:   selectedRow[1],
						ResourceName:   selectedRow[2],
						PatternType:    selectedRow[3],
						Operation:      selectedRow[4],
						PermissionType: selectedRow[5],
						Host:           selectedRow[6],
					}
					m.editACLModel = NewEditACLHuhModel(m.client, selectedACL, m.width, m.height)
					m.mode = EditACLView
					return m, m.editACLModel.Init()
				}
			}
			return m, nil
		case "enter":
			if m.activeTab == ConsumerGroupsTab {
				selectedRow := m.consumersTable.SelectedRow()
				if len(selectedRow) > 0 {
					m.groupLagModel = NewGroupLagModel(m.client, selectedRow[0], m.width, m.height)
					m.mode = GroupLagView
					return m, m.groupLagModel.Init()
				}
				return m, nil
			}
			if m.activeTab == BrokersTab {
				m.brokerDetail = !m.brokerDetail
				return m, nil
			}
			if m.activeTab == TopicsTab && len(m.topics) > 0 && !m.loading && m.err == nil {
				selectedRow := m.topicsTable.SelectedRow()
				if len(selectedRow) > 0 {
					m.selectedTopic = selectedRow[0]
					m.consumerModel = NewConsumerModel(m.selectedTopic, m.client)
					m.mode = ConsumerView
					return m, m.consumerModel.Init()
				}
			}
			return m, nil
		}

	case topicsMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.topics = msg.topics
		m.err = nil
		m.rebuildTopicRows()

		// Message counts and disk usage are a separate, much more expensive
		// round trip, so the list paints now and the numbers arrive later.
		if !m.metricsInFlight {
			m.metricsInFlight = true
			cmds = append(cmds, fetchTopicMetrics(m.client))
		}

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
				cmds = append(cmds, fetchTopicConfig(m.client, topicName))
				return m, tea.Batch(cmds...)
			}
		}

	case topicMetricsMsg:
		m.metricsInFlight = false
		// Metrics are a nicety: a cluster that denies DescribeLogDirs still
		// shows a usable topic list, so a failure here is logged by the client
		// and the columns simply stay empty.
		if msg.err == nil {
			m.topicMetrics = msg.metrics
			m.rebuildTopicRows()
		}

	case toastExpiredMsg:
		m.toast = m.toast.expire(msg)

	case clipboardMsg:
		var cmd tea.Cmd
		m, cmd = m.handleClipboardMsg(msg)
		return m, cmd

	case autoRefreshTickMsg:
		if !m.autoRefresh {
			return m, nil
		}
		return m, tea.Batch(m.refreshActiveTab(), autoRefreshTick())

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
		m.rebuildBrokerRows()

		// Also fetch cluster stats when brokers are loaded
		return m, fetchClusterStats(m.client)

	case clusterStatsMsg:
		if msg.err == nil {
			m.clusterStats = msg.stats
		}
		// Don't set error here as it's not critical

	case consumerGroupsMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.consumerGroups = msg.groups
		m.err = nil
		m.rebuildConsumerRows()

	case aclsMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.acls = msg.acls
		m.err = nil

		// The ACL table is built on first use: a cluster with no authorizer
		// never opens this tab, and the columns are wide.
		if m.aclTable == nil {
			t := table.New(
				table.WithColumns(aclColumns),
				table.WithFocused(true),
				table.WithHeight(10),
			)

			as := table.DefaultStyles()
			as.Header = tableHeaderStyle()
			as.Selected = tableSelectedStyle()
			t.SetStyles(as)
			m.aclTable = &t
			m.aclTable.SetWidth(m.width - 4)
			m.aclTable.SetHeight(max(m.height-10, 3))
		}
		m.rebuildACLRows()

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

		// The ACL table is created lazily, so it misses the resize that built
		// the others and has to be caught up here.
		if m.aclTable != nil {
			m.aclTable.SetHeight(tableHeight)
			m.aclTable.SetWidth(msg.Width - 4)
		}

		// The topic name column is sized from the table width, so the rows are
		// rebuilt rather than just resized.
		m.rebuildTopicRows()
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
		return m.backToList(msg, nil, "")

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
		return m.backToList(msg, nil, "")

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
		return m.backToList(msg, fetchTopics(m.client), "Reloading topics…")

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
			m.activeTab = ACLsTab
			// Only a dialog that changed something carries a notice. Cancelling
			// out of one changed nothing on the cluster, so the list is already
			// correct and does not need a refetch or a spinner over it — the
			// same rule the delete-topic dialog follows.
			if msg.Notice == "" {
				return m.backToList(SwitchToListViewMsg{}, nil, "")
			}
			return m.backToList(
				SwitchToListViewMsg{Notice: msg.Notice, Level: msg.Level},
				fetchACLs(m.client), "Reloading ACLs…")
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	updatedModel, cmd := m.createACLModel.Update(msg)
	m.createACLModel = updatedModel.(*CreateACLHuhModel)
	return m, cmd
}

func (m Model) updateEditACLView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case ViewChangedMsg:
		if msg.View == ACLsTab {
			m.activeTab = ACLsTab
			// Only a dialog that changed something carries a notice. Cancelling
			// out of one changed nothing on the cluster, so the list is already
			// correct and does not need a refetch or a spinner over it — the
			// same rule the delete-topic dialog follows.
			if msg.Notice == "" {
				return m.backToList(SwitchToListViewMsg{}, nil, "")
			}
			return m.backToList(
				SwitchToListViewMsg{Notice: msg.Notice, Level: msg.Level},
				fetchACLs(m.client), "Reloading ACLs…")
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	updatedModel, cmd := m.editACLModel.Update(msg)
	m.editACLModel = updatedModel.(EditACLHuhModel)
	return m, cmd
}

func (m Model) updateDeleteACLView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ViewChangedMsg:
		if msg.View == ACLsTab {
			m.activeTab = ACLsTab
			// Only a dialog that changed something carries a notice. Cancelling
			// out of one changed nothing on the cluster, so the list is already
			// correct and does not need a refetch or a spinner over it — the
			// same rule the delete-topic dialog follows.
			if msg.Notice == "" {
				return m.backToList(SwitchToListViewMsg{}, nil, "")
			}
			return m.backToList(
				SwitchToListViewMsg{Notice: msg.Notice, Level: msg.Level},
				fetchACLs(m.client), "Reloading ACLs…")
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	_, cmd := m.deleteACLModel.Update(msg)
	return m, cmd
}

func (m Model) updateEditConfigView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case SwitchToListViewMsg:
		// Reload the config so the table shows the value that was just written.
		return m.backToList(msg, fetchTopicConfig(m.client, m.selectedTopic), "")

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
		return m.backToList(msg, fetchTopics(m.client), "Reloading topics…")

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
		// Only a delete that happened carries a notice. Cancelling the dialog
		// changed nothing, so the list is already correct and does not need a
		// spinner over it.
		if msg.Notice == "" {
			return m.backToList(msg, nil, "")
		}
		return m.backToList(msg, fetchTopics(m.client), "Reloading topics…")

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	updatedModel, cmd := m.deleteTopicModel.Update(msg)
	m.deleteTopicModel = updatedModel

	return m, cmd
}

func (m Model) updateSessionManagerView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SwitchToListViewMsg:
		return m.backToList(msg, nil, "")
	}

	updated, cmd := m.sessionManagerModel.Update(msg)
	m.sessionManagerModel = updated
	return m, cmd
}

func (m Model) updateGroupLagView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SwitchToListViewMsg:
		return m.backToList(msg, fetchConsumerGroups(m.client), "")

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case clipboardMsg:
		var cmd tea.Cmd
		m, cmd = m.handleClipboardMsg(msg)
		return m, cmd

	case toastExpiredMsg:
		m.toast = m.toast.expire(msg)
		return m, nil
	}

	updated, cmd := m.groupLagModel.Update(msg)
	m.groupLagModel = updated
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
	case EditACLView:
		return m.editACLModel.View()
	case DeleteACLView:
		return m.deleteACLModel.View()
	case EditConfigView:
		return m.editConfigModel.View()
	case AIAssistantView:
		return m.aiAssistantModel.View()
	case DeleteTopicView:
		return m.deleteTopicModel.View()
	case SessionManagerView:
		return m.sessionManagerModel.View()
	case GroupLagView:
		view := m.groupLagModel.View()
		if m.toast.visible() {
			view = lipgloss.JoinVertical(lipgloss.Left, view, m.toast.View(m.width))
		}
		return m.withOverlays(view)
	default:
		return m.withOverlays(m.listView())
	}
}

// withOverlays draws the help window over a view, if it is open.
//
// The help window replaces the screen rather than compositing onto it: Bubble
// Tea renders a single string and slicing styled text apart to punch a hole in
// it is how escape sequences get torn in half.
func (m Model) withOverlays(view string) string {
	if m.help.active {
		return m.help.View(m.width, m.height)
	}
	if m.brokerDetail && m.activeTab == BrokersTab && m.mode == ListView {
		if detail, ok := m.brokerDetailBox(); ok {
			return renderOverlay(detail, m.width, m.height)
		}
	}
	return view
}

// brokerDetailBox renders everything known about the selected broker.
//
// The table has to fit eight columns across the terminal, so it abbreviates;
// this is where the full host, the listener and log-directory counts, and the
// negotiated API version are readable.
func (m Model) brokerDetailBox() (string, bool) {
	row := m.brokersTable.SelectedRow()
	if len(row) == 0 {
		return "", false
	}

	var selected *kafka.BrokerInfo
	for i := range m.brokers {
		if fmt.Sprintf("%d", m.brokers[i].ID) == row[0] {
			selected = &m.brokers[i]
			break
		}
	}
	if selected == nil {
		return "", false
	}

	role := "Broker"
	if selected.IsController {
		role = "Broker + Controller"
	}

	status := successStyle.Render(selected.Status)
	if selected.Status != "Online" {
		status = errorStyle.Render(selected.Status)
	}

	fields := []struct{ label, value string }{
		{"ID", fmt.Sprintf("%d", selected.ID)},
		{"Address", fmt.Sprintf("%s:%d", selected.Host, selected.Port)},
		{"Role", role},
		{"Rack", orDash(selected.Rack)},
		{"API version", orDash(selected.ApiVersions)},
		{"Listeners", fmt.Sprintf("%d", selected.ListenerCount)},
		{"Log directories", fmt.Sprintf("%d", selected.LogDirCount)},
	}

	var sb strings.Builder
	sb.WriteString(sectionTitleStyle.Render("Broker " + row[0]))
	sb.WriteString("\n\n")
	sb.WriteString(labelStyle.Render(pad("Status", 16)))
	sb.WriteString(status)
	sb.WriteString("\n")
	for _, f := range fields {
		sb.WriteString(labelStyle.Render(pad(f.label, 16)))
		sb.WriteString(valueStyle.Render(f.value))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	sb.WriteString(renderHelpBar("y", "copy row", "esc", "close"))

	return overlayBoxStyle(min(max(m.width-10, 30), 60)).Render(sb.String()), true
}

// orDash renders an empty value as a dash, so a blank line never reads as a
// missing field.
func orDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func (m Model) listView() string {
	// Build the three sections: header, content, footer
	header := m.renderTabBar()

	var content string
	if m.loading {
		loadingStyle := lipgloss.NewStyle().
			Foreground(theme.SubText).
			PaddingLeft(2).
			PaddingTop(1)
		label := m.loadingLabel
		if label == "" {
			label = "Loading…"
		}
		content = loadingStyle.Render(m.spinner.View() + "  " + label)
	} else if m.err != nil {
		errBox := lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(theme.Error).
			Padding(1, 2).
			Width(60)
		content = errBox.Render(
			errorStyle.Render("Error: "+m.err.Error()) + "\n\n" +
				labelStyle.Render("Press ") + helpKeyStyle.Render("r") + labelStyle.Render(" to retry or ") +
				helpKeyStyle.Render("q") + labelStyle.Render(" to quit"),
		)
	} else {
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
	}

	footer := m.renderStatusBar()
	if m.toast.visible() {
		footer = lipgloss.JoinVertical(lipgloss.Left, m.toast.View(m.width), footer)
	}
	if m.tabStates[m.activeTab].filtering {
		footer = lipgloss.JoinVertical(lipgloss.Left,
			m.tabStates[m.activeTab].filter.View(), footer)
	}

	// Stack: header + content + spacer + footer
	// Calculate available height for content
	headerHeight := lipgloss.Height(header)
	footerHeight := lipgloss.Height(footer)
	contentHeight := m.height - headerHeight - footerHeight - 3 // 3 for spacing

	paddedContent := lipgloss.NewStyle().
		Height(contentHeight).
		Render(content)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		paddedContent,
		footer,
	)
}

func (m Model) renderTabBar() string {
	tabs := []struct {
		key  string
		name string
	}{
		{"1", "Brokers"},
		{"2", "Topics"},
		{"3", "Groups"},
		{"4", "ACLs"},
	}

	var renderedTabs []string
	for i, tab := range tabs {
		label := tab.key + " " + tab.name
		if TabView(i) == m.activeTab {
			renderedTabs = append(renderedTabs, activeTabStyle.Render(label))
		} else {
			renderedTabs = append(renderedTabs, inactiveTabStyle.Render(label))
		}
	}

	row := lipgloss.JoinHorizontal(lipgloss.Bottom, renderedTabs...)

	// Fill remaining width with a bottom border
	rowWidth := lipgloss.Width(row)
	if remaining := m.width - rowWidth - 2; remaining > 0 {
		gap := tabGapStyle.Width(remaining).Render("")
		row = lipgloss.JoinHorizontal(lipgloss.Bottom, row, gap)
	}

	// Title line above tabs
	title := appHeaderStyle.Render("KConduit")
	titleRight := ""
	if m.activeSession != "" {
		titleRight = lipgloss.NewStyle().
			Foreground(theme.Accent).
			Render("⏣ " + m.activeSession)
	}
	titleLine := lipgloss.JoinHorizontal(lipgloss.Top,
		title,
		lipgloss.NewStyle().Width(m.width-lipgloss.Width(title)-lipgloss.Width(titleRight)-2).Render(""),
		titleRight,
	)

	return lipgloss.JoinVertical(lipgloss.Left, titleLine, row)
}

func (m Model) renderStatusBar() string {
	// Right side: which tab is showing, plus anything narrowing it.
	right := statusBarModeStyle.Render(m.tabLabel())

	var indicators []string
	if state := m.tabStates[m.activeTab]; state.active() {
		indicators = append(indicators, fmt.Sprintf("filter %q  %d/%d",
			state.filter.Value(), m.visibleRowCount(), m.totalRowCount()))
	}
	if m.autoRefresh {
		indicators = append(indicators, "⟳ auto")
	}
	if len(indicators) > 0 {
		right = lipgloss.JoinHorizontal(lipgloss.Top,
			statusBarStyle.Render(strings.Join(indicators, "  ·  ")), right)
	}

	// The bar is one row, always: a left side long enough to wrap would push
	// the whole layout a row past the bottom of the terminal, so the help items
	// are fitted to the space that is left and MaxHeight makes a mistake there
	// cost an item rather than the layout.
	leftWidth := max(m.width-lipgloss.Width(right)-2, 0)
	left := statusBarStyle.Width(leftWidth).MaxHeight(1).Render(m.renderHelpItems(leftWidth))

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

// tabLabel names the active tab for the status bar.
func (m Model) tabLabel() string {
	switch m.activeTab {
	case BrokersTab:
		return "BROKERS"
	case TopicsTab:
		return "TOPICS"
	case ConsumerGroupsTab:
		return "GROUPS"
	case ACLsTab:
		return "ACLS"
	}
	return ""
}

// renderHelpItems builds the footer hints for the active tab, most useful
// first, and drops the ones that do not fit in width.
//
// "?" is always kept, and kept last: whatever else is cut, the way to find the
// rest of the bindings stays on screen.
func (m Model) renderHelpItems(width int) string {
	items := []string{
		"tab", "switch",
		"/", "filter",
		"enter", m.enterAction(),
	}

	switch m.activeTab {
	case TopicsTab:
		if m.focusedPanel == 1 {
			items = append(items, "e", "edit config")
		}
		items = append(items, "p", "produce", "C", "create", "d", "delete")
	case ACLsTab:
		items = append(items, "C", "create")
		if len(m.acls) > 0 {
			items = append(items, "e", "edit", "d", "delete")
		}
	}

	items = append(items, "r", "refresh", "y", "copy", "s", "sessions", "a", "AI", "q", "quit")

	// Trim two at a time (key and description) from just before the trailing
	// "?" until the line fits.
	for {
		line := renderHelpBar(append(items, "?", "help")...)
		if width <= 0 || lipgloss.Width(line) <= width || len(items) <= 2 {
			return line
		}
		items = items[:len(items)-2]
	}
}

// enterAction says what Enter does on the active tab, since it differs per tab.
func (m Model) enterAction() string {
	switch m.activeTab {
	case ConsumerGroupsTab:
		return "lag detail"
	case BrokersTab:
		return "broker detail"
	case TopicsTab:
		return "consume"
	}
	return "open"
}

func (m Model) renderBrokersView() string {
	if len(m.brokers) == 0 {
		return labelStyle.Render("  No brokers found.")
	}

	// Calculate broker statistics
	totalBrokers := len(m.brokers)
	offlineBrokers := 0
	controllerID := -1

	for _, broker := range m.brokers {
		if broker.Status != "Online" {
			offlineBrokers++
		}
		if broker.IsController {
			controllerID = int(broker.ID)
		}
	}

	// Left panel: brokers table (70% width)
	leftPanelWidth := int(float64(m.width-6) * 0.7)
	leftPanel := activePanelStyle.
		Width(leftPanelWidth).
		Height(m.height - 12)
	brokersTableView := leftPanel.Render(m.brokersTable.View())

	// Right panel: cluster info (30% width)
	rightPanelWidth := m.width - leftPanelWidth - 6
	infoBox := panelStyle.
		Padding(1, 2).
		Width(rightPanelWidth).
		Height(m.height - 12)

	var sb strings.Builder
	sb.WriteString(sectionTitleStyle.Render("Cluster Status"))
	sb.WriteString("\n\n")

	// Broker count
	sb.WriteString(labelStyle.Render("Brokers  "))
	sb.WriteString(valueStyle.Render(fmt.Sprintf("%d", totalBrokers)))
	sb.WriteString("\n")

	// Status
	sb.WriteString(labelStyle.Render("Health   "))
	if offlineBrokers == 0 {
		sb.WriteString(successStyle.Render(fmt.Sprintf("%d/%d online", totalBrokers, totalBrokers)))
	} else {
		sb.WriteString(errorStyle.Render(fmt.Sprintf("%d offline", offlineBrokers)))
	}
	sb.WriteString("\n")

	// Controller
	sb.WriteString(labelStyle.Render("Leader   "))
	if controllerID >= 0 {
		sb.WriteString(valueStyle.Render(fmt.Sprintf("node %d", controllerID)))
	} else {
		sb.WriteString(warningStyle.Render("unknown"))
	}

	sb.WriteString("\n\n")
	sb.WriteString(sectionTitleStyle.Render("Partitions"))
	sb.WriteString("\n\n")

	if m.clusterStats != nil {
		sb.WriteString(labelStyle.Render("Total    "))
		sb.WriteString(valueStyle.Render(fmt.Sprintf("%d", m.clusterStats.TotalPartitions)))
		sb.WriteString("\n")

		sb.WriteString(labelStyle.Render("Replicas "))
		sb.WriteString(valueStyle.Render(fmt.Sprintf("%d", m.clusterStats.TotalReplicas)))
		sb.WriteString("\n")

		sb.WriteString(labelStyle.Render("Under-rep"))
		if m.clusterStats.UnderReplicatedPartitions == 0 {
			sb.WriteString(successStyle.Render(" none"))
		} else {
			sb.WriteString(errorStyle.Render(fmt.Sprintf(" %d", m.clusterStats.UnderReplicatedPartitions)))
		}

		if m.clusterStats.OfflinePartitions > 0 {
			sb.WriteString("\n")
			sb.WriteString(labelStyle.Render("Offline  "))
			sb.WriteString(errorStyle.Render(fmt.Sprintf("%d", m.clusterStats.OfflinePartitions)))
		}
	} else {
		totalPartitions := 0
		totalReplicas := 0
		for _, topic := range m.topics {
			totalPartitions += topic.Partitions
			totalReplicas += topic.Partitions * topic.ReplicationFactor
		}
		sb.WriteString(labelStyle.Render("Total    "))
		sb.WriteString(valueStyle.Render(fmt.Sprintf("%d", totalPartitions)))
		sb.WriteString("\n")
		sb.WriteString(labelStyle.Render("Replicas "))
		sb.WriteString(valueStyle.Render(fmt.Sprintf("%d", totalReplicas)))
		sb.WriteString("\n")
		sb.WriteString(labelStyle.Render(m.spinner.View() + " fetching stats..."))
	}

	infoBoxView := infoBox.Render(sb.String())

	return lipgloss.JoinHorizontal(lipgloss.Top, brokersTableView, " ", infoBoxView)
}

func (m Model) renderTopicsView() string {
	if len(m.topics) == 0 {
		return labelStyle.Render("  No topics found.")
	}

	halfWidth := (m.width - 6) / 2
	panelHeight := m.height - 12

	// Left panel: topics list
	leftBorder := panelStyle
	if m.focusedPanel == 0 {
		leftBorder = activePanelStyle
	}
	leftPanel := leftBorder.Width(halfWidth).Height(panelHeight)
	topicsView := leftPanel.Render(m.topicsTable.View())

	// Right panel: topic config
	rightBorder := panelStyle
	if m.focusedPanel == 1 {
		rightBorder = activePanelStyle
	}
	rightPanel := rightBorder.Width(halfWidth).Height(panelHeight).Padding(1)

	var configView string
	if m.loadingConfig {
		configView = rightPanel.Render(m.spinner.View() + "  Loading configuration...")
	} else if m.topicConfig != nil {
		configView = rightPanel.Render(m.renderTopicConfig())
	} else {
		configView = rightPanel.Render(labelStyle.Render("Select a topic to view configuration"))
	}

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

	sb.WriteString(sectionTitleStyle.Render(m.topicConfig.Name))
	sb.WriteString("\n")

	// Compact stats line
	sb.WriteString(labelStyle.Render(fmt.Sprintf("partitions %d  replication %d",
		m.topicConfig.Partitions, m.topicConfig.ReplicationFactor)))
	sb.WriteString("\n\n")

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
		return labelStyle.PaddingLeft(2).Render(m.spinner.View() + "  Loading consumer groups...")
	}

	if m.err != nil {
		return errorStyle.PaddingLeft(2).Render("Error: " + m.err.Error())
	}

	if len(m.consumerGroups) == 0 {
		return labelStyle.PaddingLeft(2).Render("No consumer groups found")
	}

	panel := activePanelStyle.
		Width(m.width - 4).
		Height(m.height - 12)

	return panel.Render(m.consumersTable.View())
}

func (m Model) renderACLsView() string {
	if m.aclTable == nil {
		return labelStyle.PaddingLeft(2).Render(m.spinner.View() + "  Loading ACLs...")
	}

	if len(m.acls) == 0 {
		empty := labelStyle.Render("No ACLs found. Press ") +
			helpKeyStyle.Render("C") +
			labelStyle.Render(" to create one.")
		return lipgloss.NewStyle().PaddingLeft(2).PaddingTop(1).Render(empty)
	}

	panel := activePanelStyle.
		Width(m.width - 4).
		Height(m.height - 12)

	return panel.Render(m.aclTable.View())
}
