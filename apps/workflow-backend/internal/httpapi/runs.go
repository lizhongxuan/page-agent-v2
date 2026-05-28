package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/page-agent/workflow-backend/internal/registry"
	"github.com/page-agent/workflow-backend/internal/replay"
)

func registerRunRoutes(mux *http.ServeMux, service *replay.Service, repo registry.Repository) {
	mux.HandleFunc("POST /api/runs/start", func(w http.ResponseWriter, r *http.Request) {
		var request startRunRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_response", "Request body must be valid JSON.")
			return
		}
		run, err := service.Start(r.Context(), replay.StartRequest{
			ProjectID:  request.ProjectID,
			WorkflowID: request.SelectedWorkflowID,
			Version:    request.Version,
			Task:       request.Task,
			URL:        request.CurrentURL,
			Bindings:   request.Bindings,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, "replay_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"runId":          run.ID,
			"status":         run.Result,
			"workflowId":     run.WorkflowID,
			"version":        run.Version,
			"failedChunkId":  run.FailedChunkID,
			"failedStepId":   run.FailedStepID,
			"fallbackReason": run.FallbackReason,
		})
	})
	mux.HandleFunc("GET /api/runs/{id}", func(w http.ResponseWriter, r *http.Request) {
		if repo == nil {
			writeError(w, http.StatusServiceUnavailable, "replay_failed", "Workflow registry is not configured.")
			return
		}
		run, err := repo.GetRun(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusNotFound, "run_not_found", "Workflow run not found.")
			return
		}
		writeJSON(w, http.StatusOK, run)
	})
}

type startRunRequest struct {
	ProjectID          string            `json:"projectId"`
	SelectedWorkflowID string            `json:"selectedWorkflowId"`
	Version            int               `json:"version"`
	Task               string            `json:"task"`
	CurrentURL         string            `json:"currentUrl"`
	Bindings           map[string]string `json:"bindings"`
}
