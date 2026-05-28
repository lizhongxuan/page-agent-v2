package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/page-agent/workflow-backend/internal/recipe"
	"github.com/page-agent/workflow-backend/internal/workflow"
)

func TestCandidateFromSessionRequiresApprovalBeforeWorkflowSearch(t *testing.T) {
	workflowService := workflow.NewService(workflow.NewMemoryRepository())
	router := NewRouter(Dependencies{
		Workflow:  workflowService,
		Candidate: recipe.NewCandidateService(recipe.NewMemoryCandidateRepository(), workflowService),
	})

	session := recipe.RecordedSession{
		ID:        "session_1",
		ProjectID: "default",
		Source:    recipe.CandidateSourceUserDemo,
		Task:      "Search for docs",
		StartURL:  "https://example.com/",
		Site:      "example.com",
		PageBefore: recipe.RecordedPageState{
			URL:               "https://example.com/",
			Title:             "Example",
			VisibleText:       []string{"Search"},
			ControlSignatures: []workflow.ControlSignature{{Role: "textbox", Name: "Search"}},
		},
		Events: []recipe.RecordedEvent{
			{
				ID:    "event_1",
				Type:  recipe.EventTypeInput,
				Label: "Search",
				TargetCandidates: []workflow.TargetCandidate{
					{Strategy: workflow.TargetStrategyRole, Role: "textbox", Name: "Search"},
				},
			},
		},
	}
	payload, _ := json.Marshal(session)
	createReq := httptest.NewRequest(http.MethodPost, "/api/workflow-candidates/from-session", bytes.NewReader(payload))
	createRes := httptest.NewRecorder()
	router.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected candidate create 201, got %d %s", createRes.Code, createRes.Body.String())
	}
	var candidate recipe.WorkflowCandidate
	if err := json.Unmarshal(createRes.Body.Bytes(), &candidate); err != nil {
		t.Fatalf("decode candidate: %v", err)
	}
	if candidate.Source != recipe.CandidateSourceUserDemo {
		t.Fatalf("expected user_demo source, got %q", candidate.Source)
	}
	if candidate.ReviewStatus != recipe.ReviewStatusPending {
		t.Fatalf("expected pending review status, got %q", candidate.ReviewStatus)
	}
	if candidate.Searchable {
		t.Fatal("pending candidate should not be searchable")
	}

	searchBody := `{"projectId":"default","task":"Search for docs","url":"https://example.com/","pageFingerprint":{"requiredText":["Search"],"controlSignatures":[{"role":"textbox","name":"Search"}]},"limit":5}`
	searchReq := httptest.NewRequest(http.MethodPost, "/api/workflows/search", bytes.NewBufferString(searchBody))
	searchRes := httptest.NewRecorder()
	router.ServeHTTP(searchRes, searchReq)
	var pendingSearch struct {
		Workflows []workflow.SearchResult `json:"workflows"`
	}
	if err := json.Unmarshal(searchRes.Body.Bytes(), &pendingSearch); err != nil {
		t.Fatalf("decode pending search: %v", err)
	}
	if len(pendingSearch.Workflows) != 0 {
		t.Fatalf("pending candidate should not appear in search: %#v", pendingSearch.Workflows)
	}

	approveReq := httptest.NewRequest(http.MethodPost, "/api/workflow-candidates/"+candidate.ID+"/approve", nil)
	approveRes := httptest.NewRecorder()
	router.ServeHTTP(approveRes, approveReq)
	if approveRes.Code != http.StatusOK {
		t.Fatalf("expected approve 200, got %d %s", approveRes.Code, approveRes.Body.String())
	}
	var approved recipe.WorkflowCandidate
	if err := json.Unmarshal(approveRes.Body.Bytes(), &approved); err != nil {
		t.Fatalf("decode approved candidate: %v", err)
	}
	if approved.ReviewStatus != recipe.ReviewStatusApproved {
		t.Fatalf("expected approved review status, got %q", approved.ReviewStatus)
	}
	if !approved.Searchable {
		t.Fatal("approved candidate should be searchable")
	}

	searchReq = httptest.NewRequest(http.MethodPost, "/api/workflows/search", bytes.NewBufferString(searchBody))
	searchRes = httptest.NewRecorder()
	router.ServeHTTP(searchRes, searchReq)
	var approvedSearch struct {
		Workflows []workflow.SearchResult `json:"workflows"`
	}
	if err := json.Unmarshal(searchRes.Body.Bytes(), &approvedSearch); err != nil {
		t.Fatalf("decode approved search: %v", err)
	}
	if len(approvedSearch.Workflows) != 1 {
		t.Fatalf("approved candidate should create searchable workflow, got %#v", approvedSearch.Workflows)
	}
}

func TestCandidateRejectKeepsWorkflowOutOfSearch(t *testing.T) {
	workflowService := workflow.NewService(workflow.NewMemoryRepository())
	router := NewRouter(Dependencies{
		Workflow:  workflowService,
		Candidate: recipe.NewCandidateService(recipe.NewMemoryCandidateRepository(), workflowService),
	})
	candidate := createCandidateFromSession(t, router, "session_reject", "default", recipe.CandidateSourceUserDemo)

	rejectReq := httptest.NewRequest(http.MethodPost, "/api/workflow-candidates/"+candidate.ID+"/reject", nil)
	rejectRes := httptest.NewRecorder()
	router.ServeHTTP(rejectRes, rejectReq)
	if rejectRes.Code != http.StatusOK {
		t.Fatalf("expected reject 200, got %d %s", rejectRes.Code, rejectRes.Body.String())
	}
	var rejected recipe.WorkflowCandidate
	if err := json.Unmarshal(rejectRes.Body.Bytes(), &rejected); err != nil {
		t.Fatalf("decode rejected candidate: %v", err)
	}
	if rejected.ReviewStatus != recipe.ReviewStatusRejected {
		t.Fatalf("expected rejected status, got %q", rejected.ReviewStatus)
	}
	if rejected.Searchable {
		t.Fatal("rejected candidate should not be searchable")
	}

	searchReq := httptest.NewRequest(http.MethodPost, "/api/workflows/search", bytes.NewBufferString(searchRequestBody("default")))
	searchRes := httptest.NewRecorder()
	router.ServeHTTP(searchRes, searchReq)
	var response struct {
		Workflows []workflow.SearchResult `json:"workflows"`
	}
	if err := json.Unmarshal(searchRes.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode search response: %v", err)
	}
	if len(response.Workflows) != 0 {
		t.Fatalf("rejected candidate should not appear in search: %#v", response.Workflows)
	}
}

func TestCandidateListFiltersByProjectSourceAndReviewStatus(t *testing.T) {
	workflowService := workflow.NewService(workflow.NewMemoryRepository())
	router := NewRouter(Dependencies{
		Workflow:  workflowService,
		Candidate: recipe.NewCandidateService(recipe.NewMemoryCandidateRepository(), workflowService),
	})
	createCandidateFromSession(t, router, "session_pending", "default", recipe.CandidateSourceUserDemo)
	approved := createCandidateFromSession(t, router, "session_approved", "default", recipe.CandidateSourceUserDemo)
	createCandidateFromSession(t, router, "session_other_project", "other", recipe.CandidateSourceUserDemo)
	createCandidateFromSession(t, router, "session_agent_run", "default", recipe.CandidateSourceAgentRun)

	approveReq := httptest.NewRequest(http.MethodPost, "/api/workflow-candidates/"+approved.ID+"/approve", nil)
	approveRes := httptest.NewRecorder()
	router.ServeHTTP(approveRes, approveReq)
	if approveRes.Code != http.StatusOK {
		t.Fatalf("expected approve 200, got %d %s", approveRes.Code, approveRes.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/workflow-candidates?projectId=default&source=user_demo&reviewStatus=pending", nil)
	listRes := httptest.NewRecorder()
	router.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d %s", listRes.Code, listRes.Body.String())
	}
	var response struct {
		Candidates []recipe.WorkflowCandidate `json:"candidates"`
	}
	if err := json.Unmarshal(listRes.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(response.Candidates) != 1 {
		t.Fatalf("expected one pending user_demo candidate, got %#v", response.Candidates)
	}
	if got := response.Candidates[0]; got.ProjectID != "default" || got.Source != recipe.CandidateSourceUserDemo || got.ReviewStatus != recipe.ReviewStatusPending {
		t.Fatalf("candidate did not match filters: %#v", got)
	}
}

func createCandidateFromSession(t *testing.T, router http.Handler, sessionID string, projectID string, source recipe.CandidateSource) recipe.WorkflowCandidate {
	t.Helper()
	session := recipe.RecordedSession{
		ID:        sessionID,
		ProjectID: projectID,
		Source:    source,
		Task:      "Search for docs",
		StartURL:  "https://example.com/",
		Site:      "example.com",
		PageBefore: recipe.RecordedPageState{
			URL:               "https://example.com/",
			Title:             "Example",
			VisibleText:       []string{"Search"},
			ControlSignatures: []workflow.ControlSignature{{Role: "textbox", Name: "Search"}},
		},
		Events: []recipe.RecordedEvent{
			{
				ID:    "event_1",
				Type:  recipe.EventTypeInput,
				Label: "Search",
				TargetCandidates: []workflow.TargetCandidate{
					{Strategy: workflow.TargetStrategyRole, Role: "textbox", Name: "Search"},
				},
			},
		},
	}
	payload, _ := json.Marshal(session)
	createReq := httptest.NewRequest(http.MethodPost, "/api/workflow-candidates/from-session", bytes.NewReader(payload))
	createRes := httptest.NewRecorder()
	router.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected candidate create 201, got %d %s", createRes.Code, createRes.Body.String())
	}
	var candidate recipe.WorkflowCandidate
	if err := json.Unmarshal(createRes.Body.Bytes(), &candidate); err != nil {
		t.Fatalf("decode candidate: %v", err)
	}
	return candidate
}

func searchRequestBody(projectID string) string {
	return `{"projectId":"` + projectID + `","task":"Search for docs","url":"https://example.com/","pageFingerprint":{"requiredText":["Search"],"controlSignatures":[{"role":"textbox","name":"Search"}]},"limit":5}`
}
