package recipe

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/page-agent/workflow-backend/internal/workflow"
)

type CandidateSource string

const (
	CandidateSourceUserDemo            CandidateSource = "user_demo"
	CandidateSourceAgentRun            CandidateSource = "agent_run"
	CandidateSourceImportedWorkflowUse CandidateSource = "imported_workflow_use"
)

type ReviewStatus string

const (
	ReviewStatusPending  ReviewStatus = "pending"
	ReviewStatusApproved ReviewStatus = "approved"
	ReviewStatusRejected ReviewStatus = "rejected"
)

type NotificationStatus string

const (
	NotificationStatusPendingNotify NotificationStatus = "pending_notify"
	NotificationStatusNotified      NotificationStatus = "notified"
	NotificationStatusDismissed     NotificationStatus = "dismissed"
)

type EventType string

const (
	EventTypeClick  EventType = "click"
	EventTypeInput  EventType = "input"
	EventTypeSelect EventType = "select"
	EventTypeScroll EventType = "scroll"
	EventTypeWait   EventType = "wait"
)

type RecordedSession struct {
	ID           string            `json:"id"`
	ProjectID    string            `json:"projectId"`
	Source       CandidateSource   `json:"source"`
	Task         string            `json:"task"`
	StartURL     string            `json:"startUrl"`
	Site         string            `json:"site"`
	PageBefore   RecordedPageState `json:"pageBefore"`
	Events       []RecordedEvent   `json:"events"`
	ArtifactRefs []string          `json:"artifactRefs,omitempty"`
}

type RecordedPageState struct {
	URL               string                      `json:"url"`
	Title             string                      `json:"title"`
	VisibleText       []string                    `json:"visibleText,omitempty"`
	ControlSignatures []workflow.ControlSignature `json:"controlSignatures,omitempty"`
}

type RecordedEvent struct {
	ID               string                     `json:"id"`
	Type             EventType                  `json:"type"`
	Label            string                     `json:"label,omitempty"`
	Value            string                     `json:"value,omitempty"`
	FieldType        string                     `json:"fieldType,omitempty"`
	Sensitive        bool                       `json:"sensitive,omitempty"`
	TargetCandidates []workflow.TargetCandidate `json:"targetCandidates,omitempty"`
}

type WorkflowCandidate struct {
	ID                       string                  `json:"id"`
	ProjectID                string                  `json:"projectId"`
	Source                   CandidateSource         `json:"source"`
	Task                     string                  `json:"task"`
	StartURL                 string                  `json:"startUrl"`
	RecipeDraft              workflow.WorkflowRecipe `json:"recipeDraft"`
	SensitiveRedactionReport RedactionReport         `json:"sensitiveRedactionReport"`
	ArtifactRefs             []string                `json:"artifactRefs,omitempty"`
	ReviewStatus             ReviewStatus            `json:"reviewStatus"`
	NotificationStatus       NotificationStatus      `json:"notificationStatus"`
	Searchable               bool                    `json:"searchable"`
	UserConfirmedAt          *time.Time              `json:"userConfirmedAt,omitempty"`
	SearchableAt             *time.Time              `json:"searchableAt,omitempty"`
	CreatedAt                time.Time               `json:"createdAt"`
}

func GenerateCandidateFromSession(session RecordedSession) (WorkflowCandidate, error) {
	if session.ProjectID == "" {
		return WorkflowCandidate{}, errors.New("project id is required")
	}
	if len(session.Events) == 0 {
		return WorkflowCandidate{}, errors.New("recorded session must contain at least one event")
	}

	redactionReport := BuildRedactionReport(session.Events)
	draft := buildRecipeDraft(session)
	if err := workflow.ValidateRecipe(draft); err != nil {
		return WorkflowCandidate{}, fmt.Errorf("generated recipe draft is invalid: %w", err)
	}

	now := time.Now().UTC()
	return WorkflowCandidate{
		ID:                       candidateID(session),
		ProjectID:                session.ProjectID,
		Source:                   defaultSource(session.Source),
		Task:                     session.Task,
		StartURL:                 session.StartURL,
		RecipeDraft:              draft,
		SensitiveRedactionReport: redactionReport,
		ArtifactRefs:             append([]string(nil), session.ArtifactRefs...),
		ReviewStatus:             ReviewStatusPending,
		NotificationStatus:       NotificationStatusPendingNotify,
		Searchable:               false,
		CreatedAt:                now,
	}, nil
}

func buildRecipeDraft(session RecordedSession) workflow.WorkflowRecipe {
	chunk := workflow.WorkflowChunk{
		ID:        "chunk_recorded_actions",
		Name:      "Recorded actions",
		RiskLevel: workflow.RiskLevelReadOnly,
	}
	variables := make([]workflow.Variable, 0)
	usedNames := map[string]int{}

	for i, event := range session.Events {
		step := workflow.WorkflowStep{
			ID:     stepID(event, i),
			Type:   workflow.StepType(event.Type),
			Target: targetFromEvent(event),
		}

		risk := riskLevelForEvent(event)
		chunk.RiskLevel = maxRisk(chunk.RiskLevel, risk)

		if event.Type == EventTypeInput {
			variableName := uniqueVariableName(variableNameFromEvent(event, i), usedNames)
			step.Value = "{{" + variableName + "}}"
			sensitive := IsSensitiveEvent(event)
			bindingMode := workflow.BindingModeAskIfMissing
			if sensitive {
				bindingMode = workflow.BindingModeHandoverOnly
			}
			variables = append(variables, workflow.Variable{
				Name:        variableName,
				Type:        workflow.VariableTypeString,
				Required:    true,
				Source:      workflow.VariableSourceRecording,
				Sensitive:   sensitive,
				Description: event.Label,
				BindingMode: bindingMode,
			})
		}

		chunk.Steps = append(chunk.Steps, step)
	}

	return workflow.WorkflowRecipe{
		ID:          recipeID(session),
		ProjectID:   session.ProjectID,
		Site:        siteFromSession(session),
		Name:        nameFromTask(session.Task),
		Description: "Generated from recorded Page Agent session.",
		Intent:      session.Task,
		Tags:        []string{"recorded"},
		URLPatterns: urlPatternsFromSession(session),
		PageFingerprint: workflow.PageFingerprint{
			URLPatterns:       urlPatternsFromSession(session),
			TitleAny:          compactStrings([]string{session.PageBefore.Title}),
			RequiredText:      compactStrings(session.PageBefore.VisibleText),
			ControlSignatures: append([]workflow.ControlSignature(nil), session.PageBefore.ControlSignatures...),
			MinMatchScore:     0.6,
		},
		Variables:    variables,
		Chunks:       []workflow.WorkflowChunk{chunk},
		SafetyPolicy: defaultSafetyPolicy(),
		Status:       workflow.WorkflowStatusDraft,
		Version:      1,
	}
}

func targetFromEvent(event RecordedEvent) workflow.StepTarget {
	candidates := nonIndexCandidates(event.TargetCandidates)
	if len(candidates) == 0 {
		candidates = []workflow.TargetCandidate{{Strategy: workflow.TargetStrategyText, Value: event.Label}}
	}
	target := workflow.StepTarget{Preferred: candidates[0]}
	if len(candidates) > 1 {
		target.Fallbacks = candidates[1:]
	}
	return target
}

func nonIndexCandidates(candidates []workflow.TargetCandidate) []workflow.TargetCandidate {
	filtered := make([]workflow.TargetCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Strategy == workflow.TargetStrategyDOMIndex {
			continue
		}
		filtered = append(filtered, candidate)
	}
	return filtered
}

func defaultSafetyPolicy() workflow.SafetyPolicy {
	return workflow.SafetyPolicy{
		AllowedRiskLevels: []workflow.RiskLevel{
			workflow.RiskLevelReadOnly,
			workflow.RiskLevelFormFill,
			workflow.RiskLevelSubmitSearch,
		},
		ConfirmationRiskLevels: []workflow.RiskLevel{
			workflow.RiskLevelStateChange,
			workflow.RiskLevelProductionChange,
			workflow.RiskLevelPermissionChange,
		},
		HandoverRiskLevels: []workflow.RiskLevel{
			workflow.RiskLevelLoginSecret,
			workflow.RiskLevelCaptcha,
			workflow.RiskLevelMFA,
		},
		BlockedRiskLevels: []workflow.RiskLevel{
			workflow.RiskLevelPayment,
			workflow.RiskLevelDelete,
		},
	}
}

func riskLevelForEvent(event RecordedEvent) workflow.RiskLevel {
	if IsSensitiveEvent(event) {
		return workflow.RiskLevelLoginSecret
	}
	switch event.Type {
	case EventTypeInput, EventTypeSelect:
		return workflow.RiskLevelFormFill
	case EventTypeClick:
		label := strings.ToLower(event.Label)
		if strings.Contains(label, "search") || strings.Contains(label, "submit") {
			return workflow.RiskLevelSubmitSearch
		}
		return workflow.RiskLevelStateChange
	default:
		return workflow.RiskLevelReadOnly
	}
}

func maxRisk(current, next workflow.RiskLevel) workflow.RiskLevel {
	order := map[workflow.RiskLevel]int{
		workflow.RiskLevelReadOnly:         0,
		workflow.RiskLevelFormFill:         1,
		workflow.RiskLevelSubmitSearch:     2,
		workflow.RiskLevelStateChange:      3,
		workflow.RiskLevelProductionChange: 4,
		workflow.RiskLevelPermissionChange: 5,
		workflow.RiskLevelPayment:          6,
		workflow.RiskLevelDelete:           6,
		workflow.RiskLevelLoginSecret:      7,
		workflow.RiskLevelCaptcha:          7,
		workflow.RiskLevelMFA:              7,
	}
	if order[next] > order[current] {
		return next
	}
	return current
}

func variableNameFromEvent(event RecordedEvent, index int) string {
	base := event.Label
	if base == "" {
		base = event.FieldType
	}
	if base == "" {
		base = fmt.Sprintf("input_%d", index+1)
	}
	return sanitizeIdentifier(base)
}

func uniqueVariableName(name string, used map[string]int) string {
	if name == "" {
		name = "value"
	}
	used[name]++
	if used[name] == 1 {
		return name
	}
	return fmt.Sprintf("%s_%d", name, used[name])
}

var nonIdentifierChars = regexp.MustCompile(`[^a-z0-9]+`)

func sanitizeIdentifier(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = nonIdentifierChars.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if value == "" {
		return "value"
	}
	return value
}

func compactStrings(values []string) []string {
	compacted := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			compacted = append(compacted, value)
		}
	}
	return compacted
}

func candidateID(session RecordedSession) string {
	if session.ID == "" {
		return "cand_generated"
	}
	return "cand_" + sanitizeIdentifier(session.ID)
}

func recipeID(session RecordedSession) string {
	if session.ID == "" {
		return "wf_generated"
	}
	return "wf_" + sanitizeIdentifier(session.ID)
}

func stepID(event RecordedEvent, index int) string {
	if event.ID == "" {
		return fmt.Sprintf("step_%d", index+1)
	}
	return "step_" + sanitizeIdentifier(event.ID)
}

func defaultSource(source CandidateSource) CandidateSource {
	if source == "" {
		return CandidateSourceAgentRun
	}
	return source
}

func nameFromTask(task string) string {
	task = strings.TrimSpace(task)
	if task == "" {
		return "Recorded workflow"
	}
	return task
}

func siteFromSession(session RecordedSession) string {
	if session.Site != "" {
		return session.Site
	}
	parsed, err := url.Parse(session.StartURL)
	if err != nil {
		return ""
	}
	return parsed.Host
}

func urlPatternsFromSession(session RecordedSession) []string {
	if session.StartURL == "" {
		return nil
	}
	parsed, err := url.Parse(session.StartURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return []string{session.StartURL}
	}
	return []string{parsed.Scheme + "://" + parsed.Host + "/*"}
}
