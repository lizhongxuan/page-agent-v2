package registry

import (
	"strings"
	"testing"
)

func TestValidateWorkflowRecipeAcceptsGitHubIssueWorkflow(t *testing.T) {
	recipe := sampleWorkflowRecipe()

	if err := ValidateWorkflowRecipe(recipe); err != nil {
		t.Fatalf("expected recipe to validate: %v", err)
	}
}

func TestValidateWorkflowRecipeRejectsMissingVariableSchema(t *testing.T) {
	recipe := sampleWorkflowRecipe()
	recipe.Variables = nil

	err := ValidateWorkflowRecipe(recipe)

	if err == nil {
		t.Fatal("expected missing variable schema to fail")
	}
	if err.Error() != "workflow variables are required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateWorkflowRecipeRejectsDestructiveAutoWorkflow(t *testing.T) {
	recipe := sampleWorkflowRecipe()
	recipe.RiskLevel = RiskDestructive
	recipe.RequiresConfirmation = false

	err := ValidateWorkflowRecipe(recipe)

	if err == nil {
		t.Fatal("expected destructive workflow without confirmation to fail")
	}
	if err.Error() != "destructive workflows must require confirmation" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateWorkflowRecipeRejectsOverlongDescriptions(t *testing.T) {
	recipe := sampleWorkflowRecipe()
	recipe.Description = repeated("a", 501)

	err := ValidateWorkflowRecipe(recipe)

	if err == nil {
		t.Fatal("expected overlong description to fail")
	}
}

func TestValidateWorkflowRecipeRejectsSearchableInstanceStepValue(t *testing.T) {
	recipe := sampleWorkflowRecipe()
	recipe.Chunks[0].Steps[0].Value = "kme-prod-001"

	err := ValidateWorkflowRecipe(recipe)

	if err == nil {
		t.Fatal("expected searchable instance value to fail")
	}
}

func TestValidateWorkflowRecipeAllowsTemplatedStepValue(t *testing.T) {
	recipe := sampleWorkflowRecipe()
	recipe.Chunks[0].Steps[0].Value = "{{service_name}}"

	if err := ValidateWorkflowRecipe(recipe); err != nil {
		t.Fatalf("expected templated step value to validate: %v", err)
	}
}

func TestValidateWorkflowCardRejectsSensitiveToken(t *testing.T) {
	card := WorkflowCard{
		WorkflowID:  "wf_secret",
		Version:     1,
		ProjectID:   "default",
		Status:      StatusActive,
		Searchable:  true,
		Site:        "example.com",
		Intent:      "Use token sk-1234567890abcdef to authenticate",
		Description: "Token based workflow",
		RiskLevel:   RiskReadOnly,
	}

	err := ValidateWorkflowCard(card)

	if err == nil {
		t.Fatal("expected sensitive card text to fail")
	}
	if err.Error() != "workflow card contains sensitive text" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateSiteManualImportRequiresSiteAndContent(t *testing.T) {
	err := ValidateSiteManualImport(SiteManualImportRequest{
		ProjectID:  "default",
		Title:      "Manual",
		SourceType: SiteManualSourceMarkdown,
		Content:    "content",
	})
	if err == nil {
		t.Fatal("expected missing site to be rejected")
	}

	err = ValidateSiteManualImport(SiteManualImportRequest{
		ProjectID:  "default",
		Site:       "ops.example.com",
		Title:      "Manual",
		SourceType: SiteManualSourceMarkdown,
	})
	if err == nil {
		t.Fatal("expected empty content to be rejected")
	}
}

func TestValidateSiteManualRejectsLongSummary(t *testing.T) {
	err := ValidateSiteManualWikiPage(SiteManualWikiPage{
		ID:        "manual_page_restore",
		ProjectID: "default",
		Site:      "ops.example.com",
		PageKey:   "restore",
		Title:     "Restore",
		Summary:   strings.Repeat("a", MaxSummaryChars+1),
		Status:    StatusActive,
	})
	if err == nil {
		t.Fatal("expected long wiki summary to be rejected")
	}
}

func TestValidateSiteTaskGuideRejectsDynamicTargets(t *testing.T) {
	err := ValidateSiteTaskGuide(SiteTaskGuide{
		ID:                "guide_restore",
		ProjectID:         "default",
		Site:              "ops.example.com",
		TaskIntentKey:     "restore_backup",
		TaskIntentSummary: "Restore backup",
		Summary:           "Restore from latest backup.",
		UIStateEntries: []SiteTaskGuideUIStateEntry{{
			ID:        "state_restore",
			StateType: SiteTaskGuideUIStatePage,
			Evidence: SiteTaskGuideUIStateEvidence{
				ControlsAll: []ControlSignature{{Role: "button", Name: "Restore"}},
			},
		}},
		Status: StatusActive,
		Steps: []SiteTaskGuideStep{
			{Text: "Click element_33"},
		},
	})
	if err == nil {
		t.Fatal("expected dynamic element target to be rejected")
	}
}

func TestValidateSiteTaskGuideRequiresUIStateEntries(t *testing.T) {
	err := ValidateSiteTaskGuide(SiteTaskGuide{
		ID:                "guide_restore",
		ProjectID:         "default",
		Site:              "ops.example.com",
		TaskIntentKey:     "restore_backup",
		TaskIntentSummary: "Restore backup",
		Summary:           "Restore from latest backup.",
		Status:            StatusActive,
		Steps:             []SiteTaskGuideStep{{Text: "Click Restore.", Target: "Restore"}},
	})
	if err == nil || err.Error() != "active site task guide requires ui state entries" {
		t.Fatalf("expected active guide without ui states to be rejected, got %v", err)
	}
}

func TestValidateSiteTaskGuideRejectsDynamicUIStateEvidence(t *testing.T) {
	err := ValidateSiteTaskGuide(SiteTaskGuide{
		ID:                "guide_restore",
		ProjectID:         "default",
		Site:              "ops.example.com",
		TaskIntentKey:     "restore_backup",
		TaskIntentSummary: "Restore backup",
		Summary:           "Restore from latest backup.",
		Status:            StatusActive,
		UIStateEntries: []SiteTaskGuideUIStateEntry{{
			ID:        "state_restore",
			StateType: SiteTaskGuideUIStatePage,
			Evidence: SiteTaskGuideUIStateEvidence{
				ControlsAll: []ControlSignature{{Role: "button", Name: "element_33"}},
			},
		}},
		Steps: []SiteTaskGuideStep{{Text: "Click Restore.", Target: "Restore"}},
	})
	if err == nil {
		t.Fatal("expected dynamic ui state evidence to be rejected")
	}
}

func sampleWorkflowRecipe() WorkflowRecipe {
	return WorkflowRecipe{
		ID:          "wf_github_issue_search",
		Version:     3,
		ProjectID:   "default",
		Status:      StatusActive,
		Searchable:  true,
		Site:        "github.com",
		App:         "github",
		Name:        "Search GitHub issues",
		Intent:      "Search issues in a GitHub repository",
		Description: "Open a repository Issues page and search by query.",
		Tags:        []string{"github", "issues", "search"},
		RiskLevel:   RiskReadOrSearch,
		Variables: []Variable{
			{Name: "repo", Type: VariableString, Required: true, Source: VariableSourceTaskOrURL},
			{Name: "query", Type: VariableString, Required: true, Source: VariableSourceTask},
		},
		Chunks: []WorkflowChunk{
			{
				ID:            "open_issues",
				Name:          "Open Issues",
				FromPageState: "github_repo_home",
				ToPageState:   "github_issues_list",
				RiskLevel:     RiskReadOrSearch,
				Steps: []WorkflowStep{
					{
						ID:   "click_issues",
						Type: StepClick,
						Target: StepTarget{
							Primary: TargetCandidate{Strategy: TargetRole, Role: "link", Name: "Issues"},
						},
						RiskLevel: RiskReadOrSearch,
					},
				},
			},
		},
	}
}

func repeated(value string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += value
	}
	return result
}
