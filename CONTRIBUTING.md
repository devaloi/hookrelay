# Contributing to HookRelay

Thank you for your interest in contributing to HookRelay! This document provides guidelines and instructions for contributing.

## Development Setup

### Prerequisites

- Go 1.22 or later
- CGO enabled (required for SQLite)
- golangci-lint (for linting)

### Getting Started

1. Fork and clone the repository:
   ```bash
   git clone https://github.com/YOUR_USERNAME/hookrelay.git
   cd hookrelay
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Run tests to verify setup:
   ```bash
   make test
   ```

## Making Changes

### Code Style

- Follow standard Go conventions and idioms
- Run `gofmt` and `goimports` before committing
- All exported types and functions must have doc comments
- Keep functions focused and small

### Testing

- Write tests for new functionality
- Ensure existing tests pass: `make test`
- Run with race detector: `make test-race`
- Aim for reasonable coverage on new code

### Linting

Run the linter before submitting:
```bash
make lint
```

All code must pass `golangci-lint` with zero issues.

## Commit Guidelines

We follow [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` New features
- `fix:` Bug fixes
- `docs:` Documentation changes
- `test:` Test additions or modifications
- `refactor:` Code changes that neither fix bugs nor add features
- `chore:` Maintenance tasks

Examples:
```
feat: add webhook retry delay configuration
fix: handle nil pointer in delivery handler
docs: update API reference for new endpoints
test: add integration tests for queue
```

## Pull Request Process

1. Create a feature branch from `main`
2. Make your changes with clear, atomic commits
3. Ensure all tests pass and linting is clean
4. Update documentation if needed
5. Submit a PR with a clear description of changes

### PR Checklist

- [ ] Tests pass locally (`make test`)
- [ ] Linting passes (`make lint`)
- [ ] Documentation updated (if applicable)
- [ ] Commit messages follow conventions
- [ ] PR description explains the changes

## Project Structure

```
hookrelay/
├── cmd/server/         # Application entry point
├── internal/
│   ├── config/         # Environment configuration
│   ├── domain/         # Core types and interfaces
│   ├── handler/        # HTTP handlers
│   ├── middleware/     # HTTP middleware (logging, recovery, request ID)
│   ├── queue/          # Queue interface and SQLite implementation
│   ├── signature/      # HMAC signing and verification
│   └── worker/         # Dispatcher and HTTP deliverer
├── migrations/         # SQL migration files
└── docs/               # Additional documentation
```

## Questions?

Open an issue for questions or discussion about potential changes.
