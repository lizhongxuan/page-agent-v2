package indexing

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/page-agent/workflow-backend/internal/qdrant"
	"github.com/page-agent/workflow-backend/internal/registry"
)

type PointWriter interface {
	UpsertPoints(context.Context, string, []qdrant.Point) error
	DeleteByFilter(context.Context, string, qdrant.Filter) error
	SetPayloadByFilter(context.Context, string, map[string]any, qdrant.Filter) error
}

type SyncService struct {
	repo        registry.Repository
	writer      PointWriter
	collections qdrant.CollectionNames
	vectorizer  Vectorizer
	processedMu sync.Mutex
	processed   map[string]struct{}
}

func NewSyncService(repo registry.Repository, writer PointWriter, collections qdrant.CollectionNames) *SyncService {
	return &SyncService{repo: repo, writer: writer, collections: collections}
}

type Vectorizer interface {
	DenseQuery(context.Context, string) ([]float32, error)
	SparseQuery(context.Context, string) (map[string]any, error)
}

func NewSyncServiceWithVectorizer(
	repo registry.Repository,
	writer PointWriter,
	collections qdrant.CollectionNames,
	vectorizer Vectorizer,
) *SyncService {
	return &SyncService{
		repo:        repo,
		writer:      writer,
		collections: collections,
		vectorizer:  vectorizer,
	}
}

func (service *SyncService) HandleEvent(ctx context.Context, event registry.OutboxEvent) error {
	if service.eventProcessed(event) {
		return nil
	}
	var err error
	switch event.Type {
	case EventWorkflowApproved, EventWorkflowVersionCreated, EventWorkflowRolledBack:
		err = service.indexWorkflow(ctx, event)
	case "run_stats_updated":
		err = service.patchWorkflowRunStats(ctx, event)
	case EventInterruptHandlerApproved:
		err = service.indexInterruptHandler(ctx, event)
	case EventRepairPatchApproved:
		err = service.indexRepairPatch(ctx, event)
	case "workflow_deleted":
		err = service.deleteWorkflow(ctx, event)
	case "workflow_disabled":
		err = service.disableWorkflow(ctx, event)
	default:
		err = nil
	}
	if err != nil {
		return err
	}
	service.markEventProcessed(event)
	return nil
}

func (service *SyncService) eventProcessed(event registry.OutboxEvent) bool {
	keys := outboxEventKeys(event)
	if len(keys) == 0 {
		return false
	}
	service.processedMu.Lock()
	defer service.processedMu.Unlock()
	for _, key := range keys {
		if _, ok := service.processed[key]; ok {
			return true
		}
	}
	return false
}

func (service *SyncService) markEventProcessed(event registry.OutboxEvent) {
	keys := outboxEventKeys(event)
	if len(keys) == 0 {
		return
	}
	service.processedMu.Lock()
	defer service.processedMu.Unlock()
	if service.processed == nil {
		service.processed = map[string]struct{}{}
	}
	for _, key := range keys {
		service.processed[key] = struct{}{}
	}
}

func outboxEventKeys(event registry.OutboxEvent) []string {
	keys := []string{}
	if event.ID != "" {
		keys = append(keys, "id:"+event.ID)
	}
	if event.IdempotencyKey != "" {
		keys = append(keys, "idempotency:"+event.IdempotencyKey)
	}
	return keys
}

func (service *SyncService) indexInterruptHandler(ctx context.Context, event registry.OutboxEvent) error {
	handlerID, _ := event.Payload["handlerId"].(string)
	projectID, _ := event.Payload["projectId"].(string)
	if handlerID == "" {
		return errors.New("handler id is required for interrupt indexing")
	}
	handlers, err := service.repo.ListActiveInterruptHandlers(ctx, projectID)
	if err != nil {
		return err
	}
	for _, handler := range handlers {
		if handler.ID != handlerID {
			continue
		}
		point, err := qdrant.BuildInterruptHandlerPoint(handler)
		if err != nil {
			return err
		}
		if err := service.attachVectors(ctx, &point, "intent_dense"); err != nil {
			return fmt.Errorf("vectorize interrupt handler: %w", err)
		}
		return service.writer.UpsertPoints(ctx, service.collections.InterruptHandlers, []qdrant.Point{point})
	}
	return errors.New("interrupt handler not found")
}

func (service *SyncService) indexRepairPatch(ctx context.Context, event registry.OutboxEvent) error {
	patchID, _ := event.Payload["patchId"].(string)
	projectID, _ := event.Payload["projectId"].(string)
	if patchID == "" {
		return errors.New("patch id is required for repair indexing")
	}
	patches, err := service.repo.ListActiveRepairPatches(ctx, projectID)
	if err != nil {
		return err
	}
	for _, patch := range patches {
		if patch.ID != patchID {
			continue
		}
		point, err := qdrant.BuildRepairPatchPoint(patch)
		if err != nil {
			return err
		}
		if err := service.attachVectors(ctx, &point, "intent_dense"); err != nil {
			return fmt.Errorf("vectorize repair patch: %w", err)
		}
		return service.writer.UpsertPoints(ctx, service.collections.RepairPatches, []qdrant.Point{point})
	}
	return errors.New("repair patch not found")
}

func (service *SyncService) IndexWorkflow(ctx context.Context, workflowID string) error {
	recipe, err := service.repo.GetWorkflow(ctx, workflowID)
	if err != nil {
		return err
	}
	return service.indexRecipe(ctx, recipe, true)
}

func (service *SyncService) Rebuild(ctx context.Context, projectID string) (int, error) {
	workflows, err := service.repo.ListActiveWorkflows(ctx, projectID)
	if err != nil {
		return 0, err
	}
	if err := service.clearRebuildCollections(ctx, projectID); err != nil {
		return 0, err
	}
	for _, workflow := range workflows {
		if err := service.indexRecipe(ctx, workflow, false); err != nil {
			return 0, err
		}
	}
	return len(workflows), nil
}

func (service *SyncService) clearRebuildCollections(ctx context.Context, projectID string) error {
	filter := projectFilter(projectID)
	for _, collection := range []string{service.collections.WorkflowCards, service.collections.WorkflowChunks, service.collections.PageStates} {
		if collection == "" {
			continue
		}
		if err := service.writer.DeleteByFilter(ctx, collection, filter); err != nil {
			return err
		}
	}
	return nil
}

func (service *SyncService) indexWorkflow(ctx context.Context, event registry.OutboxEvent) error {
	workflowID, _ := event.Payload["workflowId"].(string)
	if workflowID == "" {
		return errors.New("workflow id is required for indexing")
	}
	recipe, err := service.repo.GetWorkflow(ctx, workflowID)
	if err != nil {
		return err
	}
	return service.indexRecipe(ctx, recipe, true)
}

func (service *SyncService) indexRecipe(ctx context.Context, recipe registry.WorkflowRecipe, clearPrevious bool) error {
	if clearPrevious {
		if err := service.deleteWorkflowPoints(ctx, recipe.ID); err != nil {
			return fmt.Errorf("clear previous workflow points: %w", err)
		}
	}
	cardPoint, err := qdrant.BuildWorkflowCardPoint(qdrant.WorkflowCardFromRecipe(recipe, nil))
	if err != nil {
		return err
	}
	if err := service.attachVectors(ctx, &cardPoint, "intent_dense"); err != nil {
		return fmt.Errorf("vectorize workflow card: %w", err)
	}
	if err := service.writer.UpsertPoints(ctx, service.collections.WorkflowCards, []qdrant.Point{cardPoint}); err != nil {
		return fmt.Errorf("upsert workflow card: %w", err)
	}
	chunkPoints, err := qdrant.BuildWorkflowChunkPoints(recipe)
	if err != nil {
		return err
	}
	if len(chunkPoints) > 0 {
		for index := range chunkPoints {
			if err := service.attachVectors(ctx, &chunkPoints[index], "intent_dense"); err != nil {
				return fmt.Errorf("vectorize workflow chunk %s: %w", chunkPoints[index].ID, err)
			}
		}
		if err := service.writer.UpsertPoints(ctx, service.collections.WorkflowChunks, chunkPoints); err != nil {
			return fmt.Errorf("upsert workflow chunks: %w", err)
		}
	}
	pagePoints := pageStatePointsFromRecipe(recipe)
	if len(pagePoints) > 0 {
		for index := range pagePoints {
			if err := service.attachVectors(ctx, &pagePoints[index], "page_dense"); err != nil {
				return fmt.Errorf("vectorize page state %s: %w", pagePoints[index].ID, err)
			}
		}
		if err := service.writer.UpsertPoints(ctx, service.collections.PageStates, pagePoints); err != nil {
			return fmt.Errorf("upsert page states: %w", err)
		}
	}
	return nil
}

func (service *SyncService) attachVectors(ctx context.Context, point *qdrant.Point, denseVectorName string) error {
	if service.vectorizer == nil {
		return nil
	}
	text, _ := point.Payload["embedding_text"].(string)
	if text == "" {
		return nil
	}
	dense, err := service.vectorizer.DenseQuery(ctx, text)
	if err != nil {
		return err
	}
	sparse, err := service.vectorizer.SparseQuery(ctx, text)
	if err != nil {
		return err
	}
	point.Vector = map[string]any{
		denseVectorName:  dense,
		"lexical_sparse": sparse,
	}
	return nil
}

func (service *SyncService) deleteWorkflow(ctx context.Context, event registry.OutboxEvent) error {
	workflowID, _ := event.Payload["workflowId"].(string)
	if workflowID == "" {
		return errors.New("workflow id is required for delete")
	}
	return service.deleteWorkflowPoints(ctx, workflowID)
}

func (service *SyncService) deleteWorkflowPoints(ctx context.Context, workflowID string) error {
	filter := qdrant.Filter{Must: []qdrant.Condition{{Key: "workflow_id", Match: map[string]any{"value": workflowID}}}}
	for _, collection := range []string{service.collections.WorkflowCards, service.collections.WorkflowChunks} {
		if collection == "" {
			continue
		}
		if err := service.writer.DeleteByFilter(ctx, collection, filter); err != nil {
			return err
		}
	}
	return nil
}

func (service *SyncService) disableWorkflow(ctx context.Context, event registry.OutboxEvent) error {
	workflowID, _ := event.Payload["workflowId"].(string)
	if workflowID == "" {
		return errors.New("workflow id is required for disable")
	}
	filter := workflowVersionFilter(workflowID, payloadVersion(event), "")
	payload := map[string]any{
		"status":     string(registry.StatusDisabled),
		"searchable": false,
	}
	for _, collection := range []string{service.collections.WorkflowCards, service.collections.WorkflowChunks} {
		if collection == "" {
			continue
		}
		if err := service.writer.SetPayloadByFilter(ctx, collection, payload, filter); err != nil {
			return err
		}
	}
	return nil
}

func (service *SyncService) patchWorkflowRunStats(ctx context.Context, event registry.OutboxEvent) error {
	workflowID, _ := event.Payload["workflowId"].(string)
	if workflowID == "" {
		return errors.New("workflow id is required for run stats patch")
	}
	recipe, err := service.repo.GetWorkflow(ctx, workflowID)
	if err != nil {
		return err
	}
	card := qdrant.WorkflowCardFromRecipe(recipe, nil)
	cardPayload := map[string]any{
		"success_rate":         card.SuccessRate,
		"recent_failure_count": card.RecentFailureCount,
		"last_failure_at":      formatOptionalTime(card.LastFailureAt),
	}
	if service.collections.WorkflowCards != "" {
		if err := service.writer.SetPayloadByFilter(
			ctx,
			service.collections.WorkflowCards,
			cardPayload,
			workflowVersionFilter(recipe.ID, recipe.Version, ""),
		); err != nil {
			return err
		}
	}
	if service.collections.WorkflowChunks == "" {
		return nil
	}
	for _, chunk := range recipe.Chunks {
		payload := map[string]any{
			"success_rate":         chunk.SuccessRate,
			"selector_health":      chunk.SelectorHealth,
			"recent_failure_count": chunk.RecentFailureCount,
			"last_failure_at":      formatOptionalTime(chunk.LastFailureAt),
		}
		if err := service.writer.SetPayloadByFilter(
			ctx,
			service.collections.WorkflowChunks,
			payload,
			workflowVersionFilter(recipe.ID, recipe.Version, chunk.ID),
		); err != nil {
			return err
		}
	}
	return nil
}

func formatOptionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func workflowVersionFilter(workflowID string, version int, chunkID string) qdrant.Filter {
	conditions := []qdrant.Condition{
		{Key: "workflow_id", Match: map[string]any{"value": workflowID}},
	}
	if version > 0 {
		conditions = append(conditions, qdrant.Condition{Key: "version", Match: map[string]any{"value": version}})
	}
	if chunkID != "" {
		conditions = append(conditions, qdrant.Condition{Key: "chunk_id", Match: map[string]any{"value": chunkID}})
	}
	return qdrant.Filter{Must: conditions}
}

func projectFilter(projectID string) qdrant.Filter {
	if projectID == "" {
		return qdrant.Filter{}
	}
	return qdrant.Filter{Must: []qdrant.Condition{{Key: "project_id", Match: map[string]any{"value": projectID}}}}
}

func payloadVersion(event registry.OutboxEvent) int {
	switch value := event.Payload["version"].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}

func pageStatePointsFromRecipe(recipe registry.WorkflowRecipe) []qdrant.Point {
	stateIDs := map[string]bool{}
	for _, stateID := range recipe.StartPageStates {
		stateIDs[stateID] = true
	}
	for _, stateID := range recipe.EndPageStates {
		stateIDs[stateID] = true
	}
	for _, chunk := range recipe.Chunks {
		if chunk.FromPageState != "" {
			stateIDs[chunk.FromPageState] = true
		}
		if chunk.ToPageState != "" {
			stateIDs[chunk.ToPageState] = true
		}
	}
	points := make([]qdrant.Point, 0, len(stateIDs))
	for stateID := range stateIDs {
		point, err := qdrant.BuildPageStatePoint(registry.PageState{
			ID:             stateID,
			ProjectID:      recipe.ProjectID,
			Site:           recipe.Site,
			App:            recipe.App,
			Status:         registry.StatusActive,
			CanonicalTitle: stateID,
		})
		if err == nil {
			points = append(points, point)
		}
	}
	return points
}
