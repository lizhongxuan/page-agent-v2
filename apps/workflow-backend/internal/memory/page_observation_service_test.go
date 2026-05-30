package memory

import (
	"context"
	"strings"
	"testing"

	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestNormalizePageObservationBuildsStableURLPatternAndSafeSummary(t *testing.T) {
	normalized, err := NormalizePageObservation(PageObservationRequest{
		ProjectID:   "default",
		URL:         "https://ops.example.com/services?k=kme-prod-001",
		Title:       "服务管理",
		VisibleText: []string{strings.Repeat("服务管理页面包含服务名称状态负责人。", 80)},
		Controls: []PageObservationControl{
			{Role: "textbox", Name: "服务名称"},
			{Role: "button", Name: "搜索"},
		},
	})
	if err != nil {
		t.Fatalf("NormalizePageObservation failed: %v", err)
	}
	if normalized.URLPattern != "https://ops.example.com/services" {
		t.Fatalf("unexpected url pattern: %q", normalized.URLPattern)
	}
	if strings.Contains(normalized.SearchableText, "kme-prod-001") {
		t.Fatalf("searchable text contains instance value: %s", normalized.SearchableText)
	}
	if len([]rune(normalized.VisibleTextSample)) > MaxSummaryChars {
		t.Fatalf("visible text summary exceeds limit: %d", len([]rune(normalized.VisibleTextSample)))
	}
}

func TestNormalizePageObservationRejectsSensitiveControls(t *testing.T) {
	_, err := NormalizePageObservation(PageObservationRequest{
		ProjectID: "default",
		URL:       "https://ops.example.com/login",
		Title:     "Login",
		Controls: []PageObservationControl{
			{Role: "textbox", Name: "password token"},
		},
	})
	if err == nil {
		t.Fatal("expected sensitive control to fail")
	}
}

func TestBuildPageFingerprintMatchesSameStablePage(t *testing.T) {
	first, err := NormalizePageObservation(PageObservationRequest{
		ProjectID: "default",
		URL:       "https://ops.example.com/services?k=kme-prod-001",
		Title:     "服务管理",
		Controls:  []PageObservationControl{{Role: "textbox", Name: "服务名称"}},
	})
	if err != nil {
		t.Fatalf("NormalizePageObservation first failed: %v", err)
	}
	second, err := NormalizePageObservation(PageObservationRequest{
		ProjectID: "default",
		URL:       "https://ops.example.com/services?k=abc-prod-999",
		Title:     "服务管理",
		Controls:  []PageObservationControl{{Role: "textbox", Name: "服务名称"}},
	})
	if err != nil {
		t.Fatalf("NormalizePageObservation second failed: %v", err)
	}
	if first.Fingerprint != second.Fingerprint {
		t.Fatalf("expected same fingerprint, got %q and %q", first.Fingerprint, second.Fingerprint)
	}
}

func TestPageObservationServiceCreatesAndUpdatesPageState(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewPageObservationService(repo)

	first, err := service.ObservePage(ctx, PageObservationRequest{
		ProjectID:   "default",
		Task:        "查询服务状态",
		URL:         "https://ops.example.com/services?k=kme-prod-001",
		Title:       "服务管理",
		VisibleText: []string{"服务管理", "服务名称", "状态", "负责人"},
		Controls: []PageObservationControl{
			{Role: "textbox", Name: "服务名称"},
			{Role: "button", Name: "搜索"},
		},
	})
	if err != nil {
		t.Fatalf("ObservePage first failed: %v", err)
	}
	if first.PageStateID == "" || first.Matched {
		t.Fatalf("expected new page state, got %#v", first)
	}

	second, err := service.ObservePage(ctx, PageObservationRequest{
		ProjectID:   "default",
		Task:        "查询服务状态",
		URL:         "https://ops.example.com/services?k=abc-prod-999",
		Title:       "服务管理",
		VisibleText: []string{"服务管理", "服务名称", "状态", "负责人"},
		Controls:    []PageObservationControl{{Role: "textbox", Name: "服务名称"}},
	})
	if err != nil {
		t.Fatalf("ObservePage second failed: %v", err)
	}
	if !second.Matched || second.PageStateID != first.PageStateID {
		t.Fatalf("expected matched existing state, got first=%#v second=%#v", first, second)
	}
	pages, err := repo.ListPageStates(ctx, registry.PageStateListQuery{ProjectID: "default", Site: "ops.example.com"})
	if err != nil {
		t.Fatalf("ListPageStates failed: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("expected one page state, got %#v", pages)
	}
	events, err := repo.ListPageObservationEvents(ctx, registry.PageObservationEventListQuery{ProjectID: "default"})
	if err != nil {
		t.Fatalf("ListPageObservationEvents failed: %v", err)
	}
	if len(events) != 1 || events[0].SeenCount != 2 {
		t.Fatalf("expected one updated observation event, got %#v", events)
	}
}

func TestPageObservationServiceUpdatesTransitions(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewPageObservationService(repo)
	list, err := service.ObservePage(ctx, PageObservationRequest{
		ProjectID: "default",
		URL:       "https://ops.example.com/services",
		Title:     "服务管理",
		Controls:  []PageObservationControl{{Role: "textbox", Name: "服务名称"}},
	})
	if err != nil {
		t.Fatalf("ObservePage list failed: %v", err)
	}
	detail, err := service.ObservePage(ctx, PageObservationRequest{
		ProjectID:           "default",
		URL:                 "https://ops.example.com/services/kme-prod-001",
		Title:               "服务详情",
		Controls:            []PageObservationControl{{Role: "button", Name: "部署记录"}},
		PreviousPageStateID: list.PageStateID,
		TransitionAction:    "Open service detail",
		TransitionTarget:    "Service name link",
	})
	if err != nil {
		t.Fatalf("ObservePage detail failed: %v", err)
	}
	if detail.PageStateID == list.PageStateID {
		t.Fatalf("expected distinct detail page: %#v", detail)
	}
	transitions, err := repo.ListPageTransitions(ctx, registry.PageTransitionListQuery{ProjectID: "default", FromPageStateID: list.PageStateID})
	if err != nil {
		t.Fatalf("ListPageTransitions failed: %v", err)
	}
	if len(transitions) != 1 || transitions[0].ToPageState != detail.PageStateID {
		t.Fatalf("unexpected transitions: %#v", transitions)
	}
}

func TestPageObservationServiceSeparatesSurfaceFromBasePageState(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewPageObservationService(repo)

	base, err := service.ObservePage(ctx, PageObservationRequest{
		ProjectID:   "default",
		URL:         "https://ops.example.com/services",
		Title:       "服务管理",
		VisibleText: []string{"服务管理", "服务名称", "状态"},
		Controls: []PageObservationControl{
			{Role: "textbox", Name: "服务名称"},
			{Role: "button", Name: "搜索"},
		},
	})
	if err != nil {
		t.Fatalf("ObservePage base failed: %v", err)
	}

	withDialog, err := service.ObservePage(ctx, PageObservationRequest{
		ProjectID:   "default",
		URL:         "https://ops.example.com/services",
		Title:       "服务管理",
		VisibleText: []string{"服务管理", "服务名称", "删除确认", "确认删除"},
		Controls: []PageObservationControl{
			{Role: "textbox", Name: "服务名称"},
			{Role: "button", Name: "搜索"},
			{Role: "dialog", Name: "删除确认"},
			{Role: "button", Name: "确认删除"},
		},
	})
	if err != nil {
		t.Fatalf("ObservePage with dialog failed: %v", err)
	}
	if withDialog.PageStateID != base.PageStateID {
		t.Fatalf("dialog observation should keep the base page state, base=%#v dialog=%#v", base, withDialog)
	}
	if withDialog.SurfaceID == "" || withDialog.SurfaceType != registry.SurfaceModal {
		t.Fatalf("expected modal surface to be recorded, got %#v", withDialog)
	}
	pages, err := repo.ListPageStates(ctx, registry.PageStateListQuery{ProjectID: "default", Site: "ops.example.com"})
	if err != nil {
		t.Fatalf("ListPageStates failed: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("expected one base page state, got %#v", pages)
	}
	for _, control := range pages[0].RequiredControls {
		if control.Name == "确认删除" || control.Role == "dialog" {
			t.Fatalf("base page state should not be polluted by surface controls: %#v", pages[0])
		}
	}
	surfaces, err := repo.ListPageSurfaces(ctx, registry.PageSurfaceListQuery{ProjectID: "default", ParentPageStateID: base.PageStateID})
	if err != nil {
		t.Fatalf("ListPageSurfaces failed: %v", err)
	}
	if len(surfaces) != 1 || surfaces[0].ParentPageStateID != base.PageStateID {
		t.Fatalf("expected one surface bound to the base page, got %#v", surfaces)
	}
	if !controlListContains(surfaces[0].RequiredControls, registry.ControlSignature{Role: "button", Name: "确认删除"}) {
		t.Fatalf("surface should keep dialog controls, got %#v", surfaces[0])
	}
}

func controlListContains(values []registry.ControlSignature, expected registry.ControlSignature) bool {
	for _, value := range values {
		if value.Role == expected.Role && value.Name == expected.Name {
			return true
		}
	}
	return false
}
