package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/page-agent/workflow-backend/internal/workflow"
)

func TestWorkflowCreatePromoteSearchHTTP(t *testing.T) {
	service := workflow.NewService(workflow.NewMemoryRepository())
	router := NewRouter(Dependencies{Workflow: service})

	recipe := workflow.WorkflowRecipe{
		ID:          "wf_search_http",
		ProjectID:   "default",
		Site:        "example.com",
		Name:        "Search",
		Intent:      "search docs",
		URLPatterns: []string{"https://example.com/*"},
		PageFingerprint: workflow.PageFingerprint{
			RequiredText:      []string{"Search"},
			ControlSignatures: []workflow.ControlSignature{{Role: "textbox", Name: "Search"}},
			MinMatchScore:     0.5,
		},
		Chunks: []workflow.WorkflowChunk{{
			ID:        "chunk_search",
			Name:      "Search",
			RiskLevel: workflow.RiskLevelSubmitSearch,
			Steps: []workflow.WorkflowStep{{
				ID:     "step_input",
				Type:   workflow.StepTypeInput,
				Target: workflow.StepTarget{Preferred: workflow.TargetCandidate{Strategy: workflow.TargetStrategyRole, Role: "textbox", Name: "Search"}},
				Value:  "{{query}}",
			}},
		}},
		SafetyPolicy: workflow.SafetyPolicy{
			AllowedRiskLevels:      []workflow.RiskLevel{workflow.RiskLevelReadOnly, workflow.RiskLevelFormFill, workflow.RiskLevelSubmitSearch},
			ConfirmationRiskLevels: []workflow.RiskLevel{workflow.RiskLevelStateChange},
			HandoverRiskLevels:     []workflow.RiskLevel{workflow.RiskLevelLoginSecret},
			BlockedRiskLevels:      []workflow.RiskLevel{workflow.RiskLevelPayment},
		},
		Status:  workflow.WorkflowStatusDraft,
		Version: 1,
	}
	payload, _ := json.Marshal(recipe)
	createReq := httptest.NewRequest(http.MethodPost, "/api/workflows", bytes.NewReader(payload))
	createRes := httptest.NewRecorder()
	router.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected create 201, got %d %s", createRes.Code, createRes.Body.String())
	}

	promoteReq := httptest.NewRequest(http.MethodPost, "/api/workflows/wf_search_http/promote", nil)
	promoteRes := httptest.NewRecorder()
	router.ServeHTTP(promoteRes, promoteReq)
	if promoteRes.Code != http.StatusOK {
		t.Fatalf("expected promote 200, got %d %s", promoteRes.Code, promoteRes.Body.String())
	}

	searchReq := httptest.NewRequest(http.MethodPost, "/api/workflows/search", bytes.NewBufferString(`{
		"projectId":"default",
		"task":"search docs",
		"url":"https://example.com/search",
		"pageFingerprint":{"requiredText":["Search"],"controlSignatures":[{"role":"textbox","name":"Search"}]},
		"limit":5
	}`))
	searchRes := httptest.NewRecorder()
	router.ServeHTTP(searchRes, searchReq)
	if searchRes.Code != http.StatusOK {
		t.Fatalf("expected search 200, got %d %s", searchRes.Code, searchRes.Body.String())
	}
	var response struct {
		Workflows []workflow.SearchResult `json:"workflows"`
	}
	if err := json.Unmarshal(searchRes.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode search response: %v", err)
	}
	if len(response.Workflows) != 1 || response.Workflows[0].Recipe.ID != "wf_search_http" {
		t.Fatalf("unexpected search response: %#v", response.Workflows)
	}
}
