package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	BackendAddr                           string
	DataDir                               string
	StorageBackend                        string
	PostgresURL                           string
	DisableQdrant                         bool
	QdrantURL                             string
	QdrantGRPCURL                         string
	QdrantAPIKey                          string
	QdrantCollectionPrefix                string
	EmbeddingBaseURL                      string
	EmbeddingAPIKey                       string
	EmbeddingModel                        string
	SparseEmbeddingModel                  string
	LLMBaseURL                            string
	LLMAPIKey                             string
	LLMModel                              string
	RetrievalTopK                         int
	RetrievalPrefetchLimit                int
	MemoryMaxTaskRunsPerSite              int
	MemoryMaxPageObservationEventsPerSite int
	MemoryMaxContextEventsPerProject      int
	MemoryMaxFailureMemoriesPerSite       int
}

func Load() Config {
	return Config{
		BackendAddr:                           envString("WORKFLOW_BACKEND_ADDR", "127.0.0.1:38402"),
		DataDir:                               envString("WORKFLOW_DATA_DIR", defaultDataDir()),
		StorageBackend:                        envStorageBackend("WORKFLOW_STORAGE_BACKEND", "file"),
		PostgresURL:                           os.Getenv("WORKFLOW_POSTGRES_URL"),
		DisableQdrant:                         envBool("WORKFLOW_DISABLE_QDRANT", true),
		QdrantURL:                             normalizeURL(envString("QDRANT_URL", "http://127.0.0.1:6333")),
		QdrantGRPCURL:                         envString("QDRANT_GRPC_URL", "127.0.0.1:6334"),
		QdrantAPIKey:                          os.Getenv("QDRANT_API_KEY"),
		QdrantCollectionPrefix:                envString("QDRANT_COLLECTION_PREFIX", "pa"),
		EmbeddingBaseURL:                      normalizeURL(envString("EMBEDDING_BASE_URL", "")),
		EmbeddingAPIKey:                       os.Getenv("EMBEDDING_API_KEY"),
		EmbeddingModel:                        envString("EMBEDDING_MODEL", "bge-m3"),
		SparseEmbeddingModel:                  envString("SPARSE_EMBEDDING_MODEL", "bge-m3-sparse"),
		LLMBaseURL:                            normalizeURL(envString("LLM_BASE_URL", "")),
		LLMAPIKey:                             os.Getenv("LLM_API_KEY"),
		LLMModel:                              envString("LLM_MODEL", "gpt-5.4"),
		RetrievalTopK:                         envPositiveInt("RETRIEVAL_TOPK", 8),
		RetrievalPrefetchLimit:                envPositiveInt("RETRIEVAL_PREFETCH_LIMIT", 50),
		MemoryMaxTaskRunsPerSite:              envPositiveInt("WORKFLOW_MEMORY_MAX_TASK_RUNS_PER_SITE", 200),
		MemoryMaxPageObservationEventsPerSite: envPositiveInt("WORKFLOW_MEMORY_MAX_PAGE_OBSERVATIONS_PER_SITE", 200),
		MemoryMaxContextEventsPerProject:      envPositiveInt("WORKFLOW_MEMORY_MAX_CONTEXT_EVENTS_PER_PROJECT", 500),
		MemoryMaxFailureMemoriesPerSite:       envPositiveInt("WORKFLOW_MEMORY_MAX_FAILURES_PER_SITE", 100),
	}
}

func envStorageBackend(key string, fallback string) string {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch value {
	case "file", "postgres":
		return value
	case "":
		return fallback
	default:
		return fallback
	}
}

func envBool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch value {
	case "":
		return fallback
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
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
