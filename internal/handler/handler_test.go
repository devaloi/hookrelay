package handler

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/devaloi/hookrelay/internal/domain"
	"github.com/devaloi/hookrelay/internal/queue"
	"github.com/devaloi/hookrelay/internal/signature"
)

func setupTestStore(t *testing.T) (*queue.SQLiteStore, func()) { //nolint:gocritic // unnamedResult is fine for test helpers
	t.Helper()
	tmpFile, err := os.CreateTemp("", "handler_test_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()

	store, err := queue.NewSQLiteStore(tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("failed to create store: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.Remove(tmpFile.Name())
	}

	return store, cleanup
}

func TestIngestHandler(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewIngestHandler(store, logger)

	endpoint := &domain.Endpoint{
		Name:      "Test Endpoint",
		Slug:      "test-ingest",
		TargetURL: "http://example.com/webhook",
		Active:    true,
	}
	store.CreateEndpoint(endpoint)

	t.Run("successful webhook ingestion", func(t *testing.T) {
		payload := []byte(`{"event":"test","data":{"id":123}}`)
		req := httptest.NewRequest(http.MethodPost, "/webhooks/test-ingest", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Errorf("expected status 202, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp["webhook_id"] == nil {
			t.Error("expected webhook_id in response")
		}
		if resp["delivery_id"] == nil {
			t.Error("expected delivery_id in response")
		}
	})

	t.Run("endpoint not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhooks/nonexistent", bytes.NewReader([]byte(`{}`)))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/webhooks/test-ingest", http.NoBody)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", rec.Code)
		}
	})

	t.Run("missing endpoint slug", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhooks/", http.NoBody)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rec.Code)
		}
	})
}

func TestIngestHandlerWithSignature(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewIngestHandler(store, logger)

	secret := "supersecret123"
	endpoint := &domain.Endpoint{
		Name:      "Signed Endpoint",
		Slug:      "signed",
		TargetURL: "http://example.com/signed",
		Secret:    secret,
		Active:    true,
	}
	store.CreateEndpoint(endpoint)

	t.Run("valid signature", func(t *testing.T) {
		payload := []byte(`{"signed":"data"}`)
		sig := signature.Sign(payload, secret)
		req := httptest.NewRequest(http.MethodPost, "/webhooks/signed", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(signature.SignatureHeader, sig)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Errorf("expected status 202, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid signature", func(t *testing.T) {
		payload := []byte(`{"signed":"data"}`)
		req := httptest.NewRequest(http.MethodPost, "/webhooks/signed", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(signature.SignatureHeader, "sha256=invalid")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("missing signature when required", func(t *testing.T) {
		payload := []byte(`{"signed":"data"}`)
		req := httptest.NewRequest(http.MethodPost, "/webhooks/signed", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})
}

func TestHealthHandler(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewHealthHandler(store, logger)

	t.Run("healthy", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
		var resp map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp["status"] != "healthy" {
			t.Errorf("expected status healthy, got %v", resp["status"])
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/health", http.NoBody)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", rec.Code)
		}
	})
}

func TestAdminHandler(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewAdminHandler(store, logger)

	t.Run("create and list endpoints", func(t *testing.T) {
		body := `{"name":"Admin Test","slug":"admin-test","target_url":"http://example.com"}`
		req := httptest.NewRequest(http.MethodPost, "/admin/endpoints", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
		}

		req = httptest.NewRequest(http.MethodGet, "/admin/endpoints", http.NoBody)
		rec = httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("list deliveries", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/deliveries", http.NoBody)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("list DLQ", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/dlq", http.NoBody)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})
}

func TestStatsHandler(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewStatsHandler(store, logger)

	t.Run("get stats", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/stats", http.NoBody)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
		var stats domain.QueueStats
		if err := json.NewDecoder(rec.Body).Decode(&stats); err != nil {
			t.Errorf("failed to decode stats: %v", err)
		}
	})
}
