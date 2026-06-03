package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/page-agent/workflow-backend/internal/memory"
	"github.com/page-agent/workflow-backend/internal/registry"
)

func registerSiteTaskGuideRoutes(mux *http.ServeMux, repo registry.Repository) {
	guideRepo, ok := repo.(registry.SiteTaskGuideRepository)
	serviceUnavailable := func(w http.ResponseWriter) {
		writeError(w, http.StatusServiceUnavailable, "site_task_guides_failed", "Site task guide repository is not configured.")
	}

	mux.HandleFunc("GET /api/memory/site-task-guides", func(w http.ResponseWriter, r *http.Request) {
		if repo == nil || !ok {
			serviceUnavailable(w)
			return
		}
		guides, err := guideRepo.ListSiteTaskGuides(r.Context(), registry.SiteTaskGuideListQuery{
			ProjectID: r.URL.Query().Get("projectId"),
			Site:      r.URL.Query().Get("site"),
			Module:    r.URL.Query().Get("module"),
			Status:    registry.Status(r.URL.Query().Get("status")),
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, "site_task_guide_list_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"guides": guides})
	})

	mux.HandleFunc("GET /api/memory/site-task-guides/{id}", func(w http.ResponseWriter, r *http.Request) {
		if repo == nil || !ok {
			serviceUnavailable(w)
			return
		}
		guide, err := guideRepo.GetSiteTaskGuide(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusNotFound, "site_task_guide_not_found", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, guide)
	})

	mux.HandleFunc("POST /api/memory/site-task-guides/{id}/disable", func(w http.ResponseWriter, r *http.Request) {
		if repo == nil || !ok {
			serviceUnavailable(w)
			return
		}
		if err := guideRepo.UpdateSiteTaskGuideStatus(r.Context(), r.PathValue("id"), registry.StatusDisabled); err != nil {
			writeError(w, http.StatusNotFound, "site_task_guide_not_found", err.Error())
			return
		}
		guide, err := guideRepo.GetSiteTaskGuide(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusNotFound, "site_task_guide_not_found", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, guide)
	})

	mux.HandleFunc("POST /api/memory/site-task-guides/{id}/feedback", func(w http.ResponseWriter, r *http.Request) {
		if repo == nil || !ok {
			serviceUnavailable(w)
			return
		}
		var feedback registry.SiteTaskGuideFeedback
		if err := json.NewDecoder(r.Body).Decode(&feedback); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
			return
		}
		if err := memory.NewSiteTaskGuideService(repo).ApplyFeedback(r.Context(), r.PathValue("id"), feedback); err != nil {
			writeError(w, http.StatusBadRequest, "site_task_guide_feedback_failed", err.Error())
			return
		}
		guide, err := guideRepo.GetSiteTaskGuide(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusNotFound, "site_task_guide_not_found", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, guide)
	})
}
