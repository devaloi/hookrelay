package queue

import (
	"os"
	"testing"
	"time"

	"github.com/devaloi/hookrelay/internal/domain"
)

func TestSQLiteStore(t *testing.T) {
	dbPath := "./test_hookrelay.db"
	defer os.Remove(dbPath)

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	t.Run("CreateAndGetEndpoint", func(t *testing.T) {
		endpoint := &domain.Endpoint{
			Name:      "Test Endpoint",
			Slug:      "test-endpoint",
			TargetURL: "http://example.com/webhook",
			Secret:    "secret123",
			Active:    true,
		}

		err := store.CreateEndpoint(endpoint)
		if err != nil {
			t.Fatalf("failed to create endpoint: %v", err)
		}
		if endpoint.ID == "" {
			t.Error("endpoint ID should be set")
		}

		got, err := store.GetEndpoint(endpoint.ID)
		if err != nil {
			t.Fatalf("failed to get endpoint: %v", err)
		}
		if got.Name != endpoint.Name {
			t.Errorf("expected name %q, got %q", endpoint.Name, got.Name)
		}
		if got.Slug != endpoint.Slug {
			t.Errorf("expected slug %q, got %q", endpoint.Slug, got.Slug)
		}
	})

	t.Run("GetEndpointBySlug", func(t *testing.T) {
		endpoint := &domain.Endpoint{
			Name:      "Slug Test",
			Slug:      "slug-test",
			TargetURL: "http://example.com/webhook2",
			Active:    true,
		}
		store.CreateEndpoint(endpoint)

		got, err := store.GetEndpointBySlug("slug-test")
		if err != nil {
			t.Fatalf("failed to get endpoint by slug: %v", err)
		}
		if got.Name != endpoint.Name {
			t.Errorf("expected name %q, got %q", endpoint.Name, got.Name)
		}
	})

	t.Run("ListEndpoints", func(t *testing.T) {
		endpoints, err := store.ListEndpoints(false)
		if err != nil {
			t.Fatalf("failed to list endpoints: %v", err)
		}
		if len(endpoints) < 2 {
			t.Errorf("expected at least 2 endpoints, got %d", len(endpoints))
		}
	})

	t.Run("EnqueueAndDequeue", func(t *testing.T) {
		endpoint := &domain.Endpoint{
			Name:      "Queue Test",
			Slug:      "queue-test",
			TargetURL: "http://example.com/queue",
			Active:    true,
		}
		store.CreateEndpoint(endpoint)

		webhook := &domain.Webhook{
			EndpointID: endpoint.ID,
			Headers:    []byte(`{"Content-Type":"application/json"}`),
			Payload:    []byte(`{"event":"test"}`),
			ReceivedAt: time.Now(),
		}

		delivery, err := store.Enqueue(webhook, endpoint)
		if err != nil {
			t.Fatalf("failed to enqueue webhook: %v", err)
		}
		if delivery.ID == "" {
			t.Error("delivery ID should be set")
		}
		if delivery.Status != domain.StatusPending {
			t.Errorf("expected status %q, got %q", domain.StatusPending, delivery.Status)
		}

		items, err := store.Dequeue(10)
		if err != nil {
			t.Fatalf("failed to dequeue: %v", err)
		}
		found := false
		for _, item := range items {
			if item.Delivery.ID == delivery.ID {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find newly created delivery in dequeue results")
		}
	})

	t.Run("MarkDelivered", func(t *testing.T) {
		endpoint := &domain.Endpoint{
			Name:      "Delivered Test",
			Slug:      "delivered-test",
			TargetURL: "http://example.com/delivered",
			Active:    true,
		}
		store.CreateEndpoint(endpoint)

		webhook := &domain.Webhook{
			EndpointID: endpoint.ID,
			Payload:    []byte(`{"event":"delivered"}`),
		}
		delivery, _ := store.Enqueue(webhook, endpoint)

		err := store.MarkDelivered(delivery.ID, 200, "OK")
		if err != nil {
			t.Fatalf("failed to mark delivered: %v", err)
		}

		got, _ := store.GetDelivery(delivery.ID)
		if got.Status != domain.StatusDelivered {
			t.Errorf("expected status %q, got %q", domain.StatusDelivered, got.Status)
		}
	})

	t.Run("MarkFailed", func(t *testing.T) {
		endpoint := &domain.Endpoint{
			Name:      "Failed Test",
			Slug:      "failed-test",
			TargetURL: "http://example.com/failed",
			Active:    true,
		}
		store.CreateEndpoint(endpoint)

		webhook := &domain.Webhook{
			EndpointID: endpoint.ID,
			Payload:    []byte(`{"event":"failed"}`),
		}
		delivery, _ := store.Enqueue(webhook, endpoint)

		nextRetry := time.Now().Add(time.Hour)
		responseCode := 500
		err := store.MarkFailed(delivery.ID, nil, &responseCode, &nextRetry)
		if err != nil {
			t.Fatalf("failed to mark failed: %v", err)
		}

		got, _ := store.GetDelivery(delivery.ID)
		if got.Status != domain.StatusFailed {
			t.Errorf("expected status %q, got %q", domain.StatusFailed, got.Status)
		}
	})

	t.Run("MarkDead", func(t *testing.T) {
		endpoint := &domain.Endpoint{
			Name:      "Dead Test",
			Slug:      "dead-test",
			TargetURL: "http://example.com/dead",
			Active:    true,
		}
		store.CreateEndpoint(endpoint)

		webhook := &domain.Webhook{
			EndpointID: endpoint.ID,
			Payload:    []byte(`{"event":"dead"}`),
		}
		delivery, _ := store.Enqueue(webhook, endpoint)

		err := store.MarkDead(delivery.ID, "max retries exceeded")
		if err != nil {
			t.Fatalf("failed to mark dead: %v", err)
		}

		got, _ := store.GetDelivery(delivery.ID)
		if got.Status != domain.StatusDead {
			t.Errorf("expected status %q, got %q", domain.StatusDead, got.Status)
		}
	})

	t.Run("RetryDelivery", func(t *testing.T) {
		endpoint := &domain.Endpoint{
			Name:      "Retry Test",
			Slug:      "retry-test",
			TargetURL: "http://example.com/retry",
			Active:    true,
		}
		store.CreateEndpoint(endpoint)

		webhook := &domain.Webhook{
			EndpointID: endpoint.ID,
			Payload:    []byte(`{"event":"retry"}`),
		}
		delivery, _ := store.Enqueue(webhook, endpoint)
		store.MarkDead(delivery.ID, "test")

		err := store.RetryDelivery(delivery.ID)
		if err != nil {
			t.Fatalf("failed to retry delivery: %v", err)
		}

		got, _ := store.GetDelivery(delivery.ID)
		if got.Status != domain.StatusPending {
			t.Errorf("expected status %q, got %q", domain.StatusPending, got.Status)
		}
	})

	t.Run("Stats", func(t *testing.T) {
		stats, err := store.Stats()
		if err != nil {
			t.Fatalf("failed to get stats: %v", err)
		}
		if stats.Pending < 0 || stats.Delivered < 0 || stats.Failed < 0 || stats.Dead < 0 {
			t.Error("stats should not be negative")
		}
	})

	t.Run("DuplicateSlug", func(t *testing.T) {
		endpoint1 := &domain.Endpoint{
			Name:      "Dup Test 1",
			Slug:      "dup-slug",
			TargetURL: "http://example.com/dup1",
			Active:    true,
		}
		store.CreateEndpoint(endpoint1)

		endpoint2 := &domain.Endpoint{
			Name:      "Dup Test 2",
			Slug:      "dup-slug",
			TargetURL: "http://example.com/dup2",
			Active:    true,
		}
		err := store.CreateEndpoint(endpoint2)
		if err == nil {
			t.Error("expected error for duplicate slug")
		}
	})
}
