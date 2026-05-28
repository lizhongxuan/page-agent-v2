package workflow

import (
	"strings"
	"testing"
)

func TestValidateRecipeAcceptsValidRecipe(t *testing.T) {
	recipe := validRecipe()

	if err := ValidateRecipe(recipe); err != nil {
		t.Fatalf("ValidateRecipe returned error for valid recipe: %v", err)
	}
}

func TestValidateRecipeRejectsUnknownRiskLevel(t *testing.T) {
	recipe := validRecipe()
	recipe.Chunks[0].RiskLevel = RiskLevel("launch_missiles")

	err := ValidateRecipe(recipe)
	if err == nil {
		t.Fatal("ValidateRecipe returned nil for unknown risk level")
	}
	if !strings.Contains(err.Error(), "unknown risk level") {
		t.Fatalf("expected unknown risk level error, got %v", err)
	}
}

func TestValidateRecipeRejectsSensitiveVariableWithoutHandover(t *testing.T) {
	recipe := validRecipe()
	recipe.Variables = append(recipe.Variables, Variable{
		Name:        "password",
		Type:        VariableTypeString,
		Required:    true,
		Source:      VariableSourceUserTask,
		Sensitive:   true,
		BindingMode: BindingModeAlwaysConfirm,
	})

	err := ValidateRecipe(recipe)
	if err == nil {
		t.Fatal("ValidateRecipe returned nil for sensitive variable with non-handover binding")
	}
	if !strings.Contains(err.Error(), "sensitive variable") {
		t.Fatalf("expected sensitive variable error, got %v", err)
	}
}

func TestValidateRecipeRejectsIndexOnlyTargets(t *testing.T) {
	recipe := validRecipe()
	recipe.Chunks[0].Steps[0].Target = StepTarget{
		Preferred: TargetCandidate{Strategy: TargetStrategyDOMIndex, Index: 42},
	}

	err := ValidateRecipe(recipe)
	if err == nil {
		t.Fatal("ValidateRecipe returned nil for index-only target")
	}
	if !strings.Contains(err.Error(), "historical DOM index") {
		t.Fatalf("expected historical DOM index error, got %v", err)
	}
}

func validRecipe() WorkflowRecipe {
	return WorkflowRecipe{
		ID:          "wf_search",
		ProjectID:   "default",
		Site:        "example.com",
		Name:        "Search Example",
		Description: "Search for a query",
		Intent:      "search example",
		Tags:        []string{"search"},
		URLPatterns: []string{"https://example.com/*"},
		PageFingerprint: PageFingerprint{
			URLPatterns:       []string{"https://example.com/*"},
			TitleAny:          []string{"Example"},
			RequiredText:      []string{"Search"},
			ControlSignatures: []ControlSignature{{Role: "textbox", Name: "Search"}},
			MinMatchScore:     0.7,
		},
		Variables: []Variable{
			{
				Name:        "query",
				Type:        VariableTypeString,
				Required:    true,
				Source:      VariableSourceUserTask,
				Sensitive:   false,
				BindingMode: BindingModeAskIfMissing,
			},
		},
		Chunks: []WorkflowChunk{
			{
				ID:        "chunk_search",
				Name:      "Search",
				RiskLevel: RiskLevelSubmitSearch,
				Steps: []WorkflowStep{
					{
						ID:     "step_input_query",
						Type:   StepTypeInput,
						Target: StepTarget{Preferred: TargetCandidate{Strategy: TargetStrategyRole, Role: "textbox", Name: "Search"}},
						Value:  "{{query}}",
					},
					{
						ID:     "step_click_search",
						Type:   StepTypeClick,
						Target: StepTarget{Preferred: TargetCandidate{Strategy: TargetStrategyRole, Role: "button", Name: "Search"}},
					},
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
