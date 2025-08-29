# Build stage
FROM golang:1.24.5-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo 'docker')" \
    -o kconduit \
    ./cmd/kconduit

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1000 kconduit && \
    adduser -D -u 1000 -G kconduit kconduit

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/kconduit /app/kconduit

# Change ownership
RUN chown -R kconduit:kconduit /app

# Switch to non-root user
USER kconduit

# Expose default port (if needed)
# EXPOSE 8080

# Set entrypoint
ENTRYPOINT ["/app/kconduit"]

# Default command (can be overridden)
CMD ["--help"]