package registry

import (
	"context"
	"os"
	"testing"
)

func TestPostgresRepositorySkipsWithoutDatabaseURL(t *testing.T) {
	if os.Getenv("WORKFLOW_POSTGRES_TEST_URL") != "" {
		t.Skip("database URL is present; this test only verifies skip behavior")
	}
	t.Skip("set WORKFLOW_POSTGRES_TEST_URL to run postgres repository integration tests")
}

func TestPostgresRepositoryRoundTrip(t *testing.T) {
	databaseURL := os.Getenv("WORKFLOW_POSTGRES_TEST_URL")
	if databaseURL == "" {
		t.Skip("set WORKFLOW_POSTGRES_TEST_URL to run postgres repository integration tests")
	}
	ctx := context.Background()
	repo, err := NewPostgresRepository(ctx, databaseURL)
	if err != nil {
		t.Fatalf("NewPostgresRepository failed: %v", err)
	}
	defer repo.Close()

	recipe := sampleWorkflowRecipe()
	recipe.ID = "wf_postgres_roundtrip"
	recipe.Version = 1
	run := TaskRun{
		ID:              "task_run_postgres_roundtrip",
		ProjectID:       "default",
		Site:            "github.com",
		TaskTemplate:    "Search {{query}}",
		Summary:         "Search issues.",
		OriginalPath:    []string{"repo", "issues"},
		OptimizedPath:   []string{"repo", "issues"},
		MemoryContextID: "ctx_postgres_roundtrip",
		Status:          TaskRunSuccess,
		MemoryEvidenceRefs: []MemoryEvidenceRef{
			{
				ID:           "exp_postgres_roundtrip",
				Source:       MemoryEvidenceSourceExperience,
				Title:        "Open repository issues and search by query",
				Rank:         1,
				Score:        0.94,
				PageStateID:  "github_issues_list",
				MatchedRules: []string{"taskTemplate", "startPageState"},
				Reason:       "Matched task template and entry page.",
			},
		},
	}
	workflowRun := WorkflowRun{
		ID:         "run_postgres_roundtrip",
		ProjectID:  "default",
		WorkflowID: recipe.ID,
		Version:    recipe.Version,
		Task:       "Search timeout",
		URL:        "https://github.com/microsoft/playwright",
		Result:     "succeeded",
	}
	disabled := recipe
	disabled.ID = "wf_postgres_disabled"
	disabled.Status = StatusDisabled

	if err := repo.SaveWorkflow(ctx, recipe); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}
	if err := repo.SaveWorkflow(ctx, disabled); err != nil {
		t.Fatalf("SaveWorkflow disabled failed: %v", err)
	}
	if err := repo.SaveVersion(ctx, WorkflowVersion{
		WorkflowID: recipe.ID,
		Version:    recipe.Version,
		Recipe:     recipe,
		Summary:    "initial version",
	}); err != nil {
		t.Fatalf("SaveVersion failed: %v", err)
	}
	if err := repo.SaveTaskRun(ctx, run); err != nil {
		t.Fatalf("SaveTaskRun failed: %v", err)
	}
	if err := repo.SaveRun(ctx, workflowRun); err != nil {
		t.Fatalf("SaveRun failed: %v", err)
	}
	outbox, err := repo.AppendOutboxEvent(ctx, OutboxEvent{
		Type:           "workflow_approved",
		IdempotencyKey: "workflow_approved:wf_postgres_roundtrip:v1",
		Payload:        map[string]any{"workflowId": recipe.ID},
	})
	if err != nil {
		t.Fatalf("AppendOutboxEvent failed: %v", err)
	}
	profile := BusinessSystemProfile{
		ProjectID: "default",
		Site:      "github.com",
		Summary:   "GitHub workflow test profile.",
		Modules:   []BusinessModule{{Name: "Issues", Purpose: "Search repository issues.", EntryPageStateID: "github_issues_list"}},
	}
	observation := PageObservationEvent{
		ID:                "obs_postgres_roundtrip",
		ProjectID:         "default",
		Site:              "github.com",
		URL:               "https://github.com/microsoft/playwright/issues?q=timeout",
		URLPattern:        "https://github.com/microsoft/playwright/issues",
		Title:             "Issues",
		VisibleTextSample: "Issues list and search field.",
		Controls:          []ControlSignature{{Role: "textbox", Name: "Search"}},
		Fingerprint:       "fp_github_issues",
	}
	experience := ExperienceMemory{
		ID:             "exp_postgres_roundtrip",
		ProjectID:      "default",
		Site:           "github.com",
		TaskTemplate:   "Search {{query}}",
		Intent:         "Search issues",
		Summary:        "Open repository issues and search by query.",
		StartPageState: "github_repo_home",
		EndPageState:   "github_issues_list",
		OptimizedPath:  []string{"github_repo_home", "github_issues_list"},
		StepsSummary:   []ExperienceStepSummary{{ActionName: "Open Issues", TargetName: "Issues"}},
		Variables:      []Variable{{Name: "query", Type: VariableString, Required: true, Source: VariableSourceTask}},
		SuccessCount:   1,
		Searchable:     true,
		ReviewStatus:   ReviewStatusAutoApproved,
	}
	failure := FailureMemory{
		ID:             "fail_postgres_roundtrip",
		ExperienceID:   experience.ID,
		ProjectID:      "default",
		Site:           "github.com",
		PageStateID:    "github_issues_list",
		ActionName:     "Search issues",
		FailureType:    "selector_failed",
		FailureSummary: "Search field was not found.",
		AvoidHint:      "Wait for the issue search field.",
	}
	contextEvent := MemoryContextEvent{
		ID:                    "ctx_postgres_roundtrip",
		ProjectID:             "default",
		Task:                  "Search timeout",
		CurrentURL:            "https://github.com/microsoft/playwright/issues",
		CurrentPageState:      "github_issues_list",
		SelectedExperienceIDs: []string{experience.ID},
		EvidenceRefs: []MemoryEvidenceRef{
			{
				ID:           experience.ID,
				Source:       MemoryEvidenceSourceExperience,
				Title:        "Open repository issues and search by query",
				Rank:         1,
				Score:        0.94,
				PageStateID:  "github_issues_list",
				MatchedRules: []string{"taskTemplate", "startPageState"},
				Reason:       "Matched task template and entry page.",
			},
		},
		RecommendedMode: MemoryModeGuided,
	}
	review := MemoryReview{
		ID:         "review_postgres_roundtrip",
		ProjectID:  "default",
		TargetType: ReviewTargetExperience,
		TargetID:   experience.ID,
		Status:     ReviewStatusPending,
		Summary:    "Review issue search experience.",
	}
	if err := repo.SaveBusinessSystemProfile(ctx, profile); err != nil {
		t.Fatalf("SaveBusinessSystemProfile failed: %v", err)
	}
	if err := repo.SavePageObservationEvent(ctx, observation); err != nil {
		t.Fatalf("SavePageObservationEvent failed: %v", err)
	}
	if err := repo.SaveExperienceMemory(ctx, experience); err != nil {
		t.Fatalf("SaveExperienceMemory failed: %v", err)
	}
	if err := repo.SaveFailureMemory(ctx, failure); err != nil {
		t.Fatalf("SaveFailureMemory failed: %v", err)
	}
	if err := repo.SaveMemoryContextEvent(ctx, contextEvent); err != nil {
		t.Fatalf("SaveMemoryContextEvent failed: %v", err)
	}
	if err := repo.SaveMemoryReview(ctx, review); err != nil {
		t.Fatalf("SaveMemoryReview failed: %v", err)
	}

	got, err := repo.GetWorkflow(ctx, recipe.ID)
	if err != nil {
		t.Fatalf("GetWorkflow failed: %v", err)
	}
	if got.ID != recipe.ID || got.Version != recipe.Version {
		t.Fatalf("unexpected workflow: %#v", got)
	}
	active, err := repo.ListActiveWorkflows(ctx, "default")
	if err != nil {
		t.Fatalf("ListActiveWorkflows failed: %v", err)
	}
	if !containsWorkflow(active, recipe.ID) || containsWorkflow(active, disabled.ID) {
		t.Fatalf("expected active searchable workflows only, got %#v", active)
	}
	version, err := repo.GetVersion(ctx, recipe.ID, recipe.Version)
	if err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}
	if version.Summary != "initial version" {
		t.Fatalf("unexpected version: %#v", version)
	}
	versions, err := repo.ListVersions(ctx, recipe.ID)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 1 || versions[0].Version != recipe.Version {
		t.Fatalf("unexpected versions: %#v", versions)
	}
	gotRun, err := repo.GetTaskRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetTaskRun failed: %v", err)
	}
	if gotRun.OptimizedPath[1] != "issues" ||
		gotRun.MemoryContextID != "ctx_postgres_roundtrip" ||
		len(gotRun.MemoryEvidenceRefs) != 1 ||
		gotRun.MemoryEvidenceRefs[0].Source != MemoryEvidenceSourceExperience {
		t.Fatalf("unexpected task run: %#v", gotRun)
	}
	gotWorkflowRun, err := repo.GetRun(ctx, workflowRun.ID)
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}
	if gotWorkflowRun.WorkflowID != recipe.ID {
		t.Fatalf("unexpected workflow run: %#v", gotWorkflowRun)
	}
	outboxEvents, err := repo.ListOutboxEvents(ctx, StatusPendingReview)
	if err != nil {
		t.Fatalf("ListOutboxEvents failed: %v", err)
	}
	if !containsOutboxEvent(outboxEvents, outbox.ID) {
		t.Fatalf("expected outbox event %#v in %#v", outbox, outboxEvents)
	}
	gotProfile, err := repo.GetBusinessSystemProfile(ctx, BusinessSystemProfileQuery{
		ProjectID:  "default",
		SourceType: MemorySourceProduction,
	})
	if err != nil {
		t.Fatalf("GetBusinessSystemProfile failed: %v", err)
	}
	if gotProfile.Modules[0].Name != "Issues" {
		t.Fatalf("unexpected profile: %#v", gotProfile)
	}
	observations, err := repo.ListPageObservationEvents(ctx, PageObservationEventListQuery{ProjectID: "default", Site: "github.com"})
	if err != nil {
		t.Fatalf("ListPageObservationEvents failed: %v", err)
	}
	if !containsObservation(observations, observation.ID) {
		t.Fatalf("expected observation in %#v", observations)
	}
	experiences, err := repo.SearchExperienceMemories(ctx, ExperienceMemorySearchQuery{ProjectID: "default", Site: "github.com", Task: "Search issues", Limit: 3})
	if err != nil {
		t.Fatalf("SearchExperienceMemories failed: %v", err)
	}
	if !containsExperience(experiences, experience.ID) {
		t.Fatalf("expected experience in %#v", experiences)
	}
	failures, err := repo.SearchFailureMemories(ctx, FailureMemorySearchQuery{ProjectID: "default", Site: "github.com", PageStateID: "github_issues_list", Limit: 3})
	if err != nil {
		t.Fatalf("SearchFailureMemories failed: %v", err)
	}
	if len(failures) == 0 || failures[0].ID != failure.ID {
		t.Fatalf("unexpected failures: %#v", failures)
	}
	gotContext, err := repo.GetMemoryContextEvent(ctx, contextEvent.ID)
	if err != nil {
		t.Fatalf("GetMemoryContextEvent failed: %v", err)
	}
	if gotContext.RecommendedMode != MemoryModeGuided ||
		len(gotContext.EvidenceRefs) != 1 ||
		gotContext.EvidenceRefs[0].ID != experience.ID {
		t.Fatalf("unexpected context event: %#v", gotContext)
	}
	if err := repo.ApproveMemoryReview(ctx, review.ID); err != nil {
		t.Fatalf("ApproveMemoryReview failed: %v", err)
	}
	reviews, err := repo.ListMemoryReviews(ctx, MemoryReviewListQuery{ProjectID: "default", Status: ReviewStatusApproved})
	if err != nil {
		t.Fatalf("ListMemoryReviews failed: %v", err)
	}
	if len(reviews) == 0 || reviews[0].ID != review.ID {
		t.Fatalf("unexpected reviews: %#v", reviews)
	}
}

func TestPostgresRepositoryMemoryAttributionEventsRoundTrip(t *testing.T) {
	databaseURL := os.Getenv("WORKFLOW_POSTGRES_TEST_URL")
	if databaseURL == "" {
		t.Skip("set WORKFLOW_POSTGRES_TEST_URL to run postgres repository integration tests")
	}
	ctx := context.Background()
	repo, err := NewPostgresRepository(ctx, databaseURL)
	if err != nil {
		t.Fatalf("NewPostgresRepository failed: %v", err)
	}
	defer repo.Close()

	event := MemoryAttributionEvent{
		ID:             "attr_postgres_helpful",
		ProjectID:      "default",
		Site:           "github.com",
		TaskRunID:      "task_run_postgres_roundtrip",
		ContextID:      "ctx_postgres_roundtrip",
		EvidenceID:     "exp_postgres_roundtrip",
		EvidenceSource: MemoryEvidenceSourceExperience,
		Label:          MemoryAttributionHelpful,
		Reason:         "Execution followed the recalled experience.",
		Signals:        []string{"path_aligned", "target_aligned"},
	}

	if err := repo.SaveMemoryAttributionEvent(ctx, event); err != nil {
		t.Fatalf("SaveMemoryAttributionEvent failed: %v", err)
	}

	events, err := repo.ListMemoryAttributionEvents(ctx, MemoryAttributionEventListQuery{
		ProjectID: "default",
		TaskRunID: "task_run_postgres_roundtrip",
		ContextID: "ctx_postgres_roundtrip",
	})
	if err != nil {
		t.Fatalf("ListMemoryAttributionEvents failed: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected attribution events")
	}
	found := false
	for _, got := range events {
		if got.ID == event.ID {
			found = true
			if got.EvidenceID != event.EvidenceID || got.Label != MemoryAttributionHelpful {
				t.Fatalf("unexpected attribution event: %#v", got)
			}
		}
	}
	if !found {
		t.Fatalf("expected event %q in %#v", event.ID, events)
	}
}

func TestPostgresRepositoryMemoryEvidenceStatsUpsert(t *testing.T) {
	databaseURL := os.Getenv("WORKFLOW_POSTGRES_TEST_URL")
	if databaseURL == "" {
		t.Skip("set WORKFLOW_POSTGRES_TEST_URL to run postgres repository integration tests")
	}
	ctx := context.Background()
	repo, err := NewPostgresRepository(ctx, databaseURL)
	if err != nil {
		t.Fatalf("NewPostgresRepository failed: %v", err)
	}
	defer repo.Close()

	stats := MemoryEvidenceStats{
		ProjectID:       "default",
		Site:            "github.com",
		EvidenceID:      "exp_postgres_roundtrip",
		EvidenceSource:  MemoryEvidenceSourceExperience,
		HelpfulCount:    1,
		UnusedCount:     0,
		MisleadingCount: 0,
		StaleCount:      0,
		NeutralCount:    0,
		UtilityScore:    0.5,
	}

	if err := repo.SaveMemoryEvidenceStats(ctx, stats); err != nil {
		t.Fatalf("SaveMemoryEvidenceStats first failed: %v", err)
	}
	stats.HelpfulCount = 4
	stats.UtilityScore = 0.95
	if err := repo.SaveMemoryEvidenceStats(ctx, stats); err != nil {
		t.Fatalf("SaveMemoryEvidenceStats second failed: %v", err)
	}

	got, err := repo.GetMemoryEvidenceStats(ctx, "default", MemoryEvidenceSourceExperience, "exp_postgres_roundtrip")
	if err != nil {
		t.Fatalf("GetMemoryEvidenceStats failed: %v", err)
	}
	if got.HelpfulCount != 4 || got.UtilityScore != 0.95 {
		t.Fatalf("unexpected memory evidence stats: %#v", got)
	}

	list, err := repo.ListMemoryEvidenceStats(ctx, MemoryEvidenceStatsListQuery{
		ProjectID:      "default",
		Site:           "github.com",
		EvidenceSource: MemoryEvidenceSourceExperience,
	})
	if err != nil {
		t.Fatalf("ListMemoryEvidenceStats failed: %v", err)
	}
	if len(list) == 0 {
		t.Fatal("expected evidence stats")
	}
}

func containsWorkflow(workflows []WorkflowRecipe, id string) bool {
	for _, workflow := range workflows {
		if workflow.ID == id {
			return true
		}
	}
	return false
}

func containsOutboxEvent(events []OutboxEvent, id string) bool {
	for _, event := range events {
		if event.ID == id {
			return true
		}
	}
	return false
}

func containsObservation(events []PageObservationEvent, id string) bool {
	for _, event := range events {
		if event.ID == id {
			return true
		}
	}
	return false
}

func containsExperience(memories []ExperienceMemory, id string) bool {
	for _, memory := range memories {
		if memory.ID == id {
			return true
		}
	}
	return false
}
