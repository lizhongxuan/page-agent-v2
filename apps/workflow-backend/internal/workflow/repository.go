package workflow

import (
	"context"
	"errors"
	"sync"
	"time"
)

type Repository interface {
	Save(context.Context, WorkflowRecipe) (WorkflowRecipe, error)
	Get(context.Context, string) (WorkflowRecipe, error)
	List(context.Context, string) ([]WorkflowRecipe, error)
}

type MemoryRepository struct {
	mu        sync.RWMutex
	workflows map[string]WorkflowRecipe
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{workflows: map[string]WorkflowRecipe{}}
}

func (repo *MemoryRepository) Save(_ context.Context, recipe WorkflowRecipe) (WorkflowRecipe, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	now := time.Now().UTC()
	if recipe.CreatedAt.IsZero() {
		recipe.CreatedAt = now
	}
	recipe.UpdatedAt = now
	repo.workflows[recipe.ID] = recipe
	return recipe, nil
}

func (repo *MemoryRepository) Get(_ context.Context, id string) (WorkflowRecipe, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	recipe, ok := repo.workflows[id]
	if !ok {
		return WorkflowRecipe{}, errors.New("workflow not found")
	}
	return recipe, nil
}

func (repo *MemoryRepository) List(_ context.Context, projectID string) ([]WorkflowRecipe, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	result := make([]WorkflowRecipe, 0)
	for _, recipe := range repo.workflows {
		if recipe.ProjectID == projectID {
			result = append(result, recipe)
		}
	}
	return result, nil
}
