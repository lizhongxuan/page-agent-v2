package artifact

import (
	"context"
	"errors"
	"sync"
)

type Service struct {
	store Store
	repo  Repository
}

func NewService(store Store, repo Repository) *Service {
	return &Service{store: store, repo: repo}
}

func (service *Service) Store(ctx context.Context, request PutRequest) (Artifact, error) {
	artifact, err := service.store.Put(ctx, request)
	if err != nil {
		return Artifact{}, err
	}
	if err := service.repo.Save(ctx, artifact); err != nil {
		_ = service.store.Delete(ctx, artifact)
		return Artifact{}, err
	}
	return artifact, nil
}

func (service *Service) List(ctx context.Context, query ListQuery) ([]Artifact, error) {
	return service.repo.List(ctx, query)
}

func (service *Service) ListAll(ctx context.Context) ([]Artifact, error) {
	return service.repo.ListAll(ctx)
}

func (service *Service) Get(ctx context.Context, id string) (Artifact, error) {
	return service.repo.Get(ctx, id)
}

func (service *Service) Delete(ctx context.Context, id string) error {
	artifact, err := service.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := service.store.Delete(ctx, artifact); err != nil {
		return err
	}
	return service.repo.Delete(ctx, id)
}

type MemoryRepository struct {
	mu        sync.RWMutex
	artifacts map[string]Artifact
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{artifacts: map[string]Artifact{}}
}

func (repo *MemoryRepository) Save(_ context.Context, artifact Artifact) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.artifacts[artifact.ID] = artifact
	return nil
}

func (repo *MemoryRepository) List(_ context.Context, query ListQuery) ([]Artifact, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	result := make([]Artifact, 0)
	for _, artifact := range repo.artifacts {
		if query.ProjectID != "" && artifact.ProjectID != query.ProjectID {
			continue
		}
		if query.OwnerType != "" && artifact.OwnerType != query.OwnerType {
			continue
		}
		if query.OwnerID != "" && artifact.OwnerID != query.OwnerID {
			continue
		}
		result = append(result, artifact)
	}
	return result, nil
}

func (repo *MemoryRepository) ListAll(_ context.Context) ([]Artifact, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	result := make([]Artifact, 0, len(repo.artifacts))
	for _, artifact := range repo.artifacts {
		result = append(result, artifact)
	}
	return result, nil
}

func (repo *MemoryRepository) Get(_ context.Context, id string) (Artifact, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	artifact, ok := repo.artifacts[id]
	if !ok {
		return Artifact{}, errors.New("artifact not found")
	}
	return artifact, nil
}

func (repo *MemoryRepository) Delete(_ context.Context, id string) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	delete(repo.artifacts, id)
	return nil
}
