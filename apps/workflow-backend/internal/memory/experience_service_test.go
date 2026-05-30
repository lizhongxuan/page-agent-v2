package memory

import (
	"context"
	"strings"
	"testing"

	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestExperienceServiceCreatesAndMergesSuccessfulTaskRuns(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewExperienceService(repo)
	run := sampleSuccessfulTaskRun()

	first, err := service.UpsertExperienceFromTaskRun(ctx, run)
	if err != nil {
		t.Fatalf("UpsertExperienceFromTaskRun first failed: %v", err)
	}
	second, err := service.UpsertExperienceFromTaskRun(ctx, run)
	if err != nil {
		t.Fatalf("UpsertExperienceFromTaskRun second failed: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected same experience id, got %q and %q", first.ID, second.ID)
	}
	if second.SuccessCount != 2 {
		t.Fatalf("expected success count to increment, got %#v", second)
	}
	if !second.Searchable {
		t.Fatalf("read/search success should be searchable: %#v", second)
	}
	if strings.Contains(second.SearchableText(), "kme-prod-001") || !strings.Contains(second.SearchableText(), "{{service_name}}") {
		t.Fatalf("searchable text should contain template only: %s", second.SearchableText())
	}
}

func TestExperienceServiceRequiresReviewForHighRisk(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewExperienceService(repo)
	run := sampleSuccessfulTaskRun()
	run.ActionSteps[0].ActionType = registry.StepClick
	run.ActionSteps[0].TargetName = "删除服务"

	experience, err := service.UpsertExperienceFromTaskRun(ctx, run)
	if err != nil {
		t.Fatalf("UpsertExperienceFromTaskRun failed: %v", err)
	}
	if experience.ReviewStatus != registry.ReviewStatusPending {
		t.Fatalf("expected pending review, got %#v", experience)
	}
}

func TestExperienceServiceStepsSummaryKeepsOnlyReusableSuccessfulSteps(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewExperienceService(repo)
	run := sampleSuccessfulTaskRun()
	run.OriginalPath = []string{"page_console", "page_incidents", "page_console", "page_service"}
	run.OptimizedPath = []string{"page_console", "page_service"}
	run.ActionSteps = []registry.ActionStep{
		{
			ID:            "wrong_branch",
			PageStateID:   "page_incidents",
			StepIndex:     1,
			ActionType:    registry.StepClick,
			TargetName:    "事件中心",
			ResultSummary: "✅ Clicked element (1).",
			IsBranchNoise: true,
		},
		{
			ID:            "pseudo_navigation_failure",
			PageStateID:   "page_service",
			StepIndex:     2,
			ActionType:    registry.StepClick,
			TargetName:    "服务管理",
			ResultSummary: "❌ Failed to click element: Error: DOM tree not indexed yet. Can not perform actions on elements.",
		},
		{
			ID:            "fill_service_name",
			PageStateID:   "page_service",
			StepIndex:     3,
			ActionType:    registry.StepFill,
			TargetName:    "查询服务",
			ValueTemplate: "{{service_name}}",
			ResultSummary: "✅ Input text (checkout-api) into element (5).",
		},
		{
			ID:            "query_service",
			PageStateID:   "page_service",
			StepIndex:     4,
			ActionType:    registry.StepClick,
			TargetName:    "查询服务",
			ResultSummary: "✅ Clicked element (6).",
		},
	}

	experience, err := service.UpsertExperienceFromTaskRun(ctx, run)
	if err != nil {
		t.Fatalf("UpsertExperienceFromTaskRun failed: %v", err)
	}
	if len(experience.StepsSummary) != 2 {
		t.Fatalf("expected only reusable successful steps, got %#v", experience.StepsSummary)
	}
	for _, step := range experience.StepsSummary {
		if step.TargetName == "事件中心" || step.TargetName == "服务管理" {
			t.Fatalf("unexpected noisy or failed step in summary: %#v", experience.StepsSummary)
		}
	}
	if len(experience.Variables) != 1 || experience.Variables[0].Name != "service_name" {
		t.Fatalf("expected variables from reusable steps only, got %#v", experience.Variables)
	}
}

func TestExperienceServiceKeepsBestPathInsteadOfLatestLongerPath(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewExperienceService(repo)
	shortRun := sampleSuccessfulTaskRun()
	longRun := sampleSuccessfulTaskRun()
	longRun.ID = "task_run_service_status_long_path"
	longRun.OriginalPath = []string{"page_service_list", "page_service_search", "page_service_detail"}
	longRun.OptimizedPath = []string{"page_service_list", "page_service_search", "page_service_detail"}

	first, err := service.UpsertExperienceFromTaskRun(ctx, shortRun)
	if err != nil {
		t.Fatalf("UpsertExperienceFromTaskRun short failed: %v", err)
	}
	second, err := service.UpsertExperienceFromTaskRun(ctx, longRun)
	if err != nil {
		t.Fatalf("UpsertExperienceFromTaskRun long failed: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("same intent and target should merge into one experience, got %q and %q", first.ID, second.ID)
	}
	if strings.Join(second.BestPath, " -> ") != "page_service_list -> page_service_detail" {
		t.Fatalf("best path should remain the shorter successful path, got %#v", second.BestPath)
	}
	if strings.Join(second.OptimizedPath, " -> ") != strings.Join(second.BestPath, " -> ") {
		t.Fatalf("optimized path should expose selected best path, got optimized=%#v best=%#v", second.OptimizedPath, second.BestPath)
	}
	if len(second.AlternativePaths) != 2 {
		t.Fatalf("expected both path candidates to be retained, got %#v", second.AlternativePaths)
	}
	if second.AverageStepCount <= 0 || second.Confidence <= 0 {
		t.Fatalf("expected aggregate quality metrics, got %#v", second)
	}
}

func TestExperienceServiceSplitsSameTaskWithDifferentTargets(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewExperienceService(repo)
	searchRun := sampleSuccessfulTaskRun()
	billingRun := sampleSuccessfulTaskRun()
	billingRun.ID = "task_run_service_status_billing_target"
	billingRun.ActionSteps[0].TargetName = "账单中心入口"
	billingRun.ActionSteps[0].ActionType = registry.StepClick
	billingRun.ActionSteps[0].ValueTemplate = ""

	searchExperience, err := service.UpsertExperienceFromTaskRun(ctx, searchRun)
	if err != nil {
		t.Fatalf("UpsertExperienceFromTaskRun search failed: %v", err)
	}
	billingExperience, err := service.UpsertExperienceFromTaskRun(ctx, billingRun)
	if err != nil {
		t.Fatalf("UpsertExperienceFromTaskRun billing failed: %v", err)
	}
	if searchExperience.ID == billingExperience.ID {
		t.Fatalf("different key targets should create separate experiences: %#v", searchExperience)
	}
	if searchExperience.TargetSignature == billingExperience.TargetSignature {
		t.Fatalf("target signatures should differ: %#v %#v", searchExperience, billingExperience)
	}
}

func TestExperienceServiceSplitsSameTargetOnDifferentSurfaces(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewExperienceService(repo)
	modalRun := sampleSuccessfulTaskRun()
	modalRun.ActionSteps[0].SurfaceID = "surface_edit_modal"
	drawerRun := sampleSuccessfulTaskRun()
	drawerRun.ID = "task_run_service_status_drawer"
	drawerRun.ActionSteps[0].SurfaceID = "surface_filter_drawer"

	modalExperience, err := service.UpsertExperienceFromTaskRun(ctx, modalRun)
	if err != nil {
		t.Fatalf("UpsertExperienceFromTaskRun modal failed: %v", err)
	}
	drawerExperience, err := service.UpsertExperienceFromTaskRun(ctx, drawerRun)
	if err != nil {
		t.Fatalf("UpsertExperienceFromTaskRun drawer failed: %v", err)
	}
	if modalExperience.ID == drawerExperience.ID {
		t.Fatalf("same target in different surfaces should not merge: %#v", modalExperience)
	}
	if modalExperience.StepsSummary[0].SurfaceID != "surface_edit_modal" || drawerExperience.StepsSummary[0].SurfaceID != "surface_filter_drawer" {
		t.Fatalf("surface ids should be preserved in reusable step summaries: %#v %#v", modalExperience.StepsSummary, drawerExperience.StepsSummary)
	}
}

func TestExperienceServiceRecordsFailureMemory(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewExperienceService(repo)
	run := sampleSuccessfulTaskRun()
	run.Status = registry.TaskRunFailed
	run.Summary = "搜索框不可用。"

	failure, err := service.RecordFailureMemory(ctx, run)
	if err != nil {
		t.Fatalf("RecordFailureMemory failed: %v", err)
	}
	if failure.FailureSummary == "" || failure.PageStateID != "page_service_list" {
		t.Fatalf("unexpected failure memory: %#v", failure)
	}
	failures, err := repo.SearchFailureMemories(ctx, registry.FailureMemorySearchQuery{ProjectID: "default", Site: "ops.example.com", PageStateID: "page_service_list"})
	if err != nil {
		t.Fatalf("SearchFailureMemories failed: %v", err)
	}
	if len(failures) != 1 {
		t.Fatalf("expected one failure memory, got %#v", failures)
	}
}

func TestExperienceServiceMergesRepeatedFailureMemory(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewExperienceService(repo)
	run := sampleSuccessfulTaskRun()
	run.Status = registry.TaskRunFailed
	run.Summary = "搜索框不可用。"

	first, err := service.RecordFailureMemory(ctx, run)
	if err != nil {
		t.Fatalf("RecordFailureMemory first failed: %v", err)
	}
	run.ID = "task_run_service_status_failed_again"
	second, err := service.RecordFailureMemory(ctx, run)
	if err != nil {
		t.Fatalf("RecordFailureMemory second failed: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected repeated failure to reuse id, got %q and %q", first.ID, second.ID)
	}
	failures, err := repo.SearchFailureMemories(ctx, registry.FailureMemorySearchQuery{ProjectID: "default", Site: "ops.example.com", PageStateID: "page_service_list"})
	if err != nil {
		t.Fatalf("SearchFailureMemories failed: %v", err)
	}
	if len(failures) != 1 || failures[0].OccurrenceCount != 2 {
		t.Fatalf("expected one merged failure memory, got %#v", failures)
	}
}

func TestExperienceServiceFailureLowersExperienceRankingSignal(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewExperienceService(repo)
	run := sampleSuccessfulTaskRun()
	if _, err := service.UpsertExperienceFromTaskRun(ctx, run); err != nil {
		t.Fatalf("UpsertExperienceFromTaskRun failed: %v", err)
	}
	before, err := repo.SearchExperienceMemories(ctx, registry.ExperienceMemorySearchQuery{
		ProjectID:      "default",
		Site:           "ops.example.com",
		StartPageState: "page_service_list",
		Task:           "查询服务状态",
		Limit:          1,
	})
	if err != nil || len(before) != 1 {
		t.Fatalf("expected one experience before failure, got %#v, %v", before, err)
	}

	failedRun := run
	failedRun.ID = "task_run_service_status_failed"
	failedRun.Status = registry.TaskRunFailed
	failedRun.Summary = "Task failed because the search control disappeared."
	if _, err := service.RecordFailureMemory(ctx, failedRun); err != nil {
		t.Fatalf("RecordFailureMemory failed: %v", err)
	}
	after, err := repo.SearchExperienceMemories(ctx, registry.ExperienceMemorySearchQuery{
		ProjectID:      "default",
		Site:           "ops.example.com",
		StartPageState: "page_service_list",
		Task:           "查询服务状态",
		Limit:          1,
	})
	if err != nil || len(after) != 1 {
		t.Fatalf("expected one experience after failure, got %#v, %v", after, err)
	}
	if after[0].FailureCount != 1 {
		t.Fatalf("expected failure count to increase, got %#v", after[0])
	}
	if after[0].Score >= before[0].Score {
		t.Fatalf("expected failure to lower ranking score, before %.2f after %.2f", before[0].Score, after[0].Score)
	}
}

func sampleSuccessfulTaskRun() registry.TaskRun {
	return registry.TaskRun{
		ID:            "task_run_service_status",
		ProjectID:     "default",
		Site:          "ops.example.com",
		TaskTemplate:  "查询 {{service_name}} 服务状态",
		Summary:       "查询服务状态。",
		OriginalPath:  []string{"page_service_list", "page_service_detail"},
		OptimizedPath: []string{"page_service_list", "page_service_detail"},
		Status:        registry.TaskRunSuccess,
		ActionSteps: []registry.ActionStep{
			{
				ID:               "step_search",
				PageStateID:      "page_service_list",
				StepIndex:        1,
				ActionType:       registry.StepFill,
				TargetName:       "服务名称搜索框",
				ValueTemplate:    "{{service_name}}",
				ReasoningSummary: "按服务名定位目标服务。",
				ResultSummary:    "服务列表过滤到目标服务。",
			},
		},
	}
}
