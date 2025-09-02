package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/digitalis-io/kconduit/pkg/kafka"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ConsumerModel struct {
	topic        string
	client       *kafka.Client
	viewport     viewport.Model
	messages     []kafka.Message
	content      string
	ctx          context.Context
	cancel       context.CancelFunc
	messageChan  chan kafka.Message
	err          error
	width        int
	height       int
	ready        bool
}

var (
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Padding(0, 1)
)

func NewConsumerModel(topic string, client *kafka.Client) ConsumerModel {
	ctx, cancel := context.WithCancel(context.Background())
	messageChan := make(chan kafka.Message, 100)

	vp := viewport.New(80, 20) // Initial size, will be resized on WindowSizeMsg

	return ConsumerModel{
		topic:       topic,
		client:      client,
		ctx:         ctx,
		cancel:      cancel,
		messageChan: messageChan,
		messages:    make([]kafka.Message, 0),
		viewport:    vp,
		ready:       false,
	}
}

type messageReceivedMsg struct {
	message kafka.Message
}

type consumerErrorMsg struct {
	err error
}

func consumeMessages(ctx context.Context, client *kafka.Client, topic string, messageChan chan kafka.Message) tea.Cmd {
	return func() tea.Msg {
		go func() {
			err := client.ConsumeMessages(ctx, topic, messageChan)
			if err != nil && ctx.Err() == nil {
				// Only report error if context wasn't cancelled
				messageChan <- kafka.Message{} // Send empty message to signal error
			}
		}()
		return nil
	}
}

func waitForMessage(messageChan chan kafka.Message) tea.Cmd {
	return func() tea.Msg {
		msg := <-messageChan
		return messageReceivedMsg{message: msg}
	}
}

func (m ConsumerModel) Init() tea.Cmd {
	return tea.Batch(
		consumeMessages(m.ctx, m.client, m.topic, m.messageChan),
		waitForMessage(m.messageChan),
	)
}

func (m ConsumerModel) Update(msg tea.Msg) (ConsumerModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			m.cancel()
			return m, ReturnToListView
		case "c":
			// Clear messages
			m.messages = []kafka.Message{}
			m.updateContent()
			m.viewport.GotoTop()
		}

	case messageReceivedMsg:
		if msg.message.Topic != "" {
			m.messages = append(m.messages, msg.message)
			m.updateContent()
			m.viewport.GotoBottom()
		}
		// Continue waiting for more messages
		cmds = append(cmds, waitForMessage(m.messageChan))

	case consumerErrorMsg:
		m.err = msg.err

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		headerHeight := 4
		footerHeight := 3

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-headerHeight-footerHeight)
			m.viewport.YPosition = headerHeight
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - headerHeight - footerHeight
		}
		m.updateContent()
	}

	// Update viewport
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *ConsumerModel) updateContent() {
	var sb strings.Builder

	if len(m.messages) == 0 {
		sb.WriteString("\n  Waiting for messages...\n")
	} else {
		sb.WriteString(fmt.Sprintf("\n  Found %d messages:\n\n", len(m.messages)))
		for i, msg := range m.messages {
			// Format message
			msgContent := m.formatMessage(msg, i+1)
			sb.WriteString(msgContent)
		}
	}

	m.content = sb.String()
	m.viewport.SetContent(m.content)
}

func (m *ConsumerModel) formatMessage(msg kafka.Message, num int) string {
	var sb strings.Builder

	// Message header
	sb.WriteString(fmt.Sprintf("━━━━━ Message #%d ━━━━━\n", num))
	
	// Metadata
	sb.WriteString(fmt.Sprintf("Partition: %d | Offset: %d | Time: %s\n",
		msg.Partition,
		msg.Offset,
		msg.Timestamp.Format("15:04:05"),
	))

	// Key (if present)
	if msg.Key != "" {
		sb.WriteString(fmt.Sprintf("Key: %s\n", msg.Key))
	}

	// Headers (if present)
	if len(msg.Headers) > 0 {
		sb.WriteString("Headers:\n")
		for k, v := range msg.Headers {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}

	// Value
	sb.WriteString("Value:\n")
	if msg.Value == "" {
		sb.WriteString("  (empty)\n")
	} else {
		lines := strings.Split(msg.Value, "\n")
		for _, line := range lines {
			sb.WriteString("  ")
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}
	
	sb.WriteString("\n")
	return sb.String()
}

func (m ConsumerModel) View() string {
	var sb strings.Builder

	// Title
	title := fmt.Sprintf("📨 Consumer Mode - Topic: %s | Messages: %d", m.topic, len(m.messages))
	sb.WriteString(titleStyle.Render(title))
	sb.WriteString("\n\n")

	// Error message
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
		sb.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v\n", m.err)))
	}

	// Viewport
	sb.WriteString(m.viewport.View())
	sb.WriteString("\n")

	// Footer
	footerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	footer := "↑/↓: Scroll | c: Clear | q/Esc: Back to topics"
	if m.viewport.AtBottom() {
		footer += " | (at bottom)"
	}
	sb.WriteString(footerStyle.Render(footer))

	return sb.String()
}

