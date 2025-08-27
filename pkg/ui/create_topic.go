package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/axonops/kconduit/pkg/kafka"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type CreateTopicModel struct {
	client     *kafka.Client
	inputs     []textinput.Model
	focusIndex int
	err        error
	successMsg string
	width      int
	height     int
}

const (
	topicNameIdx = iota
	partitionsIdx
	replicationIdx
)

var (
	focusedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	blurredStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursorStyle  = focusedStyle.Copy()
	noStyle      = lipgloss.NewStyle()
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	focusedButton = focusedStyle.Copy().Render("[ Create ]")
	blurredButton = fmt.Sprintf("[ %s ]", blurredStyle.Render("Create"))
)

func NewCreateTopicModel(client *kafka.Client) CreateTopicModel {
	m := CreateTopicModel{
		client: client,
		inputs: make([]textinput.Model, 3),
	}

	var t textinput.Model
	for i := range m.inputs {
		t = textinput.New()
		t.Cursor.Style = cursorStyle
		t.CharLimit = 32

		switch i {
		case topicNameIdx:
			t.Prompt = "Topic Name: "
			t.Placeholder = ""
			t.Focus()
			t.PromptStyle = focusedStyle
			t.TextStyle = focusedStyle
			t.CharLimit = 255
		case partitionsIdx:
			t.Prompt = "Number of partitions (default: 1): "
			t.Placeholder = "1"
			t.CharLimit = 5
		case replicationIdx:
			t.Prompt = "Replication factor (default: 1): "
			t.Placeholder = "1"
			t.CharLimit = 3
		}

		m.inputs[i] = t
	}

	return m
}

type topicCreatedMsg struct {
	name string
	err  error
}

func createTopic(client *kafka.Client, name string, partitions int32, replication int16) tea.Cmd {
	return func() tea.Msg {
		err := client.CreateTopic(name, partitions, replication)
		return topicCreatedMsg{name: name, err: err}
	}
}

func (m CreateTopicModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m CreateTopicModel) Update(msg tea.Msg) (CreateTopicModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, ReturnToListView

		case "tab", "shift+tab", "up", "down":
			s := msg.String()

			// Navigate through inputs
			if s == "up" || s == "shift+tab" {
				m.focusIndex--
			} else {
				m.focusIndex++
			}

			if m.focusIndex > len(m.inputs) {
				m.focusIndex = 0
			} else if m.focusIndex < 0 {
				m.focusIndex = len(m.inputs)
			}

			cmds := make([]tea.Cmd, len(m.inputs))
			for i := 0; i <= len(m.inputs)-1; i++ {
				if i == m.focusIndex {
					cmds[i] = m.inputs[i].Focus()
					m.inputs[i].PromptStyle = focusedStyle
					m.inputs[i].TextStyle = focusedStyle
				} else {
					m.inputs[i].Blur()
					m.inputs[i].PromptStyle = noStyle
					m.inputs[i].TextStyle = noStyle
				}
			}

			return m, tea.Batch(cmds...)

		case "enter":
			if m.focusIndex == len(m.inputs) {
				// Create button is focused
				return m.createTopic()
			}
			// Move to next input
			m.focusIndex++
			if m.focusIndex > len(m.inputs) {
				m.focusIndex = 0
			}
			return m.updateFocus()
		}

	case topicCreatedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.successMsg = ""
		} else {
			m.err = nil
			m.successMsg = fmt.Sprintf("✓ Topic '%s' created successfully!", msg.name)
			// Clear inputs
			for i := range m.inputs {
				m.inputs[i].SetValue("")
			}
			m.focusIndex = 0
			return m.updateFocus()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	// Update text inputs
	cmd := m.updateInputs(msg)
	return m, cmd
}

func (m *CreateTopicModel) createTopic() (CreateTopicModel, tea.Cmd) {
	name := m.inputs[topicNameIdx].Value()
	if name == "" {
		m.err = fmt.Errorf("topic name is required")
		return *m, nil
	}

	// Parse partitions
	partitionsStr := m.inputs[partitionsIdx].Value()
	partitions := int32(1)
	if partitionsStr != "" {
		if p, err := strconv.ParseInt(partitionsStr, 10, 32); err == nil && p > 0 {
			partitions = int32(p)
		} else {
			m.err = fmt.Errorf("invalid partitions value")
			return *m, nil
		}
	}

	// Parse replication factor
	replicationStr := m.inputs[replicationIdx].Value()
	replication := int16(1)
	if replicationStr != "" {
		if r, err := strconv.ParseInt(replicationStr, 10, 16); err == nil && r > 0 {
			replication = int16(r)
		} else {
			m.err = fmt.Errorf("invalid replication factor")
			return *m, nil
		}
	}

	return *m, createTopic(m.client, name, partitions, replication)
}

func (m *CreateTopicModel) updateFocus() (CreateTopicModel, tea.Cmd) {
	cmds := make([]tea.Cmd, len(m.inputs))
	for i := 0; i <= len(m.inputs)-1; i++ {
		if i == m.focusIndex {
			cmds[i] = m.inputs[i].Focus()
			m.inputs[i].PromptStyle = focusedStyle
			m.inputs[i].TextStyle = focusedStyle
		} else {
			m.inputs[i].Blur()
			m.inputs[i].PromptStyle = noStyle
			m.inputs[i].TextStyle = noStyle
		}
	}
	return *m, tea.Batch(cmds...)
}

func (m *CreateTopicModel) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))

	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}

	return tea.Batch(cmds...)
}

func (m CreateTopicModel) View() string {
	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Padding(0, 1)

	sb.WriteString(titleStyle.Render("🎯 Create New Topic"))
	sb.WriteString("\n\n")

	// Input fields
	for i := range m.inputs {
		sb.WriteString(m.inputs[i].View())
		if i < len(m.inputs)-1 {
			sb.WriteString("\n\n")
		}
	}

	// Create button
	sb.WriteString("\n\n")
	button := &blurredButton
	if m.focusIndex == len(m.inputs) {
		button = &focusedButton
	}
	sb.WriteString(*button)
	sb.WriteString("\n\n")

	// Error or success message
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

	// Help
	sb.WriteString("\n")
	sb.WriteString(helpStyle.Render("Tab: Navigate fields • Enter: Next/Create • Esc: Cancel"))

	return sb.String()
}
