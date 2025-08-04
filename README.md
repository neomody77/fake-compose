# Fake Compose

A Docker Compose compatible tool with extended features for init containers, post containers, lifecycle hooks, and cloud-native integrations.

## Features

### 🚀 Init Containers
Run initialization tasks before your main service starts:
- Database migrations
- Configuration preparation
- Dependency checks
- Volume initialization

### 📦 Post Containers
Execute tasks after service lifecycle events:
- Health checks after startup
- Cleanup on shutdown
- Notifications on success/failure
- Log collection

### 🎣 Lifecycle Hooks
Execute custom logic at various stages:
- **Pre/Post Start**: Before and after container starts
- **Pre/Post Stop**: Before and after container stops
- **Pre/Post Build**: Before and after image builds
- **Pre/Post Deploy**: Before and after deployment

Hook types supported:
- **Command**: Execute shell commands
- **Script**: Run inline scripts
- **HTTP**: Make HTTP requests
- **Exec**: Execute commands in containers

### ☁️ Cloud Native Integrations
- **Kubernetes**: Annotations, labels, resource limits
- **Helm**: Chart deployments with custom values
- **Istio**: Traffic management and routing
- **Prometheus**: Metrics and monitoring configuration

## Installation

```bash
go get github.com/neomody77/fake-compose-extended
```

## Usage

### Basic Commands

```bash
# Start services
fake-compose up -f docker-compose.yml

# Stop services
fake-compose down -f docker-compose.yml

# Validate compose file
fake-compose validate -f docker-compose.yml
```

### Command Line Options

- `-f, --file`: Specify compose file (default: docker-compose.yml)
- `-e, --env-file`: Load environment variables from file
- `-p, --project-name`: Set project name
- `-v, --verbose`: Enable verbose logging

## Compose File Extensions

### Init Containers

```yaml
services:
  myapp:
    image: myapp:latest
    init_containers:
      - name: migrate
        image: migrate/migrate
        command: ["-path", "/migrations", "-database", "$DATABASE_URL", "up"]
        volumes:
          - ./migrations:/migrations
```

### Post Containers

```yaml
services:
  myapp:
    image: myapp:latest
    post_containers:
      - name: healthcheck
        image: curlimages/curl
        command: ["curl", "-f", "http://myapp/health"]
        wait_for: "10s"
        on_success: true
      - name: cleanup
        image: busybox
        command: ["rm", "-rf", "/tmp/cache"]
        on_failure: true
```

### Lifecycle Hooks

```yaml
services:
  myapp:
    image: myapp:latest
    hooks:
      pre_start:
        - name: validate-config
          type: command
          command: ["./validate.sh"]
      post_start:
        - name: notify
          type: http
          http:
            url: "${WEBHOOK_URL}"
            method: POST
            body: '{"status": "started"}'
      pre_stop:
        - name: backup
          type: script
          script: |
            tar -czf backup.tar.gz /data
            aws s3 cp backup.tar.gz s3://backups/
```

### Cloud Native Configuration

```yaml
services:
  myapp:
    image: myapp:latest
    cloud_native:
      kubernetes:
        namespace: production
        annotations:
          prometheus.io/scrape: "true"
        resources:
          limits:
            cpu: "1"
            memory: "512Mi"
      helm:
        chart: myapp-chart
        repository: https://charts.example.com
        values:
          replicas: 3
          autoscaling:
            enabled: true
```

## Examples

See the `examples/` directory for complete examples:
- `simple-compose.yml`: Basic example with init containers and hooks
- `full-featured-compose.yml`: Comprehensive example showcasing all features

## Architecture

The project is organized into the following packages:

- `cmd/fake-compose`: CLI entry point
- `pkg/compose`: Compose file types and structures
- `pkg/container`: Docker container management
- `pkg/lifecycle`: Service lifecycle management
- `pkg/hooks`: Hook execution engine
- `internal/parser`: YAML parsing and validation
- `internal/executor`: Service orchestration

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License.