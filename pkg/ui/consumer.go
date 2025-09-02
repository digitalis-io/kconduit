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
	topicInfo    *kafka.TopicInfo
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
	consuming    bool
	totalBytes   int64
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

	return ConsumerModel{
		topic:       topic,
		topicInfo:   topicInfo,
		client:      client,
		ctx:         ctx,
		cancel:      cancel,
		messageChan: messageChan,
		messages:    make([]kafka.Message, 0),
		viewport:    vp,
		ready:       false,
		consuming:   true,
		totalBytes:  0,
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
			m.consuming = false
			return m, ReturnToListView
		case "c":
			// Clear messages
			m.messages = []kafka.Message{}
			m.totalBytes = 0
			m.updateContent()
			m.viewport.GotoTop()
		case "p":
			// Pause/Resume consumption
			m.consuming = !m.consuming
		}

	case messageReceivedMsg:
		if msg.message.Topic != "" && m.consuming {
			m.messages = append(m.messages, msg.message)
			// Calculate message size
			m.totalBytes += int64(len(msg.message.Key) + len(msg.message.Value))
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
		headerHeight := 14  // Increased for the new table
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

	// Styles
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86"))
	
	metaStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))
	
	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("220"))
	
	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	// Message header with separator
	sb.WriteString(headerStyle.Render(fmt.Sprintf("╭─── Message #%d ", num)))
	sb.WriteString(strings.Repeat("─", 40))
	sb.WriteString("\n")
	
	// Metadata line
	sb.WriteString("│ ")
	sb.WriteString(metaStyle.Render(fmt.Sprintf("📍 Partition: %d | Offset: %d | ⏰ %s",
		msg.Partition,
		msg.Offset,
		msg.Timestamp.Format("15:04:05.000"),
	)))
	sb.WriteString("\n")

	// Key (if present)
	if msg.Key != "" {
		sb.WriteString("│ ")
		sb.WriteString(keyStyle.Render(fmt.Sprintf("🔑 Key: %s", msg.Key)))
		sb.WriteString("\n")
	}

	// Headers (if present)
	if len(msg.Headers) > 0 {
		sb.WriteString("│ ")
		sb.WriteString(headerStyle.Render("📎 Headers:"))
		sb.WriteString("\n")
		for k, v := range msg.Headers {
			sb.WriteString("│   ")
			sb.WriteString(metaStyle.Render(fmt.Sprintf("%s: %s", k, v)))
			sb.WriteString("\n")
		}
	}

	// Value
	sb.WriteString("│ ")
	sb.WriteString(headerStyle.Render("📄 Value:"))
	sb.WriteString("\n")
	if msg.Value == "" {
		sb.WriteString("│   ")
		sb.WriteString(metaStyle.Render("(empty)"))
		sb.WriteString("\n")
	} else {
		lines := strings.Split(msg.Value, "\n")
		for _, line := range lines {
			sb.WriteString("│   ")
			sb.WriteString(valueStyle.Render(line))
			sb.WriteString("\n")
		}
	}
	
	sb.WriteString("╰")
	sb.WriteString(strings.Repeat("─", 50))
	sb.WriteString("\n\n")
	
	return sb.String()
}

func (m ConsumerModel) View() string {
	var sb strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Padding(0, 1)

	sb.WriteString(headerStyle.Render("📨 Kafka Consumer"))
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
	
	tableContent.WriteString(labelStyle.Render("Messages Received:"))
	tableContent.WriteString(valueStyle.Render(fmt.Sprintf(" %d", len(m.messages))) + "\n")
	
	tableContent.WriteString(labelStyle.Render("Total Bytes:      "))
	tableContent.WriteString(valueStyle.Render(fmt.Sprintf("%s", formatBytes(m.totalBytes))) + "\n")
	
	tableContent.WriteString(labelStyle.Render("Status:           "))
	if m.err != nil {
		tableContent.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("❌ Error"))
	} else if !m.consuming {
		tableContent.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render("⏸️  Paused"))
	} else if len(m.messages) == 0 {
		tableContent.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render("⏳ Waiting"))
	} else {
		tableContent.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render("✅ Consuming"))
	}

	sb.WriteString(tableStyle.Render(tableContent.String()))
	sb.WriteString("\n\n")

	// Messages Header
	messagesHeaderStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86"))

	sb.WriteString(messagesHeaderStyle.Render("📦 Message Stream"))
	sb.WriteString("\n")

	// Error message
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)
		sb.WriteString(errorStyle.Render(fmt.Sprintf("\n❌ Error: %v\n", m.err)))
	}

	// Viewport with messages
	viewportStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240"))
	
	sb.WriteString(viewportStyle.Render(m.viewport.View()))
	sb.WriteString("\n")

	// Footer with help text
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true)
	
	footer := "↑/↓: Scroll | p: Pause/Resume | c: Clear | q/Esc: Back"
	if m.viewport.AtBottom() {
		footer += " (auto-scrolling)"
	}
	sb.WriteString(helpStyle.Render(footer))

	return sb.String()
}

// Helper function to format bytes
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

