package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/digitalis-io/kconduit/pkg/kafka"
	"github.com/digitalis-io/kconduit/pkg/logger"
)

type DeleteACLModel struct {
	client   *kafka.Client
	acl      kafka.ACL
	form     *huh.Form
	deleting bool
	spinner  spinner.Model
	err      error
	success  bool
	width    int
	height   int
	confirm  bool
}

// NewDeleteACLModel builds the delete confirmation. width and height come from
// the view that opened it, for the reason given on NewCreateTopicModel.
func NewDeleteACLModel(client *kafka.Client, acl kafka.ACL, width, height int) *DeleteACLModel {
	m := &DeleteACLModel{
		client:  client,
		width:   width,
		height:  height,
		acl:     acl,
		confirm: false,
	}

	// Create spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(theme.Primary)
	m.spinner = s

	// Build the form
	m.buildForm()

	return m
}

func (m *DeleteACLModel) buildForm() {
	// The ACL being deleted is described by the frame, so the form is only the
	// confirmation. It used to be a note repeating all seven fields followed by
	// a confirm, which made the dialog scroll on a short terminal.
	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Delete this ACL?").
				Description("This cannot be undone.").
				Affirmative("Delete").
				Negative("Cancel").
				Value(&m.confirm),
		),
	)

	frame := aclFrame{width: m.width}
	m.form = m.form.
		WithTheme(aclHuhTheme()).
		WithShowHelp(true).
		WithShowErrors(true).
		WithWidth(frame.boxWidth() - 6).
		WithHeight(6)
}

func (m *DeleteACLModel) Init() tea.Cmd {
	return m.form.Init()
}

type aclDeletedMsg struct {
	err error
}

func (m *DeleteACLModel) deleteACL() tea.Cmd {
	return func() tea.Msg {
		log := logger.Get()
		log.WithFields(map[string]interface{}{
			"principal":      m.acl.Principal,
			"host":           m.acl.Host,
			"resourceType":   m.acl.ResourceType,
			"resourceName":   m.acl.ResourceName,
			"patternType":    m.acl.PatternType,
			"operation":      m.acl.Operation,
			"permissionType": m.acl.PermissionType,
		}).Info("Attempting to delete ACL")

		err := m.client.DeleteACL(m.acl)
		if err != nil {
			log.WithError(err).Error("Failed to delete ACL")
		} else {
			log.Info("Successfully deleted ACL")
		}
		return aclDeletedMsg{err: err}
	}
}

func (m *DeleteACLModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	log := logger.Get()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Update form dimensions without rebuilding
		if m.form != nil {
			frame := aclFrame{width: m.width}
			m.form = m.form.
				WithWidth(frame.boxWidth() - 6).
				WithHeight(aclFormHeight(m.height))
		}
		return m, nil

	case tea.KeyMsg:
		log.WithField("key", msg.String()).Debug("Key pressed in DeleteACL")

		switch msg.String() {
		case "esc":
			if !m.deleting {
				log.Debug("ESC pressed, returning to ACLs tab")
				return m, func() tea.Msg { return ViewChangedMsg{View: ACLsTab} }
			}
		case "ctrl+c":
			return m, tea.Quit
		}

	case aclDeletedMsg:
		log.WithField("error", msg.err).Info("ACL deletion completed")
		if msg.err != nil {
			log.WithError(msg.err).Error("ACL deletion failed")
			m.deleting = false
			m.err = msg.err
			m.success = false
			// Show error but don't return to list yet
			return m, nil
		}
		// Set success first, then clear deleting flag to avoid brief error display
		m.success = true
		m.deleting = false
		log.Info("ACL deleted successfully, returning to ACLs tab")
		// Add a small delay before returning to see the success message
		return m, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
			return ViewChangedMsg{
				View:   ACLsTab,
				Notice: "ACL deleted",
				Level:  toastSuccess,
			}
		})

	case spinner.TickMsg:
		if m.deleting {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	// If deleting, don't process form updates
	if m.deleting {
		return m, m.spinner.Tick
	}

	// Update form
	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f

		// Log current field values to debug the binding issue
		log.WithFields(map[string]interface{}{
			"state":   m.form.State,
			"confirm": m.confirm,
		}).Debug("Current form values during update")

		// Check if form is complete
		if m.form.State == huh.StateCompleted {
			log.WithField("confirm", m.confirm).Info("Form completed, checking confirmation")

			// Check if user confirmed
			if m.confirm {
				log.Info("User confirmed, deleting ACL")
				// Form completed and confirmed, delete ACL
				m.deleting = true
				return m, tea.Batch(
					m.spinner.Tick,
					m.deleteACL(),
				)
			} else {
				log.Info("User cancelled, returning to ACLs tab")
				// User cancelled
				return m, func() tea.Msg { return ViewChangedMsg{View: ACLsTab} }
			}
		}
	}

	return m, cmd
}

func (m *DeleteACLModel) View() string {
	frame := aclFrame{
		title:   "Delete ACL",
		summary: aclSummary(m.acl) + "\n\n" + aclFieldTable(m.acl),
		form:    m.form.View(),
		help:    []string{"tab", "switch", "enter", "confirm", "esc", "cancel"},
		danger:  true,
		width:   m.width,
		height:  m.height,
	}

	switch {
	case m.success:
		frame.form = successStyle.Render("✓  Deleted")
		frame.help = []string{"esc", "back"}
	case m.deleting:
		frame.form = aclProgress(m.spinner.View(), "Deleting the ACL…")
		frame.help = []string{"esc", "cancel"}
	case m.err != nil:
		frame.status = errorStyle.Render("✗  " + m.err.Error())
	}

	return frame.render()
}
