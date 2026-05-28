package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/page-agent/workflow-backend/internal/config"
	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestRepairPatchRoutesCreateApproveRejectAndList(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	router := NewRouterWithServices(config.Config{}, Services{
		Registry:      repo,
		RepairPatches: registry.NewRepairPatchService(repo),
	})

	create := performJSON(router, http.MethodPost, "/api/repair-patches/candidates", map[string]any{
		"patch": sampleHTTPRepairPatch(),
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("expected create 201, got %d: %s", create.Code, create.Body.String())
	}
	var patch registry.RepairPatch
	if err := json.Unmarshal(create.Body.Bytes(), &patch); err != nil {
		t.Fatalf("decode patch failed: %v", err)
	}
	if patch.Status != registry.StatusPendingReview {
		t.Fatalf("expected pending patch, got %#v", patch)
	}

	listPending := performJSON(router, http.MethodGet, "/api/repair-patches?projectId=default&status=pending_review", nil)
	if listPending.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d: %s", listPending.Code, listPending.Body.String())
	}
	var listed struct {
		Patches []registry.RepairPatch `json:"patches"`
	}
	if err := json.Unmarshal(listPending.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list failed: %v", err)
	}
	if len(listed.Patches) != 1 || listed.Patches[0].ID != patch.ID {
		t.Fatalf("expected pending patch in list, got %#v", listed)
	}

	approve := performJSON(router, http.MethodPost, "/api/repair-patches/"+patch.ID+"/approve", nil)
	if approve.Code != http.StatusOK {
		t.Fatalf("expected approve 200, got %d: %s", approve.Code, approve.Body.String())
	}
	active, err := repo.ListActiveRepairPatches(t.Context(), "default")
	if err != nil {
		t.Fatalf("ListActiveRepairPatches failed: %v", err)
	}
	if len(active) != 1 || active[0].ID != patch.ID {
		t.Fatalf("expected approved patch active, got %#v", active)
	}

	reject := performJSON(router, http.MethodPost, "/api/repair-patches/"+patch.ID+"/reject", nil)
	if reject.Code != http.StatusOK {
		t.Fatalf("expected reject 200, got %d: %s", reject.Code, reject.Body.String())
	}
}

func sampleHTTPRepairPatch() map[string]any {
	return map[string]any{
		"projectId":           "default",
		"workflowId":          "wf_github_issue_search",
		"workflowVersion":     3,
		"chunkId":             "search_issues",
		"stepId":              "fill_query",
		"site":                "github.com",
		"failureType":         "locator_not_found",
		"failureSignature":    "old selector was not visible",
		"oldTarget":           "placeholder Search all issues",
		"newTargetSummary":    "new issue query field",
		"appliesToPageStates": []string{"github_issues_list"},
		"riskLevel":           "read_only",
		"newTarget": map[string]any{
			"primary": map[string]any{
				"strategy": "css",
				"value":    "#new-query",
			},
		},
	}
}
