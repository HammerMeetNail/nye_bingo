package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type HealthChecker interface {
	Health(ctx context.Context) error
}

type HealthHandler struct {
	db    HealthChecker
	redis HealthChecker
}

func NewHealthHandler(db, redis HealthChecker) *HealthHandler {
	return &HealthHandler{
		db:    db,
		redis: redis,
	}
}

type HealthResponse struct {
	Status    string            `json:"status"`
	Checks    map[string]string `json:"checks"`
	Timestamp string            `json:"timestamp"`
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	response := HealthResponse{
		Status:    "healthy",
		Checks:    make(map[string]string),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	// Check PostgreSQL
	if err := h.db.Health(ctx); err != nil {
		response.Status = "unhealthy"
		response.Checks["postgres"] = "unhealthy: " + err.Error()
	} else {
		response.Checks["postgres"] = "healthy"
	}

	// Check Redis
	if err := h.redis.Health(ctx); err != nil {
		response.Status = "unhealthy"
		response.Checks["redis"] = "unhealthy: " + err.Error()
	} else {
		response.Checks["redis"] = "healthy"
	}

	w.Header().Set("Content-Type", "application/json")

	if response.Status == "unhealthy" {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	_ = json.NewEncoder(w).Encode(response)
}

func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Check both dependencies
	dbErr := h.db.Health(ctx)
	redisErr := h.redis.Health(ctx)

	if dbErr != nil || redisErr != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("not ready"))
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}

func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("alive"))
}
