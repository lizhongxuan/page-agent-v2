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

func TestMemoryContextRouteUsesGuideAndManualShape(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	router := NewRouterWithServices(config.Config{}, Services{Registry: repo})

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
		ContextPrompt       string              `json:"contextPrompt"`
		RecommendedMode     registry.MemoryMode `json:"recommendedMode"`
		SiteTaskGuides      []map[string]any    `json:"siteTaskGuides"`
		SiteManualKnowledge []map[string]any    `json:"siteManualKnowledge"`
		EvidenceRefs        []map[string]any    `json:"evidenceRefs"`
	}
	if err := json.Unmarshal(contextResponse.Body.Bytes(), &contextBody); err != nil {
		t.Fatalf("decode context response failed: %v", err)
	}
	if !strings.Contains(contextBody.ContextPrompt, "<webops_memory>") {
		t.Fatalf("expected webops memory prompt: %s", contextBody.ContextPrompt)
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
		OptimizedPath []string `json:"optimizedPath"`
		MemoryUpdates []string `json:"memoryUpdates"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body.TaskRunID == "" || len(body.OptimizedPath) == 0 {
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

func TestMemoryMaintenancePruneUpdatesSiteGuideAndManualStatuses(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	if err := repo.SaveSiteTaskGuide(nil, registry.SiteTaskGuide{
		ID:            "guide_stale",
		ProjectID:     "default",
		Site:          "ops.example.com",
		TaskIntentKey: "restore_instance",
		Summary:       "Restore instance.",
		Steps:         []registry.SiteTaskGuideStep{{Text: "Click Restore.", Target: "Restore"}},
		UIStateEntries: []registry.SiteTaskGuideUIStateEntry{{
			ID:        "state_restore",
			Name:      "Restore page",
			StateType: registry.SiteTaskGuideUIStatePage,
			Evidence: registry.SiteTaskGuideUIStateEvidence{
				ControlsAll: []registry.ControlSignature{{Role: "button", Name: "Restore"}},
			},
			MinimumScore: 1,
		}},
		Status:          registry.StatusActive,
		MisleadingCount: 3,
	}); err != nil {
		t.Fatalf("SaveSiteTaskGuide failed: %v", err)
	}
	if err := repo.SaveSiteManualSource(nil, registry.SiteManualSource{
		ID:          "manual_source_stale",
		ProjectID:   "default",
		Site:        "ops.example.com",
		Title:       "Ops manual",
		SourceType:  registry.SiteManualSourceMarkdown,
		ContentHash: "hash_stale_manual",
		RawContent:  "Click legacy restore.",
		Status:      registry.StatusActive,
	}); err != nil {
		t.Fatalf("SaveSiteManualSource failed: %v", err)
	}
	if err := repo.SaveSiteManualWiki(nil, []registry.SiteManualWikiPage{{
		ID:        "manual_page_stale",
		ProjectID: "default",
		Site:      "ops.example.com",
		PageKey:   "restore",
		Title:     "Restore",
		Summary:   "Restore summary.",
		SourceRefs: []registry.MemorySourceRef{{
			Type: "site_manual_source",
			ID:   "manual_source_stale",
		}},
		Status: registry.StatusActive,
	}}, []registry.SiteManualWikiChunk{{
		ID:         "manual_chunk_stale",
		WikiPageID: "manual_page_stale",
		ProjectID:  "default",
		Site:       "ops.example.com",
		ChunkType:  registry.SiteManualChunkProcedure,
		Text:       "Click legacy restore.",
		SourceRefs: []registry.MemorySourceRef{{
			Type: "site_manual_source",
			ID:   "manual_source_stale",
		}},
		Status: registry.StatusActive,
	}}); err != nil {
		t.Fatalf("SaveSiteManualWiki failed: %v", err)
	}
	if err := repo.SaveMemoryEvidenceStats(nil, registry.MemoryEvidenceStats{
		ProjectID:      "default",
		Site:           "ops.example.com",
		EvidenceID:     "manual_chunk_stale",
		EvidenceSource: registry.MemoryEvidenceSourceManual,
		StaleCount:     3,
	}); err != nil {
		t.Fatalf("SaveMemoryEvidenceStats failed: %v", err)
	}
	router := NewRouterWithServices(config.Config{}, Services{Registry: repo})

	response := performJSON(router, http.MethodPost, "/api/memory/maintenance/prune", map[string]any{
		"projectId": "default",
		"site":      "ops.example.com",
	})
	if response.Code != http.StatusOK {
		t.Fatalf("expected prune status 200, got %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		UpdatedSiteTaskGuides    int `json:"updatedSiteTaskGuides"`
		UpdatedSiteManualSources int `json:"updatedSiteManualSources"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode prune response failed: %v", err)
	}
	if body.UpdatedSiteTaskGuides != 1 || body.UpdatedSiteManualSources != 1 {
		t.Fatalf("expected site memory status updates, got %#v", body)
	}
	guide, err := repo.GetSiteTaskGuide(nil, "guide_stale")
	if err != nil {
		t.Fatalf("GetSiteTaskGuide failed: %v", err)
	}
	if guide.Status != registry.StatusHidden {
		t.Fatalf("expected misleading guide to be hidden, got %#v", guide)
	}
	source, err := repo.GetSiteManualSource(nil, "manual_source_stale")
	if err != nil {
		t.Fatalf("GetSiteManualSource failed: %v", err)
	}
	if source.Status != registry.StatusStale {
		t.Fatalf("expected stale manual source, got %#v", source)
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
	if err := repo.SaveMemoryAttributionEvent(nil, registry.MemoryAttributionEvent{
		ID:             "attr_guide",
		ProjectID:      "default",
		Site:           "ops.example.com",
		TaskRunID:      "task_run_inspector",
		ContextID:      "ctx_inspector",
		EvidenceID:     "guide_restore",
		EvidenceSource: registry.MemoryEvidenceSourceGuide,
		Label:          registry.MemoryAttributionUnused,
		Signals:        []string{"pageRuleMismatch"},
	}); err != nil {
		t.Fatalf("SaveMemoryAttributionEvent guide failed: %v", err)
	}
	if err := repo.SaveMemoryAttributionEvent(nil, registry.MemoryAttributionEvent{
		ID:             "attr_manual",
		ProjectID:      "default",
		Site:           "ops.example.com",
		TaskRunID:      "task_run_inspector",
		ContextID:      "ctx_inspector",
		EvidenceID:     "manual_chunk_restore",
		EvidenceSource: registry.MemoryEvidenceSourceManual,
		Label:          registry.MemoryAttributionStale,
		Signals:        []string{"pageRuleMismatch"},
	}); err != nil {
		t.Fatalf("SaveMemoryAttributionEvent manual failed: %v", err)
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
		EvidenceRefs: []registry.MemoryEvidenceRef{
			{ID: "exp_inspector", Source: registry.MemoryEvidenceSourceExperience},
			{ID: "guide_restore", Source: registry.MemoryEvidenceSourceGuide, Reason: "Matched guide."},
			{ID: "manual_chunk_restore", Source: registry.MemoryEvidenceSourceManual, Reason: "Matched manual."},
		},
		Payload: map[string]any{
			"pageObservationSignal": map[string]any{
				"site":  "ops.example.com",
				"title": "服务管理",
			},
			"debug": map[string]any{
				"filteredEvidence": []map[string]any{{"source": "manual", "reason": "page_guard_failed"}},
			},
		},
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
	if events, ok := body["attributionEvents"].([]any); !ok || len(events) != 2 {
		t.Fatalf("expected attribution events in inspector: %#v", body)
	}
	if stats, ok := body["evidenceStats"].([]any); !ok || len(stats) != 0 {
		t.Fatalf("expected evidence stats in inspector: %#v", body)
	}
	contextEvent, ok := body["contextEvent"].(map[string]any)
	if !ok || !strings.Contains(contextEvent["contextPrompt"].(string), "<webops_memory>") {
		t.Fatalf("expected context prompt in inspector: %#v", body)
	}
	if signal, ok := body["pageObservationSignal"].(map[string]any); !ok || signal["site"] != "ops.example.com" {
		t.Fatalf("expected page observation signal in inspector: %#v", body)
	}
	if guides, ok := body["candidateSiteTaskGuides"].([]any); !ok || len(guides) != 1 {
		t.Fatalf("expected candidate guides in inspector: %#v", body)
	}
	if manuals, ok := body["candidateSiteManualKnowledge"].([]any); !ok || len(manuals) != 1 {
		t.Fatalf("expected candidate manuals in inspector: %#v", body)
	}
	if filtered, ok := body["filteredEvidence"].([]any); !ok || len(filtered) != 1 {
		t.Fatalf("expected filtered evidence in inspector: %#v", body)
	}
	if guideFeedback, ok := body["guideFeedback"].([]any); !ok || len(guideFeedback) != 1 {
		t.Fatalf("expected guide feedback in inspector: %#v", body)
	}
	if manualFeedback, ok := body["manualFeedback"].([]any); !ok || len(manualFeedback) != 1 {
		t.Fatalf("expected manual feedback in inspector: %#v", body)
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
