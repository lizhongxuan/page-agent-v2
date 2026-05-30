package memory

import (
	"context"
	"strings"
	"testing"

	"github.com/page-agent/workflow-backend/internal/knowledge"
	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestBusinessProfileServiceUpdatesProfileFromDocuments(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewBusinessProfileService(repo)

	profile, updated, err := service.UpdateFromDocuments(ctx, []knowledge.DocumentInput{
		{
			ID:        "doc_service_manual",
			ProjectID: "default",
			Title:     "服务管理手册",
			Source:    "manual",
			URL:       "https://ops.example.com/services",
			Content:   "# 服务管理\n服务管理页用于查询服务运行状态、负责人和最近部署记录。",
			Tags:      []string{"服务管理", "service"},
		},
	})
	if err != nil {
		t.Fatalf("UpdateFromDocuments failed: %v", err)
	}
	if !updated {
		t.Fatal("expected profile to be updated")
	}
	if len([]rune(profile.Summary)) > MaxSummaryChars {
		t.Fatalf("profile summary exceeds limit: %d", len([]rune(profile.Summary)))
	}
	if !strings.Contains(profile.Summary, "服务管理") {
		t.Fatalf("expected service management summary, got %q", profile.Summary)
	}
	if len(profile.Modules) != 1 || profile.Modules[0].Name != "服务管理" {
		t.Fatalf("unexpected modules: %#v", profile.Modules)
	}
	if len(profile.EntryPages) != 1 || profile.EntryPages[0].URLPattern != "https://ops.example.com/services" {
		t.Fatalf("unexpected entry pages: %#v", profile.EntryPages)
	}

	got, err := repo.GetBusinessSystemProfile(ctx, registry.BusinessSystemProfileQuery{
		ProjectID:  "default",
		Site:       "ops.example.com",
		Module:     "服务管理",
		SourceType: registry.MemorySourceProduction,
	})
	if err != nil {
		t.Fatalf("GetBusinessSystemProfile failed: %v", err)
	}
	if got.Summary != profile.Summary {
		t.Fatalf("persisted profile mismatch: %#v", got)
	}
}

func TestBusinessProfileServiceRejectsSensitiveProfileText(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewBusinessProfileService(repo)

	_, _, err = service.UpdateFromDocuments(ctx, []knowledge.DocumentInput{
		{
			ID:        "doc_secret",
			ProjectID: "default",
			Title:     "token=secret-value",
			Content:   "服务管理说明。",
		},
	})
	if err == nil {
		t.Fatal("expected sensitive profile text to fail")
	}
}
