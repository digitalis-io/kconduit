# KConduit Features

## Topic Deletion

Topics can be deleted using the `D` key from the Topics tab. For safety:
- A confirmation dialog will appear requiring you to type the exact topic name
- The delete button is only enabled when the typed name matches exactly
- This feature is NOT available through the AI Assistant to prevent accidental deletions
- Press `ESC` to cancel the deletion at any time

## AI Assistant for KConduit

The AI Assistant feature allows you to interact with Kafka using natural language commands. Press `A` from any screen to open the AI Assistant.

## Command Line Options

You can specify the AI engine and model via command-line arguments:

```bash
./kconduit -b localhost:9092 --ai-engine gemini --ai-model gemini-1.5-pro-latest
./kconduit -b localhost:9092 --ai-engine openai --ai-model gpt-4
./kconduit -b localhost:9092 --ai-engine anthropic --ai-model claude-3-opus-20240229
./kconduit -b localhost:9092 --ai-engine ollama --ai-model llama2
```

## Supported Providers

### 1. OpenAI (ChatGPT)
```bash
export OPENAI_API_KEY="your-api-key-here"
export OPENAI_MODEL="gpt-3.5-turbo"  # Optional, defaults to gpt-3.5-turbo
./kconduit -b localhost:9092
# Or override with command-line
./kconduit -b localhost:9092 --ai-engine openai --ai-model gpt-4
```

### 2. Google Gemini
```bash
export GEMINI_API_KEY="your-api-key-here"
export GEMINI_MODEL="gemini-1.5-pro-latest"  # Optional, defaults to gemini-1.5-pro-latest
./kconduit -b localhost:9092
# Or override with command-line
./kconduit -b localhost:9092 --ai-engine gemini --ai-model gemini-1.5-flash
```

### 3. Anthropic (Claude)
```bash
export ANTHROPIC_API_KEY="your-api-key-here"
export ANTHROPIC_MODEL="claude-3-haiku-20240307"  # Optional, defaults to claude-3-haiku
./kconduit -b localhost:9092
# Or override with command-line
./kconduit -b localhost:9092 --ai-engine anthropic --ai-model claude-3-opus-20240229
```

### 4. Ollama (Local LLM)
First install and run Ollama:
```bash
# Install Ollama
curl -fsSL https://ollama.com/install.sh | sh

# Pull a model (e.g., llama2, mistral, codellama)
ollama pull llama2

# Run Ollama server (it runs on http://localhost:11434 by default)
ollama serve

# Configure environment (optional)
export OLLAMA_URL="http://localhost:11434"  # Optional, defaults to localhost:11434
export OLLAMA_MODEL="llama2"  # Optional, defaults to llama2
./kconduit -b localhost:9092
# Or override with command-line
./kconduit -b localhost:9092 --ai-engine ollama --ai-model mistral
```

## Using the AI Assistant

1. Press `A` from any screen to open the AI Assistant
2. Type your command in natural language
3. Press `Tab` to cycle through providers (OpenAI → Gemini → Anthropic → Ollama)
4. Press `Enter` to execute the command
5. Press `ESC` to exit the AI Assistant

The assistant will automatically select the first provider that has an API key configured.

## Example Commands

### Topic Management
- "Create a topic named my-events with 3 partitions and gzip compression"
- "Create a topic called user-activity with 5 partitions and replication factor 2"
- "Make a new topic test-topic with snappy compression"
- "Create topic orders with 10 partitions"
- "Change the partitions to 100 on topic my-events"
- "Change the compression to gzip on topic user-activity"
- "Modify retention to 7 days on topic orders"

### Batch Operations (ALL Topics)
- "Increase partitions to 100 on all topics"
- "Set compression to gzip on all topics"
- "Change retention to 7 days for all topics"
- "Update all topics to have 50 partitions"
- "Enable compression on all topics"

### Topic Queries
- "List topics with no compression"
- "Find topics with more than 10 partitions"
- "Show topics that contain 'events' in their name"
- "List topics with replication factor 3"
- "Find topics using gzip compression"

### Consumer Group Queries
- "Find consumer groups with lag greater than 10"
- "Show me all consumer groups with lag above 1000"
- "List consumer groups that contain 'payment' in their name"
- "Find consumer groups in Stable state"
- "Show consumer groups with high lag"

## Configuration

### Configuration Precedence

Configuration options are applied in the following order (highest priority first):
1. Command-line arguments (`--ai-engine`, `--ai-model`)
2. Environment variables (e.g., `OPENAI_MODEL`, `GEMINI_MODEL`)
3. Default values

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `OPENAI_API_KEY` | OpenAI API key for ChatGPT | - |
| `OPENAI_MODEL` | OpenAI model to use | gpt-3.5-turbo |
| `GEMINI_API_KEY` | Google Gemini API key | - |
| `GEMINI_MODEL` | Gemini model to use | gemini-pro |
| `ANTHROPIC_API_KEY` | Anthropic API key for Claude | - |
| `ANTHROPIC_MODEL` | Claude model to use | claude-3-haiku-20240307 |
| `OLLAMA_URL` | Ollama server URL | http://localhost:11434 |
| `OLLAMA_MODEL` | Ollama model to use | llama2 |

### Supported Operations

The AI Assistant will automatically detect and parse commands for:
- Creating topics with specific configurations
- Setting partition counts
- Setting replication factors
- Configuring compression (gzip, snappy, lz4, zstd)
- Modifying existing topic partitions (increase only)
- Modifying topic configurations (retention, compression, etc.)
- Batch operations on ALL topics:
  - Modify partitions for all topics at once
  - Update configurations for all topics at once
- Querying topics by:
  - Compression type (none, gzip, snappy, lz4, zstd)
  - Partition count thresholds
  - Topic name patterns
  - Replication factor
- Querying consumer groups by:
  - Consumer lag thresholds
  - Group ID patterns
  - Group state (Stable, PreparingRebalance, CompletingRebalance, Empty, Dead)
- Additional topic configurations

## Notes

- The assistant automatically selects the first provider with a configured API key
- You can cycle through all providers using the Tab key
- The current provider and model are displayed in the title bar
- The AI Assistant can parse JSON responses and execute Kafka operations automatically
- Topic creation happens immediately after the AI parses the command successfully