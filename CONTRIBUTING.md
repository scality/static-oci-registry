# Contributing to Static OCI Registry

This documents provides the guidelines for contributing to this project.

## Project Layout

```text
├── cmd/                    # Application entry point and startup logic
├── pkg/infrastructure/     # Infrastructure layer: framework integrations (logging, DI, server setup)
├── pkg/domain              # Domain layer: core business logic, entities, and interfaces
├── pkg/presentation        # Presentation layer: HTTP handlers, API endpoints, input/output formatting
├── pkg/service             # Service layer: business rules, orchestration, and coordination
├── pkg/usecase             # Use case layer: application-specific operations, user actions
└── Dockerfile              # Containerization instructions
```

## Development Setup

### Prerequisites
- Docker
- go

### Building

You can build the dockerfile directly or run
```bash
go build cmd/main.go
```
