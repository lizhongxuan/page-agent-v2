package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(ctx context.Context, databaseURL string) (*PostgresRepository, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, errors.New("postgres database URL is required")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	repo := &PostgresRepository{pool: pool}
	if err := repo.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return repo, nil
}

func (repo *PostgresRepository) Close() {
	if repo != nil && repo.pool != nil {
		repo.pool.Close()
	}
}

func (repo *PostgresRepository) migrate(ctx context.Context) error {
	for _, migration := range PostgresMigrations {
		if _, err := repo.pool.Exec(ctx, migration); err != nil {
			return err
		}
	}
	return nil
}

func (repo *PostgresRepository) SaveCandidate(ctx context.Context, candidate WorkflowCandidate) error {
	if candidate.ID == "" {
		candidate.ID = newID("cand")
	}
	if candidate.CreatedAt.IsZero() {
		candidate.CreatedAt = time.Now().UTC()
	}
	payload, err := marshalPayload(candidate)
	if err != nil {
		return err
	}
	_, err = repo.pool.Exec(ctx, `
insert into workflow_candidates (id, project_id, source, status, created_at, payload)
values ($1, $2, $3, $4, $5, $6)
on conflict(id) do update set
  project_id = excluded.project_id,
  source = excluded.source,
  status = excluded.status,
  created_at = excluded.created_at,
  payload = excluded.payload`,
		candidate.ID, candidate.ProjectID, candidate.Source, string(candidate.Status), candidate.CreatedAt, payload)
	return err
}

func (repo *PostgresRepository) GetCandidate(ctx context.Context, id string) (WorkflowCandidate, error) {
	return getPayloadByID[WorkflowCandidate](ctx, repo.pool, "workflow_candidates", id, "workflow candidate not found")
}

func (repo *PostgresRepository) ListCandidates(ctx context.Context, query CandidateListQuery) ([]WorkflowCandidate, error) {
	where := []string{"true"}
	args := []any{}
	if query.ProjectID != "" {
		args = append(args, query.ProjectID)
		where = append(where, fmt.Sprintf("project_id = $%d", len(args)))
	}
	if query.Source != "" {
		args = append(args, query.Source)
		where = append(where, fmt.Sprintf("source = $%d", len(args)))
	}
	if query.Status != "" {
		args = append(args, string(query.Status))
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	return listPayloads[WorkflowCandidate](ctx, repo.pool,
		"select payload from workflow_candidates where "+strings.Join(where, " and ")+" order by created_at desc, id asc",
		args...)
}

func (repo *PostgresRepository) SaveWorkflow(ctx context.Context, recipe WorkflowRecipe) error {
	if err := ValidateWorkflowRecipe(recipe); err != nil {
		return err
	}
	if recipe.UpdatedAt.IsZero() {
		recipe.UpdatedAt = time.Now().UTC()
	}
	payload, err := marshalPayload(recipe)
	if err != nil {
		return err
	}
	_, err = repo.pool.Exec(ctx, `
insert into workflow_recipes (id, project_id, site, status, searchable, current_version, risk_level, description, recipe_json, embedding, updated_at, payload)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, nullif($10, '')::vector, $11, $12)
on conflict(id) do update set
  project_id = excluded.project_id,
  site = excluded.site,
  status = excluded.status,
  searchable = excluded.searchable,
  current_version = excluded.current_version,
  risk_level = excluded.risk_level,
  description = excluded.description,
  recipe_json = excluded.recipe_json,
  embedding = excluded.embedding,
  updated_at = excluded.updated_at,
  payload = excluded.payload`,
		recipe.ID, recipe.ProjectID, recipe.Site, string(recipe.Status), recipe.Searchable,
		recipe.Version, string(recipe.RiskLevel), recipe.Description, string(payload), vectorLiteral(recipe.Embedding), recipe.UpdatedAt, payload)
	return err
}

func (repo *PostgresRepository) GetWorkflow(ctx context.Context, id string) (WorkflowRecipe, error) {
	return getPayloadByID[WorkflowRecipe](ctx, repo.pool, "workflow_recipes", id, "workflow not found")
}

func (repo *PostgresRepository) ListActiveWorkflows(ctx context.Context, projectID string) ([]WorkflowRecipe, error) {
	args := []any{string(StatusActive)}
	where := []string{"status = $1", "searchable = true"}
	if projectID != "" {
		args = append(args, projectID)
		where = append(where, fmt.Sprintf("project_id = $%d", len(args)))
	}
	return listPayloads[WorkflowRecipe](ctx, repo.pool,
		"select payload from workflow_recipes where "+strings.Join(where, " and ")+" order by id asc",
		args...)
}

func (repo *PostgresRepository) SaveVersion(ctx context.Context, version WorkflowVersion) error {
	if version.CreatedAt.IsZero() {
		version.CreatedAt = time.Now().UTC()
	}
	payload, err := marshalPayload(version)
	if err != nil {
		return err
	}
	_, err = repo.pool.Exec(ctx, `
insert into workflow_versions (workflow_id, version, created_at, payload)
values ($1, $2, $3, $4)
on conflict(workflow_id, version) do update set created_at = excluded.created_at, payload = excluded.payload`,
		version.WorkflowID, version.Version, version.CreatedAt, payload)
	return err
}

func (repo *PostgresRepository) GetVersion(ctx context.Context, workflowID string, version int) (WorkflowVersion, error) {
	return getPayload[WorkflowVersion](ctx, repo.pool,
		"select payload from workflow_versions where workflow_id = $1 and version = $2",
		"workflow version not found", workflowID, version)
}

func (repo *PostgresRepository) ListVersions(ctx context.Context, workflowID string) ([]WorkflowVersion, error) {
	where := "true"
	args := []any{}
	if workflowID != "" {
		args = append(args, workflowID)
		where = "workflow_id = $1"
	}
	return listPayloads[WorkflowVersion](ctx, repo.pool,
		"select payload from workflow_versions where "+where+" order by workflow_id asc, version asc",
		args...)
}

func (repo *PostgresRepository) SaveRun(ctx context.Context, run WorkflowRun) error {
	if run.ID == "" {
		run.ID = newID("run")
	}
	if run.CreatedAt.IsZero() {
		run.CreatedAt = time.Now().UTC()
	}
	payload, err := marshalPayload(run)
	if err != nil {
		return err
	}
	_, err = repo.pool.Exec(ctx, `
insert into workflow_runs (id, project_id, workflow_id, version, created_at, payload)
values ($1, $2, $3, $4, $5, $6)
on conflict(id) do update set
  project_id = excluded.project_id,
  workflow_id = excluded.workflow_id,
  version = excluded.version,
  created_at = excluded.created_at,
  payload = excluded.payload`,
		run.ID, run.ProjectID, run.WorkflowID, run.Version, run.CreatedAt, payload)
	return err
}

func (repo *PostgresRepository) GetRun(ctx context.Context, id string) (WorkflowRun, error) {
	return getPayloadByID[WorkflowRun](ctx, repo.pool, "workflow_runs", id, "workflow run not found")
}

func (repo *PostgresRepository) SaveTaskRun(ctx context.Context, run TaskRun) error {
	if run.ID == "" {
		run.ID = newID("task_run")
	}
	if run.CreatedAt.IsZero() {
		run.CreatedAt = time.Now().UTC()
	}
	if err := ValidateSummaryLength(run.Summary); err != nil {
		return err
	}
	for index := range run.ActionSteps {
		if run.ActionSteps[index].ID == "" {
			run.ActionSteps[index].ID = newID("step")
		}
		if err := ValidateSummaryLength(run.ActionSteps[index].ReasoningSummary); err != nil {
			return err
		}
		if err := ValidateSummaryLength(run.ActionSteps[index].ResultSummary); err != nil {
			return err
		}
		run.ActionSteps[index].TaskRunID = run.ID
		if run.ActionSteps[index].CreatedAt.IsZero() {
			run.ActionSteps[index].CreatedAt = run.CreatedAt
		}
	}
	payload, err := marshalPayload(run)
	if err != nil {
		return err
	}
	originalPath, err := json.Marshal(run.OriginalPath)
	if err != nil {
		return err
	}
	optimizedPath, err := json.Marshal(run.OptimizedPath)
	if err != nil {
		return err
	}
	memoryEvidenceRefs, err := json.Marshal(run.MemoryEvidenceRefs)
	if err != nil {
		return err
	}
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `
insert into task_runs (id, project_id, site, task_template, summary, original_path, optimized_path, memory_context_id, memory_evidence_refs, status, created_at, payload)
values ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8, $9::jsonb, $10, $11, $12)
on conflict(id) do update set
  project_id = excluded.project_id,
  site = excluded.site,
  task_template = excluded.task_template,
  summary = excluded.summary,
  original_path = excluded.original_path,
  optimized_path = excluded.optimized_path,
  memory_context_id = excluded.memory_context_id,
  memory_evidence_refs = excluded.memory_evidence_refs,
  status = excluded.status,
  created_at = excluded.created_at,
  payload = excluded.payload`,
		run.ID, run.ProjectID, run.Site, run.TaskTemplate, run.Summary, string(originalPath), string(optimizedPath),
		run.MemoryContextID, string(memoryEvidenceRefs), string(run.Status), run.CreatedAt, payload); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "delete from action_steps where task_run_id = $1", run.ID); err != nil {
		return err
	}
	for _, step := range run.ActionSteps {
		stepPayload, err := marshalPayload(step)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
insert into action_steps (id, task_run_id, page_state_id, step_index, action_type, target_name, value_template, reasoning_summary, result_summary, is_branch_noise, created_at, payload)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
			step.ID, run.ID, step.PageStateID, step.StepIndex, string(step.ActionType), step.TargetName,
			step.ValueTemplate, step.ReasoningSummary, step.ResultSummary, step.IsBranchNoise, step.CreatedAt, stepPayload); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (repo *PostgresRepository) GetTaskRun(ctx context.Context, id string) (TaskRun, error) {
	return getPayloadByID[TaskRun](ctx, repo.pool, "task_runs", id, "task run not found")
}

func (repo *PostgresRepository) ListTaskRuns(ctx context.Context, query TaskRunListQuery) ([]TaskRun, error) {
	where := []string{"true"}
	args := []any{}
	if query.ProjectID != "" {
		args = append(args, query.ProjectID)
		where = append(where, fmt.Sprintf("project_id = $%d", len(args)))
	}
	if query.Site != "" {
		args = append(args, query.Site)
		where = append(where, fmt.Sprintf("site = $%d", len(args)))
	}
	if query.Status != "" {
		args = append(args, string(query.Status))
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	return listPayloads[TaskRun](ctx, repo.pool,
		"select payload from task_runs where "+strings.Join(where, " and ")+" order by created_at desc, id asc",
		args...)
}

func (repo *PostgresRepository) SavePageState(ctx context.Context, state PageState) error {
	if state.ID == "" {
		state.ID = newID("page")
	}
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = time.Now().UTC()
	}
	payload, err := marshalPayload(state)
	if err != nil {
		return err
	}
	hardRules, err := json.Marshal(state.HardRules)
	if err != nil {
		return err
	}
	requiredControls, err := json.Marshal(state.RequiredControls)
	if err != nil {
		return err
	}
	requiredText, err := json.Marshal(state.RequiredText)
	if err != nil {
		return err
	}
	_, err = repo.pool.Exec(ctx, `
insert into page_states (id, project_id, site, url_pattern, page_name, purpose_summary, hard_rules, required_controls, required_text, embedding, status, updated_at, payload)
values ($1, $2, $3, $4, $5, $6, $7::jsonb, $8::jsonb, $9::jsonb, nullif($10, '')::vector, $11, $12, $13)
on conflict(id) do update set
  project_id = excluded.project_id,
  site = excluded.site,
  url_pattern = excluded.url_pattern,
  page_name = excluded.page_name,
  purpose_summary = excluded.purpose_summary,
  hard_rules = excluded.hard_rules,
  required_controls = excluded.required_controls,
  required_text = excluded.required_text,
  embedding = excluded.embedding,
  status = excluded.status,
  updated_at = excluded.updated_at,
  payload = excluded.payload`,
		state.ID, state.ProjectID, state.Site, state.URLPattern, state.CanonicalTitle, state.CanonicalTitle,
		string(hardRules), string(requiredControls), string(requiredText), vectorLiteral(state.Embedding),
		string(state.Status), state.UpdatedAt, payload)
	return err
}

func (repo *PostgresRepository) GetPageState(ctx context.Context, id string) (PageState, error) {
	return getPayloadByID[PageState](ctx, repo.pool, "page_states", id, "page state not found")
}

func (repo *PostgresRepository) ListPageStates(ctx context.Context, query PageStateListQuery) ([]PageState, error) {
	where := []string{"true"}
	args := []any{}
	if query.ProjectID != "" {
		args = append(args, query.ProjectID)
		where = append(where, fmt.Sprintf("project_id = $%d", len(args)))
	}
	if query.Site != "" {
		args = append(args, query.Site)
		where = append(where, fmt.Sprintf("site = $%d", len(args)))
	}
	return listPayloads[PageState](ctx, repo.pool,
		"select payload from page_states where "+strings.Join(where, " and ")+" order by id asc",
		args...)
}

func (repo *PostgresRepository) SavePageSurface(ctx context.Context, surface PageSurface) error {
	if surface.ID == "" {
		surface.ID = newID("surface")
	}
	now := time.Now().UTC()
	if surface.FirstSeenAt.IsZero() {
		surface.FirstSeenAt = now
	}
	if surface.LastSeenAt.IsZero() {
		surface.LastSeenAt = now
	}
	if surface.ObservationCount <= 0 {
		surface.ObservationCount = 1
	}
	if surface.Status == "" {
		surface.Status = StatusActive
	}
	payload, err := marshalPayload(surface)
	if err != nil {
		return err
	}
	visibilityRules, err := json.Marshal(surface.VisibilityRules)
	if err != nil {
		return err
	}
	requiredControls, err := json.Marshal(surface.RequiredControls)
	if err != nil {
		return err
	}
	requiredText, err := json.Marshal(surface.RequiredText)
	if err != nil {
		return err
	}
	_, err = repo.pool.Exec(ctx, `
insert into page_surfaces (id, project_id, site, parent_page_state_id, surface_type, surface_fingerprint, title, required_text, required_controls, visibility_rules, observation_count, status, first_seen_at, last_seen_at, payload)
values ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9::jsonb, $10::jsonb, $11, $12, $13, $14, $15)
on conflict(id) do update set
  project_id = excluded.project_id,
  site = excluded.site,
  parent_page_state_id = excluded.parent_page_state_id,
  surface_type = excluded.surface_type,
  surface_fingerprint = excluded.surface_fingerprint,
  title = excluded.title,
  required_text = excluded.required_text,
  required_controls = excluded.required_controls,
  visibility_rules = excluded.visibility_rules,
  observation_count = page_surfaces.observation_count + 1,
  status = excluded.status,
  last_seen_at = excluded.last_seen_at,
  payload = excluded.payload`,
		surface.ID, surface.ProjectID, surface.Site, surface.ParentPageStateID, string(surface.SurfaceType),
		surface.SurfaceFingerprint, surface.Title, string(requiredText), string(requiredControls), string(visibilityRules),
		surface.ObservationCount, string(surface.Status), surface.FirstSeenAt, surface.LastSeenAt, payload)
	return err
}

func (repo *PostgresRepository) ListPageSurfaces(ctx context.Context, query PageSurfaceListQuery) ([]PageSurface, error) {
	where := []string{"true"}
	args := []any{}
	if query.ProjectID != "" {
		args = append(args, query.ProjectID)
		where = append(where, fmt.Sprintf("project_id = $%d", len(args)))
	}
	if query.Site != "" {
		args = append(args, query.Site)
		where = append(where, fmt.Sprintf("site = $%d", len(args)))
	}
	if query.ParentPageStateID != "" {
		args = append(args, query.ParentPageStateID)
		where = append(where, fmt.Sprintf("parent_page_state_id = $%d", len(args)))
	}
	if query.SurfaceType != "" {
		args = append(args, string(query.SurfaceType))
		where = append(where, fmt.Sprintf("surface_type = $%d", len(args)))
	}
	return listPayloads[PageSurface](ctx, repo.pool,
		"select payload from page_surfaces where "+strings.Join(where, " and ")+" order by id asc",
		args...)
}

func (repo *PostgresRepository) SavePageTransition(ctx context.Context, transition PageTransition) error {
	if transition.ID == "" {
		transition.ID = newID("transition")
	}
	payload, err := marshalPayload(transition)
	if err != nil {
		return err
	}
	guardRules, err := json.Marshal(transition.GuardRules)
	if err != nil {
		return err
	}
	targetCandidates, err := json.Marshal(transition.TargetCandidates)
	if err != nil {
		return err
	}
	_, err = repo.pool.Exec(ctx, `
insert into page_transitions (id, project_id, site, from_page_state, to_page_state, workflow_id, version, chunk_id, action_name, target_name, target_candidates, guard_rules, risk_level, success_count, failure_count, payload)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::jsonb, $12::jsonb, $13, $14, $15, $16)
on conflict(id) do update set
  project_id = excluded.project_id,
  site = excluded.site,
  from_page_state = excluded.from_page_state,
  to_page_state = excluded.to_page_state,
  workflow_id = excluded.workflow_id,
  version = excluded.version,
  chunk_id = excluded.chunk_id,
  action_name = excluded.action_name,
  target_name = excluded.target_name,
  target_candidates = excluded.target_candidates,
  guard_rules = excluded.guard_rules,
  risk_level = excluded.risk_level,
  success_count = excluded.success_count,
  failure_count = excluded.failure_count,
  payload = excluded.payload`,
		transition.ID, transition.ProjectID, transition.Site, transition.FromPageState, transition.ToPageState,
		transition.WorkflowID, transition.Version, transition.ChunkID, transition.ActionName, transition.TargetName,
		string(targetCandidates), string(guardRules), string(transition.RiskLevel), transition.SuccessCount,
		transition.FailureCount, payload)
	return err
}

func (repo *PostgresRepository) ListPageTransitions(ctx context.Context, query PageTransitionListQuery) ([]PageTransition, error) {
	where := []string{"true"}
	args := []any{}
	if query.ProjectID != "" {
		args = append(args, query.ProjectID)
		where = append(where, fmt.Sprintf("project_id = $%d", len(args)))
	}
	if query.Site != "" {
		args = append(args, query.Site)
		where = append(where, fmt.Sprintf("site = $%d", len(args)))
	}
	if query.FromPageStateID != "" {
		args = append(args, query.FromPageStateID)
		where = append(where, fmt.Sprintf("from_page_state = $%d", len(args)))
	}
	if query.ToPageStateID != "" {
		args = append(args, query.ToPageStateID)
		where = append(where, fmt.Sprintf("to_page_state = $%d", len(args)))
	}
	return listPayloads[PageTransition](ctx, repo.pool,
		"select payload from page_transitions where "+strings.Join(where, " and ")+" order by id asc",
		args...)
}

func (repo *PostgresRepository) SaveKnowledgeDocument(ctx context.Context, document KnowledgeDocument) error {
	if document.ID == "" {
		document.ID = newID("doc")
	}
	if document.CreatedAt.IsZero() {
		document.CreatedAt = time.Now().UTC()
	}
	payload, err := marshalPayload(document)
	if err != nil {
		return err
	}
	_, err = repo.pool.Exec(ctx, `
insert into knowledge_documents (id, project_id, created_at, payload)
values ($1, $2, $3, $4)
on conflict(id) do update set project_id = excluded.project_id, created_at = excluded.created_at, payload = excluded.payload`,
		document.ID, document.ProjectID, document.CreatedAt, payload)
	return err
}

func (repo *PostgresRepository) SaveKnowledgeChunks(ctx context.Context, chunks []KnowledgeChunk) error {
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	now := time.Now().UTC()
	documentIDs := map[string]bool{}
	for index := range chunks {
		if chunks[index].DocumentID != "" {
			documentIDs[chunks[index].DocumentID] = true
		}
		if chunks[index].ID == "" {
			chunks[index].ID = newID("chunk")
		}
	}
	for documentID := range documentIDs {
		if _, err := tx.Exec(ctx, "delete from knowledge_chunks where document_id = $1", documentID); err != nil {
			return err
		}
	}
	for _, chunk := range chunks {
		if chunk.ID == "" {
			chunk.ID = newID("chunk")
		}
		if chunk.UpdatedAt.IsZero() {
			chunk.UpdatedAt = now
		}
		payload, err := marshalPayload(chunk)
		if err != nil {
			return err
		}
		tags, err := json.Marshal(chunk.Tags)
		if err != nil {
			return err
		}
		metadata, err := json.Marshal(chunk.Metadata)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
insert into knowledge_chunks (id, document_id, project_id, title, source, chunk_text, tags, metadata, embedding, updated_at, payload)
values ($1, $2, $3, $4, $5, $6, $7::jsonb, $8::jsonb, nullif($9, '')::vector, $10, $11)
on conflict(id) do update set
  document_id = excluded.document_id,
  project_id = excluded.project_id,
  title = excluded.title,
  source = excluded.source,
  chunk_text = excluded.chunk_text,
  tags = excluded.tags,
  metadata = excluded.metadata,
  embedding = excluded.embedding,
  updated_at = excluded.updated_at,
  payload = excluded.payload`,
			chunk.ID, chunk.DocumentID, chunk.ProjectID, chunk.Title, chunk.Source, chunk.ChunkText, string(tags), string(metadata), vectorLiteral(chunk.Embedding), chunk.UpdatedAt, payload); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (repo *PostgresRepository) SearchKnowledgeChunks(ctx context.Context, query KnowledgeSearchQuery) ([]KnowledgeChunk, error) {
	if len(query.Embedding) > 0 {
		hits, err := repo.searchKnowledgeChunksByEmbedding(ctx, query)
		if err != nil {
			return nil, err
		}
		if len(hits) > 0 {
			return hits, nil
		}
	}
	chunks, err := listPayloads[KnowledgeChunk](ctx, repo.pool,
		"select payload from knowledge_chunks where ($1 = '' or project_id = $1)",
		query.ProjectID)
	if err != nil {
		return nil, err
	}
	queryText := strings.ToLower(strings.Join([]string{
		query.Task,
		query.Title,
		query.URL,
		query.VisibleText,
		strings.Join(query.Hints, " "),
	}, " "))
	result := make([]KnowledgeChunk, 0)
	for _, chunk := range chunks {
		if !knowledgeChunkHardGateMatches(chunk, query) {
			continue
		}
		score := scoreKnowledgeChunk(chunk, queryText)
		if score <= 0 {
			continue
		}
		chunk.Score = score
		result = append(result, chunk)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Score == result[j].Score {
			return result[i].ID < result[j].ID
		}
		return result[i].Score > result[j].Score
	})
	limit := query.Limit
	if limit <= 0 {
		limit = 3
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (repo *PostgresRepository) SaveBusinessSystemProfile(ctx context.Context, profile BusinessSystemProfile) error {
	if profile.UpdatedAt.IsZero() {
		profile.UpdatedAt = time.Now().UTC()
	}
	if err := ValidateMemoryRecord(profile); err != nil {
		return err
	}
	payload, err := marshalPayload(profile)
	if err != nil {
		return err
	}
	modules, err := json.Marshal(profile.Modules)
	if err != nil {
		return err
	}
	entryPages, err := json.Marshal(profile.EntryPages)
	if err != nil {
		return err
	}
	terms, err := json.Marshal(profile.Terms)
	if err != nil {
		return err
	}
	sourceRefs, err := json.Marshal(profile.SourceRefs)
	if err != nil {
		return err
	}
	if profile.Status == "" {
		profile.Status = StatusActive
	}
	if profile.SourceType == "" {
		profile.SourceType = MemorySourceProduction
	}
	_, err = repo.pool.Exec(ctx, `
	insert into business_system_profiles (project_id, site, module, source_type, status, summary, modules, entry_pages, terms, source_refs, confidence, updated_at, payload)
	values ($1, $2, $3, $4, $5, $6, $7::jsonb, $8::jsonb, $9::jsonb, $10::jsonb, $11, $12, $13)
	on conflict(project_id, site, module) do update set
	  site = excluded.site,
	  module = excluded.module,
	  source_type = excluded.source_type,
	  status = excluded.status,
	  summary = excluded.summary,
	  modules = excluded.modules,
	  entry_pages = excluded.entry_pages,
	  terms = excluded.terms,
	  source_refs = excluded.source_refs,
	  confidence = excluded.confidence,
	  updated_at = excluded.updated_at,
	  payload = excluded.payload`,
		profile.ProjectID, profile.Site, profile.Module, string(profile.SourceType), string(profile.Status),
		profile.Summary, string(modules), string(entryPages), string(terms), string(sourceRefs),
		profile.Confidence, profile.UpdatedAt, payload)
	return err
}

func (repo *PostgresRepository) GetBusinessSystemProfile(ctx context.Context, query BusinessSystemProfileQuery) (BusinessSystemProfile, error) {
	profiles, err := listPayloads[BusinessSystemProfile](ctx, repo.pool,
		"select payload from business_system_profiles where ($1 = '' or project_id = $1) and ($2 = '' or site = '' or site = $2)",
		query.ProjectID, query.Site)
	if err != nil {
		return BusinessSystemProfile{}, err
	}
	sort.Slice(profiles, func(i, j int) bool {
		return businessProfileRank(profiles[i], query) > businessProfileRank(profiles[j], query)
	})
	for _, profile := range profiles {
		if profileSourceAllowed(profile.SourceType, query.SourceType) && businessProfileRank(profile, query) > 0 {
			return profile, nil
		}
	}
	return BusinessSystemProfile{}, errors.New("business system profile not found")
}

func (repo *PostgresRepository) SavePageObservationEvent(ctx context.Context, event PageObservationEvent) error {
	now := time.Now().UTC()
	if event.ID == "" {
		event.ID = newID("obs")
	}
	if existing, err := getPayloadByID[PageObservationEvent](ctx, repo.pool, "page_observation_events", event.ID, "page observation event not found"); err == nil {
		if event.CreatedAt.IsZero() {
			event.CreatedAt = existing.CreatedAt
		}
		if event.SeenCount <= 0 {
			event.SeenCount = existing.SeenCount + 1
		}
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = now
	}
	if event.LastSeenAt.IsZero() {
		event.LastSeenAt = now
	}
	if event.SeenCount <= 0 {
		event.SeenCount = 1
	}
	if err := ValidateSummaryLength(event.VisibleTextSample); err != nil {
		return err
	}
	if containsSensitiveText(event.VisibleTextSample) {
		return errors.New("page observation visible text contains sensitive text")
	}
	payload, err := marshalPayload(event)
	if err != nil {
		return err
	}
	controls, err := json.Marshal(event.Controls)
	if err != nil {
		return err
	}
	links, err := json.Marshal(event.Links)
	if err != nil {
		return err
	}
	_, err = repo.pool.Exec(ctx, `
insert into page_observation_events (id, project_id, site, url, url_pattern, title, visible_text_sample, controls, links, fingerprint, created_at, last_seen_at, seen_count, payload)
values ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9::jsonb, $10, $11, $12, $13, $14)
on conflict(id) do update set
  project_id = excluded.project_id,
  site = excluded.site,
  url = excluded.url,
  url_pattern = excluded.url_pattern,
  title = excluded.title,
  visible_text_sample = excluded.visible_text_sample,
  controls = excluded.controls,
  links = excluded.links,
  fingerprint = excluded.fingerprint,
  created_at = excluded.created_at,
  last_seen_at = excluded.last_seen_at,
  seen_count = excluded.seen_count,
  payload = excluded.payload`,
		event.ID, event.ProjectID, event.Site, event.URL, event.URLPattern, event.Title,
		event.VisibleTextSample, string(controls), string(links), event.Fingerprint, event.CreatedAt,
		event.LastSeenAt, event.SeenCount, payload)
	return err
}

func (repo *PostgresRepository) ListPageObservationEvents(ctx context.Context, query PageObservationEventListQuery) ([]PageObservationEvent, error) {
	where := []string{"true"}
	args := []any{}
	if query.ProjectID != "" {
		args = append(args, query.ProjectID)
		where = append(where, fmt.Sprintf("project_id = $%d", len(args)))
	}
	if query.Site != "" {
		args = append(args, query.Site)
		where = append(where, fmt.Sprintf("site = $%d", len(args)))
	}
	if query.URLPattern != "" {
		args = append(args, query.URLPattern)
		where = append(where, fmt.Sprintf("url_pattern = $%d", len(args)))
	}
	return listPayloads[PageObservationEvent](ctx, repo.pool,
		"select payload from page_observation_events where "+strings.Join(where, " and ")+" order by created_at desc, id asc",
		args...)
}

func (repo *PostgresRepository) SaveExperienceMemory(ctx context.Context, memory ExperienceMemory) error {
	if memory.ID == "" {
		memory.ID = newID("exp")
	}
	now := time.Now().UTC()
	if memory.CreatedAt.IsZero() {
		memory.CreatedAt = now
	}
	if memory.UpdatedAt.IsZero() {
		memory.UpdatedAt = now
	}
	if memory.ReviewStatus == "" {
		memory.ReviewStatus = ReviewStatusPending
	}
	if err := ValidateMemoryRecord(memory); err != nil {
		return err
	}
	payload, err := marshalPayload(memory)
	if err != nil {
		return err
	}
	optimizedPath, err := json.Marshal(memory.OptimizedPath)
	if err != nil {
		return err
	}
	stepsSummary, err := json.Marshal(memory.StepsSummary)
	if err != nil {
		return err
	}
	variables, err := json.Marshal(memory.Variables)
	if err != nil {
		return err
	}
	failureWarnings, err := json.Marshal(memory.FailureWarnings)
	if err != nil {
		return err
	}
	_, err = repo.pool.Exec(ctx, `
insert into experience_memories (
  id, project_id, site, task_template, intent, summary, start_page_state, end_page_state,
  optimized_path, steps_summary, variables, success_count, failure_count, last_success_at,
  last_failure_at, failure_warnings, searchable, review_status, embedding, created_at, updated_at, payload
) values (
  $1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10::jsonb, $11::jsonb, $12, $13, $14,
  $15, $16::jsonb, $17, $18, nullif($19, '')::vector, $20, $21, $22
)
on conflict(id) do update set
  project_id = excluded.project_id,
  site = excluded.site,
  task_template = excluded.task_template,
  intent = excluded.intent,
  summary = excluded.summary,
  start_page_state = excluded.start_page_state,
  end_page_state = excluded.end_page_state,
  optimized_path = excluded.optimized_path,
  steps_summary = excluded.steps_summary,
  variables = excluded.variables,
  success_count = excluded.success_count,
  failure_count = excluded.failure_count,
  last_success_at = excluded.last_success_at,
  last_failure_at = excluded.last_failure_at,
  failure_warnings = excluded.failure_warnings,
  searchable = excluded.searchable,
  review_status = excluded.review_status,
  embedding = excluded.embedding,
  created_at = excluded.created_at,
  updated_at = excluded.updated_at,
  payload = excluded.payload`,
		memory.ID, memory.ProjectID, memory.Site, memory.TaskTemplate, memory.Intent, memory.Summary,
		memory.StartPageState, memory.EndPageState, string(optimizedPath), string(stepsSummary),
		string(variables), memory.SuccessCount, memory.FailureCount, nullableTime(memory.LastSuccessAt),
		nullableTime(memory.LastFailureAt), string(failureWarnings), memory.Searchable,
		string(memory.ReviewStatus), vectorLiteral(memory.Embedding), memory.CreatedAt, memory.UpdatedAt, payload)
	return err
}

func (repo *PostgresRepository) GetExperienceMemory(ctx context.Context, id string) (ExperienceMemory, error) {
	return getPayloadByID[ExperienceMemory](ctx, repo.pool, "experience_memories", id, "experience memory not found")
}

func (repo *PostgresRepository) SearchExperienceMemories(ctx context.Context, query ExperienceMemorySearchQuery) ([]ExperienceMemory, error) {
	if query.SearchableOnly == false {
		query.SearchableOnly = true
	}
	return repo.ListExperienceMemories(ctx, query)
}

func (repo *PostgresRepository) ListExperienceMemories(ctx context.Context, query ExperienceMemorySearchQuery) ([]ExperienceMemory, error) {
	where := []string{"true"}
	args := []any{}
	if query.ProjectID != "" {
		args = append(args, query.ProjectID)
		where = append(where, fmt.Sprintf("project_id = $%d", len(args)))
	}
	if query.Site != "" {
		args = append(args, query.Site)
		where = append(where, fmt.Sprintf("site = $%d", len(args)))
	}
	if query.Status != "" {
		args = append(args, string(query.Status))
		where = append(where, fmt.Sprintf("review_status = $%d", len(args)))
	}
	if query.SearchableOnly {
		where = append(where, "searchable = true")
	}
	if query.StartPageState != "" {
		args = append(args, query.StartPageState)
		where = append(where, fmt.Sprintf("start_page_state = $%d", len(args)))
	}
	memories, err := listPayloads[ExperienceMemory](ctx, repo.pool,
		"select payload from experience_memories where "+strings.Join(where, " and ")+" order by updated_at desc, id asc",
		args...)
	if err != nil {
		return nil, err
	}
	queryText := strings.ToLower(query.Task)
	result := make([]ExperienceMemory, 0, len(memories))
	for _, memory := range memories {
		score := scoreExperienceMemory(memory, queryText)
		if len(query.Embedding) > 0 && len(memory.Embedding) > 0 {
			if vectorScore := cosineSimilarity(query.Embedding, memory.Embedding); vectorScore > score {
				score = vectorScore
			}
		}
		if query.Task != "" && score <= 0 {
			continue
		}
		memory.Score = score
		result = append(result, memory)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Score == result[j].Score {
			if result[i].SuccessCount == result[j].SuccessCount {
				return result[i].ID < result[j].ID
			}
			return result[i].SuccessCount > result[j].SuccessCount
		}
		return result[i].Score > result[j].Score
	})
	limit := query.Limit
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (repo *PostgresRepository) SaveFailureMemory(ctx context.Context, memory FailureMemory) error {
	now := time.Now().UTC()
	if memory.ID == "" {
		memory.ID = newID("fail")
	}
	if existing, err := getPayloadByID[FailureMemory](ctx, repo.pool, "failure_memories", memory.ID, "failure memory not found"); err == nil {
		if memory.CreatedAt.IsZero() {
			memory.CreatedAt = existing.CreatedAt
		}
		if memory.OccurrenceCount <= 0 {
			memory.OccurrenceCount = existing.OccurrenceCount + 1
		}
	}
	if memory.CreatedAt.IsZero() {
		memory.CreatedAt = now
	}
	if memory.LastSeenAt.IsZero() {
		memory.LastSeenAt = now
	}
	if memory.OccurrenceCount <= 0 {
		memory.OccurrenceCount = 1
	}
	if err := ValidateMemoryRecord(memory); err != nil {
		return err
	}
	payload, err := marshalPayload(memory)
	if err != nil {
		return err
	}
	_, err = repo.pool.Exec(ctx, `
insert into failure_memories (id, experience_id, project_id, site, page_state_id, action_name, failure_type, failure_summary, avoid_hint, created_at, last_seen_at, occurrence_count, payload)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
on conflict(id) do update set
  experience_id = excluded.experience_id,
  project_id = excluded.project_id,
  site = excluded.site,
  page_state_id = excluded.page_state_id,
  action_name = excluded.action_name,
  failure_type = excluded.failure_type,
  failure_summary = excluded.failure_summary,
  avoid_hint = excluded.avoid_hint,
  created_at = excluded.created_at,
  last_seen_at = excluded.last_seen_at,
  occurrence_count = excluded.occurrence_count,
  payload = excluded.payload`,
		memory.ID, memory.ExperienceID, memory.ProjectID, memory.Site, memory.PageStateID,
		memory.ActionName, memory.FailureType, memory.FailureSummary, memory.AvoidHint, memory.CreatedAt,
		memory.LastSeenAt, memory.OccurrenceCount, payload)
	return err
}

func (repo *PostgresRepository) SearchFailureMemories(ctx context.Context, query FailureMemorySearchQuery) ([]FailureMemory, error) {
	return repo.ListFailureMemories(ctx, query)
}

func (repo *PostgresRepository) ListFailureMemories(ctx context.Context, query FailureMemorySearchQuery) ([]FailureMemory, error) {
	where := []string{"true"}
	args := []any{}
	if query.ProjectID != "" {
		args = append(args, query.ProjectID)
		where = append(where, fmt.Sprintf("project_id = $%d", len(args)))
	}
	if query.Site != "" {
		args = append(args, query.Site)
		where = append(where, fmt.Sprintf("site = $%d", len(args)))
	}
	if query.PageStateID != "" {
		args = append(args, query.PageStateID)
		where = append(where, fmt.Sprintf("page_state_id = $%d", len(args)))
	}
	failures, err := listPayloads[FailureMemory](ctx, repo.pool,
		"select payload from failure_memories where "+strings.Join(where, " and ")+" order by created_at desc, id asc",
		args...)
	if err != nil {
		return nil, err
	}
	queryText := strings.ToLower(query.Task)
	result := make([]FailureMemory, 0, len(failures))
	for _, memory := range failures {
		score := scoreFailureMemory(memory, queryText)
		if query.Task != "" && score <= 0 {
			continue
		}
		memory.Score = score
		result = append(result, memory)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Score == result[j].Score {
			if result[i].CreatedAt.Equal(result[j].CreatedAt) {
				return result[i].ID < result[j].ID
			}
			return result[i].CreatedAt.After(result[j].CreatedAt)
		}
		return result[i].Score > result[j].Score
	})
	if query.Limit > 0 && len(result) > query.Limit {
		result = result[:query.Limit]
	}
	return result, nil
}

func (repo *PostgresRepository) SaveMemoryContextEvent(ctx context.Context, event MemoryContextEvent) error {
	if event.ID == "" {
		event.ID = newID("ctx")
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	payload, err := marshalPayload(event)
	if err != nil {
		return err
	}
	experienceIDs, err := json.Marshal(event.SelectedExperienceIDs)
	if err != nil {
		return err
	}
	knowledgeIDs, err := json.Marshal(event.SelectedKnowledgeChunkIDs)
	if err != nil {
		return err
	}
	workflowIDs, err := json.Marshal(event.SelectedWorkflowIDs)
	if err != nil {
		return err
	}
	evidenceRefs, err := json.Marshal(event.EvidenceRefs)
	if err != nil {
		return err
	}
	_, err = repo.pool.Exec(ctx, `
insert into memory_context_events (id, project_id, task, current_url, current_page_state, selected_experience_ids, selected_knowledge_chunk_ids, selected_workflow_ids, evidence_refs, recommended_mode, created_at, payload)
values ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8::jsonb, $9::jsonb, $10, $11, $12)
on conflict(id) do update set
  project_id = excluded.project_id,
  task = excluded.task,
  current_url = excluded.current_url,
  current_page_state = excluded.current_page_state,
  selected_experience_ids = excluded.selected_experience_ids,
  selected_knowledge_chunk_ids = excluded.selected_knowledge_chunk_ids,
  selected_workflow_ids = excluded.selected_workflow_ids,
  evidence_refs = excluded.evidence_refs,
  recommended_mode = excluded.recommended_mode,
  created_at = excluded.created_at,
  payload = excluded.payload`,
		event.ID, event.ProjectID, event.Task, event.CurrentURL, event.CurrentPageState,
		string(experienceIDs), string(knowledgeIDs), string(workflowIDs), string(evidenceRefs),
		string(event.RecommendedMode), event.CreatedAt, payload)
	return err
}

func (repo *PostgresRepository) GetMemoryContextEvent(ctx context.Context, id string) (MemoryContextEvent, error) {
	return getPayloadByID[MemoryContextEvent](ctx, repo.pool, "memory_context_events", id, "memory context event not found")
}

func (repo *PostgresRepository) SaveMemoryAttributionEvent(ctx context.Context, event MemoryAttributionEvent) error {
	if event.ID == "" {
		event.ID = newID("attr")
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	if err := ValidateSummaryLength(event.Reason); err != nil {
		return err
	}
	payload, err := marshalPayload(event)
	if err != nil {
		return err
	}
	signals, err := json.Marshal(event.Signals)
	if err != nil {
		return err
	}
	_, err = repo.pool.Exec(ctx, `
insert into memory_attribution_events (
  id, project_id, site, task_run_id, context_id, evidence_id, evidence_source, label, reason, signals, created_at, payload
) values (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb, $11, $12
)
on conflict(id) do update set
  project_id = excluded.project_id,
  site = excluded.site,
  task_run_id = excluded.task_run_id,
  context_id = excluded.context_id,
  evidence_id = excluded.evidence_id,
  evidence_source = excluded.evidence_source,
  label = excluded.label,
  reason = excluded.reason,
  signals = excluded.signals,
  created_at = excluded.created_at,
  payload = excluded.payload`,
		event.ID, event.ProjectID, event.Site, event.TaskRunID, event.ContextID, event.EvidenceID,
		string(event.EvidenceSource), string(event.Label), event.Reason, string(signals), event.CreatedAt, payload)
	return err
}

func (repo *PostgresRepository) ListMemoryAttributionEvents(ctx context.Context, query MemoryAttributionEventListQuery) ([]MemoryAttributionEvent, error) {
	where := []string{"true"}
	args := []any{}
	if query.ProjectID != "" {
		args = append(args, query.ProjectID)
		where = append(where, fmt.Sprintf("project_id = $%d", len(args)))
	}
	if query.Site != "" {
		args = append(args, query.Site)
		where = append(where, fmt.Sprintf("site = $%d", len(args)))
	}
	if query.TaskRunID != "" {
		args = append(args, query.TaskRunID)
		where = append(where, fmt.Sprintf("task_run_id = $%d", len(args)))
	}
	if query.ContextID != "" {
		args = append(args, query.ContextID)
		where = append(where, fmt.Sprintf("context_id = $%d", len(args)))
	}
	if query.EvidenceID != "" {
		args = append(args, query.EvidenceID)
		where = append(where, fmt.Sprintf("evidence_id = $%d", len(args)))
	}
	if query.EvidenceSource != "" {
		args = append(args, string(query.EvidenceSource))
		where = append(where, fmt.Sprintf("evidence_source = $%d", len(args)))
	}
	if query.Label != "" {
		args = append(args, string(query.Label))
		where = append(where, fmt.Sprintf("label = $%d", len(args)))
	}
	return listPayloads[MemoryAttributionEvent](ctx, repo.pool,
		"select payload from memory_attribution_events where "+strings.Join(where, " and ")+" order by created_at desc, id asc",
		args...)
}

func (repo *PostgresRepository) SaveMemoryEvidenceStats(ctx context.Context, stats MemoryEvidenceStats) error {
	if stats.LastFeedbackAt.IsZero() {
		stats.LastFeedbackAt = time.Now().UTC()
	}
	payload, err := marshalPayload(stats)
	if err != nil {
		return err
	}
	_, err = repo.pool.Exec(ctx, `
insert into memory_evidence_stats (
  project_id, site, evidence_id, evidence_source, helpful_count, unused_count, misleading_count, stale_count, neutral_count, utility_score, last_feedback_at, payload
) values (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
)
on conflict(project_id, evidence_source, evidence_id) do update set
  site = excluded.site,
  helpful_count = excluded.helpful_count,
  unused_count = excluded.unused_count,
  misleading_count = excluded.misleading_count,
  stale_count = excluded.stale_count,
  neutral_count = excluded.neutral_count,
  utility_score = excluded.utility_score,
  last_feedback_at = excluded.last_feedback_at,
  payload = excluded.payload`,
		stats.ProjectID, stats.Site, stats.EvidenceID, string(stats.EvidenceSource), stats.HelpfulCount,
		stats.UnusedCount, stats.MisleadingCount, stats.StaleCount, stats.NeutralCount, stats.UtilityScore,
		stats.LastFeedbackAt, payload)
	return err
}

func (repo *PostgresRepository) GetMemoryEvidenceStats(ctx context.Context, projectID string, source MemoryEvidenceSource, evidenceID string) (MemoryEvidenceStats, error) {
	return getPayload[MemoryEvidenceStats](ctx, repo.pool,
		"select payload from memory_evidence_stats where project_id = $1 and evidence_source = $2 and evidence_id = $3",
		"memory evidence stats not found", projectID, string(source), evidenceID)
}

func (repo *PostgresRepository) ListMemoryEvidenceStats(ctx context.Context, query MemoryEvidenceStatsListQuery) ([]MemoryEvidenceStats, error) {
	where := []string{"true"}
	args := []any{}
	if query.ProjectID != "" {
		args = append(args, query.ProjectID)
		where = append(where, fmt.Sprintf("project_id = $%d", len(args)))
	}
	if query.Site != "" {
		args = append(args, query.Site)
		where = append(where, fmt.Sprintf("site = $%d", len(args)))
	}
	if query.EvidenceID != "" {
		args = append(args, query.EvidenceID)
		where = append(where, fmt.Sprintf("evidence_id = $%d", len(args)))
	}
	if query.EvidenceSource != "" {
		args = append(args, string(query.EvidenceSource))
		where = append(where, fmt.Sprintf("evidence_source = $%d", len(args)))
	}
	return listPayloads[MemoryEvidenceStats](ctx, repo.pool,
		"select payload from memory_evidence_stats where "+strings.Join(where, " and ")+" order by utility_score desc, last_feedback_at desc, evidence_id asc",
		args...)
}

func (repo *PostgresRepository) SaveMemoryReview(ctx context.Context, review MemoryReview) error {
	if review.ID == "" {
		review.ID = newID("review")
	}
	if review.Status == "" {
		review.Status = ReviewStatusPending
	}
	if review.CreatedAt.IsZero() {
		review.CreatedAt = time.Now().UTC()
	}
	if err := ValidateMemoryRecord(review); err != nil {
		return err
	}
	payload, err := marshalPayload(review)
	if err != nil {
		return err
	}
	_, err = repo.pool.Exec(ctx, `
insert into memory_reviews (id, project_id, target_type, target_id, status, summary, created_at, reviewed_at, payload)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
on conflict(id) do update set
  project_id = excluded.project_id,
  target_type = excluded.target_type,
  target_id = excluded.target_id,
  status = excluded.status,
  summary = excluded.summary,
  created_at = excluded.created_at,
  reviewed_at = excluded.reviewed_at,
  payload = excluded.payload`,
		review.ID, review.ProjectID, string(review.TargetType), review.TargetID, string(review.Status),
		review.Summary, review.CreatedAt, nullableTime(review.ReviewedAt), payload)
	return err
}

func (repo *PostgresRepository) ListMemoryReviews(ctx context.Context, query MemoryReviewListQuery) ([]MemoryReview, error) {
	where := []string{"true"}
	args := []any{}
	if query.ProjectID != "" {
		args = append(args, query.ProjectID)
		where = append(where, fmt.Sprintf("project_id = $%d", len(args)))
	}
	if query.TargetType != "" {
		args = append(args, string(query.TargetType))
		where = append(where, fmt.Sprintf("target_type = $%d", len(args)))
	}
	if query.Status != "" {
		args = append(args, string(query.Status))
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	return listPayloads[MemoryReview](ctx, repo.pool,
		"select payload from memory_reviews where "+strings.Join(where, " and ")+" order by created_at asc, id asc",
		args...)
}

func (repo *PostgresRepository) ApproveMemoryReview(ctx context.Context, id string) error {
	return repo.updateMemoryReviewStatus(ctx, id, ReviewStatusApproved)
}

func (repo *PostgresRepository) RejectMemoryReview(ctx context.Context, id string) error {
	return repo.updateMemoryReviewStatus(ctx, id, ReviewStatusRejected)
}

func (repo *PostgresRepository) updateMemoryReviewStatus(ctx context.Context, id string, status ReviewStatus) error {
	review, err := getPayloadByID[MemoryReview](ctx, repo.pool, "memory_reviews", id, "memory review not found")
	if err != nil {
		return err
	}
	review.Status = status
	review.ReviewedAt = time.Now().UTC()
	payload, err := marshalPayload(review)
	if err != nil {
		return err
	}
	tag, err := repo.pool.Exec(ctx,
		"update memory_reviews set status = $2, reviewed_at = $3, payload = $4 where id = $1",
		id, string(status), review.ReviewedAt, payload)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("memory review not found")
	}
	return nil
}

func (repo *PostgresRepository) searchKnowledgeChunksByEmbedding(ctx context.Context, query KnowledgeSearchQuery) ([]KnowledgeChunk, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 3
	}
	rows, err := repo.pool.Query(ctx, `
select payload, 1 - (embedding <=> $2::vector) as score
from knowledge_chunks
where ($1 = '' or project_id = $1) and embedding is not null
order by embedding <=> $2::vector
limit $3`, query.ProjectID, vectorLiteral(query.Embedding), limit*3)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []KnowledgeChunk{}
	for rows.Next() {
		var raw []byte
		var score float64
		if err := rows.Scan(&raw, &score); err != nil {
			return nil, err
		}
		var chunk KnowledgeChunk
		if err := json.Unmarshal(raw, &chunk); err != nil {
			return nil, err
		}
		if !knowledgeChunkHardGateMatches(chunk, query) {
			continue
		}
		chunk.Score = score
		result = append(result, chunk)
		if len(result) >= limit {
			break
		}
	}
	return result, rows.Err()
}

func (repo *PostgresRepository) RankActiveWorkflowsByEmbedding(ctx context.Context, projectID string, workflowIDs []string, embedding []float32, limit int) ([]WorkflowEmbeddingHit, error) {
	if len(workflowIDs) == 0 || len(embedding) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 3
	}
	rows, err := repo.pool.Query(ctx, `
select payload, 1 - (embedding <=> $3::vector) as score
from workflow_recipes
where ($1 = '' or project_id = $1)
  and id = any($2)
  and status = 'active'
  and searchable = true
  and embedding is not null
order by embedding <=> $3::vector
limit $4`, projectID, workflowIDs, vectorLiteral(embedding), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hits := []WorkflowEmbeddingHit{}
	for rows.Next() {
		var raw []byte
		var score float64
		if err := rows.Scan(&raw, &score); err != nil {
			return nil, err
		}
		var workflow WorkflowRecipe
		if err := json.Unmarshal(raw, &workflow); err != nil {
			return nil, err
		}
		hits = append(hits, WorkflowEmbeddingHit{Workflow: workflow, Score: score})
	}
	return hits, rows.Err()
}

func (repo *PostgresRepository) SaveSelectorStats(ctx context.Context, stats SelectorStats) error {
	payload, err := marshalPayload(stats)
	if err != nil {
		return err
	}
	key := selectorStatsKey(stats)
	_, err = repo.pool.Exec(ctx, `
insert into selector_stats (key, workflow_id, version, payload)
values ($1, $2, $3, $4)
on conflict(key) do update set payload = excluded.payload`,
		key, stats.WorkflowID, stats.Version, payload)
	return err
}

func (repo *PostgresRepository) ListSelectorStats(ctx context.Context, workflowID string, version int) ([]SelectorStats, error) {
	where := []string{"true"}
	args := []any{}
	if workflowID != "" {
		args = append(args, workflowID)
		where = append(where, fmt.Sprintf("workflow_id = $%d", len(args)))
	}
	if version != 0 {
		args = append(args, version)
		where = append(where, fmt.Sprintf("version = $%d", len(args)))
	}
	return listPayloads[SelectorStats](ctx, repo.pool,
		"select payload from selector_stats where "+strings.Join(where, " and ")+" order by workflow_id, version, key",
		args...)
}

func (repo *PostgresRepository) SaveInterruptHandler(ctx context.Context, handler InterruptHandler) error {
	if handler.ID == "" {
		handler.ID = newID("ih")
	}
	payload, err := marshalPayload(handler)
	if err != nil {
		return err
	}
	_, err = repo.pool.Exec(ctx, `
insert into interrupt_handlers (id, project_id, site, status, payload)
values ($1, $2, $3, $4, $5)
on conflict(id) do update set project_id = excluded.project_id, site = excluded.site, status = excluded.status, payload = excluded.payload`,
		handler.ID, handler.ProjectID, handler.Site, string(handler.Status), payload)
	return err
}

func (repo *PostgresRepository) ListActiveInterruptHandlers(ctx context.Context, projectID string) ([]InterruptHandler, error) {
	args := []any{string(StatusActive)}
	where := []string{"status = $1"}
	if projectID != "" {
		args = append(args, projectID)
		where = append(where, fmt.Sprintf("project_id = $%d", len(args)))
	}
	return listPayloads[InterruptHandler](ctx, repo.pool,
		"select payload from interrupt_handlers where "+strings.Join(where, " and ")+" order by id asc",
		args...)
}

func (repo *PostgresRepository) SaveRepairPatch(ctx context.Context, patch RepairPatch) error {
	if patch.ID == "" {
		patch.ID = newID("patch")
	}
	if patch.CreatedAt.IsZero() {
		patch.CreatedAt = time.Now().UTC()
	}
	payload, err := marshalPayload(patch)
	if err != nil {
		return err
	}
	_, err = repo.pool.Exec(ctx, `
insert into repair_patches (id, project_id, workflow_id, status, created_at, payload)
values ($1, $2, $3, $4, $5, $6)
on conflict(id) do update set project_id = excluded.project_id, workflow_id = excluded.workflow_id, status = excluded.status, created_at = excluded.created_at, payload = excluded.payload`,
		patch.ID, patch.ProjectID, patch.WorkflowID, string(patch.Status), patch.CreatedAt, payload)
	return err
}

func (repo *PostgresRepository) GetRepairPatch(ctx context.Context, id string) (RepairPatch, error) {
	return getPayloadByID[RepairPatch](ctx, repo.pool, "repair_patches", id, "repair patch not found")
}

func (repo *PostgresRepository) ListRepairPatches(ctx context.Context, query RepairPatchListQuery) ([]RepairPatch, error) {
	where := []string{"true"}
	args := []any{}
	if query.ProjectID != "" {
		args = append(args, query.ProjectID)
		where = append(where, fmt.Sprintf("project_id = $%d", len(args)))
	}
	if query.WorkflowID != "" {
		args = append(args, query.WorkflowID)
		where = append(where, fmt.Sprintf("workflow_id = $%d", len(args)))
	}
	if query.Status != "" {
		args = append(args, string(query.Status))
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	return listPayloads[RepairPatch](ctx, repo.pool,
		"select payload from repair_patches where "+strings.Join(where, " and ")+" order by created_at desc, id asc",
		args...)
}

func (repo *PostgresRepository) ListActiveRepairPatches(ctx context.Context, projectID string) ([]RepairPatch, error) {
	return repo.ListRepairPatches(ctx, RepairPatchListQuery{ProjectID: projectID, Status: StatusActive})
}

func (repo *PostgresRepository) AppendOutboxEvent(ctx context.Context, event OutboxEvent) (OutboxEvent, error) {
	if event.ID == "" {
		event.ID = newID("evt")
	}
	if event.Status == "" {
		event.Status = StatusPendingReview
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	payload, err := marshalPayload(event)
	if err != nil {
		return OutboxEvent{}, err
	}
	_, err = repo.pool.Exec(ctx, `
insert into workflow_outbox (id, type, idempotency_key, status, created_at, payload)
values ($1, $2, $3, $4, $5, $6)
on conflict(idempotency_key) do nothing`,
		event.ID, event.Type, event.IdempotencyKey, string(event.Status), event.CreatedAt, payload)
	if err != nil {
		return OutboxEvent{}, err
	}
	return getPayload[OutboxEvent](ctx, repo.pool,
		"select payload from workflow_outbox where idempotency_key = $1",
		"outbox event not found", event.IdempotencyKey)
}

func (repo *PostgresRepository) ListOutboxEvents(ctx context.Context, status Status) ([]OutboxEvent, error) {
	where := "true"
	args := []any{}
	if status != "" {
		args = append(args, string(status))
		where = "status = $1"
	}
	return listPayloads[OutboxEvent](ctx, repo.pool,
		"select payload from workflow_outbox where "+where+" order by created_at asc, id asc",
		args...)
}

func (repo *PostgresRepository) PruneMemory(ctx context.Context, policy MemoryPrunePolicy) (MemoryPruneResult, error) {
	result := MemoryPruneResult{}
	var err error
	if policy.MaxTaskRuns > 0 {
		result.DeletedTaskRuns, err = repo.pruneByLimit(ctx, "task_runs", memoryPruneWhere(policy, true), "created_at", policy.MaxTaskRuns)
		if err != nil {
			return MemoryPruneResult{}, err
		}
	}
	if policy.MaxPageObservationEvents > 0 {
		result.DeletedPageObservationEvents, err = repo.pruneByLimit(ctx, "page_observation_events", memoryPruneWhere(policy, true), "coalesce(last_seen_at, created_at)", policy.MaxPageObservationEvents)
		if err != nil {
			return MemoryPruneResult{}, err
		}
	}
	if policy.MaxMemoryContextEvents > 0 {
		result.DeletedMemoryContextEvents, err = repo.pruneByLimit(ctx, "memory_context_events", memoryPruneWhere(policy, false), "created_at", policy.MaxMemoryContextEvents)
		if err != nil {
			return MemoryPruneResult{}, err
		}
	}
	if policy.MaxFailureMemories > 0 {
		result.DeletedFailureMemories, err = repo.pruneByLimit(ctx, "failure_memories", memoryPruneWhere(policy, true), "coalesce(last_seen_at, created_at)", policy.MaxFailureMemories)
		if err != nil {
			return MemoryPruneResult{}, err
		}
	}
	return result, nil
}

func (repo *PostgresRepository) pruneByLimit(ctx context.Context, table string, where memoryPruneSQL, orderColumn string, limit int) (int, error) {
	if limit <= 0 {
		return 0, nil
	}
	query := fmt.Sprintf(`
with stale as (
  select id from %s
  where %s
  order by %s desc, id asc
  offset $%d
)
delete from %s where id in (select id from stale)`, table, where.sql, orderColumn, len(where.args)+1, table)
	args := append(where.args, limit)
	tag, err := repo.pool.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

type memoryPruneSQL struct {
	sql  string
	args []any
}

func memoryPruneWhere(policy MemoryPrunePolicy, includeSite bool) memoryPruneSQL {
	where := []string{"true"}
	args := []any{}
	if policy.ProjectID != "" {
		args = append(args, policy.ProjectID)
		where = append(where, fmt.Sprintf("project_id = $%d", len(args)))
	}
	if includeSite && policy.Site != "" {
		args = append(args, policy.Site)
		where = append(where, fmt.Sprintf("site = $%d", len(args)))
	}
	return memoryPruneSQL{sql: strings.Join(where, " and "), args: args}
}

func marshalPayload(value any) ([]byte, error) {
	return json.Marshal(value)
}

func getPayloadByID[T any](ctx context.Context, pool *pgxpool.Pool, table string, id string, notFoundMessage string) (T, error) {
	return getPayload[T](ctx, pool, "select payload from "+table+" where id = $1", notFoundMessage, id)
}

func getPayload[T any](ctx context.Context, pool *pgxpool.Pool, query string, notFoundMessage string, args ...any) (T, error) {
	var zero T
	var raw []byte
	if err := pool.QueryRow(ctx, query, args...).Scan(&raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return zero, errors.New(notFoundMessage)
		}
		return zero, err
	}
	var value T
	if err := json.Unmarshal(raw, &value); err != nil {
		return zero, err
	}
	return value, nil
}

func listPayloads[T any](ctx context.Context, pool *pgxpool.Pool, query string, args ...any) ([]T, error) {
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []T{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var value T
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func vectorLiteral(vector []float32) string {
	if len(vector) == 0 {
		return ""
	}
	parts := make([]string, 0, len(vector))
	for _, value := range vector {
		parts = append(parts, strconv.FormatFloat(float64(value), 'g', -1, 32))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}
