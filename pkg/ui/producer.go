package ui

import (
	"fmt"
	"strings"

	"github.com/digitalis-io/kconduit/pkg/kafka"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ProducerModel struct {
	topic       string
	topicInfo   *kafka.TopicInfo
	client      *kafka.Client
	keyInput    textinput.Model
	valueInput  textarea.Model
	focusIndex  int
	err         error
	successMsg  string
	width       int
	height      int
	msgCount    int
}

func NewProducerModel(topic string, client *kafka.Client) ProducerModel {
	ki := textinput.New()
	ki.Placeholder = "Message key (optional, press Enter to skip)"
	ki.Focus()
	ki.CharLimit = 256
	ki.Width = 50

	vi := textarea.New()
	vi.Placeholder = "Enter your message here...\n\nPress Ctrl+S to send, Esc to go back"
	vi.CharLimit = 10000
	vi.SetWidth(50)
	vi.SetHeight(10)

	// Fetch topic information
	var topicInfo *kafka.TopicInfo
	topics, err := client.GetTopicDetails()
	if err == nil {
		for _, t := range topics {
			if t.Name == topic {
				info := t
				topicInfo = &info
				break
			}
		}
	}

	return ProducerModel{
		topic:      topic,
		topicInfo:  topicInfo,
		client:     client,
		keyInput:   ki,
		valueInput: vi,
		focusIndex: 0,
		msgCount:   0,
	}
}

type messageSentMsg struct {
	err error
}

func sendMessage(client *kafka.Client, topic, key, value string) tea.Cmd {
	return func() tea.Msg {
		err := client.ProduceMessage(topic, key, value)
		return messageSentMsg{err: err}
	}
}

func (m ProducerModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m ProducerModel) Update(msg tea.Msg) (ProducerModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			return m, ReturnToListView

		case tea.KeyTab, tea.KeyEnter:
			if m.focusIndex == 0 {
				m.focusIndex = 1
				m.keyInput.Blur()
				cmds = append(cmds, m.valueInput.Focus())
			}

		case tea.KeyCtrlS:
			if m.valueInput.Value() != "" {
				key := m.keyInput.Value()
				value := m.valueInput.Value()
				return m, sendMessage(m.client, m.topic, key, value)
			}
		}

	case messageSentMsg:
		if msg.err != nil {
			m.err = msg.err
			m.successMsg = ""
		} else {
			m.err = nil
			m.msgCount++
			m.successMsg = fmt.Sprintf("✓ Message sent successfully! (Total sent: %d)", m.msgCount)
			m.keyInput.SetValue("")
			m.valueInput.SetValue("")
			m.focusIndex = 0
			m.keyInput.Focus()
			m.valueInput.Blur()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Adjust input widths with some padding for the table
		inputWidth := msg.Width - 10
		if inputWidth > 100 {
			inputWidth = 100 // Cap max width for readability
		}
		m.keyInput.Width = inputWidth
		m.valueInput.SetWidth(inputWidth)
		// Adjust height accounting for the new table and headers
		valueHeight := msg.Height - 25
		if valueHeight < 5 {
			valueHeight = 5
		} else if valueHeight > 20 {
			valueHeight = 20
		}
		m.valueInput.SetHeight(valueHeight)
	}

	if m.focusIndex == 0 {
		var cmd tea.Cmd
		m.keyInput, cmd = m.keyInput.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		var cmd tea.Cmd
		m.valueInput, cmd = m.valueInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m ProducerModel) View() string {
	var sb strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Padding(0, 1)

	sb.WriteString(headerStyle.Render("📝 Kafka Producer"))
	sb.WriteString("\n\n")

	// Topic Information Table
	tableStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("86")).
		Padding(1, 2)

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("229"))

	var tableContent strings.Builder
	tableContent.WriteString(labelStyle.Render("📋 Topic Details") + "\n")
	tableContent.WriteString(strings.Repeat("─", 60) + "\n\n")
	
	tableContent.WriteString(labelStyle.Render("Topic Name:       "))
	tableContent.WriteString(valueStyle.Render(m.topic) + "\n")
	
	if m.topicInfo != nil {
		tableContent.WriteString(labelStyle.Render("Partitions:       "))
		tableContent.WriteString(valueStyle.Render(fmt.Sprintf("%d", m.topicInfo.Partitions)) + "\n")
		
		tableContent.WriteString(labelStyle.Render("Replication:      "))
		tableContent.WriteString(valueStyle.Render(fmt.Sprintf("%d", m.topicInfo.ReplicationFactor)) + "\n")
	}
	
	tableContent.WriteString(labelStyle.Render("Messages Sent:    "))
	tableContent.WriteString(valueStyle.Render(fmt.Sprintf("%d", m.msgCount)) + "\n")
	
	tableContent.WriteString(labelStyle.Render("Status:           "))
	if m.err != nil {
		tableContent.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("❌ Error"))
	} else if m.successMsg != "" {
		tableContent.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render("✅ Ready"))
	} else {
		tableContent.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render("🔄 Composing"))
	}

	sb.WriteString(tableStyle.Render(tableContent.String()))
	sb.WriteString("\n\n")

	// Input Fields
	inputHeaderStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86"))

	sb.WriteString(inputHeaderStyle.Render("📨 Message Composer"))
	sb.WriteString("\n\n")

	sb.WriteString(labelStyle.Render("Key:") + "\n")
	sb.WriteString(m.keyInput.View())
	sb.WriteString("\n\n")

	sb.WriteString(labelStyle.Render("Value:") + "\n")
	sb.WriteString(m.valueInput.View())
	sb.WriteString("\n\n")

	// Status Messages
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)
		sb.WriteString(errorStyle.Render(fmt.Sprintf("❌ Error: %v", m.err)))
		sb.WriteString("\n")
	}

	if m.successMsg != "" {
		successStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("46")).
			Bold(true)
		sb.WriteString(successStyle.Render(m.successMsg))
		sb.WriteString("\n")
	}

	// Help text
	sb.WriteString("\n")
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true)
	sb.WriteString(helpStyle.Render("Tab: Switch fields • Ctrl+S: Send message • Esc: Back to topics"))

	return sb.String()
}

func ReturnToListView() tea.Msg {
	return SwitchToListViewMsg{}
}

type SwitchToListViewMsg struct{}