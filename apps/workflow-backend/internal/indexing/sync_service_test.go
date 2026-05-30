package indexing

import (
	"context"
	"testing"

	"github.com/page-agent/workflow-backend/internal/qdrant"
	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestSyncWorkflowApprovedUpsertsWorkflowPoints(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	recipe := sampleIndexWorkflow()
	if err := repo.SaveWorkflow(ctx, recipe); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}
	event, err := repo.AppendOutboxEvent(ctx, registry.OutboxEvent{
		Type:           "workflow_approved",
		IdempotencyKey: "workflow_approved:wf_github_issue_search:v3",
		Payload: map[string]any{
			"workflowId": recipe.ID,
			"version":    float64(recipe.Version),
			"projectId":  recipe.ProjectID,
		},
	})
	if err != nil {
		t.Fatalf("AppendOutboxEvent failed: %v", err)
	}
	writer := &fakePointWriter{}
	service := NewSyncService(repo, writer, qdrant.CollectionNames{
		WorkflowCards:  "pa_workflow_cards",
		WorkflowChunks: "pa_workflow_chunks",
		PageStates:     "pa_page_states",
	})

	if err := service.HandleEvent(ctx, event); err != nil {
		t.Fatalf("HandleEvent failed: %v", err)
	}

	if len(writer.upserts["pa_workflow_cards"]) != 1 {
		t.Fatalf("expected workflow card upsert, got %#v", writer.upserts)
	}
	if len(writer.upserts["pa_workflow_chunks"]) != 1 {
		t.Fatalf("expected chunk upsert, got %#v", writer.upserts)
	}
	if len(writer.upserts["pa_page_states"]) != 2 {
		t.Fatalf("expected page state upserts, got %#v", writer.upserts)
	}
}

func TestSyncWorkflowApprovedClearsPreviousWorkflowVersionsBeforeUpsert(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	recipe := sampleIndexWorkflow()
	recipe.Version = 4
	if err := repo.SaveWorkflow(ctx, recipe); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}
	writer := &fakePointWriter{}
	service := NewSyncService(repo, writer, qdrant.CollectionNames{
		WorkflowCards:  "pa_workflow_cards",
		WorkflowChunks: "pa_workflow_chunks",
	})

	if err := service.HandleEvent(ctx, registry.OutboxEvent{
		Type: EventWorkflowApproved,
		Payload: map[string]any{
			"workflowId": recipe.ID,
			"version":    recipe.Version,
		},
	}); err != nil {
		t.Fatalf("HandleEvent failed: %v", err)
	}

	if len(writer.deleted) != 2 {
		t.Fatalf("expected prior workflow card and chunk points to be cleared, got %#v", writer.deleted)
	}
	for _, deleted := range writer.deleted {
		if !filterHasMatch(deleted.filter, "workflow_id", recipe.ID) {
			t.Fatalf("expected workflow-scoped delete filter, got %#v", deleted.filter)
		}
	}
	if writer.upserts["pa_workflow_cards"][0].Payload["version"] != recipe.Version {
		t.Fatalf("expected current version point upsert, got %#v", writer.upserts["pa_workflow_cards"][0])
	}
}

func TestSyncWorkflowApprovedAttachesVectorsWhenVectorizerConfigured(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	recipe := sampleIndexWorkflow()
	if err := repo.SaveWorkflow(ctx, recipe); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}
	writer := &fakePointWriter{}
	service := NewSyncServiceWithVectorizer(
		repo,
		writer,
		qdrant.CollectionNames{
			WorkflowCards:  "pa_workflow_cards",
			WorkflowChunks: "pa_workflow_chunks",
			PageStates:     "pa_page_states",
		},
		fakeIndexVectorizer{},
	)

	err = service.HandleEvent(ctx, registry.OutboxEvent{
		Type: "workflow_approved",
		Payload: map[string]any{
			"workflowId": recipe.ID,
		},
	})

	if err != nil {
		t.Fatalf("HandleEvent failed: %v", err)
	}
	card := writer.upserts["pa_workflow_cards"][0]
	if len(card.Vector["intent_dense"].([]float32)) != 2 {
		t.Fatalf("expected card dense vector, got %#v", card.Vector)
	}
	chunk := writer.upserts["pa_workflow_chunks"][0]
	if _, ok := chunk.Vector["lexical_sparse"]; !ok {
		t.Fatalf("expected chunk sparse vector, got %#v", chunk.Vector)
	}
	page := writer.upserts["pa_page_states"][0]
	if len(page.Vector["page_dense"].([]float32)) != 2 {
		t.Fatalf("expected page dense vector, got %#v", page.Vector)
	}
}

func TestRebuildIndexesActiveProjectWorkflows(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	recipe := sampleIndexWorkflow()
	if err := repo.SaveWorkflow(ctx, recipe); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}
	other := recipe
	other.ID = "wf_other_project"
	other.ProjectID = "other"
	if err := repo.SaveWorkflow(ctx, other); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}
	writer := &fakePointWriter{}
	service := NewSyncService(repo, writer, qdrant.CollectionNames{
		WorkflowCards:  "pa_workflow_cards",
		WorkflowChunks: "pa_workflow_chunks",
		PageStates:     "pa_page_states",
	})

	count, err := service.Rebuild(ctx, "default")

	if err != nil {
		t.Fatalf("Rebuild failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one workflow indexed, got %d", count)
	}
	if len(writer.upserts["pa_workflow_cards"]) != 1 {
		t.Fatalf("expected one workflow card, got %#v", writer.upserts["pa_workflow_cards"])
	}
}

func TestRebuildClearsProjectScopedWorkflowCollectionsBeforeUpsert(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	recipe := sampleIndexWorkflow()
	if err := repo.SaveWorkflow(ctx, recipe); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}
	writer := &fakePointWriter{}
	service := NewSyncService(repo, writer, qdrant.CollectionNames{
		WorkflowCards:  "pa_workflow_cards",
		WorkflowChunks: "pa_workflow_chunks",
		PageStates:     "pa_page_states",
	})

	if _, err := service.Rebuild(ctx, "default"); err != nil {
		t.Fatalf("Rebuild failed: %v", err)
	}

	if len(writer.deleted) != 3 {
		t.Fatalf("expected rebuild to clear cards, chunks, and page states, got %#v", writer.deleted)
	}
	for _, deleted := range writer.deleted {
		if !filterHasProject(deleted.filter, "default") {
			t.Fatalf("expected project-scoped delete filter, got %#v", deleted.filter)
		}
	}
	if len(writer.upserts["pa_workflow_cards"]) != 1 {
		t.Fatalf("expected workflow cards to be rebuilt after delete, got %#v", writer.upserts)
	}
}

func TestSyncWorkflowDisabledDeletesByWorkflowFilter(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	writer := &fakePointWriter{}
	service := NewSyncService(repo, writer, qdrant.CollectionNames{
		WorkflowCards:  "pa_workflow_cards",
		WorkflowChunks: "pa_workflow_chunks",
	})

	err = service.HandleEvent(ctx, registry.OutboxEvent{
		Type: "workflow_deleted",
		Payload: map[string]any{
			"workflowId": "wf_github_issue_search",
		},
	})

	if err != nil {
		t.Fatalf("HandleEvent failed: %v", err)
	}
	if len(writer.deleted) != 2 {
		t.Fatalf("expected delete calls for card and chunks, got %#v", writer.deleted)
	}
}

func TestSyncWorkflowDisabledPatchesQdrantStatus(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	writer := &fakePointWriter{}
	service := NewSyncService(repo, writer, qdrant.CollectionNames{
		WorkflowCards:  "pa_workflow_cards",
		WorkflowChunks: "pa_workflow_chunks",
	})

	err = service.HandleEvent(ctx, registry.OutboxEvent{
		Type: "workflow_disabled",
		Payload: map[string]any{
			"workflowId": "wf_github_issue_search",
		},
	})

	if err != nil {
		t.Fatalf("HandleEvent failed: %v", err)
	}
	if len(writer.patches) != 2 {
		t.Fatalf("expected status patch calls for card and chunks, got %#v", writer.patches)
	}
	for _, patch := range writer.patches {
		if patch.payload["status"] != "disabled" {
			t.Fatalf("expected disabled status patch, got %#v", patch.payload)
		}
	}
}

func TestRunStatsUpdatedPatchesWorkflowHealthPayload(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	recipe := sampleIndexWorkflow()
	recipe.Chunks[0].SelectorHealth = 0.25
	recipe.Chunks[0].SuccessRate = 0.25
	if err := repo.SaveWorkflow(ctx, recipe); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}
	writer := &fakePointWriter{}
	service := NewSyncService(repo, writer, qdrant.CollectionNames{
		WorkflowCards:  "pa_workflow_cards",
		WorkflowChunks: "pa_workflow_chunks",
		PageStates:     "pa_page_states",
	})

	err = service.HandleEvent(ctx, registry.OutboxEvent{
		Type: "run_stats_updated",
		Payload: map[string]any{
			"workflowId": recipe.ID,
			"version":    recipe.Version,
			"runId":      "run_1",
		},
	})

	if err != nil {
		t.Fatalf("HandleEvent failed: %v", err)
	}
	if len(writer.patches) != 2 {
		t.Fatalf("expected card and chunk health patches, got %#v", writer.patches)
	}
	var chunkPatch map[string]any
	for _, patch := range writer.patches {
		if patch.collection == "pa_workflow_chunks" {
			chunkPatch = patch.payload
		}
	}
	if chunkPatch["selector_health"] != 0.25 || chunkPatch["success_rate"] != 0.25 {
		t.Fatalf("expected patched selector health and success rate, got %#v", chunkPatch)
	}
}

func TestSyncApprovedInterruptAndRepairUpsertsSpecializedCollections(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	handler := registry.InterruptHandler{
		ID:                  "ih_github_dialog",
		ProjectID:           "default",
		Site:                "github.com",
		Status:              registry.StatusActive,
		AppliesToPageStates: []string{"github_issues_list"},
		InterruptType:       "modal",
		FingerprintText:     "New feature",
		TargetControls:      []registry.ControlSignature{{Role: "button", Name: "Got it"}},
		RiskLevel:           registry.RiskReadOnly,
		WorkflowID:          "wf_close_dialog",
		Version:             1,
	}
	patch := registry.RepairPatch{
		ID:                  "patch_github_search",
		ProjectID:           "default",
		Status:              registry.StatusActive,
		WorkflowID:          "wf_github_issue_search",
		WorkflowVersion:     3,
		ChunkID:             "search_issues",
		StepID:              "fill_query",
		Site:                "github.com",
		FailureType:         "locator_not_found",
		FailureSignature:    "search input missing",
		NewTargetSummary:    "Search textbox",
		AppliesToPageStates: []string{"github_issues_list"},
		RiskLevel:           registry.RiskReadOnly,
	}
	if err := repo.SaveInterruptHandler(ctx, handler); err != nil {
		t.Fatalf("SaveInterruptHandler failed: %v", err)
	}
	if err := repo.SaveRepairPatch(ctx, patch); err != nil {
		t.Fatalf("SaveRepairPatch failed: %v", err)
	}
	writer := &fakePointWriter{}
	service := NewSyncService(repo, writer, qdrant.CollectionNames{
		InterruptHandlers: "pa_interrupt_handlers",
		RepairPatches:     "pa_repair_patches",
	})

	if err := service.HandleEvent(ctx, registry.OutboxEvent{
		Type: "interrupt_handler_approved",
		Payload: map[string]any{
			"handlerId": handler.ID,
			"projectId": handler.ProjectID,
		},
	}); err != nil {
		t.Fatalf("HandleEvent interrupt failed: %v", err)
	}
	if err := service.HandleEvent(ctx, registry.OutboxEvent{
		Type: "repair_patch_approved",
		Payload: map[string]any{
			"patchId":   patch.ID,
			"projectId": patch.ProjectID,
		},
	}); err != nil {
		t.Fatalf("HandleEvent repair failed: %v", err)
	}
	if len(writer.upserts["pa_interrupt_handlers"]) != 1 {
		t.Fatalf("expected interrupt upsert, got %#v", writer.upserts)
	}
	if len(writer.upserts["pa_repair_patches"]) != 1 {
		t.Fatalf("expected repair upsert, got %#v", writer.upserts)
	}
}

func TestHandleEventIsIdempotentForWorkflowApproved(t *testing.T) {
	ctx := context.Background()
	repo, event := setupWorkflowSyncEvent(t, EventWorkflowApproved)
	writer := &fakePointWriter{}
	service := NewSyncService(repo, writer, qdrant.CollectionNames{
		WorkflowCards:  "pa_workflow_cards",
		WorkflowChunks: "pa_workflow_chunks",
		PageStates:     "pa_page_states",
	})

	handleSameEventTwice(t, service, ctx, event)

	if len(writer.upserts["pa_workflow_cards"]) != 1 {
		t.Fatalf("expected one workflow card upsert after duplicate event, got %#v", writer.upserts["pa_workflow_cards"])
	}
	if len(writer.upserts["pa_workflow_chunks"]) != 1 {
		t.Fatalf("expected one workflow chunk upsert after duplicate event, got %#v", writer.upserts["pa_workflow_chunks"])
	}
	if len(writer.upserts["pa_page_states"]) != 2 {
		t.Fatalf("expected page states to be upserted once after duplicate event, got %#v", writer.upserts["pa_page_states"])
	}
}

func TestHandleEventIsIdempotentForWorkflowVersionCreated(t *testing.T) {
	ctx := context.Background()
	repo, event := setupWorkflowSyncEvent(t, EventWorkflowVersionCreated)
	writer := &fakePointWriter{}
	service := NewSyncService(repo, writer, qdrant.CollectionNames{
		WorkflowCards:  "pa_workflow_cards",
		WorkflowChunks: "pa_workflow_chunks",
		PageStates:     "pa_page_states",
	})

	handleSameEventTwice(t, service, ctx, event)

	if len(writer.upserts["pa_workflow_cards"]) != 1 {
		t.Fatalf("expected one workflow card upsert after duplicate event, got %#v", writer.upserts["pa_workflow_cards"])
	}
	if len(writer.upserts["pa_workflow_chunks"]) != 1 {
		t.Fatalf("expected one workflow chunk upsert after duplicate event, got %#v", writer.upserts["pa_workflow_chunks"])
	}
	if len(writer.upserts["pa_page_states"]) != 2 {
		t.Fatalf("expected page states to be upserted once after duplicate event, got %#v", writer.upserts["pa_page_states"])
	}
}

func TestHandleEventIsIdempotentForWorkflowDisabled(t *testing.T) {
	ctx := context.Background()
	repo, event := setupWorkflowSyncEvent(t, EventWorkflowDisabled)
	writer := &fakePointWriter{}
	service := NewSyncService(repo, writer, qdrant.CollectionNames{
		WorkflowCards:  "pa_workflow_cards",
		WorkflowChunks: "pa_workflow_chunks",
	})

	handleSameEventTwice(t, service, ctx, event)

	if len(writer.patches) != 2 {
		t.Fatalf("expected one status patch per workflow collection after duplicate event, got %#v", writer.patches)
	}
}

func TestHandleEventIsIdempotentForWorkflowDeleted(t *testing.T) {
	ctx := context.Background()
	repo, event := setupWorkflowSyncEvent(t, EventWorkflowDeleted)
	writer := &fakePointWriter{}
	service := NewSyncService(repo, writer, qdrant.CollectionNames{
		WorkflowCards:  "pa_workflow_cards",
		WorkflowChunks: "pa_workflow_chunks",
	})

	handleSameEventTwice(t, service, ctx, event)

	if len(writer.deleted) != 2 {
		t.Fatalf("expected one delete per workflow collection after duplicate event, got %#v", writer.deleted)
	}
}

func TestHandleEventIsIdempotentForRunStatsUpdated(t *testing.T) {
	ctx := context.Background()
	repo, event := setupWorkflowSyncEvent(t, EventRunStatsUpdated)
	writer := &fakePointWriter{}
	service := NewSyncService(repo, writer, qdrant.CollectionNames{
		WorkflowCards:  "pa_workflow_cards",
		WorkflowChunks: "pa_workflow_chunks",
	})

	handleSameEventTwice(t, service, ctx, event)

	if len(writer.patches) != 2 {
		t.Fatalf("expected one stats patch per workflow collection after duplicate event, got %#v", writer.patches)
	}
}

func TestHandleEventIsIdempotentForInterruptHandlerApproved(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	handler := registry.InterruptHandler{
		ID:                  "ih_github_dialog",
		ProjectID:           "default",
		Site:                "github.com",
		Status:              registry.StatusActive,
		AppliesToPageStates: []string{"github_issues_list"},
		InterruptType:       "modal",
		FingerprintText:     "New feature",
		TargetControls:      []registry.ControlSignature{{Role: "button", Name: "Got it"}},
		RiskLevel:           registry.RiskReadOnly,
		WorkflowID:          "wf_close_dialog",
		Version:             1,
	}
	if err := repo.SaveInterruptHandler(ctx, handler); err != nil {
		t.Fatalf("SaveInterruptHandler failed: %v", err)
	}
	writer := &fakePointWriter{}
	service := NewSyncService(repo, writer, qdrant.CollectionNames{InterruptHandlers: "pa_interrupt_handlers"})
	event := registry.OutboxEvent{
		ID:             "evt_interrupt",
		Type:           EventInterruptHandlerApproved,
		IdempotencyKey: "interrupt_handler_approved:ih_github_dialog:v1",
		Payload: map[string]any{
			"handlerId": handler.ID,
			"projectId": handler.ProjectID,
		},
	}

	handleSameEventTwice(t, service, ctx, event)

	if len(writer.upserts["pa_interrupt_handlers"]) != 1 {
		t.Fatalf("expected one interrupt handler upsert after duplicate event, got %#v", writer.upserts["pa_interrupt_handlers"])
	}
}

func TestHandleEventIsIdempotentForRepairPatchApproved(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	patch := registry.RepairPatch{
		ID:                  "patch_github_search",
		ProjectID:           "default",
		Status:              registry.StatusActive,
		WorkflowID:          "wf_github_issue_search",
		WorkflowVersion:     3,
		ChunkID:             "search_issues",
		StepID:              "fill_query",
		Site:                "github.com",
		FailureType:         "locator_not_found",
		FailureSignature:    "search input missing",
		NewTargetSummary:    "Search textbox",
		AppliesToPageStates: []string{"github_issues_list"},
		RiskLevel:           registry.RiskReadOnly,
	}
	if err := repo.SaveRepairPatch(ctx, patch); err != nil {
		t.Fatalf("SaveRepairPatch failed: %v", err)
	}
	writer := &fakePointWriter{}
	service := NewSyncService(repo, writer, qdrant.CollectionNames{RepairPatches: "pa_repair_patches"})
	event := registry.OutboxEvent{
		ID:             "evt_repair",
		Type:           EventRepairPatchApproved,
		IdempotencyKey: "repair_patch_approved:patch_github_search:v3",
		Payload: map[string]any{
			"patchId":   patch.ID,
			"projectId": patch.ProjectID,
		},
	}

	handleSameEventTwice(t, service, ctx, event)

	if len(writer.upserts["pa_repair_patches"]) != 1 {
		t.Fatalf("expected one repair patch upsert after duplicate event, got %#v", writer.upserts["pa_repair_patches"])
	}
}

type fakePointWriter struct {
	upserts map[string][]qdrant.Point
	deleted []fakeDeleteCall
	patches []fakePayloadPatch
}

type fakeDeleteCall struct {
	collection string
	filter     qdrant.Filter
}

type fakePayloadPatch struct {
	collection string
	payload    map[string]any
}

func handleSameEventTwice(t *testing.T, service *SyncService, ctx context.Context, event registry.OutboxEvent) {
	t.Helper()
	if err := service.HandleEvent(ctx, event); err != nil {
		t.Fatalf("first HandleEvent failed: %v", err)
	}
	if err := service.HandleEvent(ctx, event); err != nil {
		t.Fatalf("second HandleEvent failed: %v", err)
	}
}

func setupWorkflowSyncEvent(t *testing.T, eventType string) (*registry.FileRepository, registry.OutboxEvent) {
	t.Helper()
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	recipe := sampleIndexWorkflow()
	recipe.Chunks[0].SelectorHealth = 0.25
	recipe.Chunks[0].SuccessRate = 0.25
	if err := repo.SaveWorkflow(ctx, recipe); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}
	return repo, registry.OutboxEvent{
		ID:             "evt_" + eventType,
		Type:           eventType,
		IdempotencyKey: eventType + ":" + recipe.ID + ":v3",
		Payload: map[string]any{
			"workflowId": recipe.ID,
			"version":    recipe.Version,
			"projectId":  recipe.ProjectID,
			"runId":      "run_1",
		},
	}
}

func (writer *fakePointWriter) UpsertPoints(_ context.Context, collection string, points []qdrant.Point) error {
	if writer.upserts == nil {
		writer.upserts = map[string][]qdrant.Point{}
	}
	writer.upserts[collection] = append(writer.upserts[collection], points...)
	return nil
}

func (writer *fakePointWriter) DeleteByFilter(_ context.Context, collection string, filter qdrant.Filter) error {
	writer.deleted = append(writer.deleted, fakeDeleteCall{collection: collection, filter: filter})
	return nil
}

func (writer *fakePointWriter) SetPayloadByFilter(_ context.Context, collection string, payload map[string]any, _ qdrant.Filter) error {
	writer.patches = append(writer.patches, fakePayloadPatch{collection: collection, payload: payload})
	return nil
}

type fakeIndexVectorizer struct{}

func (fakeIndexVectorizer) DenseQuery(context.Context, string) ([]float32, error) {
	return []float32{0.1, 0.2}, nil
}

func (fakeIndexVectorizer) SparseQuery(context.Context, string) (map[string]any, error) {
	return map[string]any{
		"indices": []uint32{1},
		"values":  []float32{0.5},
	}, nil
}

func filterHasProject(filter qdrant.Filter, projectID string) bool {
	for _, condition := range filter.Must {
		if condition.Key == "project_id" && condition.Match["value"] == projectID {
			return true
		}
	}
	return false
}

func filterHasMatch(filter qdrant.Filter, key string, value string) bool {
	for _, condition := range filter.Must {
		if condition.Key == key && condition.Match["value"] == value {
			return true
		}
	}
	return false
}

func sampleIndexWorkflow() registry.WorkflowRecipe {
	return registry.WorkflowRecipe{
		ID:          "wf_github_issue_search",
		Version:     3,
		ProjectID:   "default",
		Status:      registry.StatusActive,
		Searchable:  true,
		Site:        "github.com",
		App:         "github",
		Name:        "Search GitHub issues",
		Intent:      "Search issues in a GitHub repository",
		Description: "Open a repository Issues page and search by query.",
		RiskLevel:   registry.RiskReadOrSearch,
		Variables: []registry.Variable{
			{Name: "repo", Type: registry.VariableString, Required: true, Source: registry.VariableSourceTaskOrURL},
			{Name: "query", Type: registry.VariableString, Required: true, Source: registry.VariableSourceTask},
		},
		StartPageStates: []string{"github_repo_home"},
		EndPageStates:   []string{"github_issues_list"},
		Chunks: []registry.WorkflowChunk{
			{
				ID:            "open_issues",
				Name:          "Open Issues",
				FromPageState: "github_repo_home",
				ToPageState:   "github_issues_list",
				StepSummary:   "Open Issues tab",
				RiskLevel:     registry.RiskReadOrSearch,
				Steps: []registry.WorkflowStep{
					{
						ID:        "click_issues",
						Type:      registry.StepClick,
						RiskLevel: registry.RiskReadOrSearch,
						Target: registry.StepTarget{
							Primary: registry.TargetCandidate{Strategy: registry.TargetRole, Role: "link", Name: "Issues"},
						},
					},
				},
			},
		},
	}
}
