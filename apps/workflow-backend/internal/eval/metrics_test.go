package eval

import "testing"

func TestCalculateMetricsForWorkflowRetrievalPredictions(t *testing.T) {
	cases := []Case{
		{
			ID:                     "hit_at_2",
			ExpectedWorkflowID:     "wf_github_issue_search",
			ExpectedBindings:       map[string]string{"repo": "microsoft/playwright", "query": "timeout"},
			ExpectedStartPageState: "github_repo_home",
			ExpectedFinalPageState: "github_issues_search_results",
		},
		{
			ID:                     "hit_at_1",
			ExpectedWorkflowID:     "wf_github_issue_search",
			ExpectedBindings:       map[string]string{"repo": "openai/codex", "query": "workflow"},
			ExpectedStartPageState: "github_repo_home",
			ExpectedFinalPageState: "github_issues_search_results",
		},
		{
			ID:                 "negative",
			ExpectedWorkflowID: "",
		},
	}
	predictions := []Prediction{
		{
			CaseID:              "hit_at_2",
			RankedWorkflowIDs:   []string{"wf_other", "wf_github_issue_search"},
			SelectedWorkflowID:  "wf_other",
			Bindings:            map[string]string{"repo": "microsoft/playwright", "query": "timeout"},
			StartPageState:      "github_repo_home",
			FinalPageState:      "github_issues_search_results",
			InterruptHandlerHit: true,
			RepairPatchReused:   true,
			ReplayCompleted:     true,
			FallbackToPageAgent: false,
		},
		{
			CaseID:              "hit_at_1",
			RankedWorkflowIDs:   []string{"wf_github_issue_search", "wf_other"},
			SelectedWorkflowID:  "wf_github_issue_search",
			Bindings:            map[string]string{"repo": "openai/codex", "query": "wrong"},
			StartPageState:      "github_repo_home",
			FinalPageState:      "wrong_final",
			ReplayCompleted:     false,
			FallbackToPageAgent: true,
		},
		{
			CaseID:             "negative",
			RankedWorkflowIDs:  []string{"wf_false_positive"},
			SelectedWorkflowID: "wf_false_positive",
		},
	}

	metrics, err := CalculateMetrics(cases, predictions, Options{K: 2})
	if err != nil {
		t.Fatalf("CalculateMetrics failed: %v", err)
	}

	assertFloat(t, "recall@K", metrics.RecallAtK, 1.0)
	assertFloat(t, "MRR", metrics.MRR, 0.75)
	assertFloat(t, "precision@1", metrics.PrecisionAt1, 0.5)
	assertFloat(t, "false positive rate", metrics.FalsePositiveRate, 1.0)
	assertFloat(t, "variable binding accuracy", metrics.VariableBindingAccuracy, 0.75)
	assertFloat(t, "page state detection accuracy", metrics.PageStateDetectionAccuracy, 0.75)
	assertFloat(t, "interrupt handler hit rate", metrics.InterruptHandlerHitRate, 1.0/3.0)
	assertFloat(t, "repair patch reuse rate", metrics.RepairPatchReuseRate, 1.0/3.0)
	assertFloat(t, "replay completion rate", metrics.ReplayCompletionRate, 1.0/3.0)
	assertFloat(t, "fallback-to-PageAgent rate", metrics.FallbackToPageAgentRate, 1.0/3.0)
}

func TestCalculateMetricsRejectsUnknownPredictionCase(t *testing.T) {
	_, err := CalculateMetrics([]Case{{ID: "known"}}, []Prediction{{CaseID: "unknown"}}, Options{K: 1})
	if err == nil {
		t.Fatal("expected unknown case error")
	}
}

func assertFloat(t *testing.T, name string, got float64, want float64) {
	t.Helper()
	const tolerance = 0.000001
	if got < want-tolerance || got > want+tolerance {
		t.Fatalf("%s = %.6f, want %.6f", name, got, want)
	}
}
