package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"net/http"
	"sort"
	"strings"
)

type SparseVector struct {
	Indices []uint32  `json:"indices"`
	Values  []float32 `json:"values"`
}

type Vector struct {
	Dense  []float32    `json:"dense,omitempty"`
	Sparse SparseVector `json:"sparse,omitempty"`
	Model  string       `json:"model,omitempty"`
}

type DenseProvider interface {
	EmbedDense(context.Context, []string) ([]Vector, error)
}

type SparseProvider interface {
	EmbedSparse(context.Context, []string) ([]Vector, error)
}

type OpenAICompatibleClient struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewOpenAICompatibleClient(baseURL string, apiKey string, model string, httpClient *http.Client) *OpenAICompatibleClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &OpenAICompatibleClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		model:      model,
		httpClient: httpClient,
	}
}

func (client *OpenAICompatibleClient) EmbedDense(ctx context.Context, texts []string) ([]Vector, error) {
	if client.baseURL == "" {
		return nil, errors.New("embedding base url is required")
	}
	body, err := json.Marshal(map[string]any{
		"model": client.model,
		"input": texts,
	})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	if client.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+client.apiKey)
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("embedding request failed with status %d", response.StatusCode)
	}
	var decoded struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	vectors := make([]Vector, 0, len(decoded.Data))
	for _, item := range decoded.Data {
		dense := make([]float32, 0, len(item.Embedding))
		for _, value := range item.Embedding {
			dense = append(dense, float32(value))
		}
		vectors = append(vectors, Vector{Dense: dense, Model: client.model})
	}
	return vectors, nil
}

type DeterministicSparseProvider struct {
	Model string
}

func (provider DeterministicSparseProvider) EmbedSparse(_ context.Context, texts []string) ([]Vector, error) {
	result := make([]Vector, 0, len(texts))
	for _, text := range texts {
		counts := map[uint32]float32{}
		for _, token := range strings.Fields(strings.ToLower(text)) {
			index := sparseIndex(token)
			counts[index] += 1
		}
		indices := make([]uint32, 0, len(counts))
		for index := range counts {
			indices = append(indices, index)
		}
		sort.Slice(indices, func(i, j int) bool { return indices[i] < indices[j] })
		values := make([]float32, 0, len(indices))
		for _, index := range indices {
			values = append(values, counts[index])
		}
		result = append(result, Vector{
			Sparse: SparseVector{Indices: indices, Values: values},
			Model:  provider.Model,
		})
	}
	return result, nil
}

func sparseIndex(token string) uint32 {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(token))
	return hash.Sum32()
}
