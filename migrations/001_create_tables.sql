-- Migration: Create initial tables for hookrelay

-- Endpoints table: stores webhook destination configurations
CREATE TABLE IF NOT EXISTS endpoints (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    target_url TEXT NOT NULL,
    secret TEXT,
    active INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_endpoints_slug ON endpoints(slug);
CREATE INDEX IF NOT EXISTS idx_endpoints_active ON endpoints(active);

-- Webhooks table: stores incoming webhook payloads
CREATE TABLE IF NOT EXISTS webhooks (
    id TEXT PRIMARY KEY,
    endpoint_id TEXT NOT NULL,
    headers TEXT NOT NULL,
    payload BLOB NOT NULL,
    received_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (endpoint_id) REFERENCES endpoints(id)
);

CREATE INDEX IF NOT EXISTS idx_webhooks_endpoint_id ON webhooks(endpoint_id);
CREATE INDEX IF NOT EXISTS idx_webhooks_received_at ON webhooks(received_at);

-- Deliveries table: tracks delivery attempts for each webhook
CREATE TABLE IF NOT EXISTS deliveries (
    id TEXT PRIMARY KEY,
    webhook_id TEXT NOT NULL,
    endpoint_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    last_attempt_at DATETIME,
    next_retry_at DATETIME,
    response_code INTEGER,
    response_body TEXT,
    error_message TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (webhook_id) REFERENCES webhooks(id),
    FOREIGN KEY (endpoint_id) REFERENCES endpoints(id)
);

CREATE INDEX IF NOT EXISTS idx_deliveries_status ON deliveries(status);
CREATE INDEX IF NOT EXISTS idx_deliveries_next_retry_at ON deliveries(next_retry_at);
CREATE INDEX IF NOT EXISTS idx_deliveries_webhook_id ON deliveries(webhook_id);
CREATE INDEX IF NOT EXISTS idx_deliveries_endpoint_id ON deliveries(endpoint_id);
CREATE INDEX IF NOT EXISTS idx_deliveries_status_retry ON deliveries(status, next_retry_at);
