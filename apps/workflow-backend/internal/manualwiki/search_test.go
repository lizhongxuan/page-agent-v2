package manualwiki

import (
	"context"
	"strings"
	"testing"

	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestSearchPreviewFiltersCrossSiteDisabledAndRanksTopThree(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	chunks := []registry.SiteManualWikiChunk{
		{
			ID:          "chunk_restore",
			WikiPageID:  "page_restore",
			ProjectID:   "default",
			Site:        "ops.example.com",
			Module:      "backup",
			Text:        "Full Backup restore starts from the Restore button.",
			TargetTerms: []string{"full", "backup", "restore"},
			Status:      registry.StatusActive,
		},
		{
			ID:          "chunk_warning",
			WikiPageID:  "page_restore",
			ProjectID:   "default",
			Site:        "ops.example.com",
			Module:      "backup",
			Text:        "Restore overwrites current data after confirmation.",
			TargetTerms: []string{"restore", "confirmation"},
			Status:      registry.StatusActive,
		},
		{
			ID:          "chunk_node",
			WikiPageID:  "page_restore",
			ProjectID:   "default",
			Site:        "ops.example.com",
			Module:      "backup",
			Text:        "Select the target node before starting restore.",
			TargetTerms: []string{"node", "restore"},
			Status:      registry.StatusActive,
		},
		{
			ID:          "chunk_extra",
			WikiPageID:  "page_restore",
			ProjectID:   "default",
			Site:        "ops.example.com",
			Module:      "backup",
			Text:        "Incremental backup has a separate detail page.",
			TargetTerms: []string{"backup"},
			Status:      registry.StatusActive,
		},
		{
			ID:          "chunk_other_site",
			WikiPageID:  "page_other",
			ProjectID:   "default",
			Site:        "other.example.com",
			Text:        "Restore from another site.",
			TargetTerms: []string{"restore"},
			Status:      registry.StatusActive,
		},
		{
			ID:          "chunk_disabled",
			WikiPageID:  "page_disabled",
			ProjectID:   "default",
			Site:        "ops.example.com",
			Module:      "backup",
			Text:        "Disabled restore note.",
			TargetTerms: []string{"restore"},
			Status:      registry.StatusDisabled,
		},
	}
	if err := repo.SaveSiteManualWiki(ctx, nil, chunks); err != nil {
		t.Fatalf("SaveSiteManualWiki failed: %v", err)
	}

	preview, err := NewService(repo).PreviewContext(ctx, PreviewContextRequest{
		ProjectID:         "default",
		Site:              "ops.example.com",
		Module:            "backup",
		Task:              "restore latest backup",
		Title:             "Backup",
		VisibleTextSample: "Full Backup restore confirmation node",
	})
	if err != nil {
		t.Fatalf("PreviewContext failed: %v", err)
	}
	if len(preview.Matches) != 3 {
		t.Fatalf("expected top 3 matches, got %#v", preview.Matches)
	}
	for _, match := range preview.Matches {
		if match.Chunk.Site != "ops.example.com" || match.Chunk.Status != registry.StatusActive {
			t.Fatalf("unexpected match passed gates: %#v", match)
		}
	}
	if preview.Prompt == "" || !strings.Contains(preview.Prompt, "<site_manual_knowledge>") {
		t.Fatalf("expected prompt with manual XML, got %q", preview.Prompt)
	}
	if len(preview.Filtered) == 0 {
		t.Fatalf("expected filtered reasons for rejected chunks")
	}
}

func TestSearchPageGuardRejectsMissingText(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	if err := repo.SaveSiteManualWiki(ctx, nil, []registry.SiteManualWikiChunk{
		{
			ID:         "chunk_restore",
			WikiPageID: "page_restore",
			ProjectID:  "default",
			Site:       "ops.example.com",
			Text:       "Restore from full backup.",
			PageGuards: registry.HardRules{TextAll: []string{"Full Backup"}},
			Status:     registry.StatusActive,
		},
	}); err != nil {
		t.Fatalf("SaveSiteManualWiki failed: %v", err)
	}

	preview, err := NewService(repo).PreviewContext(ctx, PreviewContextRequest{
		ProjectID:         "default",
		Site:              "ops.example.com",
		Task:              "restore backup",
		VisibleTextSample: "Overview",
	})
	if err != nil {
		t.Fatalf("PreviewContext failed: %v", err)
	}
	if len(preview.Matches) != 0 {
		t.Fatalf("expected page guard to reject chunk, got %#v", preview.Matches)
	}
}
