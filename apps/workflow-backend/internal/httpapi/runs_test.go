package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/page-agent/workflow-backend/internal/selector"
)

func TestWorkflowRunsHTTP(t *testing.T) {
	service := selector.NewService(selector.NewMemoryRepository())
	router := NewRouter(Dependencies{Selector: service})

	runReq := httptest.NewRequest(http.MethodPost, "/api/workflow-runs", strings.NewReader(`{
		"id":"run_http",
		"projectId":"default",
		"task":"search docs",
		"url":"https://example.com",
		"result":"success"
	}`))
	runRes := httptest.NewRecorder()
	router.ServeHTTP(runRes, runReq)
	if runRes.Code != http.StatusCreated {
		t.Fatalf("expected run create 201, got %d %s", runRes.Code, runRes.Body.String())
	}

	statsReq := httptest.NewRequest(http.MethodPost, "/api/workflow-runs/run_http/selector-stats", strings.NewReader(`{
		"updates":[{"workflowId":"wf_1","workflowVersion":1,"stepId":"step_1","strategy":"role","selector":"button:Search","success":true}]
	}`))
	statsRes := httptest.NewRecorder()
	router.ServeHTTP(statsRes, statsReq)
	if statsRes.Code != http.StatusOK {
		t.Fatalf("expected stats 200, got %d %s", statsRes.Code, statsRes.Body.String())
	}
}
