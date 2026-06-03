package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/page-agent/workflow-backend/internal/manualwiki"
	"github.com/page-agent/workflow-backend/internal/registry"
)

func registerSiteManualRoutes(mux *http.ServeMux, repo registry.Repository) {
	manualRepo, ok := repo.(registry.SiteManualRepository)
	serviceUnavailable := func(w http.ResponseWriter) {
		writeError(w, http.StatusServiceUnavailable, "site_manuals_failed", "Site manual repository is not configured.")
	}

	mux.HandleFunc("POST /api/memory/site-manuals/import", func(w http.ResponseWriter, r *http.Request) {
		if repo == nil || !ok {
			serviceUnavailable(w)
			return
		}
		var request manualwiki.ImportManualRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
			return
		}
		response, err := manualwiki.NewService(manualRepo).ImportManual(r.Context(), request)
		if err != nil {
			writeError(w, http.StatusBadRequest, "site_manual_import_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, response)
	})

	mux.HandleFunc("GET /api/memory/site-manuals", func(w http.ResponseWriter, r *http.Request) {
		if repo == nil || !ok {
			serviceUnavailable(w)
			return
		}
		sources, err := manualRepo.ListSiteManualSources(r.Context(), registry.SiteManualSourceListQuery{
			ProjectID:     r.URL.Query().Get("projectId"),
			Site:          r.URL.Query().Get("site"),
			Module:        r.URL.Query().Get("module"),
			IncludeHidden: r.URL.Query().Get("includeHidden") == "true",
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, "site_manual_list_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"sources": sources})
	})

	mux.HandleFunc("GET /api/memory/site-manuals/{id}", func(w http.ResponseWriter, r *http.Request) {
		if repo == nil || !ok {
			serviceUnavailable(w)
			return
		}
		source, err := manualRepo.GetSiteManualSource(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusNotFound, "site_manual_not_found", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, source)
	})

	mux.HandleFunc("GET /api/memory/site-manuals/{id}/wiki", func(w http.ResponseWriter, r *http.Request) {
		if repo == nil || !ok {
			serviceUnavailable(w)
			return
		}
		wiki, err := manualRepo.GetSiteManualWikiForSource(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusNotFound, "site_manual_wiki_not_found", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, wiki)
	})

	mux.HandleFunc("POST /api/memory/site-manuals/{id}/rebuild", func(w http.ResponseWriter, r *http.Request) {
		if repo == nil || !ok {
			serviceUnavailable(w)
			return
		}
		response, err := manualwiki.NewService(manualRepo).Rebuild(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "site_manual_rebuild_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, response)
	})

	mux.HandleFunc("POST /api/memory/site-manuals/{id}/disable", func(w http.ResponseWriter, r *http.Request) {
		updateSiteManualStatus(w, r, manualRepo, ok, registry.StatusDisabled)
	})
	mux.HandleFunc("POST /api/memory/site-manuals/{id}/enable", func(w http.ResponseWriter, r *http.Request) {
		updateSiteManualStatus(w, r, manualRepo, ok, registry.StatusActive)
	})
	mux.HandleFunc("DELETE /api/memory/site-manuals/{id}", func(w http.ResponseWriter, r *http.Request) {
		if repo == nil || !ok {
			serviceUnavailable(w)
			return
		}
		if err := manualRepo.DeleteSiteManualSource(r.Context(), r.PathValue("id")); err != nil {
			writeError(w, http.StatusNotFound, "site_manual_not_found", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/memory/site-manuals/preview-context", func(w http.ResponseWriter, r *http.Request) {
		if repo == nil || !ok {
			serviceUnavailable(w)
			return
		}
		var request manualwiki.PreviewContextRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
			return
		}
		preview, err := manualwiki.NewService(manualRepo).PreviewContext(r.Context(), request)
		if err != nil {
			writeError(w, http.StatusBadRequest, "site_manual_preview_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, preview)
	})
}

func updateSiteManualStatus(w http.ResponseWriter, r *http.Request, repo registry.SiteManualRepository, ok bool, status registry.Status) {
	if repo == nil || !ok {
		writeError(w, http.StatusServiceUnavailable, "site_manuals_failed", "Site manual repository is not configured.")
		return
	}
	if err := repo.UpdateSiteManualSourceStatus(r.Context(), r.PathValue("id"), status); err != nil {
		writeError(w, http.StatusNotFound, "site_manual_not_found", err.Error())
		return
	}
	source, err := repo.GetSiteManualSource(r.Context(), r.PathValue("id"))
	if err != nil && status != registry.StatusDisabled {
		writeError(w, http.StatusNotFound, "site_manual_not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, source)
}
