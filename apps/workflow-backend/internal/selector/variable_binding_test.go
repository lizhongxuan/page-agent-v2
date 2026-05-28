package selector

import (
	"testing"

	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestBindVariablesUsesURLAndTaskSlots(t *testing.T) {
	bindings, err := BindVariables([]registry.Variable{
		{Name: "repo", Type: registry.VariableString, Required: true, Source: registry.VariableSourceTaskOrURL},
		{Name: "query", Type: registry.VariableString, Required: true, Source: registry.VariableSourceTask},
	}, BindingInput{
		CandidateSlots: map[string]string{
			"repo":  "microsoft/playwright",
			"query": "timeout 报错",
		},
	})

	if err != nil {
		t.Fatalf("BindVariables failed: %v", err)
	}
	if bindings["repo"].Value != "microsoft/playwright" || bindings["repo"].Source != "slot" {
		t.Fatalf("unexpected repo binding: %#v", bindings)
	}
	if bindings["query"].Value != "timeout 报错" {
		t.Fatalf("unexpected query binding: %#v", bindings)
	}
}

func TestBindVariablesRejectsMissingRequired(t *testing.T) {
	_, err := BindVariables([]registry.Variable{
		{Name: "query", Type: registry.VariableString, Required: true, Source: registry.VariableSourceTask},
	}, BindingInput{CandidateSlots: map[string]string{}})

	if err == nil {
		t.Fatal("expected missing required variable to fail")
	}
}

func TestBindVariablesRejectsSensitiveValue(t *testing.T) {
	_, err := BindVariables([]registry.Variable{
		{Name: "query", Type: registry.VariableString, Required: true, Source: registry.VariableSourceTask},
	}, BindingInput{CandidateSlots: map[string]string{"query": "sk-1234567890abcdef"}})

	if err == nil {
		t.Fatal("expected sensitive value to fail")
	}
}
