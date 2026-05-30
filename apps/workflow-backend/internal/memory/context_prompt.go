package memory

import (
	"fmt"
	"html"
	"strings"

	"github.com/page-agent/workflow-backend/internal/registry"
)

type MemoryContextRequest struct {
	ProjectID          string                 `json:"projectId"`
	Task               string                 `json:"task"`
	CurrentURL         string                 `json:"currentUrl,omitempty"`
	CurrentPageStateID string                 `json:"currentPageStateId,omitempty"`
	PageObservation    *PageObservationInput  `json:"pageObservation,omitempty"`
	Mode               string                 `json:"mode,omitempty"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
}

type PageObservationInput struct {
	Title       string                   `json:"title,omitempty"`
	VisibleText []string                 `json:"visibleText,omitempty"`
	Controls    []PageObservationControl `json:"controls,omitempty"`
}

type MemoryContextResponse struct {
	ContextID         string                       `json:"contextId,omitempty"`
	ProjectID         string                       `json:"projectId,omitempty"`
	CurrentPage       *MemoryPageSummary           `json:"currentPageState,omitempty"`
	CurrentSurface    *MemorySurfaceSummary        `json:"currentSurface,omitempty"`
	RecommendedMode   registry.MemoryMode          `json:"recommendedMode"`
	ContextPrompt     string                       `json:"contextPrompt"`
	BusinessContext   *BusinessContext             `json:"businessContext,omitempty"`
	NavigationHints   []NavigationHint             `json:"navigationHints,omitempty"`
	ExperienceHints   []ExperienceHint             `json:"experienceHints,omitempty"`
	FailureWarnings   []FailureWarning             `json:"failureWarnings,omitempty"`
	KnowledgeEvidence []KnowledgeEvidence          `json:"knowledgeEvidence,omitempty"`
	EvidenceRefs      []registry.MemoryEvidenceRef `json:"evidenceRefs,omitempty"`
	Debug             *MemoryContextDebug          `json:"debug,omitempty"`
}

type BusinessContext struct {
	Summary string `json:"summary"`
}

type MemoryPageSummary struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Summary string `json:"summary"`
}

type MemorySurfaceSummary struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Name         string `json:"name,omitempty"`
	ParentPageID string `json:"parentPageStateId,omitempty"`
}

type MemoryContextDebug struct {
	PromptChars      int                      `json:"promptChars"`
	PromptBudget     MemoryPromptBudgetDebug  `json:"promptBudget"`
	FilteredEvidence []MemoryFilteredEvidence `json:"filteredEvidence,omitempty"`
}

type MemoryPromptBudgetDebug struct {
	MaxChars  int `json:"maxChars"`
	UsedChars int `json:"usedChars"`
}

type MemoryFilteredEvidence struct {
	Source string `json:"source"`
	ID     string `json:"id,omitempty"`
	Reason string `json:"reason"`
}

type NavigationHint struct {
	From       string  `json:"from"`
	To         string  `json:"to"`
	Action     string  `json:"action"`
	Confidence float64 `json:"confidence,omitempty"`
}

type ExperienceHint struct {
	ID            string                      `json:"id"`
	Summary       string                      `json:"summary"`
	OptimizedPath []string                    `json:"optimizedPath,omitempty"`
	Variables     []string                    `json:"variables,omitempty"`
	StepTargets   []registry.MemoryStepTarget `json:"stepTargets,omitempty"`
	Confidence    float64                     `json:"confidence,omitempty"`
}

type FailureWarning struct {
	ID        string `json:"id,omitempty"`
	Summary   string `json:"summary"`
	AvoidHint string `json:"avoidHint,omitempty"`
}

type KnowledgeEvidence struct {
	ChunkID string  `json:"chunkId"`
	Title   string  `json:"title"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score,omitempty"`
}

type MemoryDocumentRequest struct {
	Documents []MemoryDocumentInput `json:"documents"`
}

type MemoryDocumentInput struct {
	ID         string   `json:"id,omitempty"`
	ProjectID  string   `json:"projectId,omitempty"`
	Title      string   `json:"title"`
	Source     string   `json:"source,omitempty"`
	URL        string   `json:"url,omitempty"`
	Content    string   `json:"content"`
	Tags       []string `json:"tags,omitempty"`
	SourceType string   `json:"sourceType,omitempty"`
}

type MemoryPageObservationRequest = PageObservationRequest

type MemoryTaskRunRequest struct {
	registry.TaskRun
}

func FormatMemoryContextPrompt(response MemoryContextResponse) string {
	sections := []string{"<webops_memory>"}
	if response.BusinessContext != nil && safeText(response.BusinessContext.Summary) != "" {
		sections = append(sections, fmt.Sprintf("  <business_system>%s</business_system>", escapePromptText(response.BusinessContext.Summary)))
	}
	if response.CurrentPage != nil {
		summary := escapePromptText(strings.TrimSpace(response.CurrentPage.Summary))
		if summary != "" {
			sections = append(sections, fmt.Sprintf("  <current_page id=\"%s\">%s</current_page>", html.EscapeString(response.CurrentPage.ID), summary))
		}
	}
	if response.CurrentSurface != nil {
		text := escapePromptText(strings.TrimSpace(response.CurrentSurface.Name + " " + response.CurrentSurface.Type))
		if text != "" {
			sections = append(sections, fmt.Sprintf("  <current_surface id=\"%s\" type=\"%s\">%s</current_surface>", html.EscapeString(response.CurrentSurface.ID), html.EscapeString(response.CurrentSurface.Type), text))
		}
	}
	if len(response.NavigationHints) > 0 {
		sections = append(sections, "  <navigation_hints>")
		for _, hint := range response.NavigationHints {
			text := escapePromptText(hint.Action)
			if text != "" {
				id := navigationEvidenceID(hint.From, hint.To, hint.Action)
				sections = append(sections, fmt.Sprintf("    <hint id=\"%s\" from=\"%s\" to=\"%s\">%s</hint>", html.EscapeString(id), html.EscapeString(hint.From), html.EscapeString(hint.To), text))
			}
		}
		sections = append(sections, "  </navigation_hints>")
	}
	if len(response.ExperienceHints) > 0 {
		sections = append(sections, "  <experience_hints>")
		for _, hint := range response.ExperienceHints {
			text := escapePromptText(strings.TrimSpace(hint.Summary + " Path: " + strings.Join(hint.OptimizedPath, " -> ")))
			if text != "" {
				sections = append(sections, fmt.Sprintf("    <success id=\"%s\">%s</success>", html.EscapeString(hint.ID), text))
			}
		}
		sections = append(sections, "  </experience_hints>")
	}
	if len(response.FailureWarnings) > 0 {
		sections = append(sections, "  <failure_warnings>")
		for _, warning := range response.FailureWarnings {
			text := escapePromptText(strings.TrimSpace(warning.Summary + " " + warning.AvoidHint))
			if text != "" {
				sections = append(sections, fmt.Sprintf("    <warning id=\"%s\">%s</warning>", html.EscapeString(warning.ID), text))
			}
		}
		sections = append(sections, "  </failure_warnings>")
	}
	if len(response.KnowledgeEvidence) > 0 {
		sections = append(sections, "  <knowledge_evidence>")
		for _, evidence := range response.KnowledgeEvidence {
			title := escapePromptText(evidence.Title)
			snippet := escapePromptText(evidence.Snippet)
			if snippet != "" {
				sections = append(sections, fmt.Sprintf("    <hit id=\"%s\" source=\"%s\">%s</hit>", html.EscapeString(evidence.ChunkID), title, snippet))
			}
		}
		sections = append(sections, "  </knowledge_evidence>")
	}
	sections = append(sections, "</webops_memory>")
	return strings.Join(sections, "\n")
}

func escapePromptText(value string) string {
	return html.EscapeString(safeText(value))
}

func safeText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || ContainsSensitiveMaterial(value) {
		return ""
	}
	return TruncateSummary(value)
}
