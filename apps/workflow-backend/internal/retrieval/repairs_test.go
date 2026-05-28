package retrieval

import (
	"testing"

	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestRepairSearchMatchesLocatorFailure(t *testing.T) {
	service := NewRepairService(fakeRepairSearcher{
		hits: []RepairHit{
			{
				PatchID:             "patch_github_searchbox",
				WorkflowID:          "wf_github_issue_search",
				WorkflowVersion:     3,
				ChunkID:             "search_issues",
				StepID:              "fill_query",
				Score:               0.91,
				Site:                "github.com",
				FailureType:         "locator_not_found",
				AppliesToPageStates: []string{"github_issues_list"},
				RiskLevel:           registry.RiskReadOnly,
			},
		},
	})

	result, ok := service.Search(RepairRequest{
		ProjectID:        "default",
		WorkflowID:       "wf_github_issue_search",
		WorkflowVersion:  3,
		ChunkID:          "search_issues",
		StepID:           "fill_query",
		Site:             "github.com",
		FailureType:      "locator_not_found",
		CurrentPageState: "github_issues_list",
		RiskPolicy:       defaultRiskPolicy(),
	})

	if !ok {
		t.Fatal("expected repair patch match")
	}
	if result.PatchID != "patch_github_searchbox" {
		t.Fatalf("unexpected repair patch: %#v", result)
	}
}

func TestRepairSearchRejectsPageStateMismatch(t *testing.T) {
	service := NewRepairService(fakeRepairSearcher{
		hits: []RepairHit{
			{
				PatchID:             "patch_other",
				WorkflowID:          "wf_github_issue_search",
				WorkflowVersion:     3,
				Score:               0.95,
				Site:                "github.com",
				FailureType:         "locator_not_found",
				AppliesToPageStates: []string{"github_repo_home"},
				RiskLevel:           registry.RiskReadOnly,
			},
		},
	})

	_, ok := service.Search(RepairRequest{
		WorkflowID:       "wf_github_issue_search",
		WorkflowVersion:  3,
		Site:             "github.com",
		FailureType:      "locator_not_found",
		CurrentPageState: "github_issues_list",
		RiskPolicy:       defaultRiskPolicy(),
	})

	if ok {
		t.Fatal("expected page-state mismatch rejection")
	}
}

type fakeRepairSearcher struct {
	hits []RepairHit
}

func (searcher fakeRepairSearcher) SearchRepairs(RepairRequest) ([]RepairHit, error) {
	return searcher.hits, nil
}
