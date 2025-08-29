package ui

import (
	"fmt"
	"strings"

	"github.com/axonops/kconduit/pkg/kafka"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type CreateACLModel struct {
	client    *kafka.Client
	inputs    []textinput.Model
	focusIndex int
	err       error
}

const (
	principalInput = iota
	hostInput
	resourceTypeInput
	resourceNameInput
	patternTypeInput
	operationInput
	permissionTypeInput
)

func NewCreateACLModel(client *kafka.Client) CreateACLModel {
	inputs := make([]textinput.Model, 7)
	
	inputs[principalInput] = textinput.New()
	inputs[principalInput].Placeholder = "User:alice or User:* for all"
	inputs[principalInput].Focus()
	inputs[principalInput].SetValue("User:")
	inputs[principalInput].Width = 40
	inputs[principalInput].CharLimit = 100
	
	inputs[hostInput] = textinput.New()
	inputs[hostInput].Placeholder = "* for all hosts"
	inputs[hostInput].SetValue("*")
	inputs[hostInput].Width = 40
	inputs[hostInput].CharLimit = 100
	
	inputs[resourceTypeInput] = textinput.New()
	inputs[resourceTypeInput].Placeholder = "Topic, Group, Cluster, TransactionalId"
	inputs[resourceTypeInput].SetValue("Topic")
	inputs[resourceTypeInput].Width = 40
	inputs[resourceTypeInput].CharLimit = 50
	
	inputs[resourceNameInput] = textinput.New()
	inputs[resourceNameInput].Placeholder = "Resource name or * for all"
	inputs[resourceNameInput].SetValue("*")
	inputs[resourceNameInput].Width = 40
	inputs[resourceNameInput].CharLimit = 100
	
	inputs[patternTypeInput] = textinput.New()
	inputs[patternTypeInput].Placeholder = "Literal, Prefixed, Any"
	inputs[patternTypeInput].SetValue("Literal")
	inputs[patternTypeInput].Width = 40
	inputs[patternTypeInput].CharLimit = 20
	
	inputs[operationInput] = textinput.New()
	inputs[operationInput].Placeholder = "Read, Write, Create, Delete, Alter, Describe, All"
	inputs[operationInput].SetValue("Read")
	inputs[operationInput].Width = 40
	inputs[operationInput].CharLimit = 50
	
	inputs[permissionTypeInput] = textinput.New()
	inputs[permissionTypeInput].Placeholder = "Allow or Deny"
	inputs[permissionTypeInput].SetValue("Allow")
	inputs[permissionTypeInput].Width = 40
	inputs[permissionTypeInput].CharLimit = 10
	
	return CreateACLModel{
		client: client,
		inputs: inputs,
	}
}

func (m CreateACLModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m CreateACLModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return ViewChangedMsg{View: ACLsTab} }
		case "tab", "shift+tab", "up", "down":
			if msg.String() == "tab" || msg.String() == "down" {
				m.focusIndex++
			} else {
				m.focusIndex--
			}
			if m.focusIndex < 0 {
				m.focusIndex = len(m.inputs) - 1
			} else if m.focusIndex >= len(m.inputs) {
				m.focusIndex = 0
			}
			
			cmds := make([]tea.Cmd, len(m.inputs))
			for i := range m.inputs {
				if i == m.focusIndex {
					cmds[i] = m.inputs[i].Focus()
				} else {
					m.inputs[i].Blur()
				}
			}
			return m, tea.Batch(cmds...)
		case "enter", "ctrl+s":
			// Validate inputs
			if err := m.validateInputs(); err != nil {
				m.err = err
				return m, nil
			}
			
			// Create the ACL
			acl := kafka.ACL{
				Principal:      m.inputs[principalInput].Value(),
				Host:           m.inputs[hostInput].Value(),
				ResourceType:   m.inputs[resourceTypeInput].Value(),
				ResourceName:   m.inputs[resourceNameInput].Value(),
				PatternType:    m.inputs[patternTypeInput].Value(),
				Operation:      m.inputs[operationInput].Value(),
				PermissionType: m.inputs[permissionTypeInput].Value(),
			}
			
			err := m.client.CreateACL(acl)
			if err != nil {
				m.err = err
				return m, nil
			}
			
			return m, func() tea.Msg { return ViewChangedMsg{View: ACLsTab} }
		}
	}
	
	// Update the focused input
	var cmd tea.Cmd
	m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
	return m, cmd
}

func (m CreateACLModel) View() string {
	var b strings.Builder
	
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		MarginBottom(1)
	
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Width(20)
	
	inputStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("255"))
	
	focusedLabelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true).
		Width(20)
	
	b.WriteString(titleStyle.Render("🔐 Create Access Control List"))
	b.WriteString("\n\n")
	
	labels := []string{
		"Principal:",
		"Host:",
		"Resource Type:",
		"Resource Name:",
		"Pattern Type:",
		"Operation:",
		"Permission:",
	}
	
	descriptions := []string{
		"(e.g., User:alice, User:*)",
		"(* for all hosts, or specific IP/hostname)",
		"(Topic, Group, Cluster, TransactionalId)",
		"(resource name or * for all)",
		"(Literal, Prefixed, Any)",
		"(Read, Write, Create, Delete, Alter, Describe, All)",
		"(Allow or Deny)",
	}
	
	for i, input := range m.inputs {
		if i == m.focusIndex {
			b.WriteString(focusedLabelStyle.Render(labels[i]))
		} else {
			b.WriteString(labelStyle.Render(labels[i]))
		}
		b.WriteString(inputStyle.Render(input.View()))
		
		// Add description for the focused field
		if i == m.focusIndex {
			descStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("244")).
				Italic(true).
				MarginLeft(20)
			b.WriteString("\n" + descStyle.Render(descriptions[i]))
		}
		b.WriteString("\n")
	}
	
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true).
			MarginTop(1)
		b.WriteString("\n" + errorStyle.Render("❌ Error: " + m.err.Error()))
	}
	
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(2)
	
	help := "\n" + helpStyle.Render("Tab/↓: Next field • Shift+Tab/↑: Previous field • Enter/Ctrl+S: Create ACL • Esc: Cancel")
	b.WriteString(help)
	
	// Add a box around the entire form
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 2)
	
	return boxStyle.Render(b.String())
}

func (m *CreateACLModel) validateInputs() error {
	// Validate Principal
	principal := strings.TrimSpace(m.inputs[principalInput].Value())
	if principal == "" {
		return fmt.Errorf("principal cannot be empty")
	}
	if !strings.HasPrefix(principal, "User:") {
		return fmt.Errorf("principal must start with 'User:' (e.g., User:alice)")
	}
	
	// Validate Host
	host := strings.TrimSpace(m.inputs[hostInput].Value())
	if host == "" {
		return fmt.Errorf("host cannot be empty (use * for all hosts)")
	}
	
	// Validate Resource Type
	resourceType := strings.TrimSpace(m.inputs[resourceTypeInput].Value())
	validResourceTypes := []string{"Topic", "Group", "Cluster", "TransactionalId", "DelegationToken"}
	if !contains(validResourceTypes, resourceType) {
		return fmt.Errorf("invalid resource type. Must be one of: %v", validResourceTypes)
	}
	
	// Validate Resource Name
	resourceName := strings.TrimSpace(m.inputs[resourceNameInput].Value())
	if resourceName == "" {
		return fmt.Errorf("resource name cannot be empty (use * for all resources)")
	}
	
	// Validate Pattern Type
	patternType := strings.TrimSpace(m.inputs[patternTypeInput].Value())
	validPatternTypes := []string{"Literal", "Prefixed", "Any"}
	if !contains(validPatternTypes, patternType) {
		return fmt.Errorf("invalid pattern type. Must be one of: %v", validPatternTypes)
	}
	
	// Validate Operation
	operation := strings.TrimSpace(m.inputs[operationInput].Value())
	validOperations := []string{"Read", "Write", "Create", "Delete", "Alter", "Describe", 
		"ClusterAction", "DescribeConfigs", "AlterConfigs", "IdempotentWrite", "All"}
	if !contains(validOperations, operation) {
		return fmt.Errorf("invalid operation. Must be one of: %v", validOperations)
	}
	
	// Validate Permission Type
	permissionType := strings.TrimSpace(m.inputs[permissionTypeInput].Value())
	validPermissionTypes := []string{"Allow", "Deny"}
	if !contains(validPermissionTypes, permissionType) {
		return fmt.Errorf("invalid permission type. Must be one of: %v", validPermissionTypes)
	}
	
	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}