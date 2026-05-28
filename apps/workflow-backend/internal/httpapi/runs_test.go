package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/page-agent/workflow-backend/internal/config"
	"github.com/page-agent/workflow-backend/internal/registry"
	"github.com/page-agent/workflow-backend/internal/replay"
)

func TestStartRunEndpoint(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	recipe := sampleHTTPWorkflowRecipe()
	recipe.Status = registry.StatusActive
	recipe.Searchable = true
	if err := repo.SaveWorkflow(t.Context(), recipe); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}
	router := NewRouterWithServices(config.Config{}, Services{
		Runs: replay.NewService(repo, replay.NoopPlaywrightRunner{}),
	})

	response := performJSON(router, http.MethodPost, "/api/runs/start", map[string]any{
		"projectId":          "default",
		"selectedWorkflowId": recipe.ID,
		"version":            recipe.Version,
		"bindings":           map[string]string{"repo": "microsoft/playwright", "query": "timeout"},
		"currentUrl":         "https://github.com/microsoft/playwright",
	})

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		RunID      string `json:"runId"`
		Status     string `json:"status"`
		WorkflowID string `json:"workflowId"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body.RunID == "" || body.Status != "succeeded" || body.WorkflowID != recipe.ID {
		t.Fatalf("unexpected run response: %#v", body)
	}
}

func TestGetRunEndpoint(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	recipe := sampleHTTPWorkflowRecipe()
	recipe.Status = registry.StatusActive
	recipe.Searchable = true
	if err := repo.SaveWorkflow(t.Context(), recipe); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}
	router := NewRouterWithServices(config.Config{}, Services{
		Registry: repo,
		Runs:     replay.NewService(repo, replay.NoopPlaywrightRunner{}),
	})

	startResponse := performJSON(router, http.MethodPost, "/api/runs/start", map[string]any{
		"projectId":          "default",
		"selectedWorkflowId": recipe.ID,
		"version":            recipe.Version,
		"bindings":           map[string]string{"query": "sk-secret"},
		"currentUrl":         "https://github.com/microsoft/playwright",
	})
	if startResponse.Code != http.StatusOK {
		t.Fatalf("expected start 200, got %d: %s", startResponse.Code, startResponse.Body.String())
	}
	var started struct {
		RunID string `json:"runId"`
	}
	if err := json.Unmarshal(startResponse.Body.Bytes(), &started); err != nil {
		t.Fatalf("decode start response failed: %v", err)
	}

	getResponse := performJSON(router, http.MethodGet, "/api/runs/"+started.RunID, nil)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("expected get 200, got %d: %s", getResponse.Code, getResponse.Body.String())
	}
	var body registry.WorkflowRun
	if err := json.Unmarshal(getResponse.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode get response failed: %v", err)
	}
	if body.ID != started.RunID || body.Variables["query"] != "[redacted]" {
		t.Fatalf("unexpected run body: %#v", body)
	}
}
