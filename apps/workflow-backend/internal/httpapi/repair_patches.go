package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/page-agent/workflow-backend/internal/registry"
)

func registerRepairPatchRoutes(mux *http.ServeMux, service *registry.RepairPatchService) {
	mux.HandleFunc("POST /api/repair-patches/candidates", func(w http.ResponseWriter, r *http.Request) {
		var request createRepairPatchRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
			return
		}
		patch, err := service.CreateCandidate(r.Context(), request.Patch)
		if err != nil {
			writeError(w, http.StatusBadRequest, "repair_patch_invalid", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, patch)
	})
	mux.HandleFunc("GET /api/repair-patches", func(w http.ResponseWriter, r *http.Request) {
		patches, err := service.List(r.Context(), registry.RepairPatchListQuery{
			ProjectID:  r.URL.Query().Get("projectId"),
			WorkflowID: r.URL.Query().Get("workflowId"),
			Status:     registry.Status(r.URL.Query().Get("status")),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "repair_patch_list_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string][]registry.RepairPatch{"patches": patches})
	})
	mux.HandleFunc("POST /api/repair-patches/{id}/approve", func(w http.ResponseWriter, r *http.Request) {
		patch, err := service.Approve(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "repair_patch_approve_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, patch)
	})
	mux.HandleFunc("POST /api/repair-patches/{id}/reject", func(w http.ResponseWriter, r *http.Request) {
		patch, err := service.Reject(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "repair_patch_reject_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, patch)
	})
}

type createRepairPatchRequest struct {
	Patch registry.RepairPatch `json:"patch"`
}
