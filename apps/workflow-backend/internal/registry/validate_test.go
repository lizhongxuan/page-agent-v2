package registry

import "testing"

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
