package queue

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/devaloi/hookrelay/internal/domain"
	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// SQLiteStore implements the Store interface using SQLite.
type SQLiteStore struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewSQLiteStore creates a new SQLite-backed store.
func NewSQLiteStore(databaseURL string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", databaseURL+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(1) // SQLite works best with a single connection for writes
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	store := &SQLiteStore{db: db}

	if err := store.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return store, nil
}

// migrate runs database migrations.
func (s *SQLiteStore) migrate() error {
	_, err := s.db.Exec(migrationSQL)
	if err != nil {
		return fmt.Errorf("executing migration: %w", err)
	}
	return nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// generateID creates a new unique identifier.
func generateID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// Endpoint operations

// CreateEndpoint creates a new endpoint.
func (s *SQLiteStore) CreateEndpoint(endpoint *domain.Endpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if endpoint.ID == "" {
		endpoint.ID = generateID()
	}
	now := time.Now().UTC()
	endpoint.CreatedAt = now
	endpoint.UpdatedAt = now

	_, err := s.db.Exec(`
		INSERT INTO endpoints (id, name, slug, target_url, secret, active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, endpoint.ID, endpoint.Name, endpoint.Slug, endpoint.TargetURL, endpoint.Secret, endpoint.Active, endpoint.CreatedAt, endpoint.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return fmt.Errorf("endpoint with slug %q already exists", endpoint.Slug)
		}
		return fmt.Errorf("inserting endpoint: %w", err)
	}

	return nil
}

// GetEndpoint retrieves an endpoint by ID.
func (s *SQLiteStore) GetEndpoint(id string) (*domain.Endpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var endpoint domain.Endpoint
	var secret sql.NullString
	err := s.db.QueryRow(`
		SELECT id, name, slug, target_url, secret, active, created_at, updated_at
		FROM endpoints WHERE id = ?
	`, id).Scan(&endpoint.ID, &endpoint.Name, &endpoint.Slug, &endpoint.TargetURL, &secret, &endpoint.Active, &endpoint.CreatedAt, &endpoint.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("endpoint not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("querying endpoint: %w", err)
	}
	endpoint.Secret = secret.String

	return &endpoint, nil
}

// GetEndpointBySlug retrieves an endpoint by its slug.
func (s *SQLiteStore) GetEndpointBySlug(slug string) (*domain.Endpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var endpoint domain.Endpoint
	var secret sql.NullString
	err := s.db.QueryRow(`
		SELECT id, name, slug, target_url, secret, active, created_at, updated_at
		FROM endpoints WHERE slug = ?
	`, slug).Scan(&endpoint.ID, &endpoint.Name, &endpoint.Slug, &endpoint.TargetURL, &secret, &endpoint.Active, &endpoint.CreatedAt, &endpoint.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("endpoint not found: %s", slug)
	}
	if err != nil {
		return nil, fmt.Errorf("querying endpoint: %w", err)
	}
	endpoint.Secret = secret.String

	return &endpoint, nil
}

// ListEndpoints retrieves all endpoints.
func (s *SQLiteStore) ListEndpoints(activeOnly bool) ([]*domain.Endpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := "SELECT id, name, slug, target_url, secret, active, created_at, updated_at FROM endpoints"
	if activeOnly {
		query += " WHERE active = 1"
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("querying endpoints: %w", err)
	}
	defer rows.Close()

	var endpoints []*domain.Endpoint
	for rows.Next() {
		var endpoint domain.Endpoint
		var secret sql.NullString
		if err := rows.Scan(&endpoint.ID, &endpoint.Name, &endpoint.Slug, &endpoint.TargetURL, &secret, &endpoint.Active, &endpoint.CreatedAt, &endpoint.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning endpoint: %w", err)
		}
		endpoint.Secret = secret.String
		endpoints = append(endpoints, &endpoint)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating endpoints: %w", err)
	}

	return endpoints, nil
}

// UpdateEndpoint updates an existing endpoint.
func (s *SQLiteStore) UpdateEndpoint(endpoint *domain.Endpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	endpoint.UpdatedAt = time.Now().UTC()

	result, err := s.db.Exec(`
		UPDATE endpoints 
		SET name = ?, target_url = ?, secret = ?, active = ?, updated_at = ?
		WHERE id = ?
	`, endpoint.Name, endpoint.TargetURL, endpoint.Secret, endpoint.Active, endpoint.UpdatedAt, endpoint.ID)
	if err != nil {
		return fmt.Errorf("updating endpoint: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("endpoint not found: %s", endpoint.ID)
	}

	return nil
}

// DeleteEndpoint deletes an endpoint by ID.
func (s *SQLiteStore) DeleteEndpoint(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec("DELETE FROM endpoints WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting endpoint: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("endpoint not found: %s", id)
	}

	return nil
}

// Queue operations

// Enqueue adds a webhook to the queue and creates a pending delivery.
func (s *SQLiteStore) Enqueue(webhook *domain.Webhook, _ *domain.Endpoint) (*domain.Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if webhook.ID == "" {
		webhook.ID = generateID()
	}
	if webhook.ReceivedAt.IsZero() {
		webhook.ReceivedAt = time.Now().UTC()
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Insert webhook
	headersJSON, err := json.Marshal(webhook.Headers)
	if err != nil {
		headersJSON = []byte("{}")
	}

	_, err = tx.Exec(`
		INSERT INTO webhooks (id, endpoint_id, headers, payload, received_at)
		VALUES (?, ?, ?, ?, ?)
	`, webhook.ID, webhook.EndpointID, string(headersJSON), webhook.Payload, webhook.ReceivedAt)
	if err != nil {
		return nil, fmt.Errorf("inserting webhook: %w", err)
	}

	// Create pending delivery
	delivery := &domain.Delivery{
		ID:          generateID(),
		WebhookID:   webhook.ID,
		EndpointID:  webhook.EndpointID,
		Status:      domain.StatusPending,
		Attempts:    0,
		NextRetryAt: &webhook.ReceivedAt, // Ready immediately
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	_, err = tx.Exec(`
		INSERT INTO deliveries (id, webhook_id, endpoint_id, status, attempts, next_retry_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, delivery.ID, delivery.WebhookID, delivery.EndpointID, delivery.Status, delivery.Attempts, delivery.NextRetryAt, delivery.CreatedAt, delivery.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("inserting delivery: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return delivery, nil
}

// Dequeue retrieves up to limit pending deliveries ready for processing.
func (s *SQLiteStore) Dequeue(limit int) ([]*domain.DeliveryWithWebhook, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()

	rows, err := s.db.Query(`
		SELECT 
			d.id, d.webhook_id, d.endpoint_id, d.status, d.attempts, 
			d.last_attempt_at, d.next_retry_at, d.response_code, 
			d.response_body, d.error_message, d.created_at, d.updated_at,
			w.id, w.endpoint_id, w.headers, w.payload, w.received_at,
			e.id, e.name, e.slug, e.target_url, e.secret, e.active, e.created_at, e.updated_at
		FROM deliveries d
		JOIN webhooks w ON d.webhook_id = w.id
		JOIN endpoints e ON d.endpoint_id = e.id
		WHERE d.status IN ('pending', 'failed')
		  AND d.next_retry_at <= ?
		  AND e.active = 1
		ORDER BY d.next_retry_at ASC
		LIMIT ?
	`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("querying deliveries: %w", err)
	}
	defer rows.Close()

	var results []*domain.DeliveryWithWebhook
	for rows.Next() {
		var d domain.Delivery
		var w domain.Webhook
		var e domain.Endpoint
		var lastAttemptAt, nextRetryAt sql.NullTime
		var responseCode sql.NullInt32
		var responseBody, errorMessage, headersStr sql.NullString
		var endpointSecret sql.NullString

		err := rows.Scan(
			&d.ID, &d.WebhookID, &d.EndpointID, &d.Status, &d.Attempts,
			&lastAttemptAt, &nextRetryAt, &responseCode,
			&responseBody, &errorMessage, &d.CreatedAt, &d.UpdatedAt,
			&w.ID, &w.EndpointID, &headersStr, &w.Payload, &w.ReceivedAt,
			&e.ID, &e.Name, &e.Slug, &e.TargetURL, &endpointSecret, &e.Active, &e.CreatedAt, &e.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning delivery: %w", err)
		}

		if lastAttemptAt.Valid {
			d.LastAttemptAt = &lastAttemptAt.Time
		}
		if nextRetryAt.Valid {
			d.NextRetryAt = &nextRetryAt.Time
		}
		if responseCode.Valid {
			code := int(responseCode.Int32)
			d.ResponseCode = &code
		}
		d.ResponseBody = responseBody.String
		d.ErrorMessage = errorMessage.String
		if headersStr.Valid {
			w.Headers = json.RawMessage(headersStr.String)
		}
		e.Secret = endpointSecret.String

		results = append(results, &domain.DeliveryWithWebhook{
			Delivery: &d,
			Webhook:  &w,
			Endpoint: &e,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating deliveries: %w", err)
	}

	// Mark dequeued items as in-progress by updating last_attempt_at
	for _, item := range results {
		_, err := s.db.Exec(`
			UPDATE deliveries 
			SET last_attempt_at = ?, updated_at = ?
			WHERE id = ?
		`, now, now, item.Delivery.ID)
		if err != nil {
			return nil, fmt.Errorf("updating delivery attempt time: %w", err)
		}
	}

	return results, nil
}

// MarkDelivered marks a delivery as successfully delivered.
func (s *SQLiteStore) MarkDelivered(id string, responseCode int, responseBody string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	truncatedBody := truncateString(responseBody, 1000)

	result, err := s.db.Exec(`
		UPDATE deliveries 
		SET status = ?, response_code = ?, response_body = ?, 
		    attempts = attempts + 1, last_attempt_at = ?, next_retry_at = NULL, updated_at = ?
		WHERE id = ?
	`, domain.StatusDelivered, responseCode, truncatedBody, now, now, id)
	if err != nil {
		return fmt.Errorf("marking delivery as delivered: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("delivery not found: %s", id)
	}

	return nil
}

// MarkFailed marks a delivery as failed with the next retry time.
func (s *SQLiteStore) MarkFailed(id string, deliveryErr error, responseCode *int, nextRetry *time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	errMsg := ""
	if deliveryErr != nil {
		errMsg = deliveryErr.Error()
	}

	var respCode sql.NullInt32
	if responseCode != nil {
		respCode = sql.NullInt32{Int32: int32(*responseCode), Valid: true}
	}

	var nextRetryAt sql.NullTime
	if nextRetry != nil {
		nextRetryAt = sql.NullTime{Time: *nextRetry, Valid: true}
	}

	result, err := s.db.Exec(`
		UPDATE deliveries 
		SET status = ?, error_message = ?, response_code = ?,
		    attempts = attempts + 1, last_attempt_at = ?, next_retry_at = ?, updated_at = ?
		WHERE id = ?
	`, domain.StatusFailed, errMsg, respCode, now, nextRetryAt, now, id)
	if err != nil {
		return fmt.Errorf("marking delivery as failed: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("delivery not found: %s", id)
	}

	return nil
}

// MarkDead moves a delivery to the dead letter queue.
func (s *SQLiteStore) MarkDead(id string, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()

	result, err := s.db.Exec(`
		UPDATE deliveries 
		SET status = ?, error_message = ?, next_retry_at = NULL, updated_at = ?
		WHERE id = ?
	`, domain.StatusDead, reason, now, id)
	if err != nil {
		return fmt.Errorf("marking delivery as dead: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("delivery not found: %s", id)
	}

	return nil
}

// GetDelivery retrieves a delivery by ID.
func (s *SQLiteStore) GetDelivery(id string) (*domain.Delivery, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var d domain.Delivery
	var lastAttemptAt, nextRetryAt sql.NullTime
	var responseCode sql.NullInt32
	var responseBody, errorMessage sql.NullString

	err := s.db.QueryRow(`
		SELECT id, webhook_id, endpoint_id, status, attempts, 
		       last_attempt_at, next_retry_at, response_code, 
		       response_body, error_message, created_at, updated_at
		FROM deliveries WHERE id = ?
	`, id).Scan(
		&d.ID, &d.WebhookID, &d.EndpointID, &d.Status, &d.Attempts,
		&lastAttemptAt, &nextRetryAt, &responseCode,
		&responseBody, &errorMessage, &d.CreatedAt, &d.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("delivery not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("querying delivery: %w", err)
	}

	if lastAttemptAt.Valid {
		d.LastAttemptAt = &lastAttemptAt.Time
	}
	if nextRetryAt.Valid {
		d.NextRetryAt = &nextRetryAt.Time
	}
	if responseCode.Valid {
		code := int(responseCode.Int32)
		d.ResponseCode = &code
	}
	d.ResponseBody = responseBody.String
	d.ErrorMessage = errorMessage.String

	return &d, nil
}

// ListDeliveries retrieves deliveries with optional filtering.
func (s *SQLiteStore) ListDeliveries(status *domain.DeliveryStatus, limit, offset int) ([]*domain.Delivery, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, webhook_id, endpoint_id, status, attempts, 
		       last_attempt_at, next_retry_at, response_code, 
		       response_body, error_message, created_at, updated_at
		FROM deliveries
	`
	var args []interface{}
	if status != nil {
		query += " WHERE status = ?"
		args = append(args, *status)
	}
	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying deliveries: %w", err)
	}
	defer rows.Close()

	return s.scanDeliveries(rows)
}

// ListDeadLetters retrieves deliveries in the dead letter queue.
func (s *SQLiteStore) ListDeadLetters(limit, offset int) ([]*domain.Delivery, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, webhook_id, endpoint_id, status, attempts, 
		       last_attempt_at, next_retry_at, response_code, 
		       response_body, error_message, created_at, updated_at
		FROM deliveries
		WHERE status = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, domain.StatusDead, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("querying dead letters: %w", err)
	}
	defer rows.Close()

	return s.scanDeliveries(rows)
}

// RetryDelivery resets a failed or dead delivery for retry.
func (s *SQLiteStore) RetryDelivery(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()

	result, err := s.db.Exec(`
		UPDATE deliveries 
		SET status = ?, next_retry_at = ?, error_message = '', updated_at = ?
		WHERE id = ? AND status IN (?, ?)
	`, domain.StatusPending, now, now, id, domain.StatusFailed, domain.StatusDead)
	if err != nil {
		return fmt.Errorf("retrying delivery: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("delivery not found or not in retryable state: %s", id)
	}

	return nil
}

// Stats returns queue statistics.
func (s *SQLiteStore) Stats() (*domain.QueueStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var stats domain.QueueStats

	err := s.db.QueryRow(`
		SELECT 
			COALESCE(SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END), 0) as pending,
			COALESCE(SUM(CASE WHEN status = 'delivered' THEN 1 ELSE 0 END), 0) as delivered,
			COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0) as failed,
			COALESCE(SUM(CASE WHEN status = 'dead' THEN 1 ELSE 0 END), 0) as dead
		FROM deliveries
	`).Scan(&stats.Pending, &stats.Delivered, &stats.Failed, &stats.Dead)
	if err != nil {
		return nil, fmt.Errorf("querying stats: %w", err)
	}

	stats.TotalDeliverd = stats.Delivered
	total := stats.Delivered + stats.Failed + stats.Dead
	if total > 0 {
		stats.SuccessRate = float64(stats.Delivered) / float64(total) * 100
	}

	return &stats, nil
}

// scanDeliveries is a helper to scan multiple deliveries from rows.
func (s *SQLiteStore) scanDeliveries(rows *sql.Rows) ([]*domain.Delivery, error) {
	var deliveries []*domain.Delivery
	for rows.Next() {
		var d domain.Delivery
		var lastAttemptAt, nextRetryAt sql.NullTime
		var responseCode sql.NullInt32
		var responseBody, errorMessage sql.NullString

		err := rows.Scan(
			&d.ID, &d.WebhookID, &d.EndpointID, &d.Status, &d.Attempts,
			&lastAttemptAt, &nextRetryAt, &responseCode,
			&responseBody, &errorMessage, &d.CreatedAt, &d.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning delivery: %w", err)
		}

		if lastAttemptAt.Valid {
			d.LastAttemptAt = &lastAttemptAt.Time
		}
		if nextRetryAt.Valid {
			d.NextRetryAt = &nextRetryAt.Time
		}
		if responseCode.Valid {
			code := int(responseCode.Int32)
			d.ResponseCode = &code
		}
		d.ResponseBody = responseBody.String
		d.ErrorMessage = errorMessage.String

		deliveries = append(deliveries, &d)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating deliveries: %w", err)
	}

	return deliveries, nil
}

// truncateString truncates a string to the specified length.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// migrationSQL contains the database schema.
const migrationSQL = `
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
`
