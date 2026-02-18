// Package domain defines the core types for the webhook relay service.
package domain

import (
	"encoding/json"
	"time"
)

// DeliveryStatus represents the state of a webhook delivery.
type DeliveryStatus string

const (
	// StatusPending indicates the delivery is waiting to be processed.
	StatusPending DeliveryStatus = "pending"
	// StatusDelivered indicates the delivery was successful.
	StatusDelivered DeliveryStatus = "delivered"
	// StatusFailed indicates the delivery failed but may be retried.
	StatusFailed DeliveryStatus = "failed"
	// StatusDead indicates the delivery has exhausted all retries.
	StatusDead DeliveryStatus = "dead"
)

// Endpoint represents a webhook destination configuration.
type Endpoint struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	TargetURL string    `json:"target_url"`
	Secret    string    `json:"-"` // Excluded from JSON for security
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Webhook represents an incoming webhook payload.
type Webhook struct {
	ID         string          `json:"id"`
	EndpointID string          `json:"endpoint_id"`
	Headers    json.RawMessage `json:"headers"`
	Payload    []byte          `json:"payload"`
	ReceivedAt time.Time       `json:"received_at"`
}

// Delivery represents a webhook delivery attempt to a target.
type Delivery struct {
	ID            string         `json:"id"`
	WebhookID     string         `json:"webhook_id"`
	EndpointID    string         `json:"endpoint_id"`
	Status        DeliveryStatus `json:"status"`
	Attempts      int            `json:"attempts"`
	LastAttemptAt *time.Time     `json:"last_attempt_at,omitempty"`
	NextRetryAt   *time.Time     `json:"next_retry_at,omitempty"`
	ResponseCode  *int           `json:"response_code,omitempty"`
	ResponseBody  string         `json:"response_body,omitempty"`
	ErrorMessage  string         `json:"error_message,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// QueueStats contains statistics about the delivery queue.
type QueueStats struct {
	Pending       int64   `json:"pending"`
	Delivered     int64   `json:"delivered"`
	Failed        int64   `json:"failed"`
	Dead          int64   `json:"dead"`
	TotalDelivered int64  `json:"total_delivered"`
	SuccessRate   float64 `json:"success_rate"`
}

// DeliveryWithWebhook combines a delivery with its associated webhook data.
type DeliveryWithWebhook struct {
	Delivery *Delivery
	Webhook  *Webhook
	Endpoint *Endpoint
}

// EndpointCreateRequest represents the request body for creating an endpoint.
type EndpointCreateRequest struct {
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	TargetURL string `json:"target_url"`
	Secret    string `json:"secret,omitempty"`
}

// EndpointUpdateRequest represents the request body for updating an endpoint.
type EndpointUpdateRequest struct {
	Name      *string `json:"name,omitempty"`
	TargetURL *string `json:"target_url,omitempty"`
	Secret    *string `json:"secret,omitempty"`
	Active    *bool   `json:"active,omitempty"`
}
