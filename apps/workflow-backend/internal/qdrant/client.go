package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ClientConfig struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type Point struct {
	ID      string         `json:"id"`
	Vector  map[string]any `json:"vector,omitempty"`
	Payload map[string]any `json:"payload,omitempty"`
}

type PayloadIndex struct {
	FieldName   string `json:"field_name"`
	FieldSchema string `json:"field_schema"`
}

type Condition struct {
	Key   string         `json:"key"`
	Match map[string]any `json:"match,omitempty"`
	Range map[string]any `json:"range,omitempty"`
}

type Filter struct {
	Must    []Condition `json:"must,omitempty"`
	Should  []Condition `json:"should,omitempty"`
	MustNot []Condition `json:"must_not,omitempty"`
}

type SearchRequest struct {
	Vector      any     `json:"vector,omitempty"`
	Filter      *Filter `json:"filter,omitempty"`
	Limit       int     `json:"limit,omitempty"`
	WithPayload any     `json:"with_payload,omitempty"`
	WithVector  any     `json:"with_vector,omitempty"`
}

type QueryRequest struct {
	Query       any     `json:"query,omitempty"`
	Using       string  `json:"using,omitempty"`
	Filter      *Filter `json:"filter,omitempty"`
	Limit       int     `json:"limit,omitempty"`
	WithPayload any     `json:"with_payload,omitempty"`
	WithVector  any     `json:"with_vector,omitempty"`
}

type QueryBatchRequest struct {
	Searches []QueryRequest `json:"searches"`
}

type SearchResponse struct {
	Result json.RawMessage `json:"result"`
}

func NewClient(config ClientConfig) *Client {
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{
		baseURL:    strings.TrimRight(config.BaseURL, "/"),
		apiKey:     config.APIKey,
		httpClient: httpClient,
	}
}

func (client *Client) Health(ctx context.Context) error {
	return client.do(ctx, http.MethodGet, "/healthz", nil, nil)
}

func (client *Client) CollectionExists(ctx context.Context, collection string) (bool, error) {
	err := client.do(ctx, http.MethodGet, "/collections/"+collection, nil, nil)
	if errors.Is(err, errNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (client *Client) CreateCollection(ctx context.Context, collection string, body map[string]any) error {
	return client.do(ctx, http.MethodPut, "/collections/"+collection, body, nil)
}

func (client *Client) CreatePayloadIndex(ctx context.Context, collection string, index PayloadIndex) error {
	return client.do(ctx, http.MethodPut, "/collections/"+collection+"/index", index, nil)
}

func (client *Client) UpsertPoints(ctx context.Context, collection string, points []Point) error {
	body := map[string]any{"points": points}
	return client.do(ctx, http.MethodPut, "/collections/"+collection+"/points?wait=true", body, nil)
}

func (client *Client) DeleteByFilter(ctx context.Context, collection string, filter Filter) error {
	body := map[string]any{"filter": filter}
	return client.do(ctx, http.MethodPost, "/collections/"+collection+"/points/delete?wait=true", body, nil)
}

func (client *Client) SetPayloadByFilter(ctx context.Context, collection string, payload map[string]any, filter Filter) error {
	body := map[string]any{
		"payload": payload,
		"filter":  filter,
	}
	return client.do(ctx, http.MethodPost, "/collections/"+collection+"/points/payload?wait=true", body, nil)
}

func (client *Client) Search(ctx context.Context, collection string, request SearchRequest) (SearchResponse, error) {
	var response SearchResponse
	err := client.do(ctx, http.MethodPost, "/collections/"+collection+"/points/search", request, &response)
	return response, err
}

func (client *Client) Query(ctx context.Context, collection string, request QueryRequest) (SearchResponse, error) {
	var response SearchResponse
	err := client.do(ctx, http.MethodPost, "/collections/"+collection+"/points/query", request, &response)
	return response, err
}

func (client *Client) QueryBatch(ctx context.Context, collection string, request QueryBatchRequest) (SearchResponse, error) {
	var response SearchResponse
	err := client.do(ctx, http.MethodPost, "/collections/"+collection+"/points/query/batch", request, &response)
	return response, err
}

var errNotFound = errors.New("qdrant resource not found")

func (client *Client) do(ctx context.Context, method string, path string, body any, out any) error {
	if client.baseURL == "" {
		return errors.New("qdrant base URL is required")
	}
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode qdrant request: %w", err)
		}
		reader = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, client.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("build qdrant request: %w", err)
	}
	if body != nil {
		request.Header.Set("content-type", "application/json")
	}
	if client.apiKey != "" {
		request.Header.Set("api-key", client.apiKey)
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("qdrant request failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return errNotFound
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return fmt.Errorf("qdrant %s %s returned %d: %s", method, path, response.StatusCode, strings.TrimSpace(string(data)))
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, response.Body)
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(out); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("decode qdrant response: %w", err)
	}
	return nil
}
