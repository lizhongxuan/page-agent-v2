package selector

import (
	"errors"
	"regexp"

	"github.com/page-agent/workflow-backend/internal/registry"
)

type BindingInput struct {
	CandidateSlots map[string]string
	LLMBindings    map[string]string
}

type Binding struct {
	Value      string  `json:"value"`
	Source     string  `json:"source"`
	Confidence float64 `json:"confidence"`
}

var sensitiveValuePattern = regexp.MustCompile(`(?i)\b(sk-[a-z0-9_-]{8,}|token[:=]\S+|api[_-]?key[:=]\S+|password[:=]\S+)`)

func BindVariables(variables []registry.Variable, input BindingInput) (map[string]Binding, error) {
	result := map[string]Binding{}
	for _, variable := range variables {
		if variable.Sensitive {
			continue
		}
		value := input.CandidateSlots[variable.Name]
		source := "slot"
		confidence := 0.95
		if value == "" {
			value = input.LLMBindings[variable.Name]
			source = "llm"
			confidence = 0.75
		}
		if value == "" {
			if variable.Required {
				return nil, errors.New("required variable missing: " + variable.Name)
			}
			continue
		}
		if sensitiveValuePattern.MatchString(value) {
			return nil, errors.New("variable contains sensitive value: " + variable.Name)
		}
		result[variable.Name] = Binding{Value: value, Source: source, Confidence: confidence}
	}
	return result, nil
}
