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

func TestPageObservationServiceStoresPageStateForGuideGuards(t *testing.T) {
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
	if first.ObservationID == "" || first.PageStateID != first.ObservationID || first.Matched {
		t.Fatalf("expected new observation page state id, got %#v", first)
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
		t.Fatalf("expected matched existing observation event, got first=%#v second=%#v", first, second)
	}
	pages, err := repo.ListPageStates(ctx, registry.PageStateListQuery{ProjectID: "default", Site: "ops.example.com"})
	if err != nil {
		t.Fatalf("ListPageStates failed: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("expected one page state for guide guards, got %#v", pages)
	}
	page := pages[0]
	if page.ID != first.PageStateID || page.URLPattern != "https://ops.example.com/services" {
		t.Fatalf("unexpected page state: %#v", page)
	}
	if page.CanonicalTitle != "服务管理" || !containsString(page.RequiredText, "服务管理") {
		t.Fatalf("expected page title/text guard, got %#v", page)
	}
	if !hasControl(page.RequiredControls, "textbox", "服务名称") {
		t.Fatalf("expected required control guard, got %#v", page.RequiredControls)
	}
	events, err := repo.ListPageObservationEvents(ctx, registry.PageObservationEventListQuery{ProjectID: "default"})
	if err != nil {
		t.Fatalf("ListPageObservationEvents failed: %v", err)
	}
	if len(events) != 1 || events[0].SeenCount != 2 {
		t.Fatalf("expected one updated observation event, got %#v", events)
	}
}

func TestPageObservationServiceDoesNotBuildTransitionGraph(t *testing.T) {
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
		ProjectID: "default",
		URL:       "https://ops.example.com/services/kme-prod-001",
		Title:     "服务详情",
		Controls:  []PageObservationControl{{Role: "button", Name: "部署记录"}},
	})
	if err != nil {
		t.Fatalf("ObservePage detail failed: %v", err)
	}
	transitions, err := repo.ListPageTransitions(ctx, registry.PageTransitionListQuery{ProjectID: "default", FromPageStateID: list.PageStateID})
	if err != nil {
		t.Fatalf("ListPageTransitions failed: %v", err)
	}
	if len(transitions) != 0 {
		t.Fatalf("page observations must not create a transition graph, detail=%#v transitions=%#v", detail, transitions)
	}
}

func TestPageObservationServiceReportsOverlayHintAndSavesSurface(t *testing.T) {
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
		t.Fatalf("same base URL should update the same lightweight event, base=%#v dialog=%#v", base, withDialog)
	}
	if withDialog.ActiveOverlayHint != "modal:确认删除" {
		t.Fatalf("expected modal overlay hint, got %#v", withDialog)
	}
	pages, err := repo.ListPageStates(ctx, registry.PageStateListQuery{ProjectID: "default", Site: "ops.example.com"})
	if err != nil {
		t.Fatalf("ListPageStates failed: %v", err)
	}
	if len(pages) != 1 || pages[0].ID != base.PageStateID {
		t.Fatalf("expected one persisted base page state: %#v", pages)
	}
	surfaces, err := repo.ListPageSurfaces(ctx, registry.PageSurfaceListQuery{ProjectID: "default", ParentPageStateID: base.PageStateID})
	if err != nil {
		t.Fatalf("ListPageSurfaces failed: %v", err)
	}
	if len(surfaces) != 1 {
		t.Fatalf("expected one persisted page surface: %#v", surfaces)
	}
	surface := surfaces[0]
	if surface.SurfaceType != registry.SurfaceModal || surface.Title != "确认删除" {
		t.Fatalf("unexpected saved surface: %#v", surface)
	}
	if !hasControl(surface.RequiredControls, "dialog", "删除确认") {
		t.Fatalf("expected surface control guard, got %#v", surface.RequiredControls)
	}
}

func hasControl(controls []registry.ControlSignature, role string, name string) bool {
	for _, control := range controls {
		if control.Role == role && control.Name == name {
			return true
		}
	}
	return false
}
