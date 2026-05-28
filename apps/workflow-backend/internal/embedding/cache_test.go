package embedding

import "testing"

func TestCacheHitAndMiss(t *testing.T) {
	cache := NewCache()
	key := CacheKey{Model: "bge-m3", ContentHash: "hash-a", Kind: VectorDense}

	if _, ok := cache.Get(key); ok {
		t.Fatal("expected initial cache miss")
	}
	cache.Set(key, Vector{Dense: []float32{1, 2, 3}})

	got, ok := cache.Get(key)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if len(got.Dense) != 3 || got.Dense[0] != 1 {
		t.Fatalf("unexpected cached vector: %#v", got)
	}
}

func TestContentHashIsStable(t *testing.T) {
	first := ContentHash("Search GitHub issues")
	second := ContentHash("Search GitHub issues")
	third := ContentHash("Search GitHub pull requests")

	if first == "" || first != second {
		t.Fatalf("expected stable non-empty hash, got %q and %q", first, second)
	}
	if first == third {
		t.Fatalf("different content should produce different hash")
	}
}
