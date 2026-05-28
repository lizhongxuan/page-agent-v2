package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/page-agent/workflow-backend/internal/config"
)

func registerHealthRoutes(mux *http.ServeMux, _ config.Config) {
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, map[string]any{
		"code":      code,
		"message":   message,
		"retryable": status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= 500,
	})
}
