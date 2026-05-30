package memory

import (
	"strings"
	"testing"
)

func TestNormalizeActionValuesReplacesServiceName(t *testing.T) {
	result := NormalizeActionValues(TemplateNormalizeInput{
		Task: "Search service kme-prod-001",
		Variables: []TemplateVariable{
			{Name: "service_name", Examples: []string{"kme-prod-001"}},
		},
		Actions: []ActionValue{{ActionID: "fill-service", Value: "kme-prod-001"}},
	})

	if !result.Searchable {
		t.Fatalf("expected result to be searchable: %+v", result)
	}
	if got, want := result.Actions[0].ValueTemplate, "{{service_name}}"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestNormalizeActionValuesReplacesIssueID(t *testing.T) {
	result := NormalizeActionValues(TemplateNormalizeInput{
		Task: "Open issue 1842",
		Variables: []TemplateVariable{
			{Name: "issue_id", Examples: []string{"1842"}},
		},
		Actions: []ActionValue{{ActionID: "fill-issue", Value: "1842"}},
	})

	if got, want := result.Actions[0].ValueTemplate, "{{issue_id}}"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestNormalizeActionValuesInfersVariableTemplates(t *testing.T) {
	result := NormalizeActionValues(TemplateNormalizeInput{
		Variables: []TemplateVariable{
			{Name: "service_name"},
			{Name: "issue_id"},
			{Name: "order_id"},
		},
		Actions: []ActionValue{
			{ActionID: "fill-service", Value: "kme-prod-001"},
			{ActionID: "fill-issue", Value: "1842"},
			{ActionID: "open-order-url", Value: "https://example.test/orders?id=ORD-998877"},
		},
	})

	if !result.Searchable {
		t.Fatalf("expected inferred templates to be searchable: %+v", result)
	}
	want := []string{"{{service_name}}", "{{issue_id}}", "https://example.test/orders?id={{order_id}}"}
	for index, expected := range want {
		if got := result.Actions[index].ValueTemplate; got != expected {
			t.Fatalf("action %d got %q, want %q", index, got, expected)
		}
	}
}

func TestNormalizeActionValuesRejectsSensitiveValues(t *testing.T) {
	cases := []string{
		"password=correct-horse-battery-staple",
		"token=ghp_abcdefghijklmnopqrstuvwxyz123456",
		"cookie=sessionid=abcdef123456",
		"captcha 123456",
		"secret API value",
	}

	for _, value := range cases {
		result := NormalizeActionValues(TemplateNormalizeInput{
			Variables: []TemplateVariable{{Name: "unsafe_value", Examples: []string{value}}},
			Actions:   []ActionValue{{ActionID: "fill", Value: value}},
		})
		if result.Searchable {
			t.Fatalf("expected %q to make result non-searchable", value)
		}
		if !ContainsSensitiveMaterial(value) {
			t.Fatalf("expected %q to be detected as sensitive", value)
		}
	}
}

func TestWorkflowCardEmbeddingTextExcludesInstanceValues(t *testing.T) {
	card := WorkflowMemoryCard{
		Name:          "Open service issue",
		Intent:        "Search service {{service_name}} and open issue {{issue_id}}",
		Description:   "Use the service search box with {{service_name}}.",
		VariableNames: []string{"service_name", "issue_id"},
		ActionValues: []NormalizedActionValue{
			{ActionID: "fill-service", ValueTemplate: "{{service_name}}"},
			{ActionID: "fill-issue", ValueTemplate: "{{issue_id}}"},
		},
	}

	embeddingText := card.EmbeddingText()
	for _, forbidden := range []string{"kme-prod-001", "1842"} {
		if strings.Contains(embeddingText, forbidden) {
			t.Fatalf("embedding text contains instance value %q: %s", forbidden, embeddingText)
		}
	}
	if ContainsSensitiveMaterial(embeddingText) {
		t.Fatalf("embedding text should not contain sensitive material: %s", embeddingText)
	}
}

func TestSearchableTextValidationDetectsSensitiveMaterial(t *testing.T) {
	if err := ValidateSearchableText("open issue {{issue_id}}"); err != nil {
		t.Fatalf("expected template text to be valid: %v", err)
	}
	if err := ValidateSearchableText("Authorization token abc123"); err == nil {
		t.Fatal("expected token text to be rejected")
	}
	if err := ValidateEmbeddingText("cookie=sessionid=abcdef"); err == nil {
		t.Fatal("expected cookie text to be rejected")
	}
}

func TestSummaryHelpers(t *testing.T) {
	long := strings.Repeat("a", MaxSummaryChars+20)
	truncated := TruncateSummary(long)
	if len(truncated) != MaxSummaryChars {
		t.Fatalf("got truncated length %d, want %d", len(truncated), MaxSummaryChars)
	}
	if err := ValidateSummaryLength(truncated); err != nil {
		t.Fatalf("expected truncated summary to be valid: %v", err)
	}
	if err := ValidateSummaryLength(long); err == nil {
		t.Fatal("expected long summary to be rejected")
	}
}
