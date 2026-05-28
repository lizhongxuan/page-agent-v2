package auth

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
)

type Config struct {
	APIKey                    string
	AllowLocalUnauthenticated bool
}

type Middleware struct {
	config Config
}

type contextKey string

const actorContextKey contextKey = "actor"

func NewMiddleware(config Config) *Middleware {
	return &Middleware{config: config}
}

func (middleware *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if middleware.config.AllowLocalUnauthenticated {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), actorContextKey, "local")))
			return
		}

		token := bearerToken(r.Header.Get("Authorization"))
		if middleware.config.APIKey == "" || !constantTimeEqual(token, middleware.config.APIKey) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), actorContextKey, "api_key")))
	})
}

func ActorFromContext(ctx context.Context) string {
	actor, _ := ctx.Value(actorContextKey).(string)
	return actor
}

func ProjectFromRequest(r *http.Request, defaultProject string) string {
	if projectID := r.URL.Query().Get("projectId"); projectID != "" {
		return projectID
	}
	if projectID := r.Header.Get("X-Page-Agent-Project"); projectID != "" {
		return projectID
	}
	return defaultProject
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}

func constantTimeEqual(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}
