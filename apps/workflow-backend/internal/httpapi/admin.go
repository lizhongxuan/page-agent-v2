package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/page-agent/workflow-backend/internal/config"
)

func registerAdminRoutes(mux *http.ServeMux, cfg config.Config, indexer WorkflowIndexer) {
	mux.HandleFunc("POST /api/admin/index/rebuild", func(w http.ResponseWriter, r *http.Request) {
		if cfg.DisableQdrant {
			writeError(w, http.StatusServiceUnavailable, "retrieval_unavailable", "Workflow indexing is unavailable because Qdrant indexing is disabled.")
			return
		}
		if indexer == nil {
			writeError(w, http.StatusServiceUnavailable, "retrieval_unavailable", "Workflow indexer is not configured; enable Qdrant indexing before rebuilding the index.")
			return
		}
		var request struct {
			ProjectID string `json:"projectId"`
		}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&request)
		}
		count, err := indexer.Rebuild(r.Context(), request.ProjectID)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "retrieval_unavailable", err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"status": "accepted", "count": count})
	})
}

func writeDeprecatedEndpoint(w http.ResponseWriter, migration string) {
	writeError(w, http.StatusGone, "endpoint_deprecated", "This endpoint has been removed. Use "+migration+" instead.")
}
