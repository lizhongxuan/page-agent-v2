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

func ValidateSiteManualImport(request SiteManualImportRequest) error {
	if strings.TrimSpace(request.ProjectID) == "" {
		return errors.New("project id is required")
	}
	if strings.TrimSpace(request.Site) == "" {
		return errors.New("site is required")
	}
	if strings.TrimSpace(request.Content) == "" {
		return errors.New("content is required")
	}
	if !validSiteManualSourceType(request.SourceType) {
		return errors.New("source type is invalid")
	}
	return nil
}

func ValidateSiteManualSource(source SiteManualSource) error {
	if strings.TrimSpace(source.ProjectID) == "" {
		return errors.New("project id is required")
	}
	if strings.TrimSpace(source.Site) == "" {
		return errors.New("site is required")
	}
	if strings.TrimSpace(source.ContentHash) == "" {
		return errors.New("content hash is required")
	}
	if strings.TrimSpace(source.RawContent) == "" {
		return errors.New("raw content is required")
	}
	if !validSiteManualSourceType(source.SourceType) {
		return errors.New("source type is invalid")
	}
	return nil
}

func ValidateSiteManualWikiPage(page SiteManualWikiPage) error {
	if strings.TrimSpace(page.ProjectID) == "" {
		return errors.New("project id is required")
	}
	if strings.TrimSpace(page.Site) == "" {
		return errors.New("site is required")
	}
	if strings.TrimSpace(page.PageKey) == "" {
		return errors.New("page key is required")
	}
	if err := validateSafeSummary("site manual wiki summary", page.Summary); err != nil {
		return err
	}
	for _, fact := range page.Facts {
		if err := validateSafeSummary("site manual fact", fact); err != nil {
			return err
		}
	}
	for _, procedure := range page.Procedures {
		if err := validateSafeSummary("site manual procedure", procedure); err != nil {
			return err
		}
	}
	return nil
}

func ValidateSiteManualWikiChunk(chunk SiteManualWikiChunk) error {
	if strings.TrimSpace(chunk.ProjectID) == "" {
		return errors.New("project id is required")
	}
	if strings.TrimSpace(chunk.Site) == "" {
		return errors.New("site is required")
	}
	if strings.TrimSpace(chunk.WikiPageID) == "" {
		return errors.New("wiki page id is required")
	}
	if err := validateSafeSummary("site manual chunk text", chunk.Text); err != nil {
		return err
	}
	return nil
}

func ValidateSiteTaskGuide(guide SiteTaskGuide) error {
	if strings.TrimSpace(guide.ProjectID) == "" {
		return errors.New("project id is required")
	}
	if strings.TrimSpace(guide.Site) == "" {
		return errors.New("site is required")
	}
	if strings.TrimSpace(guide.TaskIntentKey) == "" {
		return errors.New("task intent key is required")
	}
	if err := validateSafeSummary("site task guide summary", guide.Summary); err != nil {
		return err
	}
	if looksLikeDynamicTarget(guide.Summary) {
		return errors.New("site task guide summary contains dynamic target")
	}
	if guide.Status == "" || guide.Status == StatusActive {
		if len(guide.UIStateEntries) == 0 {
			return errors.New("active site task guide requires ui state entries")
		}
	}
	for _, term := range guide.TaskIntentTerms.Positive {
		if err := validateSafeSummary("site task guide positive intent term", term); err != nil {
			return err
		}
	}
	for _, term := range guide.TaskIntentTerms.Negative {
		if err := validateSafeSummary("site task guide negative intent term", term); err != nil {
			return err
		}
	}
	for _, state := range guide.UIStateEntries {
		if err := validateSiteTaskGuideUIStateEntry(state); err != nil {
			return err
		}
	}
	for _, step := range guide.Steps {
		if err := validateSafeSummary("site task guide step", step.Text); err != nil {
			return err
		}
		if looksLikeDynamicTarget(step.Text) ||
			looksLikeDynamicTarget(step.Target) ||
			looksLikeDynamicTarget(step.SemanticTarget.Text) ||
			looksLikeDynamicTarget(step.SemanticTarget.ContainerHint) {
			return errors.New("site task guide step contains dynamic target")
		}
		if err := validateControlSignatureList("site task guide expected outcome control", step.ExpectedOutcome.ControlsAll); err != nil {
			return err
		}
		if err := validateControlSignatureList("site task guide expected outcome control", step.ExpectedOutcome.ControlsAny); err != nil {
			return err
		}
	}
	return nil
}

func validateSiteTaskGuideUIStateEntry(state SiteTaskGuideUIStateEntry) error {
	if strings.TrimSpace(state.ID) == "" {
		return errors.New("ui state id is required")
	}
	if strings.TrimSpace(string(state.StateType)) == "" {
		return errors.New("ui state type is required")
	}
	for _, value := range append([]string{state.Name}, state.Evidence.TitleAny...) {
		if err := validateSafeSummary("ui state evidence", value); err != nil {
			return err
		}
		if looksLikeDynamicTarget(value) {
			return errors.New("ui state evidence contains dynamic target")
		}
	}
	textGroups := [][]string{
		state.Evidence.BreadcrumbAny,
		state.Evidence.ActiveTabAny,
		state.Evidence.TextAll,
		state.Evidence.TextAny,
		state.Evidence.NegativeTextAny,
	}
	for _, group := range textGroups {
		for _, value := range group {
			if err := validateSafeSummary("ui state evidence", value); err != nil {
				return err
			}
			if looksLikeDynamicTarget(value) {
				return errors.New("ui state evidence contains dynamic target")
			}
		}
	}
	for _, headers := range state.Evidence.TableHeadersAny {
		for _, value := range headers {
			if err := validateSafeSummary("ui state table header", value); err != nil {
				return err
			}
		}
	}
	if err := validateControlSignatureList("ui state required control", state.Evidence.ControlsAll); err != nil {
		return err
	}
	if err := validateControlSignatureList("ui state optional control", state.Evidence.ControlsAny); err != nil {
		return err
	}
	if err := validateControlSignatureList("ui state negative control", state.Evidence.NegativeControlsAny); err != nil {
		return err
	}
	for _, surface := range state.Evidence.ActiveSurfacesAny {
		if err := validateSafeSummary("ui state active surface title", surface.Title); err != nil {
			return err
		}
		if err := validateControlSignatureList("ui state active surface control", surface.Controls); err != nil {
			return err
		}
	}
	return nil
}

func validateControlSignatureList(label string, controls []ControlSignature) error {
	for _, control := range controls {
		if err := validateSafeSummary(label, strings.TrimSpace(control.Role+" "+control.Name)); err != nil {
			return err
		}
		if looksLikeDynamicTarget(control.Name) {
			return fmt.Errorf("%s contains dynamic target", label)
		}
	}
	return nil
}

func validSiteManualSourceType(sourceType SiteManualSourceType) bool {
	switch sourceType {
	case SiteManualSourceMarkdown, SiteManualSourceHTML, SiteManualSourcePDFText, SiteManualSourceText:
		return true
	default:
		return false
	}
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

func looksLikeDynamicTarget(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return false
	}
	return regexp.MustCompile(`\b(element|index)[_-]?\d+\b`).MatchString(value) ||
		regexp.MustCompile(`\bindex\s*=\s*\d+\b`).MatchString(value)
}
