package recipe

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/page-agent/workflow-backend/internal/workflow"
)

func TestFileCandidateRepositoryPersistsCandidates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "candidates.json")
	repo, err := NewFileCandidateRepository(path)
	if err != nil {
		t.Fatal(err)
	}
	candidate := WorkflowCandidate{
		ID:           "cand_file",
		ProjectID:    "default",
		Source:       CandidateSourceUserDemo,
		Task:         "Search issue",
		StartURL:     "https://example.test",
		ReviewStatus: ReviewStatusPending,
		RecipeDraft: workflow.WorkflowRecipe{
			ID:           "wf_file_candidate",
			ProjectID:    "default",
			Site:         "example.test",
			Name:         "File candidate",
			Intent:       "search issue",
			Chunks:       []workflow.WorkflowChunk{{ID: "chunk", Name: "Chunk", RiskLevel: workflow.RiskLevelReadOnly, Steps: []workflow.WorkflowStep{{ID: "step", Type: workflow.StepTypeClick, Target: workflow.StepTarget{Preferred: workflow.TargetCandidate{Strategy: workflow.TargetStrategyText, Value: "Search"}}}}}},
			SafetyPolicy: workflow.SafetyPolicy{AllowedRiskLevels: []workflow.RiskLevel{workflow.RiskLevelReadOnly}},
			Status:       workflow.WorkflowStatusDraft,
			Version:      1,
		},
	}
	if _, err := repo.Save(context.Background(), candidate); err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewFileCandidateRepository(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reloaded.Get(context.Background(), "cand_file")
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != CandidateSourceUserDemo || got.RecipeDraft.Name != "File candidate" {
		t.Fatalf("unexpected persisted candidate: %#v", got)
	}
}
