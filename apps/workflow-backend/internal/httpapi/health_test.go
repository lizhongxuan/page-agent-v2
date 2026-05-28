package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type pingerFunc func(context.Context) error

func (fn pingerFunc) PingContext(ctx context.Context) error {
	return fn(ctx)
}

func TestHealthEndpointReturnsOK(t *testing.T) {
	router := NewRouter(Dependencies{})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"status":"ok"`) {
		t.Fatalf("expected health body, got %s", response.Body.String())
	}
}

func TestReadyEndpointChecksDatabaseWhenConfigured(t *testing.T) {
	router := NewRouter(Dependencies{
		DB: pingerFunc(func(context.Context) error {
			return errors.New("database unavailable")
		}),
	})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/ready", nil)

	router.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "database unavailable") {
		t.Fatalf("expected readiness error in body, got %s", response.Body.String())
	}
}

func TestReadyEndpointAllowsMissingDatabaseDuringSkeletonPhase(t *testing.T) {
	router := NewRouter(Dependencies{})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/ready", nil)

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200 without DB dependency, got %d", response.Code)
	}
}
