package qdrant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCollectionManagerCreatesCollectionsAndPayloadIndexes(t *testing.T) {
	var creates []map[string]any
	indexes := map[string][]map[string]any{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path != "/healthz":
			http.NotFound(w, r)
		case r.Method == http.MethodPut && r.URL.Path == "/collections/custom_workflow_cards":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			creates = append(creates, body)
			_, _ = w.Write([]byte(`{"result":true}`))
		case r.Method == http.MethodPut && r.URL.Path == "/collections/custom_workflow_cards/index":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			indexes["custom_workflow_cards"] = append(indexes["custom_workflow_cards"], body)
			_, _ = w.Write([]byte(`{"result":true}`))
		case r.Method == http.MethodPut && r.URL.Path == "/collections/custom_workflow_chunks/index":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			indexes["custom_workflow_chunks"] = append(indexes["custom_workflow_chunks"], body)
			_, _ = w.Write([]byte(`{"result":true}`))
		case r.Method == http.MethodPut && r.URL.Path == "/collections/custom_page_states/index":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			indexes["custom_page_states"] = append(indexes["custom_page_states"], body)
			_, _ = w.Write([]byte(`{"result":true}`))
		case r.Method == http.MethodPut && r.URL.Path == "/collections/custom_interrupt_handlers/index":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			indexes["custom_interrupt_handlers"] = append(indexes["custom_interrupt_handlers"], body)
			_, _ = w.Write([]byte(`{"result":true}`))
		case r.Method == http.MethodPut && r.URL.Path == "/collections/custom_repair_patches/index":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			indexes["custom_repair_patches"] = append(indexes["custom_repair_patches"], body)
			_, _ = w.Write([]byte(`{"result":true}`))
		case r.Method == http.MethodPut:
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			creates = append(creates, body)
			_, _ = w.Write([]byte(`{"result":true}`))
		default:
			_, _ = w.Write([]byte(`{"result":true}`))
		}
	}))
	defer server.Close()

	manager := NewCollectionManager(NewClient(ClientConfig{BaseURL: server.URL}), "custom")
	if err := manager.EnsureCollections(context.Background()); err != nil {
		t.Fatalf("ensure collections failed: %v", err)
	}

	names := manager.CollectionNames()
	wantNames := CollectionNames{
		WorkflowCards:     "custom_workflow_cards",
		WorkflowChunks:    "custom_workflow_chunks",
		PageStates:        "custom_page_states",
		InterruptHandlers: "custom_interrupt_handlers",
		RepairPatches:     "custom_repair_patches",
	}
	if names != wantNames {
		t.Fatalf("collection names mismatch: got %#v want %#v", names, wantNames)
	}

	if len(creates) != 5 {
		t.Fatalf("expected five collection create requests, got %d", len(creates))
	}
	vectors := creates[0]["vectors"].(map[string]any)
	if _, ok := vectors["intent_dense"]; !ok {
		t.Fatalf("expected intent_dense vector config: %#v", creates[0])
	}
	if _, ok := vectors["page_dense"]; !ok {
		t.Fatalf("expected page_dense vector config: %#v", creates[0])
	}
	sparse := creates[0]["sparse_vectors"].(map[string]any)
	if _, ok := sparse["lexical_sparse"]; !ok {
		t.Fatalf("expected lexical_sparse vector config: %#v", creates[0])
	}

	assertHasIndex(t, indexes["custom_workflow_cards"], "project_id", "keyword")
	assertHasIndex(t, indexes["custom_workflow_cards"], "tenant_id", "keyword")
	assertHasIndex(t, indexes["custom_workflow_cards"], "doc_type", "keyword")
	assertHasIndex(t, indexes["custom_workflow_cards"], "status", "keyword")
	assertHasIndex(t, indexes["custom_workflow_cards"], "site", "keyword")
	assertHasIndex(t, indexes["custom_workflow_cards"], "app", "keyword")
	assertHasIndex(t, indexes["custom_workflow_cards"], "risk_level", "keyword")
	assertHasIndex(t, indexes["custom_workflow_cards"], "updated_at", "datetime")
	assertHasIndex(t, indexes["custom_workflow_cards"], "success_rate", "float")
	assertHasIndex(t, indexes["custom_workflow_cards"], "workflow_id", "keyword")
	assertHasIndex(t, indexes["custom_workflow_cards"], "version", "integer")
	assertHasIndex(t, indexes["custom_workflow_cards"], "variable_names", "keyword")
	assertHasIndex(t, indexes["custom_workflow_chunks"], "from_page_state", "keyword")
	assertHasIndex(t, indexes["custom_workflow_chunks"], "target_names", "text")
	assertHasIndex(t, indexes["custom_page_states"], "page_state_id", "keyword")
	assertHasIndex(t, indexes["custom_interrupt_handlers"], "applies_to_page_states", "keyword")
	assertHasIndex(t, indexes["custom_repair_patches"], "failure_signature", "text")
}

func TestValidatePayloadIndexesRejectsMissingHighFrequencyField(t *testing.T) {
	defs := DefaultCollectionDefinitions("pa")
	defs[0].PayloadIndexes = defs[0].PayloadIndexes[1:]

	err := ValidateCollectionDefinitions(defs)

	if err == nil {
		t.Fatal("expected missing high-frequency index to fail")
	}
	if err.Error() != "collection pa_workflow_cards missing required payload index project_id" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertHasIndex(t *testing.T, indexes []map[string]any, field string, schema string) {
	t.Helper()
	for _, index := range indexes {
		if index["field_name"] == field && index["field_schema"] == schema {
			return
		}
	}
	t.Fatalf("missing index %s/%s in %#v", field, schema, indexes)
}
