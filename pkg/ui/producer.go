package ui

import (
	"fmt"
	"strings"

	"github.com/axonops/kconduit/pkg/kafka"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ProducerModel struct {
	topic       string
	client      *kafka.Client
	keyInput    textinput.Model
	valueInput  textarea.Model
	focusIndex  int
	err         error
	successMsg  string
	width       int
	height      int
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

	return ProducerModel{
		topic:      topic,
		client:     client,
		keyInput:   ki,
		valueInput: vi,
		focusIndex: 0,
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
			m.successMsg = "✓ Message sent successfully!"
			m.keyInput.SetValue("")
			m.valueInput.SetValue("")
			m.focusIndex = 0
			m.keyInput.Focus()
			m.valueInput.Blur()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.keyInput.Width = msg.Width - 4
		m.valueInput.SetWidth(msg.Width - 4)
		m.valueInput.SetHeight(msg.Height - 15)
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

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Padding(0, 1)

	sb.WriteString(headerStyle.Render(fmt.Sprintf("📝 Producer Mode - Topic: %s", m.topic)))
	sb.WriteString("\n\n")

	sb.WriteString("Key:\n")
	sb.WriteString(m.keyInput.View())
	sb.WriteString("\n\n")

	sb.WriteString("Value:\n")
	sb.WriteString(m.valueInput.View())
	sb.WriteString("\n\n")

	if m.err != nil {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
		sb.WriteString(errorStyle.Render(fmt.Sprintf("❌ Error: %v", m.err)))
		sb.WriteString("\n")
	}

	if m.successMsg != "" {
		successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
		sb.WriteString(successStyle.Render(m.successMsg))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	sb.WriteString(helpStyle.Render("Tab: Switch fields • Ctrl+S: Send message • Esc: Back to topics"))

	return sb.String()
}

func ReturnToListView() tea.Msg {
	return SwitchToListViewMsg{}
}

type SwitchToListViewMsg struct{}