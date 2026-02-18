package worker

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/devaloi/hookrelay/internal/config"
	"github.com/devaloi/hookrelay/internal/domain"
)

func TestDeliverer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.Config{
		DeliveryTimeout:  5 * time.Second,
		RetryBackoffBase: 30,
	}

	t.Run("successful delivery", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("expected Content-Type application/json")
			}
			if r.Header.Get("X-Hook-ID") == "" {
				t.Error("expected X-Hook-ID header")
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}))
		defer server.Close()

		deliverer := NewDeliverer(cfg, logger)
		item := &domain.DeliveryWithWebhook{
			Delivery: &domain.Delivery{ID: "del-1", WebhookID: "wh-1"},
			Webhook:  &domain.Webhook{ID: "wh-1", Payload: []byte(`{"test":true}`)},
			Endpoint: &domain.Endpoint{ID: "ep-1", TargetURL: server.URL},
		}

		result := deliverer.Deliver(item)

		if !result.Success {
			t.Errorf("expected success, got error: %v", result.Error)
		}
		if result.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", result.StatusCode)
		}
	})

	t.Run("server returns 500", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"internal error"}`))
		}))
		defer server.Close()

		deliverer := NewDeliverer(cfg, logger)
		item := &domain.DeliveryWithWebhook{
			Delivery: &domain.Delivery{ID: "del-2"},
			Webhook:  &domain.Webhook{ID: "wh-2", Payload: []byte(`{}`)},
			Endpoint: &domain.Endpoint{ID: "ep-2", TargetURL: server.URL},
		}

		result := deliverer.Deliver(item)

		if result.Success {
			t.Error("expected failure for 500 response")
		}
		if result.StatusCode != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", result.StatusCode)
		}
		if result.Error == nil {
			t.Error("expected error to be set")
		}
	})

	t.Run("server returns 404", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		deliverer := NewDeliverer(cfg, logger)
		item := &domain.DeliveryWithWebhook{
			Delivery: &domain.Delivery{ID: "del-3"},
			Webhook:  &domain.Webhook{ID: "wh-3", Payload: []byte(`{}`)},
			Endpoint: &domain.Endpoint{ID: "ep-3", TargetURL: server.URL},
		}

		result := deliverer.Deliver(item)

		if result.Success {
			t.Error("expected failure for 404 response")
		}
		if result.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", result.StatusCode)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		shortCfg := &config.Config{
			DeliveryTimeout:  100 * time.Millisecond,
			RetryBackoffBase: 30,
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(500 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		deliverer := NewDeliverer(shortCfg, logger)
		item := &domain.DeliveryWithWebhook{
			Delivery: &domain.Delivery{ID: "del-4"},
			Webhook:  &domain.Webhook{ID: "wh-4", Payload: []byte(`{}`)},
			Endpoint: &domain.Endpoint{ID: "ep-4", TargetURL: server.URL},
		}

		result := deliverer.Deliver(item)

		if result.Success {
			t.Error("expected failure for timeout")
		}
		if result.Error == nil {
			t.Error("expected error to be set for timeout")
		}
	})

	t.Run("invalid URL", func(t *testing.T) {
		deliverer := NewDeliverer(cfg, logger)
		item := &domain.DeliveryWithWebhook{
			Delivery: &domain.Delivery{ID: "del-5"},
			Webhook:  &domain.Webhook{ID: "wh-5", Payload: []byte(`{}`)},
			Endpoint: &domain.Endpoint{ID: "ep-5", TargetURL: "http://invalid.invalid.invalid:12345"},
		}

		result := deliverer.Deliver(item)

		if result.Success {
			t.Error("expected failure for invalid URL")
		}
		if result.Error == nil {
			t.Error("expected error to be set")
		}
	})

	t.Run("with signature", func(t *testing.T) {
		var gotSig string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotSig = r.Header.Get("X-Hook-Signature")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		deliverer := NewDeliverer(cfg, logger)
		item := &domain.DeliveryWithWebhook{
			Delivery: &domain.Delivery{ID: "del-6"},
			Webhook:  &domain.Webhook{ID: "wh-6", Payload: []byte(`{"signed":true}`)},
			Endpoint: &domain.Endpoint{ID: "ep-6", TargetURL: server.URL, Secret: "testsecret"},
		}

		result := deliverer.Deliver(item)

		if !result.Success {
			t.Errorf("expected success, got error: %v", result.Error)
		}
		if gotSig == "" {
			t.Error("expected signature header to be set")
		}
		if len(gotSig) < 10 {
			t.Errorf("signature seems too short: %s", gotSig)
		}
	})
}
