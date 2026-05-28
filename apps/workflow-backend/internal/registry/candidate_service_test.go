package registry

import (
	"context"
	"testing"
)

func TestCandidateServiceCreatesPendingCandidate(t *testing.T) {
	ctx := context.Background()
	repo, err := NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewCandidateService(repo)

	candidate, err := service.CreateFromSession(ctx, RecordedSession{
		ProjectID: "default",
		Source:    "user_demo",
		Task:      "Search GitHub issues",
		StartURL:  "https://github.com/alibaba/page-agent",
		Recipe:    sampleWorkflowRecipe(),
	})

	if err != nil {
		t.Fatalf("CreateFromSession failed: %v", err)
	}
	if candidate.Status != StatusPendingReview || candidate.Searchable {
		t.Fatalf("candidate should be pending and non-searchable: %#v", candidate)
	}

	workflows, err := repo.ListActiveWorkflows(ctx, "default")
	if err != nil {
		t.Fatalf("ListActiveWorkflows failed: %v", err)
	}
	if len(workflows) != 0 {
		t.Fatalf("pending candidate should not create active workflow: %#v", workflows)
	}
}

func TestCandidateServiceApproveCreatesActiveWorkflowAndOutboxEvent(t *testing.T) {
	ctx := context.Background()
	repo, err := NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewCandidateService(repo)
	candidate, err := service.CreateFromSession(ctx, RecordedSession{
		ProjectID: "default",
		Source:    "user_demo",
		Task:      "Search GitHub issues",
		StartURL:  "https://github.com/alibaba/page-agent",
		Recipe:    sampleWorkflowRecipe(),
	})
	if err != nil {
		t.Fatalf("CreateFromSession failed: %v", err)
	}

	approved, err := service.Approve(ctx, candidate.ID)

	if err != nil {
		t.Fatalf("Approve failed: %v", err)
	}
	if approved.Status != StatusActive || !approved.Searchable {
		t.Fatalf("approved candidate should be active and searchable: %#v", approved)
	}
	workflow, err := repo.GetWorkflow(ctx, approved.RecipeDraft.ID)
	if err != nil {
		t.Fatalf("GetWorkflow failed after approve: %v", err)
	}
	if workflow.Status != StatusActive || !workflow.Searchable {
		t.Fatalf("workflow should be active and searchable: %#v", workflow)
	}
	events, err := repo.ListOutboxEvents(ctx, StatusPendingReview)
	if err != nil {
		t.Fatalf("ListOutboxEvents failed: %v", err)
	}
	if len(events) != 1 || events[0].Type != "workflow_approved" {
		t.Fatalf("expected workflow_approved event, got %#v", events)
	}
}

func TestCandidateServiceRejectDoesNotIndex(t *testing.T) {
	ctx := context.Background()
	repo, err := NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewCandidateService(repo)
	candidate, err := service.CreateFromSession(ctx, RecordedSession{
		ProjectID: "default",
		Source:    "user_demo",
		Task:      "Search GitHub issues",
		StartURL:  "https://github.com/alibaba/page-agent",
		Recipe:    sampleWorkflowRecipe(),
	})
	if err != nil {
		t.Fatalf("CreateFromSession failed: %v", err)
	}

	rejected, err := service.Reject(ctx, candidate.ID)

	if err != nil {
		t.Fatalf("Reject failed: %v", err)
	}
	if rejected.Status != StatusRejected || rejected.Searchable {
		t.Fatalf("rejected candidate should not be searchable: %#v", rejected)
	}
	events, err := repo.ListOutboxEvents(ctx, StatusPendingReview)
	if err != nil {
		t.Fatalf("ListOutboxEvents failed: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("rejected candidate should not emit index event: %#v", events)
	}
}
