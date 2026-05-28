package recipe

import (
	"testing"

	"github.com/page-agent/workflow-backend/internal/workflow"
)

func TestGenerateCandidateFromSessionCreatesPendingNonSearchableCandidate(t *testing.T) {
	session := RecordedSession{
		ID:        "session_1",
		ProjectID: "default",
		Source:    CandidateSourceUserDemo,
		Task:      "Search for quarterly results",
		StartURL:  "https://example.com/",
		Site:      "example.com",
		PageBefore: RecordedPageState{
			URL:               "https://example.com/",
			Title:             "Example",
			VisibleText:       []string{"Search"},
			ControlSignatures: []workflow.ControlSignature{{Role: "textbox", Name: "Search"}},
		},
		Events: []RecordedEvent{
			{
				ID:    "event_1",
				Type:  EventTypeInput,
				Label: "Search",
				Value: "quarterly results",
				TargetCandidates: []workflow.TargetCandidate{
					{Strategy: workflow.TargetStrategyRole, Role: "textbox", Name: "Search"},
				},
			},
			{
				ID:    "event_2",
				Type:  EventTypeClick,
				Label: "Search",
				TargetCandidates: []workflow.TargetCandidate{
					{Strategy: workflow.TargetStrategyRole, Role: "button", Name: "Search"},
				},
			},
		},
		ArtifactRefs: []string{"artifact_session_1"},
	}

	candidate, err := GenerateCandidateFromSession(session)
	if err != nil {
		t.Fatalf("GenerateCandidateFromSession returned error: %v", err)
	}

	if candidate.ReviewStatus != ReviewStatusPending {
		t.Fatalf("expected pending review status, got %q", candidate.ReviewStatus)
	}
	if candidate.NotificationStatus != NotificationStatusPendingNotify {
		t.Fatalf("expected pending_notify notification status, got %q", candidate.NotificationStatus)
	}
	if candidate.Searchable {
		t.Fatal("expected pending candidate to be non-searchable")
	}
	if candidate.SearchableAt != nil {
		t.Fatalf("expected nil SearchableAt for pending candidate, got %v", candidate.SearchableAt)
	}
	if candidate.RecipeDraft.Status != workflow.WorkflowStatusDraft {
		t.Fatalf("expected draft recipe status, got %q", candidate.RecipeDraft.Status)
	}
	if got := candidate.RecipeDraft.Chunks[0].Steps[0].Value; got != "{{search}}" {
		t.Fatalf("expected input value to be variable reference, got %q", got)
	}
	if got := candidate.RecipeDraft.Variables[0].Name; got != "search" {
		t.Fatalf("expected generated variable name search, got %q", got)
	}
	if got := candidate.RecipeDraft.PageFingerprint.RequiredText[0]; got != "Search" {
		t.Fatalf("expected page fingerprint required text, got %q", got)
	}
	if len(candidate.ArtifactRefs) != 1 || candidate.ArtifactRefs[0] != "artifact_session_1" {
		t.Fatalf("artifact refs were not preserved: %#v", candidate.ArtifactRefs)
	}
}

func TestGenerateCandidateFromSessionRedactsSensitiveInputs(t *testing.T) {
	session := RecordedSession{
		ID:        "session_sensitive",
		ProjectID: "default",
		Source:    CandidateSourceAgentRun,
		Task:      "Log in",
		StartURL:  "https://example.com/login",
		Site:      "example.com",
		PageBefore: RecordedPageState{
			URL:         "https://example.com/login",
			Title:       "Login",
			VisibleText: []string{"Password"},
		},
		Events: []RecordedEvent{
			{
				ID:        "event_password",
				Type:      EventTypeInput,
				Label:     "Password",
				Value:     "correct horse battery staple",
				FieldType: "password",
				Sensitive: true,
				TargetCandidates: []workflow.TargetCandidate{
					{Strategy: workflow.TargetStrategyLabel, Value: "Password"},
				},
			},
		},
	}

	candidate, err := GenerateCandidateFromSession(session)
	if err != nil {
		t.Fatalf("GenerateCandidateFromSession returned error: %v", err)
	}

	variable := candidate.RecipeDraft.Variables[0]
	if !variable.Sensitive {
		t.Fatal("expected generated password variable to be sensitive")
	}
	if variable.BindingMode != workflow.BindingModeHandoverOnly {
		t.Fatalf("expected sensitive variable to be handover_only, got %q", variable.BindingMode)
	}
	if got := candidate.SensitiveRedactionReport.RedactedFields[0].Replacement; got != RedactedValue {
		t.Fatalf("expected redacted replacement marker, got %q", got)
	}
	if got := candidate.SensitiveRedactionReport.RedactedFields[0].OriginalValuePreview; got != "" {
		t.Fatalf("expected no original value preview, got %q", got)
	}
	if candidate.SensitiveRedactionReport.ContainsRawSensitiveValues {
		t.Fatal("redaction report should not contain raw sensitive values")
	}
}
