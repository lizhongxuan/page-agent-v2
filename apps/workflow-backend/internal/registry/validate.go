package registry

import (
	"errors"
	"regexp"
	"strings"
)

var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bsk-[a-z0-9_-]{8,}`),
	regexp.MustCompile(`(?i)\b(token|api[_-]?key|password|passwd|secret)\b\s*[:=]\s*\S+`),
	regexp.MustCompile(`(?i)\b\d{6}\b.*\b(mfa|otp|验证码|verification)\b`),
}

func ValidateWorkflowRecipe(recipe WorkflowRecipe) error {
	switch {
	case recipe.ID == "":
		return errors.New("workflow id is required")
	case recipe.Version <= 0:
		return errors.New("workflow version is required")
	case recipe.ProjectID == "":
		return errors.New("project id is required")
	case recipe.Status == "":
		return errors.New("workflow status is required")
	case recipe.Site == "":
		return errors.New("workflow site is required")
	case recipe.Intent == "":
		return errors.New("workflow intent is required")
	case len(recipe.Variables) == 0:
		return errors.New("workflow variables are required")
	case len(recipe.Chunks) == 0:
		return errors.New("workflow chunks are required")
	}
	if recipe.RiskLevel == RiskDestructive && !recipe.RequiresConfirmation {
		return errors.New("destructive workflows must require confirmation")
	}
	if containsSensitiveText(strings.Join([]string{
		recipe.Name,
		recipe.Intent,
		recipe.Description,
		strings.Join(recipe.Tags, " "),
	}, " ")) {
		return errors.New("workflow contains sensitive text")
	}
	for _, variable := range recipe.Variables {
		if variable.Name == "" {
			return errors.New("workflow variable name is required")
		}
		if variable.Type == "" {
			return errors.New("workflow variable type is required")
		}
	}
	for _, chunk := range recipe.Chunks {
		if chunk.ID == "" {
			return errors.New("workflow chunk id is required")
		}
		if len(chunk.Steps) == 0 {
			return errors.New("workflow chunk steps are required")
		}
	}
	return nil
}

func ValidateWorkflowCard(card WorkflowCard) error {
	switch {
	case card.WorkflowID == "":
		return errors.New("workflow id is required")
	case card.Version <= 0:
		return errors.New("workflow version is required")
	case card.ProjectID == "":
		return errors.New("project id is required")
	case card.Site == "":
		return errors.New("workflow site is required")
	case card.Intent == "":
		return errors.New("workflow intent is required")
	}
	if containsSensitiveText(card.EmbeddingText()) {
		return errors.New("workflow card contains sensitive text")
	}
	return nil
}

func containsSensitiveText(value string) bool {
	for _, pattern := range sensitivePatterns {
		if pattern.MatchString(value) {
			return true
		}
	}
	return false
}
