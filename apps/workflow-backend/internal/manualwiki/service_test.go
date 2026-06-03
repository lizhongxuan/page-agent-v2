package manualwiki

import (
	"context"
	"strings"
	"testing"

	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestImportManualRequiresSite(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewService(repo)

	_, err = service.ImportManual(context.Background(), ImportManualRequest{
		ProjectID:  "default",
		Title:      "Manual",
		SourceType: registry.SiteManualSourceMarkdown,
		Content:    "Manual content",
	})
	if err == nil {
		t.Fatal("expected missing site to fail")
	}
}

func TestImportManualDedupesByProjectSiteModuleHash(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewService(repo)
	request := ImportManualRequest{
		ProjectID:  "default",
		Site:       "ops.example.com",
		Module:     "backup",
		Title:      "Backup Manual",
		SourceType: registry.SiteManualSourceMarkdown,
		Content:    "# Backup Restore\nUse the Full Backup tab and click Restore.",
	}

	first, err := service.ImportManual(ctx, request)
	if err != nil {
		t.Fatalf("first import failed: %v", err)
	}
	second, err := service.ImportManual(ctx, request)
	if err != nil {
		t.Fatalf("second import failed: %v", err)
	}
	if first.Source.ID != second.Source.ID {
		t.Fatalf("expected duplicate import to return existing source, got %q and %q", first.Source.ID, second.Source.ID)
	}
	sources, err := repo.ListSiteManualSources(ctx, registry.SiteManualSourceListQuery{ProjectID: "default", Site: "ops.example.com"})
	if err != nil {
		t.Fatalf("ListSiteManualSources failed: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected one source after duplicate import, got %#v", sources)
	}
}

func TestImportManualCompilesWiki(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewService(repo)

	result, err := service.ImportManual(ctx, ImportManualRequest{
		ProjectID:  "default",
		Site:       "ops.example.com",
		Module:     "backup",
		Title:      "Backup Restore Manual",
		SourceType: registry.SiteManualSourceMarkdown,
		Content: strings.Join([]string{
			"# Backup Restore",
			"Backup page contains Full Backup and Incremental Backup tabs.",
			"- Click Restore from a full backup record.",
			"- Select the target node IP before confirmation.",
			"Warning: restore overwrites current data.",
		}, "\n"),
	})
	if err != nil {
		t.Fatalf("ImportManual failed: %v", err)
	}
	if len(result.Pages) == 0 || len(result.Chunks) == 0 {
		t.Fatalf("expected compiled wiki, got %#v", result)
	}
	if len([]rune(result.Pages[0].Summary)) > registry.MaxSummaryChars {
		t.Fatalf("summary too long: %q", result.Pages[0].Summary)
	}
}
