package ui

import (
	"fmt"
	"strings"

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

type Model struct {
	table            table.Model
	client           *kafka.Client
	topics           []kafka.TopicInfo
	err              error
	loading          bool
	width            int
	height           int
	mode             ViewMode
	producerModel    ProducerModel
	consumerModel    ConsumerModel
	createTopicModel CreateTopicModel
	selectedTopic    string
}

func NewModel(client *kafka.Client) Model {
	columns := []table.Column{
		{Title: "Topic Name", Width: 40},
		{Title: "Partitions", Width: 12},
		{Title: "Replication Factor", Width: 18},
	}

	t := table.New(
		table.WithColumns(columns),
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

	return Model{
		table:   t,
		client:  client,
		loading: true,
		mode:    ListView,
	}
}

type tickMsg struct{}

type topicsMsg struct {
	topics []kafka.TopicInfo
	err    error
}

func fetchTopics(client *kafka.Client) tea.Cmd {
	return func() tea.Msg {
		topics, err := client.GetTopicDetails()
		return topicsMsg{topics: topics, err: err}
	}
}

func (m Model) Init() tea.Cmd {
	return fetchTopics(m.client)
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
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			m.loading = true
			return m, fetchTopics(m.client)
		case "c", "C":
			m.createTopicModel = NewCreateTopicModel(m.client)
			m.mode = CreateTopicView
			return m, m.createTopicModel.Init()
		case "p", "P":
			if len(m.topics) > 0 && !m.loading && m.err == nil {
				selectedRow := m.table.SelectedRow()
				if len(selectedRow) > 0 {
					m.selectedTopic = selectedRow[0]
					m.producerModel = NewProducerModel(m.selectedTopic, m.client)
					m.mode = ProducerView
					return m, m.producerModel.Init()
				}
			}
		case "enter":
			if len(m.topics) > 0 && !m.loading && m.err == nil {
				selectedRow := m.table.SelectedRow()
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
		m.table.SetRows(rows)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetHeight(msg.Height - 8)
		m.table.SetWidth(msg.Width - 4)
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
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

	sb.WriteString("🚀 KConduit - Kafka Topics\n\n")

	if m.loading {
		sb.WriteString("Loading topics...")
		return sb.String()
	}

	if m.err != nil {
		sb.WriteString(fmt.Sprintf("Error: %v\n", m.err))
		sb.WriteString("\nPress 'r' to retry or 'q' to quit")
		return sb.String()
	}

	sb.WriteString(baseStyle.Render(m.table.View()))
	sb.WriteString("\n\n")
	sb.WriteString("Enter: Consume | P: Produce | C: Create Topic | r: Refresh | q: Quit")

	return sb.String()
}