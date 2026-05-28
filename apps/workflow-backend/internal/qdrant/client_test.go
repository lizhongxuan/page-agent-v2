package qdrant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientUsesQdrantRESTEndpoints(t *testing.T) {
	var requests []recordedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body any
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		requests = append(requests, recordedRequest{
			Method: r.Method,
			Path:   r.URL.Path,
			Query:  r.URL.RawQuery,
			APIKey: r.Header.Get("api-key"),
			Body:   body,
		})

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/collections/cards":
			_, _ = w.Write([]byte(`{"result":{"status":"green"}}`))
		default:
			_, _ = w.Write([]byte(`{"result":true}`))
		}
	}))
	defer server.Close()

	client := NewClient(ClientConfig{BaseURL: server.URL, APIKey: "secret-key"})
	ctx := context.Background()

	if err := client.Health(ctx); err != nil {
		t.Fatalf("health failed: %v", err)
	}
	exists, err := client.CollectionExists(ctx, "cards")
	if err != nil {
		t.Fatalf("collection exists failed: %v", err)
	}
	if !exists {
		t.Fatal("expected collection to exist")
	}
	if err := client.CreatePayloadIndex(ctx, "cards", PayloadIndex{FieldName: "project_id", FieldSchema: "keyword"}); err != nil {
		t.Fatalf("create payload index failed: %v", err)
	}
	if err := client.UpsertPoints(ctx, "cards", []Point{{ID: "workflow:wf:v1", Payload: map[string]any{"project_id": "default"}}}); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}
	if err := client.DeleteByFilter(ctx, "cards", Filter{Must: []Condition{{Key: "workflow_id", Match: map[string]any{"value": "wf"}}}}); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if err := client.SetPayloadByFilter(
		ctx,
		"cards",
		map[string]any{"status": "disabled"},
		Filter{Must: []Condition{{Key: "workflow_id", Match: map[string]any{"value": "wf"}}}},
	); err != nil {
		t.Fatalf("set payload failed: %v", err)
	}
	if _, err := client.Search(ctx, "cards", SearchRequest{Vector: map[string]any{"name": "intent_dense", "vector": []float32{0.1, 0.2}}, Limit: 3}); err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if _, err := client.Query(ctx, "cards", QueryRequest{Query: []float32{0.1, 0.2}, Using: "intent_dense", Limit: 1}); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if _, err := client.QueryBatch(ctx, "cards", QueryBatchRequest{Searches: []QueryRequest{{Limit: 1}}}); err != nil {
		t.Fatalf("query batch failed: %v", err)
	}

	want := []string{
		"GET /healthz",
		"GET /collections/cards",
		"PUT /collections/cards/index",
		"PUT /collections/cards/points?wait=true",
		"POST /collections/cards/points/delete?wait=true",
		"POST /collections/cards/points/payload?wait=true",
		"POST /collections/cards/points/search",
		"POST /collections/cards/points/query",
		"POST /collections/cards/points/query/batch",
	}
	if len(requests) != len(want) {
		t.Fatalf("expected %d requests, got %d: %#v", len(want), len(requests), requests)
	}
	for i, request := range requests {
		got := strings.TrimSpace(request.Method + " " + request.Path)
		if request.Query != "" {
			got += "?" + request.Query
		}
		if got != want[i] {
			t.Fatalf("request %d path mismatch: got %q want %q", i, got, want[i])
		}
		if request.APIKey != "secret-key" {
			t.Fatalf("request %d missing api-key header", i)
		}
	}

	indexBody := requests[2].Body.(map[string]any)
	if indexBody["field_name"] != "project_id" || indexBody["field_schema"] != "keyword" {
		t.Fatalf("unexpected index body: %#v", indexBody)
	}
	upsertBody := requests[3].Body.(map[string]any)
	points := upsertBody["points"].([]any)
	if points[0].(map[string]any)["id"] != "workflow:wf:v1" {
		t.Fatalf("unexpected upsert body: %#v", upsertBody)
	}
}

type recordedRequest struct {
	Method string
	Path   string
	Query  string
	APIKey string
	Body   any
}
