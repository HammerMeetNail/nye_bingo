package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Mock health checker for testing
type mockHealthChecker struct {
	healthy bool
	err     error
}

func (m *mockHealthChecker) Health(ctx context.Context) error {
	if !m.healthy {
		return m.err
	}
	return nil
}

func TestHealthHandler_Health_AllHealthy(t *testing.T) {
	db := &mockHealthChecker{healthy: true}
	redis := &mockHealthChecker{healthy: true}
	handler := NewHealthHandler(db, redis)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.Health(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response HealthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Status != "healthy" {
		t.Errorf("expected status 'healthy', got %q", response.Status)
	}
	if response.Checks["postgres"] != "healthy" {
		t.Errorf("expected postgres 'healthy', got %q", response.Checks["postgres"])
	}
	if response.Checks["redis"] != "healthy" {
		t.Errorf("expected redis 'healthy', got %q", response.Checks["redis"])
	}
}

func TestHealthHandler_Health_DBUnhealthy(t *testing.T) {
	db := &mockHealthChecker{healthy: false, err: errors.New("connection refused")}
	redis := &mockHealthChecker{healthy: true}
	handler := NewHealthHandler(db, redis)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.Health(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}

	var response HealthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Status != "unhealthy" {
		t.Errorf("expected status 'unhealthy', got %q", response.Status)
	}
}

func TestHealthHandler_Health_RedisUnhealthy(t *testing.T) {
	db := &mockHealthChecker{healthy: true}
	redis := &mockHealthChecker{healthy: false, err: errors.New("connection timeout")}
	handler := NewHealthHandler(db, redis)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.Health(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}

	var response HealthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Status != "unhealthy" {
		t.Errorf("expected status 'unhealthy', got %q", response.Status)
	}
	if response.Checks["redis"] != "unhealthy" {
		t.Errorf("expected redis check 'unhealthy', got %q", response.Checks["redis"])
	}
	// The raw dependency error must not leak to anonymous callers.
	if strings.Contains(rr.Body.String(), "connection timeout") {
		t.Errorf("health response leaked internal error detail: %s", rr.Body.String())
	}
}

func TestHealthHandler_Health_BothUnhealthy(t *testing.T) {
	db := &mockHealthChecker{healthy: false, err: errors.New("db error")}
	redis := &mockHealthChecker{healthy: false, err: errors.New("redis error")}
	handler := NewHealthHandler(db, redis)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.Health(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}
}

func TestHealthHandler_Health_ContentType(t *testing.T) {
	db := &mockHealthChecker{healthy: true}
	redis := &mockHealthChecker{healthy: true}
	handler := NewHealthHandler(db, redis)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.Health(rr, req)

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type: application/json, got %q", contentType)
	}
}

func TestHealthHandler_Health_HasTimestamp(t *testing.T) {
	db := &mockHealthChecker{healthy: true}
	redis := &mockHealthChecker{healthy: true}
	handler := NewHealthHandler(db, redis)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.Health(rr, req)

	var response HealthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Timestamp == "" {
		t.Error("expected timestamp to be set")
	}
}

func TestHealthHandler_Ready_AllHealthy(t *testing.T) {
	db := &mockHealthChecker{healthy: true}
	redis := &mockHealthChecker{healthy: true}
	handler := NewHealthHandler(db, redis)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rr := httptest.NewRecorder()

	handler.Ready(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	if rr.Body.String() != "ready" {
		t.Errorf("expected body 'ready', got %q", rr.Body.String())
	}
}

func TestHealthHandler_Ready_DBUnhealthy(t *testing.T) {
	db := &mockHealthChecker{healthy: false, err: errors.New("error")}
	redis := &mockHealthChecker{healthy: true}
	handler := NewHealthHandler(db, redis)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rr := httptest.NewRecorder()

	handler.Ready(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}
	if rr.Body.String() != "not ready" {
		t.Errorf("expected body 'not ready', got %q", rr.Body.String())
	}
}

func TestHealthHandler_Ready_RedisUnhealthy(t *testing.T) {
	db := &mockHealthChecker{healthy: true}
	redis := &mockHealthChecker{healthy: false, err: errors.New("error")}
	handler := NewHealthHandler(db, redis)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rr := httptest.NewRecorder()

	handler.Ready(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}
}

func TestHealthHandler_Live(t *testing.T) {
	// Live check doesn't depend on services
	handler := NewHealthHandler(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/live", nil)
	rr := httptest.NewRecorder()

	handler.Live(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	if rr.Body.String() != "alive" {
		t.Errorf("expected body 'alive', got %q", rr.Body.String())
	}
}
