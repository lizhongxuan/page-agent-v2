package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	BackendAddr            string
	DataDir                string
	QdrantURL              string
	QdrantGRPCURL          string
	QdrantAPIKey           string
	QdrantCollectionPrefix string
	EmbeddingBaseURL       string
	EmbeddingAPIKey        string
	EmbeddingModel         string
	SparseEmbeddingModel   string
	LLMBaseURL             string
	LLMAPIKey              string
	LLMModel               string
	RetrievalTopK          int
	RetrievalPrefetchLimit int
}

func Load() Config {
	return Config{
		BackendAddr:            envString("WORKFLOW_BACKEND_ADDR", "127.0.0.1:38402"),
		DataDir:                envString("WORKFLOW_DATA_DIR", defaultDataDir()),
		QdrantURL:              normalizeURL(envString("QDRANT_URL", "http://127.0.0.1:6333")),
		QdrantGRPCURL:          envString("QDRANT_GRPC_URL", "127.0.0.1:6334"),
		QdrantAPIKey:           os.Getenv("QDRANT_API_KEY"),
		QdrantCollectionPrefix: envString("QDRANT_COLLECTION_PREFIX", "pa"),
		EmbeddingBaseURL:       normalizeURL(envString("EMBEDDING_BASE_URL", "")),
		EmbeddingAPIKey:        os.Getenv("EMBEDDING_API_KEY"),
		EmbeddingModel:         envString("EMBEDDING_MODEL", "bge-m3"),
		SparseEmbeddingModel:   envString("SPARSE_EMBEDDING_MODEL", "bge-m3-sparse"),
		LLMBaseURL:             normalizeURL(envString("LLM_BASE_URL", "")),
		LLMAPIKey:              os.Getenv("LLM_API_KEY"),
		LLMModel:               envString("LLM_MODEL", "gpt-5.4"),
		RetrievalTopK:          envPositiveInt("RETRIEVAL_TOPK", 8),
		RetrievalPrefetchLimit: envPositiveInt("RETRIEVAL_PREFETCH_LIMIT", 50),
	}
}

func envString(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envPositiveInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func normalizeURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), "page-agent-workflow-backend")
	}
	return filepath.Join(home, ".page-agent", "workflow-backend")
}
