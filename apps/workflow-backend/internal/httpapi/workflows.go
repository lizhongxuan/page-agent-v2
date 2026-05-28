package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/page-agent/workflow-backend/internal/workflow"
)

func registerWorkflowRoutes(mux *http.ServeMux, service *workflow.Service) {
	if service == nil {
		return
	}

	mux.HandleFunc("POST /api/workflows", func(w http.ResponseWriter, r *http.Request) {
		var recipe workflow.WorkflowRecipe
		if err := json.NewDecoder(r.Body).Decode(&recipe); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		created, err := service.Create(r.Context(), recipe)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, created)
	})

	mux.HandleFunc("GET /api/workflows/{id}", func(w http.ResponseWriter, r *http.Request) {
		recipe, err := service.Get(r.Context(), r.PathValue("id"))
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, recipe)
	})

	mux.HandleFunc("POST /api/workflows/{id}/versions", func(w http.ResponseWriter, r *http.Request) {
		var recipe workflow.WorkflowRecipe
		if err := json.NewDecoder(r.Body).Decode(&recipe); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		updated, err := service.AddVersion(r.Context(), r.PathValue("id"), recipe)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, updated)
	})

	mux.HandleFunc("POST /api/workflows/{id}/promote", func(w http.ResponseWriter, r *http.Request) {
		if err := service.Promote(r.Context(), r.PathValue("id")); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "active"})
	})

	mux.HandleFunc("POST /api/workflows/{id}/disable", func(w http.ResponseWriter, r *http.Request) {
		if err := service.Disable(r.Context(), r.PathValue("id")); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
	})

	mux.HandleFunc("POST /api/workflows/search", func(w http.ResponseWriter, r *http.Request) {
		var request workflow.SearchRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		results, err := service.Search(r.Context(), request)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"workflows": results})
	})
}
