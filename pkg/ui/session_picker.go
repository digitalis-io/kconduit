package ui

import (
	"fmt"

	"github.com/digitalis-io/kconduit/pkg/config"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SessionConnect holds the result of a session selection — the session config
// plus an in-memory password that is never written to disk.
type SessionConnect struct {
	Name     string
	Session  config.Session
	Password string
}

type pickerState int

const (
	pickerStateList     pickerState = iota
	pickerStateForm                  // creating a new session
	pickerStatePassword              // password prompt before connect
)

// SessionPickerModel is a standalone Bubble Tea program shown at startup
// when no --brokers or --session flag is provided.
type SessionPickerModel struct {
	state    pickerState
	table    table.Model
	sessions map[string]config.Session
	names    []string // sorted

	form          *SessionFormModel
	passwordInput textinput.Model
	pendingName   string // session name waiting for password

	result *SessionConnect // set when user completes selection
	quit   bool

	width  int
	height int
	err    error
}

func newPickerTable() table.Model {
	cols := []table.Column{
		{Title: "Name", Width: 20},
		{Title: "Brokers", Width: 30},
		{Title: "SASL", Width: 6},
		{Title: "TLS", Width: 5},
	}
	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	s := table.DefaultStyles()
	s.Header = tableHeaderStyle()
	s.Selected = tableSelectedStyle()
	t.SetStyles(s)
	return t
}

func newPickerPasswordInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "Enter password"
	ti.EchoMode = textinput.EchoPassword
	ti.CharLimit = 256
	ti.Width = 40
	return ti
}

type pickerSessionsLoadedMsg struct {
	names    []string
	sessions map[string]config.Session
	err      error
}

func loadPickerSessions() tea.Cmd {
	return func() tea.Msg {
		names, err := config.List()
		if err != nil {
			return pickerSessionsLoadedMsg{err: err}
		}
		sessions := make(map[string]config.Session, len(names))
		for _, name := range names {
			sess, err := config.Load(name)
			if err == nil && sess != nil {
				sessions[name] = *sess
			}
		}
		return pickerSessionsLoadedMsg{names: names, sessions: sessions}
	}
}

func newSessionPickerModel(width, height int) SessionPickerModel {
	return SessionPickerModel{
		state:         pickerStateList,
		table:         newPickerTable(),
		passwordInput: newPickerPasswordInput(),
		width:         width,
		height:        height,
	}
}

func (m SessionPickerModel) Init() tea.Cmd {
	return loadPickerSessions()
}

func (m SessionPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.form != nil {
			updated, cmd := m.form.Update(msg)
			m.form = updated
			return m, cmd
		}
		return m, nil

	case pickerSessionsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.names = msg.names
		m.sessions = msg.sessions
		m.rebuildTable()
		// No sessions → open create form immediately
		if len(m.names) == 0 {
			m.state = pickerStateForm
			m.form = NewSessionFormModel(m.width, m.height)
			return m, m.form.Init()
		}
		return m, nil

	case sessionFormDoneMsg:
		if msg.cancelled {
			m.state = pickerStateList
			m.form = nil
			return m, nil
		}
		// Save the session (without password) then reload
		saveErr := config.Save(msg.name, msg.session)
		m.state = pickerStateList
		m.form = nil
		if saveErr != nil {
			m.err = saveErr
			return m, nil
		}
		return m, loadPickerSessions()

	case tea.KeyMsg:
		switch m.state {
		case pickerStateList:
			return m.updateList(msg)
		case pickerStateForm:
			return m.updateForm(msg)
		case pickerStatePassword:
			return m.updatePassword(msg)
		}
	}

	// Delegate non-key messages to sub-states
	switch m.state {
	case pickerStateForm:
		if m.form != nil {
			updated, cmd := m.form.Update(msg)
			m.form = updated
			return m, cmd
		}
	case pickerStatePassword:
		var cmd tea.Cmd
		m.passwordInput, cmd = m.passwordInput.Update(msg)
		return m, cmd
	case pickerStateList:
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m SessionPickerModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quit = true
		return m, tea.Quit

	case "n":
		m.state = pickerStateForm
		m.form = NewSessionFormModel(m.width, m.height)
		return m, m.form.Init()

	case "enter", "c":
		return m.connectSelected()
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m SessionPickerModel) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.form == nil {
		return m, nil
	}
	updated, cmd := m.form.Update(msg)
	m.form = updated
	return m, cmd
}

func (m SessionPickerModel) updatePassword(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.state = pickerStateList
		m.pendingName = ""
		m.passwordInput.Reset()
		return m, nil

	case "enter":
		sess := m.sessions[m.pendingName]
		m.result = &SessionConnect{
			Name:     m.pendingName,
			Session:  sess,
			Password: m.passwordInput.Value(),
		}
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.passwordInput, cmd = m.passwordInput.Update(msg)
	return m, cmd
}

func (m SessionPickerModel) connectSelected() (tea.Model, tea.Cmd) {
	if len(m.names) == 0 {
		return m, nil
	}
	row := m.table.SelectedRow()
	if row == nil {
		return m, nil
	}
	name := row[0]
	sess, ok := m.sessions[name]
	if !ok {
		return m, nil
	}

	if sess.SASL.Enabled {
		m.state = pickerStatePassword
		m.pendingName = name
		m.passwordInput.Reset()
		return m, m.passwordInput.Focus()
	}

	// No SASL — connect immediately
	m.result = &SessionConnect{Name: name, Session: sess}
	return m, tea.Quit
}

func (m *SessionPickerModel) rebuildTable() {
	rows := make([]table.Row, 0, len(m.names))
	for _, name := range m.names {
		sess := m.sessions[name]
		sasl := "No"
		if sess.SASL.Enabled {
			sasl = "Yes"
		}
		tls := "No"
		if sess.TLS.Enabled {
			tls = "Yes"
		}
		rows = append(rows, table.Row{name, sess.Brokers, sasl, tls})
	}
	m.table.SetRows(rows)
}

func (m SessionPickerModel) View() string {
	switch m.state {
	case pickerStateForm:
		if m.form != nil {
			return m.form.View()
		}

	case pickerStatePassword:
		content := fmt.Sprintf("Connecting to: %s\n\n%s\n\n%s",
			valueStyle.Render(m.pendingName),
			m.passwordInput.View(),
			renderHelpBar("enter", "connect", "esc", "cancel"),
		)
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			dialogBoxStyle.Render(content),
		)

	case pickerStateList:
		errStr := ""
		if m.err != nil {
			errStr = "\n" + errorStyle.Render(m.err.Error())
		}

		title := appHeaderStyle.Render("KConduit")

		if len(m.names) == 0 {
			help := renderHelpBar("n", "new session", "q", "quit")
			return title + "\n\n" +
				labelStyle.Render("No saved sessions.") + "\n\n" +
				help + errStr
		}

		help := renderHelpBar("↑↓", "navigate", "enter", "connect", "n", "new", "q", "quit")
		return title + "\n\n" +
			m.table.View() + "\n\n" + help + errStr
	}

	return ""
}

// RunSessionPicker runs the standalone session picker program and returns
// the selected SessionConnect, or nil if the user quit.
func RunSessionPicker() (*SessionConnect, error) {
	m := newSessionPickerModel(80, 24)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return nil, err
	}
	pm, ok := final.(SessionPickerModel)
	if !ok || pm.quit || pm.result == nil {
		return nil, nil
	}
	return pm.result, nil
}
