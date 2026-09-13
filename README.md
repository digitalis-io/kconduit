<p align="center">
  <a href="https://digitalis.io">
    <img src="https://digitalis-marketplace-assets.s3.us-east-1.amazonaws.com/DigitalisDigital_DigitalisFullLogoGradient+-+medium.png" alt="Digitalis.IO" width="300">
  </a>
</p>

<p align="center">
  <em>Built and maintained by <a href="https://digitalis.io">Digitalis.IO</a></em>
</p>

# KConduit — Kafka Terminal UI with AI Assistant

> ⚠️ **BETA RELEASE** - This software is in beta. While functional, it may contain bugs or unexpected behaviors. Please use with caution in production environments.

**KConduit** is an open-source Kafka CLI and terminal UI (TUI) for Apache Kafka management, built with Go and [Charm's Bubble Tea](https://github.com/charmbracelet/bubbletea) framework. It provides a fast, keyboard-driven alternative to web-based Kafka GUI tools, with a built-in AI assistant that accepts natural language commands for topic management, consumer group monitoring, and cluster operations — no browser required.

[![Go Version](https://img.shields.io/badge/go-1.27%2B-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue)](LICENSE)
[![Release](https://img.shields.io/github/v/release/digitalis-io/kconduit)](https://github.com/digitalis-io/kconduit/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/digitalis-io/kconduit)](https://goreportcard.com/report/github.com/digitalis-io/kconduit)

[![KConduit — Kafka TUI demo showing topic management and AI assistant](https://img.youtube.com/vi/bRF6hGm72gM/maxresdefault.jpg)](https://youtu.be/bRF6hGm72gM)

## ✨ Features

### Core Kafka Management
- 🔌 **Multi-Broker Support** - Connect to Apache Kafka clusters with multiple brokers
- 📊 **Comprehensive Views** - Browse brokers, topics, consumer groups, and ACLs in a tabbed interface
- 🎯 **Topic Management** - Create, configure, and delete Kafka topics with safety confirmations
- 📨 **Message Operations** - Produce and consume Kafka messages with formatted display
- ⚙️ **Configuration Editor** - View and modify topic configurations in real-time
- 👥 **Consumer Group Monitoring** - Track consumer groups with lag calculation, and drill into per-partition lag
- 📈 **Topic Metrics** - Message counts and on-disk size per topic, alongside partitions and replication factor
- 🔍 **Filter and Sort** - Narrow any table with `/` and re-order it by any column
- 🔄 **Auto-Refresh** - Toggle a live 5-second refresh of the current tab with `Ctrl+R`
- 📋 **Clipboard** - Copy the selected row, or a config value, with `y`
- ❓ **Built-in Help** - Press `?` for the full keyboard reference without leaving the app

### AI Assistant for Kafka
- 🤖 **Natural Language Commands** - Manage Kafka using plain English instead of CLI flags
- 🎯 **Multi-Provider Support** - OpenAI (ChatGPT), Google Gemini, Anthropic Claude, and Ollama (local LLM)
- 🔄 **Batch Operations** - Modify all topics at once with a single command
- 📝 **Multi-Step Execution** - Execute complex Kafka operations in sequence
- 🔍 **Smart Queries** - Find topics and consumer groups by partition count, compression, lag, and more

See [README_AI.md](README_AI.md) for full AI assistant documentation and example commands.

## 📦 Installation

### Using Go Install
```bash
go install github.com/digitalis-io/kconduit/cmd/kconduit@latest
```

### Building from Source
```bash
git clone https://github.com/digitalis-io/kconduit
cd kconduit
make build
```

## 🚀 Usage

### Basic Connection
```bash
# Connect to local Kafka (localhost:9092)
./kconduit

# Connect to specific brokers
./kconduit -b broker1:9092,broker2:9092

# With logging
./kconduit -b localhost:9092 --log-level debug --log-file kconduit.log
```

### SASL Authentication
```bash
# Connect with SASL/PLAIN authentication
./kconduit -b localhost:29092 \
  --sasl \
  --sasl-mechanism PLAIN \
  --sasl-username admin \
  --sasl-password admin-secret \
  --sasl-protocol SASL_PLAINTEXT

# Connect with SASL/SCRAM-SHA-256
./kconduit -b localhost:9092 \
  --sasl \
  --sasl-mechanism SCRAM-SHA-256 \
  --sasl-username alice \
  --sasl-password alice-secret

# Connect with SASL over SSL (using default system certificates)
./kconduit -b broker:9093 \
  --sasl \
  --sasl-mechanism PLAIN \
  --sasl-username admin \
  --sasl-password admin-secret \
  --sasl-protocol SASL_SSL

# Connect with SASL_SSL and custom certificates
./kconduit -b broker:9093 \
  --sasl \
  --sasl-mechanism PLAIN \
  --sasl-username admin \
  --sasl-password admin-secret \
  --sasl-protocol SASL_SSL \
  --tls-ca-cert /path/to/ca-cert.pem \
  --tls-client-cert /path/to/client-cert.pem \
  --tls-client-key /path/to/client-key.pem

# Connect with SSL/TLS only (no SASL)
./kconduit -b broker:9093 \
  --tls \
  --tls-ca-cert /path/to/ca-cert.pem \
  --tls-client-cert /path/to/client-cert.pem \
  --tls-client-key /path/to/client-key.pem

# Connect with SSL/TLS and skip certificate verification (insecure, for testing only)
./kconduit -b broker:9093 \
  --tls \
  --tls-skip-verify
```

### AI Assistant Configuration

KConduit's built-in AI assistant lets you manage Kafka topics and consumer groups using natural language. See [README_AI.md](README_AI.md) for the full command reference.

```bash
# Using OpenAI
export OPENAI_API_KEY="your-api-key"
./kconduit -b localhost:9092 --ai-engine openai --ai-model gpt-4

# Using Google Gemini
export GEMINI_API_KEY="your-api-key"   # or GOOGLE_API_KEY
./kconduit -b localhost:9092 --ai-engine gemini --ai-model gemini-3.1-pro-preview

# Using Anthropic Claude
export ANTHROPIC_API_KEY="your-api-key"
./kconduit -b localhost:9092 --ai-engine anthropic --ai-model claude-3-opus-20240229

# Using Local Ollama (no API key required)
ollama serve  # In another terminal
./kconduit -b localhost:9092 --ai-engine ollama --ai-model llama2
```

### Session Management

KConduit can save and load connection profiles so you do not need to repeat broker addresses and authentication flags on every invocation.

```bash
# Save the current connection config as a named session
./kconduit -b broker:9092 --sasl --sasl-username alice --save-session prod

# Load a saved session
./kconduit --session prod

# List all saved sessions
./kconduit --list-sessions
```

Saved sessions are stored in `~/.config/kconduit/sessions.yaml`. Plaintext passwords are never persisted to disk. Supply the password at runtime via the `KCONDUIT_SASL_PASSWORD` environment variable, or set the `password_file` field in the YAML to a path containing the password.

You can also open the Session Manager interactively at any time by pressing `s` from the main view.

## ⌨️ Keyboard Shortcuts

### Global Navigation
- `Tab` / `Shift+Tab` - Cycle forward/backward through tabs (Brokers, Topics, Consumer Groups, ACLs)
- `1-4` - Jump directly to a tab by number
- `↑/↓` - Move the selection
- `g` / `G` - Jump to the first or last row
- `/` - Filter the current table; `Enter` keeps the filter, `Esc` clears it
- `<` / `>` - Step the sort order (each column has an ascending and a descending place)
- `y` - Copy the selected row to the clipboard
- `r` / `R` - Refresh current view
- `Ctrl+R` - Toggle auto-refresh of the current tab (every 5 seconds)
- `?` / `F1` - Open the keyboard reference
- `A` / `a` - Open AI Assistant
- `s` / `S` - Open Session Manager
- `Esc` - Close an overlay, or clear the current filter
- `q` or `Ctrl+C` - Quit application

### Brokers Tab
- `Enter` - Show full detail for the selected broker (address, role, rack, API version, listeners, log directories)

### Topics Tab
- `↑/↓` - Navigate through topics
- `Tab` - Switch between topic list and configuration panel
- `Enter` - Start consuming from selected topic
- `P` - Produce messages to selected topic
- `C` - Create new topic
- `D` - Delete selected topic (with confirmation)
- `e` - Edit topic configuration

The create-topic dialog validates as you type, shows a summary of exactly what
will be created including the defaults for any field left blank, and checks the
replication factor against the number of brokers in the cluster.
- `y` - Copy the selected config value when the configuration panel is focused

The topic table shows message count and on-disk size per topic. Both are
measured after the list appears — they show `…` until they arrive. Message count
is the number of records currently retained, not everything ever produced. Disk
size is the total across every replica, so a 1 GiB topic with replication factor
3 reports 3 GiB.

### Consumer Groups Tab
- `↑/↓` - Navigate through consumer groups
- `Enter` - Break the group's lag down by partition (committed offset, log end offset, lag, owning member)
- `/`, `<` / `>`, `g` / `G`, `y` - Filter, sort, jump, and copy, as in every other table

### Consumer Mode
- `↑/↓` or `PgUp/PgDn` - Scroll through messages
- `c` - Clear message list
- `Esc` - Return to topic list

### Producer Mode
- `Tab` - Switch between key and value fields
- `Ctrl+S` - Send message
- `Esc` - Return to topic list

### Delete Topic Dialog
- `Type topic name` - Confirmation required
- `Tab` - Navigate between input and buttons
- `Enter` - Confirm deletion (only when name matches)
- `Esc` - Cancel deletion

### ACLs Tab
- `↑/↓` - Navigate through ACL entries
- `C` - Create new ACL
- `e` - Edit selected ACL
- `d` - Delete selected ACL (with confirmation)
- `Tab` / `Shift+Tab` - Navigate between fields in the create/edit dialog
- `Space` - Select an operation in the multi-select
- `Enter` - Confirm
- `Esc` - Cancel / return to the ACL list

Each dialog shows the rule in plain English as you fill it in — for example
`User:alice may read on topic "orders" from host *` — so the effect is readable
without assembling it from the individual fields. Editing an ACL deletes the
existing rule and creates the replacement, because Kafka has no in-place update;
the dialog says so before you save.

## 🤖 AI Assistant Commands

Press `A` from any screen to open the AI assistant. For full provider setup and command examples, see [README_AI.md](README_AI.md).

### Topic Management
```
"Create a topic named events with 10 partitions and gzip compression"
"Change the partitions to 50 on topic user-events"
"Set retention to 7 days on orders topic"
"Delete topic test-topic"  // Not supported for safety
```

### Batch Operations (ALL Topics)
```
"Increase partitions to 100 on all topics"
"Set compression to lz4 on all topics"
"Change retention to 30 days for all topics"
```

### Topic Queries
```
"List topics with no compression"
"Find topics with more than 10 partitions"
"Show topics that contain 'events' in their name"
```

### Consumer Group Queries
```
"Find consumer groups with lag greater than 1000"
"List consumer groups that contain 'payment'"
"Show consumer groups in Stable state"
```

### Multi-Step Operations
```
"Change hello-topic to use lz4 compression and increase partitions to 100"
// This executes both operations in sequence
```

## 🔧 Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `KCONDUIT_BROKERS` | Kafka broker addresses | localhost:9092 |
| `KCONDUIT_LOG_LEVEL` | Log level (debug, info, warn, error) | info |
| `KCONDUIT_LOG_FILE` | Log file path | stderr |
| `KCONDUIT_AI_ENGINE` | AI engine (openai, gemini, anthropic, ollama) | gemini |
| `KCONDUIT_AI_MODEL` | AI model to use | gemini-3.1-pro-preview |
| `KCONDUIT_SASL_ENABLED` | Enable SASL authentication | false |
| `KCONDUIT_SASL_MECHANISM` | SASL mechanism | PLAIN |
| `KCONDUIT_SASL_USERNAME` | SASL username | - |
| `KCONDUIT_SASL_PASSWORD` | SASL password | - |
| `KCONDUIT_SASL_PROTOCOL` | Security protocol | SASL_PLAINTEXT |
| `KCONDUIT_TLS_ENABLED` | Enable TLS/SSL | false |
| `KCONDUIT_TLS_CA_CERT` | Path to CA certificate file | - |
| `KCONDUIT_TLS_CLIENT_CERT` | Path to client certificate file | - |
| `KCONDUIT_TLS_CLIENT_KEY` | Path to client key file | - |
| `KCONDUIT_TLS_SKIP_VERIFY` | Skip TLS certificate verification | false |
| `OPENAI_API_KEY` | OpenAI API key for AI assistant | - |
| `OPENAI_MODEL` | OpenAI model to use | gpt-3.5-turbo |
| `GEMINI_API_KEY` | Google Gemini API key | - |
| `GOOGLE_API_KEY` | Accepted in place of `GEMINI_API_KEY`; `GEMINI_API_KEY` wins if both are set | - |
| `GEMINI_MODEL` | Gemini model to use | gemini-3.1-pro-preview |
| `ANTHROPIC_API_KEY` | Anthropic API key | - |
| `ANTHROPIC_MODEL` | Claude model to use | claude-3-haiku-20240307 |
| `OLLAMA_URL` | Ollama server URL | http://localhost:11434 |
| `OLLAMA_MODEL` | Ollama model to use | llama2 |

### Command Line Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-b, --brokers` | Comma-separated list of Kafka brokers | localhost:9092 |
| `--log-level` | Log level (debug, info, warn, error) | info |
| `--log-file` | Log file path (empty for stderr) | - |
| `--ai-engine` | AI engine (openai, gemini, anthropic, ollama) | gemini |
| `--ai-model` | AI model to use | gemini-3.1-pro-preview |
| `--session` | Load a saved connection session by name | - |
| `--save-session` | Save current connection config as a named session and exit | - |
| `--list-sessions` | List all saved connection sessions and exit | - |
| `--sasl` | Enable SASL authentication | false |
| `--sasl-mechanism` | SASL mechanism (PLAIN, SCRAM-SHA-256, SCRAM-SHA-512) | PLAIN |
| `--sasl-username` | SASL username | - |
| `--sasl-password` | SASL password (**deprecated** — use `KCONDUIT_SASL_PASSWORD` instead) | - |
| `--sasl-protocol` | Security protocol (SASL_PLAINTEXT, SASL_SSL) | SASL_PLAINTEXT |
| `--tls` | Enable TLS/SSL | false |
| `--tls-ca-cert` | Path to CA certificate file | - |
| `--tls-client-cert` | Path to client certificate file | - |
| `--tls-client-key` | Path to client key file | - |
| `--tls-skip-verify` | Skip TLS certificate verification (insecure) | false |

## 🏗️ Building & Development

### Requirements
- Go 1.27+
- Access to a Kafka cluster

### Build Commands
```bash
# Build the binary
make build

# Run directly
make run

# Clean build artifacts
make clean

# Start a local test cluster and connect to it
make kafka-up        # plaintext, no ACLs
make run-plain

# Or the SASL cluster with ACLs enabled
make kafka-acls-up
make run-acls

# Stop them again
make kafka-down
make kafka-acls-down
```

The two clusters publish some of the same ports, so only one can run at a time;
`make kafka-up` and `make kafka-acls-up` each refuse to start if the other is
already up. `make help` lists every target.

## 🔒 Safety Features

- **Topic Deletion Protection** - Requires typing the exact topic name for confirmation
- **AI Safety** - The AI assistant cannot perform delete operations
- **Error Recovery** - Failed operations in a batch do not stop other operations
- **Comprehensive Logging** - All operations logged for audit trail

## 📋 Supported Kafka Operations

### Topic Operations
- ✅ Create topics with custom configurations
- ✅ Modify topic partitions (increase only)
- ✅ Update topic configurations
- ✅ Delete topics (with confirmation)
- ✅ View all topic configurations
- ✅ Batch operations on all topics

### Message Operations
- ✅ Produce messages with key-value pairs
- ✅ Consume messages from any partition
- ✅ Format and display message headers
- ✅ Clear consumer display

### Consumer Group Operations
- ✅ List all consumer groups
- ✅ Calculate consumer lag per group
- ✅ View group members and state
- ✅ Query groups by various criteria

### ACL Operations
- ✅ List all ACLs with detailed information
- ✅ Create new ACLs with form interface
- ✅ Edit existing ACLs with pre-filled values
- ✅ Multi-select operations — create multiple ACLs at once
- ✅ Support for all resource types (Topic, Group, Cluster, TransactionalId)
- ✅ Support for all operations (Read, Write, Create, Delete, etc.)
- ✅ Pattern-based resource matching (Literal, Prefixed, Any)
- ✅ Allow and Deny permissions
- ✅ Input validation and error handling

### Broker Operations
- ✅ List all brokers with status
- ✅ Identify active controller
- ✅ Display broker versions and roles
- ✅ Show rack information

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## About Digitalis

[Digitalis](https://digitalis.io) is a cloud-native technology services company specializing in data engineering, DevOps, and digital transformation. With deep expertise in Apache Kafka and distributed streaming systems, Digitalis provides comprehensive support and consulting services to help organizations leverage their data infrastructure effectively.

### Kafka Support Services
Digitalis offers professional support and consulting for Apache Kafka deployments, including:
- 24x7 Fully Managed Service for Kafka clusters
- Architecture design and implementation
- Performance optimization and troubleshooting
- Data streaming solutions and integrations

For enterprise support or consulting services for your Kafka infrastructure, visit [digitalis.io](https://digitalis.io) or contact their team for assistance with your data streaming needs.

## 📄 License

This project is licensed under the Apache License 2.0 — see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) by Charm
- Uses [Sarama](https://github.com/IBM/sarama) for Kafka client operations
- AI providers: OpenAI, Google Gemini, Anthropic, and Ollama

## 💬 Support

This project is maintained by [Digitalis.io](https://digitalis.io). For support,
visit [digitalis.io/contact](https://digitalis.io/contact).

## 📄 Legal Notices

*This project may contain trademarks or logos for projects, products, or services. Any use of third-party trademarks or logos are subject to those third-party's policies.*

- **Apache**, **Apache Kafka** and **Kafka** are either registered trademarks or trademarks of the Apache Software Foundation or its subsidiaries in Canada, the United States and/or other countries.
