package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func performJSON(handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	var requestBody bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&requestBody).Encode(body)
	}
	request := httptest.NewRequest(method, path, &requestBody)
	if body != nil {
		request.Header.Set("content-type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func assertGoneWithMigration(t *testing.T, response *httptest.ResponseRecorder, migration string) {
	t.Helper()
	if response.Code != http.StatusGone {
		t.Fatalf("expected 410, got %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body.Code != "endpoint_deprecated" || !bytes.Contains([]byte(body.Message), []byte(migration)) {
		t.Fatalf("expected migration hint %q, got %#v", migration, body)
	}
}

type fakeHTTPIndexer struct {
	count            int
	rebuildProjectID string
	indexWorkflowID  string
}

func (indexer *fakeHTTPIndexer) IndexWorkflow(_ context.Context, workflowID string) error {
	indexer.indexWorkflowID = workflowID
	return nil
}

func (indexer *fakeHTTPIndexer) Rebuild(_ context.Context, projectID string) (int, error) {
	indexer.rebuildProjectID = projectID
	return indexer.count, nil
}
