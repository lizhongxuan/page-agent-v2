package memory

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const MaxSummaryChars = 500

type TemplateNormalizeInput struct {
	Task      string
	Variables []TemplateVariable
	Actions   []ActionValue
}

type TemplateVariable struct {
	Name      string
	Examples  []string
	Sensitive bool
}

type ActionValue struct {
	ActionID string
	Value    string
}

type NormalizedActionValue struct {
	ActionID      string
	ValueTemplate string
}

type TemplateNormalizeResult struct {
	TaskTemplate string
	Actions      []NormalizedActionValue
	Searchable   bool
	RejectReason string
}

type WorkflowMemoryCard struct {
	Name          string
	Intent        string
	Description   string
	VariableNames []string
	ActionValues  []NormalizedActionValue
}

var sensitiveMaterialPattern = regexp.MustCompile(`(?i)\b(password|passwd|pwd|token|access[_-]?token|refresh[_-]?token|cookie|captcha|secret|api[_-]?key|authorization|bearer)\b`)

func NormalizeActionValues(input TemplateNormalizeInput) TemplateNormalizeResult {
	result := TemplateNormalizeResult{
		TaskTemplate: input.Task,
		Actions:      make([]NormalizedActionValue, 0, len(input.Actions)),
		Searchable:   true,
	}

	if ContainsSensitiveMaterial(input.Task) {
		result.Searchable = false
		result.RejectReason = "task contains sensitive material"
	}

	replacements := variableReplacements(input.Variables)
	result.TaskTemplate = applyReplacements(result.TaskTemplate, replacements)

	for _, action := range input.Actions {
		normalized := NormalizedActionValue{ActionID: action.ActionID}
		value := strings.TrimSpace(action.Value)
		switch {
		case value == "":
			normalized.ValueTemplate = ""
		case ContainsSensitiveMaterial(value) || variableForSensitiveValue(value, input.Variables) != "":
			result.Searchable = false
			if result.RejectReason == "" {
				result.RejectReason = fmt.Sprintf("action %s contains sensitive material", action.ActionID)
			}
		default:
			normalized.ValueTemplate = normalizeValue(value, replacements, input.Variables)
			if normalized.ValueTemplate == value && looksLikeInstanceValue(value) {
				result.Searchable = false
				if result.RejectReason == "" {
					result.RejectReason = fmt.Sprintf("action %s contains untemplated instance value", action.ActionID)
				}
			}
		}
		result.Actions = append(result.Actions, normalized)
	}

	return result
}

func ContainsSensitiveMaterial(text string) bool {
	return sensitiveMaterialPattern.MatchString(text)
}

func ValidateSearchableText(text string) error {
	if ContainsSensitiveMaterial(text) {
		return errors.New("searchable text contains sensitive material")
	}
	return nil
}

func ValidateEmbeddingText(text string) error {
	return ValidateSearchableText(text)
}

func (card WorkflowMemoryCard) EmbeddingText() string {
	parts := []string{
		"Workflow: " + card.Name,
		"Intent: " + card.Intent,
		"Description: " + card.Description,
		"Variables: " + strings.Join(card.VariableNames, ", "),
	}
	for _, action := range card.ActionValues {
		parts = append(parts, strings.TrimSpace(action.ActionID+" "+action.ValueTemplate))
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func ValidateSummaryLength(value string) error {
	if utf8.RuneCountInString(value) > MaxSummaryChars {
		return fmt.Errorf("summary exceeds %d characters", MaxSummaryChars)
	}
	return nil
}

func TruncateSummary(value string) string {
	runes := []rune(value)
	if len(runes) <= MaxSummaryChars {
		return value
	}
	return string(runes[:MaxSummaryChars])
}

type templateReplacement struct {
	value    string
	template string
}

func variableReplacements(variables []TemplateVariable) []templateReplacement {
	replacements := []templateReplacement{}
	for _, variable := range variables {
		if strings.TrimSpace(variable.Name) == "" || variable.Sensitive {
			continue
		}
		template := "{{" + variable.Name + "}}"
		for _, example := range variable.Examples {
			example = strings.TrimSpace(example)
			if example == "" || ContainsSensitiveMaterial(example) {
				continue
			}
			replacements = append(replacements, templateReplacement{value: example, template: template})
		}
	}
	sort.SliceStable(replacements, func(i, j int) bool {
		return len(replacements[i].value) > len(replacements[j].value)
	})
	return replacements
}

func normalizeValue(value string, replacements []templateReplacement, variables []TemplateVariable) string {
	normalized := applyReplacements(value, replacements)
	if normalized != value {
		return normalized
	}
	if inferred := inferVariableTemplate(value, variables); inferred != "" {
		return inferred
	}
	return value
}

func applyReplacements(value string, replacements []templateReplacement) string {
	result := value
	for _, replacement := range replacements {
		result = strings.ReplaceAll(result, replacement.value, replacement.template)
	}
	return result
}

func variableForSensitiveValue(value string, variables []TemplateVariable) string {
	for _, variable := range variables {
		if !variable.Sensitive {
			continue
		}
		for _, example := range variable.Examples {
			if strings.TrimSpace(example) == strings.TrimSpace(value) {
				return variable.Name
			}
		}
	}
	return ""
}

func looksLikeInstanceValue(value string) bool {
	if strings.Contains(value, "{{") && strings.Contains(value, "}}") {
		return false
	}
	if regexp.MustCompile(`\d{3,}`).MatchString(value) {
		return true
	}
	if regexp.MustCompile(`[A-Za-z]+-[A-Za-z0-9]+-[A-Za-z0-9]+`).MatchString(value) {
		return true
	}
	if strings.Contains(value, "?") && strings.Contains(value, "=") {
		return true
	}
	return false
}

func inferVariableTemplate(value string, variables []TemplateVariable) string {
	if withURLQueryTemplate := inferURLQueryTemplate(value, variables); withURLQueryTemplate != "" {
		return withURLQueryTemplate
	}
	for _, variable := range variables {
		if variable.Sensitive {
			continue
		}
		name := strings.ToLower(variable.Name)
		template := "{{" + variable.Name + "}}"
		switch {
		case strings.Contains(name, "service") && strings.Contains(name, "name") && isServiceInstance(value):
			return template
		case strings.Contains(name, "issue") && strings.Contains(name, "id") && regexp.MustCompile(`^\d{2,}$`).MatchString(value):
			return template
		case strings.Contains(name, "order") && (strings.Contains(name, "id") || strings.Contains(name, "number")) && isOrderInstance(value):
			return template
		case (name == "query" || strings.Contains(name, "query")) && strings.TrimSpace(value) != "":
			return template
		}
	}
	return ""
}

func inferURLQueryTemplate(value string, variables []TemplateVariable) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.RawQuery == "" {
		return ""
	}
	changed := false
	queryParts := strings.Split(parsed.RawQuery, "&")
	for index, part := range queryParts {
		key, rawValue, ok := strings.Cut(part, "=")
		if !ok || rawValue == "" {
			continue
		}
		queryValue, err := url.QueryUnescape(rawValue)
		if err != nil {
			queryValue = rawValue
		}
		if template := inferVariableTemplateFromNameAndValue(key, queryValue, variables); template != "" {
			queryParts[index] = key + "=" + template
			changed = true
		}
	}
	if !changed {
		return ""
	}
	parsed.RawQuery = strings.Join(queryParts, "&")
	return parsed.String()
}

func inferVariableTemplateFromNameAndValue(queryKey, value string, variables []TemplateVariable) string {
	queryKey = strings.ToLower(queryKey)
	for _, variable := range variables {
		if variable.Sensitive {
			continue
		}
		name := strings.ToLower(variable.Name)
		template := "{{" + variable.Name + "}}"
		switch {
		case strings.Contains(name, "issue") && (strings.Contains(queryKey, "issue") || queryKey == "id") && regexp.MustCompile(`^\d{2,}$`).MatchString(value):
			return template
		case strings.Contains(name, "order") && (strings.Contains(queryKey, "order") || queryKey == "id") && isOrderInstance(value):
			return template
		case strings.Contains(name, "service") && strings.Contains(queryKey, "service") && isServiceInstance(value):
			return template
		case (name == "query" || strings.Contains(name, "query")) && (queryKey == "q" || strings.Contains(queryKey, "query") || strings.Contains(queryKey, "search")):
			return template
		}
	}
	return ""
}

func isServiceInstance(value string) bool {
	return regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*-[A-Za-z0-9-]*\d[A-Za-z0-9-]*$`).MatchString(value)
}

func isOrderInstance(value string) bool {
	return regexp.MustCompile(`^(?:[A-Za-z]+-)?\d{3,}$`).MatchString(value)
}
