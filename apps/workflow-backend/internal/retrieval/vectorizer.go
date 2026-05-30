package retrieval

import (
	"context"
	"errors"
	"hash/fnv"
	"math"
	"sort"
	"strings"

	"github.com/page-agent/workflow-backend/internal/embedding"
)

const deterministicDenseSize = 1024

type Vectorizer interface {
	DenseQuery(context.Context, string) ([]float32, error)
	SparseQuery(context.Context, string) (map[string]any, error)
}

type EmbeddingVectorizer struct {
	denseProvider  embedding.DenseProvider
	sparseProvider embedding.SparseProvider
	cache          *embedding.Cache
	denseModel     string
	sparseModel    string
}

func NewEmbeddingVectorizer(
	denseProvider embedding.DenseProvider,
	sparseProvider embedding.SparseProvider,
	cache *embedding.Cache,
	denseModel string,
	sparseModel string,
) *EmbeddingVectorizer {
	if cache == nil {
		cache = embedding.NewCache()
	}
	return &EmbeddingVectorizer{
		denseProvider:  denseProvider,
		sparseProvider: sparseProvider,
		cache:          cache,
		denseModel:     denseModel,
		sparseModel:    sparseModel,
	}
}

func (vectorizer *EmbeddingVectorizer) DenseQuery(ctx context.Context, text string) ([]float32, error) {
	if vectorizer.denseProvider == nil {
		return nil, errors.New("dense embedding provider is not configured")
	}
	key := embedding.CacheKey{
		Model:       vectorizer.denseModel,
		ContentHash: embedding.ContentHash(text),
		Kind:        embedding.VectorDense,
	}
	if value, ok := vectorizer.cache.Get(key); ok {
		return value.Dense, nil
	}
	vectors, err := vectorizer.denseProvider.EmbedDense(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 || len(vectors[0].Dense) == 0 {
		return nil, errors.New("dense embedding provider returned no vector")
	}
	vectorizer.cache.Set(key, vectors[0])
	return vectors[0].Dense, nil
}

func (vectorizer *EmbeddingVectorizer) SparseQuery(ctx context.Context, text string) (map[string]any, error) {
	if vectorizer.sparseProvider == nil {
		return nil, errors.New("sparse embedding provider is not configured")
	}
	key := embedding.CacheKey{
		Model:       vectorizer.sparseModel,
		ContentHash: embedding.ContentHash(text),
		Kind:        embedding.VectorSparse,
	}
	if value, ok := vectorizer.cache.Get(key); ok {
		return sparseVectorPayload(value.Sparse), nil
	}
	vectors, err := vectorizer.sparseProvider.EmbedSparse(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return nil, errors.New("sparse embedding provider returned no vector")
	}
	vectorizer.cache.Set(key, vectors[0])
	return sparseVectorPayload(vectors[0].Sparse), nil
}

type DeterministicVectorizer struct{}

func (DeterministicVectorizer) DenseQuery(_ context.Context, text string) ([]float32, error) {
	vector := make([]float32, deterministicDenseSize)
	for _, token := range tokenize(text) {
		index := int(hashToken(token) % uint32(deterministicDenseSize))
		vector[index] += 1
	}
	normalize(vector)
	return vector, nil
}

func (DeterministicVectorizer) SparseQuery(_ context.Context, text string) (map[string]any, error) {
	counts := map[uint32]float32{}
	for _, token := range tokenize(text) {
		counts[hashToken(token)] += 1
	}
	indices := make([]uint32, 0, len(counts))
	for index := range counts {
		indices = append(indices, index)
	}
	sort.Slice(indices, func(i, j int) bool { return indices[i] < indices[j] })
	values := make([]float32, 0, len(indices))
	for _, index := range indices {
		values = append(values, counts[index])
	}
	return map[string]any{"indices": indices, "values": values}, nil
}

func sparseVectorPayload(vector embedding.SparseVector) map[string]any {
	return map[string]any{
		"indices": vector.Indices,
		"values":  vector.Values,
	}
}

func tokenize(text string) []string {
	return strings.Fields(strings.ToLower(text))
}

func hashToken(token string) uint32 {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(token))
	return hash.Sum32()
}

func normalize(vector []float32) {
	var sum float64
	for _, value := range vector {
		sum += float64(value * value)
	}
	if sum == 0 {
		return
	}
	scale := float32(1 / math.Sqrt(sum))
	for index := range vector {
		vector[index] *= scale
	}
}
