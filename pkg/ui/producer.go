package ui

import (
	"fmt"
	"strings"

	"github.com/digitalis-io/kconduit/pkg/kafka"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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
	sb.WriteString(appHeaderStyle.Render("Producer"))
	sb.WriteString("  ")
	sb.WriteString(valueStyle.Render(m.topic))
	sb.WriteString("\n\n")

	// Topic info panel
	var infoContent strings.Builder
	infoContent.WriteString(labelStyle.Render("Topic      ") + valueStyle.Render(m.topic) + "\n")
	if m.topicInfo != nil {
		infoContent.WriteString(labelStyle.Render("Partitions ") + valueStyle.Render(fmt.Sprintf("%d", m.topicInfo.Partitions)) + "\n")
		infoContent.WriteString(labelStyle.Render("Replicas   ") + valueStyle.Render(fmt.Sprintf("%d", m.topicInfo.ReplicationFactor)) + "\n")
	}
	infoContent.WriteString(labelStyle.Render("Sent       ") + valueStyle.Render(fmt.Sprintf("%d", m.msgCount)) + "\n")
	infoContent.WriteString(labelStyle.Render("Status     "))
	if m.err != nil {
		infoContent.WriteString(errorStyle.Render("error"))
	} else if m.successMsg != "" {
		infoContent.WriteString(successStyle.Render("ready"))
	} else {
		infoContent.WriteString(warningStyle.Render("composing"))
	}

	sb.WriteString(panelStyle.Padding(1, 2).Render(infoContent.String()))
	sb.WriteString("\n\n")

	// Input fields
	sb.WriteString(sectionTitleStyle.Render("Message"))
	sb.WriteString("\n\n")

	sb.WriteString(labelStyle.Render("Key") + "\n")
	sb.WriteString(m.keyInput.View())
	sb.WriteString("\n\n")

	sb.WriteString(labelStyle.Render("Value") + "\n")
	sb.WriteString(m.valueInput.View())
	sb.WriteString("\n\n")

	// Status
	if m.err != nil {
		sb.WriteString(errorStyle.Bold(true).Render("Error: "+m.err.Error()) + "\n")
	}
	if m.successMsg != "" {
		sb.WriteString(successStyle.Bold(true).Render(m.successMsg) + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString(renderHelpBar("tab", "switch fields", "ctrl+s", "send", "esc", "back"))

	return sb.String()
}

func ReturnToListView() tea.Msg {
	return SwitchToListViewMsg{}
}

type SwitchToListViewMsg struct{}