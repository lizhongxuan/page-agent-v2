package config

import "testing"

func TestLoadFromLookupUsesSafeDefaults(t *testing.T) {
	cfg := LoadFromLookup(func(string) (string, bool) {
		return "", false
	})

	if cfg.Addr != "127.0.0.1:38402" {
		t.Fatalf("expected default addr, got %q", cfg.Addr)
	}
	if cfg.DataDir != "./data" {
		t.Fatalf("expected default data dir, got %q", cfg.DataDir)
	}
	if cfg.DatabaseURL != "" {
		t.Fatalf("expected empty database url by default, got %q", cfg.DatabaseURL)
	}
}

func TestLoadFromLookupReadsPageAgentEnvironment(t *testing.T) {
	values := map[string]string{
		"PAGE_AGENT_BACKEND_ADDR":       "127.0.0.1:9999",
		"PAGE_AGENT_DATABASE_URL":       "postgres://page-agent.test/db",
		"PAGE_AGENT_DATA_DIR":           "/tmp/page-agent",
		"PAGE_AGENT_API_KEY":            "secret",
		"PAGE_AGENT_EMBEDDING_BASE_URL": "https://llm.example.test/v1",
		"PAGE_AGENT_EMBEDDING_API_KEY":  "embed-key",
		"PAGE_AGENT_EMBEDDING_MODEL":    "embedding-model",
	}

	cfg := LoadFromLookup(func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	})

	if cfg.Addr != values["PAGE_AGENT_BACKEND_ADDR"] {
		t.Fatalf("addr mismatch: %#v", cfg)
	}
	if cfg.DatabaseURL != values["PAGE_AGENT_DATABASE_URL"] {
		t.Fatalf("database url mismatch: %#v", cfg)
	}
	if cfg.DataDir != values["PAGE_AGENT_DATA_DIR"] {
		t.Fatalf("data dir mismatch: %#v", cfg)
	}
	if cfg.APIKey != values["PAGE_AGENT_API_KEY"] {
		t.Fatalf("api key mismatch: %#v", cfg)
	}
	if cfg.EmbeddingBaseURL != values["PAGE_AGENT_EMBEDDING_BASE_URL"] {
		t.Fatalf("embedding base url mismatch: %#v", cfg)
	}
	if cfg.EmbeddingAPIKey != values["PAGE_AGENT_EMBEDDING_API_KEY"] {
		t.Fatalf("embedding api key mismatch: %#v", cfg)
	}
	if cfg.EmbeddingModel != values["PAGE_AGENT_EMBEDDING_MODEL"] {
		t.Fatalf("embedding model mismatch: %#v", cfg)
	}
}
