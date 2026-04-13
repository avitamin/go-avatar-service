# Avatar Service - Project Context

## Project Overview

**Avatar Service** ("Аватарница") is a Go-based microservice for managing user avatars. It provides REST API functionality for uploading, retrieving, and deleting user avatar images. The project also includes a simple web interface for interacting with the service.

This is a **template repository** designed as a graduation project for a "Go Developer" course. It contains a basic project structure ready for further development, following Go best practices.

## Architecture

The project follows a standard Go service architecture with separation of concerns:

- **cmd/** - Application entry points
  - `cmd/server/` - HTTP server binary
  - `cmd/worker/` - Async worker binary for background task processing
  
- **internal/** - Internal application logic (to be developed)
  - `api/` - API specifications (OpenAPI/Swagger)
  - `config/` - Application configuration
  - `domain/` - Domain entities
  - `handlers/` - HTTP handlers
  - `repository/` - Storage layer (database, S3)
  - `services/` - Business logic
  - `worker/` - Worker logic

- **web/** - Web interface
  - `web/static/index.html` - Single-page avatar upload UI (Tailwind CSS styled)

- **Other directories** (planned):
  - `pkg/` - Public libraries
  - `migrations/` - Database migrations
  - `docker/` - Docker configurations
  - `k8s/` - Kubernetes manifests
  - `tests/` - Integration and e2e tests
  - `docs/` - Project documentation

## Technologies

- **Language:** Go 1.25.1
- **Web UI:** HTML/JS with Tailwind CSS (via CDN)
- **Infrastructure:** Docker Compose, Kubernetes (planned)

## Building and Running

### Prerequisites

- Go 1.25.1 or later
- Docker and Docker Compose (for containerized deployment)

### Build Commands

```bash
# Install/update dependencies
go mod tidy

# Build server binary
go build -o ./bin/server ./cmd/server

# Build worker binary
go build -o ./bin/worker ./cmd/worker

# Run server
go run ./cmd/server

# Run worker
go run ./cmd/worker
```

### Docker Deployment

```bash
# Start all services
docker-compose up --build
```

After starting, the service will be available at `http://localhost:8080`.

### Web Interface

The web UI is served at `http://localhost:8080/`. It provides a form to:
- Enter a User ID
- Upload an avatar image
- View API responses

The frontend sends requests to `/api/v1/avatars` with `X-User-ID` header and multipart form data.

## Configuration

Create a `.env` file based on `.env.example` (to be created) with:
- Database connection credentials
- S3/storage configuration
- Other environment-specific settings

Note: `.env` is gitignored.

## Development Conventions

### Project Structure

Follow the standard Go project layout:
- `cmd/` for main applications
- `internal/` for private application code
- `pkg/` for reusable public libraries

### Coding Style

- Follow standard Go conventions and idioms
- Use structured logging
- Implement proper error handling
- Keep business logic in `internal/services/`
- Keep HTTP handlers thin, delegating to services

### Testing

- Unit tests should be placed alongside the code they test (`*_test.go`)
- Integration and e2e tests go in `tests/`

## API Endpoints (Planned)

Based on the web interface, the API should include:

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/avatars` | Upload avatar (requires `X-User-ID` header) |
| `GET` | `/api/v1/avatars/:userId` | Get user's avatar |
| `DELETE` | `/api/v1/avatars/:userId` | Delete user's avatar |

## Current State

This is a **template/skeleton project** with:
- ✅ Basic project structure
- ✅ Minimal HTTP server (placeholder)
- ✅ Minimal worker (placeholder)
- ✅ Web UI for avatar upload
- ❌ Full API implementation (pending)
- ❌ Database integration (pending)
- ❌ S3 storage integration (pending)
- ❌ Worker task processing (pending)
- ❌ Tests (pending)
- ❌ Docker configuration (pending)

## Key Files

| File | Description |
|------|-------------|
| `go.mod` | Go module definition |
| `cmd/server/main.go` | Server entry point |
| `cmd/worker/main.go` | Worker entry point |
| `web/static/index.html` | Avatar upload web interface |
| `.gitignore` | Git ignore rules |
| `README.md` | Project documentation (Russian) |
