package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/page-agent/workflow-backend/internal/config"
	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestSiteTaskGuideRoutesListDisableAndFeedback(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	guide := registry.SiteTaskGuide{
		ID:                "guide_restore",
		ProjectID:         "default",
		Site:              "ops.example.com",
		TaskIntentKey:     "restore_instance",
		TaskIntentSummary: "Restore instance from backup.",
		Summary:           "Restore instance from backup.",
		Steps:             []registry.SiteTaskGuideStep{{Text: "Click Restore.", Target: "Restore"}},
		UIStateEntries: []registry.SiteTaskGuideUIStateEntry{{
			ID:        "state_restore",
			Name:      "Restore page",
			StateType: registry.SiteTaskGuideUIStatePage,
			Evidence: registry.SiteTaskGuideUIStateEvidence{
				ControlsAll: []registry.ControlSignature{{Role: "button", Name: "Restore"}},
			},
			MinimumScore: 1,
		}},
		Status: registry.StatusActive,
	}
	if err := repo.SaveSiteTaskGuide(context.Background(), guide); err != nil {
		t.Fatalf("SaveSiteTaskGuide failed: %v", err)
	}
	router := NewRouterWithServices(config.Config{}, Services{Registry: repo})

	listResponse := performJSON(router, http.MethodGet, "/api/memory/site-task-guides?projectId=default&site=ops.example.com", nil)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected list status 200, got %d: %s", listResponse.Code, listResponse.Body.String())
	}
	var listBody struct {
		Guides []registry.SiteTaskGuide `json:"guides"`
	}
	if err := json.Unmarshal(listResponse.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode list failed: %v", err)
	}
	if len(listBody.Guides) != 1 || listBody.Guides[0].ID != guide.ID {
		t.Fatalf("unexpected guides: %#v", listBody.Guides)
	}

	feedbackResponse := performJSON(router, http.MethodPost, "/api/memory/site-task-guides/guide_restore/feedback", map[string]any{
		"label":  "abandoned_mismatch",
		"reason": "Current page is not a backup page.",
	})
	if feedbackResponse.Code != http.StatusOK {
		t.Fatalf("expected feedback status 200, got %d: %s", feedbackResponse.Code, feedbackResponse.Body.String())
	}
	var feedbackBody registry.SiteTaskGuide
	if err := json.Unmarshal(feedbackResponse.Body.Bytes(), &feedbackBody); err != nil {
		t.Fatalf("decode feedback failed: %v", err)
	}
	if feedbackBody.AbandonedCount != 1 {
		t.Fatalf("expected abandoned count to increment: %#v", feedbackBody)
	}

	disableResponse := performJSON(router, http.MethodPost, "/api/memory/site-task-guides/guide_restore/disable", nil)
	if disableResponse.Code != http.StatusOK {
		t.Fatalf("expected disable status 200, got %d: %s", disableResponse.Code, disableResponse.Body.String())
	}
	var disabled registry.SiteTaskGuide
	if err := json.Unmarshal(disableResponse.Body.Bytes(), &disabled); err != nil {
		t.Fatalf("decode disabled failed: %v", err)
	}
	if disabled.Status != registry.StatusDisabled {
		t.Fatalf("expected disabled guide: %#v", disabled)
	}
}

func TestTaskRunCreatesGuideAndContextRecallsIt(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	router := NewRouterWithServices(config.Config{}, Services{Registry: repo})

	taskRunResponse := performJSON(router, http.MethodPost, "/api/memory/task-runs", map[string]any{
		"id":           "task_run_restore",
		"projectId":    "default",
		"site":         "ops.example.com",
		"taskTemplate": "恢复 {{instance_name}} 实例",
		"summary":      "恢复完成。",
		"status":       "success",
		"actionSteps": []map[string]any{
			{"id": "step_open", "stepIndex": 1, "actionType": "click", "targetName": "实例名称", "resultSummary": "进入详情页。", "beforeObservation": map[string]any{"projectId": "default", "site": "ops.example.com", "url": "https://ops.example.com/instances", "urlPattern": "https://ops.example.com/instances", "title": "实例列表", "visibleTextSample": "实例名称", "controlSignatures": []map[string]any{{"role": "button", "name": "实例名称"}}}},
			{"id": "step_restore", "stepIndex": 2, "actionType": "click", "targetName": "数据恢复", "resultSummary": "点击数据恢复。", "beforeObservation": map[string]any{"projectId": "default", "site": "ops.example.com", "url": "https://ops.example.com/instances", "urlPattern": "https://ops.example.com/instances", "title": "实例详情", "visibleTextSample": "数据恢复 开始恢复", "controlSignatures": []map[string]any{{"role": "button", "name": "数据恢复"}}}},
			{"id": "step_confirm", "stepIndex": 3, "actionType": "click", "targetName": "开始恢复", "resultSummary": "开始恢复。", "beforeObservation": map[string]any{"projectId": "default", "site": "ops.example.com", "url": "https://ops.example.com/instances", "urlPattern": "https://ops.example.com/instances", "title": "恢复弹窗", "visibleTextSample": "开始恢复", "activeSurfaces": []map[string]any{{"surfaceType": "modal", "title": "恢复", "controls": []map[string]any{{"role": "button", "name": "开始恢复"}}}}}},
		},
	})
	if taskRunResponse.Code != http.StatusCreated {
		t.Fatalf("expected task run status 201, got %d: %s", taskRunResponse.Code, taskRunResponse.Body.String())
	}
	var taskRunBody struct {
		SiteTaskGuideID string `json:"siteTaskGuideId"`
	}
	if err := json.Unmarshal(taskRunResponse.Body.Bytes(), &taskRunBody); err != nil {
		t.Fatalf("decode task run failed: %v", err)
	}
	if taskRunBody.SiteTaskGuideID == "" {
		t.Fatalf("expected task run to create guide: %s", taskRunResponse.Body.String())
	}

	contextResponse := performJSON(router, http.MethodPost, "/api/memory/context", map[string]any{
		"projectId":  "default",
		"task":       "帮我恢复一个实例",
		"currentUrl": "https://ops.example.com/instances",
		"pageObservation": map[string]any{
			"title":       "实例详情",
			"visibleText": []string{"实例名称", "数据恢复", "开始恢复"},
			"controls":    []map[string]any{{"role": "button", "name": "数据恢复"}},
		},
	})
	if contextResponse.Code != http.StatusOK {
		t.Fatalf("expected context status 200, got %d: %s", contextResponse.Code, contextResponse.Body.String())
	}
	var contextBody struct {
		SiteTaskGuides []map[string]any `json:"siteTaskGuides"`
		ContextPrompt  string           `json:"contextPrompt"`
	}
	if err := json.Unmarshal(contextResponse.Body.Bytes(), &contextBody); err != nil {
		t.Fatalf("decode context failed: %v", err)
	}
	if len(contextBody.SiteTaskGuides) != 1 {
		t.Fatalf("expected guide recall: %s", contextResponse.Body.String())
	}
	if !strings.Contains(contextBody.ContextPrompt, "<site_task_guides>") {
		t.Fatalf("expected guide prompt: %s", contextBody.ContextPrompt)
	}
}
