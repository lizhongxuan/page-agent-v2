package registry

import (
	"context"
	"errors"
	"strconv"
	"time"
)

type RecordedSession struct {
	ProjectID string
	Source    string
	Task      string
	StartURL  string
	Recipe    WorkflowRecipe
}

type CandidateService struct {
	repo Repository
}

func NewCandidateService(repo Repository) *CandidateService {
	return &CandidateService{repo: repo}
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
		ID:          newID("cand"),
		ProjectID:   session.ProjectID,
		Source:      session.Source,
		Task:        session.Task,
		StartURL:    session.StartURL,
		RecipeDraft: recipe,
		Status:      StatusPendingReview,
		Searchable:  false,
		CreatedAt:   now,
	}
	if err := service.repo.SaveCandidate(ctx, candidate); err != nil {
		return WorkflowCandidate{}, err
	}
	return candidate, nil
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
		Summary:    "approved candidate " + candidate.ID,
		CreatedAt:  time.Now().UTC(),
	}); err != nil {
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
