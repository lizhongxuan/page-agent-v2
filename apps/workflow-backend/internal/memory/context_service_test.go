package memory

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestMemoryContextServiceReturnsBusinessPageKnowledgeAndExperience(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	seedMemoryContextFixture(t, ctx, repo)
	service := NewMemoryContextService(repo)

	response, err := service.GetContext(ctx, MemoryContextRequest{
		ProjectID:  "default",
		Task:       "查询服务状态",
		CurrentURL: "https://ops.example.com/services",
		PageObservation: &PageObservationInput{
			Title:       "服务管理",
			VisibleText: []string{"服务管理", "服务名称", "状态"},
			Controls:    []PageObservationControl{{Role: "textbox", Name: "服务名称"}},
		},
		Mode: "before_task",
	})
	if err != nil {
		t.Fatalf("GetContext failed: %v", err)
	}
	if response.BusinessContext == nil || !strings.Contains(response.BusinessContext.Summary, "运维系统") {
		t.Fatalf("expected business context, got %#v", response.BusinessContext)
	}
	if response.CurrentPage == nil || response.CurrentPage.ID != "page_service_list" {
		t.Fatalf("expected current page, got %#v", response.CurrentPage)
	}
	if len(response.KnowledgeEvidence) != 1 || response.KnowledgeEvidence[0].ChunkID != "chunk_service" {
		t.Fatalf("unexpected knowledge evidence: %#v", response.KnowledgeEvidence)
	}
	if len(response.NavigationHints) != 1 {
		t.Fatalf("expected navigation hint, got %#v", response.NavigationHints)
	}
	if len(response.ExperienceHints) != 1 || response.ExperienceHints[0].ID != "exp_service_status" {
		t.Fatalf("unexpected experience hints: %#v", response.ExperienceHints)
	}
	if len(response.FailureWarnings) != 1 {
		t.Fatalf("expected failure warning, got %#v", response.FailureWarnings)
	}
	if response.RecommendedMode != registry.MemoryModeGuided {
		t.Fatalf("expected guided mode, got %q", response.RecommendedMode)
	}
	if len(response.EvidenceRefs) < 4 {
		t.Fatalf("expected evidence refs for all recalled memory sources, got %#v", response.EvidenceRefs)
	}
	assertEvidenceRef(t, response.EvidenceRefs, registry.MemoryEvidenceSourceKnowledge, "chunk_service")
	assertEvidenceRef(t, response.EvidenceRefs, registry.MemoryEvidenceSourceNavigation, "nav:page_service_list:page_service_detail")
	assertEvidenceRef(t, response.EvidenceRefs, registry.MemoryEvidenceSourceExperience, "exp_service_status")
	assertEvidenceRef(t, response.EvidenceRefs, registry.MemoryEvidenceSourceFailure, "fail_service_status")
	if !strings.Contains(response.ContextPrompt, "<webops_memory>") {
		t.Fatalf("expected context prompt, got %s", response.ContextPrompt)
	}
	events, err := repo.GetMemoryContextEvent(ctx, response.ContextID)
	if err != nil {
		t.Fatalf("expected context event to be saved: %v", err)
	}
	if events.RecommendedMode != registry.MemoryModeGuided {
		t.Fatalf("unexpected context event: %#v", events)
	}
	if len(events.EvidenceRefs) != len(response.EvidenceRefs) {
		t.Fatalf("expected context event to persist evidence refs, got %#v", events.EvidenceRefs)
	}
	if events.ContextPrompt != response.ContextPrompt {
		t.Fatalf("expected context event to persist injected prompt")
	}
}

func TestMemoryContextServiceCreatesDistinctContextEventsForRepeatedRequests(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	seedMemoryContextFixture(t, ctx, repo)
	service := NewMemoryContextService(repo)
	request := MemoryContextRequest{
		ProjectID:          "default",
		Task:               "查询服务状态",
		CurrentURL:         "https://ops.example.com/services",
		CurrentPageStateID: "page_service_list",
		Mode:               "before_task",
	}

	first, err := service.GetContext(ctx, request)
	if err != nil {
		t.Fatalf("first GetContext failed: %v", err)
	}
	second, err := service.GetContext(ctx, request)
	if err != nil {
		t.Fatalf("second GetContext failed: %v", err)
	}

	if first.ContextID == "" || second.ContextID == "" || first.ContextID == second.ContextID {
		t.Fatalf("expected distinct context ids, got first=%q second=%q", first.ContextID, second.ContextID)
	}
	if _, err := repo.GetMemoryContextEvent(ctx, first.ContextID); err != nil {
		t.Fatalf("first context event should still be retrievable: %v", err)
	}
	if _, err := repo.GetMemoryContextEvent(ctx, second.ContextID); err != nil {
		t.Fatalf("second context event should be retrievable: %v", err)
	}
}

func TestMemoryContextServiceReranksHelpfulEvidenceAheadOfMisleadingEvidence(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	seedMemoryContextFixture(t, ctx, repo)
	if err := repo.SaveExperienceMemory(ctx, registry.ExperienceMemory{
		ID:             "exp_wrong_branch",
		ProjectID:      "default",
		Site:           "ops.example.com",
		TaskTemplate:   "查询 {{service_name}} 服务状态",
		Intent:         "查询服务状态",
		Summary:        "旧路径会先进入账单页再返回。",
		StartPageState: "page_service_list",
		EndPageState:   "page_billing",
		OptimizedPath:  []string{"page_service_list", "page_billing"},
		Searchable:     true,
		ReviewStatus:   registry.ReviewStatusAutoApproved,
		Score:          0.99,
	}); err != nil {
		t.Fatalf("SaveExperienceMemory wrong branch failed: %v", err)
	}
	if err := repo.SaveMemoryEvidenceStats(ctx, registry.MemoryEvidenceStats{
		ProjectID:      "default",
		Site:           "ops.example.com",
		EvidenceID:     "exp_service_status",
		EvidenceSource: registry.MemoryEvidenceSourceExperience,
		HelpfulCount:   3,
		UtilityScore:   3,
		LastFeedbackAt: nowForTest(),
	}); err != nil {
		t.Fatalf("SaveMemoryEvidenceStats helpful failed: %v", err)
	}
	if err := repo.SaveMemoryEvidenceStats(ctx, registry.MemoryEvidenceStats{
		ProjectID:       "default",
		Site:            "ops.example.com",
		EvidenceID:      "exp_wrong_branch",
		EvidenceSource:  registry.MemoryEvidenceSourceExperience,
		MisleadingCount: 3,
		UtilityScore:    -3.6,
		LastFeedbackAt:  nowForTest(),
	}); err != nil {
		t.Fatalf("SaveMemoryEvidenceStats misleading failed: %v", err)
	}
	service := NewMemoryContextService(repo)

	response, err := service.GetContext(ctx, MemoryContextRequest{
		ProjectID:          "default",
		Task:               "查询服务状态",
		CurrentPageStateID: "page_service_list",
	})
	if err != nil {
		t.Fatalf("GetContext failed: %v", err)
	}

	if len(response.ExperienceHints) == 0 || response.ExperienceHints[0].ID != "exp_service_status" {
		t.Fatalf("expected helpful experience first, got %#v", response.ExperienceHints)
	}
	for _, hint := range response.ExperienceHints {
		if hint.ID == "exp_wrong_branch" {
			t.Fatalf("stale or misleading experience should be filtered from prompt candidates, got %#v", response.ExperienceHints)
		}
	}
}

func TestMemoryContextServiceFiltersStaleEvidenceFromPrompt(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	seedMemoryContextFixture(t, ctx, repo)
	if err := repo.SaveMemoryEvidenceStats(ctx, registry.MemoryEvidenceStats{
		ProjectID:      "default",
		Site:           "ops.example.com",
		EvidenceID:     "exp_service_status",
		EvidenceSource: registry.MemoryEvidenceSourceExperience,
		StaleCount:     3,
		UtilityScore:   -4.5,
		LastFeedbackAt: nowForTest(),
	}); err != nil {
		t.Fatalf("SaveMemoryEvidenceStats stale failed: %v", err)
	}
	service := NewMemoryContextService(repo)

	response, err := service.GetContext(ctx, MemoryContextRequest{
		ProjectID:          "default",
		Task:               "查询服务状态",
		CurrentPageStateID: "page_service_list",
	})
	if err != nil {
		t.Fatalf("GetContext failed: %v", err)
	}

	for _, hint := range response.ExperienceHints {
		if hint.ID == "exp_service_status" {
			t.Fatalf("stale experience should be filtered from hints, got %#v", response.ExperienceHints)
		}
	}
	if strings.Contains(response.ContextPrompt, "查询服务状态的最佳路径") {
		t.Fatalf("stale experience leaked into prompt: %s", response.ContextPrompt)
	}
}

func TestMemoryContextServiceRequiresPageHardRulesToMatch(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	seedMemoryContextFixture(t, ctx, repo)
	service := NewMemoryContextService(repo)

	response, err := service.GetContext(ctx, MemoryContextRequest{
		ProjectID:  "default",
		Task:       "查询服务状态",
		CurrentURL: "https://ops.example.com/services",
		PageObservation: &PageObservationInput{
			Title:       "服务管理",
			VisibleText: []string{"服务管理", "全局导航"},
			Controls:    []PageObservationControl{{Role: "button", Name: "打开菜单"}},
		},
	})
	if err != nil {
		t.Fatalf("GetContext failed: %v", err)
	}
	if response.CurrentPage != nil {
		t.Fatalf("page hard rules should not match, got %#v", response.CurrentPage)
	}
	if len(response.ExperienceHints) != 0 {
		t.Fatalf("experience hints should be filtered by page rules, got %#v", response.ExperienceHints)
	}
}

func TestMemoryContextServiceUsesSiteModuleScopedBusinessProfile(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	if err := repo.SaveBusinessSystemProfile(ctx, registry.BusinessSystemProfile{
		ProjectID:  "default",
		Site:       "ops.example.com",
		Module:     "services",
		Summary:    "运维系统的服务模块用于查询服务状态。",
		SourceType: registry.MemorySourceProduction,
	}); err != nil {
		t.Fatalf("SaveBusinessSystemProfile services failed: %v", err)
	}
	if err := repo.SaveBusinessSystemProfile(ctx, registry.BusinessSystemProfile{
		ProjectID:  "default",
		Site:       "docs.example.com",
		Module:     "docs",
		Summary:    "测试文档模块包含误导性的服务状态说明。",
		SourceType: registry.MemorySourceTest,
	}); err != nil {
		t.Fatalf("SaveBusinessSystemProfile docs failed: %v", err)
	}
	if err := repo.SavePageState(ctx, registry.PageState{
		ID:               "page_service_list",
		ProjectID:        "default",
		Site:             "ops.example.com",
		URLPattern:       "https://ops.example.com/services",
		CanonicalTitle:   "服务管理",
		RequiredText:     []string{"服务管理"},
		RequiredControls: []registry.ControlSignature{{Role: "textbox", Name: "服务名称"}},
		Status:           registry.StatusActive,
	}); err != nil {
		t.Fatalf("SavePageState failed: %v", err)
	}
	service := NewMemoryContextService(repo)

	response, err := service.GetContext(ctx, MemoryContextRequest{
		ProjectID:  "default",
		Task:       "查询服务状态",
		CurrentURL: "https://ops.example.com/services",
		PageObservation: &PageObservationInput{
			Title:       "服务管理",
			VisibleText: []string{"服务管理", "服务名称", "状态"},
			Controls:    []PageObservationControl{{Role: "textbox", Name: "服务名称"}},
		},
	})
	if err != nil {
		t.Fatalf("GetContext failed: %v", err)
	}
	if response.BusinessContext == nil || !strings.Contains(response.BusinessContext.Summary, "运维系统的服务模块") {
		t.Fatalf("expected site/module scoped business context, got %#v", response.BusinessContext)
	}
	if strings.Contains(response.ContextPrompt, "测试文档模块") {
		t.Fatalf("test or cross-site business profile leaked into prompt: %s", response.ContextPrompt)
	}
}

func TestMemoryContextServiceDoesNotFallbackToWrongSiteModuleProfile(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	if err := repo.SaveBusinessSystemProfile(ctx, registry.BusinessSystemProfile{
		ProjectID:  "default",
		Site:       "ops.example.com",
		Module:     "billing",
		Summary:    "账单模块用于查看发票，不适合服务状态任务。",
		SourceType: registry.MemorySourceProduction,
	}); err != nil {
		t.Fatalf("SaveBusinessSystemProfile billing failed: %v", err)
	}
	if err := repo.SavePageState(ctx, registry.PageState{
		ID:               "page_service_list",
		ProjectID:        "default",
		Site:             "ops.example.com",
		URLPattern:       "https://ops.example.com/services",
		CanonicalTitle:   "服务管理",
		RequiredText:     []string{"服务管理"},
		RequiredControls: []registry.ControlSignature{{Role: "textbox", Name: "服务名称"}},
		Status:           registry.StatusActive,
	}); err != nil {
		t.Fatalf("SavePageState failed: %v", err)
	}
	service := NewMemoryContextService(repo)

	response, err := service.GetContext(ctx, MemoryContextRequest{
		ProjectID:  "default",
		Task:       "查询服务状态",
		CurrentURL: "https://ops.example.com/services",
		PageObservation: &PageObservationInput{
			Title:       "服务管理",
			VisibleText: []string{"服务管理", "服务名称"},
			Controls:    []PageObservationControl{{Role: "textbox", Name: "服务名称"}},
		},
	})
	if err != nil {
		t.Fatalf("GetContext failed: %v", err)
	}
	if response.BusinessContext != nil {
		t.Fatalf("wrong module profile should not be injected: %#v", response.BusinessContext)
	}
}

func TestMemoryContextServiceAppliesHardGatingBeforeSemanticTopK(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	seedMemoryContextFixture(t, ctx, repo)
	if err := repo.SaveKnowledgeChunks(ctx, []registry.KnowledgeChunk{
		{
			ID:        "chunk_unscoped_test_decoy",
			ProjectID: "default",
			Title:     "服务管理测试资料",
			ChunkText: "服务管理页应该优先打开账单中心，这是测试资料。",
			Metadata: map[string]any{
				"sourceType": string(registry.MemorySourceTest),
			},
		},
		{
			ID:        "chunk_cross_site_decoy",
			ProjectID: "default",
			Title:     "服务管理跨站资料",
			ChunkText: "服务管理页应该进入跨站控制台，这是错误站点资料。",
			Metadata: map[string]any{
				"url":        "https://docs.example.com/services",
				"sourceType": string(registry.MemorySourceProduction),
			},
		},
	}); err != nil {
		t.Fatalf("SaveKnowledgeChunks decoys failed: %v", err)
	}
	service := NewMemoryContextService(repo)

	response, err := service.GetContext(ctx, MemoryContextRequest{
		ProjectID:  "default",
		Task:       "查询服务管理服务状态",
		CurrentURL: "https://ops.example.com/services",
		PageObservation: &PageObservationInput{
			Title:       "服务管理",
			VisibleText: []string{"服务管理", "服务名称", "状态"},
			Controls:    []PageObservationControl{{Role: "textbox", Name: "服务名称"}},
		},
	})
	if err != nil {
		t.Fatalf("GetContext failed: %v", err)
	}
	for _, evidence := range response.KnowledgeEvidence {
		if evidence.ChunkID == "chunk_unscoped_test_decoy" || evidence.ChunkID == "chunk_cross_site_decoy" {
			t.Fatalf("hard-gated knowledge should not include decoy evidence: %#v", response.KnowledgeEvidence)
		}
	}
	if response.Debug == nil || response.Debug.PromptBudget.MaxChars <= 0 || response.Debug.PromptChars <= 0 {
		t.Fatalf("expected prompt debug budget details, got %#v", response.Debug)
	}
	if len(response.Debug.FilteredEvidence) == 0 {
		t.Fatalf("expected debug filtered evidence entries, got %#v", response.Debug)
	}
}

func TestMemoryContextServiceExperienceEvidenceRefsIncludeTargets(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	seedMemoryContextFixture(t, ctx, repo)
	service := NewMemoryContextService(repo)

	response, err := service.GetContext(ctx, MemoryContextRequest{
		ProjectID:          "default",
		Task:               "查询服务状态",
		CurrentPageStateID: "page_service_list",
	})
	if err != nil {
		t.Fatalf("GetContext failed: %v", err)
	}
	for _, ref := range response.EvidenceRefs {
		if ref.Source != registry.MemoryEvidenceSourceExperience || ref.ID != "exp_service_status" {
			continue
		}
		targets := stringListFromPayload(ref.Payload, "targetNames")
		if !containsString(targets, "服务名称搜索框") {
			t.Fatalf("experience evidence ref should include reusable target names, got %#v", ref)
		}
		stepTargets, ok := ref.Payload["stepTargets"].([]registry.MemoryStepTarget)
		if !ok || len(stepTargets) == 0 || stepTargets[0].ValueTemplate != "{{service_name}}" {
			t.Fatalf("experience evidence ref should include structured step targets, got %#v", ref.Payload)
		}
		return
	}
	t.Fatalf("missing experience evidence ref in %#v", response.EvidenceRefs)
}

func TestMemoryContextServiceFiltersExperienceByCurrentSurface(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	if err := repo.SavePageState(ctx, registry.PageState{
		ID:               "page_service_list",
		ProjectID:        "default",
		Site:             "ops.example.com",
		URLPattern:       "https://ops.example.com/services",
		CanonicalTitle:   "服务管理",
		RequiredText:     []string{"服务管理"},
		RequiredControls: []registry.ControlSignature{{Role: "textbox", Name: "服务名称"}},
		Status:           registry.StatusActive,
	}); err != nil {
		t.Fatalf("SavePageState failed: %v", err)
	}
	currentObservation, err := NormalizePageObservation(PageObservationRequest{
		ProjectID:   "default",
		URL:         "https://ops.example.com/services",
		Title:       "服务管理",
		VisibleText: []string{"服务管理", "编辑服务弹窗", "确认保存"},
		Controls: []PageObservationControl{
			{Role: "textbox", Name: "服务名称"},
			{Role: "dialog", Name: "编辑服务弹窗"},
			{Role: "button", Name: "确认保存"},
		},
	})
	if err != nil || currentObservation.Surface == nil {
		t.Fatalf("expected current surface observation, got %#v err=%v", currentObservation, err)
	}
	currentSurfaceID := stableSurfaceID(currentObservation, "page_service_list", *currentObservation.Surface)
	if err := repo.SavePageSurface(ctx, registry.PageSurface{
		ID:                 currentSurfaceID,
		ProjectID:          "default",
		Site:               "ops.example.com",
		ParentPageStateID:  "page_service_list",
		SurfaceType:        currentObservation.Surface.Type,
		SurfaceFingerprint: BuildSurfaceFingerprint(currentObservation, *currentObservation.Surface),
		Title:              currentObservation.Surface.Title,
		RequiredControls:   currentObservation.Surface.Controls,
		Status:             registry.StatusActive,
	}); err != nil {
		t.Fatalf("SavePageSurface current failed: %v", err)
	}
	otherSurfaceID := "surface_other_drawer"
	for _, item := range []struct {
		id        string
		summary   string
		surfaceID string
	}{
		{id: "exp_edit_modal", summary: "在编辑弹窗中点击确认保存。", surfaceID: currentSurfaceID},
		{id: "exp_filter_drawer", summary: "在筛选抽屉中点击应用筛选。", surfaceID: otherSurfaceID},
	} {
		if err := repo.SaveExperienceMemory(ctx, registry.ExperienceMemory{
			ID:             item.id,
			ProjectID:      "default",
			Site:           "ops.example.com",
			TaskTemplate:   "编辑服务配置",
			Intent:         "编辑服务配置",
			Summary:        item.summary,
			StartPageState: "page_service_list",
			EndPageState:   "page_service_list",
			OptimizedPath:  []string{"page_service_list"},
			StepsSummary: []registry.ExperienceStepSummary{{
				ActionName:  string(registry.StepClick),
				TargetName:  "确认保存",
				PageStateID: "page_service_list",
				SurfaceID:   item.surfaceID,
			}},
			Searchable:   true,
			ReviewStatus: registry.ReviewStatusAutoApproved,
			SuccessCount: 2,
		}); err != nil {
			t.Fatalf("SaveExperienceMemory %s failed: %v", item.id, err)
		}
	}
	service := NewMemoryContextService(repo)

	response, err := service.GetContext(ctx, MemoryContextRequest{
		ProjectID:  "default",
		Task:       "编辑服务配置",
		CurrentURL: "https://ops.example.com/services",
		PageObservation: &PageObservationInput{
			Title:       "服务管理",
			VisibleText: []string{"服务管理", "编辑服务弹窗", "确认保存"},
			Controls: []PageObservationControl{
				{Role: "textbox", Name: "服务名称"},
				{Role: "dialog", Name: "编辑服务弹窗"},
				{Role: "button", Name: "确认保存"},
			},
		},
	})
	if err != nil {
		t.Fatalf("GetContext failed: %v", err)
	}
	if response.CurrentSurface == nil || response.CurrentSurface.ID != currentSurfaceID {
		t.Fatalf("expected current surface %q, got %#v", currentSurfaceID, response.CurrentSurface)
	}
	if len(response.ExperienceHints) != 1 || response.ExperienceHints[0].ID != "exp_edit_modal" {
		t.Fatalf("expected only matching surface experience, got %#v", response.ExperienceHints)
	}
	if !debugContainsFilter(response.Debug, "experience", "exp_filter_drawer", "surface_mismatch") {
		t.Fatalf("expected surface mismatch debug filter, got %#v", response.Debug)
	}
}

func TestMemoryContextServiceAppliesSurfaceGateBeforeTop3Limit(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	if err := repo.SavePageState(ctx, registry.PageState{
		ID:               "page_service_list",
		ProjectID:        "default",
		Site:             "ops.example.com",
		URLPattern:       "https://ops.example.com/services",
		CanonicalTitle:   "服务管理",
		RequiredText:     []string{"服务管理"},
		RequiredControls: []registry.ControlSignature{{Role: "textbox", Name: "服务名称"}},
		Status:           registry.StatusActive,
	}); err != nil {
		t.Fatalf("SavePageState failed: %v", err)
	}
	currentObservation, err := NormalizePageObservation(PageObservationRequest{
		ProjectID:   "default",
		URL:         "https://ops.example.com/services",
		Title:       "服务管理",
		VisibleText: []string{"服务管理", "编辑服务弹窗", "确认保存"},
		Controls: []PageObservationControl{
			{Role: "textbox", Name: "服务名称"},
			{Role: "dialog", Name: "编辑服务弹窗"},
			{Role: "button", Name: "确认保存"},
		},
	})
	if err != nil {
		t.Fatalf("NormalizePageObservation failed: %v", err)
	}
	currentSurfaceID := stableSurfaceID(currentObservation, "page_service_list", *currentObservation.Surface)
	if err := repo.SavePageSurface(ctx, registry.PageSurface{
		ID:                 currentSurfaceID,
		ProjectID:          "default",
		Site:               "ops.example.com",
		ParentPageStateID:  "page_service_list",
		SurfaceType:        currentObservation.Surface.Type,
		SurfaceFingerprint: BuildSurfaceFingerprint(currentObservation, *currentObservation.Surface),
		Title:              currentObservation.Surface.Title,
		Status:             registry.StatusActive,
	}); err != nil {
		t.Fatalf("SavePageSurface failed: %v", err)
	}
	for _, item := range []struct {
		id           string
		surfaceID    string
		successCount int
	}{
		{id: "exp_wrong_drawer_a", surfaceID: "surface_wrong_drawer_a", successCount: 10},
		{id: "exp_wrong_drawer_b", surfaceID: "surface_wrong_drawer_b", successCount: 9},
		{id: "exp_wrong_drawer_c", surfaceID: "surface_wrong_drawer_c", successCount: 8},
		{id: "exp_right_modal", surfaceID: currentSurfaceID, successCount: 1},
	} {
		if err := repo.SaveExperienceMemory(ctx, registry.ExperienceMemory{
			ID:             item.id,
			ProjectID:      "default",
			Site:           "ops.example.com",
			TaskTemplate:   "编辑服务配置",
			Intent:         "编辑服务配置",
			Summary:        "编辑服务配置时使用当前叠加态按钮。",
			StartPageState: "page_service_list",
			EndPageState:   "page_service_list",
			OptimizedPath:  []string{"page_service_list"},
			StepsSummary: []registry.ExperienceStepSummary{{
				ActionName:  string(registry.StepClick),
				TargetName:  "确认保存",
				PageStateID: "page_service_list",
				SurfaceID:   item.surfaceID,
			}},
			Searchable:    true,
			ReviewStatus:  registry.ReviewStatusAutoApproved,
			SuccessCount:  item.successCount,
			LastSuccessAt: nowForTest().Add(time.Duration(item.successCount) * time.Minute),
		}); err != nil {
			t.Fatalf("SaveExperienceMemory %s failed: %v", item.id, err)
		}
	}
	service := NewMemoryContextService(repo)

	response, err := service.GetContext(ctx, MemoryContextRequest{
		ProjectID:  "default",
		Task:       "编辑服务配置",
		CurrentURL: "https://ops.example.com/services",
		PageObservation: &PageObservationInput{
			Title:       "服务管理",
			VisibleText: []string{"服务管理", "编辑服务弹窗", "确认保存"},
			Controls: []PageObservationControl{
				{Role: "textbox", Name: "服务名称"},
				{Role: "dialog", Name: "编辑服务弹窗"},
				{Role: "button", Name: "确认保存"},
			},
		},
	})
	if err != nil {
		t.Fatalf("GetContext failed: %v", err)
	}
	if len(response.ExperienceHints) != 1 || response.ExperienceHints[0].ID != "exp_right_modal" {
		t.Fatalf("surface gate should run before top3 limit, got %#v", response.ExperienceHints)
	}
	for _, id := range []string{"exp_wrong_drawer_a", "exp_wrong_drawer_b", "exp_wrong_drawer_c"} {
		if !debugContainsFilter(response.Debug, "experience", id, "surface_mismatch") {
			t.Fatalf("expected surface mismatch filter for %s, got %#v", id, response.Debug)
		}
	}
}

func TestMemoryContextServiceBudgetPrunesWithoutBreakingPromptXML(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	if err := repo.SaveBusinessSystemProfile(ctx, registry.BusinessSystemProfile{
		ProjectID:  "default",
		Site:       "ops.example.com",
		Module:     "services",
		Summary:    strings.Repeat("服务模块说明", 80),
		SourceType: registry.MemorySourceProduction,
	}); err != nil {
		t.Fatalf("SaveBusinessSystemProfile failed: %v", err)
	}
	if err := repo.SavePageState(ctx, registry.PageState{
		ID:               "page_service_list",
		ProjectID:        "default",
		Site:             "ops.example.com",
		URLPattern:       "https://ops.example.com/services",
		CanonicalTitle:   "服务管理",
		RequiredText:     []string{"服务管理"},
		RequiredControls: []registry.ControlSignature{{Role: "textbox", Name: "服务名称"}},
		Status:           registry.StatusActive,
	}); err != nil {
		t.Fatalf("SavePageState failed: %v", err)
	}
	for index := 0; index < 3; index++ {
		if err := repo.SavePageTransition(ctx, registry.PageTransition{
			ID:            "transition_budget_" + string(rune('a'+index)),
			ProjectID:     "default",
			Site:          "ops.example.com",
			FromPageState: "page_service_list",
			ToPageState:   "page_service_detail_" + string(rune('a'+index)),
			ActionName:    strings.Repeat("打开服务详情并确认当前状态。", 45),
			TargetName:    "服务详情入口",
			RiskLevel:     registry.RiskReadOrSearch,
		}); err != nil {
			t.Fatalf("SavePageTransition failed: %v", err)
		}
	}
	for index := 0; index < 3; index++ {
		id := "exp_budget_" + string(rune('a'+index))
		if err := repo.SaveExperienceMemory(ctx, registry.ExperienceMemory{
			ID:             id,
			ProjectID:      "default",
			Site:           "ops.example.com",
			TaskTemplate:   "查询服务状态",
			Intent:         "查询服务状态",
			Summary:        strings.Repeat("成功路径需要先搜索服务再打开详情。", 25),
			StartPageState: "page_service_list",
			EndPageState:   "page_service_detail",
			OptimizedPath:  []string{"page_service_list", "page_service_detail"},
			StepsSummary: []registry.ExperienceStepSummary{{
				ActionName:  string(registry.StepFill),
				TargetName:  "服务名称搜索框",
				PageStateID: "page_service_list",
			}},
			Searchable:   true,
			ReviewStatus: registry.ReviewStatusAutoApproved,
			SuccessCount: 5,
		}); err != nil {
			t.Fatalf("SaveExperienceMemory %s failed: %v", id, err)
		}
		if err := repo.SaveFailureMemory(ctx, registry.FailureMemory{
			ID:             "fail_budget_" + string(rune('a'+index)),
			ProjectID:      "default",
			Site:           "ops.example.com",
			PageStateID:    "page_service_list",
			FailureType:    "wrong_branch",
			FailureSummary: strings.Repeat("不要进入无关页面。", 40),
			AvoidHint:      strings.Repeat("优先使用当前页搜索框。", 40),
		}); err != nil {
			t.Fatalf("SaveFailureMemory failed: %v", err)
		}
	}
	chunks := []registry.KnowledgeChunk{}
	for index := 0; index < 3; index++ {
		chunks = append(chunks, registry.KnowledgeChunk{
			ID:        "chunk_budget_" + string(rune('a'+index)),
			ProjectID: "default",
			Title:     "服务管理手册",
			ChunkText: strings.Repeat("服务状态字段说明，运行中表示服务健康。", 90),
			Metadata: map[string]any{
				"url":        "https://ops.example.com/services",
				"site":       "ops.example.com",
				"module":     "services",
				"sourceType": string(registry.MemorySourceProduction),
			},
		})
	}
	if err := repo.SaveKnowledgeChunks(ctx, chunks); err != nil {
		t.Fatalf("SaveKnowledgeChunks failed: %v", err)
	}
	service := NewMemoryContextService(repo)

	response, err := service.GetContext(ctx, MemoryContextRequest{
		ProjectID:  "default",
		Task:       "查询服务状态",
		CurrentURL: "https://ops.example.com/services",
		PageObservation: &PageObservationInput{
			Title:       "服务管理",
			VisibleText: []string{"服务管理", "服务名称"},
			Controls:    []PageObservationControl{{Role: "textbox", Name: "服务名称"}},
		},
	})
	if err != nil {
		t.Fatalf("GetContext failed: %v", err)
	}
	if len([]rune(response.ContextPrompt)) > defaultMemoryPromptBudgetChars {
		t.Fatalf("prompt exceeded budget: %d", len([]rune(response.ContextPrompt)))
	}
	if !strings.HasSuffix(strings.TrimSpace(response.ContextPrompt), "</webops_memory>") {
		t.Fatalf("prompt should remain well-formed XML after budget pruning: %s", response.ContextPrompt)
	}
	if !debugContainsReason(response.Debug, "budget_exceeded") {
		t.Fatalf("expected budget_exceeded debug filters, got %#v", response.Debug)
	}
	for _, ref := range response.EvidenceRefs {
		if !strings.Contains(response.ContextPrompt, ref.ID) && ref.Source != registry.MemoryEvidenceSourceNavigation {
			t.Fatalf("evidence ref should only include prompt-visible evidence, ref=%#v prompt=%s", ref, response.ContextPrompt)
		}
	}
}

func seedMemoryContextFixture(t *testing.T, ctx context.Context, repo registry.Repository) {
	t.Helper()
	if err := repo.SaveBusinessSystemProfile(ctx, registry.BusinessSystemProfile{
		ProjectID:  "default",
		Site:       "ops.example.com",
		Module:     "services",
		Summary:    "这是运维系统，服务管理模块用于查询服务状态。",
		SourceType: registry.MemorySourceProduction,
	}); err != nil {
		t.Fatalf("SaveBusinessSystemProfile failed: %v", err)
	}
	if err := repo.SavePageState(ctx, registry.PageState{
		ID:               "page_service_list",
		ProjectID:        "default",
		Site:             "ops.example.com",
		URLPattern:       "https://ops.example.com/services",
		CanonicalTitle:   "服务管理",
		RequiredText:     []string{"服务管理"},
		RequiredControls: []registry.ControlSignature{{Role: "textbox", Name: "服务名称"}},
		Status:           registry.StatusActive,
	}); err != nil {
		t.Fatalf("SavePageState list failed: %v", err)
	}
	if err := repo.SavePageTransition(ctx, registry.PageTransition{
		ID:            "transition_service_detail",
		ProjectID:     "default",
		Site:          "ops.example.com",
		FromPageState: "page_service_list",
		ToPageState:   "page_service_detail",
		ActionName:    "搜索服务名后打开详情",
		TargetName:    "服务名称链接",
		RiskLevel:     registry.RiskReadOrSearch,
	}); err != nil {
		t.Fatalf("SavePageTransition failed: %v", err)
	}
	if err := repo.SaveKnowledgeChunks(ctx, []registry.KnowledgeChunk{
		{
			ID:        "chunk_service",
			ProjectID: "default",
			Title:     "服务管理手册",
			ChunkText: "服务管理页可通过服务名称搜索框定位服务。",
			Metadata: map[string]any{
				"url":        "https://ops.example.com/services",
				"site":       "ops.example.com",
				"module":     "services",
				"sourceType": string(registry.MemorySourceProduction),
			},
		},
	}); err != nil {
		t.Fatalf("SaveKnowledgeChunks failed: %v", err)
	}
	if err := repo.SaveExperienceMemory(ctx, registry.ExperienceMemory{
		ID:             "exp_service_status",
		ProjectID:      "default",
		Site:           "ops.example.com",
		TaskTemplate:   "查询 {{service_name}} 服务状态",
		Intent:         "查询服务状态",
		Summary:        "查询服务状态的最佳路径是服务列表页搜索服务名，然后进入详情页。",
		StartPageState: "page_service_list",
		EndPageState:   "page_service_detail",
		OptimizedPath:  []string{"page_service_list", "page_service_detail"},
		StepsSummary: []registry.ExperienceStepSummary{
			{
				ActionName:    string(registry.StepFill),
				TargetName:    "服务名称搜索框",
				ValueTemplate: "{{service_name}}",
				PageStateID:   "page_service_list",
			},
			{
				ActionName:  string(registry.StepClick),
				TargetName:  "服务名称链接",
				PageStateID: "page_service_list",
			},
		},
		Variables:    []registry.Variable{{Name: "service_name", Type: registry.VariableString}},
		SuccessCount: 2,
		Searchable:   true,
		ReviewStatus: registry.ReviewStatusAutoApproved,
	}); err != nil {
		t.Fatalf("SaveExperienceMemory failed: %v", err)
	}
	if err := repo.SaveFailureMemory(ctx, registry.FailureMemory{
		ID:             "fail_service_status",
		ProjectID:      "default",
		Site:           "ops.example.com",
		PageStateID:    "page_service_list",
		ActionName:     "打开菜单",
		FailureType:    "branch_noise",
		FailureSummary: "不要从首页菜单逐项探索。",
		AvoidHint:      "当前页已有服务名称搜索框，优先使用。",
	}); err != nil {
		t.Fatalf("SaveFailureMemory failed: %v", err)
	}
}

func assertEvidenceRef(t *testing.T, refs []registry.MemoryEvidenceRef, source registry.MemoryEvidenceSource, idPrefix string) {
	t.Helper()
	for _, ref := range refs {
		if ref.Source == source && strings.HasPrefix(ref.ID, idPrefix) && ref.Rank > 0 {
			return
		}
	}
	t.Fatalf("missing evidence ref %s %s in %#v", source, idPrefix, refs)
}

func debugContainsFilter(debug *MemoryContextDebug, source, id, reason string) bool {
	if debug == nil {
		return false
	}
	for _, item := range debug.FilteredEvidence {
		if item.Source == source && item.ID == id && item.Reason == reason {
			return true
		}
	}
	return false
}

func debugContainsReason(debug *MemoryContextDebug, reason string) bool {
	if debug == nil {
		return false
	}
	for _, item := range debug.FilteredEvidence {
		if item.Reason == reason {
			return true
		}
	}
	return false
}

func nowForTest() time.Time {
	return time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
}
