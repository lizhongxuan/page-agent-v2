package httpapi

import (
	"net/http"
	"testing"

	"github.com/page-agent/workflow-backend/internal/config"
)

func TestKnowledgeRoutesReturnGoneWithMigrationHints(t *testing.T) {
	router := NewRouterWithServices(config.Config{}, Services{})
	tests := []struct {
		name        string
		path        string
		migration   string
		requestBody any
	}{
		{name: "root document", path: "/documents", migration: "/api/memory/documents", requestBody: map[string]any{"content": "doc"}},
		{name: "root ingest", path: "/ingest", migration: "/api/memory/documents", requestBody: map[string]any{"documents": []map[string]any{{"content": "doc"}}}},
		{name: "root search", path: "/search", migration: "/api/memory/context", requestBody: map[string]any{"task": "search"}},
		{name: "knowledge document", path: "/api/knowledge/documents", migration: "/api/memory/documents", requestBody: map[string]any{"content": "doc"}},
		{name: "knowledge ingest", path: "/api/knowledge/ingest", migration: "/api/memory/documents", requestBody: map[string]any{"documents": []map[string]any{{"content": "doc"}}}},
		{name: "knowledge search", path: "/api/knowledge/search", migration: "/api/memory/context", requestBody: map[string]any{"task": "search"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := performJSON(router, http.MethodPost, tt.path, tt.requestBody)

			assertGoneWithMigration(t, response, tt.migration)
		})
	}
}
