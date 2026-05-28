package eval

import (
	"path/filepath"
	"testing"
)

func TestLoadCasesFromGitHubIssueSearchFixture(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "eval", "github_issue_search.json")

	cases, err := LoadCases(path)
	if err != nil {
		t.Fatalf("LoadCases failed: %v", err)
	}
	if len(cases) != 1 {
		t.Fatalf("expected one eval case, got %#v", cases)
	}

	evalCase := cases[0]
	if evalCase.RecordedTask == "" {
		t.Fatal("expected recordedTask")
	}
	if evalCase.ReplayTask == "" {
		t.Fatal("expected replayTask")
	}
	if evalCase.CurrentURL != "https://github.com/microsoft/playwright" {
		t.Fatalf("unexpected currentUrl: %q", evalCase.CurrentURL)
	}
	if evalCase.ExpectedWorkflowID != "wf_github_issue_search" {
		t.Fatalf("unexpected expectedWorkflowId: %q", evalCase.ExpectedWorkflowID)
	}
	if evalCase.ExpectedBindings["repo"] != "microsoft/playwright" {
		t.Fatalf("expected repo binding, got %#v", evalCase.ExpectedBindings)
	}
	if evalCase.ExpectedBindings["query"] != "timeout" {
		t.Fatalf("expected query binding, got %#v", evalCase.ExpectedBindings)
	}
	if evalCase.ExpectedStartPageState != "github_repo_home" {
		t.Fatalf("unexpected expectedStartPageState: %q", evalCase.ExpectedStartPageState)
	}
	if evalCase.ExpectedFinalPageState != "github_issues_search_results" {
		t.Fatalf("unexpected expectedFinalPageState: %q", evalCase.ExpectedFinalPageState)
	}
}

func TestRunnerCalculatesMetricsFromSuppliedPredictions(t *testing.T) {
	runner := Runner{
		Cases: []Case{
			{
				ID:                     "github_issue_search",
				ExpectedWorkflowID:     "wf_github_issue_search",
				ExpectedBindings:       map[string]string{"repo": "microsoft/playwright", "query": "timeout"},
				ExpectedStartPageState: "github_repo_home",
				ExpectedFinalPageState: "github_issues_search_results",
			},
		},
		Options: Options{K: 3},
	}

	metrics, err := runner.Evaluate([]Prediction{
		{
			CaseID:             "github_issue_search",
			RankedWorkflowIDs:  []string{"wf_github_issue_search"},
			SelectedWorkflowID: "wf_github_issue_search",
			Bindings:           map[string]string{"repo": "microsoft/playwright", "query": "timeout"},
			StartPageState:     "github_repo_home",
			FinalPageState:     "github_issues_search_results",
			ReplayCompleted:    true,
		},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	assertFloat(t, "recall@K", metrics.RecallAtK, 1)
	assertFloat(t, "precision@1", metrics.PrecisionAt1, 1)
	assertFloat(t, "variable binding accuracy", metrics.VariableBindingAccuracy, 1)
	assertFloat(t, "page state detection accuracy", metrics.PageStateDetectionAccuracy, 1)
	assertFloat(t, "replay completion rate", metrics.ReplayCompletionRate, 1)
}
