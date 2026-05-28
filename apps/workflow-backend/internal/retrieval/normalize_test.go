package retrieval

import "testing"

func TestNormalizeGitHubIssueSearch(t *testing.T) {
	request := SearchRequest{
		ProjectID:  "default",
		Task:       "在 github.com/microsoft/playwright 的 Issues 里搜索 timeout 报错",
		CurrentURL: "https://github.com/microsoft/playwright",
		PageObservation: PageObservation{
			Title:       "microsoft/playwright",
			VisibleText: []string{"Code", "Issues", "Pull requests"},
			Controls:    []Control{{Role: "link", Name: "Issues"}},
		},
	}

	normalized := NormalizeRequest(request)

	if normalized.ProjectID != "default" {
		t.Fatalf("unexpected project id: %#v", normalized)
	}
	if normalized.Site != "github.com" || normalized.App != "github" {
		t.Fatalf("expected github site/app: %#v", normalized)
	}
	if normalized.CandidateSlots["repo"] != "microsoft/playwright" {
		t.Fatalf("expected repo slot, got %#v", normalized.CandidateSlots)
	}
	if normalized.CandidateSlots["query"] != "timeout 报错" {
		t.Fatalf("expected query slot, got %#v", normalized.CandidateSlots)
	}
	if normalized.PageFingerprintText == "" {
		t.Fatal("expected page fingerprint text")
	}
}

func TestNormalizeEnglishSearchQuery(t *testing.T) {
	normalized := NormalizeRequest(SearchRequest{
		Task:       "search timeout error in microsoft/playwright issues",
		CurrentURL: "https://github.com/microsoft/playwright/issues",
	})

	if normalized.CandidateSlots["repo"] != "microsoft/playwright" {
		t.Fatalf("expected repo slot, got %#v", normalized.CandidateSlots)
	}
	if normalized.CandidateSlots["query"] != "timeout error" {
		t.Fatalf("expected English query slot, got %#v", normalized.CandidateSlots)
	}
}

func TestNormalizeExtractsGitHubRepoFromTaskWhenCurrentURLIsLocalFixture(t *testing.T) {
	normalized := NormalizeRequest(SearchRequest{
		Task:       "在 github.com/microsoft/playwright 的 Issues 里搜索 timeout 报错",
		CurrentURL: "http://127.0.0.1:49152/workflow-github-issues.html",
	})

	if normalized.Site != "127.0.0.1" {
		t.Fatalf("expected local fixture site, got %#v", normalized)
	}
	if normalized.CandidateSlots["repo"] != "microsoft/playwright" {
		t.Fatalf("expected repo slot from task, got %#v", normalized.CandidateSlots)
	}
	if normalized.CandidateSlots["query"] != "timeout 报错" {
		t.Fatalf("expected query slot, got %#v", normalized.CandidateSlots)
	}
}

func TestNormalizeMissingURLUsesDefaultProject(t *testing.T) {
	normalized := NormalizeRequest(SearchRequest{Task: "帮我查一下这个"})

	if normalized.ProjectID != "default" {
		t.Fatalf("expected default project, got %#v", normalized)
	}
	if normalized.Site != "" || normalized.App != "" {
		t.Fatalf("missing URL should not invent site/app: %#v", normalized)
	}
}
