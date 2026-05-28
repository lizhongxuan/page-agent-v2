package qdrant

import (
	"strings"
	"testing"
	"time"

	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestWorkflowPointBuildersProduceDeterministicPayloadsAndSafeText(t *testing.T) {
	recipe := sampleWorkflowRecipe()
	card := WorkflowCardFromRecipe(recipe, []string{"Find timeout bugs in microsoft/playwright issues", "api_key=secret-value"})

	cardPoint, err := BuildWorkflowCardPoint(card)
	if err != nil {
		t.Fatalf("build card point failed: %v", err)
	}
	cardPointAgain, err := BuildWorkflowCardPoint(card)
	if err != nil {
		t.Fatalf("build card point failed: %v", err)
	}
	if !isUUIDPointID(cardPoint.ID) || cardPoint.ID != cardPointAgain.ID {
		t.Fatalf("unexpected card point id: %s", cardPoint.ID)
	}
	if cardPoint.Payload["doc_type"] != "workflow_card" || cardPoint.Payload["workflow_id"] != recipe.ID {
		t.Fatalf("unexpected card payload: %#v", cardPoint.Payload)
	}
	text := cardPoint.Payload["embedding_text"].(string)
	if strings.Contains(text, "secret-value") || strings.Contains(text, "api_key") {
		t.Fatalf("embedding text contains sensitive value: %s", text)
	}

	chunkPoints, err := BuildWorkflowChunkPoints(recipe)
	if err != nil {
		t.Fatalf("build chunk points failed: %v", err)
	}
	if len(chunkPoints) != 1 || !isUUIDPointID(chunkPoints[0].ID) {
		t.Fatalf("unexpected chunk points: %#v", chunkPoints)
	}
	if chunkPoints[0].Payload["from_page_state"] != "github_issues_list" {
		t.Fatalf("unexpected chunk payload: %#v", chunkPoints[0].Payload)
	}
	targetNames := chunkPoints[0].Payload["target_names"].([]string)
	if len(targetNames) != 1 || targetNames[0] != "Search" {
		t.Fatalf("unexpected target names: %#v", targetNames)
	}
	if strings.Contains(chunkPoints[0].Payload["embedding_text"].(string), "sk-1234567890") {
		t.Fatalf("chunk embedding text contains sensitive value: %s", chunkPoints[0].Payload["embedding_text"])
	}
}

func TestPageStateInterruptAndRepairPointBuilders(t *testing.T) {
	pageState := registry.PageState{
		ID:             "github_issues_list",
		ProjectID:      "default",
		Site:           "github.com",
		App:            "github",
		URLPattern:     "https://github.com/{owner}/{repo}/issues",
		RequiredText:   []string{"Issues", "Labels"},
		CanonicalTitle: "GitHub issues list",
		Status:         registry.StatusActive,
		UpdatedAt:      time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC),
		RequiredControls: []registry.ControlSignature{
			{Role: "textbox", Name: "Search"},
		},
	}
	pagePoint, err := BuildPageStatePoint(pageState)
	if err != nil {
		t.Fatalf("build page state point failed: %v", err)
	}
	if !isUUIDPointID(pagePoint.ID) {
		t.Fatalf("unexpected page point id: %s", pagePoint.ID)
	}
	if pagePoint.Payload["page_state_id"] != "github_issues_list" {
		t.Fatalf("unexpected page payload: %#v", pagePoint.Payload)
	}

	handler := registry.InterruptHandler{
		ID:                  "ih_cookie",
		ProjectID:           "default",
		Site:                "github.com",
		App:                 "github",
		Status:              registry.StatusActive,
		AppliesToPageStates: []string{"github_issues_list"},
		InterruptType:       "modal",
		FingerprintText:     "Cookie dialog with Close button",
		TargetControls:      []registry.ControlSignature{{Role: "button", Name: "Close"}},
		RiskLevel:           registry.RiskReadOnly,
		WorkflowID:          "wf_github_issue_search",
		Version:             3,
		SuccessRate:         0.96,
		UpdatedAt:           pageState.UpdatedAt,
	}
	handlerPoint, err := BuildInterruptHandlerPoint(handler)
	if err != nil {
		t.Fatalf("build interrupt handler point failed: %v", err)
	}
	if !isUUIDPointID(handlerPoint.ID) {
		t.Fatalf("unexpected handler point id: %s", handlerPoint.ID)
	}
	if handlerPoint.Payload["target_names"].([]string)[0] != "Close" {
		t.Fatalf("unexpected handler payload: %#v", handlerPoint.Payload)
	}

	patch := registry.RepairPatch{
		ID:                  "patch_searchbox",
		ProjectID:           "default",
		Status:              registry.StatusActive,
		WorkflowID:          "wf_github_issue_search",
		WorkflowVersion:     3,
		ChunkID:             "search_issues",
		StepID:              "fill_query",
		Site:                "github.com",
		App:                 "github",
		FailureType:         "locator_not_found",
		FailureSignature:    "search textbox locator failed",
		OldTarget:           "old secret=hidden",
		NewTargetSummary:    "Use issues search input placeholder",
		AppliesToPageStates: []string{"github_issues_list"},
		RiskLevel:           registry.RiskReadOrSearch,
		SuccessRate:         0.91,
		UpdatedAt:           pageState.UpdatedAt,
	}
	repairPoint, err := BuildRepairPatchPoint(patch)
	if err != nil {
		t.Fatalf("build repair patch point failed: %v", err)
	}
	if !isUUIDPointID(repairPoint.ID) {
		t.Fatalf("unexpected repair point id: %s", repairPoint.ID)
	}
	if strings.Contains(repairPoint.Payload["embedding_text"].(string), "hidden") {
		t.Fatalf("repair embedding text contains sensitive value: %s", repairPoint.Payload["embedding_text"])
	}
}

func isUUIDPointID(value string) bool {
	return len(value) == 36 &&
		value[8] == '-' &&
		value[13] == '-' &&
		value[18] == '-' &&
		value[23] == '-'
}

func sampleWorkflowRecipe() registry.WorkflowRecipe {
	return registry.WorkflowRecipe{
		ID:          "wf_github_issue_search",
		Version:     3,
		ProjectID:   "default",
		TenantID:    "local",
		Status:      registry.StatusActive,
		Searchable:  true,
		Site:        "github.com",
		App:         "github",
		Name:        "Search GitHub issues",
		Intent:      "Search issues in a GitHub repository",
		Description: "Open a repository Issues page and search by query.",
		Tags:        []string{"github", "issues", "search"},
		RiskLevel:   registry.RiskReadOrSearch,
		Variables: []registry.Variable{
			{Name: "repo", Type: registry.VariableString, Required: true, Source: registry.VariableSourceTaskOrURL},
			{Name: "query", Type: registry.VariableString, Required: true, Source: registry.VariableSourceTask},
			{Name: "token", Type: registry.VariableString, Required: false, Source: registry.VariableSourceUser, Sensitive: true},
		},
		Chunks: []registry.WorkflowChunk{
			{
				ID:                "search_issues",
				Name:              "Search Issues",
				FromPageState:     "github_issues_list",
				ToPageState:       "github_issues_search_results",
				PreconditionText:  "GitHub issues list page with search textbox",
				PostconditionText: "Issues search results page",
				StepSummary:       "Fill issue search query and press Enter with sk-1234567890",
				RiskLevel:         registry.RiskReadOrSearch,
				VariableNames:     []string{"query", "token"},
				SuccessRate:       0.93,
				SelectorHealth:    0.88,
				Steps: []registry.WorkflowStep{
					{
						ID:   "fill_query",
						Type: registry.StepFill,
						Target: registry.StepTarget{
							Primary: registry.TargetCandidate{Strategy: registry.TargetRole, Role: "textbox", Name: "Search"},
						},
						RiskLevel: registry.RiskReadOrSearch,
					},
				},
			},
		},
		StartPageStates: []string{"github_issues_list"},
		EndPageStates:   []string{"github_issues_search_results"},
		UpdatedAt:       time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC),
	}
}
