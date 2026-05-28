package registry

import "testing"

func TestStatusAndRiskConstants(t *testing.T) {
	if StatusPendingReview != "pending_review" || StatusIndexed != "indexed" {
		t.Fatalf("unexpected status constants: %q %q", StatusPendingReview, StatusIndexed)
	}
	if RiskReadOnly != "read_only" || RiskDestructive != "destructive" {
		t.Fatalf("unexpected risk constants: %q %q", RiskReadOnly, RiskDestructive)
	}
}

func TestWorkflowCardEmbeddingTextOmitsSensitiveFields(t *testing.T) {
	card := WorkflowCard{
		WorkflowID:    "wf_github_issue_search",
		Version:       3,
		ProjectID:     "default",
		Status:        StatusActive,
		Searchable:    true,
		Site:          "github.com",
		App:           "github",
		Name:          "Search GitHub issues",
		Intent:        "Search issues in a GitHub repository",
		Description:   "Open a repository Issues page and search by query.",
		Examples:      []string{"在 alibaba/page-agent 的 Issues 里搜索 startsWith 报错"},
		Tags:          []string{"github", "issues"},
		VariableNames: []string{"repo", "query"},
		RiskLevel:     RiskReadOrSearch,
	}

	text := card.EmbeddingText()

	if text == "" {
		t.Fatal("expected embedding text")
	}
	if containsSensitiveText(text) {
		t.Fatalf("embedding text should not contain sensitive material: %s", text)
	}
}
