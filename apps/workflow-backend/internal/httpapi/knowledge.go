package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/page-agent/workflow-backend/internal/knowledge"
)

func registerKnowledgeRoutes(mux *http.ServeMux, service *knowledge.Service) {
	if service == nil {
		return
	}

	mux.HandleFunc("POST /search", func(w http.ResponseWriter, r *http.Request) {
		var request knowledge.SearchRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		hits, err := service.Search(r.Context(), request)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]any{"hits": []knowledge.Hit{}})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"hits": hits})
	})

	mux.HandleFunc("POST /api/knowledge/documents", func(w http.ResponseWriter, r *http.Request) {
		var doc knowledge.Document
		if err := json.NewDecoder(r.Body).Decode(&doc); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		created, err := service.Create(r.Context(), doc)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, created)
	})

	mux.HandleFunc("POST /api/knowledge/ingest", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Documents []knowledge.Document `json:"documents"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		result, err := service.Ingest(r.Context(), payload.Documents)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("GET /api/knowledge/documents/{id}", func(w http.ResponseWriter, r *http.Request) {
		doc, err := service.Get(r.Context(), projectIDFromRequest(r), r.PathValue("id"))
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, doc)
	})
}

func projectIDFromRequest(r *http.Request) string {
	if projectID := r.URL.Query().Get("projectId"); projectID != "" {
		return projectID
	}
	if projectID := r.URL.Query().Get("projectKey"); projectID != "" {
		return projectID
	}
	if projectID := r.Header.Get("X-Page-Agent-Project"); projectID != "" {
		return projectID
	}
	if strings.Contains(r.URL.Path, "/api/") {
		return "default"
	}
	return "default"
}
