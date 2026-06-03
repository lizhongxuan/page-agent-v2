package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
