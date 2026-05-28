package workflow

import (
	"context"
	"testing"
)

func TestServiceCreatesVersionsAndSearchesOnlyActiveWorkflows(t *testing.T) {
	service := NewService(NewMemoryRepository())
	ctx := context.Background()
	recipe := testRecipe()
	recipe.Status = WorkflowStatusDraft

	created, err := service.Create(ctx, recipe)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.Version != 1 {
		t.Fatalf("expected version 1, got %d", created.Version)
	}

	results, err := service.Search(ctx, SearchRequest{
		ProjectID:       "default",
		Task:            "search docs",
		URL:             "https://example.com/search",
		PageFingerprint: recipe.PageFingerprint,
		Limit:           5,
	})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("draft workflow should not be searchable: %#v", results)
	}

	if err := service.Promote(ctx, created.ID); err != nil {
		t.Fatalf("Promote returned error: %v", err)
	}
	results, err = service.Search(ctx, SearchRequest{
		ProjectID:       "default",
		Task:            "search docs",
		URL:             "https://example.com/search",
		PageFingerprint: recipe.PageFingerprint,
		Limit:           5,
	})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 || results[0].Recipe.ID != created.ID {
		t.Fatalf("expected promoted workflow in results, got %#v", results)
	}

	next := created
	next.Name = "Search Example V2"
	updated, err := service.AddVersion(ctx, created.ID, next)
	if err != nil {
		t.Fatalf("AddVersion returned error: %v", err)
	}
	if updated.Version != 2 {
		t.Fatalf("expected version 2, got %d", updated.Version)
	}

	if err := service.Disable(ctx, created.ID); err != nil {
		t.Fatalf("Disable returned error: %v", err)
	}
	results, err = service.Search(ctx, SearchRequest{
		ProjectID:       "default",
		Task:            "search docs",
		URL:             "https://example.com/search",
		PageFingerprint: recipe.PageFingerprint,
		Limit:           5,
	})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("disabled workflow should not be searchable: %#v", results)
	}
}

func testRecipe() WorkflowRecipe {
	return WorkflowRecipe{
		ID:          "wf_search",
		ProjectID:   "default",
		Site:        "example.com",
		Name:        "Search Example",
		Description: "Search docs",
		Intent:      "search docs",
		Tags:        []string{"search"},
		URLPatterns: []string{"https://example.com/*"},
		PageFingerprint: PageFingerprint{
			URLPatterns:       []string{"https://example.com/*"},
			RequiredText:      []string{"Search"},
			ControlSignatures: []ControlSignature{{Role: "textbox", Name: "Search"}},
			MinMatchScore:     0.5,
		},
		Variables: []Variable{
			{Name: "query", Type: VariableTypeString, Required: true, Source: VariableSourceUserTask, BindingMode: BindingModeAskIfMissing},
		},
		Chunks: []WorkflowChunk{
			{
				ID:        "chunk_search",
				Name:      "Search",
				RiskLevel: RiskLevelSubmitSearch,
				Steps: []WorkflowStep{
					{ID: "step_input", Type: StepTypeInput, Target: StepTarget{Preferred: TargetCandidate{Strategy: TargetStrategyRole, Role: "textbox", Name: "Search"}}, Value: "{{query}}"},
				},
			},
		},
		SafetyPolicy: SafetyPolicy{
			AllowedRiskLevels:      []RiskLevel{RiskLevelReadOnly, RiskLevelFormFill, RiskLevelSubmitSearch},
			ConfirmationRiskLevels: []RiskLevel{RiskLevelStateChange},
			HandoverRiskLevels:     []RiskLevel{RiskLevelLoginSecret, RiskLevelCaptcha, RiskLevelMFA},
			BlockedRiskLevels:      []RiskLevel{RiskLevelPayment, RiskLevelDelete},
		},
		Status:  WorkflowStatusDraft,
		Version: 1,
	}
}
