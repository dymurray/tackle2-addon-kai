# Tackle2 Addon Kai - Modernized

A modernized Konveyor addon that orchestrates AI-driven code migration using agents like Goose or OpenCode.

## Modernization Changes

This project has been updated to follow modern Go standards and conventions:

### Go Version
- Updated from Go 1.21 to Go 1.22
- Leverages modern Go features and improved performance

### Code Quality Improvements

#### Error Handling
- Added proper error wrapping with `fmt.Errorf` and `%w` verb
- Comprehensive input validation with meaningful error messages
- Context-aware error handling throughout the codebase

#### Structured Logging
- Enhanced activity logging with proper context
- Better command execution logging with input/output capture
- Consistent error reporting

#### Modern HTTP Client
- Updated fetch-analysis tool with context-aware HTTP requests
- Proper timeout handling and request cancellation
- Better error responses and status code handling

#### Code Organization
- Improved package structure and internal organization
- Better separation of concerns between components
- Enhanced data structures with proper validation

### Testing
- Added comprehensive unit tests for all packages
- Integration tests for end-to-end functionality
- Mock server tests for HTTP client functionality
- Performance benchmarks for critical functions
- Test coverage for error conditions and edge cases

### Documentation
- Improved Go doc comments following conventions
- Better inline code documentation
- Comprehensive test examples

## Architecture

### Components

1. **Main Addon** (`cmd/addon/`): Orchestrates the migration process
2. **Fetch Analysis** (`cmd/fetch-analysis/`): Retrieves analysis data from Konveyor Hub
3. **Repository Handler**: Manages Git repository operations with improved error handling

### Key Features

- **Multi-Agent Support**: Works with Goose and OpenCode AI agents
- **Flexible Configuration**: Supports various AI providers (OpenAI, Azure, etc.)
- **Robust Error Handling**: Comprehensive validation and error reporting
- **Modern HTTP Client**: Context-aware API communications
- **Git Integration**: Repository cloning, branching, and pushing capabilities

## Building

```bash
# Build the addon
go build -o addon ./cmd/addon

# Build the fetch-analysis tool
go build -o fetch-analysis ./cmd/fetch-analysis
```

## Testing

```bash
# Run unit tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run integration tests
go test -v ./integration_test.go

# Run with race detection
go test -race ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Usage

### Addon

The addon requires specific configuration through environment variables and input data structure:

```go
type Data struct {
    Branch string      `json:"branch"`
    Agent  AgentConfig `json:"agent"`
    Plan   PlanConfig  `json:"plan"`
}
```

### Fetch Analysis

The fetch-analysis tool retrieves migration analysis data:

```bash
export HUB_BASE_URL="https://hub.example.com"
export HUB_TOKEN="your-auth-token"
export APP_ID="123"
./fetch-analysis
```

## Environment Variables

### Common
- `HUB_BASE_URL`: Konveyor Hub API base URL
- `HUB_TOKEN`: Authentication token for Hub API

### Agent Configuration
- `GOOSE_PROVIDER`: AI provider (openai, azure, etc.)
- `GOOSE_MODEL`: Model name (gpt-4, gpt-3.5-turbo, etc.)
- `OPENAI_API_KEY`: OpenAI API key
- `AZURE_OPENAI_API_KEY`: Azure OpenAI API key
- `AZURE_OPENAI_ENDPOINT`: Azure OpenAI endpoint
- `AZURE_OPENAI_API_VERSION`: Azure OpenAI API version

## Error Handling

The modernized codebase includes comprehensive error handling:

- **Input Validation**: All functions validate inputs and return meaningful errors
- **Context Awareness**: HTTP requests and long-running operations support context cancellation
- **Error Wrapping**: Errors are properly wrapped to maintain context and stack traces
- **Graceful Degradation**: Operations fail safely with appropriate cleanup

## Performance

- **Efficient Repository Operations**: Optimized Git operations with better error handling
- **Timeout Management**: All network operations have appropriate timeouts
- **Memory Management**: Reduced memory allocations in hot paths
- **Concurrent Safety**: Thread-safe operations where applicable

## Dependencies

The project uses modern, well-maintained dependencies:

- `github.com/konveyor/tackle2-addon`: Core addon functionality
- `github.com/konveyor/tackle2-hub`: Hub API client
- Standard library packages for HTTP, JSON, and file operations

## Contributing

When contributing to this project:

1. Follow Go best practices and conventions
2. Add comprehensive tests for new functionality
3. Update documentation for any API changes
4. Ensure all tests pass before submitting
5. Use `go fmt` and `go vet` to check code quality

## License

This project maintains the same license as the original Konveyor project.
