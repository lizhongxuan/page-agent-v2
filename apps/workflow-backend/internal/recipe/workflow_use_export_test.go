package recipe

import (
	"encoding/json"
	"testing"

	"github.com/page-agent/workflow-backend/internal/workflow"
)

func TestExportWorkflowUsePreservesVariablesStepsAndLocatorMetadata(t *testing.T) {
	recipe := workflow.WorkflowRecipe{
		ID:          "wf_form",
		ProjectID:   "default",
		Site:        "example.com",
		Name:        "Form",
		Description: "Fill form",
		Intent:      "Submit form",
		URLPatterns: []string{"https://example.com/forms/*"},
		Variables: []workflow.Variable{
			{Name: "first_name", Type: workflow.VariableTypeString, Required: true, Source: workflow.VariableSourceUserTask, BindingMode: workflow.BindingModeAskIfMissing},
			{Name: "password", Type: workflow.VariableTypeString, Required: true, Source: workflow.VariableSourceUserTask, Sensitive: true, BindingMode: workflow.BindingModeHandoverOnly},
		},
		Chunks: []workflow.WorkflowChunk{{
			ID:        "chunk_fill",
			Name:      "Fill",
			RiskLevel: workflow.RiskLevelFormFill,
			Steps: []workflow.WorkflowStep{{
				ID:   "step_first_name",
				Type: workflow.StepTypeInput,
				Target: workflow.StepTarget{
					Preferred: workflow.TargetCandidate{Strategy: workflow.TargetStrategyText, Value: "First name"},
					Fallbacks: []workflow.TargetCandidate{{Strategy: workflow.TargetStrategyCSS, Value: "#firstName"}},
				},
				Value: "{{first_name}}",
			}, {
				ID:    "step_password",
				Type:  workflow.StepTypeInput,
				Value: "{{password}}",
				Target: workflow.StepTarget{
					Preferred: workflow.TargetCandidate{Strategy: workflow.TargetStrategyLabel, Value: "Password"},
				},
			}, {
				ID:   "step_submit",
				Type: workflow.StepTypeClick,
				Target: workflow.StepTarget{
					Preferred: workflow.TargetCandidate{Strategy: workflow.TargetStrategyRole, Role: "button", Name: "Submit"},
				},
			}},
		}},
		Version: 3,
	}

	exported := ExportWorkflowUse(recipe)

	if exported.Name != "Form" || exported.Intent != "Submit form" {
		t.Fatalf("top-level fields not preserved: %#v", exported)
	}
	if len(exported.InputSchema) != 2 {
		t.Fatalf("variables not exported: %#v", exported.InputSchema)
	}
	if exported.InputSchema[1].Name != "password" || !exported.InputSchema[1].Sensitive {
		t.Fatalf("sensitive variable metadata not preserved: %#v", exported.InputSchema[1])
	}
	if len(exported.Chunks) != 1 || exported.Chunks[0].ID != "chunk_fill" {
		t.Fatalf("chunk metadata not preserved: %#v", exported.Chunks)
	}
	if len(exported.Steps) != 3 {
		t.Fatalf("steps not exported: %#v", exported.Steps)
	}
	first := exported.Steps[0]
	if first.Type != "input" || first.Value != "{first_name}" || first.TargetText != "First name" {
		t.Fatalf("input step not workflow-use-like: %#v", first)
	}
	if len(first.LocatorCandidates) != 2 || first.LocatorCandidates[1].Strategy != string(workflow.TargetStrategyCSS) {
		t.Fatalf("locator candidates metadata not preserved: %#v", first.LocatorCandidates)
	}
	password := exported.Steps[1]
	if password.Value != RedactedValue {
		t.Fatalf("sensitive variable step should be redacted, got %q", password.Value)
	}
	data, err := json.Marshal(exported)
	if err != nil {
		t.Fatalf("marshal exported workflow: %v", err)
	}
	if string(data) == "" || containsWarning([]string{string(data)}, "correct horse") {
		t.Fatalf("export leaked raw sensitive-looking value: %s", data)
	}
}
