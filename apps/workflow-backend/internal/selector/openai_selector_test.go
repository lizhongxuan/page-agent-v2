package selector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/page-agent/workflow-backend/internal/retrieval"
)

func TestOpenAICompatibleSelectorCallsChatCompletionsAndParsesJSON(t *testing.T) {
	var auth string
	var requestModel string
	var userContent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request failed: %v", err)
		}
		requestModel = body.Model
		userContent = body.Messages[len(body.Messages)-1].Content
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]string{
						"content": `{"selectedWorkflowId":"wf_github_issue_search","selectedVersion":3,"confidence":0.91,"bindings":{"query":"timeout"},"decision":"replay"}`,
					},
				},
			},
		})
	}))
	defer server.Close()
	client := NewOpenAICompatibleSelector(server.URL, "secret", "gpt-5.4", server.Client())

	selection, err := client.SelectWorkflow(context.Background(), SelectionPrompt{
		Task: "search timeout",
		Candidates: []CandidateSummary{{
			WorkflowID: "wf_github_issue_search",
			Version:    3,
		}},
	})

	if err != nil {
		t.Fatalf("SelectWorkflow failed: %v", err)
	}
	if auth != "Bearer secret" {
		t.Fatalf("expected bearer auth, got %q", auth)
	}
	if requestModel != "gpt-5.4" || !strings.Contains(userContent, "wf_github_issue_search") {
		t.Fatalf("unexpected request: model=%q content=%q", requestModel, userContent)
	}
	if selection.SelectedWorkflowID != "wf_github_issue_search" || selection.Bindings["query"] != "timeout" {
		t.Fatalf("unexpected selection: %#v", selection)
	}
}

func TestHeuristicSelectorPicksFirstCandidate(t *testing.T) {
	selection, err := (HeuristicSelector{}).SelectWorkflow(context.Background(), SelectionPrompt{
		Candidates: []CandidateSummary{
			{
				WorkflowID: "first",
				Version:    1,
				Retrieval: retrieval.WorkflowCandidate{
					WorkflowID: "first",
					Version:    1,
					FinalScore: 0.86,
					ScoreBreakdown: retrieval.ScoreBreakdown{
						WorkflowDenseScore: 0.8,
					},
				},
			},
			{WorkflowID: "second", Version: 1},
		},
	})
	if err != nil {
		t.Fatalf("SelectWorkflow failed: %v", err)
	}
	if selection.SelectedWorkflowID != "first" || selection.Decision != "replay" {
		t.Fatalf("unexpected selection: %#v", selection)
	}
}

func TestHeuristicSelectorRejectsWeakCandidate(t *testing.T) {
	selection, err := (HeuristicSelector{}).SelectWorkflow(context.Background(), SelectionPrompt{
		Candidates: []CandidateSummary{{
			WorkflowID: "wf_site_only",
			Version:    1,
			Retrieval: retrieval.WorkflowCandidate{
				WorkflowID: "wf_site_only",
				Version:    1,
				FinalScore: 0.16,
				ScoreBreakdown: retrieval.ScoreBreakdown{
					VariableBindability:   1,
					HistoricalSuccessRate: 1,
					SelectorHealth:        1,
				},
			},
		}},
	})
	if err != nil {
		t.Fatalf("SelectWorkflow failed: %v", err)
	}
	if selection.Decision != "no_match" {
		t.Fatalf("expected weak candidate rejection, got %#v", selection)
	}
}
