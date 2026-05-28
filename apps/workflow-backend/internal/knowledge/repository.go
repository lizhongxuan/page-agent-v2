package knowledge

import (
	"context"
	"errors"
	"sync"
)

type Repository interface {
	Save(context.Context, Document) (Document, error)
	Get(context.Context, string, string) (Document, error)
	List(context.Context, string) ([]Document, error)
	Delete(context.Context, string, string) error
}

type MemoryRepository struct {
	mu        sync.RWMutex
	documents map[string]Document
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{documents: map[string]Document{}}
}

func (repo *MemoryRepository) Save(_ context.Context, doc Document) (Document, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.documents[doc.ID] = doc
	return doc, nil
}

func (repo *MemoryRepository) Get(_ context.Context, projectID string, id string) (Document, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	doc, ok := repo.documents[id]
	if !ok || doc.ProjectID != projectID {
		return Document{}, errors.New("knowledge document not found")
	}
	return doc, nil
}

func (repo *MemoryRepository) List(_ context.Context, projectID string) ([]Document, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	result := make([]Document, 0)
	for _, doc := range repo.documents {
		if doc.ProjectID == projectID && doc.Status != StatusDeleted {
			result = append(result, doc)
		}
	}
	return result, nil
}

func (repo *MemoryRepository) Delete(_ context.Context, projectID string, id string) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	doc, ok := repo.documents[id]
	if !ok || doc.ProjectID != projectID {
		return errors.New("knowledge document not found")
	}
	doc.Status = StatusDeleted
	repo.documents[id] = doc
	return nil
}
