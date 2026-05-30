package config

import (
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("WORKFLOW_BACKEND_ADDR", "")
	t.Setenv("WORKFLOW_DATA_DIR", "")
	t.Setenv("QDRANT_URL", "")
	t.Setenv("QDRANT_COLLECTION_PREFIX", "")
	t.Setenv("WORKFLOW_STORAGE_BACKEND", "")
	t.Setenv("WORKFLOW_POSTGRES_URL", "")
	t.Setenv("WORKFLOW_DISABLE_QDRANT", "")
	t.Setenv("EMBEDDING_MODEL", "")
	t.Setenv("LLM_MODEL", "")
	t.Setenv("RETRIEVAL_TOPK", "")
	t.Setenv("RETRIEVAL_PREFETCH_LIMIT", "")

	cfg := Load()

	if cfg.BackendAddr != "127.0.0.1:38402" {
		t.Fatalf("expected default backend addr, got %q", cfg.BackendAddr)
	}
	if cfg.QdrantURL != "http://127.0.0.1:6333" {
		t.Fatalf("expected default qdrant url, got %q", cfg.QdrantURL)
	}
	if cfg.QdrantCollectionPrefix != "pa" {
		t.Fatalf("expected default qdrant prefix, got %q", cfg.QdrantCollectionPrefix)
	}
	if cfg.StorageBackend != "file" {
		t.Fatalf("expected default file storage backend, got %q", cfg.StorageBackend)
	}
	if cfg.PostgresURL != "" {
		t.Fatalf("expected empty default postgres URL, got %q", cfg.PostgresURL)
	}
	if !cfg.DisableQdrant {
		t.Fatal("expected qdrant to be disabled by default")
	}
	if cfg.RetrievalTopK != 8 {
		t.Fatalf("expected default topK 8, got %d", cfg.RetrievalTopK)
	}
	if cfg.RetrievalPrefetchLimit != 50 {
		t.Fatalf("expected default prefetch 50, got %d", cfg.RetrievalPrefetchLimit)
	}
	if cfg.DataDir == "" {
		t.Fatal("expected default data dir")
	}
}

func TestLoadEnvironmentOverrides(t *testing.T) {
	t.Setenv("WORKFLOW_BACKEND_ADDR", "127.0.0.1:39000")
	t.Setenv("WORKFLOW_DATA_DIR", "/tmp/page-agent-workflows")
	t.Setenv("QDRANT_URL", "http://qdrant.example.test:6333/")
	t.Setenv("QDRANT_API_KEY", "secret")
	t.Setenv("QDRANT_COLLECTION_PREFIX", "custom")
	t.Setenv("WORKFLOW_STORAGE_BACKEND", "postgres")
	t.Setenv("WORKFLOW_POSTGRES_URL", "postgres://page_agent:secret@127.0.0.1:5432/page_agent")
	t.Setenv("WORKFLOW_DISABLE_QDRANT", "true")
	t.Setenv("EMBEDDING_BASE_URL", "https://llm.example.test/v1")
	t.Setenv("EMBEDDING_API_KEY", "embed-secret")
	t.Setenv("EMBEDDING_MODEL", "bge-m3")
	t.Setenv("SPARSE_EMBEDDING_MODEL", "bge-m3-sparse")
	t.Setenv("LLM_BASE_URL", "https://llm.example.test/v1")
	t.Setenv("LLM_API_KEY", "llm-secret")
	t.Setenv("LLM_MODEL", "gpt-5.4")
	t.Setenv("RETRIEVAL_TOPK", "12")
	t.Setenv("RETRIEVAL_PREFETCH_LIMIT", "80")

	cfg := Load()

	if cfg.BackendAddr != "127.0.0.1:39000" {
		t.Fatalf("backend addr override not applied: %#v", cfg)
	}
	if cfg.DataDir != "/tmp/page-agent-workflows" {
		t.Fatalf("data dir override not applied: %#v", cfg)
	}
	if cfg.QdrantURL != "http://qdrant.example.test:6333" {
		t.Fatalf("qdrant url should be normalized, got %q", cfg.QdrantURL)
	}
	if cfg.QdrantAPIKey != "secret" {
		t.Fatal("qdrant api key override not applied")
	}
	if cfg.StorageBackend != "postgres" {
		t.Fatalf("storage backend override not applied: %#v", cfg)
	}
	if cfg.PostgresURL != "postgres://page_agent:secret@127.0.0.1:5432/page_agent" {
		t.Fatalf("postgres URL override not applied: %#v", cfg)
	}
	if !cfg.DisableQdrant {
		t.Fatal("disable qdrant override not applied")
	}
	if cfg.EmbeddingModel != "bge-m3" || cfg.SparseEmbeddingModel != "bge-m3-sparse" {
		t.Fatalf("embedding overrides not applied: %#v", cfg)
	}
	if cfg.LLMModel != "gpt-5.4" {
		t.Fatalf("llm override not applied: %#v", cfg)
	}
	if cfg.RetrievalTopK != 12 || cfg.RetrievalPrefetchLimit != 80 {
		t.Fatalf("retrieval overrides not applied: %#v", cfg)
	}
}

func TestLoadNormalizesInvalidStorageBackend(t *testing.T) {
	t.Setenv("WORKFLOW_STORAGE_BACKEND", "sqlite")
	t.Setenv("WORKFLOW_DISABLE_QDRANT", "YES")

	cfg := Load()

	if cfg.StorageBackend != "file" {
		t.Fatalf("expected invalid storage backend to fall back to file, got %q", cfg.StorageBackend)
	}
	if !cfg.DisableQdrant {
		t.Fatal("expected yes to parse as disabling qdrant")
	}
}

func TestLoadIgnoresInvalidIntegerOverrides(t *testing.T) {
	t.Setenv("RETRIEVAL_TOPK", "not-a-number")
	t.Setenv("RETRIEVAL_PREFETCH_LIMIT", "-1")

	cfg := Load()

	if cfg.RetrievalTopK != 8 {
		t.Fatalf("expected invalid topK to fall back to 8, got %d", cfg.RetrievalTopK)
	}
	if cfg.RetrievalPrefetchLimit != 50 {
		t.Fatalf("expected invalid prefetch to fall back to 50, got %d", cfg.RetrievalPrefetchLimit)
	}
}
