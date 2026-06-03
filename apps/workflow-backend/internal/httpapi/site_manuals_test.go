package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/page-agent/workflow-backend/internal/config"
	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestSiteManualImportRequiresSite(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	handler := NewRouterWithServices(config.Config{}, Services{Registry: repo})

	response := performJSON(handler, http.MethodPost, "/api/memory/site-manuals/import", map[string]any{
		"projectId":  "default",
		"title":      "Manual",
		"sourceType": "markdown",
		"content":    "content",
	})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", response.Code, response.Body.String())
	}
}

func TestSiteManualImportListGetWikiAndPreview(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	handler := NewRouterWithServices(config.Config{}, Services{Registry: repo})

	importResponse := performJSON(handler, http.MethodPost, "/api/memory/site-manuals/import", map[string]any{
		"projectId":  "default",
		"site":       "ops.example.com",
		"module":     "backup",
		"title":      "Backup Restore Manual",
		"sourceType": "markdown",
		"content":    "# Backup Restore\nUse Full Backup and click Restore.",
	})
	if importResponse.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", importResponse.Code, importResponse.Body.String())
	}
	var imported struct {
		Source registry.SiteManualSource `json:"source"`
	}
	if err := json.Unmarshal(importResponse.Body.Bytes(), &imported); err != nil {
		t.Fatalf("decode import failed: %v", err)
	}
	if imported.Source.ID == "" {
		t.Fatalf("expected source id: %#v", imported)
	}

	listResponse := performJSON(handler, http.MethodGet, "/api/memory/site-manuals?projectId=default&site=ops.example.com", nil)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d: %s", listResponse.Code, listResponse.Body.String())
	}
	var listed struct {
		Sources []registry.SiteManualSource `json:"sources"`
	}
	if err := json.Unmarshal(listResponse.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list failed: %v", err)
	}
	if len(listed.Sources) != 1 || listed.Sources[0].ID != imported.Source.ID {
		t.Fatalf("unexpected sources: %#v", listed.Sources)
	}

	wikiResponse := performJSON(handler, http.MethodGet, "/api/memory/site-manuals/"+imported.Source.ID+"/wiki", nil)
	if wikiResponse.Code != http.StatusOK {
		t.Fatalf("expected wiki 200, got %d: %s", wikiResponse.Code, wikiResponse.Body.String())
	}

	previewResponse := performJSON(handler, http.MethodPost, "/api/memory/site-manuals/preview-context", map[string]any{
		"projectId":         "default",
		"site":              "ops.example.com",
		"module":            "backup",
		"task":              "restore backup",
		"title":             "Backup Restore",
		"visibleTextSample": "Full Backup Restore",
	})
	if previewResponse.Code != http.StatusOK {
		t.Fatalf("expected preview 200, got %d: %s", previewResponse.Code, previewResponse.Body.String())
	}
	var preview registry.SiteManualPreviewContext
	if err := json.Unmarshal(previewResponse.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview failed: %v", err)
	}
	if len(preview.Matches) == 0 || preview.Prompt == "" {
		t.Fatalf("expected preview matches and prompt, got %#v", preview)
	}
}

func TestSiteManualDisableAndDeleteAffectPreviewAndList(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	handler := NewRouterWithServices(config.Config{}, Services{Registry: repo})

	importResponse := performJSON(handler, http.MethodPost, "/api/memory/site-manuals/import", map[string]any{
		"projectId":  "default",
		"site":       "ops.example.com",
		"title":      "Manual",
		"sourceType": "markdown",
		"content":    "Restore backup from the Backup page.",
	})
	var imported struct {
		Source registry.SiteManualSource `json:"source"`
	}
	if err := json.Unmarshal(importResponse.Body.Bytes(), &imported); err != nil {
		t.Fatalf("decode import failed: %v", err)
	}

	disableResponse := performJSON(handler, http.MethodPost, "/api/memory/site-manuals/"+imported.Source.ID+"/disable", nil)
	if disableResponse.Code != http.StatusOK {
		t.Fatalf("expected disable 200, got %d: %s", disableResponse.Code, disableResponse.Body.String())
	}
	previewResponse := performJSON(handler, http.MethodPost, "/api/memory/site-manuals/preview-context", map[string]any{
		"projectId": "default",
		"site":      "ops.example.com",
		"task":      "restore backup",
	})
	var preview registry.SiteManualPreviewContext
	if err := json.Unmarshal(previewResponse.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview failed: %v", err)
	}
	if len(preview.Matches) != 0 {
		t.Fatalf("expected disabled source to suppress matches, got %#v", preview.Matches)
	}

	deleteResponse := performJSON(handler, http.MethodDelete, "/api/memory/site-manuals/"+imported.Source.ID, nil)
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("expected delete 204, got %d: %s", deleteResponse.Code, deleteResponse.Body.String())
	}
	listResponse := performJSON(handler, http.MethodGet, "/api/memory/site-manuals?projectId=default&site=ops.example.com", nil)
	var listed struct {
		Sources []registry.SiteManualSource `json:"sources"`
	}
	if err := json.Unmarshal(listResponse.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list failed: %v", err)
	}
	if len(listed.Sources) != 0 {
		t.Fatalf("expected deleted source not to list, got %#v", listed.Sources)
	}
}
