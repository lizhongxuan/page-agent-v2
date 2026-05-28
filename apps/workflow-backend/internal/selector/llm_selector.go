package selector

import (
	"context"
	"errors"

	"github.com/page-agent/workflow-backend/internal/registry"
	"github.com/page-agent/workflow-backend/internal/retrieval"
)

type LLMSelector interface {
	SelectWorkflow(context.Context, SelectionPrompt) (Selection, error)
}

type Service struct {
	llm                 LLMSelector
	confidenceThreshold float64
	autoReplayThreshold float64
}

func NewService(llm LLMSelector) *Service {
	return &Service{llm: llm, confidenceThreshold: 0.7, autoReplayThreshold: 0.85}
}

type CandidateSummary struct {
	WorkflowID string
	Version    int
	Variables  []registry.Variable
	Retrieval  retrieval.WorkflowCandidate
}

type SelectionInput struct {
	Task             string
	CurrentPageState string
	CandidateSlots   map[string]string
	Candidates       []CandidateSummary
}

type SelectionPrompt struct {
	Task             string
	CurrentPageState string
	Candidates       []CandidateSummary
}

type Selection struct {
	SelectedWorkflowID    string            `json:"selectedWorkflowId"`
	SelectedVersion       int               `json:"selectedVersion"`
	Confidence            float64           `json:"confidence"`
	Bindings              map[string]string `json:"bindings"`
	NeedsUserConfirmation bool              `json:"needsUserConfirmation"`
	RejectReason          string            `json:"rejectReason"`
	Decision              string            `json:"decision"`
}

type SelectionResult struct {
	SelectedWorkflowID    string             `json:"selectedWorkflowId,omitempty"`
	SelectedVersion       int                `json:"selectedVersion,omitempty"`
	Confidence            float64            `json:"confidence,omitempty"`
	Bindings              map[string]Binding `json:"bindings,omitempty"`
	NeedsUserConfirmation bool               `json:"needsUserConfirmation,omitempty"`
	RejectReason          string             `json:"rejectReason,omitempty"`
	Decision              string             `json:"decision"`
}

func (service *Service) Select(ctx context.Context, input SelectionInput) (SelectionResult, error) {
	if len(input.Candidates) == 0 {
		return SelectionResult{Decision: "no_match", RejectReason: "no candidates"}, nil
	}
	selection, err := service.llm.SelectWorkflow(ctx, SelectionPrompt{
		Task:             input.Task,
		CurrentPageState: input.CurrentPageState,
		Candidates:       topK(input.Candidates, 3),
	})
	if err != nil {
		return SelectionResult{}, err
	}
	decision := selection.Decision
	if decision == "" {
		decision = "replay"
	}
	if decision != "replay" {
		return SelectionResult{
			Confidence:            selection.Confidence,
			NeedsUserConfirmation: selection.NeedsUserConfirmation,
			RejectReason:          selection.RejectReason,
			Decision:              decision,
		}, nil
	}
	if selection.Confidence < service.confidenceThreshold {
		return SelectionResult{}, errors.New("selector confidence below threshold")
	}
	candidate, ok := findCandidate(input.Candidates, selection.SelectedWorkflowID, selection.SelectedVersion)
	if !ok {
		return SelectionResult{}, errors.New("selected workflow is not in topK")
	}
	bindings, err := BindVariables(candidate.Variables, BindingInput{
		CandidateSlots: input.CandidateSlots,
		LLMBindings:    selection.Bindings,
	})
	if err != nil {
		return SelectionResult{}, err
	}
	return SelectionResult{
		SelectedWorkflowID:    selection.SelectedWorkflowID,
		SelectedVersion:       selection.SelectedVersion,
		Confidence:            selection.Confidence,
		Bindings:              bindings,
		NeedsUserConfirmation: selection.NeedsUserConfirmation || selection.Confidence < service.autoReplayThreshold,
		RejectReason:          selection.RejectReason,
		Decision:              decision,
	}, nil
}

func topK(candidates []CandidateSummary, limit int) []CandidateSummary {
	if len(candidates) <= limit {
		return candidates
	}
	return candidates[:limit]
}

func findCandidate(candidates []CandidateSummary, workflowID string, version int) (CandidateSummary, bool) {
	for _, candidate := range candidates {
		if candidate.WorkflowID == workflowID && candidate.Version == version {
			return candidate, true
		}
	}
	return CandidateSummary{}, false
}
