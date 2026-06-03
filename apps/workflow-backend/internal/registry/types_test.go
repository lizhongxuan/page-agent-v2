package registry

import (
	"encoding/json"
	"strings"
	"testing"
)

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

func TestValidateSummaryLength(t *testing.T) {
	if err := ValidateSummaryLength("short summary"); err != nil {
		t.Fatalf("short summary should be valid: %v", err)
	}
	long := ""
	for i := 0; i < 501; i++ {
		long += "a"
	}
	if err := ValidateSummaryLength(long); err == nil {
		t.Fatal("expected overlong summary to be rejected")
	}
}

func TestTaskRunCarriesOriginalAndOptimizedPaths(t *testing.T) {
	run := TaskRun{
		ID:            "task_run_1",
		ProjectID:     "default",
		Site:          "ops.example.com",
		TaskTemplate:  "查看 {{service_name}} 运行状态",
		Summary:       "查看服务运行状态。",
		OriginalPath:  []string{"page_a", "page_b", "page_c", "page_a", "page_d"},
		OptimizedPath: []string{"page_a", "page_d"},
		Status:        TaskRunSuccess,
		ActionSteps: []ActionStep{
			{
				ID:               "step_1",
				PageStateID:      "page_a",
				StepIndex:        1,
				ActionType:       "fill",
				TargetName:       "服务名称搜索框",
				ValueTemplate:    "{{service_name}}",
				ReasoningSummary: "使用固定搜索框定位服务。",
				ResultSummary:    "搜索已提交。",
			},
		},
	}

	if len(run.OriginalPath) != 5 || len(run.OptimizedPath) != 2 {
		t.Fatalf("unexpected paths on task run: %#v", run)
	}
	if run.ActionSteps[0].ValueTemplate != "{{service_name}}" {
		t.Fatalf("expected action step to store a value template, got %#v", run.ActionSteps[0])
	}
}

func TestMemoryV2TypesValidateSummariesAndSearchableText(t *testing.T) {
	experience := ExperienceMemory{
		ID:             "exp_service_status",
		ProjectID:      "default",
		Site:           "ops.example.com",
		TaskTemplate:   "Check {{service_name}} status",
		Intent:         "Check service status",
		Summary:        "Open service list and inspect the matching service.",
		StartPageState: "service-list_d7c302ab",
		EndPageState:   "service-detail_8f9012ef",
		OptimizedPath:  []string{"service-list_d7c302ab", "service-detail_8f9012ef"},
		StepsSummary: []ExperienceStepSummary{
			{ActionName: "Search service", TargetName: "Service search input", ValueTemplate: "{{service_name}}"},
		},
		Variables:    []Variable{{Name: "service_name", Type: VariableString, Required: true, Source: VariableSourceTask}},
		Searchable:   true,
		ReviewStatus: ReviewStatusAutoApproved,
	}
	if err := ValidateMemoryRecord(experience); err != nil {
		t.Fatalf("expected experience to validate: %v", err)
	}
	if text := experience.SearchableText(); containsSensitiveText(text) ||
		strings.Contains(text, "kme-prod-001") ||
		strings.Contains(text, "service-list_d7c302ab") ||
		strings.Contains(text, "service-detail_8f9012ef") {
		t.Fatalf("searchable text should only contain templates and safe labels: %s", text)
	}

	experience.Summary = repeated("x", 501)
	if err := ValidateMemoryRecord(experience); err == nil {
		t.Fatal("expected overlong experience summary to fail")
	}
	experience.Summary = "Use token=secret-value to authenticate."
	if err := ValidateMemoryRecord(experience); err == nil {
		t.Fatal("expected sensitive experience summary to fail")
	}
}

func TestMemoryAttributionLabelConstantsStable(t *testing.T) {
	if MemoryEvidenceSourceGuide != "guide" ||
		MemoryEvidenceSourceExperience != "experience" ||
		MemoryEvidenceSourceFailure != "failure" ||
		MemoryEvidenceSourceNavigation != "navigation" ||
		MemoryEvidenceSourceManual != "manual" {
		t.Fatalf("unexpected memory evidence source constants: %q %q %q %q %q",
			MemoryEvidenceSourceGuide,
			MemoryEvidenceSourceExperience,
			MemoryEvidenceSourceFailure,
			MemoryEvidenceSourceNavigation,
			MemoryEvidenceSourceManual,
		)
	}
	if MemoryAttributionHelpful != "helpful" ||
		MemoryAttributionUnused != "unused" ||
		MemoryAttributionMisleading != "misleading" ||
		MemoryAttributionStale != "stale" ||
		MemoryAttributionNeutral != "neutral" {
		t.Fatalf("unexpected memory attribution label constants: %q %q %q %q %q",
			MemoryAttributionHelpful,
			MemoryAttributionUnused,
			MemoryAttributionMisleading,
			MemoryAttributionStale,
			MemoryAttributionNeutral,
		)
	}
}

func TestSiteTaskGuideSearchableTextIncludesIntentAndSemanticTargets(t *testing.T) {
	guide := SiteTaskGuide{
		TaskIntentKey:     "restore_instance_latest_full_backup",
		TaskIntentSummary: "restore instance from latest full backup",
		TaskIntentTerms: SiteTaskIntentTerms{
			Positive: []string{"restore", "backup"},
			Negative: []string{"delete", "remove"},
		},
		TaskExamples: []string{"restore instance by latest backup"},
		Steps: []SiteTaskGuideStep{{
			Text:   "Click Data Restore.",
			Target: "Data Restore",
			SemanticTarget: SiteTaskGuideStepTarget{
				Role: "button",
				Text: "Data Restore",
			},
		}},
		UIStateEntries: []SiteTaskGuideUIStateEntry{{
			ID:         "state_full_backup_tab",
			Name:       "Full Backup tab",
			StateType:  SiteTaskGuideUIStateTab,
			StepOffset: 1,
			Evidence: SiteTaskGuideUIStateEvidence{
				ActiveTabAny: []string{"Full Backup"},
				ControlsAll:  []ControlSignature{{Role: "button", Name: "Data Restore"}},
			},
		}},
	}

	text := guide.SearchableText()

	if !strings.Contains(text, "restore instance from latest full backup") {
		t.Fatalf("expected task intent in searchable text: %s", text)
	}
	if strings.Contains(text, "delete") || strings.Contains(text, "remove") {
		t.Fatalf("negative intent terms must not boost searchable text: %s", text)
	}
	if !strings.Contains(text, "Data Restore") {
		t.Fatalf("expected stable target text in searchable text: %s", text)
	}
}

func TestMemoryEvidenceStatsJSONFieldNames(t *testing.T) {
	stats := MemoryEvidenceStats{
		ProjectID:       "default",
		Site:            "ops.example.com",
		EvidenceID:      "exp_service_status",
		EvidenceSource:  MemoryEvidenceSourceExperience,
		HelpfulCount:    3,
		UnusedCount:     1,
		MisleadingCount: 2,
		StaleCount:      1,
		NeutralCount:    4,
		UtilityScore:    0.75,
	}

	payload, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	expected := []string{
		"projectId",
		"site",
		"evidenceId",
		"evidenceSource",
		"helpfulCount",
		"unusedCount",
		"misleadingCount",
		"staleCount",
		"neutralCount",
		"utilityScore",
		"lastFeedbackAt",
	}
	for _, key := range expected {
		if _, ok := got[key]; !ok {
			t.Fatalf("expected JSON key %q in %s", key, string(payload))
		}
	}
}
