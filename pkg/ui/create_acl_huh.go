package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/digitalis-io/kconduit/pkg/kafka"
	"github.com/digitalis-io/kconduit/pkg/logger"
)

type CreateACLHuhModel struct {
	client   *kafka.Client
	form     *huh.Form
	creating bool
	spinner  spinner.Model
	err      error
	success  bool
	width    int
	height   int

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

var (
	resourceTypes = []huh.Option[string]{
		huh.NewOption("Topic", "Topic"),
		huh.NewOption("Group", "Group"),
		huh.NewOption("Cluster", "Cluster"),
		huh.NewOption("TransactionalId", "TransactionalId"),
		huh.NewOption("DelegationToken", "DelegationToken"),
	}

	patternTypes = []huh.Option[string]{
		huh.NewOption("Literal (Exact Match)", "Literal"),
		huh.NewOption("Prefixed (Prefix Match)", "Prefixed"),
		huh.NewOption("Any (Match All)", "Any"),
	}

	operationOptions = []huh.Option[string]{
		huh.NewOption("Read", "Read"),
		huh.NewOption("Write", "Write"),
		huh.NewOption("Create", "Create"),
		huh.NewOption("Delete", "Delete"),
		huh.NewOption("Alter", "Alter"),
		huh.NewOption("Describe", "Describe"),
		huh.NewOption("ClusterAction", "ClusterAction"),
		huh.NewOption("DescribeConfigs", "DescribeConfigs"),
		huh.NewOption("AlterConfigs", "AlterConfigs"),
		huh.NewOption("IdempotentWrite", "IdempotentWrite"),
		huh.NewOption("All", "All"),
	}

	permissionTypes = []huh.Option[string]{
		huh.NewOption("✅ Allow", "Allow"),
		huh.NewOption("❌ Deny", "Deny"),
	}
)

// NewCreateACLHuhModel builds the create dialog. width and height come from
// the view that opened it, for the reason given on NewCreateTopicModel.
func NewCreateACLHuhModel(client *kafka.Client, width, height int) *CreateACLHuhModel {
	m := &CreateACLHuhModel{
		client:         client,
		width:          width,
		height:         height,
		principal:      "",  // Start empty to ensure user input is captured
		host:           "*", // Default host to all
		resourceType:   "Topic",
		resourceName:   "", // Start empty to ensure user input is captured
		patternType:    "Literal",
		operations:     []string{}, // Start with no operations selected
		permissionType: "Allow",
		confirm:        false,
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

func (m *CreateACLHuhModel) buildForm() {
	formHeight := aclFormHeight(m.height)

	// Single group with all fields in one view
	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Principal").
				Description("User principal (e.g., User:alice, User:*)").
				Placeholder("User:alice").
				Value(&m.principal).
				Validate(m.validatePrincipal),

			huh.NewInput().
				Title("Host").
				Description("Client host (* for all hosts)").
				Placeholder("*").
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
				Placeholder("my-topic").
				Value(&m.resourceName).
				Validate(m.validateResourceName),

			huh.NewSelect[string]().
				Title("Pattern Type").
				Description("How to match the resource name").
				Options(patternTypes...).
				Value(&m.patternType),

			huh.NewMultiSelect[string]().
				Title("Operations").
				Description("Space to select, Enter to confirm").
				Options(operationOptions...).
				Value(&m.operations).
				Validate(m.validateOperations).
				Height(min(10, len(operationOptions))),

			huh.NewSelect[string]().
				Title("Permission").
				Description("Allow or Deny").
				Options(permissionTypes...).
				Value(&m.permissionType),

			huh.NewConfirm().
				Title("Ready to create ACL?").
				Description("Review your settings and confirm").
				Affirmative("✅ Create ACL").
				Negative("❌ Cancel").
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

// Validation methods
func (m *CreateACLHuhModel) validatePrincipal(s string) error {
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

func (m *CreateACLHuhModel) validateHost(s string) error {
	if s == "" {
		return fmt.Errorf("host cannot be empty")
	}
	return nil
}

func (m *CreateACLHuhModel) validateResourceName(s string) error {
	if s == "" {
		return fmt.Errorf("resource name cannot be empty")
	}
	return nil
}

func (m *CreateACLHuhModel) validateOperations(ops []string) error {
	if len(ops) == 0 {
		return fmt.Errorf("select at least one operation")
	}
	return nil
}

func (m *CreateACLHuhModel) Init() tea.Cmd {
	return m.form.Init()
}

type aclCreatedMsg struct {
	err error
}

func (m *CreateACLHuhModel) createACLs() tea.Cmd {
	return func() tea.Msg {
		// Validate we have operations to create
		if len(m.operations) == 0 {
			return aclCreatedMsg{err: fmt.Errorf("no operations selected")}
		}

		// Log what we're about to create for debugging
		log := logger.Get()
		log.WithFields(map[string]interface{}{
			"principal":      m.principal,
			"host":           m.host,
			"resourceType":   m.resourceType,
			"resourceName":   m.resourceName,
			"patternType":    m.patternType,
			"operations":     m.operations,
			"permissionType": m.permissionType,
		}).Info("Creating ACLs")

		// Create an ACL for each selected operation
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

			log.WithFields(map[string]interface{}{
				"operation": operation,
				"acl":       acl,
			}).Debug("Creating individual ACL")

			err := m.client.CreateACL(acl)
			if err != nil {
				log.WithError(err).WithField("operation", operation).Error("Failed to create ACL")
				errors = append(errors, fmt.Sprintf("%s: %v", operation, err))
			} else {
				log.WithField("operation", operation).Info("Successfully created ACL")
				successCount++
			}
		}

		if len(errors) > 0 {
			return aclCreatedMsg{err: fmt.Errorf("failed to create %d ACLs: %s", len(errors), strings.Join(errors, "; "))}
		}

		if successCount == 0 {
			return aclCreatedMsg{err: fmt.Errorf("no ACLs were created")}
		}

		log.WithField("count", successCount).Info("Successfully created all ACLs")
		return aclCreatedMsg{err: nil}
	}
}

func (m *CreateACLHuhModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		log.WithField("key", msg.String()).Debug("Key pressed in CreateACL")

		switch msg.String() {
		case "esc":
			if !m.creating {
				log.Debug("ESC pressed, returning to ACLs tab")
				return m, func() tea.Msg { return ViewChangedMsg{View: ACLsTab} }
			}
		case "ctrl+c":
			return m, tea.Quit
		}

	case aclCreatedMsg:
		log.WithField("error", msg.err).Info("ACL creation completed")
		m.creating = false
		if msg.err != nil {
			log.WithError(msg.err).Error("ACL creation failed")
			m.err = msg.err
			m.success = false
			// Don't rebuild form, just return to preserve state
			return m, nil
		}
		m.success = true
		log.Info("ACL(s) created successfully, returning to ACLs tab")
		return m, func() tea.Msg {
			return ViewChangedMsg{
				View:   ACLsTab,
				Notice: "ACL created",
				Level:  toastSuccess,
			}
		}

	case spinner.TickMsg:
		if m.creating {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	// If creating, don't process form updates
	if m.creating {
		return m, m.spinner.Tick
	}

	// Update form
	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f

		// Log current field values to debug the binding issue
		log.WithFields(map[string]interface{}{
			"state":        m.form.State,
			"principal":    m.principal,
			"resourceName": m.resourceName,
			"operations":   m.operations,
		}).Debug("Current form values during update")

		// Check if form is complete
		if m.form.State == huh.StateCompleted {
			// The form is completed
			// Now that we're using pointer receivers, the confirm field should be properly set

			log.WithFields(map[string]interface{}{
				"confirm":        m.confirm,
				"principal":      m.principal,
				"host":           m.host,
				"resourceType":   m.resourceType,
				"resourceName":   m.resourceName,
				"patternType":    m.patternType,
				"operations":     m.operations,
				"permissionType": m.permissionType,
			}).Info("Form completed, checking confirmation")

			// Check if user confirmed (selected "Create ACL")
			if m.confirm {
				log.Info("User confirmed, creating ACLs")
				// Form completed and confirmed, create ACLs
				m.creating = true
				return m, tea.Batch(
					m.spinner.Tick,
					m.createACLs(),
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

func (m *CreateACLHuhModel) View() string {
	frame := aclFrame{
		title: "Create ACL",
		summary: draftACLSummary(m.principal, m.host, m.resourceType,
			m.resourceName, m.patternType, m.permissionType, m.operations),
		form:   m.form.View(),
		help:   []string{"tab", "next field", "space", "select", "enter", "confirm", "esc", "cancel"},
		width:  m.width,
		height: m.height,
	}

	switch {
	case m.creating:
		frame.form = aclProgress(m.spinner.View(),
			fmt.Sprintf("Creating %s…", pluralise(len(m.operations), "ACL")))
		frame.help = []string{"esc", "cancel"}
	case m.success:
		frame.form = successStyle.Render("✓  Created")
		frame.help = []string{"esc", "back"}
	case m.err != nil:
		frame.status = errorStyle.Render("✗  " + m.err.Error())
	}

	return frame.render()
}
