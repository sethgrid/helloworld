# helloworld
## An example HTTP Server

A production-ready Go HTTP server template demonstrating modern Go patterns and best practices.

### Features

**Core Functionality:**
  - Fully unit and unit-integration testable JSON HTTP web server
  - Modern handler pattern with dependency injection (following [Grafana's approach](https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/))
  - Graceful shutdown with proper resource cleanup
  - Context propagation throughout request handling
  - Health check endpoint with database connectivity verification

**Observability:**
  - Structured logging with `slog`
  - Key-value error data via `kverr` package for better structured error logging
  - Prometheus metrics for HTTP requests and database connection pools
  - Metrics labeled by data store (taskqueue, events) for better observability
  - Grafana and Loki integration for log aggregation

**Testing:**
  - Able to spin up multiple running HTTP servers in parallel during tests
  - Able to assert against the server's logged contents
  - Fakes as test doubles, demonstrating practical alternatives to mock frameworks
  - Comprehensive middleware tests
  - Table-driven tests for maintainability

**Architecture:**
  - Internal packages (`/internal/`) for implementation details
  - Each component responsible for its own observability
  - Dependency injection at route definition time
  - No global state (except where explicitly documented)
  
### Architecture

**Handler Pattern:**
  - Handlers are standalone functions that receive dependencies via closure
  - Dependencies are injected at route definition: `router.Get("/", handleHelloworld(eventStore))`
  - Logger is injected via middleware and accessed through request context
  - No handlers are methods on the Server struct

**Package Structure:**
  - `/internal/taskqueue/` - Task queue implementation (internal)
  - `/internal/events/` - Event store implementation (internal)
  - `/server/` - HTTP server and handlers
  - `/logger/` - Structured logging utilities
  - `/metrics/` - Prometheus metrics definitions

**Design Choices:**
  - Test servers take Options for flexible configuration (log buffers, custom configs, etc.)
  - Logger is placed in request context via middleware
  - Each data store (taskqueue, events) manages its own database connection metrics
  - Metrics use `store` label to distinguish between different data stores
  - Graceful shutdown ensures all components (HTTP servers, event store, task queue) are properly closed

## Getting Started

### Prerequisites

Install `make`, `docker`, `docker compose`, and `go` (1.23+).

Recommended: `alias dc='docker compose'`

### Quick Start

```bash
# Terminal 1: Start database
dc up mysql
# or
docker compose up -d mysql

# Terminal 2: Start server
source settings.env
go run cmd/helloworld/main.go

# Terminal 3: Test the server
curl localhost:16666/
# {"hello":"World!","event_store_available":true,"event_store_message":"Event store is available"}

curl localhost:16667/healthcheck
# 200 OK (or 503 if database is unreachable)
```

### Configuration

Configuration is loaded from environment variables with the `HELLOWORLD_` prefix. See `settings_example.env` for available options.

**Key Configuration Options:**
- `HELLOWORLD_DB_HOST` - Database host (default: `127.0.0.1`)
- `HELLOWORLD_DB_USER` - Database user (default: `testuser`)
- `HELLOWORLD_DB_PASS` - Database password (default: `testuser`)
- `HELLOWORLD_DB_NAME` - Database name (default: `helloworld`)
- `HELLOWORLD_PORT` - Public HTTP port (default: `16666`)
- `HELLOWORLD_INTERNAL_PORT` - Internal metrics/health port (default: `16667`)
- `HELLOWORLD_REQUEST_TIMEOUT` - Request timeout duration (default: `30s`)
- `HELLOWORLD_ENABLE_DEBUG` - Enable debug logging (default: `true`)
- `HELLOWORLD_SHOULD_SECURE` - Enable secure cookies and TLS (default: `false`)

**Security Notes:**
- When `HELLOWORLD_SHOULD_SECURE=true`, `HELLOWORLD_DB_CA_CERT_PATH` must be set
- Passwords are never logged (DSN is masked in debug logs)
- CORS uses explicit origins even in development mode

### Observability

**Metrics:**
- Prometheus metrics available at `http://localhost:16667/metrics`
- HTTP metrics: request count, duration, in-flight requests
- Database connection pool metrics labeled by store (`taskqueue`, `events`)

**Logs:**
- Structured JSON logging via `slog`
- Request-scoped loggers with request IDs
- Errors include structured key-value pairs via `kverr`

**Grafana Dashboard:**
```bash
dc up -d mysql grafana
go run cmd/helloworld/main.go >> ./logs/helloworld.logs
# Open http://localhost:3000
```

Metrics are polled from the internal port `/metrics` to Prometheus where Grafana displays data. Similarly, Promtail follows logs and pushes them into Loki, where Grafana can display the results.

**Note:** This requires Prometheus and Promtail to be able to reach the host system outside the docker container.


### Testing

**Unit Tests:**
```bash
go test ./...
# or
make test
```

**Test Types:**
- **Unit tests** - Fast, isolated tests with fakes/mocks
- **Middleware tests** - Test CORS, timeout, and other middleware
- **Integration tests** - Test against real database
- **Unit-integration tests** - Headless browser tests from unit test framework

**Integration Testing:**
```bash
# May need to prefix with "sudo" if you get permission errors
make test-integration           # Tear down DB, reseed, rebuild, and test blackbox style
make test-integration-docker    # Run everything in docker, like CI/CD
```

**Test Features:**
- Dynamic port allocation for parallel test execution
- Log buffer assertions for verifying logged content
- Graceful shutdown testing
- Context timeout and propagation testing


## API Endpoints

### Public Endpoints

- `GET /` - Hello world endpoint
  - Query params: `?delay=<duration>` - Simulate delayed response (e.g., `?delay=1s`)
  - Response: `{"hello":"World!","event_store_available":bool,"event_store_message":string}`

### Internal Endpoints

- `GET /healthcheck` - Health check endpoint
  - Returns `200 OK` if database is reachable
  - Returns `503 Service Unavailable` if database is unreachable
- `GET /metrics` - Prometheus metrics endpoint

## Deployment

### Building

```bash
# Build binary with version from VERSION file
make build
# Binary will be in ./bin/helloworld
```

### Docker

```bash
# Build and run with docker compose
docker compose build helloworld
docker compose up helloworld
```

### Production Considerations

- Set `HELLOWORLD_SHOULD_SECURE=true` for production
- Configure `HELLOWORLD_DB_CA_CERT_PATH` for secure database connections
- Use explicit CORS origins (not wildcards)
- Set appropriate timeouts (`HELLOWORLD_REQUEST_TIMEOUT`, `HELLOWORLD_SHUTDOWN_TIMEOUT`)
- Monitor metrics at `/metrics` endpoint
- Ensure graceful shutdown is respected by orchestration (Kubernetes, etc.)

## Development

### Code Style

- Handlers receive dependencies via closure, not as methods on Server
- Logger is injected via middleware, accessed through request context
- Errors are wrapped with context using `fmt.Errorf("...: %w", err)`
- Use `kverr` for structured error data
- Each component manages its own observability (metrics, logging)

### Project Structure

```
.
├── cmd/
│   └── helloworld/          # Main application entry point
├── internal/
│   ├── events/              # Event store implementation
│   ├── taskqueue/           # Task queue implementation
│   └── util/                # Internal utilities
├── server/                  # HTTP server and handlers
├── logger/                  # Logging utilities
├── metrics/                 # Prometheus metrics
└── migrations/              # Database migrations
```

## TODOs

 - [ ] Make a testcase showing grpc handling
 - [ ] Make a testcase showing graphql handling
 - [ ] UI route handling? or just keep this as a JSON api server?



