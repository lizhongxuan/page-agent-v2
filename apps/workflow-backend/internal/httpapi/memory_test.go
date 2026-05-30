package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/page-agent/workflow-backend/internal/config"
	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestMemoryPageObservationsRoute(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	router := NewRouterWithServices(config.Config{}, Services{Registry: repo})

	response := performJSON(router, http.MethodPost, "/api/memory/page-observations", map[string]any{
		"projectId":   "default",
		"task":        "查询服务状态",
		"url":         "https://ops.example.com/services?k=kme-prod-001",
		"title":       "服务管理",
		"visibleText": []string{"服务管理", "服务名称", "状态"},
		"controls": []map[string]any{
			{"role": "textbox", "name": "服务名称"},
		},
	})

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		PageStateID string `json:"pageStateId"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body.PageStateID == "" {
		t.Fatalf("expected page state id: %#v", body)
	}
}

func TestMemoryPageObservationsRouteRejectsMissingProject(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	router := NewRouterWithServices(config.Config{}, Services{Registry: repo})

	response := performJSON(router, http.MethodPost, "/api/memory/page-observations", map[string]any{
		"url": "https://ops.example.com/services",
	})

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", response.Code, response.Body.String())
	}
}

func TestMemoryDocumentsAndContextRoutes(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	router := NewRouterWithServices(config.Config{}, Services{Registry: repo})

	docResponse := performJSON(router, http.MethodPost, "/api/memory/documents", map[string]any{
		"documents": []map[string]any{
			{
				"id":        "doc_service",
				"projectId": "default",
				"title":     "服务管理手册",
				"source":    "manual",
				"url":       "https://ops.example.com/services",
				"content":   "# 服务管理\n服务管理页可通过服务名称搜索框定位服务，状态列表示当前运行状态。",
			},
		},
	})
	if docResponse.Code != http.StatusOK {
		t.Fatalf("expected document status 200, got %d: %s", docResponse.Code, docResponse.Body.String())
	}
	var docBody struct {
		DocumentIDs            []string `json:"documentIds"`
		UpdatedBusinessProfile bool     `json:"updatedBusinessProfile"`
	}
	if err := json.Unmarshal(docResponse.Body.Bytes(), &docBody); err != nil {
		t.Fatalf("decode document response failed: %v", err)
	}
	if len(docBody.DocumentIDs) != 1 || !docBody.UpdatedBusinessProfile {
		t.Fatalf("unexpected document response: %#v", docBody)
	}

	observeResponse := performJSON(router, http.MethodPost, "/api/memory/page-observations", map[string]any{
		"projectId":   "default",
		"url":         "https://ops.example.com/services",
		"title":       "服务管理",
		"visibleText": []string{"服务管理", "服务名称", "状态"},
		"controls":    []map[string]any{{"role": "textbox", "name": "服务名称"}},
	})
	if observeResponse.Code != http.StatusOK {
		t.Fatalf("observe failed: %d %s", observeResponse.Code, observeResponse.Body.String())
	}
	contextResponse := performJSON(router, http.MethodPost, "/api/memory/context", map[string]any{
		"projectId":  "default",
		"task":       "查询服务状态",
		"currentUrl": "https://ops.example.com/services",
		"pageObservation": map[string]any{
			"title":       "服务管理",
			"visibleText": []string{"服务管理", "服务名称", "状态"},
			"controls":    []map[string]any{{"role": "textbox", "name": "服务名称"}},
		},
	})
	if contextResponse.Code != http.StatusOK {
		t.Fatalf("expected context status 200, got %d: %s", contextResponse.Code, contextResponse.Body.String())
	}
	var contextBody struct {
		ContextPrompt     string              `json:"contextPrompt"`
		RecommendedMode   registry.MemoryMode `json:"recommendedMode"`
		KnowledgeEvidence []map[string]any    `json:"knowledgeEvidence"`
		BusinessContext   map[string]string   `json:"businessContext"`
		EvidenceRefs      []map[string]any    `json:"evidenceRefs"`
	}
	if err := json.Unmarshal(contextResponse.Body.Bytes(), &contextBody); err != nil {
		t.Fatalf("decode context response failed: %v", err)
	}
	if !strings.Contains(contextBody.ContextPrompt, "<webops_memory>") {
		t.Fatalf("expected webops memory prompt: %s", contextBody.ContextPrompt)
	}
	if len(contextBody.KnowledgeEvidence) > 3 {
		t.Fatalf("knowledge evidence should be capped: %#v", contextBody.KnowledgeEvidence)
	}
	if len(contextBody.EvidenceRefs) == 0 || contextBody.EvidenceRefs[0]["source"] == "" || contextBody.EvidenceRefs[0]["id"] == "" {
		t.Fatalf("expected context evidence refs: %#v", contextBody.EvidenceRefs)
	}
}

func TestMemoryTaskRunsRouteConsolidates(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	router := NewRouterWithServices(config.Config{}, Services{Registry: repo})

	response := performJSON(router, http.MethodPost, "/api/memory/task-runs", sampleTaskRunBody())

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		TaskRunID     string   `json:"taskRunId"`
		ExperienceID  string   `json:"experienceId"`
		OptimizedPath []string `json:"optimizedPath"`
		MemoryUpdates []string `json:"memoryUpdates"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body.TaskRunID == "" || body.ExperienceID == "" || len(body.OptimizedPath) == 0 {
		t.Fatalf("unexpected consolidation response: %#v", body)
	}
}

func TestMemoryTaskRunsRouteRejectsActionValue(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	router := NewRouterWithServices(config.Config{}, Services{Registry: repo})
	body := sampleTaskRunBody()
	body["actionSteps"] = []map[string]any{{"actionType": "fill", "targetName": "服务名称", "value": "kme-prod-001"}}

	response := performJSON(router, http.MethodPost, "/api/memory/task-runs", body)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", response.Code, response.Body.String())
	}
}

func TestMemoryTaskRunsRouteAcceptsContextAndUpdatesAttribution(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	if err := repo.SaveMemoryContextEvent(nil, registry.MemoryContextEvent{
		ID:        "ctx_http_task_run",
		ProjectID: "default",
		Task:      "查看服务状态",
		EvidenceRefs: []registry.MemoryEvidenceRef{
			{
				ID:     "exp_http_service_status",
				Source: registry.MemoryEvidenceSourceExperience,
				Payload: map[string]any{
					"optimizedPath": []string{"page_a", "page_d"},
					"targetNames":   []string{"服务名称搜索框"},
				},
			},
		},
		RecommendedMode: registry.MemoryModeGuided,
	}); err != nil {
		t.Fatalf("SaveMemoryContextEvent failed: %v", err)
	}
	router := NewRouterWithServices(config.Config{}, Services{Registry: repo})
	body := sampleTaskRunBody()
	body["memoryContextId"] = "ctx_http_task_run"

	response := performJSON(router, http.MethodPost, "/api/memory/task-runs", body)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", response.Code, response.Body.String())
	}
	var result struct {
		MemoryUpdates []string `json:"memoryUpdates"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if !stringSliceContains(result.MemoryUpdates, "updated_memory_attribution") || !stringSliceContains(result.MemoryUpdates, "updated_evidence_stats") {
		t.Fatalf("expected attribution updates, got %#v", result.MemoryUpdates)
	}
}

func TestMemoryMaintenancePruneRouteDeletesOldLogRecords(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	router := NewRouterWithServices(config.Config{}, Services{Registry: repo})

	for index := 1; index <= 3; index++ {
		body := sampleTaskRunBody()
		body["id"] = "task_run_prune_" + string(rune('0'+index))
		body["summary"] = "查看服务运行状态。"
		response := performJSON(router, http.MethodPost, "/api/memory/task-runs", body)
		if response.Code != http.StatusCreated {
			t.Fatalf("seed task run %d failed: %d %s", index, response.Code, response.Body.String())
		}
	}

	response := performJSON(router, http.MethodPost, "/api/memory/maintenance/prune", map[string]any{
		"projectId":   "default",
		"site":        "ops.example.com",
		"maxTaskRuns": 1,
	})
	if response.Code != http.StatusOK {
		t.Fatalf("expected prune status 200, got %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		DeletedTaskRuns int `json:"deletedTaskRuns"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode prune response failed: %v", err)
	}
	if body.DeletedTaskRuns != 2 {
		t.Fatalf("expected two deleted task runs, got %#v", body)
	}
	runs, err := repo.ListTaskRuns(nil, registry.TaskRunListQuery{ProjectID: "default", Site: "ops.example.com"})
	if err != nil {
		t.Fatalf("ListTaskRuns failed: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected one task run after prune, got %#v", runs)
	}
}

func TestMemoryReviewsAndInspectorRoutes(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	if err := repo.SaveMemoryReview(nil, registry.MemoryReview{
		ID:         "review_exp",
		ProjectID:  "default",
		TargetType: registry.ReviewTargetExperience,
		TargetID:   "exp_1",
		Status:     registry.ReviewStatusPending,
		Summary:    "Review experience.",
	}); err != nil {
		t.Fatalf("SaveMemoryReview failed: %v", err)
	}
	if err := repo.SaveMemoryAttributionEvent(nil, registry.MemoryAttributionEvent{
		ID:             "attr_inspector",
		ProjectID:      "default",
		Site:           "ops.example.com",
		TaskRunID:      "task_run_inspector",
		ContextID:      "ctx_inspector",
		EvidenceID:     "exp_inspector",
		EvidenceSource: registry.MemoryEvidenceSourceExperience,
		Label:          registry.MemoryAttributionMisleading,
		Signals:        []string{"branchNoise"},
	}); err != nil {
		t.Fatalf("SaveMemoryAttributionEvent failed: %v", err)
	}
	if err := repo.SaveMemoryEvidenceStats(nil, registry.MemoryEvidenceStats{
		ProjectID:       "default",
		Site:            "ops.example.com",
		EvidenceID:      "exp_inspector",
		EvidenceSource:  registry.MemoryEvidenceSourceExperience,
		MisleadingCount: 1,
		UtilityScore:    -1.2,
	}); err != nil {
		t.Fatalf("SaveMemoryEvidenceStats failed: %v", err)
	}
	if err := repo.SaveMemoryContextEvent(nil, registry.MemoryContextEvent{
		ID:               "ctx_inspector",
		ProjectID:        "default",
		Task:             "查看服务状态",
		CurrentURL:       "https://ops.example.com/services",
		CurrentPageState: "page_service_list",
		CurrentSurface:   "surface_filter_drawer",
		ContextPrompt:    "<webops_memory>Use service search.</webops_memory>",
		RecommendedMode:  registry.MemoryModeGuided,
		EvidenceRefs:     []registry.MemoryEvidenceRef{{ID: "exp_inspector", Source: registry.MemoryEvidenceSourceExperience}},
	}); err != nil {
		t.Fatalf("SaveMemoryContextEvent failed: %v", err)
	}
	router := NewRouterWithServices(config.Config{}, Services{Registry: repo})

	listResponse := performJSON(router, http.MethodGet, "/api/memory/reviews?projectId=default", nil)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected reviews status 200, got %d: %s", listResponse.Code, listResponse.Body.String())
	}
	approveResponse := performJSON(router, http.MethodPost, "/api/memory/reviews/review_exp/approve", nil)
	if approveResponse.Code != http.StatusOK {
		t.Fatalf("expected approve status 200, got %d: %s", approveResponse.Code, approveResponse.Body.String())
	}
	inspectorResponse := performJSON(router, http.MethodGet, "/api/memory/inspector?projectId=default&contextId=ctx_inspector", nil)
	if inspectorResponse.Code != http.StatusOK {
		t.Fatalf("expected inspector status 200, got %d: %s", inspectorResponse.Code, inspectorResponse.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(inspectorResponse.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode inspector failed: %v", err)
	}
	if _, ok := body["reviews"]; !ok {
		t.Fatalf("expected reviews in inspector: %#v", body)
	}
	if events, ok := body["attributionEvents"].([]any); !ok || len(events) != 1 {
		t.Fatalf("expected attribution events in inspector: %#v", body)
	}
	if stats, ok := body["evidenceStats"].([]any); !ok || len(stats) != 1 {
		t.Fatalf("expected evidence stats in inspector: %#v", body)
	}
	contextEvent, ok := body["contextEvent"].(map[string]any)
	if !ok || !strings.Contains(contextEvent["contextPrompt"].(string), "<webops_memory>") {
		t.Fatalf("expected context prompt in inspector: %#v", body)
	}
}

func TestApproveWorkflowReviewActivatesWorkflowRecipe(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	workflow := reviewedWorkflowRecipe()
	if err := repo.SaveWorkflow(nil, workflow); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}
	if err := repo.SaveMemoryReview(nil, registry.MemoryReview{
		ID:         "review_workflow",
		ProjectID:  "default",
		TargetType: registry.ReviewTargetWorkflow,
		TargetID:   workflow.ID,
		Status:     registry.ReviewStatusPending,
		Summary:    "Review workflow recipe.",
	}); err != nil {
		t.Fatalf("SaveMemoryReview failed: %v", err)
	}
	router := NewRouterWithServices(config.Config{}, Services{Registry: repo})

	response := performJSON(router, http.MethodPost, "/api/memory/reviews/review_workflow/approve", nil)

	if response.Code != http.StatusOK {
		t.Fatalf("expected approve status 200, got %d: %s", response.Code, response.Body.String())
	}
	active, err := repo.GetWorkflow(nil, workflow.ID)
	if err != nil {
		t.Fatalf("GetWorkflow failed: %v", err)
	}
	if active.Status != registry.StatusActive || !active.Searchable {
		t.Fatalf("workflow review approval should activate searchable recipe: %#v", active)
	}
}

func reviewedWorkflowRecipe() registry.WorkflowRecipe {
	return registry.WorkflowRecipe{
		ID:          "wf_reviewed_service_status",
		Version:     1,
		ProjectID:   "default",
		Status:      registry.StatusPendingReview,
		Searchable:  false,
		Site:        "ops.example.com",
		Name:        "Check service status",
		Intent:      "查询服务状态",
		Description: "Open service detail from the service list.",
		RiskLevel:   registry.RiskReadOrSearch,
		Variables: []registry.Variable{
			{Name: "service_name", Type: registry.VariableString, Required: true, Source: registry.VariableSourceTask},
		},
		StartPageStates: []string{"page_service_list"},
		Chunks: []registry.WorkflowChunk{
			{
				ID:            "open_detail",
				Name:          "Open detail",
				FromPageState: "page_service_list",
				ToPageState:   "page_service_detail",
				RiskLevel:     registry.RiskReadOrSearch,
				Steps: []registry.WorkflowStep{
					{ID: "click_service", Type: registry.StepClick, RiskLevel: registry.RiskReadOrSearch},
				},
			},
		},
	}
}

func stringSliceContains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
