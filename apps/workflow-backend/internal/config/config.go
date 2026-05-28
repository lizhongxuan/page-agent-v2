package config

import "os"

const (
	defaultAddr    = "127.0.0.1:38402"
	defaultDataDir = "./data"
)

type Config struct {
	Addr             string
	DatabaseURL      string
	DataDir          string
	APIKey           string
	EmbeddingBaseURL string
	EmbeddingAPIKey  string
	EmbeddingModel   string
}

func Load() Config {
	return LoadFromLookup(os.LookupEnv)
}

func LoadFromLookup(lookup func(string) (string, bool)) Config {
	return Config{
		Addr:             getEnv(lookup, "PAGE_AGENT_BACKEND_ADDR", defaultAddr),
		DatabaseURL:      getEnv(lookup, "PAGE_AGENT_DATABASE_URL", ""),
		DataDir:          getEnv(lookup, "PAGE_AGENT_DATA_DIR", defaultDataDir),
		APIKey:           getEnv(lookup, "PAGE_AGENT_API_KEY", ""),
		EmbeddingBaseURL: getEnv(lookup, "PAGE_AGENT_EMBEDDING_BASE_URL", ""),
		EmbeddingAPIKey:  getEnv(lookup, "PAGE_AGENT_EMBEDDING_API_KEY", ""),
		EmbeddingModel:   getEnv(lookup, "PAGE_AGENT_EMBEDDING_MODEL", ""),
	}
}

func getEnv(lookup func(string) (string, bool), key string, fallback string) string {
	value, ok := lookup(key)
	if !ok || value == "" {
		return fallback
	}
	return value
}
