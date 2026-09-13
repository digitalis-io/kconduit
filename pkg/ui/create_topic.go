package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/digitalis-io/kconduit/pkg/kafka"
)

type CreateTopicModel struct {
	client     *kafka.Client
	inputs     []textinput.Model
	focusIndex int
	err        error
	successMsg string
	width      int
	height     int

	// brokerCount caps the replication factor. It is 0 until the broker list
	// arrives, which leaves the ceiling unenforced rather than guessed at.
	brokerCount int
}

const (
	topicNameIdx = iota
	partitionsIdx
	replicationIdx
)

// The two places focus can be beyond the last input.
const (
	createButtonIdx = 3
	cancelButtonIdx = 4
	lastFocusIdx    = cancelButtonIdx
)

// labelWidth is the width of the label column, so every input starts at the
// same column and the form reads as a table rather than a stack of prompts.
const labelWidth = 14

// NewCreateTopicModel builds the form. width and height come from the view that
// opened it: a sub-view is created between resizes, so it never sees a
// WindowSizeMsg of its own until the terminal happens to change, and without
// them the frame would be laid out for a zero-width screen.
func NewCreateTopicModel(client *kafka.Client, width, height int) CreateTopicModel {
	m := CreateTopicModel{
		client: client,
		inputs: make([]textinput.Model, 3),
		width:  width,
		height: height,
	}

	for i := range m.inputs {
		t := textinput.New()
		t.Cursor.Style = lipgloss.NewStyle().Foreground(theme.Primary)
		// The label lives in its own column, so the input carries no prompt.
		t.Prompt = ""

		switch i {
		case topicNameIdx:
			t.Placeholder = "orders.v2"
			t.CharLimit = maxTopicNameLength
			t.Width = 32
			t.Focus()
		case partitionsIdx:
			t.Placeholder = "1"
			t.CharLimit = 5
			t.Width = 8
		case replicationIdx:
			t.Placeholder = "1"
			t.CharLimit = 3
			t.Width = 8
		}

		m.inputs[i] = t
	}

	m.styleInputs()
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
	// The broker count is what makes the replication field checkable, so it is
	// fetched as the form opens rather than waiting for a failed create to
	// report the same thing back from the broker.
	return tea.Batch(textinput.Blink, fetchBrokers(m.client))
}

func (m CreateTopicModel) Update(msg tea.Msg) (CreateTopicModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, ReturnToListView

		case "tab", "down":
			return m.moveFocus(1)

		case "shift+tab", "up":
			return m.moveFocus(-1)

		case "enter":
			switch m.focusIndex {
			case cancelButtonIdx:
				return m, ReturnToListView
			case createButtonIdx:
				return m.createTopic()
			default:
				// Enter on a field advances, so the whole form can be filled in
				// without reaching for Tab.
				return m.moveFocus(1)
			}
		}

	case brokersMsg:
		if msg.err == nil {
			m.brokerCount = len(msg.brokers)
		}
		return m, nil

	case topicCreatedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.successMsg = ""
			return m, nil
		}

		m.err = nil
		m.successMsg = fmt.Sprintf("Created %s", msg.name)
		// The form stays open and clears, because creating several topics in a
		// row is the common case.
		for i := range m.inputs {
			m.inputs[i].SetValue("")
		}
		m.focusIndex = topicNameIdx
		return m.applyFocus()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	cmd := m.updateInputs(msg)
	return m, cmd
}

// moveFocus steps focus by delta, wrapping past the buttons back to the first
// field.
func (m CreateTopicModel) moveFocus(delta int) (CreateTopicModel, tea.Cmd) {
	places := lastFocusIdx + 1
	m.focusIndex = ((m.focusIndex+delta)%places + places) % places
	return m.applyFocus()
}

func (m *CreateTopicModel) createTopic() (CreateTopicModel, tea.Cmd) {
	// Every field is validated as it is typed, so this only has to catch Create
	// being reached with the form incomplete.
	if !m.valid() {
		m.err = fmt.Errorf("fill in the highlighted fields before creating")
		return *m, nil
	}

	m.err = nil
	m.successMsg = ""

	return *m, createTopic(m.client,
		strings.TrimSpace(m.inputs[topicNameIdx].Value()),
		int32(m.countOrDefault(partitionsIdx)),
		int16(m.countOrDefault(replicationIdx)),
	)
}

// applyFocus moves the terminal cursor to the focused field and restyles the
// inputs to match.
func (m *CreateTopicModel) applyFocus() (CreateTopicModel, tea.Cmd) {
	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		if i == m.focusIndex {
			cmds[i] = m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
	m.styleInputs()
	return *m, tea.Batch(cmds...)
}

// styleInputs colours each input by its state: the focused one is highlighted,
// and one holding something invalid is coloured as an error whether it has
// focus or not.
func (m *CreateTopicModel) styleInputs() {
	for i := range m.inputs {
		switch {
		case m.fieldError(i) != "":
			m.inputs[i].TextStyle = lipgloss.NewStyle().Foreground(theme.Error)
		case i == m.focusIndex:
			m.inputs[i].TextStyle = lipgloss.NewStyle().Foreground(theme.Text).Bold(true)
		default:
			m.inputs[i].TextStyle = lipgloss.NewStyle().Foreground(theme.SubText)
		}
	}
}

func (m *CreateTopicModel) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}
	// Typing can make a field valid or invalid, so the colours are refreshed
	// with the values rather than only when focus moves.
	m.styleInputs()
	return tea.Batch(cmds...)
}

func (m CreateTopicModel) View() string {
	var sb strings.Builder

	sb.WriteString(sectionTitleStyle.Render("Create topic"))
	sb.WriteString("\n\n")
	sb.WriteString(m.renderFields())
	sb.WriteString("\n")
	sb.WriteString(m.renderSummary())
	sb.WriteString("\n\n")
	sb.WriteString(m.renderButtons())

	if status := m.renderStatus(); status != "" {
		sb.WriteString("\n\n")
		sb.WriteString(status)
	}

	box := overlayBoxStyle(m.boxWidth()).Render(sb.String())
	help := renderHelpBar(
		"tab", "next field",
		"enter", "next / create",
		"esc", "cancel",
	)

	form := lipgloss.JoinVertical(lipgloss.Center, box, "", help)
	return renderOverlay(form, m.width, m.height)
}

// boxWidth keeps the frame readable on a narrow terminal and stops it sprawling
// on a wide one.
func (m CreateTopicModel) boxWidth() int {
	if m.width <= 0 {
		return 56
	}
	return min(max(m.width-8, 34), 64)
}

// renderFields draws the label/value rows, with a hint or a validation message
// under the field it belongs to.
func (m CreateTopicModel) renderFields() string {
	var sb strings.Builder

	for i := range m.inputs {
		label := topicFields[i].label
		labelStyled := labelStyle.Render(pad(label, labelWidth))
		if i == m.focusIndex {
			labelStyled = lipgloss.NewStyle().
				Foreground(theme.Primary).
				Bold(true).
				Render(pad("› "+label, labelWidth))
		}

		sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, labelStyled, m.inputs[i].View()))
		sb.WriteString("\n")

		// One line of guidance at a time, under the field it is about: a
		// validation message when there is one, otherwise the hint for the
		// field being filled in.
		if note := m.fieldNote(i); note != "" {
			sb.WriteString(strings.Repeat(" ", labelWidth))
			sb.WriteString(note)
			sb.WriteString("\n")
		}

		if i < len(m.inputs)-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// fieldNote is the line shown under a field: its error if it has one, or its
// hint while it has focus.
func (m CreateTopicModel) fieldNote(index int) string {
	if err := m.fieldError(index); err != "" {
		return errorStyle.Render(err)
	}
	if index == m.focusIndex {
		return helpDescStyle.Render(topicFields[index].hint)
	}
	return ""
}

// renderSummary spells out what Create will do, including the defaults that
// apply to any field left blank.
func (m CreateTopicModel) renderSummary() string {
	rule := helpSepStyle.Render(strings.Repeat("─", max(m.boxWidth()-6, 10)))

	summary := m.summary()
	if summary == "" {
		summary = helpDescStyle.Render("Name the topic to continue")
	} else {
		summary = labelStyle.Render("Creates  ") + valueStyle.Render(summary)
	}

	return rule + "\n" + summary
}

// renderButtons draws Create and Cancel, with Create dimmed until the form is
// complete.
func (m CreateTopicModel) renderButtons() string {
	create := formButtonStyle(m.focusIndex == createButtonIdx, m.valid()).Render("Create")
	cancel := formButtonStyle(m.focusIndex == cancelButtonIdx, true).Render("Cancel")
	return lipgloss.JoinHorizontal(lipgloss.Top, create, "  ", cancel)
}

// formButtonStyle renders a form button. An action that cannot be taken is
// dimmed rather than hidden, so the form does not change shape as it is filled
// in.
func formButtonStyle(focused, enabled bool) lipgloss.Style {
	style := lipgloss.NewStyle().Padding(0, 2)

	switch {
	case !enabled:
		return style.Foreground(theme.Muted).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(theme.DimBorder)
	case focused:
		return style.Foreground(lipgloss.Color("229")).
			Background(theme.Secondary).
			Bold(true).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(theme.Primary)
	default:
		return style.Foreground(theme.SubText).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border)
	}
}

// renderStatus shows the outcome of the last create, if there was one.
func (m CreateTopicModel) renderStatus() string {
	if m.err != nil {
		return errorStyle.Render("✗  " + m.err.Error())
	}
	if m.successMsg != "" {
		return successStyle.Render("✓  " + m.successMsg)
	}
	return ""
}
