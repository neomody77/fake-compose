# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

fake-compose is a Docker Compose compatible tool written in Go that extends standard Docker Compose functionality with init containers, post containers, lifecycle hooks, and cloud-native integrations. It can be used as a drop-in replacement for Docker Compose or as a Docker CLI plugin.

## Development Commands

This is a Go project using Go modules. Common development commands:

```bash
# Build the main application
go build -o fake-compose cmd/fake-compose/main.go

# Build Docker Compose plugin
go build -o docker-compose cmd/docker-compose/main.go

# Run with example compose file
./fake-compose up -f examples/simple-compose.yml

# Validate compose file syntax
./fake-compose validate -f examples/simple-compose.yml

# View parsed configuration
./fake-compose config -f examples/simple-compose.yml

# Run all tests
go test ./...

# Run tests for specific package
go test ./internal/parser
go test ./pkg/compose
```

## Architecture

The project follows a layered architecture:

### Core Packages (`pkg/`)
- **`pkg/compose/types.go`** - Complete type definitions for Docker Compose + extended features
- **`pkg/container/`** - Container management interface (currently stub implementations)
- **`pkg/lifecycle/`** - Service lifecycle management with hooks
- **`pkg/hooks/`** - Hook execution engine for various lifecycle events

### Internal Components (`internal/`)
- **`internal/parser/parser.go`** - YAML parsing, environment variable expansion, validation
- **`internal/executor/executor.go`** - Service orchestration, dependency ordering, rollback handling

### Entry Points (`cmd/`)
- **`cmd/fake-compose/main.go`** - Standalone CLI tool with all Docker Compose commands
- **`cmd/docker-compose/main.go`** - Docker CLI plugin that masquerades as docker-compose

## Extended Features

Beyond standard Docker Compose, this tool adds:

1. **Init Containers**: Run before main service starts (database migrations, setup tasks)
2. **Post Containers**: Run after service lifecycle events (health checks, cleanup, notifications)
3. **Lifecycle Hooks**: Custom logic at 8 different stages (pre/post start/stop/build/deploy)
4. **Cloud Native Integrations**: Kubernetes, Helm, Istio, Prometheus configurations

### Hook Types
- `command`: Execute shell commands
- `script`: Run inline scripts  
- `http`: Make HTTP requests
- `exec`: Execute commands in containers

## Key Implementation Details

- **Service Ordering**: Uses dependency graph traversal in `executor.go:181`
- **Environment Variables**: Expanded during parsing using `os.Expand` in `parser.go:48`
- **Path Resolution**: Relative paths resolved against compose file directory in `parser.go:57`
- **Error Handling**: Rollback mechanism stops and removes containers on startup failure
- **Stub Operations**: Container operations are stubbed but provide complete framework for Docker API integration

## Testing

Test your changes with the provided examples:
- `examples/simple-compose.yml` - Basic example with init containers and hooks
- `examples/full-featured-compose.yml` - Comprehensive example with all features

The tool maintains 100% backward compatibility with Docker Compose while adding extended functionality.