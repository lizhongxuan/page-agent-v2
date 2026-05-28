package retrieval

import (
	"testing"
	"time"

	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestRerankScoresAndOrdersCandidates(t *testing.T) {
	candidates := []RawCandidate{
		{
			WorkflowID:     "wf_low",
			Version:        1,
			Status:         registry.StatusActive,
			Searchable:     true,
			RiskLevel:      registry.RiskReadOrSearch,
			VariableNames:  []string{"repo", "query"},
			WorkflowDense:  0.5,
			WorkflowSparse: 0.4,
			BestChunk:      0.3,
			PageState:      0.4,
			SuccessRate:    0.7,
			SelectorHealth: 0.7,
		},
		{
			WorkflowID:     "wf_high",
			Version:        1,
			Status:         registry.StatusActive,
			Searchable:     true,
			RiskLevel:      registry.RiskReadOrSearch,
			VariableNames:  []string{"repo", "query"},
			WorkflowDense:  0.9,
			WorkflowSparse: 0.8,
			BestChunk:      0.8,
			PageState:      0.9,
			SuccessRate:    0.9,
			SelectorHealth: 0.9,
		},
	}

	results := Rerank(candidates, RerankInput{
		CandidateSlots: map[string]string{"repo": "microsoft/playwright", "query": "timeout"},
		RiskPolicy:     defaultRiskPolicy(),
	})

	if len(results) != 2 {
		t.Fatalf("expected two results, got %#v", results)
	}
	if results[0].WorkflowID != "wf_high" {
		t.Fatalf("expected high candidate first, got %#v", results)
	}
	if results[0].FinalScore <= results[1].FinalScore {
		t.Fatalf("expected descending score, got %#v", results)
	}
}

func TestRerankRejectsUnbindableAndBlockedRisk(t *testing.T) {
	candidates := []RawCandidate{
		{
			WorkflowID:    "wf_unbindable",
			Version:       1,
			Status:        registry.StatusActive,
			Searchable:    true,
			RiskLevel:     registry.RiskReadOrSearch,
			VariableNames: []string{"repo", "query"},
			WorkflowDense: 0.9,
		},
		{
			WorkflowID:    "wf_destructive",
			Version:       1,
			Status:        registry.StatusActive,
			Searchable:    true,
			RiskLevel:     registry.RiskDestructive,
			VariableNames: []string{"repo"},
			WorkflowDense: 0.95,
		},
	}

	results := Rerank(candidates, RerankInput{
		CandidateSlots: map[string]string{"repo": "microsoft/playwright"},
		RiskPolicy:     defaultRiskPolicy(),
	})

	if len(results) != 0 {
		t.Fatalf("expected rejected candidates, got %#v", results)
	}
}

func TestRerankRejectsCandidatesWithOnlySiteAndBindableVariables(t *testing.T) {
	results := Rerank([]RawCandidate{
		{
			WorkflowID:     "wf_site_only",
			Version:        1,
			Status:         registry.StatusActive,
			Searchable:     true,
			Site:           "github.com",
			RiskLevel:      registry.RiskReadOrSearch,
			VariableNames:  []string{"repo"},
			StepText:       "click Issues -> click Pull requests",
			SuccessRate:    1,
			SelectorHealth: 1,
		},
	}, RerankInput{
		Task:           "随便问一下今天吃什么",
		CandidateSlots: map[string]string{"repo": "browser-use/workflow-use"},
		RiskPolicy:     defaultRiskPolicy(),
		Site:           "github.com",
	})

	if len(results) != 0 {
		t.Fatalf("expected weak site-only candidate to be rejected, got %#v", results)
	}
}

func TestRerankOnlyReportsStepCoverageReasonWhenRequestedActionsMatch(t *testing.T) {
	results := Rerank([]RawCandidate{
		{
			WorkflowID:    "wf_semantic_only",
			Version:       1,
			Status:        registry.StatusActive,
			Searchable:    true,
			Site:          "github.com",
			RiskLevel:     registry.RiskReadOnly,
			VariableNames: []string{"repo"},
			WorkflowDense: 0.9,
			StepText:      "click Issues -> click Pull requests",
			SuccessRate:   1,
		},
	}, RerankInput{
		Task:           "打开仓库主页并查看概览",
		CandidateSlots: map[string]string{"repo": "browser-use/workflow-use"},
		RiskPolicy:     defaultRiskPolicy(),
		Site:           "github.com",
	})

	if len(results) != 1 {
		t.Fatalf("expected semantic candidate, got %#v", results)
	}
	for _, reason := range results[0].Reasons {
		if reason == "recorded steps cover requested actions" {
			t.Fatalf("step coverage reason should require matched requested actions, got %#v", results[0].Reasons)
		}
	}
}

func TestRerankIncludesExplainability(t *testing.T) {
	results := Rerank([]RawCandidate{
		{
			WorkflowID:     "wf_github_issue_search",
			Version:        3,
			Status:         registry.StatusActive,
			Searchable:     true,
			Site:           "github.com",
			RiskLevel:      registry.RiskReadOrSearch,
			VariableNames:  []string{"repo", "query"},
			WorkflowDense:  0.9,
			WorkflowSparse: 0.8,
			PageState:      0.9,
			SuccessRate:    0.94,
			SelectorHealth: 0.9,
		},
	}, RerankInput{
		CandidateSlots: map[string]string{"repo": "microsoft/playwright", "query": "timeout"},
		RiskPolicy:     defaultRiskPolicy(),
		Site:           "github.com",
	})

	if len(results) != 1 {
		t.Fatalf("expected one result, got %#v", results)
	}
	if results[0].ScoreBreakdown.WorkflowDenseScore != 0.9 {
		t.Fatalf("expected score breakdown, got %#v", results[0])
	}
	if len(results[0].Reasons) == 0 {
		t.Fatalf("expected reasons, got %#v", results[0])
	}
}

func TestRerankPenalizesRepeatedRecentFailures(t *testing.T) {
	candidates := []RawCandidate{
		{
			WorkflowID:         "wf_failing",
			Version:            1,
			Status:             registry.StatusActive,
			Searchable:         true,
			Site:               "github.com",
			RiskLevel:          registry.RiskReadOrSearch,
			VariableNames:      []string{"repo", "query"},
			WorkflowDense:      0.9,
			WorkflowSparse:     0.8,
			BestChunk:          0.8,
			PageState:          0.8,
			SuccessRate:        0.95,
			SelectorHealth:     0.9,
			RecentFailureCount: 5,
		},
		{
			WorkflowID:     "wf_healthy",
			Version:        1,
			Status:         registry.StatusActive,
			Searchable:     true,
			Site:           "github.com",
			RiskLevel:      registry.RiskReadOrSearch,
			VariableNames:  []string{"repo", "query"},
			WorkflowDense:  0.86,
			WorkflowSparse: 0.76,
			BestChunk:      0.76,
			PageState:      0.76,
			SuccessRate:    0.95,
			SelectorHealth: 0.95,
		},
	}

	results := Rerank(candidates, RerankInput{
		CandidateSlots: map[string]string{"repo": "microsoft/playwright", "query": "timeout"},
		RiskPolicy:     defaultRiskPolicy(),
		Site:           "github.com",
	})

	if len(results) != 2 {
		t.Fatalf("expected two candidates, got %#v", results)
	}
	if results[0].WorkflowID != "wf_healthy" {
		t.Fatalf("expected healthy workflow to outrank repeated failure, got %#v", results)
	}
	if results[1].ScoreBreakdown.RecentFailurePenalty == 0 {
		t.Fatalf("expected failure penalty in score breakdown, got %#v", results[1].ScoreBreakdown)
	}
}

func TestRerankPrefersWorkflowWhoseRecordedStepsCoverRequestedActions(t *testing.T) {
	candidates := []RawCandidate{
		{
			WorkflowID:    "wf_exact_intent_but_partial_steps",
			Version:       1,
			Status:        registry.StatusActive,
			Searchable:    true,
			Site:          "github.com",
			RiskLevel:     registry.RiskReadOnly,
			VariableNames: []string{"repo"},
			WorkflowDense: 0.9,
			StepText:      "click Code -> click Issues -> click Pull requests -> click Agents",
			SuccessRate:   1,
		},
		{
			WorkflowID:    "wf_lower_intent_but_fuller_steps",
			Version:       1,
			Status:        registry.StatusActive,
			Searchable:    true,
			Site:          "github.com",
			RiskLevel:     registry.RiskReadOnly,
			VariableNames: []string{"repo"},
			WorkflowDense: 0.3,
			StepText:      "click Issues -> click Pull requests -> click Agents -> click Discussions -> click Actions",
			SuccessRate:   1,
		},
	}

	results := Rerank(candidates, RerankInput{
		Task: "到 https://github.com/browser-use/workflow-use 上,依次点击 Code/Issues/Pull requests/Agents/Discussions/Actions 的按钮",
		CandidateSlots: map[string]string{
			"repo": "browser-use/workflow-use",
		},
		RiskPolicy: defaultRiskPolicy(),
		Site:       "github.com",
	})

	if len(results) != 2 {
		t.Fatalf("expected two candidates, got %#v", results)
	}
	if results[0].WorkflowID != "wf_lower_intent_but_fuller_steps" {
		t.Fatalf("expected fuller recorded steps to outrank partial exact intent, got %#v", results)
	}
	if results[0].ScoreBreakdown.StepCoverage <= results[1].ScoreBreakdown.StepCoverage {
		t.Fatalf("expected first candidate to have better step coverage, got %#v", results)
	}
}

func TestRerankRecentFailureRecencyDecaysHealthPenalty(t *testing.T) {
	now := time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC)
	base := RawCandidate{
		Version:            1,
		Status:             registry.StatusActive,
		Searchable:         true,
		Site:               "github.com",
		RiskLevel:          registry.RiskReadOrSearch,
		VariableNames:      []string{"repo", "query"},
		WorkflowDense:      0.9,
		WorkflowSparse:     0.8,
		BestChunk:          0.8,
		PageState:          0.8,
		SuccessRate:        0.9,
		SelectorHealth:     0.9,
		RecentFailureCount: 1,
		Now:                now,
	}
	recentFailure := base
	recentFailure.WorkflowID = "wf_recent_failure"
	recentFailure.LastFailureAt = now.Add(-30 * time.Minute)
	oldFailure := base
	oldFailure.WorkflowID = "wf_old_failure"
	oldFailure.LastFailureAt = now.Add(-14 * 24 * time.Hour)

	results := Rerank([]RawCandidate{recentFailure, oldFailure}, RerankInput{
		CandidateSlots: map[string]string{"repo": "microsoft/playwright", "query": "timeout"},
		RiskPolicy:     defaultRiskPolicy(),
		Site:           "github.com",
	})

	if len(results) != 2 {
		t.Fatalf("expected two candidates, got %#v", results)
	}
	if results[0].WorkflowID != "wf_old_failure" {
		t.Fatalf("expected old failure to outrank recent failure, got %#v", results)
	}
	if results[0].ScoreBreakdown.RecentFailurePenalty >= results[1].ScoreBreakdown.RecentFailurePenalty {
		t.Fatalf("expected old failure penalty to be lower than recent failure penalty, got %#v", results)
	}
	if results[0].ScoreBreakdown.SelectorHealth <= results[1].ScoreBreakdown.SelectorHealth {
		t.Fatalf("expected old failure health to recover above recent failure health, got %#v", results)
	}
}

func defaultRiskPolicy() RiskPolicy {
	return RiskPolicy{
		AutoAllowed: []registry.RiskLevel{registry.RiskReadOnly, registry.RiskReadOrSearch},
		Blocked:     []registry.RiskLevel{registry.RiskDestructive},
	}
}
