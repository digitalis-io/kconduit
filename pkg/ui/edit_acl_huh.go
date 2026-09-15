package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/digitalis-io/kconduit/pkg/kafka"
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

// NewEditACLHuhModel builds the edit dialog. width and height come from the
// view that opened it, for the reason given on NewCreateTopicModel.
func NewEditACLHuhModel(client *kafka.Client, acl kafka.ACL, width, height int) EditACLHuhModel {
	m := EditACLHuhModel{
		client:         client,
		width:          width,
		height:         height,
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
	formHeight := aclFormHeight(m.height)

	// Single group with all fields in one view
	m.form = huh.NewForm(
		huh.NewGroup(
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

	frame := aclFrame{width: m.width}
	m.form = m.form.
		WithTheme(aclHuhTheme()).
		WithShowHelp(true).
		WithShowErrors(true).
		WithWidth(frame.boxWidth() - 6).
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
			frame := aclFrame{width: m.width}
			m.form = m.form.
				WithWidth(frame.boxWidth() - 6).
				WithHeight(aclFormHeight(m.height))
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
		return m, func() tea.Msg {
			return ViewChangedMsg{
				View:   ACLsTab,
				Notice: "ACL updated",
				Level:  toastSuccess,
			}
		}

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
	// Editing an ACL is a delete followed by a create — Kafka has no update —
	// so the frame says so rather than letting "Save" imply an in-place change.
	summary := labelStyle.Render("Replacing") + "\n" +
		aclSummary(m.originalACL) + "\n\n" +
		labelStyle.Render("With") + "\n" +
		draftACLSummary(m.principal, m.host, m.resourceType,
			m.resourceName, m.patternType, m.permissionType, m.operations)

	frame := aclFrame{
		title:   "Edit ACL",
		summary: summary,
		form:    m.form.View(),
		help:    []string{"tab", "next field", "space", "select", "enter", "save", "esc", "cancel"},
		width:   m.width,
		height:  m.height,
	}

	switch {
	case m.updating:
		frame.form = aclProgress(m.spinner.View(), "Replacing the ACL…")
		frame.help = []string{"esc", "cancel"}
	case m.success:
		frame.form = successStyle.Render("✓  Updated")
		frame.help = []string{"esc", "back"}
	case m.err != nil:
		frame.status = errorStyle.Render("✗  " + m.err.Error())
	}

	return frame.render()
}
