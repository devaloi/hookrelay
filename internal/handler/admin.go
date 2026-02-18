package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/devaloi/hookrelay/internal/domain"
	"github.com/devaloi/hookrelay/internal/queue"
)

type AdminHandler struct {
	store  queue.Store
	logger *slog.Logger
}

func NewAdminHandler(store queue.Store, logger *slog.Logger) *AdminHandler {
	return &AdminHandler{store: store, logger: logger}
}

func (h *AdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/admin")

	switch {
	case path == "/endpoints" || path == "/endpoints/":
		h.handleEndpoints(w, r)
	case strings.HasPrefix(path, "/endpoints/"):
		h.handleEndpoint(w, r, strings.TrimPrefix(path, "/endpoints/"))
	case path == "/deliveries" || path == "/deliveries/":
		h.handleDeliveries(w, r)
	case strings.HasPrefix(path, "/deliveries/") && strings.HasSuffix(path, "/retry"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/deliveries/"), "/retry")
		h.handleRetryDelivery(w, r, id)
	case strings.HasPrefix(path, "/deliveries/"):
		h.handleDelivery(w, r, strings.TrimPrefix(path, "/deliveries/"))
	case path == "/dlq" || path == "/dlq/":
		h.handleDLQ(w, r)
	case strings.HasPrefix(path, "/dlq/") && strings.HasSuffix(path, "/retry"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/dlq/"), "/retry")
		h.handleRetryDLQ(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

func (h *AdminHandler) handleEndpoints(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		endpoints, err := h.store.ListEndpoints(false)
		if err != nil {
			h.logger.Error("listing endpoints", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, endpoints)
	case http.MethodPost:
		var req domain.EndpointCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		endpoint := &domain.Endpoint{
			Name:      req.Name,
			Slug:      req.Slug,
			TargetURL: req.TargetURL,
			Secret:    req.Secret,
			Active:    true,
		}
		if err := h.store.CreateEndpoint(endpoint); err != nil {
			h.logger.Error("creating endpoint", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, endpoint)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *AdminHandler) handleEndpoint(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		endpoint, err := h.store.GetEndpoint(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "endpoint not found"})
			return
		}
		writeJSON(w, http.StatusOK, endpoint)
	case http.MethodDelete:
		if err := h.store.DeleteEndpoint(id); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "endpoint not found"})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *AdminHandler) handleDeliveries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	var status *domain.DeliveryStatus
	if s := r.URL.Query().Get("status"); s != "" {
		st := domain.DeliveryStatus(s)
		status = &st
	}

	deliveries, err := h.store.ListDeliveries(status, limit, offset)
	if err != nil {
		h.logger.Error("listing deliveries", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, deliveries)
}

func (h *AdminHandler) handleDelivery(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	delivery, err := h.store.GetDelivery(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "delivery not found"})
		return
	}
	writeJSON(w, http.StatusOK, delivery)
}

func (h *AdminHandler) handleRetryDelivery(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := h.store.RetryDelivery(id); err != nil {
		h.logger.Error("retrying delivery", "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "delivery queued for retry"})
}

func (h *AdminHandler) handleDLQ(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	deliveries, err := h.store.ListDeadLetters(limit, offset)
	if err != nil {
		h.logger.Error("listing DLQ", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, deliveries)
}

func (h *AdminHandler) handleRetryDLQ(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := h.store.RetryDelivery(id); err != nil {
		h.logger.Error("retrying DLQ delivery", "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "delivery moved from DLQ to pending"})
}

type StatsHandler struct {
	store  queue.Store
	logger *slog.Logger
}

func NewStatsHandler(store queue.Store, logger *slog.Logger) *StatsHandler {
	return &StatsHandler{store: store, logger: logger}
}

func (h *StatsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stats, err := h.store.Stats()
	if err != nil {
		h.logger.Error("getting stats", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}
