package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/page-agent/workflow-backend/internal/config"
)

func TestAdminIndexRebuildReturnsUnavailableWhenIndexerMissing(t *testing.T) {
	router := NewRouterWithServices(config.Config{}, Services{})

	response := performJSON(router, http.MethodPost, "/api/admin/index/rebuild", map[string]string{"projectId": "default"})

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body.Code != "retrieval_unavailable" {
		t.Fatalf("expected retrieval_unavailable, got %#v", body)
	}
	if body.Message == "" {
		t.Fatal("expected clear unavailable message")
	}
}

func TestAdminIndexRebuildReturnsUnavailableWhenQdrantDisabled(t *testing.T) {
	indexer := &fakeHTTPIndexer{count: 2}
	router := NewRouterWithServices(config.Config{DisableQdrant: true}, Services{Indexer: indexer})

	response := performJSON(router, http.MethodPost, "/api/admin/index/rebuild", map[string]string{"projectId": "default"})

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body.Code != "retrieval_unavailable" {
		t.Fatalf("expected retrieval_unavailable, got %#v", body)
	}
	if indexer.rebuildProjectID != "" {
		t.Fatalf("disabled Qdrant must not call indexer, got project %q", indexer.rebuildProjectID)
	}
}

func TestAdminIndexRebuildRunsIndexer(t *testing.T) {
	indexer := &fakeHTTPIndexer{count: 2}
	router := NewRouterWithServices(config.Config{}, Services{Indexer: indexer})

	response := performJSON(router, http.MethodPost, "/api/admin/index/rebuild", map[string]string{"projectId": "default"})

	if response.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", response.Code, response.Body.String())
	}
	if indexer.rebuildProjectID != "default" {
		t.Fatalf("expected project id forwarded, got %q", indexer.rebuildProjectID)
	}
	var body struct {
		Status string `json:"status"`
		Count  int    `json:"count"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body.Status != "accepted" || body.Count != 2 {
		t.Fatalf("unexpected rebuild response: %#v", body)
	}
}
