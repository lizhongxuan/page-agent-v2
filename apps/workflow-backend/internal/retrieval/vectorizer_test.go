package retrieval

import (
	"context"
	"testing"

	"github.com/page-agent/workflow-backend/internal/embedding"
)

func TestDeterministicVectorizerReturnsStableDenseAndSparseVectors(t *testing.T) {
	vectorizer := DeterministicVectorizer{}

	firstDense, err := vectorizer.DenseQuery(context.Background(), "search github issues")
	if err != nil {
		t.Fatalf("DenseQuery failed: %v", err)
	}
	secondDense, err := vectorizer.DenseQuery(context.Background(), "search github issues")
	if err != nil {
		t.Fatalf("DenseQuery failed: %v", err)
	}
	sparse, err := vectorizer.SparseQuery(context.Background(), "search github issues")
	if err != nil {
		t.Fatalf("SparseQuery failed: %v", err)
	}

	if len(firstDense) != deterministicDenseSize {
		t.Fatalf("expected dense vector size %d, got %d", deterministicDenseSize, len(firstDense))
	}
	if firstDense[0] != secondDense[0] {
		t.Fatalf("expected stable dense vector")
	}
	if len(sparse["indices"].([]uint32)) == 0 || len(sparse["values"].([]float32)) == 0 {
		t.Fatalf("expected sparse values, got %#v", sparse)
	}
}

func TestEmbeddingVectorizerCachesProviderResults(t *testing.T) {
	provider := &fakeEmbeddingProvider{}
	vectorizer := NewEmbeddingVectorizer(provider, provider, embedding.NewCache(), "dense-model", "sparse-model")

	_, err := vectorizer.DenseQuery(context.Background(), "hello")
	if err != nil {
		t.Fatalf("DenseQuery failed: %v", err)
	}
	_, err = vectorizer.DenseQuery(context.Background(), "hello")
	if err != nil {
		t.Fatalf("DenseQuery failed: %v", err)
	}
	_, err = vectorizer.SparseQuery(context.Background(), "hello")
	if err != nil {
		t.Fatalf("SparseQuery failed: %v", err)
	}
	_, err = vectorizer.SparseQuery(context.Background(), "hello")
	if err != nil {
		t.Fatalf("SparseQuery failed: %v", err)
	}

	if provider.denseCalls != 1 || provider.sparseCalls != 1 {
		t.Fatalf("expected cached calls, got dense=%d sparse=%d", provider.denseCalls, provider.sparseCalls)
	}
}

type fakeEmbeddingProvider struct {
	denseCalls  int
	sparseCalls int
}

func (provider *fakeEmbeddingProvider) EmbedDense(context.Context, []string) ([]embedding.Vector, error) {
	provider.denseCalls++
	return []embedding.Vector{{Dense: []float32{0.1, 0.2}}}, nil
}

func (provider *fakeEmbeddingProvider) EmbedSparse(context.Context, []string) ([]embedding.Vector, error) {
	provider.sparseCalls++
	return []embedding.Vector{{Sparse: embedding.SparseVector{
		Indices: []uint32{1, 2},
		Values:  []float32{0.5, 0.25},
	}}}, nil
}
