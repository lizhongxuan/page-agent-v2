package knowledge

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"strings"

	"github.com/page-agent/workflow-backend/internal/registry"
)

type DenseVectorizer interface {
	DenseQuery(context.Context, string) ([]float32, error)
}

type Service struct {
	repo       registry.Repository
	vectorizer DenseVectorizer
}

func NewService(repo registry.Repository) *Service {
	return NewServiceWithVectorizer(repo, nil)
}

func NewServiceWithVectorizer(repo registry.Repository, vectorizer DenseVectorizer) *Service {
	return &Service{repo: repo, vectorizer: vectorizer}
}

func (service *Service) Ingest(ctx context.Context, documents []DocumentInput) ([]string, error) {
	ids := make([]string, 0, len(documents))
	for _, input := range documents {
		if strings.TrimSpace(input.ID) == "" {
			input.ID = stableDocumentID(input)
		}
		document := registry.KnowledgeDocument{
			ID:        input.ID,
			ProjectID: input.ProjectID,
			Title:     input.Title,
			Source:    input.Source,
			URL:       input.URL,
			Content:   input.Content,
			Tags:      input.Tags,
		}
		chunks, err := ChunkDocument(input)
		if err != nil {
			return nil, err
		}
		if service.vectorizer != nil {
			for index := range chunks {
				embedding, err := service.vectorizer.DenseQuery(ctx, knowledgeChunkEmbeddingText(chunks[index]))
				if err != nil {
					return nil, err
				}
				chunks[index].Embedding = embedding
			}
		}
		if err := service.repo.SaveKnowledgeDocument(ctx, document); err != nil {
			return nil, err
		}
		if err := service.repo.SaveKnowledgeChunks(ctx, chunks); err != nil {
			return nil, err
		}
		ids = append(ids, document.ID)
	}
	return ids, nil
}

func (service *Service) Search(ctx context.Context, query registry.KnowledgeSearchQuery) ([]registry.KnowledgeChunk, error) {
	if service.vectorizer != nil && len(query.Embedding) == 0 {
		text := knowledgeSearchEmbeddingText(query)
		if text != "" {
			embedding, err := service.vectorizer.DenseQuery(ctx, text)
			if err != nil {
				return nil, err
			}
			query.Embedding = embedding
		}
	}
	return service.repo.SearchKnowledgeChunks(ctx, query)
}

func knowledgeChunkEmbeddingText(chunk registry.KnowledgeChunk) string {
	parts := []string{
		chunk.Title,
		chunk.Source,
		chunk.ChunkText,
		strings.Join(chunk.Tags, " "),
	}
	if url, ok := chunk.Metadata["url"].(string); ok {
		parts = append(parts, url)
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func knowledgeSearchEmbeddingText(query registry.KnowledgeSearchQuery) string {
	return strings.TrimSpace(strings.Join([]string{
		query.Task,
		query.Title,
		query.URL,
		query.VisibleText,
		strings.Join(query.Hints, " "),
	}, "\n"))
}

func stableDocumentID(input DocumentInput) string {
	scope := strings.TrimSpace(input.URL)
	if scope == "" {
		scope = strings.TrimSpace(input.Title)
	}
	hash := sha1.Sum([]byte(strings.Join([]string{
		input.ProjectID,
		input.Source,
		scope,
	}, "\x00")))
	return "doc_" + hex.EncodeToString(hash[:8])
}
