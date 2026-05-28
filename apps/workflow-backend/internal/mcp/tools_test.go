package mcp

import (
	"context"
	"testing"

	"github.com/page-agent/workflow-backend/internal/artifact"
	"github.com/page-agent/workflow-backend/internal/knowledge"
	"github.com/page-agent/workflow-backend/internal/recipe"
	"github.com/page-agent/workflow-backend/internal/workflow"
)

func TestRegistryExposesWorkflowMemoryToolsButNoBrowserControl(t *testing.T) {
	registry := NewRegistry(Dependencies{})
	names := registry.ToolNames()

	for _, expected := range []string{
		"search_site_knowledge",
		"search_workflows",
		"get_workflow",
		"prepare_workflow_run",
		"store_run_artifact",
	} {
		if !contains(names, expected) {
			t.Fatalf("expected tool %q in %#v", expected, names)
		}
	}
	if contains(names, "run_workflow") {
		t.Fatalf("run_workflow must not be exposed: %#v", names)
	}
}

func TestPrepareWorkflowRunReturnsPlanOnly(t *testing.T) {
	workflowService := workflow.NewService(workflow.NewMemoryRepository())
	recipe := workflow.WorkflowRecipe{
		ID:          "wf_search",
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
		SafetyPolicy: workflow.SafetyPolicy{AllowedRiskLevels: []workflow.RiskLevel{workflow.RiskLevelSubmitSearch}},
		Status:       workflow.WorkflowStatusActive,
		Version:      1,
	}
	if _, err := workflowService.Create(context.Background(), recipe); err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	registry := NewRegistry(Dependencies{Workflow: workflowService})
	result, err := registry.Call(context.Background(), "prepare_workflow_run", map[string]any{
		"projectId": "default",
		"task":      "search docs",
		"url":       "https://example.com/search",
		"pageFingerprint": map[string]any{
			"requiredText":      []any{"Search"},
			"controlSignatures": []any{map[string]any{"role": "textbox", "name": "Search"}},
		},
	})
	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}
	plan, ok := result.(PreparedWorkflowRun)
	if !ok {
		t.Fatalf("expected PreparedWorkflowRun, got %#v", result)
	}
	if plan.WorkflowID != "wf_search" || plan.ControlsBrowser {
		t.Fatalf("unexpected plan: %#v", plan)
	}
}

func TestStoreRunArtifactRejectsScreenshots(t *testing.T) {
	store := artifact.NewLocalStore(t.TempDir())
	service := artifact.NewService(store, artifact.NewMemoryRepository())
	registry := NewRegistry(Dependencies{Artifact: service})

	_, err := registry.Call(context.Background(), "store_run_artifact", map[string]any{
		"projectId":   "default",
		"ownerType":   "workflow_run",
		"ownerId":     "run_1",
		"kind":        "screenshot",
		"contentType": "image/png",
		"content":     "not allowed",
	})
	if err == nil {
		t.Fatal("expected screenshot artifact rejected")
	}
}

func TestImportWorkflowUseToolReturnsCandidateAndDoesNotControlBrowser(t *testing.T) {
	registry := NewRegistry(Dependencies{})

	result, err := registry.Call(context.Background(), "import_workflow_use", map[string]any{
		"projectId": "default",
		"source": `{
			"name": "Search",
			"description": "Search docs",
			"urlPatterns": ["https://example.com/*"],
			"input_schema": [{"name": "query", "type": "string", "required": true}],
			"steps": [
				{"type": "input", "target_text": "Search", "value": "{query}"},
				{"type": "click", "target_text": "Search"}
			]
		}`,
	})
	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}
	imported, ok := result.(recipe.ImportWorkflowUseResult)
	if !ok {
		t.Fatalf("expected ImportWorkflowUseResult, got %#v", result)
	}
	if imported.Candidate.RecipeDraft.Chunks[0].Steps[0].Value != "{{query}}" {
		t.Fatalf("workflow-use variable was not converted: %#v", imported.Candidate.RecipeDraft.Chunks[0].Steps[0])
	}
	if _, hasControlsBrowser := result.(interface{ ControlsBrowser() bool }); hasControlsBrowser {
		t.Fatalf("import tool must not expose browser control result: %#v", result)
	}
}

func TestExportWorkflowUseToolFetchesRecipePlanOnly(t *testing.T) {
	workflowService := workflow.NewService(workflow.NewMemoryRepository())
	created, err := workflowService.Create(context.Background(), workflow.WorkflowRecipe{
		ID:          "wf_export",
		ProjectID:   "default",
		Site:        "example.com",
		Name:        "Export me",
		Intent:      "export a recipe",
		URLPatterns: []string{"https://example.com/*"},
		Variables: []workflow.Variable{
			{Name: "password", Type: workflow.VariableTypeString, Required: true, Source: workflow.VariableSourceUserTask, Sensitive: true, BindingMode: workflow.BindingModeHandoverOnly},
		},
		Chunks: []workflow.WorkflowChunk{{
			ID:        "chunk_one",
			Name:      "One",
			RiskLevel: workflow.RiskLevelFormFill,
			Steps: []workflow.WorkflowStep{{
				ID:    "step_password",
				Type:  workflow.StepTypeInput,
				Value: "{{password}}",
				Target: workflow.StepTarget{
					Preferred: workflow.TargetCandidate{Strategy: workflow.TargetStrategyLabel, Value: "Password"},
				},
			}},
		}},
		SafetyPolicy: workflow.SafetyPolicy{AllowedRiskLevels: []workflow.RiskLevel{workflow.RiskLevelFormFill}},
		Status:       workflow.WorkflowStatusActive,
		Version:      1,
	})
	if err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	registry := NewRegistry(Dependencies{Workflow: workflowService})

	result, err := registry.Call(context.Background(), "export_workflow_use", map[string]any{"workflowId": created.ID})
	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}
	exported, ok := result.(recipe.WorkflowUseExport)
	if !ok {
		t.Fatalf("expected WorkflowUseExport, got %#v", result)
	}
	if exported.Steps[0].Value != recipe.RedactedValue {
		t.Fatalf("expected sensitive step value redacted, got %#v", exported.Steps[0])
	}
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

var _ = knowledge.Hit{}
