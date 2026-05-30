package registry

import (
	"context"
	"path/filepath"
	"testing"
)

func TestFileRepositoryPersistsAcrossRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo, err := NewFileRepository(dir)
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	recipe := sampleWorkflowRecipe()

	if err := repo.SaveWorkflow(ctx, recipe); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}
	if err := repo.SaveVersion(ctx, WorkflowVersion{
		WorkflowID: recipe.ID,
		Version:    recipe.Version,
		Recipe:     recipe,
		Summary:    "initial version",
	}); err != nil {
		t.Fatalf("SaveVersion failed: %v", err)
	}

	restarted, err := NewFileRepository(dir)
	if err != nil {
		t.Fatalf("restart NewFileRepository failed: %v", err)
	}
	got, err := restarted.GetWorkflow(ctx, recipe.ID)
	if err != nil {
		t.Fatalf("GetWorkflow after restart failed: %v", err)
	}
	if got.ID != recipe.ID || got.Version != recipe.Version {
		t.Fatalf("unexpected workflow after restart: %#v", got)
	}
	version, err := restarted.GetVersion(ctx, recipe.ID, recipe.Version)
	if err != nil {
		t.Fatalf("GetVersion after restart failed: %v", err)
	}
	if version.Summary != "initial version" {
		t.Fatalf("unexpected version after restart: %#v", version)
	}
}

func TestFileRepositoryListsOnlyActiveSearchableWorkflows(t *testing.T) {
	ctx := context.Background()
	repo, err := NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	active := sampleWorkflowRecipe()
	disabled := sampleWorkflowRecipe()
	disabled.ID = "wf_disabled"
	disabled.Status = StatusDisabled

	if err := repo.SaveWorkflow(ctx, active); err != nil {
		t.Fatalf("SaveWorkflow active failed: %v", err)
	}
	if err := repo.SaveWorkflow(ctx, disabled); err != nil {
		t.Fatalf("SaveWorkflow disabled failed: %v", err)
	}

	workflows, err := repo.ListActiveWorkflows(ctx, "default")
	if err != nil {
		t.Fatalf("ListActiveWorkflows failed: %v", err)
	}
	if len(workflows) != 1 || workflows[0].ID != active.ID {
		t.Fatalf("expected only active workflow, got %#v", workflows)
	}
}

func TestFileRepositoryOutboxIdempotency(t *testing.T) {
	ctx := context.Background()
	repo, err := NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	event := OutboxEvent{
		Type:           "workflow_approved",
		IdempotencyKey: "workflow_approved:wf_github_issue_search:v3",
		Payload:        map[string]any{"workflowId": "wf_github_issue_search"},
	}

	first, err := repo.AppendOutboxEvent(ctx, event)
	if err != nil {
		t.Fatalf("AppendOutboxEvent first failed: %v", err)
	}
	second, err := repo.AppendOutboxEvent(ctx, event)
	if err != nil {
		t.Fatalf("AppendOutboxEvent second failed: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("idempotent event should reuse ID, got %q and %q", first.ID, second.ID)
	}

	events, err := repo.ListOutboxEvents(ctx, StatusPendingReview)
	if err != nil {
		t.Fatalf("ListOutboxEvents failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one event, got %#v", events)
	}
}

func TestFileRepositoryPersistsRunLogsAndRedactedVariables(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo, err := NewFileRepository(dir)
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	run := WorkflowRun{
		ID:         "run_1",
		ProjectID:  "default",
		WorkflowID: "wf_github_issue_search",
		Version:    1,
		Task:       "search timeout",
		URL:        "https://github.com/microsoft/playwright",
		Variables: map[string]string{
			"query": "[redacted]",
		},
		Result: "succeeded",
		Logs: []WorkflowRunLog{
			{
				Event:     "chunk_started",
				ChunkID:   "open_issues",
				PageState: "github_repo_home",
			},
		},
	}

	if err := repo.SaveRun(ctx, run); err != nil {
		t.Fatalf("SaveRun failed: %v", err)
	}
	restarted, err := NewFileRepository(dir)
	if err != nil {
		t.Fatalf("restart NewFileRepository failed: %v", err)
	}
	got, err := restarted.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun after restart failed: %v", err)
	}
	if got.Variables["query"] != "[redacted]" {
		t.Fatalf("expected redacted variable, got %#v", got.Variables)
	}
	if len(got.Logs) != 1 || got.Logs[0].Event != "chunk_started" {
		t.Fatalf("expected persisted run log, got %#v", got.Logs)
	}
}

func TestFileRepositoryUpsertsSelectorStats(t *testing.T) {
	ctx := context.Background()
	repo, err := NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	stats := SelectorStats{
		WorkflowID:   "wf_github_issue_search",
		Version:      1,
		StepID:       "click_issues",
		Strategy:     "role",
		Selector:     "link:Issues",
		SuccessCount: 1,
	}
	if err := repo.SaveSelectorStats(ctx, stats); err != nil {
		t.Fatalf("SaveSelectorStats first failed: %v", err)
	}
	stats.SuccessCount = 2
	stats.FailCount = 1
	if err := repo.SaveSelectorStats(ctx, stats); err != nil {
		t.Fatalf("SaveSelectorStats second failed: %v", err)
	}
	got, err := repo.ListSelectorStats(ctx, "wf_github_issue_search", 1)
	if err != nil {
		t.Fatalf("ListSelectorStats failed: %v", err)
	}
	if len(got) != 1 || got[0].SuccessCount != 2 || got[0].FailCount != 1 {
		t.Fatalf("expected upserted stats, got %#v", got)
	}
}

func TestFileRepositoryPersistsInterruptHandlersAndRepairPatches(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo, err := NewFileRepository(dir)
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	handler := InterruptHandler{
		ID:            "ih_dialog",
		ProjectID:     "default",
		Site:          "github.com",
		Status:        StatusActive,
		InterruptType: "modal",
		RiskLevel:     RiskReadOnly,
		WorkflowID:    "wf_close_dialog",
		Version:       1,
	}
	patch := RepairPatch{
		ID:              "patch_search",
		ProjectID:       "default",
		Status:          StatusActive,
		WorkflowID:      "wf_github_issue_search",
		WorkflowVersion: 3,
		ChunkID:         "search_issues",
		StepID:          "fill_query",
		Site:            "github.com",
		FailureType:     "locator_not_found",
		RiskLevel:       RiskReadOnly,
	}

	if err := repo.SaveInterruptHandler(ctx, handler); err != nil {
		t.Fatalf("SaveInterruptHandler failed: %v", err)
	}
	if err := repo.SaveRepairPatch(ctx, patch); err != nil {
		t.Fatalf("SaveRepairPatch failed: %v", err)
	}
	restarted, err := NewFileRepository(dir)
	if err != nil {
		t.Fatalf("restart NewFileRepository failed: %v", err)
	}
	handlers, err := restarted.ListActiveInterruptHandlers(ctx, "default")
	if err != nil {
		t.Fatalf("ListActiveInterruptHandlers failed: %v", err)
	}
	patches, err := restarted.ListActiveRepairPatches(ctx, "default")
	if err != nil {
		t.Fatalf("ListActiveRepairPatches failed: %v", err)
	}
	if len(handlers) != 1 || handlers[0].ID != handler.ID {
		t.Fatalf("unexpected handlers: %#v", handlers)
	}
	if len(patches) != 1 || patches[0].ID != patch.ID {
		t.Fatalf("unexpected patches: %#v", patches)
	}
}

func TestFileRepositoryPersistsTaskRunsAndPageGraph(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo, err := NewFileRepository(dir)
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	run := TaskRun{
		ID:              "task_run_1",
		ProjectID:       "default",
		Site:            "ops.example.com",
		TaskTemplate:    "查看 {{service_name}} 运行状态",
		Summary:         "查看服务运行状态。",
		OriginalPath:    []string{"page_a", "page_b", "page_c", "page_a", "page_d"},
		OptimizedPath:   []string{"page_a", "page_d"},
		MemoryContextID: "ctx_service_status",
		Status:          TaskRunSuccess,
		MemoryEvidenceRefs: []MemoryEvidenceRef{
			{
				ID:           "exp_service_status",
				Source:       MemoryEvidenceSourceExperience,
				Title:        "Search service and open details",
				Rank:         1,
				Score:        0.92,
				PageStateID:  "page_a",
				MatchedRules: []string{"taskTemplate", "startPageState"},
				Reason:       "Matched task template and start page.",
			},
		},
		ActionSteps: []ActionStep{
			{
				ID:               "step_1",
				PageStateID:      "page_a",
				StepIndex:        1,
				ActionType:       StepFill,
				TargetName:       "服务名称搜索框",
				ValueTemplate:    "{{service_name}}",
				ReasoningSummary: "使用固定搜索框定位服务。",
				ResultSummary:    "搜索已提交。",
			},
		},
	}
	page := PageState{
		ID:             "page_a",
		ProjectID:      "default",
		Site:           "ops.example.com",
		URLPattern:     "https://ops.example.com/service*",
		CanonicalTitle: "服务管理",
		RequiredText:   []string{"服务管理"},
		Status:         StatusActive,
	}
	transition := PageTransition{
		ID:            "transition_a_d",
		ProjectID:     "default",
		Site:          "ops.example.com",
		FromPageState: "page_a",
		ToPageState:   "page_d",
		WorkflowID:    "wf_service_status",
		Version:       1,
		ChunkID:       "chunk_open_detail",
	}

	if err := repo.SaveTaskRun(ctx, run); err != nil {
		t.Fatalf("SaveTaskRun failed: %v", err)
	}
	if err := repo.SavePageState(ctx, page); err != nil {
		t.Fatalf("SavePageState failed: %v", err)
	}
	if err := repo.SavePageTransition(ctx, transition); err != nil {
		t.Fatalf("SavePageTransition failed: %v", err)
	}

	restarted, err := NewFileRepository(dir)
	if err != nil {
		t.Fatalf("restart NewFileRepository failed: %v", err)
	}
	gotRun, err := restarted.GetTaskRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetTaskRun failed: %v", err)
	}
	if gotRun.OptimizedPath[1] != "page_d" ||
		gotRun.ActionSteps[0].ValueTemplate != "{{service_name}}" ||
		gotRun.MemoryContextID != "ctx_service_status" ||
		len(gotRun.MemoryEvidenceRefs) != 1 ||
		gotRun.MemoryEvidenceRefs[0].Source != MemoryEvidenceSourceExperience {
		t.Fatalf("unexpected persisted task run: %#v", gotRun)
	}
	pages, err := restarted.ListPageStates(ctx, PageStateListQuery{ProjectID: "default", Site: "ops.example.com"})
	if err != nil {
		t.Fatalf("ListPageStates failed: %v", err)
	}
	if len(pages) != 1 || pages[0].ID != page.ID {
		t.Fatalf("unexpected pages: %#v", pages)
	}
	transitions, err := restarted.ListPageTransitions(ctx, PageTransitionListQuery{ProjectID: "default", FromPageStateID: "page_a"})
	if err != nil {
		t.Fatalf("ListPageTransitions failed: %v", err)
	}
	if len(transitions) != 1 || transitions[0].ID != transition.ID {
		t.Fatalf("unexpected transitions: %#v", transitions)
	}
}

func TestFileRepositorySearchesKnowledgeChunks(t *testing.T) {
	ctx := context.Background()
	repo, err := NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	document := KnowledgeDocument{
		ID:        "doc_service",
		ProjectID: "default",
		Title:     "服务管理手册",
		Source:    "manual",
		Content:   "服务管理页可通过服务名称搜索框定位服务，状态列表示当前运行状态。",
		Tags:      []string{"service", "status"},
	}
	chunk := KnowledgeChunk{
		ID:         "chunk_service",
		DocumentID: document.ID,
		ProjectID:  "default",
		Title:      document.Title,
		Source:     document.Source,
		ChunkText:  document.Content,
		Tags:       document.Tags,
	}
	if err := repo.SaveKnowledgeDocument(ctx, document); err != nil {
		t.Fatalf("SaveKnowledgeDocument failed: %v", err)
	}
	if err := repo.SaveKnowledgeChunks(ctx, []KnowledgeChunk{chunk}); err != nil {
		t.Fatalf("SaveKnowledgeChunks failed: %v", err)
	}

	hits, err := repo.SearchKnowledgeChunks(ctx, KnowledgeSearchQuery{
		ProjectID: "default",
		Task:      "查看服务状态",
		Title:     "服务管理",
		Limit:     3,
	})
	if err != nil {
		t.Fatalf("SearchKnowledgeChunks failed: %v", err)
	}
	if len(hits) != 1 || hits[0].ID != chunk.ID || hits[0].Score <= 0 {
		t.Fatalf("unexpected knowledge hits: %#v", hits)
	}
}

func TestKnowledgeChunkScopeMatchesLocalhostDynamicPortAndFragment(t *testing.T) {
	chunk := KnowledgeChunk{
		ID:        "chunk_service",
		ProjectID: "default",
		Title:     "服务健康巡检手册",
		ChunkText: "从 WebOps 控制台进入服务管理，输入服务名称并点击查询服务。",
		Metadata: map[string]any{
			"url": "http://127.0.0.1/console#services",
		},
	}

	if !knowledgeChunkScopeMatches(chunk, "http://127.0.0.1:55272/console") {
		t.Fatal("expected localhost document scope to match dynamic-port console URL")
	}
	if knowledgeChunkScopeMatches(chunk, "http://127.0.0.1:55272/settings") {
		t.Fatal("document scope should not match unrelated local path")
	}
}

func TestFileRepositoryPersistsMemoryV2Records(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo, err := NewFileRepository(dir)
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}

	profile := BusinessSystemProfile{
		ProjectID: "default",
		Site:      "ops.example.com",
		Summary:   "Service operations system.",
		Modules:   []BusinessModule{{Name: "Service Management", Purpose: "Query service status.", EntryPageStateID: "page_service_list"}},
		Terms:     map[string]string{"service": "Managed runtime component."},
	}
	observation := PageObservationEvent{
		ID:                "obs_service_list",
		ProjectID:         "default",
		Site:              "ops.example.com",
		URL:               "https://ops.example.com/services?k=kme-prod-001",
		URLPattern:        "https://ops.example.com/services",
		Title:             "Service Management",
		VisibleTextSample: "Service list and status filter.",
		Controls:          []ControlSignature{{Role: "textbox", Name: "Service name"}},
		Fingerprint:       "fp_service_list",
	}
	experience := ExperienceMemory{
		ID:             "exp_service_status",
		ProjectID:      "default",
		Site:           "ops.example.com",
		TaskTemplate:   "Check {{service_name}} status",
		Intent:         "Check service status",
		Summary:        "Search service and open details.",
		StartPageState: "page_service_list",
		EndPageState:   "page_service_detail",
		OptimizedPath:  []string{"page_service_list", "page_service_detail"},
		StepsSummary:   []ExperienceStepSummary{{ActionName: "Search service", TargetName: "Service name", ValueTemplate: "{{service_name}}"}},
		Variables:      []Variable{{Name: "service_name", Type: VariableString, Required: true, Source: VariableSourceTask}},
		SuccessCount:   1,
		Searchable:     true,
		ReviewStatus:   ReviewStatusAutoApproved,
	}
	failure := FailureMemory{
		ID:             "fail_service_status",
		ExperienceID:   experience.ID,
		ProjectID:      "default",
		Site:           "ops.example.com",
		PageStateID:    "page_service_list",
		ActionName:     "Search service",
		FailureType:    "selector_failed",
		FailureSummary: "Search input was not visible.",
		AvoidHint:      "Wait for the service list search controls before filling.",
	}
	contextEvent := MemoryContextEvent{
		ID:                        "ctx_service_status",
		ProjectID:                 "default",
		Task:                      "Check kme-prod-001 status",
		CurrentURL:                "https://ops.example.com/services",
		CurrentPageState:          "page_service_list",
		SelectedExperienceIDs:     []string{experience.ID},
		SelectedKnowledgeChunkIDs: []string{"chunk_service"},
		EvidenceRefs: []MemoryEvidenceRef{
			{
				ID:           experience.ID,
				Source:       MemoryEvidenceSourceExperience,
				Title:        "Search service and open details",
				Rank:         1,
				Score:        0.97,
				PageStateID:  "page_service_list",
				MatchedRules: []string{"taskTemplate"},
				Reason:       "Strong task and page state match.",
			},
		},
		RecommendedMode: MemoryModeGuided,
	}
	review := MemoryReview{
		ID:         "review_exp",
		ProjectID:  "default",
		TargetType: ReviewTargetExperience,
		TargetID:   experience.ID,
		Status:     ReviewStatusPending,
		Summary:    "Review service status experience.",
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

	restarted, err := NewFileRepository(dir)
	if err != nil {
		t.Fatalf("restart NewFileRepository failed: %v", err)
	}
	gotProfile, err := restarted.GetBusinessSystemProfile(ctx, BusinessSystemProfileQuery{
		ProjectID:  "default",
		SourceType: MemorySourceProduction,
	})
	if err != nil {
		t.Fatalf("GetBusinessSystemProfile failed: %v", err)
	}
	if gotProfile.Modules[0].EntryPageStateID != "page_service_list" {
		t.Fatalf("unexpected profile: %#v", gotProfile)
	}
	events, err := restarted.ListPageObservationEvents(ctx, PageObservationEventListQuery{ProjectID: "default", Site: "ops.example.com"})
	if err != nil {
		t.Fatalf("ListPageObservationEvents failed: %v", err)
	}
	if len(events) != 1 || events[0].Fingerprint != observation.Fingerprint {
		t.Fatalf("unexpected observations: %#v", events)
	}
	experienceHits, err := restarted.SearchExperienceMemories(ctx, ExperienceMemorySearchQuery{
		ProjectID: "default",
		Site:      "ops.example.com",
		Task:      "Check service status",
		Limit:     3,
	})
	if err != nil {
		t.Fatalf("SearchExperienceMemories failed: %v", err)
	}
	if len(experienceHits) != 1 || experienceHits[0].ID != experience.ID {
		t.Fatalf("unexpected experience hits: %#v", experienceHits)
	}
	failures, err := restarted.SearchFailureMemories(ctx, FailureMemorySearchQuery{ProjectID: "default", Site: "ops.example.com", PageStateID: "page_service_list", Limit: 3})
	if err != nil {
		t.Fatalf("SearchFailureMemories failed: %v", err)
	}
	if len(failures) != 1 || failures[0].ID != failure.ID {
		t.Fatalf("unexpected failure hits: %#v", failures)
	}
	gotContext, err := restarted.GetMemoryContextEvent(ctx, contextEvent.ID)
	if err != nil {
		t.Fatalf("GetMemoryContextEvent failed: %v", err)
	}
	if gotContext.RecommendedMode != MemoryModeGuided ||
		len(gotContext.EvidenceRefs) != 1 ||
		gotContext.EvidenceRefs[0].ID != experience.ID {
		t.Fatalf("unexpected context event: %#v", gotContext)
	}
	reviews, err := restarted.ListMemoryReviews(ctx, MemoryReviewListQuery{ProjectID: "default", Status: ReviewStatusPending})
	if err != nil {
		t.Fatalf("ListMemoryReviews failed: %v", err)
	}
	if len(reviews) != 1 || reviews[0].ID != review.ID {
		t.Fatalf("unexpected reviews: %#v", reviews)
	}
	if err := restarted.ApproveMemoryReview(ctx, review.ID); err != nil {
		t.Fatalf("ApproveMemoryReview failed: %v", err)
	}
	approved, err := restarted.ListMemoryReviews(ctx, MemoryReviewListQuery{ProjectID: "default", Status: ReviewStatusApproved})
	if err != nil {
		t.Fatalf("ListMemoryReviews approved failed: %v", err)
	}
	if len(approved) != 1 || approved[0].ReviewedAt.IsZero() {
		t.Fatalf("unexpected approved reviews: %#v", approved)
	}
}

func TestNewFileRepositoryCreatesDataDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "data")

	if _, err := NewFileRepository(dir); err != nil {
		t.Fatalf("NewFileRepository should create data dir: %v", err)
	}
}

func TestFileRepositoryPersistsMemoryAttributionEventsAndEvidenceStatsAcrossRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo, err := NewFileRepository(dir)
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}

	event := MemoryAttributionEvent{
		ID:             "attr_exp_helpful",
		ProjectID:      "default",
		Site:           "ops.example.com",
		TaskRunID:      "task_run_1",
		ContextID:      "ctx_service_status",
		EvidenceID:     "exp_service_status",
		EvidenceSource: MemoryEvidenceSourceExperience,
		Label:          MemoryAttributionHelpful,
		Reason:         "Execution followed the recalled path.",
		Signals:        []string{"path_aligned", "target_aligned"},
	}
	stats := MemoryEvidenceStats{
		ProjectID:       "default",
		Site:            "ops.example.com",
		EvidenceID:      "exp_service_status",
		EvidenceSource:  MemoryEvidenceSourceExperience,
		HelpfulCount:    2,
		UnusedCount:     1,
		MisleadingCount: 0,
		StaleCount:      0,
		NeutralCount:    1,
		UtilityScore:    0.8,
	}

	if err := repo.SaveMemoryAttributionEvent(ctx, event); err != nil {
		t.Fatalf("SaveMemoryAttributionEvent failed: %v", err)
	}
	if err := repo.SaveMemoryEvidenceStats(ctx, stats); err != nil {
		t.Fatalf("SaveMemoryEvidenceStats failed: %v", err)
	}

	restarted, err := NewFileRepository(dir)
	if err != nil {
		t.Fatalf("restart NewFileRepository failed: %v", err)
	}
	events, err := restarted.ListMemoryAttributionEvents(ctx, MemoryAttributionEventListQuery{
		ProjectID: "default",
		TaskRunID: "task_run_1",
		ContextID: "ctx_service_status",
	})
	if err != nil {
		t.Fatalf("ListMemoryAttributionEvents failed: %v", err)
	}
	if len(events) != 1 || events[0].EvidenceID != "exp_service_status" || events[0].Label != MemoryAttributionHelpful {
		t.Fatalf("unexpected attribution events: %#v", events)
	}
	gotStats, err := restarted.GetMemoryEvidenceStats(ctx, "default", MemoryEvidenceSourceExperience, "exp_service_status")
	if err != nil {
		t.Fatalf("GetMemoryEvidenceStats failed: %v", err)
	}
	if gotStats.HelpfulCount != 2 || gotStats.UtilityScore != 0.8 {
		t.Fatalf("unexpected persisted evidence stats: %#v", gotStats)
	}
}

func TestFileRepositoryUpsertsMemoryEvidenceStats(t *testing.T) {
	ctx := context.Background()
	repo, err := NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}

	stats := MemoryEvidenceStats{
		ProjectID:       "default",
		Site:            "ops.example.com",
		EvidenceID:      "exp_service_status",
		EvidenceSource:  MemoryEvidenceSourceExperience,
		HelpfulCount:    1,
		UnusedCount:     0,
		MisleadingCount: 0,
		StaleCount:      0,
		NeutralCount:    0,
		UtilityScore:    0.4,
	}
	if err := repo.SaveMemoryEvidenceStats(ctx, stats); err != nil {
		t.Fatalf("SaveMemoryEvidenceStats first failed: %v", err)
	}
	stats.HelpfulCount = 3
	stats.UtilityScore = 0.9
	if err := repo.SaveMemoryEvidenceStats(ctx, stats); err != nil {
		t.Fatalf("SaveMemoryEvidenceStats second failed: %v", err)
	}

	list, err := repo.ListMemoryEvidenceStats(ctx, MemoryEvidenceStatsListQuery{
		ProjectID:      "default",
		Site:           "ops.example.com",
		EvidenceSource: MemoryEvidenceSourceExperience,
	})
	if err != nil {
		t.Fatalf("ListMemoryEvidenceStats failed: %v", err)
	}
	if len(list) != 1 || list[0].HelpfulCount != 3 || list[0].UtilityScore != 0.9 {
		t.Fatalf("expected one upserted evidence stats record, got %#v", list)
	}
}
