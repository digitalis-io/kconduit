package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/digitalis-io/kconduit/pkg/kafka"
	"github.com/digitalis-io/kconduit/pkg/logger"
)

type AIProvider int

const (
	OpenAI AIProvider = iota
	Gemini
	Anthropic
	Ollama
)

// aiRequestTimeout bounds a single AI provider call. Reasoning models
// (e.g. Gemini 3.x Pro) routinely take 30s+ with the full system prompt.
const aiRequestTimeout = 120 * time.Second

const aiSystemPrompt = `You are a Kafka assistant. Convert natural language commands into specific Kafka operations.

For creating topics, respond with JSON:
{"action": "create_topic", "name": "topic-name", "partitions": 3, "replication_factor": 1, "configs": {"compression.type": "gzip"}}

For modifying topic partitions, respond with JSON:
{"action": "modify_partitions", "topic": "topic-name", "partitions": 10}

For modifying partitions on ALL topics, respond with JSON:
{"action": "modify_all_partitions", "partitions": 100}

For modifying topic configurations (like compression, retention, etc.), respond with JSON:
{"action": "modify_config", "topic": "topic-name", "configs": {"compression.type": "snappy", "retention.ms": "86400000"}}

For modifying configurations on ALL topics, respond with JSON:
{"action": "modify_all_configs", "configs": {"compression.type": "gzip", "retention.ms": "604800000"}}

For modifying configurations on topics matching a pattern, respond with JSON:
{"action": "modify_matching_configs", "pattern": "starts_with:random", "configs": {"compression.type": "lz4"}}
or
{"action": "modify_matching_configs", "pattern": "contains:events", "configs": {"retention.ms": "86400000"}}
or
{"action": "modify_matching_configs", "pattern": "ends_with:log", "configs": {"compression.type": "gzip"}}

For querying consumer groups (find groups with lag, list groups, etc.), respond with JSON:
{"action": "query_consumer_groups", "filter": {"lag_greater_than": 10}}
or
{"action": "query_consumer_groups", "filter": {"group_id_contains": "my-group"}}
or
{"action": "query_consumer_groups", "filter": {"state": "Stable"}}

For querying topics (list topics with specific configurations), respond with JSON:
{"action": "query_topics", "filter": {"compression": "none"}}
or
{"action": "query_topics", "filter": {"partitions_greater_than": 10}}
or
{"action": "query_topics", "filter": {"name_contains": "events"}}
or
{"action": "query_topics", "filter": {"replication_factor": 3}}

For ACL operations:

To create an ACL, respond with JSON:
{"action": "create_acl", "principal": "User:alice", "host": "*", "resource_type": "Topic", "resource_name": "my-topic", "pattern_type": "Literal", "operation": "Read", "permission_type": "Allow"}

To create multiple ACLs at once:
{"action": "create_acls", "acls": [{"principal": "User:alice", "host": "*", "resource_type": "Topic", "resource_name": "my-topic", "pattern_type": "Literal", "operation": "Read", "permission_type": "Allow"}, {"principal": "User:alice", "host": "*", "resource_type": "Topic", "resource_name": "my-topic", "pattern_type": "Literal", "operation": "Write", "permission_type": "Allow"}]}

To delete an ACL, respond with JSON:
{"action": "delete_acl", "principal": "User:alice", "host": "*", "resource_type": "Topic", "resource_name": "my-topic", "pattern_type": "Literal", "operation": "Read", "permission_type": "Allow"}

To query/list ACLs, respond with JSON:
{"action": "query_acls", "filter": {"principal": "User:alice"}}
or
{"action": "query_acls", "filter": {"resource_type": "Topic", "resource_name": "my-topic"}}
or
{"action": "query_acls", "filter": {}} (for all ACLs)

Valid resource types: Topic, Group, Cluster, TransactionalId
Valid operations: Read, Write, Create, Delete, Alter, Describe, ClusterAction, DescribeConfigs, AlterConfigs, IdempotentWrite, All
Valid permission types: Allow, Deny
Valid pattern types: Literal, Prefixed

Always respond with ONLY the appropriate JSON for the requested operation. Do NOT include explanations, markdown formatting, or multiple JSON blocks. Return a single, clean JSON object that can be directly executed.

If it requires multiple steps, ensure they are in the right order and all necessary fields are included.

Refuse to perform any actions that are not related to Kafka. Never delete anything.`

type AIConfig struct {
	OpenAIKey      string
	OpenAIModel    string
	GeminiKey      string
	GeminiModel    string
	AnthropicKey   string
	AnthropicModel string
	OllamaURL      string
	OllamaModel    string
}

type AIAssistantModel struct {
	client       *kafka.Client
	textarea     textarea.Model
	viewport     viewport.Model
	provider     AIProvider
	config       AIConfig
	processing   bool
	response     string
	err          error
	width        int
	height       int
	showResponse bool
}

func NewAIAssistantModel(client *kafka.Client, aiEngine string, aiModel string) AIAssistantModel {
	ta := textarea.New()
	ta.Placeholder = "Enter your Kafka command in natural language...\nExamples: 'Create a topic named my-new-topic with 3 partitions' or 'Give user alice read access to topic events'"
	ta.Focus()
	ta.CharLimit = 500
	ta.SetWidth(100)
	ta.SetHeight(4)

	vp := viewport.New(100, 15)
	vp.SetContent("")
	vp.Style = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62"))

	// Load configuration from environment variables
	config := AIConfig{
		OpenAIKey:      getEnv("OPENAI_API_KEY", ""),
		OpenAIModel:    getEnv("OPENAI_MODEL", "gpt-3.5-turbo"),
		GeminiKey:      getFirstEnv("", "GEMINI_API_KEY", "GOOGLE_API_KEY"),
		GeminiModel:    getEnv("GEMINI_MODEL", "gemini-flash-latest"),
		AnthropicKey:   getEnv("ANTHROPIC_API_KEY", ""),
		AnthropicModel: getEnv("ANTHROPIC_MODEL", "claude-3-haiku-20240307"),
		OllamaURL:      getEnv("OLLAMA_URL", "http://localhost:11434"),
		OllamaModel:    getEnv("OLLAMA_MODEL", "llama2"),
	}

	// Override with command-line arguments if provided
	if aiModel != "" {
		// Apply model override based on engine
		switch aiEngine {
		case "openai":
			config.OpenAIModel = aiModel
		case "gemini":
			config.GeminiModel = aiModel
		case "anthropic":
			config.AnthropicModel = aiModel
		case "ollama":
			config.OllamaModel = aiModel
		}
	}

	// Determine provider based on aiEngine parameter or available keys
	defaultProvider := OpenAI
	if aiEngine != "" {
		switch aiEngine {
		case "openai":
			defaultProvider = OpenAI
		case "gemini":
			defaultProvider = Gemini
		case "anthropic":
			defaultProvider = Anthropic
		case "ollama":
			defaultProvider = Ollama
		}
	} else {
		// Fall back to auto-detection based on available keys
		if config.OpenAIKey == "" && config.GeminiKey != "" {
			defaultProvider = Gemini
		} else if config.OpenAIKey == "" && config.GeminiKey == "" && config.AnthropicKey != "" {
			defaultProvider = Anthropic
		} else if config.OpenAIKey == "" && config.GeminiKey == "" && config.AnthropicKey == "" {
			defaultProvider = Ollama
		}
	}

	return AIAssistantModel{
		client:   client,
		textarea: ta,
		viewport: vp,
		provider: defaultProvider,
		config:   config,
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getFirstEnv returns the first of keys that is set to a non-empty value, or
// defaultValue if none is.
//
// Gemini credentials reach a machine under either name: GEMINI_API_KEY is what
// Google's own Gemini tooling uses, GOOGLE_API_KEY what the wider Google Cloud
// SDKs set. Accepting both means a shell already set up for one does not have
// to be re-exported for the other. The order is the preference: an explicit
// GEMINI_API_KEY wins over a general-purpose GOOGLE_API_KEY.
func getFirstEnv(defaultValue string, keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return defaultValue
}

// wrapText wraps text to fit within the specified width
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		if len(line) <= width {
			result.WriteString(line)
			result.WriteString("\n")
			continue
		}

		// Wrap long lines
		words := strings.Fields(line)
		currentLine := ""

		for _, word := range words {
			if len(currentLine)+len(word)+1 > width {
				if currentLine != "" {
					result.WriteString(currentLine)
					result.WriteString("\n")
				}
				currentLine = word
			} else {
				if currentLine != "" {
					currentLine += " "
				}
				currentLine += word
			}
		}

		if currentLine != "" {
			result.WriteString(currentLine)
			result.WriteString("\n")
		}
	}

	return strings.TrimRight(result.String(), "\n")
}

func (m AIAssistantModel) Init() tea.Cmd {
	return textarea.Blink
}

type AIResponseMsg struct {
	response string
	err      error
}

func (m AIAssistantModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			if m.showResponse {
				// Go back to input
				m.showResponse = false
				m.textarea.Focus()
				return m, textarea.Blink
			}
			// Exit AI assistant
			return m, ReturnToListView

		case tea.KeyCtrlC:
			return m, ReturnToListView

		case tea.KeyEnter:
			if !m.processing && !m.showResponse {
				if m.textarea.Value() != "" {
					m.processing = true
					query := m.textarea.Value()
					return m, m.processAIQuery(query)
				}
			}

		case tea.KeyTab:
			// Cycle through providers
			switch m.provider {
			case OpenAI:
				m.provider = Gemini
			case Gemini:
				m.provider = Anthropic
			case Anthropic:
				m.provider = Ollama
			case Ollama:
				m.provider = OpenAI
			}
			return m, nil
		}

	case AIResponseMsg:
		m.processing = false
		if msg.err != nil {
			m.err = msg.err
			m.response = fmt.Sprintf("Error: %v", msg.err)
		} else {
			// Format the response with proper wrapping
			m.response = wrapText(msg.response, m.viewport.Width-4)
			m.err = nil
			// Try to execute the command
			if cmd := m.parseAndExecuteCommand(msg.response); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		m.viewport.SetContent(m.response)
		m.showResponse = true
		m.viewport.GotoTop()
		return m, tea.Batch(cmds...)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Use almost full width with some padding
		textWidth := msg.Width - 8
		if textWidth > 120 {
			textWidth = 120 // Cap max width for readability
		}
		m.textarea.SetWidth(textWidth)
		m.viewport.Width = textWidth
		// Use more vertical space for response
		m.viewport.Height = msg.Height - 15
		if m.viewport.Height > 40 {
			m.viewport.Height = 40 // Cap max height
		}
	}

	// Update the appropriate component
	if m.showResponse {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m AIAssistantModel) View() string {
	var s strings.Builder

	providerText := m.getProviderName()
	modelText := m.getCurrentModel()

	s.WriteString(appHeaderStyle.Render("AI Assistant"))
	s.WriteString("  ")
	s.WriteString(labelStyle.Render(providerText))
	s.WriteString(helpDescStyle.Render(" / "))
	s.WriteString(labelStyle.Render(modelText))
	s.WriteString("\n\n")

	// Provider info panel
	apiKeyStatus := m.getAPIKeyStatus()
	var statusLine string
	if apiKeyStatus == "Configured" {
		statusLine = successStyle.Render("ready")
	} else {
		statusLine = warningStyle.Render(apiKeyStatus)
	}

	boxWidth := m.width - 10
	if boxWidth > 100 {
		boxWidth = 100
	}
	if boxWidth < 60 {
		boxWidth = 60
	}

	info := labelStyle.Render("Provider ") + valueStyle.Render(providerText) +
		labelStyle.Render("  Model ") + valueStyle.Render(modelText) +
		labelStyle.Render("  API ") + statusLine

	s.WriteString(panelStyle.Padding(0, 2).Width(boxWidth).Render(info))
	s.WriteString("\n\n")

	// Show input or response
	if m.showResponse {
		s.WriteString(sectionTitleStyle.Render("Response"))

		if m.viewport.TotalLineCount() > m.viewport.Height {
			s.WriteString("  ")
			s.WriteString(helpDescStyle.Render(fmt.Sprintf("line %d/%d",
				m.viewport.YOffset+1, m.viewport.TotalLineCount())))
		}
		s.WriteString("\n\n")
		s.WriteString(m.viewport.View())
		s.WriteString("\n\n")
		s.WriteString(renderHelpBar("esc", "new query", "ctrl+c", "exit"))
	} else {
		s.WriteString(m.textarea.View())
		s.WriteString("\n\n")

		if m.processing {
			s.WriteString(warningStyle.Bold(true).Render("Processing..."))
		} else {
			available := m.getAvailableProviders()
			s.WriteString(renderHelpBar("enter", "send", "tab", "provider ("+available+")", "esc", "exit"))
		}
	}

	return s.String()
}

func (m AIAssistantModel) getProviderName() string {
	switch m.provider {
	case OpenAI:
		return "OpenAI"
	case Gemini:
		return "Gemini"
	case Anthropic:
		return "Anthropic"
	case Ollama:
		return "Ollama"
	default:
		return "Unknown"
	}
}

func (m AIAssistantModel) getCurrentModel() string {
	switch m.provider {
	case OpenAI:
		return m.config.OpenAIModel
	case Gemini:
		return m.config.GeminiModel
	case Anthropic:
		return m.config.AnthropicModel
	case Ollama:
		return m.config.OllamaModel
	default:
		return "unknown"
	}
}

func (m AIAssistantModel) getAPIKeyStatus() string {
	switch m.provider {
	case OpenAI:
		if m.config.OpenAIKey != "" {
			return "Configured"
		}
		return "API key not set (OPENAI_API_KEY)"
	case Gemini:
		if m.config.GeminiKey != "" {
			return "Configured"
		}
		return "API key not set (GEMINI_API_KEY or GOOGLE_API_KEY)"
	case Anthropic:
		if m.config.AnthropicKey != "" {
			return "Configured"
		}
		return "API key not set (ANTHROPIC_API_KEY)"
	case Ollama:
		// Check if Ollama is running by attempting a simple request
		return "Local (no API key needed)"
	default:
		return "Unknown"
	}
}

func (m AIAssistantModel) getAvailableProviders() string {
	var available []string

	if m.config.OpenAIKey != "" {
		available = append(available, "OpenAI✓")
	} else {
		available = append(available, "OpenAI")
	}

	if m.config.GeminiKey != "" {
		available = append(available, "Gemini✓")
	} else {
		available = append(available, "Gemini")
	}

	if m.config.AnthropicKey != "" {
		available = append(available, "Anthropic✓")
	} else {
		available = append(available, "Anthropic")
	}

	available = append(available, "Ollama")

	// Highlight current provider
	for i, p := range available {
		switch m.provider {
		case OpenAI:
			if strings.HasPrefix(p, "OpenAI") {
				available[i] = "[" + p + "]"
			}
		case Gemini:
			if strings.HasPrefix(p, "Gemini") {
				available[i] = "[" + p + "]"
			}
		case Anthropic:
			if strings.HasPrefix(p, "Anthropic") {
				available[i] = "[" + p + "]"
			}
		case Ollama:
			if p == "Ollama" {
				available[i] = "[" + p + "]"
			}
		}
	}

	return strings.Join(available, " → ")
}

func (m *AIAssistantModel) processAIQuery(query string) tea.Cmd {
	return func() tea.Msg {
		var response string
		var err error

		switch m.provider {
		case OpenAI:
			response, err = m.queryOpenAI(query)
		case Gemini:
			response, err = m.queryGemini(query)
		case Anthropic:
			response, err = m.queryAnthropic(query)
		case Ollama:
			response, err = m.queryOllama(query)
		default:
			err = fmt.Errorf("unsupported AI provider")
		}

		return AIResponseMsg{response: response, err: err}
	}
}

func (m *AIAssistantModel) queryOpenAI(query string) (string, error) {
	if m.config.OpenAIKey == "" {
		return "", fmt.Errorf("openAI API key not configured; set OPENAI_API_KEY environment variable")
	}

	requestBody := map[string]interface{}{
		"model": m.config.OpenAIModel,
		"messages": []map[string]string{
			{"role": "system", "content": aiSystemPrompt},
			{"role": "user", "content": query},
		},
		"temperature": 0.3,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.config.OpenAIKey)

	client := &http.Client{Timeout: aiRequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Body close errors are typically safe to ignore in HTTP clients
			_ = err
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	choices, ok := result["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return "", fmt.Errorf("unexpected API response format")
	}

	firstChoice, ok := choices[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("unexpected OpenAI response: invalid choice structure")
	}
	message, ok := firstChoice["message"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("unexpected OpenAI response: invalid message structure")
	}
	content, ok := message["content"].(string)
	if !ok {
		return "", fmt.Errorf("unexpected OpenAI response: content is not a string")
	}

	return content, nil
}

func (m *AIAssistantModel) queryGemini(query string) (string, error) {
	if m.config.GeminiKey == "" {
		return "", fmt.Errorf("gemini API key not configured; set GEMINI_API_KEY or GOOGLE_API_KEY")
	}

	fullPrompt := aiSystemPrompt + "\n\nUser: " + query

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent",
		m.config.GeminiModel)

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": fullPrompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature": 0.3,
			// Thinking tokens count against this limit on Gemini 2.5+/3.x.
			"maxOutputTokens": 8192,
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", m.config.GeminiKey)

	client := &http.Client{Timeout: aiRequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Body close errors are typically safe to ignore in HTTP clients
			_ = err
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	// Parse Gemini response
	candidates, ok := result["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		return "", fmt.Errorf("unexpected Gemini API response format")
	}

	firstCandidate, ok := candidates[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("unexpected Gemini response: invalid candidate structure")
	}
	content, ok := firstCandidate["content"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("unexpected Gemini response structure")
	}

	parts, ok := content["parts"].([]interface{})
	if !ok || len(parts) == 0 {
		return "", fmt.Errorf("no content parts in Gemini response")
	}

	firstPart, ok := parts[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("unexpected Gemini response: invalid part structure")
	}
	text, ok := firstPart["text"].(string)
	if !ok {
		return "", fmt.Errorf("no text in Gemini response part")
	}

	return text, nil
}

func (m *AIAssistantModel) queryAnthropic(query string) (string, error) {
	if m.config.AnthropicKey == "" {
		return "", fmt.Errorf("anthropic API key not configured; set ANTHROPIC_API_KEY environment variable")
	}

	requestBody := map[string]interface{}{
		"model":      m.config.AnthropicModel,
		"max_tokens": 2048,
		"messages": []map[string]string{
			{"role": "user", "content": query},
		},
		"system":      aiSystemPrompt,
		"temperature": 0.3,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", m.config.AnthropicKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: aiRequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Body close errors are typically safe to ignore in HTTP clients
			_ = err
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	// Parse Anthropic response
	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		return "", fmt.Errorf("unexpected Anthropic API response format")
	}

	firstContent, ok := content[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("unexpected Anthropic response: invalid content structure")
	}
	text, ok := firstContent["text"].(string)
	if !ok {
		return "", fmt.Errorf("no text in Anthropic response")
	}

	return text, nil
}

func (m *AIAssistantModel) queryOllama(query string) (string, error) {
	requestBody := map[string]interface{}{
		"model":  m.config.OllamaModel,
		"prompt": aiSystemPrompt + "\n\nUser: " + query + "\n\nAssistant:",
		"stream": false,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", m.config.OllamaURL+"/api/generate", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: aiRequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to connect to Ollama. Make sure it's running: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Body close errors are typically safe to ignore in HTTP clients
			_ = err
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	response, ok := result["response"].(string)
	if !ok {
		return "", fmt.Errorf("unexpected Ollama response format")
	}

	return response, nil
}

func (m *AIAssistantModel) executeMultipleCommands(commands []map[string]interface{}) tea.Cmd {
	log := logger.Get()

	return func() tea.Msg {
		var responses []string

		for i, command := range commands {
			action, ok := command["action"].(string)
			if !ok {
				continue
			}

			log.WithField("action", action).WithField("step", i+1).Info("Executing command")

			// Execute each command synchronously
			var result string
			var err error

			switch action {
			case "modify_partitions":
				topic, _ := command["topic"].(string)
				partitions, _ := command["partitions"].(float64)

				if topic != "" && partitions > 0 {
					err = m.client.ModifyTopicPartitions(topic, int32(partitions))
					if err != nil {
						result = fmt.Sprintf("❌ Failed to modify partitions for %s: %v", topic, err)
					} else {
						result = fmt.Sprintf("✅ Successfully increased partitions for '%s' to %d", topic, int(partitions))
					}
				}

			case "modify_config":
				topic, _ := command["topic"].(string)
				configs, _ := command["configs"].(map[string]interface{})

				if topic != "" && configs != nil {
					var configChanges []string
					var configErrors []string

					for key, value := range configs {
						if strValue, ok := value.(string); ok {
							if err := m.client.UpdateTopicConfig(topic, key, strValue); err != nil {
								configErrors = append(configErrors, fmt.Sprintf("%s: %v", key, err))
							} else {
								configChanges = append(configChanges, fmt.Sprintf("%s=%s", key, strValue))
							}
						}
					}

					if len(configErrors) > 0 {
						result = fmt.Sprintf("⚠️ Partially updated '%s'. Success: %s, Failed: %s",
							topic, strings.Join(configChanges, ", "), strings.Join(configErrors, ", "))
					} else if len(configChanges) > 0 {
						result = fmt.Sprintf("✅ Successfully updated '%s': %s", topic, strings.Join(configChanges, ", "))
					}
				}

			case "create_topic":
				name, _ := command["name"].(string)
				partitions, _ := command["partitions"].(float64)
				replicationFactor, _ := command["replication_factor"].(float64)

				if name != "" {
					err = m.client.CreateTopic(name, int32(partitions), int16(replicationFactor))
					if err != nil {
						result = fmt.Sprintf("❌ Failed to create topic %s: %v", name, err)
					} else {
						result = fmt.Sprintf("✅ Successfully created topic '%s'", name)

						// Apply configs if any
						if configs, ok := command["configs"].(map[string]interface{}); ok {
							for key, value := range configs {
								if strValue, ok := value.(string); ok {
									if err := m.client.UpdateTopicConfig(name, key, strValue); err != nil {
										// Log error but continue processing other configs
										fmt.Printf("Failed to update config %s for topic %s: %v\n", key, name, err)
									}
								}
							}
						}
					}
				}

			case "create_acl":
				principal, _ := command["principal"].(string)
				host, _ := command["host"].(string)
				resourceType, _ := command["resource_type"].(string)
				resourceName, _ := command["resource_name"].(string)
				patternType, _ := command["pattern_type"].(string)
				operation, _ := command["operation"].(string)
				permissionType, _ := command["permission_type"].(string)

				if principal != "" && resourceType != "" && resourceName != "" {
					acl := kafka.ACL{
						Principal:      principal,
						Host:           host,
						ResourceType:   resourceType,
						ResourceName:   resourceName,
						PatternType:    patternType,
						Operation:      operation,
						PermissionType: permissionType,
					}

					err = m.client.CreateACL(acl)
					if err != nil {
						result = fmt.Sprintf("❌ Failed to create ACL: %v", err)
					} else {
						result = fmt.Sprintf("✅ Created ACL: %s on %s %s (%s %s)",
							principal, resourceType, resourceName, operation, permissionType)
					}
				}

			case "delete_acl":
				principal, _ := command["principal"].(string)
				host, _ := command["host"].(string)
				resourceType, _ := command["resource_type"].(string)
				resourceName, _ := command["resource_name"].(string)
				patternType, _ := command["pattern_type"].(string)
				operation, _ := command["operation"].(string)
				permissionType, _ := command["permission_type"].(string)

				if principal != "" && resourceType != "" && resourceName != "" {
					acl := kafka.ACL{
						Principal:      principal,
						Host:           host,
						ResourceType:   resourceType,
						ResourceName:   resourceName,
						PatternType:    patternType,
						Operation:      operation,
						PermissionType: permissionType,
					}

					err = m.client.DeleteACL(acl)
					if err != nil {
						result = fmt.Sprintf("❌ Failed to delete ACL: %v", err)
					} else {
						result = fmt.Sprintf("✅ Deleted ACL: %s on %s %s (%s %s)",
							principal, resourceType, resourceName, operation, permissionType)
					}
				}
			}

			if result != "" {
				responses = append(responses, fmt.Sprintf("Step %d: %s", i+1, result))
			}
		}

		// Combine all responses
		finalResponse := strings.Join(responses, "\n")
		if finalResponse == "" {
			finalResponse = "No actions were executed"
		}

		return AIResponseMsg{
			response: finalResponse,
			err:      nil,
		}
	}
}

func (m *AIAssistantModel) parseAndExecuteCommand(response string) tea.Cmd {
	log := logger.Get()

	// Remove markdown code block markers if present
	response = strings.ReplaceAll(response, "```json", "")
	response = strings.ReplaceAll(response, "```", "")

	// Find all JSON objects in the response
	var commands []map[string]interface{}
	remaining := response

	for {
		start := strings.Index(remaining, "{")
		if start == -1 {
			break
		}

		// Find the matching closing brace
		end := -1
		braceCount := 0
		for i := start; i < len(remaining); i++ {
			if remaining[i] == '{' {
				braceCount++
			} else if remaining[i] == '}' {
				braceCount--
				if braceCount == 0 {
					end = i
					break
				}
			}
		}

		if end == -1 {
			break
		}

		jsonStr := remaining[start : end+1]
		var command map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &command); err == nil {
			if _, ok := command["action"]; ok {
				commands = append(commands, command)
			}
		}

		remaining = remaining[end+1:]
	}

	// If no commands found, return nil
	if len(commands) == 0 {
		log.Debug("No valid JSON commands found in AI response")
		return nil
	}

	// Log the commands found
	log.WithField("count", len(commands)).Info("Found JSON commands in AI response")

	// If there are multiple commands, execute them all in sequence
	if len(commands) > 1 {
		return m.executeMultipleCommands(commands)
	}

	// Single command execution
	command := commands[0]
	action, ok := command["action"].(string)
	if !ok {
		log.Debug("No action field found in JSON command")
		return nil
	}

	log.WithField("action", action).Info("Executing AI command")

	switch action {
	case "create_topic":
		name, _ := command["name"].(string)
		partitions, _ := command["partitions"].(float64)
		replicationFactor, _ := command["replication_factor"].(float64)

		if name != "" {
			// Execute the topic creation
			return func() tea.Msg {
				err := m.client.CreateTopic(name, int32(partitions), int16(replicationFactor))
				if err != nil {
					return AIResponseMsg{
						response: fmt.Sprintf("Failed to create topic: %v", err),
						err:      err,
					}
				}

				// Apply configurations if any
				if configs, ok := command["configs"].(map[string]interface{}); ok {
					for key, value := range configs {
						if strValue, ok := value.(string); ok {
							if err := m.client.UpdateTopicConfig(name, key, strValue); err != nil {
								log.WithError(err).Warn("Failed to apply config")
							}
						}
					}
				}

				return AIResponseMsg{
					response: fmt.Sprintf("✅ Successfully created topic '%s' with %d partitions and replication factor %d",
						name, int(partitions), int(replicationFactor)),
					err: nil,
				}
			}
		}

	case "modify_partitions":
		topic, _ := command["topic"].(string)
		partitions, _ := command["partitions"].(float64)

		if topic != "" && partitions > 0 {
			return func() tea.Msg {
				err := m.client.ModifyTopicPartitions(topic, int32(partitions))
				if err != nil {
					return AIResponseMsg{
						response: fmt.Sprintf("❌ Failed to modify partitions: %v", err),
						err:      err,
					}
				}

				return AIResponseMsg{
					response: fmt.Sprintf("✅ Successfully increased partitions for topic '%s' to %d",
						topic, int(partitions)),
					err: nil,
				}
			}
		}

	case "modify_config":
		topic, _ := command["topic"].(string)
		configs, _ := command["configs"].(map[string]interface{})

		if topic != "" && configs != nil {
			return func() tea.Msg {
				var configChanges []string
				var errors []string

				for key, value := range configs {
					if strValue, ok := value.(string); ok {
						if err := m.client.UpdateTopicConfig(topic, key, strValue); err != nil {
							errors = append(errors, fmt.Sprintf("%s: %v", key, err))
							log.WithError(err).WithField("config", key).Warn("Failed to apply config")
						} else {
							configChanges = append(configChanges, fmt.Sprintf("%s=%s", key, strValue))
						}
					}
				}

				if len(errors) > 0 {
					return AIResponseMsg{
						response: fmt.Sprintf("⚠️ Partially updated topic '%s'.\nSuccessful: %s\nFailed: %s",
							topic,
							strings.Join(configChanges, ", "),
							strings.Join(errors, ", ")),
						err: nil,
					}
				}

				return AIResponseMsg{
					response: fmt.Sprintf("✅ Successfully updated configuration for topic '%s':\n%s",
						topic, strings.Join(configChanges, "\n")),
					err: nil,
				}
			}
		}

	case "modify_all_partitions":
		partitions, _ := command["partitions"].(float64)

		if partitions > 0 {
			return func() tea.Msg {
				// Get all topics
				topics, err := m.client.GetTopicDetails()
				if err != nil {
					return AIResponseMsg{
						response: fmt.Sprintf("❌ Failed to fetch topics: %v", err),
						err:      err,
					}
				}

				var successes []string
				var failures []string

				for _, topic := range topics {
					// Only increase partitions (Kafka doesn't allow decreasing)
					if topic.Partitions < int(partitions) {
						err := m.client.ModifyTopicPartitions(topic.Name, int32(partitions))
						if err != nil {
							failures = append(failures, fmt.Sprintf("%s: %v", topic.Name, err))
							log.WithField("topic", topic.Name).WithError(err).Warn("Failed to modify partitions")
						} else {
							successes = append(successes, fmt.Sprintf("%s (%d→%d)", topic.Name, topic.Partitions, int(partitions)))
						}
					} else {
						// Skip topics that already have enough partitions
						log.WithField("topic", topic.Name).Debug("Topic already has sufficient partitions")
					}
				}

				// Format response
				var response strings.Builder
				if len(successes) > 0 {
					fmt.Fprintf(&response, "✅ Successfully updated %d topic(s):\n", len(successes))
					for _, s := range successes {
						fmt.Fprintf(&response, "  • %s\n", s)
					}
				}

				if len(failures) > 0 {
					if len(successes) > 0 {
						response.WriteString("\n")
					}
					fmt.Fprintf(&response, "❌ Failed to update %d topic(s):\n", len(failures))
					for _, f := range failures {
						fmt.Fprintf(&response, "  • %s\n", f)
					}
				}

				if len(successes) == 0 && len(failures) == 0 {
					fmt.Fprintf(&response, "ℹ️ All topics already have %d or more partitions", int(partitions))
				}

				return AIResponseMsg{
					response: response.String(),
					err:      nil,
				}
			}
		}

	case "modify_matching_configs":
		pattern, _ := command["pattern"].(string)
		configs, _ := command["configs"].(map[string]interface{})

		if pattern != "" && configs != nil {
			return func() tea.Msg {
				// Parse the pattern
				var matchFunc func(string) bool
				if strings.HasPrefix(pattern, "starts_with:") {
					prefix := strings.TrimPrefix(pattern, "starts_with:")
					matchFunc = func(name string) bool {
						return strings.HasPrefix(name, prefix)
					}
				} else if strings.HasPrefix(pattern, "contains:") {
					substr := strings.TrimPrefix(pattern, "contains:")
					matchFunc = func(name string) bool {
						return strings.Contains(name, substr)
					}
				} else if strings.HasPrefix(pattern, "ends_with:") {
					suffix := strings.TrimPrefix(pattern, "ends_with:")
					matchFunc = func(name string) bool {
						return strings.HasSuffix(name, suffix)
					}
				} else {
					// Default to exact match
					matchFunc = func(name string) bool {
						return name == pattern
					}
				}

				// Get all topics
				topics, err := m.client.GetTopicDetails()
				if err != nil {
					return AIResponseMsg{
						response: fmt.Sprintf("❌ Failed to fetch topics: %v", err),
						err:      err,
					}
				}

				var topicResults []string
				var topicErrors []string
				var matchedCount int

				for _, topic := range topics {
					// Check if topic matches the pattern
					if !matchFunc(topic.Name) {
						continue
					}

					matchedCount++
					var configChanges []string
					var configErrors []string

					for key, value := range configs {
						if strValue, ok := value.(string); ok {
							if err := m.client.UpdateTopicConfig(topic.Name, key, strValue); err != nil {
								configErrors = append(configErrors, fmt.Sprintf("%s: %v", key, err))
								log.WithField("topic", topic.Name).WithField("config", key).WithError(err).Warn("Failed to apply config")
							} else {
								configChanges = append(configChanges, fmt.Sprintf("%s=%s", key, strValue))
							}
						}
					}

					if len(configChanges) > 0 {
						topicResults = append(topicResults, fmt.Sprintf("%s: %s", topic.Name, strings.Join(configChanges, ", ")))
					}

					if len(configErrors) > 0 {
						topicErrors = append(topicErrors, fmt.Sprintf("%s: %s", topic.Name, strings.Join(configErrors, ", ")))
					}
				}

				// Format response
				var response strings.Builder
				if matchedCount == 0 {
					fmt.Fprintf(&response, "ℹ️ No topics found matching pattern '%s'\n", pattern)
				} else {
					if len(topicResults) > 0 {
						fmt.Fprintf(&response, "✅ Successfully updated configuration for %d topic(s) matching '%s':\n", len(topicResults), pattern)
						for _, result := range topicResults {
							fmt.Fprintf(&response, "  • %s\n", result)
						}
					}

					if len(topicErrors) > 0 {
						if len(topicResults) > 0 {
							response.WriteString("\n")
						}
						fmt.Fprintf(&response, "❌ Failed to update configuration for %d topic(s):\n", len(topicErrors))
						for _, err := range topicErrors {
							fmt.Fprintf(&response, "  • %s\n", err)
						}
					}
				}

				return AIResponseMsg{
					response: response.String(),
					err:      nil,
				}
			}
		}

	case "modify_all_configs":
		configs, _ := command["configs"].(map[string]interface{})

		if configs != nil {
			return func() tea.Msg {
				// Get all topics
				topics, err := m.client.GetTopicDetails()
				if err != nil {
					return AIResponseMsg{
						response: fmt.Sprintf("❌ Failed to fetch topics: %v", err),
						err:      err,
					}
				}

				var topicResults []string
				var topicErrors []string

				for _, topic := range topics {
					var configChanges []string
					var configErrors []string

					for key, value := range configs {
						if strValue, ok := value.(string); ok {
							if err := m.client.UpdateTopicConfig(topic.Name, key, strValue); err != nil {
								configErrors = append(configErrors, fmt.Sprintf("%s: %v", key, err))
								log.WithField("topic", topic.Name).WithField("config", key).WithError(err).Warn("Failed to apply config")
							} else {
								configChanges = append(configChanges, fmt.Sprintf("%s=%s", key, strValue))
							}
						}
					}

					if len(configChanges) > 0 {
						topicResults = append(topicResults, fmt.Sprintf("%s: %s", topic.Name, strings.Join(configChanges, ", ")))
					}

					if len(configErrors) > 0 {
						topicErrors = append(topicErrors, fmt.Sprintf("%s: %s", topic.Name, strings.Join(configErrors, ", ")))
					}
				}

				// Format response
				var response strings.Builder
				if len(topicResults) > 0 {
					fmt.Fprintf(&response, "✅ Successfully updated configuration for %d topic(s):\n", len(topicResults))
					for _, result := range topicResults {
						fmt.Fprintf(&response, "  • %s\n", result)
					}
				}

				if len(topicErrors) > 0 {
					if len(topicResults) > 0 {
						response.WriteString("\n")
					}
					fmt.Fprintf(&response, "❌ Failed to update configuration for %d topic(s):\n", len(topicErrors))
					for _, err := range topicErrors {
						fmt.Fprintf(&response, "  • %s\n", err)
					}
				}

				return AIResponseMsg{
					response: response.String(),
					err:      nil,
				}
			}
		}

	case "query_consumer_groups":
		filter, _ := command["filter"].(map[string]interface{})

		return func() tea.Msg {
			// Get all consumer groups
			groups, err := m.client.GetConsumerGroups()
			if err != nil {
				return AIResponseMsg{
					response: fmt.Sprintf("❌ Failed to fetch consumer groups: %v", err),
					err:      err,
				}
			}

			// Apply filters
			var filteredGroups []kafka.ConsumerGroupInfo
			for _, group := range groups {
				include := true

				// Check lag filter
				if lagThreshold, ok := filter["lag_greater_than"].(float64); ok {
					if group.ConsumerLag <= int64(lagThreshold) {
						include = false
					}
				}

				// Check group ID filter
				if groupIDContains, ok := filter["group_id_contains"].(string); ok {
					if !strings.Contains(group.GroupID, groupIDContains) {
						include = false
					}
				}

				// Check state filter
				if state, ok := filter["state"].(string); ok {
					if !strings.EqualFold(group.State, state) {
						include = false
					}
				}

				if include {
					filteredGroups = append(filteredGroups, group)
				}
			}

			// Format response
			if len(filteredGroups) == 0 {
				return AIResponseMsg{
					response: "No consumer groups found matching the criteria.",
					err:      nil,
				}
			}

			var responseText strings.Builder
			fmt.Fprintf(&responseText, "Found %d consumer group(s):\n\n", len(filteredGroups))

			for _, group := range filteredGroups {
				fmt.Fprintf(&responseText, "📊 Group: %s\n", group.GroupID)
				fmt.Fprintf(&responseText, "   • State: %s\n", group.State)
				fmt.Fprintf(&responseText, "   • Members: %d\n", group.NumMembers)
				fmt.Fprintf(&responseText, "   • Topics: %d\n", group.NumTopics)
				fmt.Fprintf(&responseText, "   • Total Lag: %d\n", group.ConsumerLag)
				if len(group.Topics) > 0 && len(group.Topics) <= 5 {
					fmt.Fprintf(&responseText, "   • Consuming: %s\n", strings.Join(group.Topics, ", "))
				} else if len(group.Topics) > 5 {
					fmt.Fprintf(&responseText, "   • Consuming: %s, ... (%d total)\n",
						strings.Join(group.Topics[:5], ", "), len(group.Topics))
				}
				responseText.WriteString("\n")
			}

			return AIResponseMsg{
				response: responseText.String(),
				err:      nil,
			}
		}

	case "query_topics":
		filter, _ := command["filter"].(map[string]interface{})

		return func() tea.Msg {
			// Get all topics with their configurations
			topics, err := m.client.GetTopicDetails()
			if err != nil {
				return AIResponseMsg{
					response: fmt.Sprintf("❌ Failed to fetch topics: %v", err),
					err:      err,
				}
			}

			// We'll need to fetch configs for filtering
			var filteredTopics []kafka.TopicInfo

			for _, topic := range topics {
				include := true

				// Check name filter
				if nameContains, ok := filter["name_contains"].(string); ok {
					if !strings.Contains(topic.Name, nameContains) {
						include = false
					}
				}

				// Check partitions filter
				if partitionsGT, ok := filter["partitions_greater_than"].(float64); ok {
					if topic.Partitions <= int(partitionsGT) {
						include = false
					}
				}

				// Check replication factor filter
				if rf, ok := filter["replication_factor"].(float64); ok {
					if topic.ReplicationFactor != int(rf) {
						include = false
					}
				}

				// Check compression filter - need to fetch topic config
				if compression, ok := filter["compression"].(string); ok {
					config, err := m.client.GetTopicConfig(topic.Name)
					if err == nil && config != nil {
						compressionType := config.Configs["compression.type"]
						// "none" in query means no compression or producer (default)
						if compression == "none" {
							if compressionType != "" && compressionType != "producer" && compressionType != "none" {
								include = false
							}
						} else if compressionType != compression {
							include = false
						}
					}
				}

				if include {
					filteredTopics = append(filteredTopics, topic)
				}
			}

			// Format response
			if len(filteredTopics) == 0 {
				return AIResponseMsg{
					response: "No topics found matching the criteria.",
					err:      nil,
				}
			}

			var responseText strings.Builder
			fmt.Fprintf(&responseText, "Found %d topic(s) matching criteria:\n\n", len(filteredTopics))

			for _, topic := range filteredTopics {
				fmt.Fprintf(&responseText, "📋 Topic: %s\n", topic.Name)
				fmt.Fprintf(&responseText, "   • Partitions: %d\n", topic.Partitions)
				fmt.Fprintf(&responseText, "   • Replication Factor: %d\n", topic.ReplicationFactor)

				// Get compression info if requested
				if _, hasCompressionFilter := filter["compression"]; hasCompressionFilter {
					config, err := m.client.GetTopicConfig(topic.Name)
					if err == nil && config != nil {
						compressionType := config.Configs["compression.type"]
						if compressionType == "" || compressionType == "producer" {
							responseText.WriteString("   • Compression: none (using producer default)\n")
						} else {
							fmt.Fprintf(&responseText, "   • Compression: %s\n", compressionType)
						}
					}
				}
				responseText.WriteString("\n")
			}

			return AIResponseMsg{
				response: responseText.String(),
				err:      nil,
			}
		}

	case "create_acl":
		principal, _ := command["principal"].(string)
		host, _ := command["host"].(string)
		resourceType, _ := command["resource_type"].(string)
		resourceName, _ := command["resource_name"].(string)
		patternType, _ := command["pattern_type"].(string)
		operation, _ := command["operation"].(string)
		permissionType, _ := command["permission_type"].(string)

		if principal != "" && resourceType != "" && resourceName != "" {
			return func() tea.Msg {
				acl := kafka.ACL{
					Principal:      principal,
					Host:           host,
					ResourceType:   resourceType,
					ResourceName:   resourceName,
					PatternType:    patternType,
					Operation:      operation,
					PermissionType: permissionType,
				}

				err := m.client.CreateACL(acl)
				if err != nil {
					return AIResponseMsg{
						response: fmt.Sprintf("❌ Failed to create ACL: %v", err),
						err:      err,
					}
				}

				return AIResponseMsg{
					response: fmt.Sprintf("✅ Successfully created ACL:\n• Principal: %s\n• Resource: %s %s\n• Operation: %s %s\n• Host: %s",
						principal, resourceType, resourceName, operation, permissionType, host),
					err: nil,
				}
			}
		}

	case "create_acls":
		aclsData, _ := command["acls"].([]interface{})

		if len(aclsData) > 0 {
			return func() tea.Msg {
				var created []string
				var errors []string

				for _, aclData := range aclsData {
					aclMap, ok := aclData.(map[string]interface{})
					if !ok {
						continue
					}

					acl := kafka.ACL{
						Principal:      aclMap["principal"].(string),
						Host:           aclMap["host"].(string),
						ResourceType:   aclMap["resource_type"].(string),
						ResourceName:   aclMap["resource_name"].(string),
						PatternType:    aclMap["pattern_type"].(string),
						Operation:      aclMap["operation"].(string),
						PermissionType: aclMap["permission_type"].(string),
					}

					err := m.client.CreateACL(acl)
					if err != nil {
						errors = append(errors, fmt.Sprintf("%s %s: %v", acl.Operation, acl.PermissionType, err))
					} else {
						created = append(created, fmt.Sprintf("%s %s", acl.Operation, acl.PermissionType))
					}
				}

				var responseText strings.Builder
				if len(created) > 0 {
					fmt.Fprintf(&responseText, "✅ Successfully created %d ACL(s):\n", len(created))
					for _, c := range created {
						fmt.Fprintf(&responseText, "  • %s\n", c)
					}
				}
				if len(errors) > 0 {
					fmt.Fprintf(&responseText, "\n❌ Failed to create %d ACL(s):\n", len(errors))
					for _, e := range errors {
						fmt.Fprintf(&responseText, "  • %s\n", e)
					}
				}

				return AIResponseMsg{
					response: responseText.String(),
					err:      nil,
				}
			}
		}

	case "delete_acl":
		principal, _ := command["principal"].(string)
		host, _ := command["host"].(string)
		resourceType, _ := command["resource_type"].(string)
		resourceName, _ := command["resource_name"].(string)
		patternType, _ := command["pattern_type"].(string)
		operation, _ := command["operation"].(string)
		permissionType, _ := command["permission_type"].(string)

		if principal != "" && resourceType != "" && resourceName != "" {
			return func() tea.Msg {
				acl := kafka.ACL{
					Principal:      principal,
					Host:           host,
					ResourceType:   resourceType,
					ResourceName:   resourceName,
					PatternType:    patternType,
					Operation:      operation,
					PermissionType: permissionType,
				}

				err := m.client.DeleteACL(acl)
				if err != nil {
					return AIResponseMsg{
						response: fmt.Sprintf("❌ Failed to delete ACL: %v", err),
						err:      err,
					}
				}

				return AIResponseMsg{
					response: fmt.Sprintf("✅ Successfully deleted ACL:\n• Principal: %s\n• Resource: %s %s\n• Operation: %s %s",
						principal, resourceType, resourceName, operation, permissionType),
					err: nil,
				}
			}
		}

	case "query_acls":
		filter, _ := command["filter"].(map[string]interface{})

		return func() tea.Msg {
			// Get all ACLs
			acls, err := m.client.ListACLs()
			if err != nil {
				return AIResponseMsg{
					response: fmt.Sprintf("❌ Failed to fetch ACLs: %v", err),
					err:      err,
				}
			}

			// Filter ACLs based on criteria
			var filteredACLs []kafka.ACL
			for _, acl := range acls {
				include := true

				// Check filters
				if principal, ok := filter["principal"].(string); ok && principal != "" {
					if !strings.Contains(strings.ToLower(acl.Principal), strings.ToLower(principal)) {
						include = false
					}
				}

				if resourceType, ok := filter["resource_type"].(string); ok && resourceType != "" {
					if !strings.EqualFold(acl.ResourceType, resourceType) {
						include = false
					}
				}

				if resourceName, ok := filter["resource_name"].(string); ok && resourceName != "" {
					if !strings.Contains(strings.ToLower(acl.ResourceName), strings.ToLower(resourceName)) {
						include = false
					}
				}

				if operation, ok := filter["operation"].(string); ok && operation != "" {
					if !strings.EqualFold(acl.Operation, operation) {
						include = false
					}
				}

				if include {
					filteredACLs = append(filteredACLs, acl)
				}
			}

			// Format response
			var responseText strings.Builder
			if len(filteredACLs) == 0 {
				responseText.WriteString("No ACLs found matching the criteria.")
			} else {
				fmt.Fprintf(&responseText, "Found %d ACL(s):\n\n", len(filteredACLs))

				// Group ACLs by resource for better readability
				resourceMap := make(map[string][]kafka.ACL)
				for _, acl := range filteredACLs {
					key := fmt.Sprintf("%s:%s", acl.ResourceType, acl.ResourceName)
					resourceMap[key] = append(resourceMap[key], acl)
				}

				for resource, aclList := range resourceMap {
					parts := strings.Split(resource, ":")
					fmt.Fprintf(&responseText, "📋 %s: %s\n", parts[0], parts[1])
					for _, acl := range aclList {
						fmt.Fprintf(&responseText, "  • %s → %s %s (from %s)\n",
							acl.Principal, acl.Operation, acl.PermissionType, acl.Host)
					}
					responseText.WriteString("\n")
				}
			}

			return AIResponseMsg{
				response: responseText.String(),
				err:      nil,
			}
		}
	}

	return nil
}
