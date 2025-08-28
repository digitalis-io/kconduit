package ui

import (
	"fmt"
	"strings"

	"github.com/axonops/kconduit/pkg/kafka"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type DeleteTopicModel struct {
	client           *kafka.Client
	topicToDelete    string
	confirmInput     textinput.Model
	focusedButton    int // 0: input field, 1: yes button, 2: no button
	err              error
	width            int
	height           int
}

func NewDeleteTopicModel(client *kafka.Client, topicName string) DeleteTopicModel {
	ti := textinput.New()
	ti.Placeholder = "Type topic name to confirm"
	ti.Focus()
	ti.CharLimit = 255
	ti.Width = 40

	return DeleteTopicModel{
		client:        client,
		topicToDelete: topicName,
		confirmInput:  ti,
		focusedButton: 0,
	}
}

type topicDeletedMsg struct {
	topicName string
	err       error
}

func deleteTopic(client *kafka.Client, topicName string) tea.Cmd {
	return func() tea.Msg {
		err := client.DeleteTopic(topicName)
		return topicDeletedMsg{topicName: topicName, err: err}
	}
}

func (m DeleteTopicModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m DeleteTopicModel) Update(msg tea.Msg) (DeleteTopicModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, ReturnToListView

		case "tab", "shift+tab":
			// Navigate between input, yes, and no buttons
			if msg.String() == "tab" {
				m.focusedButton++
				if m.focusedButton > 2 {
					m.focusedButton = 0
				}
			} else {
				m.focusedButton--
				if m.focusedButton < 0 {
					m.focusedButton = 2
				}
			}

			// Update focus on text input
			if m.focusedButton == 0 {
				cmd = m.confirmInput.Focus()
			} else {
				m.confirmInput.Blur()
			}
			return m, cmd

		case "enter":
			switch m.focusedButton {
			case 0: // Input field - move to Yes button
				m.focusedButton = 1
				m.confirmInput.Blur()
				return m, nil
			case 1: // Yes button - confirm deletion
				// Check if the entered name matches
				if m.confirmInput.Value() == m.topicToDelete {
					return m, deleteTopic(m.client, m.topicToDelete)
				} else {
					m.err = fmt.Errorf("topic name does not match")
					return m, nil
				}
			case 2: // No button - cancel
				return m, ReturnToListView
			}

		default:
			// Only update text input if it's focused
			if m.focusedButton == 0 {
				m.confirmInput, cmd = m.confirmInput.Update(msg)
				// Clear error when user starts typing again
				if m.err != nil && m.confirmInput.Value() != "" {
					m.err = nil
				}
			}
		}

	case topicDeletedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		// Success - return to list view
		return m, ReturnToListView

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, cmd
}

func (m DeleteTopicModel) View() string {
	var s strings.Builder

	// Title with warning style
	warningStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196")).
		Background(lipgloss.Color("52")).
		Padding(0, 1)

	s.WriteString(warningStyle.Render("⚠️  DELETE TOPIC"))
	s.WriteString("\n\n")

	// Warning message
	dangerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true)
	
	s.WriteString(dangerStyle.Render("WARNING: This action cannot be undone!"))
	s.WriteString("\n\n")

	// Topic to delete
	topicStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("229")).
		Bold(true)
	
	s.WriteString(fmt.Sprintf("You are about to delete topic: %s\n\n", 
		topicStyle.Render(m.topicToDelete)))

	// Confirmation prompt
	s.WriteString("Type the topic name to confirm:\n")
	s.WriteString(m.confirmInput.View())
	s.WriteString("\n\n")

	// Buttons
	buttonStyle := lipgloss.NewStyle().
		Padding(0, 2).
		MarginRight(2)

	yesStyle := buttonStyle.Copy()
	noStyle := buttonStyle.Copy()

	// Style buttons based on focus and validation
	validInput := m.confirmInput.Value() == m.topicToDelete

	if m.focusedButton == 1 {
		if validInput {
			yesStyle = yesStyle.
				Foreground(lipgloss.Color("231")).
				Background(lipgloss.Color("196")).
				Bold(true)
		} else {
			yesStyle = yesStyle.
				Foreground(lipgloss.Color("240")).
				Bold(false)
		}
	} else {
		if validInput {
			yesStyle = yesStyle.
				Foreground(lipgloss.Color("196")).
				Bold(false)
		} else {
			yesStyle = yesStyle.
				Foreground(lipgloss.Color("240")).
				Bold(false)
		}
	}

	if m.focusedButton == 2 {
		noStyle = noStyle.
			Foreground(lipgloss.Color("231")).
			Background(lipgloss.Color("28")).
			Bold(true)
	} else {
		noStyle = noStyle.
			Foreground(lipgloss.Color("28")).
			Bold(false)
	}

	// Only enable Yes button if input matches
	if validInput {
		s.WriteString(yesStyle.Render("[ Delete ]"))
	} else {
		disabledStyle := buttonStyle.Copy().
			Foreground(lipgloss.Color("240"))
		s.WriteString(disabledStyle.Render("[ Delete ]"))
	}
	
	s.WriteString(noStyle.Render("[ Cancel ]"))
	s.WriteString("\n\n")

	// Error message
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)
		s.WriteString(errorStyle.Render(fmt.Sprintf("❌ Error: %v\n", m.err)))
	}

	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))
	
	if !validInput && m.confirmInput.Value() != "" {
		mismatchStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("208"))
		s.WriteString(mismatchStyle.Render("⚠️  Topic name doesn't match\n\n"))
	}
	
	s.WriteString(helpStyle.Render("Tab: Navigate • Enter: Select • Esc: Cancel"))

	return s.String()
}