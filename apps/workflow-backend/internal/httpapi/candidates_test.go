package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/page-agent/workflow-backend/internal/config"
	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestCandidateRoutesCreateApproveRejectAndList(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	router := NewRouterWithServices(config.Config{}, Services{
		Candidates: registry.NewCandidateService(repo),
	})

	createBody := map[string]any{
		"projectId": "default",
		"source":    "user_demo",
		"task":      "Search GitHub issues",
		"startUrl":  "https://github.com/alibaba/page-agent",
		"recipe":    sampleHTTPWorkflowRecipe(),
	}
	createResponse := performJSON(router, http.MethodPost, "/api/workflow-candidates/from-session", createBody)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("expected create status 201, got %d: %s", createResponse.Code, createResponse.Body.String())
	}
	var candidate registry.WorkflowCandidate
	if err := json.Unmarshal(createResponse.Body.Bytes(), &candidate); err != nil {
		t.Fatalf("decode candidate failed: %v", err)
	}
	if candidate.Status != registry.StatusPendingReview || candidate.Searchable {
		t.Fatalf("candidate should be pending and non-searchable: %#v", candidate)
	}

	listResponse := performJSON(router, http.MethodGet, "/api/workflow-candidates?projectId=default&reviewStatus=pending_review", nil)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected list status 200, got %d", listResponse.Code)
	}
	var listBody struct {
		Candidates []registry.WorkflowCandidate `json:"candidates"`
	}
	if err := json.Unmarshal(listResponse.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode list failed: %v", err)
	}
	if len(listBody.Candidates) != 1 {
		t.Fatalf("expected one pending candidate, got %#v", listBody)
	}

	approveResponse := performJSON(router, http.MethodPost, "/api/workflow-candidates/"+candidate.ID+"/approve", nil)
	if approveResponse.Code != http.StatusOK {
		t.Fatalf("expected approve status 200, got %d: %s", approveResponse.Code, approveResponse.Body.String())
	}
	var approved registry.WorkflowCandidate
	if err := json.Unmarshal(approveResponse.Body.Bytes(), &approved); err != nil {
		t.Fatalf("decode approved candidate failed: %v", err)
	}
	if approved.Status != registry.StatusActive || !approved.Searchable {
		t.Fatalf("approved candidate should be active and searchable: %#v", approved)
	}
}

func TestApproveCandidateIndexesWorkflowWhenIndexerConfigured(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := registry.NewCandidateService(repo)
	candidate, err := service.CreateFromSession(t.Context(), registry.RecordedSession{
		ProjectID: "default",
		Source:    "user_demo",
		Task:      "search github issues",
		StartURL:  "https://github.com/microsoft/playwright",
		Recipe:    sampleHTTPWorkflowRecipe(),
	})
	if err != nil {
		t.Fatalf("CreateFromSession failed: %v", err)
	}
	indexer := &fakeHTTPIndexer{}
	router := NewRouterWithServices(config.Config{}, Services{
		Candidates: service,
		Indexer:    indexer,
	})

	response := performJSON(router, http.MethodPost, "/api/workflow-candidates/"+candidate.ID+"/approve", nil)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if indexer.indexedWorkflowID != candidate.RecipeDraft.ID {
		t.Fatalf("expected indexed workflow %q, got %q", candidate.RecipeDraft.ID, indexer.indexedWorkflowID)
	}
}

func TestCandidateRoutesRejectDoesNotApprove(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	router := NewRouterWithServices(config.Config{}, Services{
		Candidates: registry.NewCandidateService(repo),
	})
	createResponse := performJSON(router, http.MethodPost, "/api/workflow-candidates/from-session", map[string]any{
		"projectId": "default",
		"source":    "user_demo",
		"task":      "Search GitHub issues",
		"startUrl":  "https://github.com/alibaba/page-agent",
		"recipe":    sampleHTTPWorkflowRecipe(),
	})
	var candidate registry.WorkflowCandidate
	if err := json.Unmarshal(createResponse.Body.Bytes(), &candidate); err != nil {
		t.Fatalf("decode candidate failed: %v", err)
	}

	rejectResponse := performJSON(router, http.MethodPost, "/api/workflow-candidates/"+candidate.ID+"/reject", nil)
	if rejectResponse.Code != http.StatusOK {
		t.Fatalf("expected reject status 200, got %d", rejectResponse.Code)
	}
	var rejected registry.WorkflowCandidate
	if err := json.Unmarshal(rejectResponse.Body.Bytes(), &rejected); err != nil {
		t.Fatalf("decode rejected candidate failed: %v", err)
	}
	if rejected.Status != registry.StatusRejected || rejected.Searchable {
		t.Fatalf("rejected candidate should not be searchable: %#v", rejected)
	}
}

func performJSON(handler http.Handler, method string, path string, body any) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		content, _ := json.Marshal(body)
		reader = bytes.NewReader(content)
	}
	request := httptest.NewRequest(method, path, reader)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func sampleHTTPWorkflowRecipe() registry.WorkflowRecipe {
	return registry.WorkflowRecipe{
		ID:          "wf_github_issue_search",
		Version:     3,
		ProjectID:   "default",
		Status:      registry.StatusPendingReview,
		Searchable:  false,
		Site:        "github.com",
		App:         "github",
		Name:        "Search GitHub issues",
		Intent:      "Search issues in a GitHub repository",
		Description: "Open a repository Issues page and search by query.",
		RiskLevel:   registry.RiskReadOrSearch,
		Variables: []registry.Variable{
			{Name: "repo", Type: registry.VariableString, Required: true, Source: registry.VariableSourceTaskOrURL},
			{Name: "query", Type: registry.VariableString, Required: true, Source: registry.VariableSourceTask},
		},
		Chunks: []registry.WorkflowChunk{
			{
				ID:            "open_issues",
				Name:          "Open Issues",
				FromPageState: "github_repo_home",
				ToPageState:   "github_issues_list",
				RiskLevel:     registry.RiskReadOrSearch,
				Steps: []registry.WorkflowStep{
					{ID: "click_issues", Type: registry.StepClick, RiskLevel: registry.RiskReadOrSearch},
				},
			},
		},
	}
}
