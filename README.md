# KConduit

A Golang implementation of kcat (kafkacat) with a beautiful terminal UI built using Charm's Bubble Tea framework.

## Features

- Connect to Apache Kafka clusters
- List topics with detailed information (partitions, replication factor)
- Interactive table view with keyboard navigation
- Create new topics with customizable partitions and replication factor
- Consumer mode for viewing messages from topics with formatted display
- Producer mode for sending messages to selected topics
- Real-time refresh capability

## Installation

```bash
go install github.com/axonops/kconduit/cmd/kconduit@latest
```

Or build from source:

```bash
make build
```

## Usage

```bash
# Connect to local Kafka
./kconduit

# Connect to specific brokers
./kconduit -brokers broker1:9092,broker2:9092

# Short form
./kconduit -b broker1:9092,broker2:9092
```

## Keyboard Shortcuts

### Topic List View
- `↑/↓` - Navigate through topics
- `Enter` - Enter consumer mode for selected topic
- `P` - Enter producer mode for selected topic
- `C` - Create a new topic
- `r` - Refresh topic list
- `q` or `Ctrl+C` - Quit

### Consumer Mode
- `↑/↓` - Scroll through messages
- `c` - Clear message list
- `q` or `Esc` - Return to topic list

### Producer Mode
- `Tab` or `Enter` - Switch between key and value fields
- `Ctrl+S` - Send message
- `Esc` - Return to topic list

### Create Topic Mode
- `Tab` - Navigate between fields (name, partitions, replication)
- `Enter` - Move to next field or create topic
- `Esc` - Cancel and return to topic list

## Requirements

- Go 1.20+
- Access to a Kafka cluster

## Building

```bash
# Build the binary
make build

# Run directly
make run

# Clean build artifacts
make clean
```