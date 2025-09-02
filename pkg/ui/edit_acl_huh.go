package ui

import (
	"fmt"
	"strings"

	"github.com/axonops/kconduit/pkg/kafka"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

type EditACLHuhModel struct {
	client      *kafka.Client
	originalACL kafka.ACL
	form        *huh.Form
	updating    bool
	spinner     spinner.Model
	err         error
	success     bool
	width       int
	height      int

	// Form fields
	principal      string
	host           string
	resourceType   string
	resourceName   string
	patternType    string
	operations     []string
	permissionType string
	confirm        bool
}

func NewEditACLHuhModel(client *kafka.Client, acl kafka.ACL) EditACLHuhModel {
	m := EditACLHuhModel{
		client:         client,
		originalACL:    acl,
		principal:      acl.Principal,
		host:           acl.Host,
		resourceType:   acl.ResourceType,
		resourceName:   acl.ResourceName,
		patternType:    acl.PatternType,
		operations:     []string{acl.Operation}, // Start with the existing operation
		permissionType: acl.PermissionType,
		confirm:        false,
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

func (m *EditACLHuhModel) buildForm() {
	theme := huh.ThemeCharm()
	theme.Focused.Title = theme.Focused.Title.Foreground(lipgloss.Color("205"))
	theme.Focused.SelectedOption = theme.Focused.SelectedOption.Foreground(lipgloss.Color("205"))
	theme.Focused.MultiSelectSelector = theme.Focused.MultiSelectSelector.Foreground(lipgloss.Color("205"))

	// Calculate available height for form (leave room for title and help)
	formHeight := m.height - 8 // Account for title, help text, and margins
	if formHeight < 15 {
		formHeight = 15 // Minimum height for usability
	}
	if formHeight > 50 {
		formHeight = 50 // Cap maximum height for better UX
	}

	// Single group with all fields in one view
	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("✏️  Edit ACL").
				Description(fmt.Sprintf("Editing ACL for %s on %s %s\n⚠️  This will delete the existing ACL and create new ACL(s) with the updated values.",
					m.originalACL.Principal,
					m.originalACL.ResourceType,
					m.originalACL.ResourceName)),

			huh.NewInput().
				Title("Principal").
				Description("User principal (e.g., User:alice, User:*)").
				Value(&m.principal).
				Validate(m.validatePrincipal),

			huh.NewInput().
				Title("Host").
				Description("Client host (* for all hosts)").
				Value(&m.host).
				Validate(m.validateHost),

			huh.NewSelect[string]().
				Title("Resource Type").
				Description("Type of Kafka resource").
				Options(resourceTypes...).
				Value(&m.resourceType),

			huh.NewInput().
				Title("Resource Name").
				Description("Name of the resource (* for all)").
				Value(&m.resourceName).
				Validate(m.validateResourceName),

			huh.NewSelect[string]().
				Title("Pattern Type").
				Description("How to match the resource name").
				Options(patternTypes...).
				Value(&m.patternType),

			huh.NewMultiSelect[string]().
				Title("Operations").
				Description("Select operations to replace the existing one").
				Options(operationOptions...).
				Value(&m.operations).
				Validate(m.validateOperations).
				Height(min(10, len(operationOptions))),

			huh.NewSelect[string]().
				Title("Permission").
				Description("Allow or Deny the selected operations").
				Options(permissionTypes...).
				Value(&m.permissionType),

			huh.NewConfirm().
				Title("Ready to update ACL?").
				Description("Press Enter to save, or Esc to cancel").
				Affirmative("Save").
				Negative("Cancel").
				Value(&m.confirm),
		),
	)

	m.form = m.form.
		WithTheme(theme).
		WithShowHelp(true).
		WithShowErrors(true).
		WithWidth(m.width - 4).
		WithHeight(formHeight)
}

func (m EditACLHuhModel) Init() tea.Cmd {
	return m.form.Init()
}

type aclUpdatedMsg struct {
	err error
}

func (m EditACLHuhModel) updateACLs() tea.Msg {
	// First delete the original ACL
	err := m.client.DeleteACL(m.originalACL)
	if err != nil {
		// Log but don't fail - the ACL might have already been deleted
		// TODO: Consider showing a warning to the user about deletion failure
		_ = err // Explicitly ignore the error as we want to continue with creation
	}

	// Create new ACLs for each selected operation
	var errors []string
	successCount := 0

	for _, operation := range m.operations {
		acl := kafka.ACL{
			Principal:      m.principal,
			Host:           m.host,
			ResourceType:   m.resourceType,
			ResourceName:   m.resourceName,
			PatternType:    m.patternType,
			Operation:      operation,
			PermissionType: m.permissionType,
		}

		err := m.client.CreateACL(acl)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", operation, err))
		} else {
			successCount++
		}
	}

	if len(errors) > 0 {
		return aclUpdatedMsg{err: fmt.Errorf("failed to create %d ACLs: %s", len(errors), strings.Join(errors, "; "))}
	}

	return aclUpdatedMsg{err: nil}
}

// Validation methods
func (m *EditACLHuhModel) validatePrincipal(s string) error {
	if s == "" {
		return fmt.Errorf("principal cannot be empty")
	}
	if !strings.HasPrefix(s, "User:") && !strings.HasPrefix(s, "Group:") {
		return fmt.Errorf("must start with 'User:' or 'Group:'")
	}
	// Check that there's actually a name after the prefix
	if s == "User:" || s == "Group:" {
		return fmt.Errorf("principal name cannot be empty (e.g., User:alice, User:*, Group:admins)")
	}
	// Validate that after "User:" or "Group:" there's at least one character
	parts := strings.SplitN(s, ":", 2)
	if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
		return fmt.Errorf("principal name cannot be empty (e.g., User:alice, User:*, Group:admins)")
	}
	return nil
}

func (m *EditACLHuhModel) validateHost(s string) error {
	if s == "" {
		return fmt.Errorf("host cannot be empty")
	}
	return nil
}

func (m *EditACLHuhModel) validateResourceName(s string) error {
	if s == "" {
		return fmt.Errorf("resource name cannot be empty")
	}
	return nil
}

func (m *EditACLHuhModel) validateOperations(ops []string) error {
	if len(ops) == 0 {
		return fmt.Errorf("select at least one operation")
	}
	return nil
}

func (m EditACLHuhModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		switch msg.String() {
		case "esc":
			if !m.updating {
				return m, func() tea.Msg { return ViewChangedMsg{View: ACLsTab} }
			}
		case "ctrl+c":
			return m, tea.Quit
		}

	case aclUpdatedMsg:
		m.updating = false
		if msg.err != nil {
			m.err = msg.err
			m.success = false
			// Don't rebuild form, just return to preserve state
			return m, nil
		}
		m.success = true
		return m, tea.Batch(
			tea.Println("✅ ACL(s) updated successfully!"),
			func() tea.Msg { return ViewChangedMsg{View: ACLsTab} },
		)

	case spinner.TickMsg:
		if m.updating {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	// If updating, don't process form updates
	if m.updating {
		return m, m.spinner.Tick
	}

	// Update form
	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f

		// Check if form is complete
		if m.form.State == huh.StateCompleted {
			// Check if user confirmed
			if m.confirm {
				// Form completed and confirmed, update ACLs
				m.updating = true
				return m, tea.Batch(
					m.spinner.Tick,
					m.updateACLs,
				)
			} else {
				// User cancelled
				return m, func() tea.Msg { return ViewChangedMsg{View: ACLsTab} }
			}
		}
	}

	return m, cmd
}

func (m EditACLHuhModel) View() string {
	if m.updating {
		return lipgloss.NewStyle().
			Padding(2, 4).
			Render(fmt.Sprintf("%s Updating ACL(s)...\n\nDeleting original: %s %s on %s %s\nCreating new: %s operations",
				m.spinner.View(),
				m.originalACL.Operation,
				m.originalACL.PermissionType,
				m.originalACL.ResourceType,
				m.originalACL.ResourceName,
				strings.Join(m.operations, ", ")))
	}

	if m.success {
		successStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true).
			Padding(2, 4)
		return successStyle.Render("✅ ACL(s) updated successfully!")
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

	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Padding(0, 2)

	helpText := helpStyle.Render("Use Tab/Shift+Tab to navigate • Space to select • Enter to confirm • Esc to cancel")

	// Combine all views
	formView := m.form.View()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		formView,
		errorView,
		helpText,
	)
}
