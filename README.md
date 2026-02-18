# HookRelay

[![Go](https://img.shields.io/badge/Go-1.22-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A lightweight webhook relay service that receives, queues, and reliably delivers webhooks with signature verification and exponential backoff retry logic.

## Features

- **Webhook Ingestion** - Receive webhooks via HTTP POST with automatic queuing
- **HMAC Signature Verification** - Verify incoming webhooks using SHA-256 signatures
- **Persistent SQLite Queue** - Durable storage with WAL mode for high concurrency
- **Configurable Retry Logic** - Exponential backoff with jitter (up to 5 retries by default)
- **Dead Letter Queue (DLQ)** - Failed webhooks moved to DLQ after max retries
- **Admin API** - Manage endpoints, view deliveries, inspect DLQ
- **Graceful Shutdown** - Clean shutdown with in-flight delivery completion
- **Structured Logging** - JSON logging with request IDs for observability

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│   Webhook   │────▶│   Ingest     │────▶│   SQLite    │
│   Source    │     │   Handler    │     │   Queue     │
└─────────────┘     └──────────────┘     └──────┬──────┘
                                                │
                    ┌──────────────┐            │
                    │   Target     │◀───────────┤
                    │   Service    │            │
                    └──────────────┘     ┌──────┴──────┐
                                         │  Dispatcher │
                                         │  + Deliverer│
                                         └─────────────┘
```

### Key Components

| Component | Description |
|-----------|-------------|
| **Ingest Handler** | Receives webhooks, validates signatures, enqueues for delivery |
| **SQLite Queue** | Persistent FIFO queue with delivery tracking |
| **Dispatcher** | Polls queue, coordinates delivery workers |
| **Deliverer** | HTTP client with timeout, retry logic, signature forwarding |
| **Admin Handler** | CRUD endpoints, delivery inspection, DLQ management |

## Quick Start

### Prerequisites

- Go 1.22+
- CGO enabled (for SQLite)

### Installation

```bash
git clone https://github.com/devaloi/hookrelay.git
cd hookrelay
make build
```

### Configuration

Create a `.env` file or set environment variables:

```bash
PORT=8080                    # HTTP server port
DATABASE_URL=hookrelay.db    # SQLite database path
MAX_RETRIES=5                # Maximum delivery attempts
RETRY_DELAY=1s               # Initial retry delay
DELIVERY_TIMEOUT=30s         # HTTP delivery timeout
POLL_INTERVAL=100ms          # Queue polling interval
```

### Running

```bash
# Development
make run

# Production
./bin/hookrelay
```

## API Reference

### Webhook Ingestion

**POST /webhooks/{endpoint_slug}**

Receive and queue a webhook for delivery.

```bash
curl -X POST http://localhost:8080/webhooks/my-endpoint \
  -H "Content-Type: application/json" \
  -H "X-Hub-Signature-256: sha256=<signature>" \
  -d '{"event": "push", "data": {}}'
```

Response:
```json
{
  "webhook_id": "abc123",
  "delivery_id": "def456",
  "status": "queued"
}
```

### Admin Endpoints

**POST /admin/endpoints** - Create a new endpoint

```bash
curl -X POST http://localhost:8080/admin/endpoints \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My Webhook",
    "slug": "my-webhook",
    "target_url": "https://api.example.com/webhook",
    "secret": "optional-hmac-secret"
  }'
```

**GET /admin/endpoints** - List all endpoints

**GET /admin/endpoints/{id}** - Get endpoint details

**DELETE /admin/endpoints/{id}** - Delete an endpoint

**GET /admin/deliveries** - List recent deliveries

**GET /admin/dlq** - List dead letter queue entries

**POST /admin/dlq/{id}/retry** - Retry a DLQ entry

### Health & Stats

**GET /health** - Health check

```json
{
  "status": "healthy",
  "database": "connected"
}
```

**GET /stats** - Queue statistics

```json
{
  "pending": 12,
  "in_flight": 3,
  "delivered": 1547,
  "failed": 23,
  "dead": 2
}
```

## Signature Verification

HookRelay supports HMAC-SHA256 signature verification for incoming webhooks. When an endpoint has a `secret` configured, incoming requests must include a valid signature in the `X-Hub-Signature-256` header.

**Signature Format:**
```
X-Hub-Signature-256: sha256=<hex-encoded-hmac>
```

**Verification Process:**
1. Extract signature from header
2. Compute HMAC-SHA256 of request body using endpoint secret
3. Compare signatures using constant-time comparison
4. Reject request if signatures don't match

Outgoing deliveries include the same signature header computed with the endpoint's secret.

## Retry Logic

Failed deliveries are retried with exponential backoff:

| Attempt | Delay |
|---------|-------|
| 1 | 1s |
| 2 | 2s |
| 3 | 4s |
| 4 | 8s |
| 5 | 16s |

After `MAX_RETRIES` failures, the delivery is moved to the Dead Letter Queue (DLQ) for manual inspection and retry.

**Retryable Failures:**
- HTTP 5xx responses
- Network timeouts
- Connection errors

**Non-Retryable Failures:**
- HTTP 4xx responses (except 429)
- Invalid endpoint configuration

## Database Schema

```sql
-- Endpoints table
CREATE TABLE endpoints (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    target_url TEXT NOT NULL,
    secret TEXT,
    active INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Webhooks table  
CREATE TABLE webhooks (
    id TEXT PRIMARY KEY,
    endpoint_id TEXT NOT NULL,
    headers TEXT,
    payload BLOB,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (endpoint_id) REFERENCES endpoints(id)
);

-- Deliveries table
CREATE TABLE deliveries (
    id TEXT PRIMARY KEY,
    webhook_id TEXT NOT NULL,
    endpoint_id TEXT NOT NULL,
    status TEXT DEFAULT 'pending',
    attempts INTEGER DEFAULT 0,
    last_error TEXT,
    next_attempt_at DATETIME,
    delivered_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (webhook_id) REFERENCES webhooks(id),
    FOREIGN KEY (endpoint_id) REFERENCES endpoints(id)
);
```

## Development

### Project Structure

```
hookrelay/
├── cmd/server/         # Application entry point
├── internal/
│   ├── config/         # Environment configuration
│   ├── domain/         # Core types and interfaces
│   ├── handler/        # HTTP handlers
│   ├── middleware/     # HTTP middleware
│   ├── queue/          # SQLite queue implementation
│   ├── signature/      # HMAC signing/verification
│   └── worker/         # Dispatcher and deliverer
├── migrations/         # SQL migrations
├── Makefile
└── README.md
```

### Commands

```bash
make build      # Build binary
make run        # Run server
make test       # Run tests
make test-race  # Run tests with race detector
make lint       # Run linter
make clean      # Clean build artifacts
```

### Testing

```bash
# Run all tests
make test

# Run with race detector
make test-race

# Run specific package
go test -v ./internal/queue/...

# Run with coverage
go test -cover ./...
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests and linting
5. Submit a pull request

## License

MIT License - see [LICENSE](LICENSE) for details.
