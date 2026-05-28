package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/page-agent/workflow-backend/internal/selector"
)

func registerRunRoutes(mux *http.ServeMux, service *selector.Service) {
	if service == nil {
		return
	}

	mux.HandleFunc("POST /api/workflow-runs", func(w http.ResponseWriter, r *http.Request) {
		var run selector.WorkflowRun
		if err := json.NewDecoder(r.Body).Decode(&run); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		created, err := service.RecordRun(r.Context(), run)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, created)
	})

	mux.HandleFunc("POST /api/workflow-runs/{id}/selector-stats", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Updates []selector.SelectorStatUpdate `json:"updates"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := service.RecordSelectorStats(r.Context(), r.PathValue("id"), payload.Updates); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}
