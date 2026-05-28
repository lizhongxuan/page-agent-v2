package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddlewareRejectsMissingAndWrongAPIKey(t *testing.T) {
	middleware := NewMiddleware(Config{APIKey: "secret"})
	handler := middleware.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, tc := range []struct {
		name  string
		token string
	}{
		{name: "missing"},
		{name: "wrong", token: "wrong"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			res := httptest.NewRecorder()

			handler.ServeHTTP(res, req)

			if res.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", res.Code)
			}
		})
	}
}

func TestMiddlewareAcceptsCorrectAPIKeyAndAddsActor(t *testing.T) {
	middleware := NewMiddleware(Config{APIKey: "secret"})
	handler := middleware.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ActorFromContext(r.Context()) != "api_key" {
			t.Fatalf("expected api_key actor, got %q", ActorFromContext(r.Context()))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer secret")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", res.Code)
	}
}

func TestMiddlewareAllowsExplicitLocalMode(t *testing.T) {
	middleware := NewMiddleware(Config{AllowLocalUnauthenticated: true})
	handler := middleware.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", res.Code)
	}
}

func TestProjectFromRequestFallsBackToDefault(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?projectId=query_project", nil)
	if got := ProjectFromRequest(req, "default"); got != "query_project" {
		t.Fatalf("expected query project, got %q", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	if got := ProjectFromRequest(req, "default"); got != "default" {
		t.Fatalf("expected default project, got %q", got)
	}
}
