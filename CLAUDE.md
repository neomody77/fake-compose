# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

fake-compose is a Docker Compose compatible tool written in Go that extends standard Docker Compose functionality with init containers, post containers, lifecycle hooks, and cloud-native integrations. It serves as a drop-in replacement for Docker Compose with additional enterprise features.

## Development Commands

```bash
# Build the main application
go build -o fake-compose cmd/fake-compose/main.go

# Run with example compose file
./fake-compose up -f examples/simple-compose.yml

# Validate compose file syntax
./fake-compose validate -f examples/simple-compose.yml

# View parsed configuration
./fake-compose config -f examples/simple-compose.yml

# Stop services
./fake-compose down -f examples/simple-compose.yml

# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests for specific package
go test ./internal/parser
go test ./pkg/compose
go test ./pkg/hooks

# Format code
go fmt ./...

# Run go vet
go vet ./...

# Update dependencies
go mod tidy
```

## Architecture

The project follows a layered architecture with clear separation between public APIs and internal implementation:

### Core Packages (`pkg/`)
- **`pkg/compose/types.go`** - Complete type definitions for Docker Compose spec plus extended features (init/post containers, hooks, cloud-native)
- **`pkg/container/stub.go`** - Container management interface with stub implementations ready for Docker API integration
- **`pkg/lifecycle/manager.go`** - Service lifecycle state machine managing phases (pre-start, start, post-start, running, pre-stop, stop, post-stop)
- **`pkg/hooks/executor.go`** - Hook execution engine supporting command, script, HTTP, and exec hook types with retry logic

### Internal Components (`internal/`)
- **`internal/parser/parser.go`** - YAML parsing with environment variable expansion (`os.Expand`), path resolution, and validation
- **`internal/executor/executor.go`** - Service orchestration with dependency graph ordering, parallel execution, and rollback on failure

### Entry Points (`cmd/`)
- **`cmd/fake-compose/main.go`** - CLI tool with standard Docker Compose commands (up, down, validate, config)

## Extended Features

Beyond standard Docker Compose, this tool adds:

1. **Init Containers**: Run before main service starts
   - Database migrations, configuration setup, dependency checks
   - Sequential execution with failure handling

2. **Post Containers**: Run after service lifecycle events
   - `OnSuccess`: Execute after successful startup
   - `OnFailure`: Execute on service failure
   - `WaitFor`: Configurable delay before execution

3. **Lifecycle Hooks**: Execute at 8 lifecycle stages
   - Pre/Post Start, Stop, Build, Deploy
   - Types: command, script, HTTP, exec
   - Retry support with configurable attempts

4. **Cloud Native Integrations**:
   - Kubernetes annotations, labels, resource limits
   - Helm chart deployments with custom values
   - Istio traffic management
   - Prometheus monitoring configuration

## Key Implementation Details

- **Service Ordering**: Dependency graph traversal in `internal/executor/executor.go:orderServices()` ensures correct startup/shutdown sequence
- **Environment Variables**: Expanded during parsing using `os.Expand` in `internal/parser/parser.go:48`
- **Path Resolution**: Relative paths resolved against compose file directory in `internal/parser/parser.go:57`
- **Rollback Mechanism**: Failed service startup triggers automatic rollback of all started services
- **Concurrent Hook Execution**: HTTP client with 30s timeout for hook execution
- **Lifecycle State Tracking**: Thread-safe state management with mutex protection in `pkg/lifecycle/manager.go`

## Testing

Test changes with provided examples:
- `examples/simple-compose.yml` - Basic example with init containers and hooks
- `examples/full-featured-compose.yml` - Comprehensive example showcasing all features

The tool maintains 100% backward compatibility with Docker Compose while adding extended functionality. All standard Docker Compose features work as expected.

## Module Information

- **Module**: `github.com/neomody77/fake-compose`
- **Go Version**: 1.23+
- **Key Dependencies**:
  - `github.com/spf13/cobra` - CLI framework
  - `gopkg.in/yaml.v3` - YAML parsing
  - `github.com/sirupsen/logrus` - Structured logging