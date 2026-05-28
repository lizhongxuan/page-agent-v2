package selector

import (
	"context"
	"testing"

	"github.com/page-agent/workflow-backend/internal/registry"
	"github.com/page-agent/workflow-backend/internal/retrieval"
)

func TestSelectWorkflowUsesTopKAndBindings(t *testing.T) {
	service := NewService(fakeLLM{
		output: Selection{
			SelectedWorkflowID: "wf_github_issue_search",
			SelectedVersion:    3,
			Confidence:         0.93,
			Bindings:           map[string]string{"repo": "microsoft/playwright", "query": "timeout 报错"},
			Decision:           "replay",
		},
	})

	result, err := service.Select(context.Background(), SelectionInput{
		Task: "在 github.com/microsoft/playwright 的 Issues 里搜索 timeout 报错",
		CandidateSlots: map[string]string{
			"repo":  "microsoft/playwright",
			"query": "timeout 报错",
		},
		Candidates: []CandidateSummary{
			{
				WorkflowID: "wf_github_issue_search",
				Version:    3,
				Variables: []registry.Variable{
					{Name: "repo", Type: registry.VariableString, Required: true, Source: registry.VariableSourceTaskOrURL},
					{Name: "query", Type: registry.VariableString, Required: true, Source: registry.VariableSourceTask},
				},
				Retrieval: retrieval.WorkflowCandidate{WorkflowID: "wf_github_issue_search", Version: 3},
			},
		},
	})

	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}
	if result.Decision != "replay" || result.SelectedWorkflowID != "wf_github_issue_search" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Bindings["repo"].Value != "microsoft/playwright" {
		t.Fatalf("unexpected bindings: %#v", result.Bindings)
	}
}

func TestSelectWorkflowOnlySendsTopThreeCandidatesToLLM(t *testing.T) {
	llm := &capturingLLM{
		output: Selection{
			SelectedWorkflowID: "wf_2",
			SelectedVersion:    1,
			Confidence:         0.93,
			Decision:           "replay",
		},
	}
	service := NewService(llm)
	candidates := make([]CandidateSummary, 0, 5)
	for i := 0; i < 5; i++ {
		workflowID := "wf_" + string(rune('0'+i))
		candidates = append(candidates, CandidateSummary{
			WorkflowID: workflowID,
			Version:    1,
			Retrieval:  retrieval.WorkflowCandidate{WorkflowID: workflowID, Version: 1},
		})
	}

	_, err := service.Select(context.Background(), SelectionInput{Candidates: candidates})
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}
	if len(llm.prompt.Candidates) != 3 {
		t.Fatalf("expected top three candidates, got %#v", llm.prompt.Candidates)
	}
	if llm.prompt.Candidates[2].WorkflowID != "wf_2" {
		t.Fatalf("expected third ranked candidate to be included, got %#v", llm.prompt.Candidates)
	}
}

func TestSelectWorkflowRejectsLowConfidence(t *testing.T) {
	service := NewService(fakeLLM{
		output: Selection{SelectedWorkflowID: "wf", SelectedVersion: 1, Confidence: 0.3, Decision: "replay"},
	})

	_, err := service.Select(context.Background(), SelectionInput{
		Candidates: []CandidateSummary{{WorkflowID: "wf", Version: 1}},
	})

	if err == nil {
		t.Fatal("expected low confidence rejection")
	}
}

type fakeLLM struct {
	output Selection
}

func (llm fakeLLM) SelectWorkflow(context.Context, SelectionPrompt) (Selection, error) {
	return llm.output, nil
}

type capturingLLM struct {
	output Selection
	prompt SelectionPrompt
}

func (llm *capturingLLM) SelectWorkflow(_ context.Context, prompt SelectionPrompt) (Selection, error) {
	llm.prompt = prompt
	return llm.output, nil
}
