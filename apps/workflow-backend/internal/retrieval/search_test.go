package retrieval

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/page-agent/workflow-backend/internal/qdrant"
	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestSearchServiceQueriesQdrantAndReranks(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	workflow := sampleSearchWorkflow("wf_github_issue_search", registry.RiskReadOrSearch)
	if err := repo.SaveWorkflow(t.Context(), workflow); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}

	var mu sync.Mutex
	var paths []string
	var cardFilter qdrant.Filter
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		var body qdrant.SearchRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode qdrant request failed: %v", err)
		}
		if strings.Contains(r.URL.Path, "workflow_cards") && body.Filter != nil {
			cardFilter = *body.Filter
			writeQdrantSearchResult(w, []map[string]any{
				{
					"id":    "workflow:wf_github_issue_search:v3",
					"score": 0.91,
					"payload": map[string]any{
						"workflow_id":    "wf_github_issue_search",
						"version":        float64(3),
						"status":         "active",
						"searchable":     true,
						"site":           "github.com",
						"risk_level":     "read_or_search",
						"variable_names": []any{"repo", "query"},
						"success_rate":   0.94,
					},
				},
			})
			return
		}
		writeQdrantSearchResult(w, []map[string]any{
			{
				"id":    "chunk:wf_github_issue_search:v3:search_issues",
				"score": 0.82,
				"payload": map[string]any{
					"workflow_id":     "wf_github_issue_search",
					"version":         float64(3),
					"selector_health": 0.88,
				},
			},
		})
	}))
	defer server.Close()

	service := NewSearchService(SearchServiceConfig{
		Repository: repo,
		Qdrant:     qdrant.NewClient(qdrant.ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()}),
		Collections: qdrant.CollectionNames{
			WorkflowCards:  "pa_workflow_cards",
			WorkflowChunks: "pa_workflow_chunks",
		},
		Vectorizer: fakeVectorizer{},
	})

	results, err := service.Search(context.Background(), SearchRequest{
		ProjectID:  "default",
		Task:       "在 github.com/microsoft/playwright 的 Issues 里搜索 timeout 报错",
		CurrentURL: "https://github.com/microsoft/playwright",
		RiskPolicy: defaultRiskPolicy(),
		Limit:      8,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 1 || results[0].WorkflowID != "wf_github_issue_search" {
		t.Fatalf("expected workflow result, got %#v", results)
	}
	if results[0].Name != "Search GitHub issues" || results[0].Intent == "" {
		t.Fatalf("expected registry-enriched workflow summary, got %#v", results[0])
	}
	if results[0].ScoreBreakdown.BestChunkScore != 0.82 {
		t.Fatalf("expected chunk boost in score breakdown, got %#v", results[0])
	}
	if len(paths) != 2 {
		t.Fatalf("expected card and chunk qdrant searches, got %#v", paths)
	}
	if !filterHasMatch(cardFilter, "project_id", "default") || !filterHasMatch(cardFilter, "site", "github.com") {
		t.Fatalf("expected hard filter on project and site, got %#v", cardFilter)
	}
}

func TestSearchServiceRejectsDestructiveWorkflow(t *testing.T) {
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	workflow := sampleSearchWorkflow("wf_delete_repo", registry.RiskDestructive)
	workflow.RequiresConfirmation = true
	if err := repo.SaveWorkflow(t.Context(), workflow); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeQdrantSearchResult(w, []map[string]any{
			{
				"id":    "workflow:wf_delete_repo:v3",
				"score": 0.99,
				"payload": map[string]any{
					"workflow_id":    "wf_delete_repo",
					"version":        float64(3),
					"status":         "active",
					"searchable":     true,
					"site":           "github.com",
					"risk_level":     "destructive",
					"variable_names": []any{"repo", "query"},
				},
			},
		})
	}))
	defer server.Close()
	service := NewSearchService(SearchServiceConfig{
		Repository:  repo,
		Qdrant:      qdrant.NewClient(qdrant.ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()}),
		Collections: qdrant.CollectionNames{WorkflowCards: "pa_workflow_cards", WorkflowChunks: "pa_workflow_chunks"},
		Vectorizer:  fakeVectorizer{},
	})

	results, err := service.Search(context.Background(), SearchRequest{
		ProjectID:  "default",
		Task:       "delete repo",
		CurrentURL: "https://github.com/microsoft/playwright",
		RiskPolicy: defaultRiskPolicy(),
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected destructive workflow to be rejected, got %#v", results)
	}
}

func TestSearchServiceQueriesQdrantForInterruptsAndRepairs(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if strings.Contains(r.URL.Path, "interrupt_handlers") {
			writeQdrantSearchResult(w, []map[string]any{
				{
					"id":    "interrupt:ih_dialog",
					"score": 0.93,
					"payload": map[string]any{
						"handler_id":             "ih_dialog",
						"workflow_id":            "wf_close_dialog",
						"version":                float64(1),
						"site":                   "github.com",
						"risk_level":             "read_only",
						"applies_to_page_states": []any{"github_issues_list"},
						"required_text":          []any{"New feature"},
						"target_names":           []any{"Got it"},
						"target_roles":           []any{"button"},
					},
				},
			})
			return
		}
		writeQdrantSearchResult(w, []map[string]any{
			{
				"id":    "repair:patch_search",
				"score": 0.91,
				"payload": map[string]any{
					"patch_id":               "patch_search",
					"workflow_id":            "wf_github_issue_search",
					"workflow_version":       float64(3),
					"chunk_id":               "search_issues",
					"step_id":                "fill_query",
					"site":                   "github.com",
					"risk_level":             "read_only",
					"failure_type":           "locator_not_found",
					"applies_to_page_states": []any{"github_issues_list"},
				},
			},
		})
	}))
	defer server.Close()
	service := NewSearchService(SearchServiceConfig{
		Qdrant: qdrant.NewClient(qdrant.ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()}),
		Collections: qdrant.CollectionNames{
			InterruptHandlers: "pa_interrupt_handlers",
			RepairPatches:     "pa_repair_patches",
		},
		Vectorizer: fakeVectorizer{},
	})

	interrupts, err := service.SearchInterrupts(InterruptRequest{
		ProjectID:        "default",
		Site:             "github.com",
		CurrentPageState: "github_issues_list",
		Observation: PageObservation{
			VisibleText: []string{"New feature"},
			Controls:    []Control{{Role: "button", Name: "Got it"}},
		},
		RiskPolicy: defaultRiskPolicy(),
	})
	if err != nil {
		t.Fatalf("SearchInterrupts failed: %v", err)
	}
	repairs, err := service.SearchRepairs(RepairRequest{
		ProjectID:        "default",
		WorkflowID:       "wf_github_issue_search",
		WorkflowVersion:  3,
		ChunkID:          "search_issues",
		StepID:           "fill_query",
		Site:             "github.com",
		FailureType:      "locator_not_found",
		CurrentPageState: "github_issues_list",
		RiskPolicy:       defaultRiskPolicy(),
	})
	if err != nil {
		t.Fatalf("SearchRepairs failed: %v", err)
	}
	if len(interrupts) != 1 || interrupts[0].HandlerID != "ih_dialog" {
		t.Fatalf("unexpected interrupts: %#v", interrupts)
	}
	if len(repairs) != 1 || repairs[0].PatchID != "patch_search" {
		t.Fatalf("unexpected repairs: %#v", repairs)
	}
	if len(paths) != 2 {
		t.Fatalf("expected two qdrant searches, got %#v", paths)
	}
}

type fakeVectorizer struct{}

func (fakeVectorizer) DenseQuery(context.Context, string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}

func (fakeVectorizer) SparseQuery(context.Context, string) (map[string]any, error) {
	return map[string]any{"indices": []uint32{1, 2}, "values": []float32{1, 0.5}}, nil
}

func writeQdrantSearchResult(w http.ResponseWriter, result []map[string]any) {
	_ = json.NewEncoder(w).Encode(map[string]any{"result": result})
}

func filterHasMatch(filter qdrant.Filter, key string, value string) bool {
	for _, condition := range filter.Must {
		if condition.Key != key {
			continue
		}
		if condition.Match["value"] == value {
			return true
		}
	}
	return false
}

func sampleSearchWorkflow(id string, risk registry.RiskLevel) registry.WorkflowRecipe {
	return registry.WorkflowRecipe{
		ID:         id,
		Version:    3,
		ProjectID:  "default",
		Status:     registry.StatusActive,
		Searchable: true,
		Site:       "github.com",
		App:        "github",
		Name:       "Search GitHub issues",
		Intent:     "Search issues in a GitHub repository",
		RiskLevel:  risk,
		Variables: []registry.Variable{
			{Name: "repo", Type: registry.VariableString, Required: true, Source: registry.VariableSourceTaskOrURL},
			{Name: "query", Type: registry.VariableString, Required: true, Source: registry.VariableSourceTask},
		},
		Chunks: []registry.WorkflowChunk{
			{ID: "search_issues", Name: "Search Issues", RiskLevel: risk, Steps: []registry.WorkflowStep{{ID: "fill", Type: registry.StepFill}}},
		},
	}
}
