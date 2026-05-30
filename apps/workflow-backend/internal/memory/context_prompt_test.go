package memory

import (
	"strings"
	"testing"

	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestFormatMemoryContextPromptUsesWebOpsMemory(t *testing.T) {
	response := MemoryContextResponse{
		BusinessContext: &BusinessContext{Summary: "运维系统用于查询服务状态。"},
		CurrentPage:     &MemoryPageSummary{ID: "page_service_list", Name: "服务管理", Summary: "可按服务名称搜索。"},
		NavigationHints: []NavigationHint{
			{From: "page_service_list", To: "page_service_detail", Action: "搜索后打开详情", Confidence: 0.86},
		},
		ExperienceHints: []ExperienceHint{
			{ID: "exp_service_status", Summary: "最佳路径是列表页到详情页。", OptimizedPath: []string{"page_service_list", "page_service_detail"}, Variables: []string{"service_name"}, Confidence: 0.88},
		},
		FailureWarnings: []FailureWarning{
			{Summary: "不要从首页菜单逐项探索。", AvoidHint: "当前页已有搜索框。"},
		},
		KnowledgeEvidence: []KnowledgeEvidence{
			{ChunkID: "chunk_service", Title: "服务管理手册", Snippet: "服务管理页可通过服务名称搜索框定位服务。"},
		},
	}

	prompt := FormatMemoryContextPrompt(response)

	if !strings.Contains(prompt, "<webops_memory>") || strings.Contains(prompt, "<project_knowledge>") {
		t.Fatalf("unexpected prompt wrapper: %s", prompt)
	}
	for _, expected := range []string{"<business_system>", "<current_page", "<navigation_hints>", "<experience_hints>", "<failure_warnings>", "<knowledge_evidence>"} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected %s in prompt: %s", expected, prompt)
		}
	}
}

func TestFormatMemoryContextPromptTruncatesAndRejectsSensitiveText(t *testing.T) {
	response := MemoryContextResponse{
		BusinessContext: &BusinessContext{Summary: strings.Repeat("a", MaxSummaryChars+20)},
		KnowledgeEvidence: []KnowledgeEvidence{
			{Title: "token=secret-value", Snippet: "safe snippet"},
		},
	}
	prompt := FormatMemoryContextPrompt(response)
	if strings.Contains(prompt, "token=secret-value") {
		t.Fatalf("prompt contains sensitive material: %s", prompt)
	}
	if strings.Contains(prompt, strings.Repeat("a", MaxSummaryChars+1)) {
		t.Fatalf("prompt contains overlong summary: %s", prompt)
	}
}

func TestFormatMemoryContextPromptDoesNotExposeAttributionStats(t *testing.T) {
	response := MemoryContextResponse{
		ExperienceHints: []ExperienceHint{
			{ID: "exp_service_status", Summary: "Use the service list search box.", OptimizedPath: []string{"page_service_list", "page_service_detail"}},
		},
		EvidenceRefs: []registry.MemoryEvidenceRef{
			{
				ID:     "exp_service_status",
				Source: registry.MemoryEvidenceSourceExperience,
				Payload: map[string]any{
					"utilityScore":    3.5,
					"misleadingCount": 2,
				},
			},
		},
	}

	prompt := FormatMemoryContextPrompt(response)

	for _, forbidden := range []string{"UtilityScore", "utilityScore", "misleadingCount", "3.5"} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("prompt leaked attribution internals %q: %s", forbidden, prompt)
		}
	}
}
