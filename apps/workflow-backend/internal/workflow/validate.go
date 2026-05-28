package workflow

import (
	"errors"
	"fmt"
)

func ValidateRecipe(recipe WorkflowRecipe) error {
	var errs []error

	if recipe.ID == "" {
		errs = append(errs, errors.New("workflow id is required"))
	}
	if recipe.ProjectID == "" {
		errs = append(errs, errors.New("project id is required"))
	}
	if len(recipe.Chunks) == 0 {
		errs = append(errs, errors.New("at least one chunk is required"))
	}

	for i, variable := range recipe.Variables {
		if variable.Name == "" {
			errs = append(errs, fmt.Errorf("variable[%d] name is required", i))
		}
		if !IsKnownBindingMode(variable.BindingMode) {
			errs = append(errs, fmt.Errorf("variable %q has unknown binding mode %q", variable.Name, variable.BindingMode))
		}
		if variable.Sensitive && variable.BindingMode != BindingModeHandoverOnly {
			errs = append(errs, fmt.Errorf("sensitive variable %q must use handover_only binding policy", variable.Name))
		}
	}

	validateRiskLevels := func(label string, levels []RiskLevel) {
		for _, level := range levels {
			if !IsKnownRiskLevel(level) {
				errs = append(errs, fmt.Errorf("%s contains unknown risk level %q", label, level))
			}
		}
	}
	validateRiskLevels("allowedRiskLevels", recipe.SafetyPolicy.AllowedRiskLevels)
	validateRiskLevels("confirmationRiskLevels", recipe.SafetyPolicy.ConfirmationRiskLevels)
	validateRiskLevels("handoverRiskLevels", recipe.SafetyPolicy.HandoverRiskLevels)
	validateRiskLevels("blockedRiskLevels", recipe.SafetyPolicy.BlockedRiskLevels)

	for chunkIndex, chunk := range recipe.Chunks {
		if chunk.ID == "" {
			errs = append(errs, fmt.Errorf("chunk[%d] id is required", chunkIndex))
		}
		if !IsKnownRiskLevel(chunk.RiskLevel) {
			errs = append(errs, fmt.Errorf("chunk %q has unknown risk level %q", chunk.ID, chunk.RiskLevel))
		}
		for stepIndex, step := range chunk.Steps {
			if usesHistoricalDOMIndexOnly(step.Target) {
				errs = append(errs, fmt.Errorf("chunk %q step[%d] uses historical DOM index as long-term target", chunk.ID, stepIndex))
			}
		}
	}

	return errors.Join(errs...)
}

func usesHistoricalDOMIndexOnly(target StepTarget) bool {
	candidates := append([]TargetCandidate{target.Preferred}, target.Fallbacks...)
	if len(candidates) == 0 {
		return false
	}

	hasCandidate := false
	for _, candidate := range candidates {
		if candidate.Strategy == "" {
			continue
		}
		hasCandidate = true
		if candidate.Strategy != TargetStrategyDOMIndex {
			return false
		}
	}

	return hasCandidate
}
