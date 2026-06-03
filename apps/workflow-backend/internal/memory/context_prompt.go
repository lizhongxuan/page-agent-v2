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
	Title          string                            `json:"title,omitempty"`
	VisibleText    []string                          `json:"visibleText,omitempty"`
	Controls       []PageObservationControl          `json:"controls,omitempty"`
	Breadcrumbs    []string                          `json:"breadcrumbs,omitempty"`
	ActiveTabs     []string                          `json:"activeTabs,omitempty"`
	Tables         []registry.ObservationTableSignal `json:"tables,omitempty"`
	ActiveSurfaces []registry.ActiveSurfaceSignal    `json:"activeSurfaces,omitempty"`
}

type MemoryContextResponse struct {
	ContextID             string                          `json:"contextId,omitempty"`
	ProjectID             string                          `json:"projectId,omitempty"`
	CurrentPage           *MemoryPageSummary              `json:"currentPageState,omitempty"`
	CurrentSurface        *MemorySurfaceSummary           `json:"currentSurface,omitempty"`
	RecommendedMode       registry.MemoryMode             `json:"recommendedMode"`
	ContextPrompt         string                          `json:"contextPrompt"`
	SiteTaskGuides        []SiteTaskGuideHint             `json:"siteTaskGuides,omitempty"`
	SiteManualKnowledge   []SiteManualKnowledgeHint       `json:"siteManualKnowledge,omitempty"`
	PageObservationSignal *registry.PageObservationSignal `json:"pageObservationSignal,omitempty"`
	NavigationHints       []NavigationHint                `json:"navigationHints,omitempty"`
	ExperienceHints       []ExperienceHint                `json:"experienceHints,omitempty"`
	FailureWarnings       []FailureWarning                `json:"failureWarnings,omitempty"`
	EvidenceRefs          []registry.MemoryEvidenceRef    `json:"evidenceRefs,omitempty"`
	Debug                 *MemoryContextDebug             `json:"debug,omitempty"`
}

type SiteTaskGuideHint struct {
	ID               string   `json:"id"`
	WhenToUse        string   `json:"whenToUse,omitempty"`
	MatchedStateID   string   `json:"matchedStateId,omitempty"`
	MatchedStateName string   `json:"matchedStateName,omitempty"`
	StartStepOffset  int      `json:"startStepOffset,omitempty"`
	MatchReasons     []string `json:"matchReasons,omitempty"`
	PageGuards       []string `json:"pageGuards,omitempty"`
	Steps            []string `json:"steps,omitempty"`
	AbandonRules     []string `json:"abandonRules,omitempty"`
	Confidence       float64  `json:"confidence,omitempty"`
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
	PromptChars             int                            `json:"promptChars"`
	PromptBudget            MemoryPromptBudgetDebug        `json:"promptBudget"`
	FilteredEvidence        []MemoryFilteredEvidence       `json:"filteredEvidence,omitempty"`
	CandidateSiteTaskGuides []MemoryCandidateSiteTaskGuide `json:"candidateSiteTaskGuides,omitempty"`
	UIStateMatches          []MemoryUIStateMatchDebug      `json:"uiStateMatches,omitempty"`
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

type MemoryCandidateSiteTaskGuide struct {
	ID           string  `json:"id"`
	TaskScore    float64 `json:"taskScore,omitempty"`
	Passed       bool    `json:"passed"`
	Reason       string  `json:"reason,omitempty"`
	MatchedState string  `json:"matchedStateId,omitempty"`
}

type MemoryUIStateMatchDebug struct {
	GuideID      string   `json:"guideId"`
	StateID      string   `json:"stateId"`
	Score        float64  `json:"score"`
	MinimumScore float64  `json:"minimumScore,omitempty"`
	Passed       bool     `json:"passed"`
	Matched      []string `json:"matched,omitempty"`
	Missing      []string `json:"missing,omitempty"`
	Reason       string   `json:"reason,omitempty"`
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

type SiteManualKnowledgeHint struct {
	ID         string   `json:"id"`
	Title      string   `json:"title,omitempty"`
	Summary    string   `json:"summary"`
	SourceRefs []string `json:"sourceRefs,omitempty"`
	Confidence float64  `json:"confidence,omitempty"`
}

type MemoryPageObservationRequest = PageObservationRequest

type MemoryTaskRunRequest struct {
	registry.TaskRun
}

func FormatMemoryContextPrompt(response MemoryContextResponse) string {
	sections := []string{"<webops_memory>"}
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
	if len(response.SiteTaskGuides) > 0 {
		sections = append(sections, "  <site_task_guides>")
		for _, guide := range response.SiteTaskGuides {
			sections = append(sections, fmt.Sprintf("    <guide id=\"%s\" matched_state_id=\"%s\" start_step_offset=\"%d\">",
				html.EscapeString(guide.ID),
				html.EscapeString(guide.MatchedStateID),
				guide.StartStepOffset,
			))
			if whenToUse := escapePromptText(guide.WhenToUse); whenToUse != "" {
				sections = append(sections, "      <when_to_use>"+whenToUse+"</when_to_use>")
			}
			if state := escapePromptText(guide.MatchedStateName); state != "" {
				sections = append(sections, "      <matched_state>"+state+"</matched_state>")
			}
			if len(guide.MatchReasons) > 0 {
				sections = append(sections, "      <match_reasons>")
				for _, reason := range guide.MatchReasons {
					if text := escapePromptText(reason); text != "" {
						sections = append(sections, "        <reason>"+text+"</reason>")
					}
				}
				sections = append(sections, "      </match_reasons>")
			}
			if len(guide.Steps) > 0 {
				sections = append(sections, "      <remaining_steps>")
				for index, step := range guide.Steps {
					if text := escapePromptText(step); text != "" {
						sections = append(sections, fmt.Sprintf("        <step index=\"%d\">%s</step>", guide.StartStepOffset+index+1, text))
					}
				}
				sections = append(sections, "      </remaining_steps>")
			}
			if len(guide.AbandonRules) > 0 {
				sections = append(sections, "      <abandon_if>")
				for _, rule := range guide.AbandonRules {
					if text := escapePromptText(rule); text != "" {
						sections = append(sections, "        <rule>"+text+"</rule>")
					}
				}
				sections = append(sections, "      </abandon_if>")
			}
			sections = append(sections, "    </guide>")
		}
		sections = append(sections, "  </site_task_guides>")
	}
	if len(response.SiteManualKnowledge) > 0 {
		sections = append(sections, "  <site_manual_knowledge>")
		for _, item := range response.SiteManualKnowledge {
			summary := escapePromptText(item.Summary)
			if summary == "" {
				continue
			}
			sourceRefs := escapePromptText(strings.Join(item.SourceRefs, "; "))
			if sourceRefs != "" {
				summary = strings.TrimSpace(summary + " Source refs: " + sourceRefs)
			}
			sections = append(sections, fmt.Sprintf("    <manual id=\"%s\" source=\"%s\">%s</manual>", html.EscapeString(item.ID), escapePromptText(item.Title), summary))
		}
		sections = append(sections, "  </site_manual_knowledge>")
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
