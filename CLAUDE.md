# KConduit

## Goal

To create a powerful terminal UI for Apache Kafka management with an intuitive interface using the excellent libraries from https://charm.land

## Core Features

### Kafka Operations
- Connect to Apache Kafka clusters and manage topics with a beautiful table-based UI using "github.com/charmbracelet/bubbletea"
- Full CRUD operations on topics with safety confirmations
- Real-time message production and consumption
- Consumer group monitoring with lag calculation
- Configuration management with live editing

### AI Assistant
- Natural language interface for Kafka operations
- Multi-provider support (OpenAI, Gemini, Anthropic, Ollama)
- Batch operations on all topics simultaneously
- Multi-step command execution
- Smart queries for topics and consumer groups

## Technical Stack
- **Language**: Go
- **UI Framework**: Bubble Tea by Charm
- **Kafka Client**: IBM Sarama
- **AI Integration**: Multiple provider APIs

## Design Principles
- User-friendly terminal interface
- Safety-first approach (confirmations for destructive operations)
- Comprehensive logging for debugging
- Extensible architecture for new features

## Workflow — Agent Gates

These agents are mandatory gates, not optional tools. Do not skip them.

### Before creating any GitHub issue:
Run the **issue-writer** agent. Every issue must have: summary, detailed requirements, numbered acceptance criteria, specific testing requirements (named tests, not "add tests"), documentation requirements, dependencies, and labels. If any section is missing or vague, rewrite it before creating.

### Before pushing to GitHub

Run `make lint` and fix **all** linting issues. Do not push code with lint errors.

### After completing any feature:
1. **code-reviewer** — on all changed files
2. **security-reviewer** — on any code touching TLS, HTTP, credentials, or external input
3. **go-quality** — as a final gate before commit

### After creating or modifying CI/CD configuration:
4. **devops** — on any workflow, GoReleaser, Dependabot, or Makefile changes

### When writing tests:
5. **test-writer** — use for creating unit, integration, and BDD tests

### When writing or reviewing documentation:
6. **docs-writer** — on any README, godoc, examples, CONTRIBUTING, CHANGELOG, SECURITY.md, or config reference changes.

### When working on complex Kubernetes tasks
7. Ask **kube** for help with kubebuilder, operators or decisions about how to implement and resolve Kubernetes issues.

