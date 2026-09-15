package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/digitalis-io/kconduit/pkg/config"
)

// sessionFormDoneMsg is emitted when the session form completes or is cancelled.
type sessionFormDoneMsg struct {
	name      string
	session   config.Session
	password  string // in-memory only, never persisted
	cancelled bool
}

// SessionFormModel is a 4-page huh form for creating or editing a session.
// Used by both the startup picker and the in-TUI session manager.
type SessionFormModel struct {
	form     *huh.Form
	isEdit   bool
	editName string // original name when editing (for rename detection)
	width    int
	height   int
	done     bool

	// form field values — pointers so huh can bind directly
	name         string
	brokers      string
	saslEnabled  bool
	mechanism    string
	username     string
	saslProtocol string
	password     string // in-memory only
	tlsEnabled   bool
	caCert       string
	clientCert   string
	clientKey    string
	skipVerify   bool
	logLevel     string
	aiEngine     string
	aiModel      string
}

var (
	saslMechanismOptions = []huh.Option[string]{
		huh.NewOption("PLAIN", "PLAIN"),
		huh.NewOption("SCRAM-SHA-256", "SCRAM-SHA-256"),
		huh.NewOption("SCRAM-SHA-512", "SCRAM-SHA-512"),
	}
	saslProtocolOptions = []huh.Option[string]{
		huh.NewOption("SASL_PLAINTEXT", "SASL_PLAINTEXT"),
		huh.NewOption("SASL_SSL", "SASL_SSL"),
	}
	logLevelOptions = []huh.Option[string]{
		huh.NewOption("info", "info"),
		huh.NewOption("debug", "debug"),
		huh.NewOption("warn", "warn"),
		huh.NewOption("error", "error"),
	}
	aiEngineOptions = []huh.Option[string]{
		huh.NewOption("gemini", "gemini"),
		huh.NewOption("openai", "openai"),
		huh.NewOption("anthropic", "anthropic"),
		huh.NewOption("ollama", "ollama"),
	}
)

// NewSessionFormModel creates a blank session form.
func NewSessionFormModel(width, height int) *SessionFormModel {
	m := &SessionFormModel{
		width:        width,
		height:       height,
		mechanism:    "PLAIN",
		saslProtocol: "SASL_PLAINTEXT",
		logLevel:     "info",
		aiEngine:     "gemini",
		aiModel:      "gemini-3.1-pro-preview",
	}
	m.buildForm()
	return m
}

// NewSessionFormModelFromSession creates a pre-filled edit form. Password is left blank.
func NewSessionFormModelFromSession(name string, sess config.Session, width, height int) *SessionFormModel {
	m := &SessionFormModel{
		isEdit:       true,
		editName:     name,
		width:        width,
		height:       height,
		name:         name,
		brokers:      sess.Brokers,
		saslEnabled:  sess.SASL.Enabled,
		mechanism:    sess.SASL.Mechanism,
		username:     sess.SASL.Username,
		saslProtocol: sess.SASL.Protocol,
		tlsEnabled:   sess.TLS.Enabled,
		caCert:       sess.TLS.CACert,
		clientCert:   sess.TLS.ClientCert,
		clientKey:    sess.TLS.ClientKey,
		skipVerify:   sess.TLS.SkipVerify,
		logLevel:     sess.LogLevel,
		aiEngine:     sess.AIEngine,
		aiModel:      sess.AIModel,
	}
	// Apply defaults for empty optional fields
	if m.mechanism == "" {
		m.mechanism = "PLAIN"
	}
	if m.saslProtocol == "" {
		m.saslProtocol = "SASL_PLAINTEXT"
	}
	if m.logLevel == "" {
		m.logLevel = "info"
	}
	if m.aiEngine == "" {
		m.aiEngine = "gemini"
	}
	m.buildForm()
	return m
}

func (m *SessionFormModel) buildForm() {
	huhTheme := huh.ThemeCharm()
	huhTheme.Focused.Title = huhTheme.Focused.Title.Foreground(theme.Primary)
	huhTheme.Focused.SelectedOption = huhTheme.Focused.SelectedOption.Foreground(theme.Primary)

	formHeight := m.height - 8
	if formHeight < 15 {
		formHeight = 15
	}
	if formHeight > 50 {
		formHeight = 50
	}

	m.form = huh.NewForm(
		// Page 1: Connection
		huh.NewGroup(
			huh.NewInput().
				Title("Session Name").
				Description("A unique name for this connection profile").
				Placeholder("my-cluster").
				Value(&m.name).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("session name cannot be empty")
					}
					return nil
				}),
			huh.NewInput().
				Title("Brokers").
				Description("Comma-separated broker addresses (host:port)").
				Placeholder("localhost:9092").
				Value(&m.brokers).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("brokers cannot be empty")
					}
					return nil
				}),
		).Title("Connection"),

		// Page 2: Authentication
		huh.NewGroup(
			huh.NewConfirm().
				Title("Enable SASL").
				Description("Use SASL authentication").
				Affirmative("Yes").
				Negative("No").
				Value(&m.saslEnabled),
			huh.NewSelect[string]().
				Title("SASL Mechanism").
				Options(saslMechanismOptions...).
				Value(&m.mechanism),
			huh.NewInput().
				Title("Username").
				Value(&m.username),
			huh.NewSelect[string]().
				Title("Security Protocol").
				Options(saslProtocolOptions...).
				Value(&m.saslProtocol),
			huh.NewInput().
				Title("Password").
				Description("In-memory only — never saved to disk").
				EchoMode(huh.EchoModePassword).
				Value(&m.password),
		).Title("Authentication"),

		// Page 3: TLS
		huh.NewGroup(
			huh.NewConfirm().
				Title("Enable TLS").
				Affirmative("Yes").
				Negative("No").
				Value(&m.tlsEnabled),
			huh.NewInput().
				Title("CA Certificate Path").
				Placeholder("/path/to/ca.pem").
				Value(&m.caCert),
			huh.NewInput().
				Title("Client Certificate Path").
				Placeholder("/path/to/client.pem").
				Value(&m.clientCert),
			huh.NewInput().
				Title("Client Key Path").
				Placeholder("/path/to/client.key").
				Value(&m.clientKey),
			huh.NewConfirm().
				Title("Skip TLS Verification").
				Description("⚠ Insecure — disables certificate validation").
				Affirmative("Yes").
				Negative("No").
				Value(&m.skipVerify),
		).Title("TLS / SSL"),

		// Page 4: Advanced
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Log Level").
				Options(logLevelOptions...).
				Value(&m.logLevel),
			huh.NewSelect[string]().
				Title("AI Engine").
				Options(aiEngineOptions...).
				Value(&m.aiEngine),
			huh.NewInput().
				Title("AI Model").
				Placeholder("gemini-3.1-pro-preview").
				Value(&m.aiModel),
		).Title("Advanced"),
	).
		WithTheme(huhTheme).
		WithShowHelp(true).
		WithShowErrors(true).
		WithWidth(m.width - 4).
		WithHeight(formHeight)
}

func (m *SessionFormModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m *SessionFormModel) Update(msg tea.Msg) (*SessionFormModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.form != nil {
			m.form = m.form.WithWidth(m.width - 4).WithHeight(m.height - 8)
		}
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f

		switch m.form.State {
		case huh.StateCompleted:
			m.done = true
			sess := config.Session{
				Brokers:  m.brokers,
				LogLevel: m.logLevel,
				AIEngine: m.aiEngine,
				AIModel:  m.aiModel,
				SASL: config.SessionSASL{
					Enabled:   m.saslEnabled,
					Mechanism: m.mechanism,
					Username:  m.username,
					Protocol:  m.saslProtocol,
				},
				TLS: config.SessionTLS{
					Enabled:    m.tlsEnabled,
					CACert:     m.caCert,
					ClientCert: m.clientCert,
					ClientKey:  m.clientKey,
					SkipVerify: m.skipVerify,
				},
			}
			return m, func() tea.Msg {
				return sessionFormDoneMsg{
					name:     m.name,
					session:  sess,
					password: m.password,
				}
			}

		case huh.StateAborted:
			m.done = true
			return m, func() tea.Msg {
				return sessionFormDoneMsg{cancelled: true}
			}
		}
	}

	return m, cmd
}

func (m *SessionFormModel) View() string {
	if m.form == nil {
		return ""
	}
	title := "New Session"
	if m.isEdit {
		title = fmt.Sprintf("Edit: %s", m.editName)
	}
	return sectionTitleStyle.Render(title) + "\n" + m.form.View()
}
