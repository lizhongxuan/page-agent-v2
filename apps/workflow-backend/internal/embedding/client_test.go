package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAICompatibleClientDenseRequest(t *testing.T) {
	var auth string
	var requestModel string
	var requestInput []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		var body struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request failed: %v", err)
		}
		requestModel = body.Model
		requestInput = body.Input
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"embedding": []float64{0.1, 0.2}},
			},
		})
	}))
	defer server.Close()
	client := NewOpenAICompatibleClient(server.URL, "secret", "bge-m3", server.Client())

	vectors, err := client.EmbedDense(context.Background(), []string{"hello"})

	if err != nil {
		t.Fatalf("EmbedDense failed: %v", err)
	}
	if auth != "Bearer secret" {
		t.Fatalf("expected bearer auth, got %q", auth)
	}
	if requestModel != "bge-m3" || len(requestInput) != 1 || requestInput[0] != "hello" {
		t.Fatalf("unexpected request body: %q %#v", requestModel, requestInput)
	}
	if len(vectors) != 1 || len(vectors[0].Dense) != 2 {
		t.Fatalf("unexpected vectors: %#v", vectors)
	}
}

func TestDeterministicSparseProvider(t *testing.T) {
	provider := DeterministicSparseProvider{Model: "test-sparse"}

	vectors, err := provider.EmbedSparse(context.Background(), []string{"github issues github"})

	if err != nil {
		t.Fatalf("EmbedSparse failed: %v", err)
	}
	if len(vectors) != 1 || len(vectors[0].Sparse.Indices) == 0 {
		t.Fatalf("expected sparse vector, got %#v", vectors)
	}
	if vectors[0].Model != "test-sparse" {
		t.Fatalf("expected model to be set, got %#v", vectors[0])
	}
}
