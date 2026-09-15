package ui

import (
	"fmt"
	"os"
	"syscall"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/digitalis-io/kconduit/pkg/config"
)

type sessionManagerState int

const (
	smStateList     sessionManagerState = iota
	smStateForm                         // create or edit
	smStateDelete                       // inline delete confirmation
	smStatePassword                     // password prompt before connect
)

// SessionManagerModel handles the in-TUI session management view.
// It is embedded as a value in the main Model struct.
type SessionManagerModel struct {
	state    sessionManagerState
	table    table.Model
	names    []string
	sessions map[string]config.Session

	form          *SessionFormModel
	passwordInput textinput.Model
	deleteConfirm textinput.Model

	pendingName     string // session waiting for password / delete
	pendingPassword string // password stored temporarily for reconnect
	activeSession   string // currently connected session name

	width  int
	height int
	err    error
	status string // transient success message
}

// Messages used internally by the session manager

type smSessionsLoadedMsg struct {
	names    []string
	sessions map[string]config.Session
	err      error
}

type smSessionSavedMsg struct {
	name string
	err  error
}

type smSessionDeletedMsg struct {
	name string
	err  error
}

type smPendingPasswordMsg struct {
	name     string
	password string
}

func smLoadSessions() tea.Cmd {
	return func() tea.Msg {
		names, err := config.List()
		if err != nil {
			return smSessionsLoadedMsg{err: err}
		}
		sessions := make(map[string]config.Session, len(names))
		for _, name := range names {
			sess, err := config.Load(name)
			if err == nil && sess != nil {
				sessions[name] = *sess
			}
		}
		return smSessionsLoadedMsg{names: names, sessions: sessions}
	}
}

func smSaveSession(name string, sess config.Session) tea.Cmd {
	return func() tea.Msg {
		return smSessionSavedMsg{name: name, err: config.Save(name, sess)}
	}
}

func smDeleteSession(name string) tea.Cmd {
	return func() tea.Msg {
		return smSessionDeletedMsg{name: name, err: config.Delete(name)}
	}
}

// reconnectWithSession replaces the current process with a new invocation
// using the given session. If password is non-empty it is passed via env var.
func reconnectWithSession(name, password string) error {
	env := os.Environ()
	if password != "" {
		env = append(env, "KCONDUIT_SASL_PASSWORD="+password)
	}
	return syscall.Exec(os.Args[0], []string{os.Args[0], "--session", name}, env)
}

func newManagerTable() table.Model {
	cols := []table.Column{
		{Title: "Name", Width: 18},
		{Title: "Brokers", Width: 28},
		{Title: "SASL", Width: 6},
		{Title: "TLS", Width: 5},
		{Title: "Active", Width: 7},
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

// NewSessionManagerModel creates the session manager model.
func NewSessionManagerModel(activeSession string) SessionManagerModel {
	pi := textinput.New()
	pi.Placeholder = "Enter password"
	pi.EchoMode = textinput.EchoPassword
	pi.CharLimit = 256
	pi.Width = 40

	dc := textinput.New()
	dc.Placeholder = "Type session name to confirm"
	dc.CharLimit = 100
	dc.Width = 40

	return SessionManagerModel{
		state:         smStateList,
		table:         newManagerTable(),
		passwordInput: pi,
		deleteConfirm: dc,
		activeSession: activeSession,
	}
}

func (m SessionManagerModel) Init() tea.Cmd {
	return smLoadSessions()
}

func (m SessionManagerModel) Update(msg tea.Msg) (SessionManagerModel, tea.Cmd) {
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

	case smSessionsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.names = msg.names
		m.sessions = msg.sessions
		m.rebuildTable()
		return m, nil

	case smSessionSavedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.status = fmt.Sprintf("Session %q saved.", msg.name)
		// If we just saved the active session, reconnect automatically
		if msg.name == m.activeSession {
			sess := m.sessions[msg.name]
			if sess.SASL.Enabled {
				// Need password — go to password state
				m.state = smStatePassword
				m.pendingName = msg.name
				m.passwordInput.Reset()
				return m, m.passwordInput.Focus()
			}
			if err := reconnectWithSession(msg.name, ""); err != nil {
				m.err = fmt.Errorf("reconnect failed: %w", err)
			}
			return m, nil
		}
		return m, smLoadSessions()

	case smSessionDeletedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.status = fmt.Sprintf("Session %q deleted.", msg.name)
		return m, smLoadSessions()

	case sessionFormDoneMsg:
		m.state = smStateList
		if msg.cancelled {
			m.form = nil
			return m, nil
		}
		m.form = nil
		// Save the session (password is excluded from the config.Session)
		// If the session had a password entered, store it temporarily to trigger reconnect
		pendingPassword := msg.password
		pendingName := msg.name
		return m, tea.Batch(
			smSaveSession(msg.name, msg.session),
			func() tea.Msg {
				// After save, if active session, we'll handle reconnect in smSessionSavedMsg
				// Store password for reconnect path via a side-channel message
				if pendingPassword != "" {
					return smPendingPasswordMsg{name: pendingName, password: pendingPassword}
				}
				return nil
			},
		)

	case smPendingPasswordMsg:
		m.pendingPassword = msg.password
		return m, nil

	case tea.KeyMsg:
		switch m.state {
		case smStateList:
			return m.updateList(msg)
		case smStateForm:
			return m.updateFormKey(msg)
		case smStateDelete:
			return m.updateDelete(msg)
		case smStatePassword:
			return m.updatePassword(msg)
		}
	}

	// Delegate non-key messages to sub-states
	switch m.state {
	case smStateForm:
		if m.form != nil {
			updated, cmd := m.form.Update(msg)
			m.form = updated
			return m, cmd
		}
	case smStateList:
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	case smStatePassword:
		var cmd tea.Cmd
		m.passwordInput, cmd = m.passwordInput.Update(msg)
		return m, cmd
	case smStateDelete:
		var cmd tea.Cmd
		m.deleteConfirm, cmd = m.deleteConfirm.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m SessionManagerModel) updateList(msg tea.KeyMsg) (SessionManagerModel, tea.Cmd) {
	m.err = nil
	m.status = ""

	switch msg.String() {
	case "esc":
		return m, ReturnToListView

	case "n":
		m.state = smStateForm
		m.form = NewSessionFormModel(m.width, m.height)
		return m, m.form.Init()

	case "e":
		row := m.table.SelectedRow()
		if row == nil {
			return m, nil
		}
		name := row[0]
		sess, ok := m.sessions[name]
		if !ok {
			return m, nil
		}
		m.state = smStateForm
		m.form = NewSessionFormModelFromSession(name, sess, m.width, m.height)
		return m, m.form.Init()

	case "d":
		row := m.table.SelectedRow()
		if row == nil {
			return m, nil
		}
		m.pendingName = row[0]
		m.state = smStateDelete
		m.deleteConfirm.Reset()
		return m, m.deleteConfirm.Focus()

	case "c":
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
			m.state = smStatePassword
			m.pendingName = name
			m.passwordInput.Reset()
			return m, m.passwordInput.Focus()
		}
		if err := reconnectWithSession(name, ""); err != nil {
			m.err = fmt.Errorf("reconnect failed: %w", err)
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m SessionManagerModel) updateFormKey(msg tea.KeyMsg) (SessionManagerModel, tea.Cmd) {
	if m.form == nil {
		m.state = smStateList
		return m, nil
	}
	updated, cmd := m.form.Update(msg)
	m.form = updated
	return m, cmd
}

func (m SessionManagerModel) updateDelete(msg tea.KeyMsg) (SessionManagerModel, tea.Cmd) {
	switch msg.String() {
	case "esc", "n", "N":
		m.state = smStateList
		m.pendingName = ""
		m.deleteConfirm.Reset()
		return m, nil

	case "y", "Y":
		name := m.pendingName
		m.state = smStateList
		m.pendingName = ""
		m.deleteConfirm.Reset()
		return m, smDeleteSession(name)
	}

	var cmd tea.Cmd
	m.deleteConfirm, cmd = m.deleteConfirm.Update(msg)
	return m, cmd
}

func (m SessionManagerModel) updatePassword(msg tea.KeyMsg) (SessionManagerModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.state = smStateList
		m.pendingName = ""
		m.passwordInput.Reset()
		return m, nil

	case "enter":
		name := m.pendingName
		pw := m.passwordInput.Value()
		m.state = smStateList
		m.pendingName = ""
		m.passwordInput.Reset()
		if err := reconnectWithSession(name, pw); err != nil {
			m.err = fmt.Errorf("reconnect failed: %w", err)
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.passwordInput, cmd = m.passwordInput.Update(msg)
	return m, cmd
}

func (m *SessionManagerModel) rebuildTable() {
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
		active := ""
		if name == m.activeSession {
			active = "✓"
		}
		rows = append(rows, table.Row{name, sess.Brokers, sasl, tls, active})
	}
	m.table.SetRows(rows)
}

func (m SessionManagerModel) View() string {
	footer := ""
	if m.err != nil {
		footer = "\n" + errorStyle.Render(m.err.Error())
	} else if m.status != "" {
		footer = "\n" + successStyle.Render(m.status)
	}

	switch m.state {
	case smStateForm:
		if m.form != nil {
			return m.form.View()
		}

	case smStateDelete:
		content := fmt.Sprintf(
			"Delete session %s?\n\nThis cannot be undone.\n\n%s",
			valueStyle.Render(m.pendingName),
			renderHelpBar("y", "delete", "n", "cancel", "esc", "back"),
		)
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			dialogBoxStyle.Render(content),
		)

	case smStatePassword:
		content := fmt.Sprintf(
			"Connect to: %s\n\n%s\n\n%s",
			valueStyle.Render(m.pendingName),
			m.passwordInput.View(),
			renderHelpBar("enter", "connect", "esc", "cancel"),
		)
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			dialogBoxStyle.Render(content),
		)
	}

	// Default: list state
	help := renderHelpBar("↑↓", "navigate", "n", "new", "e", "edit", "d", "delete", "c", "connect", "esc", "back")

	if len(m.names) == 0 {
		return sectionTitleStyle.Render("Sessions") +
			"\n\n" + labelStyle.Render("No saved sessions.") + "\n\n" +
			help + footer
	}

	return sectionTitleStyle.Render("Sessions") + "\n\n" +
		m.table.View() + "\n\n" + help + footer
}
