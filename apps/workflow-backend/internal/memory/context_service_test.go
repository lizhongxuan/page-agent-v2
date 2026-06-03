package memory

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestMemoryContextServiceDoesNotEmitOldKnowledgeContext(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewMemoryContextService(repo)

	response, err := service.GetContext(context.Background(), MemoryContextRequest{
		ProjectID:  "default",
		Task:       "restore an instance from the latest full backup",
		CurrentURL: "https://ops.example.com/instances/pg-1/backups",
		PageObservation: &PageObservationInput{
			Title:       "Data backup",
			VisibleText: []string{"Data backup", "Full Backup", "Data Restore"},
			Controls: []PageObservationControl{
				{Role: "button", Name: "Data Restore"},
			},
		},
	})
	if err != nil {
		t.Fatalf("GetContext failed: %v", err)
	}
	if response.ContextID == "" {
		t.Fatalf("expected context id")
	}
	if !strings.Contains(response.ContextPrompt, "<webops_memory>") {
		t.Fatalf("expected memory wrapper: %s", response.ContextPrompt)
	}
}

func TestMemoryContextRecommendedModeUsesNewGuideAndManualFields(t *testing.T) {
	response := MemoryContextResponse{
		SiteTaskGuides: []SiteTaskGuideHint{{ID: "guide_restore", WhenToUse: "restore from backup"}},
	}
	if got := recommendedMode(response); got != registry.MemoryModeGuided {
		t.Fatalf("expected guided mode for task guide, got %s", got)
	}
	response = MemoryContextResponse{
		SiteManualKnowledge: []SiteManualKnowledgeHint{{ID: "manual_restore", Summary: "restore starts from full backup"}},
	}
	if got := recommendedMode(response); got != registry.MemoryModeGuided {
		t.Fatalf("expected guided mode for manual knowledge, got %s", got)
	}
}

func TestMemoryContextDoesNotInjectLegacyExperienceHints(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	if err := repo.SavePageState(ctx, registry.PageState{
		ID:             "page_backups",
		ProjectID:      "default",
		Site:           "middleware.example.com",
		URLPattern:     "https://middleware.example.com/instances/backups",
		CanonicalTitle: "Data Backup",
		RequiredText:   []string{"Data Backup", "Full Backup", "Data Restore"},
		RequiredControls: []registry.ControlSignature{
			{Role: "button", Name: "Data Restore"},
		},
		Status: registry.StatusActive,
	}); err != nil {
		t.Fatalf("SavePageState failed: %v", err)
	}
	if err := repo.SaveExperienceMemory(ctx, registry.ExperienceMemory{
		ID:             "exp_legacy_dynamic",
		ProjectID:      "default",
		Site:           "middleware.example.com",
		TaskTemplate:   "restore instance from latest backup",
		Intent:         "restore instance from latest backup",
		Summary:        "{{input_value}}",
		StartPageState: "page_backups",
		OptimizedPath:  []string{"page_backups"},
		StepsSummary: []registry.ExperienceStepSummary{
			{ActionName: "click", TargetName: "element_33"},
		},
		Searchable:   true,
		ReviewStatus: registry.ReviewStatusApproved,
		SuccessCount: 1,
		Confidence:   0.9,
	}); err != nil {
		t.Fatalf("SaveExperienceMemory failed: %v", err)
	}

	response, err := NewMemoryContextService(repo).GetContext(ctx, MemoryContextRequest{
		ProjectID:          "default",
		Task:               "restore instance from latest backup",
		CurrentURL:         "https://middleware.example.com/instances/backups",
		CurrentPageStateID: "page_backups",
		PageObservation: &PageObservationInput{
			Title:       "Data Backup",
			VisibleText: []string{"Data Backup", "Full Backup", "Data Restore"},
			Controls:    []PageObservationControl{{Role: "button", Name: "Data Restore"}},
		},
	})
	if err != nil {
		t.Fatalf("GetContext failed: %v", err)
	}
	if len(response.ExperienceHints) != 0 {
		t.Fatalf("legacy experience hints should not be injected: %#v", response.ExperienceHints)
	}
	if strings.Contains(response.ContextPrompt, "<experience_hints>") || strings.Contains(response.ContextPrompt, "element_33") {
		t.Fatalf("legacy experience prompt leaked: %s", response.ContextPrompt)
	}
	for _, ref := range response.EvidenceRefs {
		if ref.Source == registry.MemoryEvidenceSourceExperience {
			t.Fatalf("legacy experience evidence should not be selected: %#v", response.EvidenceRefs)
		}
	}
}

func TestMemoryContextFiltersGuideWhenUIStatePreflightFails(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	if err := repo.SaveSiteTaskGuide(context.Background(), contextTestRestoreGuide()); err != nil {
		t.Fatalf("SaveSiteTaskGuide failed: %v", err)
	}

	response, err := NewMemoryContextService(repo).GetContext(context.Background(), MemoryContextRequest{
		ProjectID:  "default",
		Task:       "restore instance from latest backup",
		CurrentURL: "https://middleware.example.com/settings/audit",
		PageObservation: &PageObservationInput{
			Title:       "Audit Settings",
			VisibleText: []string{"Audit Settings", "Retention days", "Save"},
			Controls:    []PageObservationControl{{Role: "button", Name: "Save"}},
		},
	})
	if err != nil {
		t.Fatalf("GetContext failed: %v", err)
	}
	if len(response.SiteTaskGuides) != 0 {
		t.Fatalf("expected no guide injection on wrong page, got %#v", response.SiteTaskGuides)
	}
	if response.Debug == nil || !debugContainsFilteredReason(response.Debug, "ui_state_preflight_failed") {
		t.Fatalf("expected ui state preflight debug, got %#v", response.Debug)
	}
}

func TestMemoryContextFiltersGuideWhenOnlyShellTitleMatches(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	guide := contextTestRestoreGuide()
	guide.TaskIntentSummary = "让{{instance_name}}实例基于最新的备份数据进行恢复操作"
	guide.TaskIntentTerms = registry.SiteTaskIntentTerms{
		Positive: []string{"恢复", "备份", "数据恢复"},
		Negative: []string{"删除"},
	}
	guide.UIStateEntries = []registry.SiteTaskGuideUIStateEntry{{
		ID:         "state_shell_title",
		Name:       "企业级云原生平台 - 企业数字战斗力引擎",
		StateType:  registry.SiteTaskGuideUIStatePage,
		StepOffset: 0,
		RouteScope: registry.SiteTaskGuideRouteScope{URLIncludes: []string{"/console"}},
		Evidence: registry.SiteTaskGuideUIStateEvidence{
			TitleAny: []string{"企业级云原生平台 - 企业数字战斗力引擎"},
			TextAny:  []string{"企业级云原生平台"},
			ControlsAny: []registry.ControlSignature{
				{Name: "lzxpg"},
			},
		},
		MinimumScore: 2,
		Status:       registry.StatusActive,
	}}
	guide.StartPageGuard = registry.SiteTaskGuideGuard{
		RequiredText: []string{"企业级云原生平台 - 企业数字战斗力引擎"},
	}
	guide.Steps = []registry.SiteTaskGuideStep{
		{Text: "点击实例名称进入详情页。", Target: "实例名称"},
		{Text: "进入实例数据备份页面。", Target: "数据备份"},
		{Text: "切换到全量备份标签页并点击数据恢复。", Target: "数据恢复"},
	}
	if err := repo.SaveSiteTaskGuide(context.Background(), guide); err != nil {
		t.Fatalf("SaveSiteTaskGuide failed: %v", err)
	}

	response, err := NewMemoryContextService(repo).GetContext(context.Background(), MemoryContextRequest{
		ProjectID:  "default",
		Task:       "使用最新的备份数据,让lzxpg实例进行数据恢复",
		CurrentURL: "https://middleware.example.com/console/#/settings/audit",
		PageObservation: &PageObservationInput{
			Title:       "企业级云原生平台 - 企业数字战斗力引擎",
			VisibleText: []string{"企业级云原生平台", "系统设置", "审计日志", "保存"},
			Controls:    []PageObservationControl{{Role: "button", Name: "保存"}},
		},
	})
	if err != nil {
		t.Fatalf("GetContext failed: %v", err)
	}
	if len(response.SiteTaskGuides) != 0 {
		t.Fatalf("expected shell-only page match to be rejected, got %#v", response.SiteTaskGuides)
	}
	if response.Debug == nil || !debugContainsFilteredReason(response.Debug, "step_anchor_mismatch") {
		t.Fatalf("expected step anchor mismatch debug, got %#v", response.Debug)
	}
}

func TestMemoryProductionCodeDoesNotHardcodeSiteSpecificBusinessLabels(t *testing.T) {
	for _, file := range []string{"context_service.go", "site_task_guide_service.go", "task_intent_gate.go"} {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("ReadFile(%s) failed: %v", file, err)
		}
		for _, forbidden := range []string{"数据备份", "数据恢复", "全量备份", "开始恢复", "实例名称", "最新备份记录", "节点 IP", "Full Backup", "Data Restore"} {
			if strings.Contains(string(source), forbidden) {
				t.Fatalf("%s must not hardcode site-specific business label %q", file, forbidden)
			}
		}
	}
}

func TestGuideStepAnchorMatchesUsesGuideOwnedUIEvidenceForAnySite(t *testing.T) {
	guide := registry.SiteTaskGuide{
		ID:            "guide_contract_review",
		ProjectID:     "default",
		Site:          "contracts.example.com",
		TaskIntentKey: "submit_contract_review",
		StartPageGuard: registry.SiteTaskGuideGuard{
			RequiredText: []string{"Quarterly Contract Approval"},
		},
		UIStateEntries: []registry.SiteTaskGuideUIStateEntry{{
			ID:        "state_review_queue",
			StateType: registry.SiteTaskGuideUIStatePage,
			Evidence: registry.SiteTaskGuideUIStateEvidence{
				TitleAny: []string{"Enterprise Portal"},
			},
			MinimumScore: 2,
			Status:       registry.StatusActive,
		}},
		Steps: []registry.SiteTaskGuideStep{
			{Text: "Open the selected work item.", Target: "Work item"},
			{Text: "Submit it for review.", Target: "Review action"},
		},
		Status: registry.StatusActive,
	}
	signal := &registry.PageObservationSignal{
		Title:             "Enterprise Portal",
		VisibleTextSample: "Quarterly Contract Approval\nPending review",
		ControlSignatures: []registry.ControlSignature{{Role: "button", Name: "Send for Legal Review"}},
	}

	if !guideStepAnchorMatches(guide, signal, uiStateMatchResult{Matched: []string{"title"}}) {
		t.Fatalf("expected guide-owned UI evidence to provide generic step anchors")
	}
}

func TestSiteTaskGuideGuardMatchesAnyRecordedControl(t *testing.T) {
	guard := registry.SiteTaskGuideGuard{
		RequiredText: []string{"Review Queue"},
		RequiredControls: []registry.ControlSignature{
			{Role: "button", Name: "Send for Legal Review"},
			{Role: "button", Name: "Refresh"},
		},
	}
	request := MemoryContextRequest{
		PageObservation: &PageObservationInput{
			VisibleText: []string{"Review Queue"},
			Controls:    []PageObservationControl{{Role: "button", Name: "Send for Legal Review"}},
		},
	}
	if !siteTaskGuideGuardMatches(guard, request) {
		t.Fatalf("expected guide guard to match when one recorded stable control is present")
	}
}

func TestSiteTaskGuideGuardMatchesActiveSurfaceOverlay(t *testing.T) {
	guard := registry.SiteTaskGuideGuard{
		RequiredText:      []string{"Pending Reviews"},
		ActiveOverlayHint: "Legal Review",
		RequiredControls:  []registry.ControlSignature{{Role: "button", Name: "Confirm"}},
	}
	request := MemoryContextRequest{
		PageObservation: &PageObservationInput{
			VisibleText: []string{"Pending Reviews"},
			ActiveSurfaces: []registry.ActiveSurfaceSignal{{
				SurfaceType: registry.SurfaceModal,
				Title:       "Legal Review",
				Controls:    []registry.ControlSignature{{Role: "button", Name: "Confirm"}},
			}},
			Controls: []PageObservationControl{{Role: "button", Name: "Confirm"}},
		},
	}
	if !siteTaskGuideGuardMatches(guard, request) {
		t.Fatalf("expected guide guard to match active surface overlay")
	}
}

func TestMemoryContextFiltersGuideForOppositeTaskIntent(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	if err := repo.SaveSiteTaskGuide(context.Background(), contextTestRestoreGuide()); err != nil {
		t.Fatalf("SaveSiteTaskGuide failed: %v", err)
	}

	response, err := NewMemoryContextService(repo).GetContext(context.Background(), MemoryContextRequest{
		ProjectID:  "default",
		Task:       "delete the latest backup record",
		CurrentURL: "https://middleware.example.com/instances/pg-prod/backups",
		PageObservation: &PageObservationInput{
			Title:       "Data Backup",
			VisibleText: []string{"Data Backup", "Full Backup", "Data Restore"},
			ActiveTabs:  []string{"Full Backup"},
			Controls:    []PageObservationControl{{Role: "button", Name: "Data Restore"}},
		},
	})
	if err != nil {
		t.Fatalf("GetContext failed: %v", err)
	}
	if len(response.SiteTaskGuides) != 0 {
		t.Fatalf("expected no guide injection for opposite intent, got %#v", response.SiteTaskGuides)
	}
	if response.Debug == nil || !debugContainsFilteredReason(response.Debug, "opposite_intent") {
		t.Fatalf("expected opposite intent debug, got %#v", response.Debug)
	}
}

func TestMemoryContextInjectsOnlyRemainingStepsFromMatchedState(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	if err := repo.SaveSiteTaskGuide(context.Background(), contextTestRestoreGuide()); err != nil {
		t.Fatalf("SaveSiteTaskGuide failed: %v", err)
	}

	response, err := NewMemoryContextService(repo).GetContext(context.Background(), MemoryContextRequest{
		ProjectID:  "default",
		Task:       "restore instance from latest backup",
		CurrentURL: "https://middleware.example.com/instances/pg-prod/backups",
		PageObservation: &PageObservationInput{
			Title:       "Data Backup",
			VisibleText: []string{"Data Backup", "Full Backup", "Data Restore"},
			ActiveTabs:  []string{"Full Backup"},
			Controls:    []PageObservationControl{{Role: "button", Name: "Data Restore"}},
		},
	})
	if err != nil {
		t.Fatalf("GetContext failed: %v", err)
	}
	if len(response.SiteTaskGuides) != 1 {
		t.Fatalf("expected one guide, got %#v", response.SiteTaskGuides)
	}
	guide := response.SiteTaskGuides[0]
	if guide.MatchedStateID != "state_full_backup_tab" || guide.StartStepOffset != 3 {
		t.Fatalf("expected full backup state, got %#v", guide)
	}
	if strings.Contains(strings.Join(guide.Steps, " "), "Filter the instance list") {
		t.Fatalf("expected only remaining steps, got %#v", guide.Steps)
	}
	if !strings.Contains(response.ContextPrompt, "<matched_state>") || !strings.Contains(response.ContextPrompt, "<remaining_steps>") {
		t.Fatalf("expected structured guide prompt: %s", response.ContextPrompt)
	}
}

func contextTestRestoreGuide() registry.SiteTaskGuide {
	return registry.SiteTaskGuide{
		ID:                "guide_restore",
		ProjectID:         "default",
		Site:              "middleware.example.com",
		Module:            "",
		TaskIntentKey:     "restore_instance_latest_full_backup",
		TaskIntentSummary: "restore instance from latest full backup",
		TaskIntentTerms: registry.SiteTaskIntentTerms{
			Positive: []string{"restore", "backup"},
			Negative: []string{"delete", "remove"},
		},
		Summary: "Restore instance from latest full backup.",
		UIStateEntries: []registry.SiteTaskGuideUIStateEntry{{
			ID:         "state_full_backup_tab",
			Name:       "Full Backup tab",
			StateType:  registry.SiteTaskGuideUIStateTab,
			StepOffset: 3,
			RouteScope: registry.SiteTaskGuideRouteScope{URLIncludes: []string{"/instances"}},
			Evidence: registry.SiteTaskGuideUIStateEvidence{
				ActiveTabAny: []string{"Full Backup"},
				ControlsAll:  []registry.ControlSignature{{Role: "button", Name: "Data Restore"}},
			},
			MinimumScore: 4,
			Confidence:   0.8,
			Status:       registry.StatusActive,
		}},
		Steps: []registry.SiteTaskGuideStep{
			{Text: "Filter the instance list.", Target: "Instance name"},
			{Text: "Open the instance detail page.", Target: "Instance name"},
			{Text: "Open the Data Backup page.", Target: "Data Backup"},
			{Text: "Click Data Restore.", Target: "Data Restore"},
			{Text: "Choose the latest backup record.", Target: "Latest backup"},
		},
		AbandonRules: []string{"Abandon if the Full Backup tab or Data Restore button is missing."},
		Confidence:   0.7,
		Status:       registry.StatusActive,
	}
}

func debugContainsFilteredReason(debug *MemoryContextDebug, expected string) bool {
	for _, item := range debug.FilteredEvidence {
		if strings.Contains(item.Reason, expected) {
			return true
		}
	}
	return false
}
