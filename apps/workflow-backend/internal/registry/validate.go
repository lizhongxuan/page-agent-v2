package registry

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const MaxSummaryChars = 500

var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bsk-[a-z0-9_-]{8,}`),
	regexp.MustCompile(`(?i)\b(token|api[_-]?key|password|passwd|secret|cookie|captcha|authorization)\b\s*[:=]?\s*\S*`),
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
	if err := ValidateSummaryLength(recipe.Description); err != nil {
		return fmt.Errorf("workflow description %w", err)
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
		if err := ValidateSummaryLength(chunk.StepSummary); err != nil {
			return fmt.Errorf("workflow chunk %s step summary %w", chunk.ID, err)
		}
		for _, step := range chunk.Steps {
			if containsSensitiveText(step.Value) {
				return errors.New("workflow step value contains sensitive text")
			}
			if recipe.Searchable && looksLikeNonTemplateInstanceValue(step.Value) {
				return errors.New("searchable workflow step value must be templated")
			}
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

func ValidateMemoryRecord(record any) error {
	switch value := record.(type) {
	case BusinessSystemProfile:
		if value.ProjectID == "" {
			return errors.New("project id is required")
		}
		if err := validateSafeSummary("business system profile summary", value.Summary); err != nil {
			return err
		}
		for _, module := range value.Modules {
			if err := validateSafeSummary("business module purpose", module.Purpose); err != nil {
				return err
			}
		}
		for _, term := range value.Terms {
			if err := validateSafeSummary("business term", term); err != nil {
				return err
			}
		}
	case ExperienceMemory:
		if value.ProjectID == "" {
			return errors.New("project id is required")
		}
		if value.ID == "" {
			return errors.New("experience id is required")
		}
		if err := validateSafeSummary("experience summary", value.Summary); err != nil {
			return err
		}
		if containsSensitiveText(value.SearchableText()) {
			return errors.New("experience contains sensitive text")
		}
		if looksLikeNonTemplateInstanceValue(value.SearchableText()) {
			return errors.New("experience searchable text contains instance value")
		}
		for _, step := range value.StepsSummary {
			if err := validateSafeSummary("experience step result", step.ResultSummary); err != nil {
				return err
			}
		}
	case FailureMemory:
		if value.ProjectID == "" {
			return errors.New("project id is required")
		}
		if err := validateSafeSummary("failure summary", value.FailureSummary); err != nil {
			return err
		}
		if err := validateSafeSummary("failure avoid hint", value.AvoidHint); err != nil {
			return err
		}
	case MemoryReview:
		if value.ProjectID == "" {
			return errors.New("project id is required")
		}
		if err := validateSafeSummary("review summary", value.Summary); err != nil {
			return err
		}
	}
	return nil
}

func validateSafeSummary(label, value string) error {
	if err := ValidateSummaryLength(value); err != nil {
		return fmt.Errorf("%s %w", label, err)
	}
	if containsSensitiveText(value) {
		return fmt.Errorf("%s contains sensitive text", label)
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

func ValidateSummaryLength(value string) error {
	if len([]rune(value)) > MaxSummaryChars {
		return fmt.Errorf("summary exceeds %d characters", MaxSummaryChars)
	}
	return nil
}

func looksLikeNonTemplateInstanceValue(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || (strings.Contains(value, "{{") && strings.Contains(value, "}}")) {
		return false
	}
	if regexp.MustCompile(`\d{3,}`).MatchString(value) {
		return true
	}
	if regexp.MustCompile(`[A-Za-z]+-[A-Za-z0-9]+-[A-Za-z0-9]+`).MatchString(value) {
		return true
	}
	return strings.Contains(value, "?") && strings.Contains(value, "=")
}
