package selector

import (
	"context"
	"testing"
)

func TestServiceRecordsRunsAndSelectorStats(t *testing.T) {
	service := NewService(NewMemoryRepository())
	ctx := context.Background()

	run, err := service.RecordRun(ctx, WorkflowRun{
		ID:        "run_1",
		ProjectID: "default",
		Task:      "search docs",
		URL:       "https://example.com",
		Result:    RunResultSuccess,
	})
	if err != nil {
		t.Fatalf("RecordRun returned error: %v", err)
	}
	if run.ID != "run_1" {
		t.Fatalf("unexpected run: %#v", run)
	}

	if err := service.RecordSelectorStats(ctx, "run_1", []SelectorStatUpdate{
		{WorkflowID: "wf_1", WorkflowVersion: 1, StepID: "step_1", Strategy: "role", Selector: "button:Search", Success: true},
		{WorkflowID: "wf_1", WorkflowVersion: 1, StepID: "step_1", Strategy: "role", Selector: "button:Search", Success: false, FailureReason: "missing"},
	}); err != nil {
		t.Fatalf("RecordSelectorStats returned error: %v", err)
	}

	stats, err := service.Stats(ctx, "wf_1", 1)
	if err != nil {
		t.Fatalf("Stats returned error: %v", err)
	}
	if len(stats) != 1 || stats[0].SuccessCount != 1 || stats[0].FailCount != 1 || stats[0].LastFailureReason != "missing" {
		t.Fatalf("unexpected stats: %#v", stats)
	}
}
