package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/axonops/kconduit/pkg/kafka"
	"github.com/axonops/kconduit/pkg/logger"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// EditConfigModel handles editing a single configuration value
type EditConfigModel struct {
	client       *kafka.Client
	topicName    string
	configKey    string
	currentValue string
	newValue     string
	form         *huh.Form
	submitted    bool
	err          error
}

func NewEditConfigModel(client *kafka.Client, topicName, configKey, currentValue string) *EditConfigModel {
	// Create a new model
	model := &EditConfigModel{
		client:       client,
		topicName:    topicName,
		configKey:    configKey,
		currentValue: currentValue,
		newValue:     "", // Start with empty string
	}

	// Create input field based on the config key type
	var input huh.Field

	// Determine if this is a numeric field
	isNumeric := strings.HasSuffix(configKey, ".ms") ||
		strings.HasSuffix(configKey, ".bytes") ||
		configKey == "min.insync.replicas" ||
		strings.Contains(configKey, "factor")

	// Determine if this is a boolean field
	isBoolean := strings.Contains(configKey, "enable") ||
		strings.Contains(configKey, "enabled")

	// Determine if this is a choice field
	isChoice := configKey == "cleanup.policy" ||
		configKey == "compression.type" ||
		configKey == "message.timestamp.type"

	switch {
	case isBoolean:
		// Boolean fields use select with true/false options
		boolOptions := []huh.Option[string]{
			huh.NewOption(fmt.Sprintf("Keep current: %s", currentValue), ""),
			huh.NewOption("true", "true"),
			huh.NewOption("false", "false"),
		}
		input = huh.NewSelect[string]().
			Title(fmt.Sprintf("Edit %s", configKey)).
			Description(fmt.Sprintf("Current value: %s", currentValue)).
			Options(boolOptions...).
			Value(&model.newValue)

	case isChoice:
		// Choice fields with known options
		var options []huh.Option[string]
		
		// Add a "Keep current" option first to detect no change
		options = append(options, huh.NewOption(fmt.Sprintf("Keep current: %s", currentValue), ""))
		
		switch configKey {
		case "cleanup.policy":
			options = append(options,
				huh.NewOption("delete", "delete"),
				huh.NewOption("compact", "compact"),
				huh.NewOption("delete,compact", "delete,compact"),
			)
		case "compression.type":
			options = append(options,
				huh.NewOption("producer", "producer"),
				huh.NewOption("none", "none"),
				huh.NewOption("gzip", "gzip"),
				huh.NewOption("snappy", "snappy"),
				huh.NewOption("lz4", "lz4"),
				huh.NewOption("zstd", "zstd"),
			)
		case "message.timestamp.type":
			options = append(options,
				huh.NewOption("CreateTime", "CreateTime"),
				huh.NewOption("LogAppendTime", "LogAppendTime"),
			)
		}
		input = huh.NewSelect[string]().
			Title(fmt.Sprintf("Edit %s", configKey)).
			Description(fmt.Sprintf("Current value: %s", currentValue)).
			Options(options...).
			Value(&model.newValue)

	case isNumeric:
		// Numeric fields use text input with validation
		description := fmt.Sprintf("Current value: %s", currentValue)
		
		// Add help text for time-based fields
		if strings.HasSuffix(configKey, ".ms") {
			description += "\n💡 Tip: You can use formats like 1h, 1d, 7d, 1w (will convert to milliseconds)"
		}
		
		input = huh.NewInput().
			Title(fmt.Sprintf("Edit %s", configKey)).
			Description(description).
			Placeholder(currentValue).
			Value(&model.newValue).
			Validate(func(s string) error {
				// Allow -1 for unlimited values
				if s == "-1" {
					return nil
				}
				// Allow empty to keep current value
				if s == "" {
					return nil
				}
				// More validation could be added here
				return nil
			})

	default:
		// Default text input for other fields
		input = huh.NewInput().
			Title(fmt.Sprintf("Edit %s", configKey)).
			Description(fmt.Sprintf("Current value: %s", currentValue)).
			Placeholder(currentValue).
			Value(&model.newValue)
	}

	model.form = huh.NewForm(
		huh.NewGroup(input),
	).WithShowHelp(false)

	return model
}

func (m *EditConfigModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m *EditConfigModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	log := logger.Get()
	
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch s := msg.String(); s {
		case "esc":
			// Cancel editing and return to list view
			return m, ReturnToListView
		}
	}

	// Let the form handle the update
	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f

		// Check if form is complete or aborted
		switch m.form.State {
		case huh.StateCompleted:
			// Log immediately when form is completed
			log.WithFields(map[string]interface{}{
				"topic":        m.topicName,
				"key":          m.configKey,
				"currentValue": m.currentValue,
				"newValue":     m.newValue,
				"formState":    "completed",
			}).Debug("Form completed, checking for changes")
			
			// If newValue is empty, it means user didn't change anything (for text inputs)
			// or pressed enter without selecting (for selects)
			if m.newValue == "" || m.newValue == m.currentValue {
				// No change, just return
				log.Debug("No change detected, returning to list view")
				return m, ReturnToListView
			}

			// Apply the configuration change to Kafka
			log.WithFields(map[string]interface{}{
				"topic":    m.topicName,
				"key":      m.configKey,
				"oldValue": m.currentValue,
				"newValue": m.newValue,
			}).Info("Applying configuration change")
			
			err := m.client.UpdateTopicConfig(m.topicName, m.configKey, m.newValue)
			if err != nil {
				m.err = err
				log.WithError(err).Error("Failed to update configuration")
				// Show error for longer so user can read it
				return m, tea.Sequence(
					tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
						return SwitchToListViewMsg{}
					}),
				)
			}
			m.submitted = true
			log.Info("Configuration updated successfully")
			// Show success message for a bit longer
			return m, tea.Sequence(
				tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
					return SwitchToListViewMsg{}
				}),
			)
		case huh.StateAborted:
			// User cancelled, return to list view
			return m, ReturnToListView
		}
	}

	return m, cmd
}

func (m *EditConfigModel) View() string {
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("196")).
			Padding(1, 2)

		content := fmt.Sprintf("❌ ERROR UPDATING CONFIGURATION\n\n%v\n\nTopic: %s\nKey: %s\nAttempted Value: %s\n\nWaiting 5 seconds before returning...",
			m.err, m.topicName, m.configKey, m.newValue)
		return "\n" + errorStyle.Render(content)
	}

	if m.submitted {
		successStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("46")).
			Bold(true).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("46")).
			Padding(1, 2)

		content := fmt.Sprintf("✅ CONFIGURATION UPDATED SUCCESSFULLY!\n\nTopic: %s\nKey: %s\nOld Value: %s\nNew Value: %s\n\nReturning to list...",
			m.topicName, m.configKey, m.currentValue, m.newValue)
		return "\n" + successStyle.Render(content)
	}

	return fmt.Sprintf("\n%s\n", m.form.View())
}
