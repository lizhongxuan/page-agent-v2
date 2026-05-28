package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type FileRepository struct {
	mu        sync.RWMutex
	path      string
	workflows map[string]WorkflowRecipe
}

func NewFileRepository(path string) (*FileRepository, error) {
	repo := &FileRepository{
		path:      path,
		workflows: map[string]WorkflowRecipe{},
	}
	if err := repo.load(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (repo *FileRepository) Save(_ context.Context, recipe WorkflowRecipe) (WorkflowRecipe, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	now := time.Now().UTC()
	if recipe.CreatedAt.IsZero() {
		recipe.CreatedAt = now
	}
	recipe.UpdatedAt = now
	repo.workflows[recipe.ID] = recipe
	return recipe, repo.persist()
}

func (repo *FileRepository) Get(_ context.Context, id string) (WorkflowRecipe, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	recipe, ok := repo.workflows[id]
	if !ok {
		return WorkflowRecipe{}, errors.New("workflow not found")
	}
	return recipe, nil
}

func (repo *FileRepository) List(_ context.Context, projectID string) ([]WorkflowRecipe, error) {
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

func (repo *FileRepository) load() error {
	content, err := os.ReadFile(repo.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return json.Unmarshal(content, &repo.workflows)
}

func (repo *FileRepository) persist() error {
	if err := os.MkdirAll(filepath.Dir(repo.path), 0o755); err != nil {
		return err
	}
	content, err := json.MarshalIndent(repo.workflows, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(repo.path, content, 0o600)
}
