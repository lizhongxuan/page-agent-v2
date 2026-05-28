package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/page-agent/workflow-backend/internal/artifact"
)

func registerArtifactRoutes(mux *http.ServeMux, service *artifact.Service) {
	if service == nil {
		return
	}

	mux.HandleFunc("POST /api/artifacts/json", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			ProjectID string                `json:"projectId"`
			OwnerType artifact.OwnerType    `json:"ownerType"`
			OwnerID   string                `json:"ownerId"`
			Kind      artifact.ArtifactKind `json:"kind"`
			Content   any                   `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		content, err := json.Marshal(payload.Content)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		stored, err := service.Store(r.Context(), artifact.PutRequest{
			ProjectID:   payload.ProjectID,
			OwnerType:   payload.OwnerType,
			OwnerID:     payload.OwnerID,
			Kind:        payload.Kind,
			ContentType: "application/json",
			Reader:      bytes.NewReader(content),
		})
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, stored)
	})

	mux.HandleFunc("GET /api/artifacts", func(w http.ResponseWriter, r *http.Request) {
		items, err := service.List(r.Context(), artifact.ListQuery{
			ProjectID: r.URL.Query().Get("projectId"),
			OwnerType: artifact.OwnerType(r.URL.Query().Get("ownerType")),
			OwnerID:   r.URL.Query().Get("ownerId"),
		})
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"artifacts": items})
	})

	mux.HandleFunc("DELETE /api/artifacts/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := service.Delete(r.Context(), r.PathValue("id")); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	})
}
