package recipe

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/page-agent/workflow-backend/internal/workflow"
)

type ImportWorkflowUseOptions struct {
	ProjectID string
}

type WorkflowUseSource struct {
	ContentType string `json:"contentType"`
	Raw         []byte `json:"raw"`
}

type WorkflowUseUnsupportedStep struct {
	Index  int    `json:"index"`
	Type   string `json:"type"`
	Reason string `json:"reason"`
}

type WorkflowUseConversionReport struct {
	MappedSteps      int                          `json:"mappedSteps"`
	UnsupportedSteps []WorkflowUseUnsupportedStep `json:"unsupportedSteps,omitempty"`
	MissingVariables []string                     `json:"missingVariables,omitempty"`
	RiskWarnings     []string                     `json:"riskWarnings,omitempty"`
	Warnings         []string                     `json:"warnings,omitempty"`
}

type ImportWorkflowUseResult struct {
	Candidate WorkflowCandidate           `json:"candidate"`
	Report    WorkflowUseConversionReport `json:"report"`
	Source    *WorkflowUseSource          `json:"source,omitempty"`
}

type workflowUseFile struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Intent      string                `json:"intent"`
	URLPatterns []string              `json:"urlPatterns"`
	Version     string                `json:"version"`
	InputSchema []workflowUseVariable `json:"input_schema"`
	Variables   []workflowUseVariable `json:"variables"`
	Steps       []workflowUseStep     `json:"steps"`
}

type workflowUseVariable struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    *bool  `json:"required,omitempty"`
	Sensitive   bool   `json:"sensitive,omitempty"`
	Description string `json:"description,omitempty"`
}

type workflowUseStep struct {
	Type               string                `json:"type"`
	Description        string                `json:"description,omitempty"`
	TargetText         string                `json:"target_text,omitempty"`
	ContainerHint      string                `json:"container_hint,omitempty"`
	CSSSelector        string                `json:"cssSelector,omitempty"`
	XPath              string                `json:"xpath,omitempty"`
	SelectorStrategies []workflowUseSelector `json:"selectorStrategies,omitempty"`
	Value              string                `json:"value,omitempty"`
	SelectedText       string                `json:"selectedText,omitempty"`
	URL                string                `json:"url,omitempty"`
	ScrollX            int                   `json:"scrollX,omitempty"`
	ScrollY            int                   `json:"scrollY,omitempty"`
	WaitTime           *float64              `json:"wait_time,omitempty"`
	Extra              map[string]any        `json:"-"`
}

type workflowUseSelector struct {
	Type     string         `json:"type"`
	Value    string         `json:"value"`
	Priority int            `json:"priority,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

func ImportWorkflowUse(source []byte, options ImportWorkflowUseOptions) (ImportWorkflowUseResult, error) {
	if strings.TrimSpace(options.ProjectID) == "" {
		return ImportWorkflowUseResult{}, errors.New("project id is required")
	}
	var file workflowUseFile
	if err := json.Unmarshal(source, &file); err != nil {
		return ImportWorkflowUseResult{}, fmt.Errorf("parse workflow-use JSON: %w", err)
	}
	if len(file.Steps) == 0 {
		return ImportWorkflowUseResult{}, errors.New("workflow-use file must contain at least one step")
	}

	variables := workflowUseVariables(file)
	chunk := workflow.WorkflowChunk{
		ID:        "chunk_imported_workflow_use",
		Name:      "Imported workflow-use steps",
		RiskLevel: workflow.RiskLevelReadOnly,
	}
	report := WorkflowUseConversionReport{}
	usedVariables := map[string]bool{}

	for index, step := range file.Steps {
		converted, risk, ok, reason := convertWorkflowUseStep(step, index)
		if !ok {
			report.UnsupportedSteps = append(report.UnsupportedSteps, WorkflowUseUnsupportedStep{
				Index:  index,
				Type:   step.Type,
				Reason: reason,
			})
			report.Warnings = append(report.Warnings, fmt.Sprintf("unsupported workflow-use step %d (%s): %s", index, step.Type, reason))
			continue
		}
		for _, name := range variableRefs(converted.Value) {
			usedVariables[name] = true
		}
		chunk.RiskLevel = maxRisk(chunk.RiskLevel, risk)
		chunk.Steps = append(chunk.Steps, converted)
		report.MappedSteps++
	}
	if len(chunk.Steps) == 0 {
		return ImportWorkflowUseResult{}, errors.New("workflow-use file has no supported executable steps")
	}

	recipeVariables, missingVariables := convertWorkflowUseVariables(variables, usedVariables)
	report.MissingVariables = missingVariables
	for _, variable := range recipeVariables {
		if variable.Sensitive {
			report.RiskWarnings = append(report.RiskWarnings, fmt.Sprintf("variable %q is sensitive and requires handover_only binding", variable.Name))
		}
	}

	draft := workflow.WorkflowRecipe{
		ID:          "wf_imported_workflow_use_" + sanitizeIdentifier(file.Name),
		ProjectID:   options.ProjectID,
		Site:        siteFromPatterns(file.URLPatterns),
		Name:        firstNonEmpty(file.Name, "Imported workflow-use workflow"),
		Description: firstNonEmpty(file.Description, "Imported from workflow-use compatible JSON."),
		Intent:      firstNonEmpty(file.Intent, file.Description, file.Name),
		Tags:        []string{"imported", "workflow-use"},
		URLPatterns: append([]string(nil), file.URLPatterns...),
		PageFingerprint: workflow.PageFingerprint{
			URLPatterns:   append([]string(nil), file.URLPatterns...),
			MinMatchScore: 0.5,
		},
		Variables:    recipeVariables,
		Chunks:       []workflow.WorkflowChunk{chunk},
		SafetyPolicy: defaultSafetyPolicy(),
		Status:       workflow.WorkflowStatusDraft,
		Version:      1,
	}
	if err := workflow.ValidateRecipe(draft); err != nil {
		return ImportWorkflowUseResult{}, fmt.Errorf("imported recipe draft is invalid: %w", err)
	}

	now := time.Now().UTC()
	candidate := WorkflowCandidate{
		ID:                       "cand_imported_workflow_use_" + sanitizeIdentifier(file.Name),
		ProjectID:                options.ProjectID,
		Source:                   CandidateSourceImportedWorkflowUse,
		Task:                     draft.Intent,
		StartURL:                 firstPattern(file.URLPatterns),
		RecipeDraft:              draft,
		SensitiveRedactionReport: RedactionReport{},
		ReviewStatus:             ReviewStatusPending,
		NotificationStatus:       NotificationStatusPendingNotify,
		Searchable:               false,
		CreatedAt:                now,
	}

	return ImportWorkflowUseResult{
		Candidate: candidate,
		Report:    report,
		Source:    &WorkflowUseSource{ContentType: "application/json", Raw: append([]byte(nil), source...)},
	}, nil
}

func workflowUseVariables(file workflowUseFile) []workflowUseVariable {
	if len(file.InputSchema) > 0 {
		return file.InputSchema
	}
	return file.Variables
}

func convertWorkflowUseStep(step workflowUseStep, index int) (workflow.WorkflowStep, workflow.RiskLevel, bool, string) {
	converted := workflow.WorkflowStep{
		ID:          fmt.Sprintf("step_imported_%d", index+1),
		Description: step.Description,
		Target:      workflowUseTarget(step),
	}
	switch step.Type {
	case "click":
		converted.Type = workflow.StepTypeClick
		return converted, workflow.RiskLevelStateChange, true, ""
	case "input":
		converted.Type = workflow.StepTypeInput
		converted.Value = workflowUseValue(step.Value)
		return converted, workflow.RiskLevelFormFill, true, ""
	case "select_change", "select":
		converted.Type = workflow.StepTypeSelect
		converted.Value = workflowUseValue(step.SelectedText)
		return converted, workflow.RiskLevelFormFill, true, ""
	case "scroll":
		converted.Type = workflow.StepTypeScroll
		converted.Value = fmt.Sprintf("x=%d,y=%d", step.ScrollX, step.ScrollY)
		return converted, workflow.RiskLevelReadOnly, true, ""
	case "wait":
		converted.Type = workflow.StepTypeWait
		if step.WaitTime != nil {
			converted.Value = fmt.Sprintf("%.3fs", *step.WaitTime)
		}
		return converted, workflow.RiskLevelReadOnly, true, ""
	case "navigation":
		return workflow.WorkflowStep{}, "", false, "navigation is browser control and is kept as URL metadata only"
	case "agent", "extract", "extract_page_content", "key_press", "go_back", "go_forward":
		return workflow.WorkflowStep{}, "", false, "no PageAgent recipe step equivalent in the lightweight compatibility layer"
	default:
		return workflow.WorkflowStep{}, "", false, "unknown workflow-use action"
	}
}

func workflowUseTarget(step workflowUseStep) workflow.StepTarget {
	candidates := workflowUseCandidates(step)
	target := workflow.StepTarget{}
	if len(candidates) > 0 {
		target.Preferred = candidates[0]
	}
	if len(candidates) > 1 {
		target.Fallbacks = candidates[1:]
	}
	return target
}

func workflowUseCandidates(step workflowUseStep) []workflow.TargetCandidate {
	var candidates []workflow.TargetCandidate
	if step.TargetText != "" {
		candidates = append(candidates, workflow.TargetCandidate{
			Strategy:  workflow.TargetStrategyText,
			Value:     step.TargetText,
			Container: step.ContainerHint,
		})
	}
	for _, selector := range step.SelectorStrategies {
		candidate := candidateFromWorkflowUseSelector(selector.Type, selector.Value)
		if candidate.Strategy != "" {
			candidates = append(candidates, candidate)
		}
	}
	if step.CSSSelector != "" {
		candidates = append(candidates, workflow.TargetCandidate{Strategy: workflow.TargetStrategyCSS, Value: step.CSSSelector})
	}
	if step.XPath != "" {
		candidates = append(candidates, workflow.TargetCandidate{Strategy: workflow.TargetStrategyCSS, Value: "xpath=" + step.XPath})
	}
	return candidates
}

func candidateFromWorkflowUseSelector(selectorType, value string) workflow.TargetCandidate {
	switch strings.ToLower(selectorType) {
	case "text", "target_text":
		return workflow.TargetCandidate{Strategy: workflow.TargetStrategyText, Value: value}
	case "css", "cssselector":
		return workflow.TargetCandidate{Strategy: workflow.TargetStrategyCSS, Value: value}
	case "label":
		return workflow.TargetCandidate{Strategy: workflow.TargetStrategyLabel, Value: value}
	case "placeholder":
		return workflow.TargetCandidate{Strategy: workflow.TargetStrategyPlaceholder, Value: value}
	case "test_id", "testid":
		return workflow.TargetCandidate{Strategy: workflow.TargetStrategyTestID, Value: value}
	default:
		return workflow.TargetCandidate{}
	}
}

func workflowUseValue(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") && !strings.HasPrefix(value, "{{") {
		return "{{" + strings.Trim(value, "{}") + "}}"
	}
	return value
}

func convertWorkflowUseVariables(input []workflowUseVariable, used map[string]bool) ([]workflow.Variable, []string) {
	byName := map[string]workflowUseVariable{}
	for _, variable := range input {
		if variable.Name != "" {
			byName[variable.Name] = variable
		}
	}
	var variables []workflow.Variable
	var missing []string
	for _, variable := range input {
		if variable.Name == "" {
			continue
		}
		variables = append(variables, workflowUseVariableToRecipe(variable))
	}
	for name := range used {
		if _, ok := byName[name]; ok {
			continue
		}
		missing = append(missing, name)
		variables = append(variables, workflowUseVariableToRecipe(workflowUseVariable{Name: name, Type: "string", Required: boolPtr(true)}))
	}
	return variables, missing
}

func workflowUseVariableToRecipe(variable workflowUseVariable) workflow.Variable {
	required := true
	if variable.Required != nil {
		required = *variable.Required
	}
	sensitive := variable.Sensitive || isSensitiveName(variable.Name)
	bindingMode := workflow.BindingModeAskIfMissing
	if sensitive {
		bindingMode = workflow.BindingModeHandoverOnly
	}
	return workflow.Variable{
		Name:        variable.Name,
		Type:        workflowUseVariableType(variable.Type),
		Required:    required,
		Source:      workflow.VariableSourceUserTask,
		Sensitive:   sensitive,
		Description: variable.Description,
		BindingMode: bindingMode,
	}
}

func workflowUseVariableType(value string) workflow.VariableType {
	switch strings.ToLower(value) {
	case "number":
		return workflow.VariableTypeNumber
	case "bool", "boolean":
		return workflow.VariableTypeBoolean
	default:
		return workflow.VariableTypeString
	}
}

func variableRefs(value string) []string {
	var refs []string
	parts := strings.Split(value, "{{")
	for _, part := range parts[1:] {
		name, _, ok := strings.Cut(part, "}}")
		if ok && strings.TrimSpace(name) != "" {
			refs = append(refs, strings.TrimSpace(name))
		}
	}
	return refs
}

func isSensitiveName(name string) bool {
	name = strings.ToLower(name)
	return strings.Contains(name, "password") ||
		strings.Contains(name, "token") ||
		strings.Contains(name, "secret") ||
		strings.Contains(name, "verification") ||
		strings.Contains(name, "mfa")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstPattern(patterns []string) string {
	if len(patterns) == 0 {
		return ""
	}
	return patterns[0]
}

func siteFromPatterns(patterns []string) string {
	if len(patterns) == 0 {
		return ""
	}
	parsed, err := url.Parse(strings.TrimSuffix(patterns[0], "*"))
	if err != nil {
		return ""
	}
	return parsed.Host
}

func boolPtr(value bool) *bool {
	return &value
}
