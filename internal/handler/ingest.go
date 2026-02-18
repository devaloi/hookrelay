package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/devaloi/hookrelay/internal/domain"
	"github.com/devaloi/hookrelay/internal/queue"
	"github.com/devaloi/hookrelay/internal/signature"
)

// IngestHandler handles incoming webhook requests.
type IngestHandler struct {
	store  queue.Store
	logger *slog.Logger
}

// NewIngestHandler creates a new ingest handler.
func NewIngestHandler(store queue.Store, logger *slog.Logger) *IngestHandler {
	return &IngestHandler{
		store:  store,
		logger: logger,
	}
}

// ServeHTTP handles POST /webhooks/:endpoint_id requests.
func (h *IngestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract endpoint slug from path: /webhooks/:slug
	path := strings.TrimPrefix(r.URL.Path, "/webhooks/")
	slug := strings.TrimSuffix(path, "/")
	if slug == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "endpoint slug required"})
		return
	}

	// Get endpoint
	endpoint, err := h.store.GetEndpointBySlug(slug)
	if err != nil {
		h.logger.Warn("endpoint not found", "slug", slug, "error", err)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "endpoint not found"})
		return
	}

	if !endpoint.Active {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "endpoint not active"})
		return
	}

	// Read payload
	payload, err := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		h.logger.Error("reading payload", "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read payload"})
		return
	}

	// Verify signature if endpoint has a secret
	if endpoint.Secret != "" {
		sig := r.Header.Get(signature.SignatureHeader)
		if err := signature.Verify(payload, sig, endpoint.Secret); err != nil {
			h.logger.Warn("signature verification failed", "endpoint", slug, "error", err)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid signature"})
			return
		}
	}

	// Extract headers
	headers := make(map[string]string)
	for key, values := range r.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	headersJSON, _ := json.Marshal(headers)

	// Create webhook
	webhook := &domain.Webhook{
		EndpointID: endpoint.ID,
		Headers:    headersJSON,
		Payload:    payload,
	}

	// Enqueue for delivery
	delivery, err := h.store.Enqueue(webhook, endpoint)
	if err != nil {
		h.logger.Error("enqueueing webhook", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to enqueue webhook"})
		return
	}

	h.logger.Info("webhook received",
		"endpoint", slug,
		"webhook_id", webhook.ID,
		"delivery_id", delivery.ID,
	)

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"message":     "webhook accepted",
		"webhook_id":  webhook.ID,
		"delivery_id": delivery.ID,
	})
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
