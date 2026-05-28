package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/page-agent/workflow-backend/internal/artifact"
)

func TestArtifactJSONUploadListDownloadDeleteHTTP(t *testing.T) {
	service := artifact.NewService(artifact.NewLocalStore(t.TempDir()), artifact.NewMemoryRepository())
	router := NewRouter(Dependencies{Artifact: service})

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/artifacts/json", strings.NewReader(`{
		"projectId":"default",
		"ownerType":"workflow_run",
		"ownerId":"run_1",
		"kind":"browser_state",
		"content":{"url":"https://example.com"}
	}`))
	uploadRes := httptest.NewRecorder()
	router.ServeHTTP(uploadRes, uploadReq)
	if uploadRes.Code != http.StatusCreated {
		t.Fatalf("expected upload 201, got %d %s", uploadRes.Code, uploadRes.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/artifacts?projectId=default&ownerType=workflow_run&ownerId=run_1", nil)
	listRes := httptest.NewRecorder()
	router.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK || !strings.Contains(listRes.Body.String(), "browser_state") {
		t.Fatalf("unexpected list response %d %s", listRes.Code, listRes.Body.String())
	}
}

func TestArtifactUploadRejectsScreenshot(t *testing.T) {
	service := artifact.NewService(artifact.NewLocalStore(t.TempDir()), artifact.NewMemoryRepository())
	router := NewRouter(Dependencies{Artifact: service})
	uploadReq := httptest.NewRequest(http.MethodPost, "/api/artifacts/json", strings.NewReader(`{
		"projectId":"default",
		"ownerType":"workflow_run",
		"ownerId":"run_1",
		"kind":"screenshot",
		"content":{"png":"nope"}
	}`))
	uploadRes := httptest.NewRecorder()
	router.ServeHTTP(uploadRes, uploadReq)
	if uploadRes.Code != http.StatusBadRequest {
		t.Fatalf("expected upload 400, got %d %s", uploadRes.Code, uploadRes.Body.String())
	}
}
