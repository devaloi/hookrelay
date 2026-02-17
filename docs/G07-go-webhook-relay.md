# G07: hookrelay — Webhook Relay Service

**Catalog ID:** G07 | **Size:** M | **Language:** Go
**Repo name:** `hookrelay`
**One-liner:** A lightweight Go service that receives, queues, and delivers webhooks with configurable retries, exponential backoff, and a dead letter queue.

---

## Why This Stands Out

- **Shows distributed systems thinking** — retries, backoff, idempotency, DLQ
- **Production patterns** — graceful shutdown, health checks, structured logging, metrics
- **Clean Go architecture** — interfaces, dependency injection, table-driven tests
- **SQLite for persistence** — no external dependencies, single binary
- **Demonstrates** queue processing, HTTP client hardening, failure handling
- **Useful standalone tool** — receive GitHub/Stripe/etc webhooks and relay to internal services

---

## Architecture

```
hookrelay/
├── cmd/
│   └── server/
│       └── main.go            # Entry point, wire dependencies, graceful shutdown
├── internal/
│   ├── config/
│   │   └── config.go          # Env-based configuration
│   ├── domain/
│   │   └── webhook.go         # Core types: Webhook, Delivery, Endpoint
│   ├── handler/
│   │   ├── ingest.go          # POST /webhooks/:endpoint — receive webhooks
│   │   ├── admin.go           # Admin API (list, retry, stats)
│   │   ├── health.go          # GET /health
│   │   └── handler_test.go
│   ├── queue/
│   │   ├── queue.go           # Queue interface
│   │   ├── sqlite.go          # SQLite-backed persistent queue
│   │   └── sqlite_test.go
│   ├── worker/
│   │   ├── dispatcher.go      # Poll queue, dispatch deliveries
│   │   ├── deliverer.go       # HTTP client with timeout, retry logic
│   │   └── worker_test.go
│   ├── middleware/
│   │   ├── logging.go
│   │   ├── requestid.go
│   │   └── recovery.go
│   └── signature/
│       ├── hmac.go            # HMAC-SHA256 signature verification/generation
│       └── hmac_test.go
├── migrations/
│   └── 001_create_tables.sql
├── main.go
├── go.mod
├── go.sum
├── Makefile
├── .env.example
├── .gitignore
├── .golangci.yml
├── LICENSE
└── README.md
```

---

## How It Works

```
[GitHub / Stripe / etc]
        │
        ▼
  POST /webhooks/:endpoint_id
        │
        ▼
  ┌─────────────┐
  │  Ingest API  │ → Validate signature, store payload
  └──────┬──────┘
         │
         ▼
  ┌─────────────┐
  │   Queue      │ → SQLite persistent queue
  └──────┬──────┘
         │
         ▼
  ┌─────────────┐     ┌───────────────┐
  │  Dispatcher  │────▶│  Deliverer    │ → HTTP POST to target URL
  └──────┬──────┘     └───────┬───────┘
         │                     │
         │              success│fail
         │                 ┌───┴───┐
         │                 │Retry? │
         │                 └───┬───┘
         │              yes│     │no (max retries)
         │                 ▼     ▼
         │            Re-queue  DLQ
         │
         ▼
  ┌─────────────┐
  │  Admin API   │ → List deliveries, retry failed, view DLQ
  └─────────────┘
```

---

## API Endpoints

### Ingest
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/webhooks/:endpoint_id` | Receive a webhook payload |

### Admin
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/admin/endpoints` | List configured endpoints |
| `POST` | `/admin/endpoints` | Create endpoint (target URL, secret) |
| `GET` | `/admin/deliveries` | List recent deliveries |
| `GET` | `/admin/deliveries/:id` | Get delivery detail (payload, attempts) |
| `POST` | `/admin/deliveries/:id/retry` | Retry a failed delivery |
| `GET` | `/admin/dlq` | List dead-lettered deliveries |
| `POST` | `/admin/dlq/:id/retry` | Retry from dead letter queue |
| `GET` | `/health` | Health check |
| `GET` | `/stats` | Queue depth, delivery counts, success rate |

---

## Data Model

```
endpoints
  id, name, slug, target_url, secret (for HMAC), active, created_at

webhooks
  id, endpoint_id, headers (JSON), payload (TEXT), received_at

deliveries
  id, webhook_id, endpoint_id, status (pending/delivered/failed/dead),
  attempts, last_attempt_at, next_retry_at, response_code,
  response_body (truncated), error_message, created_at
```

---

## Retry Strategy

- Max retries: configurable (default 5)
- Backoff: exponential with jitter
  - Attempt 1: ~30s
  - Attempt 2: ~1m
  - Attempt 3: ~4m
  - Attempt 4: ~15m
  - Attempt 5: ~1h
- Formula: `base * 2^attempt + random_jitter`
- After max retries → move to dead letter queue
- Dead letter items can be manually retried via admin API

---

## Phases

### Phase 1: Scaffold & Core Types

**1.1 — Project setup**
- `go mod init github.com/devaloi/hookrelay`
- Dependencies: `github.com/mattn/go-sqlite3`
- stdlib HTTP only (no frameworks)
- Create Makefile, .gitignore, .golangci.yml

**1.2 — Domain types**
- `Endpoint`: id, name, slug, target_url, secret, active
- `Webhook`: id, endpoint_id, headers, payload, received_at
- `Delivery`: id, webhook_id, status, attempts, next_retry_at, response_code, error

**1.3 — Config**
- Env-based: PORT, DATABASE_URL, MAX_RETRIES, WORKER_INTERVAL, DELIVERY_TIMEOUT
- Sensible defaults

### Phase 2: Queue & Storage

**2.1 — Queue interface**
```go
type Queue interface {
    Enqueue(webhook *Webhook) error
    Dequeue(limit int) ([]*Delivery, error)
    MarkDelivered(id string) error
    MarkFailed(id string, err error, nextRetry time.Time) error
    MarkDead(id string) error
    Stats() (*QueueStats, error)
}
```

**2.2 — SQLite implementation**
- Migrations: create tables on startup
- Enqueue: insert webhook + pending delivery
- Dequeue: SELECT WHERE status=pending AND next_retry_at <= now
- Atomic status transitions
- Dead letter query

### Phase 3: Worker & Delivery

**3.1 — Dispatcher**
- Background goroutine, polls queue on configurable interval
- Dequeue batch of pending deliveries
- Pass to deliverer
- Handle graceful shutdown (finish in-flight, stop polling)

**3.2 — Deliverer**
- HTTP POST to endpoint target_url
- Set headers: Content-Type, X-Hook-ID, X-Hook-Signature, X-Hook-Delivery
- Timeout per delivery (configurable, default 30s)
- Success: 2xx → mark delivered
- Failure: non-2xx or timeout → increment attempts, calculate next retry
- Max retries exceeded → mark dead

**3.3 — HMAC signatures**
- Sign payloads with endpoint secret using HMAC-SHA256
- `X-Hook-Signature: sha256=<hex>`
- Verify incoming webhooks if endpoint has a secret configured

### Phase 4: HTTP Handlers

**4.1 — Ingest handler**
- `POST /webhooks/:endpoint_id` — receive payload, verify signature if configured, enqueue
- Return 202 Accepted with delivery ID

**4.2 — Admin handlers**
- CRUD for endpoints
- List/detail for deliveries
- DLQ management
- Stats endpoint

**4.3 — Middleware**
- Request ID, logging, recovery (reuse patterns from shrink)

### Phase 5: Tests

**5.1 — Unit tests**
- Queue: enqueue, dequeue, mark transitions, stats
- Deliverer: mock HTTP server — success, failure, timeout, retries
- HMAC: sign, verify, invalid signature
- Retry calculation: exponential backoff values

**5.2 — Integration test**
- Start test HTTP target server
- Ingest webhook → dispatcher processes → target receives payload
- Test failure path: target returns 500 → retries → eventually dead letters

### Phase 6: Documentation & Polish

**6.1 — README.md**
- Architecture diagram (ASCII)
- Quick start: create endpoint → send webhook → check delivery
- API reference with curl examples
- Retry strategy explanation
- Configuration reference
- Development commands

**6.2 — Final checks**
- `go build` / `go test -race` / `golangci-lint` all clean
- Fresh clone → build → run → curl test works
- No hardcoded paths, no secrets in code

---

## Tech Stack

| Component | Choice |
|-----------|--------|
| HTTP | Go stdlib net/http |
| Database | SQLite via go-sqlite3 |
| Config | Environment variables |
| Signatures | crypto/hmac (stdlib) |
| Testing | stdlib testing + httptest |
| Linting | golangci-lint |

---

## Commit Plan

1. `feat: scaffold project with config and domain types`
2. `feat: add SQLite queue with migrations`
3. `feat: add HMAC signature signing and verification`
4. `feat: add ingest handler for receiving webhooks`
5. `feat: add dispatcher and deliverer with retry logic`
6. `feat: add admin API endpoints`
7. `test: add queue, deliverer, and HMAC unit tests`
8. `test: add integration test for full delivery pipeline`
9. `refactor: error handling, logging, code cleanup`
10. `docs: add README with architecture and API reference`
11. `chore: final lint pass and cleanup`
