package recipe

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

type FileCandidateRepository struct {
	mu         sync.RWMutex
	path       string
	candidates map[string]WorkflowCandidate
}

func NewFileCandidateRepository(path string) (*FileCandidateRepository, error) {
	repo := &FileCandidateRepository{
		path:       path,
		candidates: map[string]WorkflowCandidate{},
	}
	if err := repo.load(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (repo *FileCandidateRepository) Save(_ context.Context, candidate WorkflowCandidate) (WorkflowCandidate, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.candidates[candidate.ID] = candidate
	return candidate, repo.persist()
}

func (repo *FileCandidateRepository) Get(_ context.Context, id string) (WorkflowCandidate, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	candidate, ok := repo.candidates[id]
	if !ok {
		return WorkflowCandidate{}, errors.New("workflow candidate not found")
	}
	return candidate, nil
}

func (repo *FileCandidateRepository) List(_ context.Context, query CandidateListQuery) ([]WorkflowCandidate, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	result := make([]WorkflowCandidate, 0)
	for _, candidate := range repo.candidates {
		if query.ProjectID != "" && candidate.ProjectID != query.ProjectID {
			continue
		}
		if query.Source != "" && candidate.Source != query.Source {
			continue
		}
		if query.ReviewStatus != "" && candidate.ReviewStatus != query.ReviewStatus {
			continue
		}
		result = append(result, candidate)
	}
	sortCandidates(result)
	return result, nil
}

func sortCandidates(candidates []WorkflowCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].CreatedAt.Equal(candidates[j].CreatedAt) {
			return candidates[i].ID < candidates[j].ID
		}
		return candidates[i].CreatedAt.After(candidates[j].CreatedAt)
	})
}

func (repo *FileCandidateRepository) load() error {
	content, err := os.ReadFile(repo.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return json.Unmarshal(content, &repo.candidates)
}

func (repo *FileCandidateRepository) persist() error {
	if err := os.MkdirAll(filepath.Dir(repo.path), 0o755); err != nil {
		return err
	}
	content, err := json.MarshalIndent(repo.candidates, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(repo.path, content, 0o600)
}
