# Tackle2 Addon Kai

[![CI](https://github.com/konveyor/tackle2-addon-kai/actions/workflows/ci.yml/badge.svg)](https://github.com/konveyor/tackle2-addon-kai/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/konveyor/tackle2-addon-kai)](https://goreportcard.com/report/github.com/konveyor/tackle2-addon-kai)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

An AI-powered code migration addon for Konveyor Tackle2 that orchestrates automated code transformations using AI agents.

## Overview

Tackle2 Addon Kai is a migration automation tool that integrates with the Konveyor Tackle2 platform to perform intelligent code migrations. It coordinates AI agents (Goose or OpenCode) to analyze source code, generate migration plans, and execute transformations with minimal human intervention.

## Features

- **AI Agent Integration**: Supports both Goose and OpenCode AI agents for code migration
- **Automated Workflow**: Fetches source code, generates migration plans, and commits changes
- **Pallet Skills**: Integrates with AI skill repositories for enhanced migration capabilities
- **Analysis Integration**: Fetches and processes analysis results from Tackle2 Hub
- **Non-interactive Execution**: Designed for automated CI/CD pipelines
- **Comprehensive Testing**: Full test coverage with modern Go testing practices

## Architecture

The addon consists of two main components:

### 1. Migration Orchestrator (`cmd/addon`)

The main addon that:
- Fetches application source code from SCM repositories
- Configures AI agents with migration context and skills
- Writes migration plans with execution preambles
- Orchestrates AI agent execution
- Commits migration results to target branches

### 2. Analysis Fetcher (`cmd/fetch-analysis`)

A utility tool that:
- Retrieves analysis results from Konveyor Hub API
- Formats analysis data for AI agent consumption
- Validates API responses and handles errors gracefully

## Prerequisites

- Go 1.22 or later
- Docker or Podman (for containerized builds)
- Access to Konveyor Tackle2 Hub instance
- SCM repository with appropriate credentials

## Installation

### From Source

```bash
git clone https://github.com/konveyor/tackle2-addon-kai.git
cd tackle2-addon-kai
make build
```

### Using Docker

```bash
docker build -t tackle2-addon-kai .
```

## Usage

### Migration Addon

The migration addon is typically invoked by the Tackle2 platform with configuration data:

```bash
./bin/addon
```

Required environment variables and configuration are provided by the Tackle2 platform.

### Analysis Fetcher

The fetch-analysis tool requires specific environment variables:

```bash
export HUB_BASE_URL="https://your-tackle2-hub.example.com"
export HUB_TOKEN="your-api-token"
export APP_ID="123"

./bin/fetch-analysis
```

## Configuration

The addon accepts configuration through a JSON payload that includes:

- **Agent Configuration**: AI agent type (goose/opencode), model settings, API keys
- **Plan Configuration**: Migration plan details, objectives, context
- **Pallet Configuration**: Skills repository settings and synchronization
- **Repository Settings**: SCM details, authentication, target branches

Example configuration structure:

```json
{
  "agent": {
    "name": "goose",
    "model": {
      "name": "gpt-4",
      "provider": "openai",
      "api_key": "sk-..."
    }
  },
  "plan": {
    "name": "spring-boot-migration",
    "markdown": "# Migration Plan\\n..."
  },
  "pallet": {
    "yaml": "skills:\\n  - name: java-migration\\n..."
  }
}
```

## Development

### Prerequisites

- Go 1.22+
- Make
- Git

### Setup

```bash
git clone https://github.com/konveyor/tackle2-addon-kai.git
cd tackle2-addon-kai
make deps
```

### Building

```bash
make build
```

### Testing

Run all tests:
```bash
make test
```

Run tests with coverage:
```bash
make test-coverage
```

Run benchmarks:
```bash
make bench
```

### Code Quality

Format code:
```bash
make fmt
```

Run linting:
```bash
make lint
```

Run static analysis:
```bash
make vet
```

### Available Make Targets

```bash
make help
```

## Testing

The project includes comprehensive test coverage:

- **Unit Tests**: Test individual functions and methods
- **Integration Tests**: Test component interactions (when addon framework is available)
- **Validation Tests**: Test configuration validation logic
- **File I/O Tests**: Test YAML marshaling and file operations
- **HTTP Tests**: Test API interactions with mock servers

Run tests with race detection:
```bash
go test -race ./...
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Add tests for your changes
5. Ensure all tests pass (`make test`)
6. Run code quality checks (`make lint`)
7. Commit your changes (`git commit -m 'Add amazing feature'`)
8. Push to the branch (`git push origin feature/amazing-feature`)
9. Open a Pull Request

### Code Standards

- Follow standard Go conventions
- Add tests for new functionality
- Ensure code passes all quality checks
- Update documentation as needed

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## Support

- **Issues**: Report bugs and request features via [GitHub Issues](https://github.com/konveyor/tackle2-addon-kai/issues)
- **Documentation**: See the [Konveyor documentation](https://konveyor.io/docs/)
- **Community**: Join the [Konveyor community](https://konveyor.io/community/)

## Related Projects

- [Konveyor Tackle2](https://github.com/konveyor/tackle2-hub) - Application modernization platform
- [Konveyor Tackle2 Addon](https://github.com/konveyor/tackle2-addon) - Addon framework
- [Kai](https://github.com/konveyor/kai) - AI-powered modernization engine

