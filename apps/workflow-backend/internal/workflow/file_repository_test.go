package workflow

import (
	"context"
	"path/filepath"
	"testing"
)

func TestFileRepositoryPersistsWorkflows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflows.json")
	repo, err := NewFileRepository(path)
	if err != nil {
		t.Fatal(err)
	}
	recipe := WorkflowRecipe{
		ID:        "wf_file",
		ProjectID: "default",
		Site:      "example.test",
		Name:      "File workflow",
		Intent:    "search issue",
		Chunks: []WorkflowChunk{{
			ID:        "chunk",
			Name:      "Chunk",
			RiskLevel: RiskLevelReadOnly,
			Steps:     []WorkflowStep{{ID: "step", Type: StepTypeClick, Target: StepTarget{Preferred: TargetCandidate{Strategy: TargetStrategyText, Value: "Search"}}}},
		}},
		SafetyPolicy: SafetyPolicy{AllowedRiskLevels: []RiskLevel{RiskLevelReadOnly}},
		Status:       WorkflowStatusActive,
		Version:      1,
	}
	if _, err := repo.Save(context.Background(), recipe); err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewFileRepository(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reloaded.Get(context.Background(), "wf_file")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "File workflow" || got.Status != WorkflowStatusActive {
		t.Fatalf("unexpected persisted workflow: %#v", got)
	}
}
