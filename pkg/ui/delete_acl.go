package ui

import (
	"fmt"
	"time"

	"github.com/digitalis-io/kconduit/pkg/kafka"
	"github.com/digitalis-io/kconduit/pkg/logger"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
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

func NewDeleteACLModel(client *kafka.Client, acl kafka.ACL) *DeleteACLModel {
	m := &DeleteACLModel{
		client:  client,
		acl:     acl,
		confirm: false,
	}

	// Create spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	m.spinner = s

	// Build the form
	m.buildForm()

	return m
}

func (m *DeleteACLModel) buildForm() {
	theme := huh.ThemeCharm()
	theme.Focused.Title = theme.Focused.Title.Foreground(lipgloss.Color("205"))

	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("🗑️  Delete ACL").
				Description(fmt.Sprintf(
					"Are you sure you want to delete this ACL?\n\n"+
						"Principal: %s\n"+
						"Host: %s\n"+
						"Resource: %s %s\n"+
						"Pattern: %s\n"+
						"Operation: %s\n"+
						"Permission: %s\n\n"+
						"⚠️  This action cannot be undone!",
					m.acl.Principal,
					m.acl.Host,
					m.acl.ResourceType,
					m.acl.ResourceName,
					m.acl.PatternType,
					m.acl.Operation,
					m.acl.PermissionType,
				)),

			huh.NewConfirm().
				Title("Delete this ACL?").
				Description("Press Enter to confirm deletion, or Esc to cancel").
				Affirmative("Yes, Delete").
				Negative("Cancel").
				Value(&m.confirm),
		),
	)

	m.form = m.form.
		WithTheme(theme).
		WithShowHelp(true).
		WithShowErrors(true).
		WithWidth(m.width - 4).
		WithHeight(m.height - 8)
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
			"host":          m.acl.Host,
			"resourceType":  m.acl.ResourceType,
			"resourceName":  m.acl.ResourceName,
			"patternType":   m.acl.PatternType,
			"operation":     m.acl.Operation,
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
			m.form = m.form.WithWidth(m.width - 4).WithHeight(m.height - 8)
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
		return m, tea.Batch(
			tea.Println("✅ ACL deleted successfully!"),
			tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
				return ViewChangedMsg{View: ACLsTab}
			}),
		)

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
			"state":    m.form.State,
			"confirm":  m.confirm,
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
	// Check success state first to avoid showing error during transition
	if m.success {
		successStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true).
			Padding(2, 4)
		return successStyle.Render("✅ ACL deleted successfully!")
	}
	
	if m.deleting {
		return lipgloss.NewStyle().
			Padding(2, 4).
			Render(fmt.Sprintf("%s Deleting ACL...\n\nPrincipal: %s\nResource: %s %s\nOperation: %s",
				m.spinner.View(),
				m.acl.Principal,
				m.acl.ResourceType,
				m.acl.ResourceName,
				m.acl.Operation))
	}

	// Error display
	var errorView string
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true).
			Padding(1, 2)
		errorView = errorStyle.Render(fmt.Sprintf("❌ Error: %v", m.err))
	}

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196")).
		MarginBottom(1).
		Padding(0, 2)

	title := titleStyle.Render("🗑️  Delete Access Control List")

	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Padding(0, 2)

	helpText := helpStyle.Render("Use Tab to navigate • Enter to confirm • Esc to cancel")

	// Combine all views
	formView := m.form.View()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		formView,
		errorView,
		helpText,
	)
}