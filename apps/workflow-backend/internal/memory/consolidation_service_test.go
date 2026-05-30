package memory

import (
	"context"
	"testing"

	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestMemoryConsolidationServiceOptimizesSuccessfulTaskRun(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	seedConsolidationPages(t, ctx, repo)
	service := NewMemoryConsolidationService(repo)
	run := sampleSuccessfulTaskRun()
	run.OriginalPath = []string{"A", "B", "C", "A", "D"}
	run.OptimizedPath = nil
	run.ActionSteps = []registry.ActionStep{
		{ID: "step-a-b", PageStateID: "A", StepIndex: 1, ActionType: registry.StepClick, TargetName: "Open B"},
		{ID: "step-b-c", PageStateID: "B", StepIndex: 2, ActionType: registry.StepClick, TargetName: "Open C"},
		{ID: "step-c-a", PageStateID: "C", StepIndex: 3, ActionType: registry.StepClick, TargetName: "Back A"},
		{ID: "step-a-d", PageStateID: "A", StepIndex: 4, ActionType: registry.StepClick, TargetName: "Open D"},
	}

	result, err := service.ConsolidateTaskRun(ctx, run)
	if err != nil {
		t.Fatalf("ConsolidateTaskRun failed: %v", err)
	}
	if len(result.OptimizedPath) != 2 || result.OptimizedPath[0] != "A" || result.OptimizedPath[1] != "D" {
		t.Fatalf("unexpected optimized path: %#v", result)
	}
	if len(result.BranchNoise) != 3 {
		t.Fatalf("expected branch noise, got %#v", result.BranchNoise)
	}
	if result.ExperienceID == "" {
		t.Fatalf("expected experience update: %#v", result)
	}
	got, err := repo.GetTaskRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetTaskRun failed: %v", err)
	}
	if got.ActionSteps[0].IsBranchNoise != true {
		t.Fatalf("expected noisy steps marked: %#v", got.ActionSteps)
	}
}

func TestMemoryConsolidationServiceRecordsFailure(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	seedConsolidationPages(t, ctx, repo)
	service := NewMemoryConsolidationService(repo)
	run := sampleSuccessfulTaskRun()
	run.Status = registry.TaskRunFailed

	result, err := service.ConsolidateTaskRun(ctx, run)
	if err != nil {
		t.Fatalf("ConsolidateTaskRun failed: %v", err)
	}
	if result.FailureMemoryID == "" {
		t.Fatalf("expected failure memory update: %#v", result)
	}
}

func TestMemoryConsolidationServiceAttributesUnusedEvidenceWithoutRewardingSuccess(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	seedConsolidationPages(t, ctx, repo)
	saveConsolidationContext(t, ctx, repo, "ctx_unused", []registry.MemoryEvidenceRef{
		{
			ID:          "exp_wrong_branch",
			Source:      registry.MemoryEvidenceSourceExperience,
			PageStateID: "B",
			Payload: map[string]any{
				"optimizedPath": []string{"A", "B"},
				"targetNames":   []string{"Open B"},
			},
		},
	})
	service := NewMemoryConsolidationService(repo)
	run := sampleSuccessfulTaskRun()
	run.MemoryContextID = "ctx_unused"
	run.OriginalPath = []string{"A", "D"}
	run.OptimizedPath = []string{"A", "D"}
	run.ActionSteps = []registry.ActionStep{{ID: "step-a-d", PageStateID: "A", StepIndex: 1, ActionType: registry.StepClick, TargetName: "Open D"}}

	result, err := service.ConsolidateTaskRun(ctx, run)
	if err != nil {
		t.Fatalf("ConsolidateTaskRun failed: %v", err)
	}
	if !containsString(result.MemoryUpdates, "updated_memory_attribution") || !containsString(result.MemoryUpdates, "updated_evidence_stats") {
		t.Fatalf("expected attribution updates, got %#v", result.MemoryUpdates)
	}
	stats, err := repo.GetMemoryEvidenceStats(ctx, "default", registry.MemoryEvidenceSourceExperience, "exp_wrong_branch")
	if err != nil {
		t.Fatalf("GetMemoryEvidenceStats failed: %v", err)
	}
	if stats.UnusedCount != 1 || stats.HelpfulCount != 0 {
		t.Fatalf("unused evidence should not be rewarded after success: %#v", stats)
	}
}

func TestMemoryConsolidationServiceDoesNotPunishUnusedEvidenceOnFailedTask(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	seedConsolidationPages(t, ctx, repo)
	saveConsolidationContext(t, ctx, repo, "ctx_failed_unused", []registry.MemoryEvidenceRef{
		{
			ID:          "exp_wrong_branch",
			Source:      registry.MemoryEvidenceSourceExperience,
			PageStateID: "B",
			Payload: map[string]any{
				"optimizedPath": []string{"A", "B"},
				"targetNames":   []string{"Open B"},
			},
		},
	})
	service := NewMemoryConsolidationService(repo)
	run := sampleSuccessfulTaskRun()
	run.Status = registry.TaskRunFailed
	run.MemoryContextID = "ctx_failed_unused"
	run.OriginalPath = []string{"A", "D"}
	run.OptimizedPath = []string{"A", "D"}
	run.ActionSteps = []registry.ActionStep{{ID: "step-a-d", PageStateID: "A", StepIndex: 1, ActionType: registry.StepClick, TargetName: "Open D", ResultSummary: "Target page timed out."}}

	if _, err := service.ConsolidateTaskRun(ctx, run); err != nil {
		t.Fatalf("ConsolidateTaskRun failed: %v", err)
	}
	stats, err := repo.GetMemoryEvidenceStats(ctx, "default", registry.MemoryEvidenceSourceExperience, "exp_wrong_branch")
	if err != nil {
		t.Fatalf("GetMemoryEvidenceStats failed: %v", err)
	}
	if stats.UnusedCount != 1 || stats.MisleadingCount != 0 {
		t.Fatalf("failed task must not punish unused evidence: %#v", stats)
	}
}

func TestMemoryConsolidationServicePunishesMisleadingEvidenceAndStoresCompetingExperience(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	seedConsolidationPages(t, ctx, repo)
	saveConsolidationContext(t, ctx, repo, "ctx_misleading", []registry.MemoryEvidenceRef{
		{
			ID:          "exp_wrong_branch",
			Source:      registry.MemoryEvidenceSourceExperience,
			PageStateID: "A",
			Payload: map[string]any{
				"optimizedPath": []string{"A", "B"},
				"targetNames":   []string{"Open B"},
			},
		},
	})
	service := NewMemoryConsolidationService(repo)
	run := sampleSuccessfulTaskRun()
	run.MemoryContextID = "ctx_misleading"
	run.OriginalPath = []string{"A", "B", "A", "D"}
	run.OptimizedPath = []string{"A", "D"}
	run.ActionSteps = []registry.ActionStep{
		{ID: "step-a-b", PageStateID: "A", StepIndex: 1, ActionType: registry.StepClick, TargetName: "Open B", IsBranchNoise: true},
		{ID: "step-a-d", PageStateID: "A", StepIndex: 2, ActionType: registry.StepClick, TargetName: "Open D"},
	}

	result, err := service.ConsolidateTaskRun(ctx, run)
	if err != nil {
		t.Fatalf("ConsolidateTaskRun failed: %v", err)
	}
	if result.ExperienceID == "" {
		t.Fatalf("expected successful path to still become competing experience: %#v", result)
	}
	stats, err := repo.GetMemoryEvidenceStats(ctx, "default", registry.MemoryEvidenceSourceExperience, "exp_wrong_branch")
	if err != nil {
		t.Fatalf("GetMemoryEvidenceStats failed: %v", err)
	}
	if stats.MisleadingCount != 1 || stats.UtilityScore >= 0 {
		t.Fatalf("expected misleading evidence penalty, got %#v", stats)
	}
}

func TestMemoryConsolidationServiceMarksStaleEvidenceFromCurrentPageState(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	seedConsolidationPages(t, ctx, repo)
	if err := repo.SavePageState(ctx, registry.PageState{
		ID:         "A",
		ProjectID:  "default",
		Site:       "ops.example.com",
		URLPattern: "https://ops.example.com/A",
		Status:     registry.StatusActive,
		RequiredControls: []registry.ControlSignature{
			{Role: "textbox", Name: "Service name"},
			{Role: "button", Name: "Search"},
		},
	}); err != nil {
		t.Fatalf("SavePageState failed: %v", err)
	}
	saveConsolidationContext(t, ctx, repo, "ctx_stale", []registry.MemoryEvidenceRef{
		{
			ID:          "exp_legacy_search",
			Source:      registry.MemoryEvidenceSourceExperience,
			PageStateID: "A",
			Payload: map[string]any{
				"targetNames": []string{"Legacy Search"},
			},
		},
	})
	service := NewMemoryConsolidationService(repo)
	run := sampleSuccessfulTaskRun()
	run.MemoryContextID = "ctx_stale"
	run.OriginalPath = []string{"A", "D"}
	run.OptimizedPath = []string{"A", "D"}
	run.ActionSteps = []registry.ActionStep{{ID: "step-a-d", PageStateID: "A", StepIndex: 1, ActionType: registry.StepClick, TargetName: "Open D"}}

	if _, err := service.ConsolidateTaskRun(ctx, run); err != nil {
		t.Fatalf("ConsolidateTaskRun failed: %v", err)
	}
	events, err := repo.ListMemoryAttributionEvents(ctx, registry.MemoryAttributionEventListQuery{
		ProjectID:  "default",
		TaskRunID:  run.ID,
		EvidenceID: "exp_legacy_search",
	})
	if err != nil {
		t.Fatalf("ListMemoryAttributionEvents failed: %v", err)
	}
	if len(events) != 1 || events[0].Label != registry.MemoryAttributionStale {
		t.Fatalf("expected stale attribution event, got %#v", events)
	}
	stats, err := repo.GetMemoryEvidenceStats(ctx, "default", registry.MemoryEvidenceSourceExperience, "exp_legacy_search")
	if err != nil {
		t.Fatalf("GetMemoryEvidenceStats failed: %v", err)
	}
	if stats.StaleCount != 1 || stats.UtilityScore >= 0 {
		t.Fatalf("expected stale evidence penalty, got %#v", stats)
	}
}

func seedConsolidationPages(t *testing.T, ctx context.Context, repo registry.Repository) {
	t.Helper()
	for _, id := range []string{"A", "B", "C", "D", "page_service_list", "page_service_detail"} {
		if err := repo.SavePageState(ctx, registry.PageState{
			ID:           id,
			ProjectID:    "default",
			Site:         "ops.example.com",
			URLPattern:   "https://ops.example.com/" + id,
			Status:       registry.StatusActive,
			HardRules:    registry.HardRules{URLPattern: "https://ops.example.com/" + id},
			RequiredText: []string{id},
		}); err != nil {
			t.Fatalf("SavePageState %s failed: %v", id, err)
		}
	}
	if err := repo.SavePageTransition(ctx, registry.PageTransition{
		ID:            "transition_a_d",
		ProjectID:     "default",
		Site:          "ops.example.com",
		FromPageState: "A",
		ToPageState:   "D",
		ActionName:    "Open D",
	}); err != nil {
		t.Fatalf("SavePageTransition failed: %v", err)
	}
}

func saveConsolidationContext(t *testing.T, ctx context.Context, repo registry.Repository, id string, refs []registry.MemoryEvidenceRef) {
	t.Helper()
	if err := repo.SaveMemoryContextEvent(ctx, registry.MemoryContextEvent{
		ID:              id,
		ProjectID:       "default",
		Task:            "查询服务状态",
		CurrentURL:      "https://ops.example.com/A",
		EvidenceRefs:    refs,
		RecommendedMode: registry.MemoryModeGuided,
	}); err != nil {
		t.Fatalf("SaveMemoryContextEvent failed: %v", err)
	}
}
