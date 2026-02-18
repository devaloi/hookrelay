package handler

import (
	"log/slog"
	"net/http"

	"github.com/devaloi/hookrelay/internal/queue"
)

// HealthHandler handles health check requests.
type HealthHandler struct {
	store  queue.Store
	logger *slog.Logger
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(store queue.Store, logger *slog.Logger) *HealthHandler {
	return &HealthHandler{
		store:  store,
		logger: logger,
	}
}

// ServeHTTP handles GET /health requests.
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	// Check database connectivity by getting stats
	stats, err := h.store.Stats()
	if err != nil {
		h.logger.Error("health check failed", "error", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "healthy",
		"pending": stats.Pending,
	})
}
