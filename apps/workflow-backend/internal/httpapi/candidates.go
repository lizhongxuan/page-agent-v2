package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/page-agent/workflow-backend/internal/registry"
)

func registerCandidateRoutes(mux *http.ServeMux, service *registry.CandidateService, indexer WorkflowIndexer) {
	mux.HandleFunc("POST /api/workflow-candidates/from-session", func(w http.ResponseWriter, r *http.Request) {
		var request createCandidateRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
			return
		}
		candidate, err := service.CreateFromSession(r.Context(), registry.RecordedSession{
			ProjectID: request.ProjectID,
			Source:    request.Source,
			Task:      request.Task,
			StartURL:  request.StartURL,
			Recipe:    request.Recipe,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, "candidate_invalid", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, candidate)
	})
	mux.HandleFunc("GET /api/workflow-candidates", func(w http.ResponseWriter, r *http.Request) {
		candidates, err := service.List(r.Context(), registry.CandidateListQuery{
			ProjectID: r.URL.Query().Get("projectId"),
			Source:    r.URL.Query().Get("source"),
			Status:    registry.Status(r.URL.Query().Get("reviewStatus")),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "candidate_list_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string][]registry.WorkflowCandidate{"candidates": candidates})
	})
	mux.HandleFunc("GET /api/workflow-candidates/{id}", func(w http.ResponseWriter, r *http.Request) {
		candidates, err := service.List(r.Context(), registry.CandidateListQuery{})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "candidate_list_failed", err.Error())
			return
		}
		id := r.PathValue("id")
		for _, candidate := range candidates {
			if candidate.ID == id {
				writeJSON(w, http.StatusOK, candidate)
				return
			}
		}
		writeError(w, http.StatusNotFound, "candidate_not_found", "Workflow candidate not found.")
	})
	mux.HandleFunc("POST /api/workflow-candidates/{id}/approve", func(w http.ResponseWriter, r *http.Request) {
		candidate, err := service.Approve(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "candidate_approve_failed", err.Error())
			return
		}
		if indexer != nil {
			if err := indexer.IndexWorkflow(r.Context(), candidate.RecipeDraft.ID); err != nil {
				writeError(w, http.StatusServiceUnavailable, "qdrant_unavailable", err.Error())
				return
			}
		}
		writeJSON(w, http.StatusOK, candidate)
	})
	mux.HandleFunc("POST /api/workflow-candidates/{id}/reject", func(w http.ResponseWriter, r *http.Request) {
		candidate, err := service.Reject(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "candidate_reject_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, candidate)
	})
}

type createCandidateRequest struct {
	ProjectID string                  `json:"projectId"`
	Source    string                  `json:"source"`
	Task      string                  `json:"task"`
	StartURL  string                  `json:"startUrl"`
	Recipe    registry.WorkflowRecipe `json:"recipe"`
}
