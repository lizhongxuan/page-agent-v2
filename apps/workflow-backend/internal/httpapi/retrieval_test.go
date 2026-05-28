package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/page-agent/workflow-backend/internal/config"
	"github.com/page-agent/workflow-backend/internal/registry"
	"github.com/page-agent/workflow-backend/internal/selector"
)

func TestWorkflowSearchEndpointReturnsRerankedCandidates(t *testing.T) {
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
	})

	response := performJSON(router, http.MethodPost, "/api/retrieval/workflows/search", map[string]any{
		"projectId":  "default",
		"task":       "在 github.com/microsoft/playwright 的 Issues 里搜索 timeout 报错",
		"currentUrl": "https://github.com/microsoft/playwright",
		"pageObservation": map[string]any{
			"title":       "microsoft/playwright",
			"visibleText": []string{"Code", "Issues", "Pull requests"},
			"controls":    []map[string]string{{"role": "link", "name": "Issues"}},
		},
		"riskPolicy": map[string]any{
			"autoAllowed": []string{"read_only", "read_or_search"},
			"blocked":     []string{"destructive"},
		},
		"limit": 8,
	})

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		CurrentPageState string `json:"currentPageState"`
		Candidates       []struct {
			WorkflowID string   `json:"workflowId"`
			Version    int      `json:"version"`
			Reasons    []string `json:"reasons"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if len(body.Candidates) != 1 || body.Candidates[0].WorkflowID != recipe.ID {
		t.Fatalf("expected workflow candidate, got %#v", body)
	}
}

func TestWorkflowSelectEndpointReturnsBindings(t *testing.T) {
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
		Selector: selector.NewService(selector.HeuristicSelector{}),
	})

	response := performJSON(router, http.MethodPost, "/api/retrieval/workflows/select", map[string]any{
		"projectId":        "default",
		"task":             "在 github.com/microsoft/playwright 的 Issues 里搜索 timeout 报错",
		"currentPageState": "github_repo_home",
		"candidateSlots": map[string]string{
			"repo":  "microsoft/playwright",
			"query": "timeout 报错",
		},
		"candidates": []map[string]any{
			{
				"workflowId": recipe.ID,
				"version":    recipe.Version,
				"finalScore": 0.91,
				"riskLevel":  "read_or_search",
				"variables":  []string{"repo", "query"},
			},
		},
	})

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		Decision           string `json:"decision"`
		SelectedWorkflowID string `json:"selectedWorkflowId"`
		Bindings           map[string]struct {
			Value string `json:"value"`
		} `json:"bindings"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body.Decision != "replay" || body.SelectedWorkflowID != recipe.ID {
		t.Fatalf("unexpected selection: %#v", body)
	}
	if body.Bindings["repo"].Value != "microsoft/playwright" || body.Bindings["query"].Value != "timeout 报错" {
		t.Fatalf("unexpected bindings: %#v", body.Bindings)
	}
}

func TestWorkflowSearchEndpointRejectsInvalidJSON(t *testing.T) {
	router := NewRouterWithServices(config.Config{}, Services{})
	response := performJSON(router, http.MethodPost, "/api/retrieval/workflows/search", "{")

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}
}

func TestInterruptAndRepairSearchEndpointsReturnRegistryMatches(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	if err := repo.SaveInterruptHandler(t.Context(), registry.InterruptHandler{
		ID:                  "ih_dialog",
		ProjectID:           "default",
		Site:                "github.com",
		Status:              registry.StatusActive,
		AppliesToPageStates: []string{"github_issues_list"},
		InterruptType:       "modal",
		FingerprintText:     "New feature",
		RequiredText:        []string{"New feature"},
		TargetControls:      []registry.ControlSignature{{Role: "button", Name: "Got it"}},
		RiskLevel:           registry.RiskReadOnly,
		WorkflowID:          "wf_close_dialog",
		Version:             1,
	}); err != nil {
		t.Fatalf("SaveInterruptHandler failed: %v", err)
	}
	if err := repo.SaveRepairPatch(t.Context(), registry.RepairPatch{
		ID:                  "patch_search",
		ProjectID:           "default",
		Status:              registry.StatusActive,
		WorkflowID:          "wf_github_issue_search",
		WorkflowVersion:     3,
		ChunkID:             "search_issues",
		StepID:              "fill_query",
		Site:                "github.com",
		FailureType:         "locator_not_found",
		FailureSignature:    "search input missing",
		NewTargetSummary:    "Search textbox",
		AppliesToPageStates: []string{"github_issues_list"},
		RiskLevel:           registry.RiskReadOnly,
	}); err != nil {
		t.Fatalf("SaveRepairPatch failed: %v", err)
	}
	router := NewRouterWithServices(config.Config{}, Services{Registry: repo})

	interruptResponse := performJSON(router, http.MethodPost, "/api/retrieval/interrupts/search", map[string]any{
		"projectId":        "default",
		"workflowId":       "wf_github_issue_search",
		"version":          3,
		"currentPageState": "github_issues_list",
		"pageObservation": map[string]any{
			"visibleText": []string{"New feature"},
			"controls":    []map[string]string{{"role": "button", "name": "Got it"}},
		},
		"riskPolicy": map[string]any{
			"autoAllowed": []string{"read_only", "read_or_search"},
			"blocked":     []string{"destructive"},
		},
	})
	repairResponse := performJSON(router, http.MethodPost, "/api/retrieval/repairs/search", map[string]any{
		"projectId":        "default",
		"workflowId":       "wf_github_issue_search",
		"version":          3,
		"chunkId":          "search_issues",
		"stepId":           "fill_query",
		"failureType":      "locator_not_found",
		"currentPageState": "github_issues_list",
		"riskPolicy": map[string]any{
			"autoAllowed": []string{"read_only", "read_or_search"},
			"blocked":     []string{"destructive"},
		},
	})

	if interruptResponse.Code != http.StatusOK {
		t.Fatalf("expected interrupt 200, got %d: %s", interruptResponse.Code, interruptResponse.Body.String())
	}
	if repairResponse.Code != http.StatusOK {
		t.Fatalf("expected repair 200, got %d: %s", repairResponse.Code, repairResponse.Body.String())
	}
	var interrupts struct {
		Handlers []struct {
			HandlerID string `json:"handlerId"`
		} `json:"handlers"`
	}
	if err := json.Unmarshal(interruptResponse.Body.Bytes(), &interrupts); err != nil {
		t.Fatalf("decode interrupt response failed: %v", err)
	}
	var repairs struct {
		Patches []struct {
			PatchID string `json:"patchId"`
		} `json:"patches"`
	}
	if err := json.Unmarshal(repairResponse.Body.Bytes(), &repairs); err != nil {
		t.Fatalf("decode repair response failed: %v", err)
	}
	if len(interrupts.Handlers) != 1 || interrupts.Handlers[0].HandlerID != "ih_dialog" {
		t.Fatalf("unexpected interrupt response: %#v", interrupts)
	}
	if len(repairs.Patches) != 1 || repairs.Patches[0].PatchID != "patch_search" {
		t.Fatalf("unexpected repair response: %#v", repairs)
	}
}

func TestWorkflowIndexRebuildEndpointRunsIndexer(t *testing.T) {
	indexer := &fakeHTTPIndexer{count: 2}
	router := NewRouterWithServices(config.Config{}, Services{Indexer: indexer})

	response := performJSON(router, http.MethodPost, "/api/retrieval/index/rebuild", map[string]string{
		"projectId": "default",
	})

	if response.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", response.Code, response.Body.String())
	}
	if indexer.rebuildProjectID != "default" {
		t.Fatalf("expected project id forwarded, got %q", indexer.rebuildProjectID)
	}
}

type fakeHTTPIndexer struct {
	indexedWorkflowID string
	rebuildProjectID  string
	count             int
}

func (indexer *fakeHTTPIndexer) IndexWorkflow(_ context.Context, workflowID string) error {
	indexer.indexedWorkflowID = workflowID
	return nil
}

func (indexer *fakeHTTPIndexer) Rebuild(_ context.Context, projectID string) (int, error) {
	indexer.rebuildProjectID = projectID
	return indexer.count, nil
}
