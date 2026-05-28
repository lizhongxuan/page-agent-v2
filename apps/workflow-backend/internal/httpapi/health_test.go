package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/page-agent/workflow-backend/internal/config"
)

func TestHealthEndpoint(t *testing.T) {
	router := NewRouter(config.Config{})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("health response is not json: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %#v", body)
	}
}

func TestReadyEndpoint(t *testing.T) {
	router := NewRouter(config.Config{})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/ready", nil)

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("ready response is not json: %v", err)
	}
	if body["status"] != "ready" {
		t.Fatalf("expected status ready, got %#v", body)
	}
}
