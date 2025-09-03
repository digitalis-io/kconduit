package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/IBM/sarama"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/digitalis-io/kconduit/pkg/kafka"
)

type ConsumerMode int

const (
	ModeNormal ConsumerMode = iota
	ModeOffsetDialog
	ModeSearch
)

type OffsetOption int

const (
	OffsetOldest OffsetOption = iota
	OffsetNewest
	OffsetSpecific
)

type ConsumerModel struct {
	topic        string
	topicInfo    *kafka.TopicInfo
	client       *kafka.Client
	messageTable table.Model
	messages     []kafka.Message
	tableRows    []table.Row
	ctx          context.Context
	cancel       context.CancelFunc
	messageChan  chan kafka.Message
	err          error
	width        int
	height       int
	ready        bool
	consuming    bool
	totalBytes   int64
	// New fields for offset control
	mode           ConsumerMode
	offsetOption   OffsetOption
	offsetInput    textinput.Model
	startOffset    int64
	// New fields for search
	searchInput     textinput.Model
	searchTerm      string
	searchResults   []int
	currentMatch    int
	filteredIndices []int
	showFiltered    bool
}

func NewConsumerModel(topic string, client *kafka.Client) ConsumerModel {
	ctx, cancel := context.WithCancel(context.Background())
	messageChan := make(chan kafka.Message, 100)

	// Initialize message table
	columns := []table.Column{
		{Title: "#", Width: 6},
		{Title: "Timestamp", Width: 20},
		{Title: "Part", Width: 5},
		{Title: "Offset", Width: 10},
		{Title: "Key", Width: 20},
		{Title: "Value", Width: 50},
		{Title: "Size", Width: 8},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(20),
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

	// Initialize text inputs
	offsetInput := textinput.New()
	offsetInput.Placeholder = "Enter offset number (e.g., 100)"
	offsetInput.CharLimit = 20

	searchInput := textinput.New()
	searchInput.Placeholder = "Search messages..."
	searchInput.CharLimit = 100

	return ConsumerModel{
		topic:           topic,
		topicInfo:       topicInfo,
		client:          client,
		ctx:             ctx,
		cancel:          cancel,
		messageChan:     messageChan,
		messages:        make([]kafka.Message, 0),
		tableRows:       []table.Row{},
		messageTable:    t,
		ready:           false,
		consuming:       false, // Start with offset dialog
		totalBytes:      0,
		mode:            ModeOffsetDialog,
		offsetOption:    OffsetNewest,
		offsetInput:     offsetInput,
		searchInput:     searchInput,
		searchResults:   []int{},
		filteredIndices: []int{},
		startOffset:     sarama.OffsetNewest,
	}
}

type messageReceivedMsg struct {
	message kafka.Message
}

type consumerErrorMsg struct {
	err error
}

func consumeMessages(ctx context.Context, client *kafka.Client, topic string, messageChan chan kafka.Message, offset int64) tea.Cmd {
	return func() tea.Msg {
		go func() {
			err := client.ConsumeMessagesWithOffset(ctx, topic, messageChan, offset)
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
	// Start with offset dialog, don't consume yet
	return textinput.Blink
}

func (m ConsumerModel) Update(msg tea.Msg) (ConsumerModel, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle offset dialog mode
	if m.mode == ModeOffsetDialog {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				m.cancel()
				return m, ReturnToListView
			case "tab", "down", "j":
				// Move to next offset option
				m.offsetOption = OffsetOption((int(m.offsetOption) + 1) % 3)
				if m.offsetOption == OffsetSpecific {
					m.offsetInput.Focus()
					cmds = append(cmds, textinput.Blink)
				} else {
					m.offsetInput.Blur()
				}
			case "shift+tab", "up", "k":
				// Move to previous offset option
				m.offsetOption = OffsetOption((int(m.offsetOption) + 2) % 3)
				if m.offsetOption == OffsetSpecific {
					m.offsetInput.Focus()
					cmds = append(cmds, textinput.Blink)
				} else {
					m.offsetInput.Blur()
				}
			case "enter":
				// Start consuming with selected offset
				switch m.offsetOption {
				case OffsetOldest:
					m.startOffset = sarama.OffsetOldest
				case OffsetNewest:
					m.startOffset = sarama.OffsetNewest
				case OffsetSpecific:
					if offset, err := strconv.ParseInt(m.offsetInput.Value(), 10, 64); err == nil {
						m.startOffset = offset
					} else {
						m.err = fmt.Errorf("invalid offset number: %s", m.offsetInput.Value())
						return m, nil
					}
				}
				m.mode = ModeNormal
				m.consuming = true
				cmds = append(cmds, consumeMessages(m.ctx, m.client, m.topic, m.messageChan, m.startOffset))
				cmds = append(cmds, waitForMessage(m.messageChan))
			}
		}
		// Update text input if focused
		if m.offsetOption == OffsetSpecific {
			var cmd tea.Cmd
			m.offsetInput, cmd = m.offsetInput.Update(msg)
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	}

	// Handle search mode
	if m.mode == ModeSearch {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				m.mode = ModeNormal
				m.searchInput.Blur()
				m.searchInput.SetValue("")
				m.showFiltered = false
				m.updateTable()
			case "enter":
				m.searchTerm = m.searchInput.Value()
				m.performSearch()
				m.mode = ModeNormal
				m.searchInput.Blur()
				if len(m.searchResults) > 0 {
					m.currentMatch = 0
					m.scrollToMessage(m.searchResults[0])
				}
			}
		}
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}

	// Normal mode
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
			m.searchResults = []int{}
			m.filteredIndices = []int{}
			m.updateTable()
		case "p":
			// Pause/Resume consumption
			m.consuming = !m.consuming
		case "/":
			// Enter search mode
			m.mode = ModeSearch
			m.searchInput.Focus()
			cmds = append(cmds, textinput.Blink)
		case "n":
			// Next search result
			if len(m.searchResults) > 0 {
				m.currentMatch = (m.currentMatch + 1) % len(m.searchResults)
				m.scrollToMessage(m.searchResults[m.currentMatch])
			}
		case "N":
			// Previous search result
			if len(m.searchResults) > 0 {
				m.currentMatch = (m.currentMatch - 1 + len(m.searchResults)) % len(m.searchResults)
				m.scrollToMessage(m.searchResults[m.currentMatch])
			}
		case "f":
			// Toggle filtered view
			if len(m.searchResults) > 0 {
				m.showFiltered = !m.showFiltered
				m.updateTable()
			}
		}

	case messageReceivedMsg:
		if msg.message.Topic != "" && m.consuming {
			m.messages = append(m.messages, msg.message)
			// Calculate message size
			m.totalBytes += int64(len(msg.message.Key) + len(msg.message.Value))
			// Check if new message matches search
			if m.searchTerm != "" {
				if m.messageMatches(msg.message, m.searchTerm) {
					m.searchResults = append(m.searchResults, len(m.messages)-1)
				}
			}
			m.updateTable()
			if !m.showFiltered && len(m.messages) > 0 {
				// Auto-scroll to bottom (select last row)
				m.messageTable.SetCursor(len(m.tableRows) - 1)
			}
		}
		// Continue waiting for more messages
		cmds = append(cmds, waitForMessage(m.messageChan))

	case consumerErrorMsg:
		m.err = msg.err

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		headerHeight := 12 // Header + search bar
		footerHeight := 3

		tableHeight := msg.Height - headerHeight - footerHeight
		if tableHeight > 0 {
			m.messageTable.SetHeight(tableHeight)
		}

		// Adjust column widths based on screen width
		m.adjustColumnWidths(msg.Width)
		m.ready = true
		m.updateTable()
	}

	// Update table
	var cmd tea.Cmd
	m.messageTable, cmd = m.messageTable.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *ConsumerModel) performSearch() {
	m.searchResults = []int{}
	m.filteredIndices = []int{}

	if m.searchTerm == "" {
		return
	}

	for i, msg := range m.messages {
		if m.messageMatches(msg, m.searchTerm) {
			m.searchResults = append(m.searchResults, i)
			m.filteredIndices = append(m.filteredIndices, i)
		}
	}
}

func (m *ConsumerModel) messageMatches(msg kafka.Message, searchTerm string) bool {
	searchLower := strings.ToLower(searchTerm)
	return strings.Contains(strings.ToLower(msg.Key), searchLower) ||
		strings.Contains(strings.ToLower(msg.Value), searchLower) ||
		strings.Contains(strings.ToLower(msg.Topic), searchLower)
}

func (m *ConsumerModel) scrollToMessage(index int) {
	if index < 0 || index >= len(m.messages) {
		return
	}

	// For table, just set the cursor to the row
	if m.showFiltered {
		// Find the filtered row index
		for i, fidx := range m.filteredIndices {
			if fidx == index {
				m.messageTable.SetCursor(i)
				break
			}
		}
	} else {
		m.messageTable.SetCursor(index)
	}
}

func (m *ConsumerModel) adjustColumnWidths(totalWidth int) {
	// Dynamically adjust column widths based on available space
	if totalWidth < 80 {
		totalWidth = 80
	}

	// Calculate proportional widths
	numCol := 6
	timestampCol := 19
	partCol := 5
	offsetCol := 10
	sizeCol := 8

	// Remaining space for key and value
	remainingWidth := totalWidth - numCol - timestampCol - partCol - offsetCol - sizeCol - 10 // padding

	keyCol := remainingWidth / 4       // 25% for key
	valueCol := remainingWidth * 3 / 4 // 75% for value

	if keyCol < 10 {
		keyCol = 10
	}
	if valueCol < 20 {
		valueCol = 20
	}

	columns := []table.Column{
		{Title: "#", Width: numCol},
		{Title: "Timestamp", Width: timestampCol},
		{Title: "Part", Width: partCol},
		{Title: "Offset", Width: offsetCol},
		{Title: "Key", Width: keyCol},
		{Title: "Value", Width: valueCol},
		{Title: "Size", Width: sizeCol},
	}

	m.messageTable.SetColumns(columns)
}

func (m *ConsumerModel) updateTable() {
	m.tableRows = []table.Row{}
	indices := []int{}

	if m.showFiltered && len(m.filteredIndices) > 0 {
		indices = append(indices, m.filteredIndices...)
	} else {
		for i := range m.messages {
			indices = append(indices, i)
		}
	}

	// Build table rows
	for _, idx := range indices {
		if idx >= len(m.messages) {
			continue
		}
		msg := m.messages[idx]

		// Check if this is a search result for highlighting
		isSearchResult := false
		for _, sIdx := range m.searchResults {
			if sIdx == idx {
				isSearchResult = true
				break
			}
		}

		row := m.formatMessageRow(msg, idx+1, isSearchResult)
		m.tableRows = append(m.tableRows, row)
	}

	m.messageTable.SetRows(m.tableRows)
}

func (m *ConsumerModel) formatMessageRow(msg kafka.Message, num int, isSearchResult bool) table.Row {
	// Format timestamp
	timestamp := msg.Timestamp.Format("2006-01-02 15:04:05")

	// Truncate and clean value for table display
	value := strings.ReplaceAll(msg.Value, "\n", " ")
	value = strings.ReplaceAll(value, "\t", " ")

	// Calculate message size
	msgSize := len(msg.Key) + len(msg.Value)
	sizeStr := formatBytes(int64(msgSize))

	return table.Row{
		fmt.Sprintf("%d", num),
		timestamp,
		fmt.Sprintf("%d", msg.Partition),
		fmt.Sprintf("%d", msg.Offset),
		msg.Key,
		value,
		sizeStr,
	}
}


func (m ConsumerModel) viewOffsetDialog() string {
	var sb strings.Builder

	// Calculate dialog width based on terminal width
	boxWidth := 90
	if m.width > 0 && boxWidth > m.width-4 {
		boxWidth = m.width - 4
	}
	// Dialog style
	dialogStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("86")).
		Padding(2, 4).
		Width(boxWidth)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("229")).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86"))

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("46")).
		Bold(true)

	sb.WriteString(titleStyle.Render("📍 Select Consumer Start Position"))
	sb.WriteString("\n\n")

	sb.WriteString("Choose where to start consuming messages from:\n\n")

	// Offset options
	options := []struct {
		option OffsetOption
		label  string
		desc   string
	}{
		{OffsetOldest, "Oldest", "Start from the beginning of the topic"},
		{OffsetNewest, "Latest", "Start from new messages only"},
		{OffsetSpecific, "Specific Offset", "Start from a specific offset number"},
	}

	for _, opt := range options {
		prefix := "  "
		style := labelStyle
		if m.offsetOption == opt.option {
			prefix = "▶ "
			style = selectedStyle
		}
		sb.WriteString(style.Render(fmt.Sprintf("%s%s", prefix, opt.label)))
		sb.WriteString(fmt.Sprintf(" - %s\n", opt.desc))

		// Show input field if this option is selected
		if m.offsetOption == opt.option {
			if opt.option == OffsetSpecific {
				sb.WriteString("    ")
				sb.WriteString(m.offsetInput.View())
				sb.WriteString("\n")
			}
		}
	}

	sb.WriteString("\n")

	// Error display
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))
		sb.WriteString(errorStyle.Render(fmt.Sprintf("❌ %v\n\n", m.err)))
	}

	// Help text with examples
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true)

	helpText := "↑/↓ or Tab: Navigate | Enter: Start | Esc: Cancel"
	sb.WriteString(helpStyle.Render(helpText))

	// Center the dialog
	content := dialogStyle.Render(sb.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m ConsumerModel) View() string {
	if m.mode == ModeOffsetDialog {
		return m.viewOffsetDialog()
	}

	var sb strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Padding(0, 1)

	sb.WriteString(headerStyle.Render("📨 Kafka Consumer"))
	sb.WriteString("\n\n")

	// Show search bar if in search mode
	if m.mode == ModeSearch {
		searchStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("220"))
		sb.WriteString(searchStyle.Render("🔍 Search: "))
		sb.WriteString(m.searchInput.View())
		sb.WriteString("\n\n")
	}

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
	tableContent.WriteString(valueStyle.Render(formatBytes(m.totalBytes)) + "\n")

	tableContent.WriteString(labelStyle.Render("Start Offset:     "))
	offsetText := "Latest"
	if m.startOffset == sarama.OffsetOldest {
		offsetText = "Oldest"
	} else if m.startOffset >= 0 {
		offsetText = fmt.Sprintf("%d", m.startOffset)
	}
	tableContent.WriteString(valueStyle.Render(offsetText) + "\n")

	if m.searchTerm != "" {
		tableContent.WriteString(labelStyle.Render("Search Results:   "))
		tableContent.WriteString(valueStyle.Render(fmt.Sprintf("%d matches", len(m.searchResults))) + "\n")
	}

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
	sb.WriteString("\n")

	// Error message
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)
		sb.WriteString(errorStyle.Render(fmt.Sprintf("❌ Error: %v\n", m.err)))
	}

	// Message table
	if len(m.messages) == 0 && !m.consuming {
		// Show a placeholder when not consuming
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true).
			Padding(2, 0)
		sb.WriteString(emptyStyle.Render("No messages to display. Start consuming to see messages."))
	} else {
		// Render the message table
		sb.WriteString(m.messageTable.View())
	}
	sb.WriteString("\n")

	// Footer with help text
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true)

	footer := "↑/↓: Navigate | /: Search | n/N: Next/Prev | f: Filter | p: Pause | c: Clear | q: Back"
	if m.searchTerm != "" && len(m.searchResults) > 0 {
		footer = fmt.Sprintf("[Match %d/%d] ", m.currentMatch+1, len(m.searchResults)) + footer
	}
	if m.showFiltered {
		footer = "[FILTERED] " + footer
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
