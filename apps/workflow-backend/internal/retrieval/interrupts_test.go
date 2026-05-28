package retrieval

import (
	"testing"

	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestInterruptSearchMatchesKnownDialog(t *testing.T) {
	service := NewInterruptService(fakeInterruptSearcher{
		hits: []InterruptHit{
			{
				HandlerID:           "ih_github_new_feature_dialog",
				WorkflowID:          "wf_close_dialog",
				Version:             1,
				Score:               0.93,
				Site:                "github.com",
				RiskLevel:           registry.RiskReadOnly,
				AppliesToPageStates: []string{"github_issues_list"},
				RequiredText:        []string{"New feature"},
				TargetControls:      []Control{{Role: "button", Name: "Got it"}},
			},
		},
	})

	result, ok := service.Search(InterruptRequest{
		ProjectID:        "default",
		Site:             "github.com",
		CurrentPageState: "github_issues_list",
		Observation: PageObservation{
			VisibleText: []string{"New feature", "Got it"},
			Controls:    []Control{{Role: "button", Name: "Got it"}},
		},
		RiskPolicy: defaultRiskPolicy(),
	})

	if !ok {
		t.Fatal("expected interrupt handler match")
	}
	if result.HandlerID != "ih_github_new_feature_dialog" {
		t.Fatalf("unexpected handler: %#v", result)
	}
}

func TestInterruptSearchRejectsBlockedRisk(t *testing.T) {
	service := NewInterruptService(fakeInterruptSearcher{
		hits: []InterruptHit{
			{
				HandlerID:           "ih_bad",
				Score:               0.99,
				Site:                "github.com",
				RiskLevel:           registry.RiskDestructive,
				AppliesToPageStates: []string{"github_issues_list"},
				RequiredText:        []string{"New feature"},
			},
		},
	})

	_, ok := service.Search(InterruptRequest{
		Site:             "github.com",
		CurrentPageState: "github_issues_list",
		Observation:      PageObservation{VisibleText: []string{"New feature"}},
		RiskPolicy:       defaultRiskPolicy(),
	})

	if ok {
		t.Fatal("expected blocked risk rejection")
	}
}

type fakeInterruptSearcher struct {
	hits []InterruptHit
}

func (searcher fakeInterruptSearcher) SearchInterrupts(InterruptRequest) ([]InterruptHit, error) {
	return searcher.hits, nil
}
