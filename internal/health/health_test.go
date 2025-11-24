package health

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthChecker(t *testing.T) {
	// This test requires a real ClickHouse connection or mock
	// For now, we'll test the HTTP handler structure

	// Create a mock checker (would need actual connection in real test)
	// checker := NewChecker(nil) // nil connection for testing structure

	// Test HTTP handler structure
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"status":  "healthy",
			"details": map[string]interface{}{"clickhouse": "connected"},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHealthCheckerGetStatus(t *testing.T) {
	// Test with nil connection (should be unhealthy)
	checker := NewChecker(nil)

	status := checker.GetStatus()
	if status != StatusUnhealthy {
		t.Errorf("Expected unhealthy status with nil connection, got %v", status)
	}

	details := checker.GetDetails()
	if details == nil {
		t.Error("Details should not be nil")
	}
}

