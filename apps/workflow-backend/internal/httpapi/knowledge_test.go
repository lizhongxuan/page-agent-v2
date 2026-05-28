package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/page-agent/workflow-backend/internal/knowledge"
)

func TestKnowledgeSearchEndpointMatchesExtensionContract(t *testing.T) {
	service := knowledge.NewService(knowledge.NewMemoryRepository(), knowledge.DeterministicEmbedder{})
	_, err := service.Create(httptest.NewRequest(http.MethodGet, "/", nil).Context(), knowledge.Document{
		ProjectID:  "default",
		Type:       "guide",
		Title:      "Search Guide",
		Source:     "manual",
		Content:    "Search services by service name.",
		Confidence: 1,
		Status:     knowledge.StatusActive,
	})
	if err != nil {
		t.Fatalf("Create fixture: %v", err)
	}
	router := NewRouter(Dependencies{Knowledge: service})

	body := bytes.NewBufferString(`{"task":"search service","url":"https://console.example.test","title":"Console","projectKey":"default","limit":3}`)
	request := httptest.NewRequest(http.MethodPost, "/search", body)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d %s", response.Code, response.Body.String())
	}
	var payload struct {
		Hits []knowledge.Hit `json:"hits"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Hits) != 1 || payload.Hits[0].Title != "Search Guide" {
		t.Fatalf("unexpected hits: %#v", payload.Hits)
	}
}
