package registry

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type RecordedSession struct {
	ProjectID       string
	Source          string
	SourceTaskRunID string
	Task            string
	StartURL        string
	Recipe          WorkflowRecipe
}

type DenseVectorizer interface {
	DenseQuery(context.Context, string) ([]float32, error)
}

type CandidateService struct {
	repo       Repository
	vectorizer DenseVectorizer
}

func NewCandidateService(repo Repository) *CandidateService {
	return NewCandidateServiceWithVectorizer(repo, nil)
}

func NewCandidateServiceWithVectorizer(repo Repository, vectorizer DenseVectorizer) *CandidateService {
	return &CandidateService{repo: repo, vectorizer: vectorizer}
}

func (service *CandidateService) CreateFromSession(ctx context.Context, session RecordedSession) (WorkflowCandidate, error) {
	if session.ProjectID == "" {
		return WorkflowCandidate{}, errors.New("project id is required")
	}
	if session.Source == "" {
		return WorkflowCandidate{}, errors.New("source is required")
	}
	if session.Task == "" {
		return WorkflowCandidate{}, errors.New("task is required")
	}
	recipe := session.Recipe
	recipe.ProjectID = session.ProjectID
	recipe.Status = StatusPendingReview
	recipe.Searchable = false
	if recipe.Version == 0 {
		recipe.Version = 1
	}
	if err := ValidateWorkflowRecipeForCandidate(recipe); err != nil {
		return WorkflowCandidate{}, err
	}
	now := time.Now().UTC()
	candidate := WorkflowCandidate{
		ID:              newID("cand"),
		ProjectID:       session.ProjectID,
		Source:          session.Source,
		SourceTaskRunID: session.SourceTaskRunID,
		Task:            session.Task,
		StartURL:        session.StartURL,
		RecipeDraft:     recipe,
		Status:          StatusPendingReview,
		Searchable:      false,
		CreatedAt:       now,
	}
	if err := service.repo.SaveCandidate(ctx, candidate); err != nil {
		return WorkflowCandidate{}, err
	}
	return candidate, nil
}

func (service *CandidateService) CreateFromTaskRun(ctx context.Context, taskRunID string, source string) (WorkflowCandidate, error) {
	if taskRunID == "" {
		return WorkflowCandidate{}, errors.New("task run id is required")
	}
	if source == "" {
		source = "task_run"
	}
	run, err := service.repo.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return WorkflowCandidate{}, err
	}
	if run.Status != TaskRunSuccess {
		return WorkflowCandidate{}, errors.New("only successful task runs can create workflow candidates")
	}
	recipe := workflowRecipeFromTaskRun(run)
	return service.CreateFromSession(ctx, RecordedSession{
		ProjectID:       run.ProjectID,
		Source:          source,
		SourceTaskRunID: run.ID,
		Task:            run.TaskTemplate,
		StartURL:        "https://" + run.Site,
		Recipe:          recipe,
	})
}

func (service *CandidateService) Approve(ctx context.Context, id string) (WorkflowCandidate, error) {
	candidate, err := service.repo.GetCandidate(ctx, id)
	if err != nil {
		return WorkflowCandidate{}, err
	}
	recipe := candidate.RecipeDraft
	recipe.Status = StatusActive
	recipe.Searchable = true
	recipe.UpdatedAt = time.Now().UTC()
	recipe.Version = service.nextWorkflowVersion(ctx, recipe)
	if service.vectorizer != nil {
		embedding, err := service.vectorizer.DenseQuery(ctx, workflowEmbeddingText(recipe))
		if err != nil {
			return WorkflowCandidate{}, err
		}
		recipe.Embedding = embedding
	}
	if err := ValidateWorkflowRecipe(recipe); err != nil {
		return WorkflowCandidate{}, err
	}
	candidate.Status = StatusActive
	candidate.Searchable = true
	candidate.RecipeDraft = recipe
	candidate.ReviewedAt = time.Now().UTC()
	if err := service.repo.SaveWorkflow(ctx, recipe); err != nil {
		return WorkflowCandidate{}, err
	}
	if err := service.repo.SaveVersion(ctx, WorkflowVersion{
		WorkflowID: recipe.ID,
		Version:    recipe.Version,
		Recipe:     recipe,
		Summary:    workflowVersionSummary(candidate),
		CreatedAt:  time.Now().UTC(),
	}); err != nil {
		return WorkflowCandidate{}, err
	}
	if err := service.saveWorkflowPageGraph(ctx, recipe); err != nil {
		return WorkflowCandidate{}, err
	}
	if err := service.repo.SaveCandidate(ctx, candidate); err != nil {
		return WorkflowCandidate{}, err
	}
	_, err = service.repo.AppendOutboxEvent(ctx, OutboxEvent{
		Type:           "workflow_approved",
		IdempotencyKey: "workflow_approved:" + recipe.ID + ":v" + strconv.Itoa(recipe.Version),
		Payload: map[string]any{
			"workflowId": recipe.ID,
			"version":    recipe.Version,
			"projectId":  recipe.ProjectID,
		},
	})
	if err != nil {
		return WorkflowCandidate{}, err
	}
	return candidate, nil
}

func (service *CandidateService) Rollback(ctx context.Context, workflowID string, version int) (WorkflowRecipe, error) {
	if workflowID == "" {
		return WorkflowRecipe{}, errors.New("workflow id is required")
	}
	if version <= 0 {
		return WorkflowRecipe{}, errors.New("workflow version is required")
	}
	target, err := service.repo.GetVersion(ctx, workflowID, version)
	if err != nil {
		return WorkflowRecipe{}, err
	}
	recipe := target.Recipe
	if recipe.ID == "" {
		recipe.ID = workflowID
	}
	if recipe.ID != workflowID {
		return WorkflowRecipe{}, errors.New("workflow version does not match workflow id")
	}
	recipe.Status = StatusActive
	recipe.Searchable = true
	recipe.Version = version
	recipe.UpdatedAt = time.Now().UTC()
	if len(recipe.Embedding) == 0 && service.vectorizer != nil {
		embedding, err := service.vectorizer.DenseQuery(ctx, workflowEmbeddingText(recipe))
		if err != nil {
			return WorkflowRecipe{}, err
		}
		recipe.Embedding = embedding
	}
	if err := ValidateWorkflowRecipe(recipe); err != nil {
		return WorkflowRecipe{}, err
	}
	if err := service.repo.SaveWorkflow(ctx, recipe); err != nil {
		return WorkflowRecipe{}, err
	}
	if err := service.saveWorkflowPageGraph(ctx, recipe); err != nil {
		return WorkflowRecipe{}, err
	}
	_, err = service.repo.AppendOutboxEvent(ctx, OutboxEvent{
		Type:           "workflow_rolled_back",
		IdempotencyKey: "workflow_rolled_back:" + recipe.ID + ":v" + strconv.Itoa(recipe.Version),
		Payload: map[string]any{
			"workflowId": recipe.ID,
			"version":    recipe.Version,
			"projectId":  recipe.ProjectID,
		},
	})
	if err != nil {
		return WorkflowRecipe{}, err
	}
	return recipe, nil
}

func (service *CandidateService) Reject(ctx context.Context, id string) (WorkflowCandidate, error) {
	candidate, err := service.repo.GetCandidate(ctx, id)
	if err != nil {
		return WorkflowCandidate{}, err
	}
	candidate.Status = StatusRejected
	candidate.Searchable = false
	candidate.ReviewedAt = time.Now().UTC()
	if err := service.repo.SaveCandidate(ctx, candidate); err != nil {
		return WorkflowCandidate{}, err
	}
	return candidate, nil
}

func (service *CandidateService) List(ctx context.Context, query CandidateListQuery) ([]WorkflowCandidate, error) {
	return service.repo.ListCandidates(ctx, query)
}

func (service *CandidateService) ListVersions(ctx context.Context, workflowID string) ([]WorkflowVersion, error) {
	if workflowID == "" {
		return nil, errors.New("workflow id is required")
	}
	return service.repo.ListVersions(ctx, workflowID)
}

func ValidateWorkflowRecipeForCandidate(recipe WorkflowRecipe) error {
	if recipe.ID == "" {
		return errors.New("workflow id is required")
	}
	if recipe.ProjectID == "" {
		return errors.New("project id is required")
	}
	if recipe.Site == "" {
		return errors.New("workflow site is required")
	}
	if recipe.Intent == "" {
		return errors.New("workflow intent is required")
	}
	if len(recipe.Variables) == 0 {
		return errors.New("workflow variables are required")
	}
	if len(recipe.Chunks) == 0 {
		return errors.New("workflow chunks are required")
	}
	return nil
}

func workflowRecipeFromTaskRun(run TaskRun) WorkflowRecipe {
	path := run.OptimizedPath
	if len(path) == 0 {
		path = run.OriginalPath
	}
	steps := workflowStepsFromTaskRun(run)
	return WorkflowRecipe{
		ID:              newID("wf"),
		Version:         1,
		ProjectID:       run.ProjectID,
		Status:          StatusPendingReview,
		Searchable:      false,
		Site:            run.Site,
		Name:            truncateCandidateText(firstNonEmpty(run.Summary, run.TaskTemplate, "Workflow from task run")),
		Intent:          truncateCandidateText(firstNonEmpty(run.TaskTemplate, run.Summary, "Workflow from task run")),
		Description:     truncateCandidateText(run.Summary),
		RiskLevel:       RiskReadOrSearch,
		Variables:       variablesFromTaskRun(run),
		Chunks:          workflowChunksFromTaskRun(run, path, steps),
		StartPageStates: firstPageState(path),
		EndPageStates:   lastPageState(path),
	}
}

func workflowStepsFromTaskRun(run TaskRun) []WorkflowStep {
	optimizedSet := stringSet(run.OptimizedPath)
	steps := make([]WorkflowStep, 0, len(run.ActionSteps))
	for _, step := range run.ActionSteps {
		if !step.IsReusableSuccessful() {
			continue
		}
		if len(optimizedSet) > 0 && step.PageStateID != "" && !optimizedSet[step.PageStateID] {
			continue
		}
		steps = append(steps, WorkflowStep{
			ID:   firstNonEmpty(step.ID, "step_"+strconv.Itoa(step.StepIndex)),
			Type: step.ActionType,
			Target: StepTarget{
				Primary: TargetCandidate{Strategy: TargetText, Name: step.TargetName},
			},
			Value:     step.ValueTemplate,
			RiskLevel: RiskReadOrSearch,
		})
	}
	if len(steps) == 0 {
		steps = append(steps, WorkflowStep{
			ID:        "observe",
			Type:      StepWait,
			RiskLevel: RiskReadOnly,
		})
	}
	return steps
}

func workflowChunksFromTaskRun(run TaskRun, path []string, steps []WorkflowStep) []WorkflowChunk {
	return []WorkflowChunk{
		{
			ID:            "chunk_" + firstNonEmpty(run.ID, "task_run"),
			Name:          truncateCandidateText(firstNonEmpty(run.Summary, run.TaskTemplate, "Task run path")),
			FromPageState: firstString(path),
			ToPageState:   lastString(path),
			StepSummary:   truncateCandidateText(firstNonEmpty(run.Summary, run.TaskTemplate)),
			RiskLevel:     RiskReadOrSearch,
			Steps:         steps,
			VariableNames: variableNamesFromVariables(variablesFromTaskRun(run)),
		},
	}
}

var templateVariablePattern = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)

func variablesFromTaskRun(run TaskRun) []Variable {
	seen := map[string]bool{}
	variables := []Variable{}
	collect := func(text string) {
		for _, match := range templateVariablePattern.FindAllStringSubmatch(text, -1) {
			if len(match) != 2 || seen[match[1]] {
				continue
			}
			seen[match[1]] = true
			variables = append(variables, Variable{
				Name:     match[1],
				Type:     VariableString,
				Required: true,
				Source:   VariableSourceTask,
			})
		}
	}
	collect(run.TaskTemplate)
	for _, step := range run.ActionSteps {
		collect(step.ValueTemplate)
	}
	return variables
}

func variableNamesFromVariables(variables []Variable) []string {
	names := make([]string, 0, len(variables))
	for _, variable := range variables {
		names = append(names, variable.Name)
	}
	return names
}

func stringSet(values []string) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		if value != "" {
			result[value] = true
		}
	}
	return result
}

func firstPageState(path []string) []string {
	if value := firstString(path); value != "" {
		return []string{value}
	}
	return nil
}

func lastPageState(path []string) []string {
	if value := lastString(path); value != "" {
		return []string{value}
	}
	return nil
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func lastString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[len(values)-1]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func truncateCandidateText(value string) string {
	runes := []rune(value)
	if len(runes) <= MaxSummaryChars {
		return value
	}
	return string(runes[:MaxSummaryChars])
}

func workflowVersionSummary(candidate WorkflowCandidate) string {
	route := append([]string{}, candidate.RecipeDraft.StartPageStates...)
	route = append(route, candidate.RecipeDraft.EndPageStates...)
	if len(route) == 0 {
		return truncateCandidateText("approved candidate " + candidate.ID)
	}
	return truncateCandidateText("approved candidate " + candidate.ID + " with optimized route " + strings.Join(route, " -> "))
}

func (service *CandidateService) nextWorkflowVersion(ctx context.Context, recipe WorkflowRecipe) int {
	next := recipe.Version
	if next <= 0 {
		next = 1
	}
	if existing, err := service.repo.GetWorkflow(ctx, recipe.ID); err == nil && existing.Version >= next {
		next = existing.Version + 1
	}
	versions, err := service.repo.ListVersions(ctx, recipe.ID)
	if err != nil {
		return next
	}
	for _, version := range versions {
		if version.Version >= next {
			next = version.Version + 1
		}
	}
	return next
}

func (service *CandidateService) saveWorkflowPageGraph(ctx context.Context, recipe WorkflowRecipe) error {
	pageIDs := workflowPageStateIDs(recipe)
	for _, pageID := range pageIDs {
		state := PageState{
			ID:             pageID,
			ProjectID:      recipe.ProjectID,
			Site:           recipe.Site,
			App:            recipe.App,
			CanonicalTitle: pageID,
			Status:         StatusActive,
			UpdatedAt:      recipe.UpdatedAt,
		}
		if service.vectorizer != nil {
			embedding, err := service.vectorizer.DenseQuery(ctx, pageStateEmbeddingText(state))
			if err != nil {
				return err
			}
			state.Embedding = embedding
		}
		if err := service.repo.SavePageState(ctx, state); err != nil {
			return err
		}
	}
	for _, chunk := range recipe.Chunks {
		if chunk.FromPageState == "" || chunk.ToPageState == "" {
			continue
		}
		transition := PageTransition{
			ID:            workflowTransitionID(recipe, chunk),
			ProjectID:     recipe.ProjectID,
			Site:          recipe.Site,
			FromPageState: chunk.FromPageState,
			ToPageState:   chunk.ToPageState,
			WorkflowID:    recipe.ID,
			Version:       recipe.Version,
			ChunkID:       chunk.ID,
			RiskLevel:     chunk.RiskLevel,
			GuardRules:    transitionGuardRules(chunk),
		}
		if len(chunk.Steps) > 0 {
			step := chunk.Steps[0]
			transition.ActionName = string(step.Type)
			transition.TargetName = firstNonEmpty(step.Target.Primary.Name, step.Target.Primary.Value, step.Target.Primary.Role)
			transition.TargetCandidates = step.Target
		}
		if err := service.repo.SavePageTransition(ctx, transition); err != nil {
			return err
		}
	}
	return nil
}

func workflowPageStateIDs(recipe WorkflowRecipe) []string {
	seen := map[string]bool{}
	for _, pageID := range recipe.StartPageStates {
		if pageID != "" {
			seen[pageID] = true
		}
	}
	for _, pageID := range recipe.EndPageStates {
		if pageID != "" {
			seen[pageID] = true
		}
	}
	for _, chunk := range recipe.Chunks {
		if chunk.FromPageState != "" {
			seen[chunk.FromPageState] = true
		}
		if chunk.ToPageState != "" {
			seen[chunk.ToPageState] = true
		}
	}
	pageIDs := make([]string, 0, len(seen))
	for pageID := range seen {
		pageIDs = append(pageIDs, pageID)
	}
	sort.Strings(pageIDs)
	return pageIDs
}

func workflowTransitionID(recipe WorkflowRecipe, chunk WorkflowChunk) string {
	return "transition_" + recipe.ID + "_v" + strconv.Itoa(recipe.Version) + "_" + chunk.ID
}

func transitionGuardRules(chunk WorkflowChunk) HardRules {
	text := firstNonEmpty(chunk.PostconditionText)
	if text == "" {
		return HardRules{}
	}
	return HardRules{TextAll: []string{text}}
}

func workflowEmbeddingText(recipe WorkflowRecipe) string {
	parts := []string{
		recipe.Name,
		recipe.Intent,
		recipe.Description,
		recipe.Site,
		recipe.App,
		strings.Join(recipe.Tags, " "),
		"start pages " + strings.Join(recipe.StartPageStates, " "),
		"end pages " + strings.Join(recipe.EndPageStates, " "),
	}
	for _, variable := range recipe.Variables {
		if variable.Sensitive {
			continue
		}
		parts = append(parts, "variable "+variable.Name)
	}
	for _, chunk := range recipe.Chunks {
		parts = append(parts, chunk.Name, chunk.FromPageState, chunk.ToPageState, chunk.PreconditionText, chunk.PostconditionText, chunk.StepSummary)
		for _, step := range chunk.Steps {
			parts = append(parts, string(step.Type), step.Target.Primary.Text())
			for _, fallback := range step.Target.Fallbacks {
				parts = append(parts, fallback.Text())
			}
		}
	}
	return truncateCandidateText(strings.Join(parts, "\n"))
}

func pageStateEmbeddingText(state PageState) string {
	return truncateCandidateText(strings.Join([]string{
		state.CanonicalTitle,
		state.Site,
		state.App,
		state.URLPattern,
		strings.Join(state.RequiredText, " "),
	}, "\n"))
}
