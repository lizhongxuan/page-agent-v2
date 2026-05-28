package knowledge

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"
)

type Embedder interface {
	Embed(context.Context, []string) ([][]float32, error)
}

type DeterministicEmbedder struct{}

func (DeterministicEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, 0, len(texts))
	for _, text := range texts {
		var sum float32
		for _, r := range text {
			sum += float32(r % 31)
		}
		result = append(result, []float32{sum, float32(len(text))})
	}
	return result, nil
}

type Service struct {
	repo     Repository
	embedder Embedder
}

func NewService(repo Repository, embedder Embedder) *Service {
	return &Service{repo: repo, embedder: embedder}
}

func (service *Service) Create(ctx context.Context, doc Document) (Document, error) {
	if err := validateDocument(doc); err != nil {
		return Document{}, err
	}
	now := time.Now().UTC()
	if doc.ID == "" {
		doc.ID = newID("kd")
	}
	if doc.Status == "" {
		doc.Status = StatusActive
	}
	if doc.Confidence == 0 {
		doc.Confidence = 0.5
	}
	doc.CreatedAt = now
	doc.UpdatedAt = now
	_, _ = service.embedder.Embed(ctx, []string{doc.Content})
	return service.repo.Save(ctx, doc)
}

func (service *Service) Get(ctx context.Context, projectID string, id string) (Document, error) {
	return service.repo.Get(ctx, projectID, id)
}

func (service *Service) Update(ctx context.Context, doc Document) (Document, error) {
	if err := validateDocument(doc); err != nil {
		return Document{}, err
	}
	doc.UpdatedAt = time.Now().UTC()
	_, _ = service.embedder.Embed(ctx, []string{doc.Content})
	return service.repo.Save(ctx, doc)
}

func (service *Service) Delete(ctx context.Context, projectID string, id string) error {
	return service.repo.Delete(ctx, projectID, id)
}

func (service *Service) Ingest(ctx context.Context, docs []Document) (IngestResult, error) {
	result := IngestResult{}
	for _, doc := range docs {
		if _, err := service.Create(ctx, doc); err != nil {
			result.Rejected++
			continue
		}
		result.Accepted++
	}
	return result, nil
}

func (service *Service) Search(ctx context.Context, request SearchRequest) ([]Hit, error) {
	projectID := request.ProjectID
	if projectID == "" {
		projectID = request.ProjectKey
	}
	if projectID == "" {
		projectID = "default"
	}
	limit := request.Limit
	if limit <= 0 {
		limit = 5
	}
	docs, err := service.repo.List(ctx, projectID)
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(strings.Join([]string{
		request.Task,
		request.Title,
		request.VisibleText,
		strings.Join(request.Hints, " "),
	}, " "))
	hits := make([]Hit, 0)
	for _, doc := range docs {
		score := scoreDocument(doc, needle, request.URL)
		if score <= 0 {
			continue
		}
		hits = append(hits, Hit{
			ID:      doc.ID,
			Title:   doc.Title,
			Source:  doc.Source,
			Snippet: snippet(doc.Content),
			URL:     doc.URL,
			Score:   score,
			Tags:    append([]string(nil), doc.Tags...),
		})
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

func validateDocument(doc Document) error {
	if doc.ProjectID == "" {
		return errors.New("project id is required")
	}
	if doc.Type == "" {
		return errors.New("type is required")
	}
	if doc.Title == "" {
		return errors.New("title is required")
	}
	if doc.Source == "" {
		return errors.New("source is required")
	}
	if doc.Content == "" {
		return errors.New("content is required")
	}
	return nil
}

func scoreDocument(doc Document, needle string, requestURL string) float64 {
	haystack := strings.ToLower(strings.Join([]string{
		doc.Title,
		doc.Content,
		doc.Source,
		strings.Join(doc.Tags, " "),
	}, " "))
	score := 0.0
	for _, token := range strings.Fields(needle) {
		if strings.Contains(haystack, token) {
			score += 0.2
		}
	}
	if doc.URL != "" && requestURL != "" && strings.HasPrefix(requestURL, doc.URL) {
		score += 0.2
	}
	score += doc.Confidence * 0.2
	if score > 1 {
		return 1
	}
	return score
}

func snippet(content string) string {
	content = strings.TrimSpace(content)
	if len(content) <= 160 {
		return content
	}
	return content[:160]
}

func newID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return prefix + "_" + strings.ReplaceAll(time.Now().UTC().Format(time.RFC3339Nano), ":", "")
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}
