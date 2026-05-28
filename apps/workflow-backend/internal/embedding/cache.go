package embedding

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

type VectorKind string

const (
	VectorDense  VectorKind = "dense"
	VectorSparse VectorKind = "sparse"
)

type CacheKey struct {
	Model       string
	ContentHash string
	Kind        VectorKind
}

type Cache struct {
	mu      sync.RWMutex
	vectors map[CacheKey]Vector
}

func NewCache() *Cache {
	return &Cache{vectors: map[CacheKey]Vector{}}
}

func (cache *Cache) Get(key CacheKey) (Vector, bool) {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	value, ok := cache.vectors[key]
	return value, ok
}

func (cache *Cache) Set(key CacheKey, value Vector) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.vectors[key] = value
}

func ContentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return "sha256:" + hex.EncodeToString(sum[:])
}
