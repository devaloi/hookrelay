// Package queue provides the interface and implementations for webhook delivery queuing.
package queue

import (
	"time"

	"github.com/devaloi/hookrelay/internal/domain"
)

// Queue defines the interface for webhook delivery queuing operations.
type Queue interface {
	// Enqueue adds a webhook to the queue and creates a pending delivery.
	// Returns the created delivery ID.
	Enqueue(webhook *domain.Webhook, endpoint *domain.Endpoint) (*domain.Delivery, error)

	// Dequeue retrieves up to limit pending deliveries ready for processing.
	// Only returns deliveries where next_retry_at <= now.
	Dequeue(limit int) ([]*domain.DeliveryWithWebhook, error)

	// MarkDelivered marks a delivery as successfully delivered.
	MarkDelivered(id string, responseCode int, responseBody string) error

	// MarkFailed marks a delivery as failed with the next retry time.
	// If nextRetry is nil, no more retries will be attempted.
	MarkFailed(id string, err error, responseCode *int, nextRetry *time.Time) error

	// MarkDead moves a delivery to the dead letter queue.
	MarkDead(id string, reason string) error

	// GetDelivery retrieves a delivery by ID.
	GetDelivery(id string) (*domain.Delivery, error)

	// ListDeliveries retrieves deliveries with optional filtering.
	ListDeliveries(status *domain.DeliveryStatus, limit, offset int) ([]*domain.Delivery, error)

	// ListDeadLetters retrieves deliveries in the dead letter queue.
	ListDeadLetters(limit, offset int) ([]*domain.Delivery, error)

	// RetryDelivery resets a failed or dead delivery for retry.
	RetryDelivery(id string) error

	// Stats returns queue statistics.
	Stats() (*domain.QueueStats, error)

	// Close closes the queue and releases resources.
	Close() error
}

// EndpointStore defines operations for managing webhook endpoints.
type EndpointStore interface {
	// CreateEndpoint creates a new endpoint.
	CreateEndpoint(endpoint *domain.Endpoint) error

	// GetEndpoint retrieves an endpoint by ID.
	GetEndpoint(id string) (*domain.Endpoint, error)

	// GetEndpointBySlug retrieves an endpoint by its slug.
	GetEndpointBySlug(slug string) (*domain.Endpoint, error)

	// ListEndpoints retrieves all endpoints.
	ListEndpoints(activeOnly bool) ([]*domain.Endpoint, error)

	// UpdateEndpoint updates an existing endpoint.
	UpdateEndpoint(endpoint *domain.Endpoint) error

	// DeleteEndpoint deletes an endpoint by ID.
	DeleteEndpoint(id string) error
}

// Store combines Queue and EndpointStore interfaces.
type Store interface {
	Queue
	EndpointStore
}
