package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/page-agent/workflow-backend/internal/recipe"
)

func registerCandidateRoutes(mux *http.ServeMux, service *recipe.CandidateService) {
	if service == nil {
		return
	}

	mux.HandleFunc("POST /api/workflow-candidates/from-session", func(w http.ResponseWriter, r *http.Request) {
		var session recipe.RecordedSession
		if err := json.NewDecoder(r.Body).Decode(&session); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		candidate, err := service.CreateFromSession(r.Context(), session)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, candidate)
	})

	mux.HandleFunc("GET /api/workflow-candidates", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		candidates, err := service.List(r.Context(), recipe.CandidateListQuery{
			ProjectID:    query.Get("projectId"),
			Source:       recipe.CandidateSource(query.Get("source")),
			ReviewStatus: recipe.ReviewStatus(query.Get("reviewStatus")),
		})
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string][]recipe.WorkflowCandidate{"candidates": candidates})
	})

	mux.HandleFunc("GET /api/workflow-candidates/{id}", func(w http.ResponseWriter, r *http.Request) {
		candidate, err := service.Get(r.Context(), r.PathValue("id"))
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, candidate)
	})

	mux.HandleFunc("POST /api/workflow-candidates/{id}/approve", func(w http.ResponseWriter, r *http.Request) {
		candidate, err := service.Approve(r.Context(), r.PathValue("id"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, candidate)
	})

	mux.HandleFunc("POST /api/workflow-candidates/{id}/reject", func(w http.ResponseWriter, r *http.Request) {
		candidate, err := service.Reject(r.Context(), r.PathValue("id"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, candidate)
	})
}
