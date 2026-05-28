package recipe

import (
	"fmt"
	"strings"

	"github.com/page-agent/workflow-backend/internal/workflow"
)

type WorkflowUseExport struct {
	Name        string                      `json:"name"`
	Description string                      `json:"description,omitempty"`
	Intent      string                      `json:"intent,omitempty"`
	Version     string                      `json:"version"`
	URLPatterns []string                    `json:"urlPatterns,omitempty"`
	InputSchema []WorkflowUseExportVariable `json:"input_schema,omitempty"`
	Chunks      []WorkflowUseExportChunk    `json:"chunks,omitempty"`
	Steps       []WorkflowUseExportStep     `json:"steps"`
}

type WorkflowUseExportVariable struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Sensitive   bool   `json:"sensitive,omitempty"`
	Description string `json:"description,omitempty"`
	BindingMode string `json:"bindingMode,omitempty"`
}

type WorkflowUseExportChunk struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	RiskLevel string `json:"riskLevel"`
	StepStart int    `json:"stepStart"`
	StepEnd   int    `json:"stepEnd"`
}

type WorkflowUseExportStep struct {
	Type              string                        `json:"type"`
	Description       string                        `json:"description,omitempty"`
	TargetText        string                        `json:"target_text,omitempty"`
	CSSSelector       string                        `json:"cssSelector,omitempty"`
	Value             string                        `json:"value,omitempty"`
	SelectedText      string                        `json:"selectedText,omitempty"`
	LocatorCandidates []WorkflowUseLocatorCandidate `json:"locatorCandidates,omitempty"`
	Metadata          map[string]any                `json:"metadata,omitempty"`
}

type WorkflowUseLocatorCandidate struct {
	Strategy  string `json:"strategy,omitempty"`
	Value     string `json:"value,omitempty"`
	Role      string `json:"role,omitempty"`
	Name      string `json:"name,omitempty"`
	NearText  string `json:"nearText,omitempty"`
	Container string `json:"container,omitempty"`
	Index     int    `json:"index,omitempty"`
}

func ExportWorkflowUse(recipe workflow.WorkflowRecipe) WorkflowUseExport {
	exported := WorkflowUseExport{
		Name:        recipe.Name,
		Description: recipe.Description,
		Intent:      recipe.Intent,
		Version:     fmt.Sprintf("%d", recipe.Version),
		URLPatterns: append([]string(nil), recipe.URLPatterns...),
		InputSchema: exportWorkflowUseVariables(recipe.Variables),
	}
	sensitiveVariables := map[string]bool{}
	for _, variable := range recipe.Variables {
		sensitiveVariables[variable.Name] = variable.Sensitive
	}

	stepIndex := 0
	for _, chunk := range recipe.Chunks {
		start := stepIndex
		for _, step := range chunk.Steps {
			exported.Steps = append(exported.Steps, exportWorkflowUseStep(step, sensitiveVariables))
			stepIndex++
		}
		exported.Chunks = append(exported.Chunks, WorkflowUseExportChunk{
			ID:        chunk.ID,
			Name:      chunk.Name,
			RiskLevel: string(chunk.RiskLevel),
			StepStart: start,
			StepEnd:   stepIndex,
		})
	}
	return exported
}

func exportWorkflowUseVariables(variables []workflow.Variable) []WorkflowUseExportVariable {
	exported := make([]WorkflowUseExportVariable, 0, len(variables))
	for _, variable := range variables {
		exported = append(exported, WorkflowUseExportVariable{
			Name:        variable.Name,
			Type:        exportVariableType(variable.Type),
			Required:    variable.Required,
			Sensitive:   variable.Sensitive,
			Description: variable.Description,
			BindingMode: string(variable.BindingMode),
		})
	}
	return exported
}

func exportWorkflowUseStep(step workflow.WorkflowStep, sensitiveVariables map[string]bool) WorkflowUseExportStep {
	exported := WorkflowUseExportStep{
		Type:              exportStepType(step.Type),
		Description:       step.Description,
		LocatorCandidates: exportLocatorCandidates(step.Target),
		Metadata: map[string]any{
			"pageAgentStepId": string(step.ID),
			"pageAgentType":   string(step.Type),
		},
	}
	if len(exported.LocatorCandidates) == 0 {
		exported.LocatorCandidates = nil
	}
	applyPreferredLocator(&exported, step.Target.Preferred)
	switch step.Type {
	case workflow.StepTypeInput:
		exported.Value = exportStepValue(step.Value, sensitiveVariables)
	case workflow.StepTypeSelect:
		exported.SelectedText = exportStepValue(step.Value, sensitiveVariables)
	default:
		exported.Value = step.Value
	}
	return exported
}

func exportStepType(stepType workflow.StepType) string {
	switch stepType {
	case workflow.StepTypeSelect:
		return "select_change"
	default:
		return string(stepType)
	}
}

func exportStepValue(value string, sensitiveVariables map[string]bool) string {
	refs := variableRefs(value)
	for _, ref := range refs {
		if sensitiveVariables[ref] {
			return RedactedValue
		}
	}
	if len(refs) == 1 && value == "{{"+refs[0]+"}}" {
		return "{" + refs[0] + "}"
	}
	return value
}

func applyPreferredLocator(exported *WorkflowUseExportStep, preferred workflow.TargetCandidate) {
	switch preferred.Strategy {
	case workflow.TargetStrategyText, workflow.TargetStrategyLabel, workflow.TargetStrategyPlaceholder, workflow.TargetStrategyTestID:
		exported.TargetText = preferred.Value
	case workflow.TargetStrategyRole:
		exported.TargetText = strings.TrimSpace(strings.Join([]string{preferred.Role, preferred.Name}, " "))
	case workflow.TargetStrategyCSS:
		exported.CSSSelector = preferred.Value
	}
}

func exportLocatorCandidates(target workflow.StepTarget) []WorkflowUseLocatorCandidate {
	candidates := append([]workflow.TargetCandidate{target.Preferred}, target.Fallbacks...)
	exported := make([]WorkflowUseLocatorCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Strategy == "" {
			continue
		}
		exported = append(exported, WorkflowUseLocatorCandidate{
			Strategy:  string(candidate.Strategy),
			Value:     candidate.Value,
			Role:      candidate.Role,
			Name:      candidate.Name,
			NearText:  candidate.NearText,
			Container: candidate.Container,
			Index:     candidate.Index,
		})
	}
	return exported
}

func exportVariableType(variableType workflow.VariableType) string {
	switch variableType {
	case workflow.VariableTypeNumber:
		return "number"
	case workflow.VariableTypeBoolean:
		return "bool"
	default:
		return "string"
	}
}
