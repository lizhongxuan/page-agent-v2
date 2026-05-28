package registry

import (
	"context"
	"testing"
)

func TestRepairPatchServiceCreatesPendingCandidateAndApprovesWithOutboxEvent(t *testing.T) {
	ctx := context.Background()
	repo, err := NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewRepairPatchService(repo)

	candidate, err := service.CreateCandidate(ctx, sampleRepairPatchCandidate())
	if err != nil {
		t.Fatalf("CreateCandidate failed: %v", err)
	}
	if candidate.Status != StatusPendingReview {
		t.Fatalf("expected pending review candidate, got %#v", candidate)
	}
	if active, err := repo.ListActiveRepairPatches(ctx, "default"); err != nil || len(active) != 0 {
		t.Fatalf("pending repair patch must not be active/searchable, active=%#v err=%v", active, err)
	}

	approved, err := service.Approve(ctx, candidate.ID)
	if err != nil {
		t.Fatalf("Approve failed: %v", err)
	}
	if approved.Status != StatusActive {
		t.Fatalf("expected active patch, got %#v", approved)
	}
	active, err := repo.ListActiveRepairPatches(ctx, "default")
	if err != nil {
		t.Fatalf("ListActiveRepairPatches failed: %v", err)
	}
	if len(active) != 1 || active[0].ID != candidate.ID {
		t.Fatalf("expected approved patch to be active, got %#v", active)
	}
	events, err := repo.ListOutboxEvents(ctx, StatusPendingReview)
	if err != nil {
		t.Fatalf("ListOutboxEvents failed: %v", err)
	}
	if len(events) != 1 || events[0].Type != "repair_patch_approved" {
		t.Fatalf("expected repair_patch_approved outbox event, got %#v", events)
	}
}

func TestRepairPatchServiceRejectsUnsafeOrIncompleteCandidates(t *testing.T) {
	repo, err := NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewRepairPatchService(repo)
	patch := sampleRepairPatchCandidate()
	patch.RiskLevel = RiskDestructive

	if _, err := service.CreateCandidate(context.Background(), patch); err == nil {
		t.Fatal("expected destructive repair patch candidate to be rejected")
	}

	patch = sampleRepairPatchCandidate()
	patch.NewTarget = StepTarget{}
	patch.NewTargetSummary = ""
	if _, err := service.CreateCandidate(context.Background(), patch); err == nil {
		t.Fatal("expected repair patch without new target to be rejected")
	}
}

func sampleRepairPatchCandidate() RepairPatch {
	return RepairPatch{
		ProjectID:           "default",
		WorkflowID:          "wf_github_issue_search",
		WorkflowVersion:     3,
		ChunkID:             "search_issues",
		StepID:              "fill_query",
		Site:                "github.com",
		FailureType:         "locator_not_found",
		FailureSignature:    "placeholder Search all issues was not visible",
		OldTarget:           "placeholder Search all issues",
		NewTargetSummary:    "Search issues query textbox",
		AppliesToPageStates: []string{"github_issues_list"},
		RiskLevel:           RiskReadOnly,
		NewTarget: StepTarget{
			Primary: TargetCandidate{
				Strategy: TargetCSS,
				Value:    "#new-query",
			},
		},
	}
}
