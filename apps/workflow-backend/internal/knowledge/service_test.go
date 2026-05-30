package knowledge

import (
	"context"
	"strings"
	"testing"

	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestServiceIngestAndSearch(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewService(repo)
	ctx := context.Background()

	ids, err := service.Ingest(ctx, []DocumentInput{
		{
			ID:        "doc_service",
			ProjectID: "default",
			Title:     "服务管理手册",
			Source:    "manual",
			Content:   "服务管理页可通过服务名称搜索框定位服务，状态列表示当前运行状态。",
			Tags:      []string{"service", "status"},
		},
	})
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}
	if len(ids) != 1 || ids[0] != "doc_service" {
		t.Fatalf("unexpected ids: %#v", ids)
	}

	hits, err := service.Search(ctx, registry.KnowledgeSearchQuery{
		ProjectID: "default",
		Task:      "查看服务状态",
		Title:     "服务管理",
		Limit:     3,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(hits) != 1 || hits[0].Title != "服务管理手册" {
		t.Fatalf("unexpected hits: %#v", hits)
	}
}

func TestServiceIngestGeneratesStableDocumentID(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewService(repo)
	ctx := context.Background()
	input := DocumentInput{
		ProjectID: "default",
		Title:     "服务管理手册",
		Source:    "manual",
		Content:   "服务管理页可通过服务名称搜索框定位服务。",
	}

	first, err := service.Ingest(ctx, []DocumentInput{input})
	if err != nil {
		t.Fatalf("first Ingest failed: %v", err)
	}
	second, err := service.Ingest(ctx, []DocumentInput{input})
	if err != nil {
		t.Fatalf("second Ingest failed: %v", err)
	}
	if len(first) != 1 || first[0] == "" || first[0] != second[0] {
		t.Fatalf("expected stable generated id, got first=%#v second=%#v", first, second)
	}
}

func TestServiceIngestUpdatesDocumentBySourceAndReplacesOldChunks(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewService(repo)
	ctx := context.Background()
	input := DocumentInput{
		ProjectID: "default",
		Title:     "服务管理手册",
		Source:    "manual",
		URL:       "https://ops.example.com/docs/services",
		Content:   strings.Repeat("legacy-obsolete-marker ", 90),
	}

	first, err := service.Ingest(ctx, []DocumentInput{input})
	if err != nil {
		t.Fatalf("first Ingest failed: %v", err)
	}
	input.Content = "fresh-updated-marker"
	second, err := service.Ingest(ctx, []DocumentInput{input})
	if err != nil {
		t.Fatalf("second Ingest failed: %v", err)
	}
	if len(first) != 1 || len(second) != 1 || first[0] != second[0] {
		t.Fatalf("expected updated document to keep stable id, first=%#v second=%#v", first, second)
	}

	oldHits, err := service.Search(ctx, registry.KnowledgeSearchQuery{
		ProjectID: "default",
		Task:      "legacy-obsolete-marker",
		URL:       "https://ops.example.com/docs/services",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("old Search failed: %v", err)
	}
	if len(oldHits) != 0 {
		t.Fatalf("expected old chunks to be replaced, got %#v", oldHits)
	}
	newHits, err := service.Search(ctx, registry.KnowledgeSearchQuery{
		ProjectID: "default",
		Task:      "fresh-updated-marker",
		URL:       "https://ops.example.com/docs/services",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("new Search failed: %v", err)
	}
	if len(newHits) != 1 || newHits[0].DocumentID != first[0] {
		t.Fatalf("expected one updated chunk, got %#v", newHits)
	}
}

func TestServiceIngestAndSearchUsesVectorizer(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewServiceWithVectorizer(repo, fakeKnowledgeVectorizer{})
	ctx := context.Background()

	if _, err := service.Ingest(ctx, []DocumentInput{
		{
			ID:        "doc_service",
			ProjectID: "default",
			Title:     "服务管理手册",
			Source:    "manual",
			URL:       "https://ops.example.com/services",
			Content:   "服务管理页用于查看服务状态。",
		},
		{
			ID:        "doc_billing",
			ProjectID: "default",
			Title:     "账单管理手册",
			Source:    "manual",
			Content:   "账单页用于查看发票。",
		},
	}); err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	hits, err := service.Search(ctx, registry.KnowledgeSearchQuery{
		ProjectID: "default",
		Task:      "查看服务状态",
		URL:       "https://ops.example.com/services/detail",
		Limit:     1,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(hits) != 1 || hits[0].DocumentID != "doc_service" {
		t.Fatalf("expected service chunk from vector search, got %#v", hits)
	}
	if hits[0].Score <= 0 {
		t.Fatalf("expected vector score, got %#v", hits[0])
	}
}

type fakeKnowledgeVectorizer struct{}

func (fakeKnowledgeVectorizer) DenseQuery(_ context.Context, text string) ([]float32, error) {
	if strings.Contains(text, "服务") || strings.Contains(text, "状态") {
		return []float32{1, 0}, nil
	}
	return []float32{0, 1}, nil
}
