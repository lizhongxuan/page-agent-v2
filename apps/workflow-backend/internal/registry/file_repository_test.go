package registry

import (
	"context"
	"path/filepath"
	"testing"
)

func TestFileRepositoryPersistsAcrossRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo, err := NewFileRepository(dir)
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	recipe := sampleWorkflowRecipe()

	if err := repo.SaveWorkflow(ctx, recipe); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}
	if err := repo.SaveVersion(ctx, WorkflowVersion{
		WorkflowID: recipe.ID,
		Version:    recipe.Version,
		Recipe:     recipe,
		Summary:    "initial version",
	}); err != nil {
		t.Fatalf("SaveVersion failed: %v", err)
	}

	restarted, err := NewFileRepository(dir)
	if err != nil {
		t.Fatalf("restart NewFileRepository failed: %v", err)
	}
	got, err := restarted.GetWorkflow(ctx, recipe.ID)
	if err != nil {
		t.Fatalf("GetWorkflow after restart failed: %v", err)
	}
	if got.ID != recipe.ID || got.Version != recipe.Version {
		t.Fatalf("unexpected workflow after restart: %#v", got)
	}
	version, err := restarted.GetVersion(ctx, recipe.ID, recipe.Version)
	if err != nil {
		t.Fatalf("GetVersion after restart failed: %v", err)
	}
	if version.Summary != "initial version" {
		t.Fatalf("unexpected version after restart: %#v", version)
	}
}

func TestFileRepositoryListsOnlyActiveSearchableWorkflows(t *testing.T) {
	ctx := context.Background()
	repo, err := NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	active := sampleWorkflowRecipe()
	disabled := sampleWorkflowRecipe()
	disabled.ID = "wf_disabled"
	disabled.Status = StatusDisabled

	if err := repo.SaveWorkflow(ctx, active); err != nil {
		t.Fatalf("SaveWorkflow active failed: %v", err)
	}
	if err := repo.SaveWorkflow(ctx, disabled); err != nil {
		t.Fatalf("SaveWorkflow disabled failed: %v", err)
	}

	workflows, err := repo.ListActiveWorkflows(ctx, "default")
	if err != nil {
		t.Fatalf("ListActiveWorkflows failed: %v", err)
	}
	if len(workflows) != 1 || workflows[0].ID != active.ID {
		t.Fatalf("expected only active workflow, got %#v", workflows)
	}
}

func TestFileRepositoryOutboxIdempotency(t *testing.T) {
	ctx := context.Background()
	repo, err := NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	event := OutboxEvent{
		Type:           "workflow_approved",
		IdempotencyKey: "workflow_approved:wf_github_issue_search:v3",
		Payload:        map[string]any{"workflowId": "wf_github_issue_search"},
	}

	first, err := repo.AppendOutboxEvent(ctx, event)
	if err != nil {
		t.Fatalf("AppendOutboxEvent first failed: %v", err)
	}
	second, err := repo.AppendOutboxEvent(ctx, event)
	if err != nil {
		t.Fatalf("AppendOutboxEvent second failed: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("idempotent event should reuse ID, got %q and %q", first.ID, second.ID)
	}

	events, err := repo.ListOutboxEvents(ctx, StatusPendingReview)
	if err != nil {
		t.Fatalf("ListOutboxEvents failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one event, got %#v", events)
	}
}

func TestFileRepositoryPersistsRunLogsAndRedactedVariables(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo, err := NewFileRepository(dir)
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	run := WorkflowRun{
		ID:         "run_1",
		ProjectID:  "default",
		WorkflowID: "wf_github_issue_search",
		Version:    1,
		Task:       "search timeout",
		URL:        "https://github.com/microsoft/playwright",
		Variables: map[string]string{
			"query": "[redacted]",
		},
		Result: "succeeded",
		Logs: []WorkflowRunLog{
			{
				Event:     "chunk_started",
				ChunkID:   "open_issues",
				PageState: "github_repo_home",
			},
		},
	}

	if err := repo.SaveRun(ctx, run); err != nil {
		t.Fatalf("SaveRun failed: %v", err)
	}
	restarted, err := NewFileRepository(dir)
	if err != nil {
		t.Fatalf("restart NewFileRepository failed: %v", err)
	}
	got, err := restarted.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun after restart failed: %v", err)
	}
	if got.Variables["query"] != "[redacted]" {
		t.Fatalf("expected redacted variable, got %#v", got.Variables)
	}
	if len(got.Logs) != 1 || got.Logs[0].Event != "chunk_started" {
		t.Fatalf("expected persisted run log, got %#v", got.Logs)
	}
}

func TestFileRepositoryUpsertsSelectorStats(t *testing.T) {
	ctx := context.Background()
	repo, err := NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	stats := SelectorStats{
		WorkflowID:   "wf_github_issue_search",
		Version:      1,
		StepID:       "click_issues",
		Strategy:     "role",
		Selector:     "link:Issues",
		SuccessCount: 1,
	}
	if err := repo.SaveSelectorStats(ctx, stats); err != nil {
		t.Fatalf("SaveSelectorStats first failed: %v", err)
	}
	stats.SuccessCount = 2
	stats.FailCount = 1
	if err := repo.SaveSelectorStats(ctx, stats); err != nil {
		t.Fatalf("SaveSelectorStats second failed: %v", err)
	}
	got, err := repo.ListSelectorStats(ctx, "wf_github_issue_search", 1)
	if err != nil {
		t.Fatalf("ListSelectorStats failed: %v", err)
	}
	if len(got) != 1 || got[0].SuccessCount != 2 || got[0].FailCount != 1 {
		t.Fatalf("expected upserted stats, got %#v", got)
	}
}

func TestFileRepositoryPersistsInterruptHandlersAndRepairPatches(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo, err := NewFileRepository(dir)
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	handler := InterruptHandler{
		ID:            "ih_dialog",
		ProjectID:     "default",
		Site:          "github.com",
		Status:        StatusActive,
		InterruptType: "modal",
		RiskLevel:     RiskReadOnly,
		WorkflowID:    "wf_close_dialog",
		Version:       1,
	}
	patch := RepairPatch{
		ID:              "patch_search",
		ProjectID:       "default",
		Status:          StatusActive,
		WorkflowID:      "wf_github_issue_search",
		WorkflowVersion: 3,
		ChunkID:         "search_issues",
		StepID:          "fill_query",
		Site:            "github.com",
		FailureType:     "locator_not_found",
		RiskLevel:       RiskReadOnly,
	}

	if err := repo.SaveInterruptHandler(ctx, handler); err != nil {
		t.Fatalf("SaveInterruptHandler failed: %v", err)
	}
	if err := repo.SaveRepairPatch(ctx, patch); err != nil {
		t.Fatalf("SaveRepairPatch failed: %v", err)
	}
	restarted, err := NewFileRepository(dir)
	if err != nil {
		t.Fatalf("restart NewFileRepository failed: %v", err)
	}
	handlers, err := restarted.ListActiveInterruptHandlers(ctx, "default")
	if err != nil {
		t.Fatalf("ListActiveInterruptHandlers failed: %v", err)
	}
	patches, err := restarted.ListActiveRepairPatches(ctx, "default")
	if err != nil {
		t.Fatalf("ListActiveRepairPatches failed: %v", err)
	}
	if len(handlers) != 1 || handlers[0].ID != handler.ID {
		t.Fatalf("unexpected handlers: %#v", handlers)
	}
	if len(patches) != 1 || patches[0].ID != patch.ID {
		t.Fatalf("unexpected patches: %#v", patches)
	}
}

func TestNewFileRepositoryCreatesDataDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "data")

	if _, err := NewFileRepository(dir); err != nil {
		t.Fatalf("NewFileRepository should create data dir: %v", err)
	}
}
