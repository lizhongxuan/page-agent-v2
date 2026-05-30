package registry

import (
	"context"
	"strings"
	"testing"
)

func TestCandidateServiceCreatesPendingCandidate(t *testing.T) {
	ctx := context.Background()
	repo, err := NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewCandidateService(repo)

	candidate, err := service.CreateFromSession(ctx, RecordedSession{
		ProjectID: "default",
		Source:    "user_demo",
		Task:      "Search GitHub issues",
		StartURL:  "https://github.com/alibaba/page-agent",
		Recipe:    sampleWorkflowRecipe(),
	})

	if err != nil {
		t.Fatalf("CreateFromSession failed: %v", err)
	}
	if candidate.Status != StatusPendingReview || candidate.Searchable {
		t.Fatalf("candidate should be pending and non-searchable: %#v", candidate)
	}

	workflows, err := repo.ListActiveWorkflows(ctx, "default")
	if err != nil {
		t.Fatalf("ListActiveWorkflows failed: %v", err)
	}
	if len(workflows) != 0 {
		t.Fatalf("pending candidate should not create active workflow: %#v", workflows)
	}
}

func TestCandidateServiceCreatesCandidateFromTaskRunOptimizedPath(t *testing.T) {
	ctx := context.Background()
	repo, err := NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	run := TaskRun{
		ID:            "task_run_1",
		ProjectID:     "default",
		Site:          "ops.example.com",
		TaskTemplate:  "查看 {{service_name}} 运行状态",
		Summary:       "查看服务运行状态。",
		OriginalPath:  []string{"page_a", "page_b", "page_c", "page_a", "page_d"},
		OptimizedPath: []string{"page_a", "page_d"},
		Status:        TaskRunSuccess,
		ActionSteps: []ActionStep{
			{
				ID:            "step_fill",
				PageStateID:   "page_a",
				StepIndex:     1,
				ActionType:    StepFill,
				TargetName:    "服务名称搜索框",
				ValueTemplate: "{{service_name}}",
			},
			{
				ID:            "step_noise",
				PageStateID:   "page_b",
				StepIndex:     2,
				ActionType:    StepClick,
				TargetName:    "错误分支",
				IsBranchNoise: true,
			},
			{
				ID:            "step_pseudo_failed_navigation",
				PageStateID:   "page_d",
				StepIndex:     3,
				ActionType:    StepClick,
				TargetName:    "服务管理",
				ResultSummary: "❌ Failed to click element: Error: DOM tree not indexed yet. Can not perform actions on elements.",
			},
		},
	}
	if err := repo.SaveTaskRun(ctx, run); err != nil {
		t.Fatalf("SaveTaskRun failed: %v", err)
	}
	service := NewCandidateService(repo)

	candidate, err := service.CreateFromTaskRun(ctx, run.ID, "task_run")

	if err != nil {
		t.Fatalf("CreateFromTaskRun failed: %v", err)
	}
	if candidate.Status != StatusPendingReview || candidate.Searchable {
		t.Fatalf("candidate should be pending and non-searchable: %#v", candidate)
	}
	if candidate.SourceTaskRunID != run.ID {
		t.Fatalf("expected source task run id, got %#v", candidate)
	}
	if candidate.RecipeDraft.StartPageStates[0] != "page_a" || candidate.RecipeDraft.EndPageStates[0] != "page_d" {
		t.Fatalf("expected optimized path page states, got %#v", candidate.RecipeDraft)
	}
	if len(candidate.RecipeDraft.Chunks[0].Steps) != 1 || candidate.RecipeDraft.Chunks[0].Steps[0].Value != "{{service_name}}" {
		t.Fatalf("expected templated non-noise workflow step, got %#v", candidate.RecipeDraft.Chunks[0].Steps)
	}

	approved, err := service.Approve(ctx, candidate.ID)
	if err != nil {
		t.Fatalf("Approve failed: %v", err)
	}
	version, err := repo.GetVersion(ctx, approved.RecipeDraft.ID, approved.RecipeDraft.Version)
	if err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}
	if version.Summary != "approved candidate "+candidate.ID+" with optimized route page_a -> page_d" {
		t.Fatalf("expected optimized route summary, got %q", version.Summary)
	}
	states, err := repo.ListPageStates(ctx, PageStateListQuery{ProjectID: "default", Site: "ops.example.com"})
	if err != nil {
		t.Fatalf("ListPageStates failed: %v", err)
	}
	if len(states) != 2 {
		t.Fatalf("expected approved workflow page states, got %#v", states)
	}
	transitions, err := repo.ListPageTransitions(ctx, PageTransitionListQuery{ProjectID: "default", Site: "ops.example.com"})
	if err != nil {
		t.Fatalf("ListPageTransitions failed: %v", err)
	}
	if len(transitions) != 1 {
		t.Fatalf("expected approved workflow transition, got %#v", transitions)
	}
	transition := transitions[0]
	if transition.FromPageState != "page_a" || transition.ToPageState != "page_d" || transition.WorkflowID != approved.RecipeDraft.ID || transition.ChunkID == "" {
		t.Fatalf("unexpected transition payload: %#v", transition)
	}
}

func TestCandidateServiceApproveCreatesActiveWorkflowAndOutboxEvent(t *testing.T) {
	ctx := context.Background()
	repo, err := NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewCandidateService(repo)
	candidate, err := service.CreateFromSession(ctx, RecordedSession{
		ProjectID: "default",
		Source:    "user_demo",
		Task:      "Search GitHub issues",
		StartURL:  "https://github.com/alibaba/page-agent",
		Recipe:    sampleWorkflowRecipe(),
	})
	if err != nil {
		t.Fatalf("CreateFromSession failed: %v", err)
	}

	approved, err := service.Approve(ctx, candidate.ID)

	if err != nil {
		t.Fatalf("Approve failed: %v", err)
	}
	if approved.Status != StatusActive || !approved.Searchable {
		t.Fatalf("approved candidate should be active and searchable: %#v", approved)
	}
	workflow, err := repo.GetWorkflow(ctx, approved.RecipeDraft.ID)
	if err != nil {
		t.Fatalf("GetWorkflow failed after approve: %v", err)
	}
	if workflow.Status != StatusActive || !workflow.Searchable {
		t.Fatalf("workflow should be active and searchable: %#v", workflow)
	}
	events, err := repo.ListOutboxEvents(ctx, StatusPendingReview)
	if err != nil {
		t.Fatalf("ListOutboxEvents failed: %v", err)
	}
	if len(events) != 1 || events[0].Type != "workflow_approved" {
		t.Fatalf("expected workflow_approved event, got %#v", events)
	}
}

func TestCandidateServiceRejectDoesNotIndex(t *testing.T) {
	ctx := context.Background()
	repo, err := NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewCandidateService(repo)
	candidate, err := service.CreateFromSession(ctx, RecordedSession{
		ProjectID: "default",
		Source:    "user_demo",
		Task:      "Search GitHub issues",
		StartURL:  "https://github.com/alibaba/page-agent",
		Recipe:    sampleWorkflowRecipe(),
	})
	if err != nil {
		t.Fatalf("CreateFromSession failed: %v", err)
	}

	rejected, err := service.Reject(ctx, candidate.ID)

	if err != nil {
		t.Fatalf("Reject failed: %v", err)
	}
	if rejected.Status != StatusRejected || rejected.Searchable {
		t.Fatalf("rejected candidate should not be searchable: %#v", rejected)
	}
	events, err := repo.ListOutboxEvents(ctx, StatusPendingReview)
	if err != nil {
		t.Fatalf("ListOutboxEvents failed: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("rejected candidate should not emit index event: %#v", events)
	}
}

func TestCandidateServiceApproveWritesWorkflowAndPageEmbeddings(t *testing.T) {
	ctx := context.Background()
	repo, err := NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewCandidateServiceWithVectorizer(repo, fakeCandidateVectorizer{})
	candidate, err := service.CreateFromSession(ctx, RecordedSession{
		ProjectID: "default",
		Source:    "user_demo",
		Task:      "Search GitHub issues",
		StartURL:  "https://github.com/alibaba/page-agent",
		Recipe:    sampleWorkflowRecipe(),
	})
	if err != nil {
		t.Fatalf("CreateFromSession failed: %v", err)
	}

	approved, err := service.Approve(ctx, candidate.ID)
	if err != nil {
		t.Fatalf("Approve failed: %v", err)
	}
	workflow, err := repo.GetWorkflow(ctx, approved.RecipeDraft.ID)
	if err != nil {
		t.Fatalf("GetWorkflow failed: %v", err)
	}
	if len(workflow.Embedding) != 2 {
		t.Fatalf("expected workflow embedding, got %#v", workflow.Embedding)
	}
	states, err := repo.ListPageStates(ctx, PageStateListQuery{ProjectID: "default"})
	if err != nil {
		t.Fatalf("ListPageStates failed: %v", err)
	}
	if len(states) != 2 || len(states[0].Embedding) != 2 {
		t.Fatalf("expected page state embeddings, got %#v", states)
	}
}

func TestCandidateServiceApproveRepublishedWorkflowIncrementsVersion(t *testing.T) {
	ctx := context.Background()
	repo, err := NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewCandidateService(repo)
	firstRecipe := sampleWorkflowRecipe()
	firstRecipe.ID = "wf_versioned"
	firstRecipe.Version = 1
	first, err := service.CreateFromSession(ctx, RecordedSession{
		ProjectID: "default",
		Source:    "user_demo",
		Task:      "Search GitHub issues",
		StartURL:  "https://github.com/alibaba/page-agent",
		Recipe:    firstRecipe,
	})
	if err != nil {
		t.Fatalf("CreateFromSession first failed: %v", err)
	}
	if _, err := service.Approve(ctx, first.ID); err != nil {
		t.Fatalf("Approve first failed: %v", err)
	}
	secondRecipe := firstRecipe
	secondRecipe.Intent = "Search issues again with a better path"
	second, err := service.CreateFromSession(ctx, RecordedSession{
		ProjectID: "default",
		Source:    "user_demo",
		Task:      "Search GitHub issues again",
		StartURL:  "https://github.com/alibaba/page-agent",
		Recipe:    secondRecipe,
	})
	if err != nil {
		t.Fatalf("CreateFromSession second failed: %v", err)
	}

	approved, err := service.Approve(ctx, second.ID)
	if err != nil {
		t.Fatalf("Approve second failed: %v", err)
	}

	if approved.RecipeDraft.Version != 2 {
		t.Fatalf("expected republished workflow version 2, got %#v", approved.RecipeDraft)
	}
	workflow, err := repo.GetWorkflow(ctx, firstRecipe.ID)
	if err != nil {
		t.Fatalf("GetWorkflow failed: %v", err)
	}
	if workflow.Version != 2 || workflow.Intent != secondRecipe.Intent {
		t.Fatalf("expected current workflow to point at version 2, got %#v", workflow)
	}
	versions, err := repo.ListVersions(ctx, firstRecipe.ID)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 2 || versions[0].Version != 1 || versions[1].Version != 2 {
		t.Fatalf("expected append-only version history, got %#v", versions)
	}
}

func TestCandidateServiceRollbackRestoresRecipeAndEmbedding(t *testing.T) {
	ctx := context.Background()
	repo, err := NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewCandidateServiceWithVectorizer(repo, fakeVersionVectorizer{})
	firstRecipe := sampleWorkflowRecipe()
	firstRecipe.ID = "wf_rollback"
	firstRecipe.Version = 1
	firstRecipe.Name = "First GitHub issue search"
	firstCandidate, err := service.CreateFromSession(ctx, RecordedSession{
		ProjectID: "default",
		Source:    "user_demo",
		Task:      "Search GitHub issues",
		StartURL:  "https://github.com/alibaba/page-agent",
		Recipe:    firstRecipe,
	})
	if err != nil {
		t.Fatalf("CreateFromSession first failed: %v", err)
	}
	if _, err := service.Approve(ctx, firstCandidate.ID); err != nil {
		t.Fatalf("Approve first failed: %v", err)
	}
	secondRecipe := firstRecipe
	secondRecipe.Name = "Second GitHub issue search"
	secondCandidate, err := service.CreateFromSession(ctx, RecordedSession{
		ProjectID: "default",
		Source:    "user_demo",
		Task:      "Search GitHub issues with updated path",
		StartURL:  "https://github.com/alibaba/page-agent",
		Recipe:    secondRecipe,
	})
	if err != nil {
		t.Fatalf("CreateFromSession second failed: %v", err)
	}
	if _, err := service.Approve(ctx, secondCandidate.ID); err != nil {
		t.Fatalf("Approve second failed: %v", err)
	}

	rolledBack, err := service.Rollback(ctx, firstRecipe.ID, 1)
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	if rolledBack.Version != 1 || rolledBack.Name != firstRecipe.Name {
		t.Fatalf("expected rollback to restore version 1 recipe, got %#v", rolledBack)
	}
	if len(rolledBack.Embedding) != 2 || rolledBack.Embedding[0] != 1 || rolledBack.Embedding[1] != 0 {
		t.Fatalf("expected rollback to restore version 1 embedding, got %#v", rolledBack.Embedding)
	}
	current, err := repo.GetWorkflow(ctx, firstRecipe.ID)
	if err != nil {
		t.Fatalf("GetWorkflow failed: %v", err)
	}
	if current.Version != 1 || current.Name != firstRecipe.Name {
		t.Fatalf("expected current workflow to point at rolled back version, got %#v", current)
	}
	if !outboxHasType(t, repo, "workflow_rolled_back") {
		t.Fatalf("expected rollback outbox event")
	}
}

type fakeCandidateVectorizer struct{}

func (fakeCandidateVectorizer) DenseQuery(context.Context, string) ([]float32, error) {
	return []float32{0.8, 0.2}, nil
}

type fakeVersionVectorizer struct{}

func (fakeVersionVectorizer) DenseQuery(_ context.Context, text string) ([]float32, error) {
	if strings.Contains(text, "First") {
		return []float32{1, 0}, nil
	}
	return []float32{0, 1}, nil
}

func outboxHasType(t *testing.T, repo Repository, eventType string) bool {
	t.Helper()
	events, err := repo.ListOutboxEvents(context.Background(), StatusPendingReview)
	if err != nil {
		t.Fatalf("ListOutboxEvents failed: %v", err)
	}
	for _, event := range events {
		if event.Type == eventType {
			return true
		}
	}
	return false
}
