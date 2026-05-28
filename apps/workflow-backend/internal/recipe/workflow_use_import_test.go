package recipe

import (
	"strings"
	"testing"

	"github.com/page-agent/workflow-backend/internal/workflow"
)

func TestImportWorkflowUseJSONBuildsDraftCandidateAndWarnings(t *testing.T) {
	source := []byte(`{
		"name": "Government Form Submission",
		"description": "Fill a government form",
		"intent": "Submit the government form",
		"urlPatterns": ["https://example.com/forms/*"],
		"input_schema": [
			{"name": "first_name", "type": "string", "required": true},
			{"name": "password", "type": "string", "required": true}
		],
		"steps": [
			{"type": "navigation", "url": "https://example.com/forms/start"},
			{"type": "input", "target_text": "First name", "value": "{first_name}", "selectorStrategies": [{"type": "css", "value": "#firstName"}]},
			{"type": "input", "target_text": "Password", "value": "{password}"},
			{"type": "click", "target_text": "Submit", "cssSelector": "button[type=submit]"},
			{"type": "key_press", "target_text": "First name", "key": "Enter"}
		]
	}`)

	result, err := ImportWorkflowUse(source, ImportWorkflowUseOptions{ProjectID: "default"})
	if err != nil {
		t.Fatalf("ImportWorkflowUse returned error: %v", err)
	}

	if result.Candidate.Source != CandidateSourceImportedWorkflowUse {
		t.Fatalf("expected imported workflow-use source, got %q", result.Candidate.Source)
	}
	if result.Candidate.RecipeDraft.Name != "Government Form Submission" {
		t.Fatalf("unexpected recipe name: %q", result.Candidate.RecipeDraft.Name)
	}
	if result.Candidate.RecipeDraft.Intent != "Submit the government form" {
		t.Fatalf("unexpected intent: %q", result.Candidate.RecipeDraft.Intent)
	}
	if got := result.Candidate.RecipeDraft.URLPatterns; len(got) != 1 || got[0] != "https://example.com/forms/*" {
		t.Fatalf("url patterns were not preserved: %#v", got)
	}
	if got := result.Candidate.RecipeDraft.Chunks[0].Steps; len(got) != 3 {
		t.Fatalf("expected three mapped executable steps, got %#v", got)
	}
	firstStep := result.Candidate.RecipeDraft.Chunks[0].Steps[0]
	if firstStep.Type != workflow.StepTypeInput || firstStep.Value != "{{first_name}}" {
		t.Fatalf("first step was not converted to PageAgent input variable: %#v", firstStep)
	}
	if firstStep.Target.Preferred.Strategy != workflow.TargetStrategyText || firstStep.Target.Preferred.Value != "First name" {
		t.Fatalf("target_text should be preferred locator: %#v", firstStep.Target)
	}
	if len(firstStep.Target.Fallbacks) != 1 || firstStep.Target.Fallbacks[0].Strategy != workflow.TargetStrategyCSS {
		t.Fatalf("selector strategy metadata should become fallback locator: %#v", firstStep.Target)
	}
	password := findVariable(t, result.Candidate.RecipeDraft.Variables, "password")
	if !password.Sensitive || password.BindingMode != workflow.BindingModeHandoverOnly {
		t.Fatalf("password variable should be sensitive handover_only: %#v", password)
	}
	if result.Source == nil || len(result.Source.Raw) == 0 {
		t.Fatal("expected raw source to be retained for caller-managed artifact storage")
	}
	if len(result.Report.UnsupportedSteps) != 2 {
		t.Fatalf("expected navigation and key_press warnings, got %#v", result.Report.UnsupportedSteps)
	}
	if !containsWarning(result.Report.Warnings, "navigation") {
		t.Fatalf("expected navigation warning, got %#v", result.Report.Warnings)
	}
}

func TestImportWorkflowUseRejectsMissingSteps(t *testing.T) {
	_, err := ImportWorkflowUse([]byte(`{"name":"Empty","steps":[]}`), ImportWorkflowUseOptions{ProjectID: "default"})
	if err == nil {
		t.Fatal("expected missing steps error")
	}
}

func findVariable(t *testing.T, variables []workflow.Variable, name string) workflow.Variable {
	t.Helper()
	for _, variable := range variables {
		if variable.Name == name {
			return variable
		}
	}
	t.Fatalf("variable %q not found in %#v", name, variables)
	return workflow.Variable{}
}

func containsWarning(warnings []string, substring string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, substring) {
			return true
		}
	}
	return false
}
