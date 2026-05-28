package replay

import (
	"context"
	"testing"

	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestServiceRunsWorkflowChunks(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	recipe := sampleReplayWorkflow()
	if err := repo.SaveWorkflow(t.Context(), recipe); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}
	runner := &fakeRunner{}
	service := NewService(repo, runner)

	run, err := service.Start(context.Background(), StartRequest{
		ProjectID:  "default",
		WorkflowID: recipe.ID,
		Version:    recipe.Version,
		Task:       "search timeout",
		URL:        "https://github.com/microsoft/playwright",
		Bindings:   map[string]string{"repo": "microsoft/playwright", "query": "timeout"},
	})

	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if run.Result != ResultSucceeded {
		t.Fatalf("expected succeeded run, got %#v", run)
	}
	if len(runner.executed) != 1 || runner.executed[0] != "open_issues" {
		t.Fatalf("expected chunk execution, got %#v", runner.executed)
	}
}

func TestServiceRecordsFailure(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	recipe := sampleReplayWorkflow()
	if err := repo.SaveWorkflow(t.Context(), recipe); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}
	service := NewService(repo, &fakeRunner{failChunkID: "open_issues"})

	run, err := service.Start(context.Background(), StartRequest{
		ProjectID:  "default",
		WorkflowID: recipe.ID,
		Version:    recipe.Version,
		Task:       "search timeout",
		URL:        "https://github.com/microsoft/playwright",
	})

	if err != nil {
		t.Fatalf("Start should return failed run without transport error: %v", err)
	}
	if run.Result != ResultFailed || run.FailedChunkID != "open_issues" {
		t.Fatalf("expected failed run, got %#v", run)
	}
}

func TestServicePersistsStructuredRunLogsAndSelectorStats(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	recipe := sampleReplayWorkflow()
	recipe.Chunks[0].FromPageState = "github_repo_home"
	recipe.Chunks[0].ToPageState = "github_issues_search"
	recipe.Chunks[0].PostconditionText = "Issues"
	if err := repo.SaveWorkflow(t.Context(), recipe); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}
	service := NewService(repo, &fakeRunner{})

	run, err := service.Start(context.Background(), StartRequest{
		ProjectID:  "default",
		WorkflowID: recipe.ID,
		Version:    recipe.Version,
		Task:       "search token sk-secret",
		URL:        "https://github.com/microsoft/playwright",
		Bindings:   map[string]string{"query": "sk-secret"},
	})

	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	persisted, err := repo.GetRun(t.Context(), run.ID)
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}
	if persisted.Variables["query"] != "[redacted]" {
		t.Fatalf("expected run variables to be redacted, got %#v", persisted.Variables)
	}
	if len(persisted.Logs) < 2 {
		t.Fatalf("expected chunk start/end logs, got %#v", persisted.Logs)
	}
	stats, err := repo.ListSelectorStats(t.Context(), recipe.ID, recipe.Version)
	if err != nil {
		t.Fatalf("ListSelectorStats failed: %v", err)
	}
	if len(stats) == 0 || stats[0].SuccessCount == 0 {
		t.Fatalf("expected selector stats from successful run, got %#v", stats)
	}
}

func TestServiceRecordsPostconditionFailure(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	recipe := sampleReplayWorkflow()
	if err := repo.SaveWorkflow(t.Context(), recipe); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}
	service := NewService(repo, &fakeDetailedRunner{
		execution: RunExecution{
			FailedChunkID: "open_issues",
			FailureReason: "postcondition_text_missing: Issues",
			Logs: []registry.WorkflowRunLog{
				{Event: "postcondition_failed", ChunkID: "open_issues", Reason: "postcondition_text_missing"},
			},
		},
		err: ErrChunkFailed,
	})

	run, err := service.Start(context.Background(), StartRequest{
		ProjectID:  "default",
		WorkflowID: recipe.ID,
		Version:    recipe.Version,
		Task:       "search timeout",
		URL:        "https://github.com/microsoft/playwright",
	})

	if err != nil {
		t.Fatalf("Start should return failed run without transport error: %v", err)
	}
	if run.Result != ResultFailed || run.FailedChunkID != "open_issues" {
		t.Fatalf("expected postcondition failure, got %#v", run)
	}
	persisted, err := repo.GetRun(t.Context(), run.ID)
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}
	if persisted.FallbackReason != "postcondition_text_missing: Issues" {
		t.Fatalf("expected structured failure reason, got %#v", persisted)
	}
}

func TestServicePassesActiveInterruptHandlersAndWorkflowsToDetailedRunner(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	recipe := sampleReplayWorkflow()
	recipe.Chunks[0].FromPageState = "github_repo_home"
	if err := repo.SaveWorkflow(t.Context(), recipe); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}
	handlerWorkflow := sampleReplayWorkflow()
	handlerWorkflow.ID = "wf_close_dialog"
	handlerWorkflow.Version = 1
	handlerWorkflow.RiskLevel = registry.RiskReadOnly
	handlerWorkflow.Chunks = []registry.WorkflowChunk{
		{
			ID:        "close_dialog",
			Name:      "Close Dialog",
			RiskLevel: registry.RiskReadOnly,
			Steps: []registry.WorkflowStep{
				{
					ID:        "click_close",
					Type:      registry.StepClick,
					RiskLevel: registry.RiskReadOnly,
					Target: registry.StepTarget{
						Primary: registry.TargetCandidate{Strategy: registry.TargetRole, Role: "button", Name: "Continue workflow"},
					},
				},
			},
		},
	}
	if err := repo.SaveWorkflow(t.Context(), handlerWorkflow); err != nil {
		t.Fatalf("SaveWorkflow handler failed: %v", err)
	}
	if err := repo.SaveInterruptHandler(t.Context(), registry.InterruptHandler{
		ID:                  "ih_dialog",
		ProjectID:           "default",
		Site:                "github.com",
		Status:              registry.StatusActive,
		AppliesToPageStates: []string{"github_repo_home"},
		InterruptType:       "modal",
		RequiredText:        []string{"Blocking dialog"},
		TargetControls:      []registry.ControlSignature{{Role: "button", Name: "Continue workflow"}},
		RiskLevel:           registry.RiskReadOnly,
		WorkflowID:          handlerWorkflow.ID,
		Version:             handlerWorkflow.Version,
	}); err != nil {
		t.Fatalf("SaveInterruptHandler failed: %v", err)
	}
	if err := repo.SaveInterruptHandler(t.Context(), registry.InterruptHandler{
		ID:                  "ih_unsafe",
		ProjectID:           "default",
		Site:                "github.com",
		Status:              registry.StatusActive,
		AppliesToPageStates: []string{"github_repo_home"},
		InterruptType:       "destructive",
		RiskLevel:           registry.RiskDestructive,
		WorkflowID:          handlerWorkflow.ID,
		Version:             handlerWorkflow.Version,
	}); err != nil {
		t.Fatalf("SaveInterruptHandler unsafe failed: %v", err)
	}
	runner := &fakeDetailedRunner{}
	service := NewService(repo, runner)

	run, err := service.Start(context.Background(), StartRequest{
		ProjectID:  "default",
		WorkflowID: recipe.ID,
		Version:    recipe.Version,
		Task:       "search timeout",
		URL:        "https://github.com/microsoft/playwright",
	})

	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if run.Result != ResultSucceeded {
		t.Fatalf("expected succeeded run, got %#v", run)
	}
	if len(runner.requests) != 1 {
		t.Fatalf("expected one detailed runner request, got %d", len(runner.requests))
	}
	request := runner.requests[0]
	if len(request.InterruptHandlers) != 1 || request.InterruptHandlers[0].ID != "ih_dialog" {
		t.Fatalf("expected only safe active interrupt handler, got %#v", request.InterruptHandlers)
	}
	if request.InterruptWorkflows["wf_close_dialog"].ID != "wf_close_dialog" {
		t.Fatalf("expected handler workflow payload, got %#v", request.InterruptWorkflows)
	}
}

type fakeRunner struct {
	failChunkID string
	executed    []string
}

func (runner *fakeRunner) RunChunk(_ context.Context, request ChunkRunRequest) error {
	runner.executed = append(runner.executed, request.Chunk.ID)
	if runner.failChunkID == request.Chunk.ID {
		return ErrChunkFailed
	}
	return nil
}

type fakeDetailedRunner struct {
	execution RunExecution
	err       error
	requests  []WorkflowRunRequest
}

func (runner *fakeDetailedRunner) RunChunk(context.Context, ChunkRunRequest) error {
	return nil
}

func (runner *fakeDetailedRunner) RunWorkflowDetailed(_ context.Context, request WorkflowRunRequest) (RunExecution, error) {
	runner.requests = append(runner.requests, request)
	return runner.execution, runner.err
}

func sampleReplayWorkflow() registry.WorkflowRecipe {
	return registry.WorkflowRecipe{
		ID:         "wf_github_issue_search",
		Version:    3,
		ProjectID:  "default",
		Status:     registry.StatusActive,
		Searchable: true,
		Site:       "github.com",
		Intent:     "Search issues",
		RiskLevel:  registry.RiskReadOrSearch,
		Variables: []registry.Variable{
			{Name: "query", Type: registry.VariableString, Required: true, Source: registry.VariableSourceTask},
		},
		Chunks: []registry.WorkflowChunk{
			{ID: "open_issues", Name: "Open Issues", RiskLevel: registry.RiskReadOrSearch, Steps: []registry.WorkflowStep{{ID: "click", Type: registry.StepClick}}},
		},
	}
}
