package retrieval

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/page-agent/workflow-backend/internal/qdrant"
	"github.com/page-agent/workflow-backend/internal/registry"
)

type Vectorizer interface {
	DenseQuery(context.Context, string) ([]float32, error)
	SparseQuery(context.Context, string) (map[string]any, error)
}

type SearchServiceConfig struct {
	Repository  registry.Repository
	Qdrant      *qdrant.Client
	Collections qdrant.CollectionNames
	Vectorizer  Vectorizer
}

type SearchService struct {
	repo        registry.Repository
	qdrant      *qdrant.Client
	collections qdrant.CollectionNames
	vectorizer  Vectorizer
}

func NewSearchService(config SearchServiceConfig) *SearchService {
	return &SearchService{
		repo:        config.Repository,
		qdrant:      config.Qdrant,
		collections: config.Collections,
		vectorizer:  config.Vectorizer,
	}
}

func (service *SearchService) Search(ctx context.Context, request SearchRequest) ([]WorkflowCandidate, error) {
	normalized := NormalizeRequest(request)
	dense, err := service.vectorizer.DenseQuery(ctx, normalized.Task)
	if err != nil {
		return nil, err
	}
	sparse, err := service.vectorizer.SparseQuery(ctx, normalized.Task)
	if err != nil {
		return nil, err
	}
	filter := workflowFilter(normalized)
	var cardHits []qdrantHit
	var chunkHits []qdrantHit
	var cardErr error
	var chunkErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		cardHits, cardErr = service.searchCollection(ctx, service.collections.WorkflowCards, qdrant.SearchRequest{
			Vector:      map[string]any{"name": "intent_dense", "vector": dense},
			Filter:      &filter,
			Limit:       50,
			WithPayload: true,
		})
	}()
	go func() {
		defer wg.Done()
		chunkHits, chunkErr = service.searchCollection(ctx, service.collections.WorkflowChunks, qdrant.SearchRequest{
			Vector:      map[string]any{"name": "lexical_sparse", "vector": sparse},
			Filter:      &filter,
			Limit:       80,
			WithPayload: true,
		})
	}()
	wg.Wait()
	if cardErr != nil {
		return nil, cardErr
	}
	if chunkErr != nil {
		return nil, chunkErr
	}
	raw := mergeHits(cardHits, chunkHits)
	service.hydrateRawCandidates(ctx, raw)
	candidates := Rerank(raw, RerankInput{
		CandidateSlots: normalized.CandidateSlots,
		RiskPolicy:     normalized.RiskPolicy,
		Site:           normalized.Site,
		Task:           normalized.Task,
	})
	service.enrichCandidates(ctx, candidates)
	return candidates, nil
}

func (service *SearchService) SearchInterrupts(request InterruptRequest) ([]InterruptHit, error) {
	text := request.FingerprintText
	if text == "" {
		text = request.Observation.Title + "\n" + joinLines(request.Observation.VisibleText)
	}
	hits, err := service.searchSpecialCollection(
		context.Background(),
		service.collections.InterruptHandlers,
		text,
		specialFilter(request.ProjectID, request.Site, "handler"),
	)
	if err != nil {
		return nil, err
	}
	result := make([]InterruptHit, 0, len(hits))
	for _, hit := range hits {
		result = append(result, InterruptHit{
			HandlerID:           payloadString(hit.Payload, "handler_id"),
			WorkflowID:          payloadString(hit.Payload, "workflow_id"),
			Version:             payloadInt(hit.Payload, "version"),
			Score:               hit.Score,
			Site:                payloadString(hit.Payload, "site"),
			RiskLevel:           registry.RiskLevel(payloadString(hit.Payload, "risk_level")),
			AppliesToPageStates: payloadStringSlice(hit.Payload, "applies_to_page_states"),
			RequiredText:        payloadStringSlice(hit.Payload, "required_text"),
			TargetControls:      controlsFromPayload(hit.Payload),
			Reasons:             []string{"qdrant interrupt handler matched"},
		})
	}
	return result, nil
}

func (service *SearchService) SearchRepairs(request RepairRequest) ([]RepairHit, error) {
	text := joinLines([]string{request.FailureType, request.ChunkID, request.StepID, request.CurrentPageState})
	hits, err := service.searchSpecialCollection(
		context.Background(),
		service.collections.RepairPatches,
		text,
		specialFilter(request.ProjectID, request.Site, "patch"),
	)
	if err != nil {
		return nil, err
	}
	result := make([]RepairHit, 0, len(hits))
	for _, hit := range hits {
		result = append(result, RepairHit{
			PatchID:             payloadString(hit.Payload, "patch_id"),
			WorkflowID:          payloadString(hit.Payload, "workflow_id"),
			WorkflowVersion:     payloadInt(hit.Payload, "workflow_version"),
			ChunkID:             payloadString(hit.Payload, "chunk_id"),
			StepID:              payloadString(hit.Payload, "step_id"),
			Score:               hit.Score,
			Site:                payloadString(hit.Payload, "site"),
			FailureType:         payloadString(hit.Payload, "failure_type"),
			AppliesToPageStates: payloadStringSlice(hit.Payload, "applies_to_page_states"),
			RiskLevel:           registry.RiskLevel(payloadString(hit.Payload, "risk_level")),
			Reasons:             []string{"qdrant repair patch matched"},
		})
	}
	return result, nil
}

func (service *SearchService) searchSpecialCollection(
	ctx context.Context,
	collection string,
	text string,
	filter qdrant.Filter,
) ([]qdrantHit, error) {
	if collection == "" {
		return nil, nil
	}
	dense, err := service.vectorizer.DenseQuery(ctx, text)
	if err != nil {
		return nil, err
	}
	return service.searchCollection(ctx, collection, qdrant.SearchRequest{
		Vector:      map[string]any{"name": "intent_dense", "vector": dense},
		Filter:      &filter,
		Limit:       20,
		WithPayload: true,
	})
}

func (service *SearchService) searchCollection(ctx context.Context, collection string, request qdrant.SearchRequest) ([]qdrantHit, error) {
	if collection == "" {
		return nil, nil
	}
	response, err := service.qdrant.Search(ctx, collection, request)
	if err != nil {
		return nil, err
	}
	var hits []qdrantHit
	if err := json.Unmarshal(response.Result, &hits); err != nil {
		return nil, fmt.Errorf("decode qdrant hits: %w", err)
	}
	return hits, nil
}

type qdrantHit struct {
	ID      string         `json:"id"`
	Score   float64        `json:"score"`
	Payload map[string]any `json:"payload"`
}

func workflowFilter(request NormalizedRequest) qdrant.Filter {
	filter := qdrant.Filter{
		Must: []qdrant.Condition{
			{Key: "project_id", Match: map[string]any{"value": request.ProjectID}},
			{Key: "status", Match: map[string]any{"value": "active"}},
		},
		MustNot: []qdrant.Condition{},
	}
	if request.Site != "" {
		filter.Must = append(filter.Must, qdrant.Condition{Key: "site", Match: map[string]any{"value": request.Site}})
	}
	filter.Must = append(filter.Must, qdrant.Condition{Key: "searchable", Match: map[string]any{"value": true}})
	for _, blocked := range request.RiskPolicy.Blocked {
		filter.MustNot = append(filter.MustNot, qdrant.Condition{Key: "risk_level", Match: map[string]any{"value": string(blocked)}})
	}
	return filter
}

func specialFilter(projectID string, site string, _ string) qdrant.Filter {
	filter := qdrant.Filter{
		Must: []qdrant.Condition{
			{Key: "project_id", Match: map[string]any{"value": projectID}},
			{Key: "status", Match: map[string]any{"value": "active"}},
		},
	}
	if site != "" {
		filter.Must = append(filter.Must, qdrant.Condition{Key: "site", Match: map[string]any{"value": site}})
	}
	return filter
}

func mergeHits(cardHits []qdrantHit, chunkHits []qdrantHit) []RawCandidate {
	merged := map[string]*RawCandidate{}
	for _, hit := range cardHits {
		workflowID := payloadString(hit.Payload, "workflow_id")
		version := payloadInt(hit.Payload, "version")
		key := workflowID + ":" + fmt.Sprint(version)
		raw := rawFromPayload(hit.Payload)
		raw.WorkflowDense = hit.Score
		merged[key] = &raw
	}
	for _, hit := range chunkHits {
		workflowID := payloadString(hit.Payload, "workflow_id")
		version := payloadInt(hit.Payload, "version")
		key := workflowID + ":" + fmt.Sprint(version)
		raw, ok := merged[key]
		if !ok {
			value := rawFromPayload(hit.Payload)
			raw = &value
			merged[key] = raw
		}
		if hit.Score > raw.BestChunk {
			raw.BestChunk = hit.Score
		}
		if stepSummary := payloadString(hit.Payload, "step_summary"); stepSummary != "" {
			raw.StepText = joinLines([]string{raw.StepText, stepSummary})
		}
		if health := payloadFloat(hit.Payload, "selector_health"); health > 0 {
			raw.SelectorHealth = health
		}
	}
	result := make([]RawCandidate, 0, len(merged))
	for _, raw := range merged {
		result = append(result, *raw)
	}
	return result
}

func (service *SearchService) hydrateRawCandidates(ctx context.Context, candidates []RawCandidate) {
	if service.repo == nil {
		return
	}
	for index := range candidates {
		workflow, err := service.repo.GetWorkflow(ctx, candidates[index].WorkflowID)
		if err != nil {
			continue
		}
		candidates[index].StepText = joinLines([]string{
			candidates[index].StepText,
			WorkflowStepText(workflow),
		})
		if candidates[index].UpdatedAt.IsZero() {
			candidates[index].UpdatedAt = workflow.UpdatedAt
		}
	}
}

func WorkflowStepText(workflow registry.WorkflowRecipe) string {
	lines := make([]string, 0, len(workflow.Chunks))
	for _, chunk := range workflow.Chunks {
		if chunk.StepSummary != "" {
			lines = append(lines, chunk.StepSummary)
			continue
		}
		lines = append(lines, workflowChunkTargetText(chunk))
	}
	return joinLines(lines)
}

func workflowChunkTargetText(chunk registry.WorkflowChunk) string {
	parts := make([]string, 0, len(chunk.Steps))
	for _, step := range chunk.Steps {
		parts = append(parts, string(step.Type)+" "+step.Target.Primary.Text())
	}
	return strings.Join(parts, " -> ")
}

func rawFromPayload(payload map[string]any) RawCandidate {
	return RawCandidate{
		WorkflowID:         payloadString(payload, "workflow_id"),
		Version:            payloadInt(payload, "version"),
		Status:             registry.Status(payloadString(payload, "status")),
		Searchable:         payloadBool(payload, "searchable"),
		Site:               payloadString(payload, "site"),
		RiskLevel:          registry.RiskLevel(payloadString(payload, "risk_level")),
		VariableNames:      payloadStringSlice(payload, "variable_names"),
		WorkflowSparse:     payloadFloat(payload, "sparse_score"),
		PageState:          payloadFloat(payload, "page_state_score"),
		SuccessRate:        payloadFloat(payload, "success_rate"),
		SelectorHealth:     payloadFloat(payload, "selector_health"),
		RecentFailureCount: payloadInt(payload, "recent_failure_count"),
	}
}

func (service *SearchService) enrichCandidates(ctx context.Context, candidates []WorkflowCandidate) {
	if service.repo == nil {
		return
	}
	for index := range candidates {
		workflow, err := service.repo.GetWorkflow(ctx, candidates[index].WorkflowID)
		if err != nil {
			continue
		}
		candidates[index].Name = workflow.Name
		candidates[index].Intent = workflow.Intent
	}
}

func payloadString(payload map[string]any, key string) string {
	value, _ := payload[key].(string)
	return value
}

func payloadInt(payload map[string]any, key string) int {
	switch value := payload[key].(type) {
	case int:
		return value
	case float64:
		return int(value)
	default:
		return 0
	}
}

func payloadFloat(payload map[string]any, key string) float64 {
	switch value := payload[key].(type) {
	case float64:
		return value
	case int:
		return float64(value)
	default:
		return 0
	}
}

func payloadBool(payload map[string]any, key string) bool {
	value, _ := payload[key].(bool)
	return value
}

func payloadStringSlice(payload map[string]any, key string) []string {
	switch values := payload[key].(type) {
	case []string:
		return values
	case []any:
		result := make([]string, 0, len(values))
		for _, value := range values {
			if text, ok := value.(string); ok {
				result = append(result, text)
			}
		}
		return result
	default:
		return nil
	}
}

func controlsFromPayload(payload map[string]any) []Control {
	roles := payloadStringSlice(payload, "target_roles")
	names := payloadStringSlice(payload, "target_names")
	limit := len(names)
	if len(roles) > limit {
		limit = len(roles)
	}
	controls := make([]Control, 0, limit)
	for index := 0; index < limit; index++ {
		control := Control{}
		if index < len(roles) {
			control.Role = roles[index]
		}
		if index < len(names) {
			control.Name = names[index]
		}
		controls = append(controls, control)
	}
	return controls
}

func joinLines(lines []string) string {
	result := ""
	for _, line := range lines {
		if line == "" {
			continue
		}
		if result != "" {
			result += "\n"
		}
		result += line
	}
	return result
}
