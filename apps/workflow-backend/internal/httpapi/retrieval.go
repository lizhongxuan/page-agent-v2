package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/page-agent/workflow-backend/internal/registry"
	"github.com/page-agent/workflow-backend/internal/retrieval"
	"github.com/page-agent/workflow-backend/internal/selector"
)

type retrievalRouteServices struct {
	repo     registry.Repository
	search   WorkflowSearchService
	selector WorkflowSelectorService
	indexer  WorkflowIndexer
}

func registerRetrievalRoutes(mux *http.ServeMux, services retrievalRouteServices) {
	mux.HandleFunc("POST /api/retrieval/workflows/search", func(w http.ResponseWriter, r *http.Request) {
		var request retrieval.SearchRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_response", "Request body must be valid JSON.")
			return
		}
		normalized := retrieval.NormalizeRequest(request)
		var candidates []retrieval.WorkflowCandidate
		var err error
		if services.search != nil {
			candidates, err = services.search.Search(r.Context(), request)
			if err != nil {
				writeError(w, http.StatusServiceUnavailable, "qdrant_unavailable", err.Error())
				return
			}
		} else {
			candidates, err = fallbackWorkflowSearch(r, services.repo, normalized)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "no_match", err.Error())
				return
			}
		}
		limit := normalized.Limit
		if limit <= 0 {
			limit = 8
		}
		if len(candidates) > limit {
			candidates = candidates[:limit]
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"currentPageState": inferPageState(normalized),
			"candidateSlots":   normalized.CandidateSlots,
			"candidates":       candidates,
		})
	})
	mux.HandleFunc("POST /api/retrieval/workflows/select", func(w http.ResponseWriter, r *http.Request) {
		var request workflowSelectRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_response", "Request body must be valid JSON.")
			return
		}
		if services.selector == nil {
			writeError(w, http.StatusServiceUnavailable, "selector_failed", "Workflow selector is not configured.")
			return
		}
		if services.repo == nil {
			writeError(w, http.StatusServiceUnavailable, "no_match", "Workflow registry is not configured.")
			return
		}
		input, err := buildSelectionInput(r, services.repo, request)
		if err != nil {
			writeError(w, http.StatusBadRequest, "binding_failed", err.Error())
			return
		}
		result, err := services.selector.Select(r.Context(), input)
		if err != nil {
			writeError(w, http.StatusBadRequest, "selector_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
	mux.HandleFunc("POST /api/retrieval/interrupts/search", func(w http.ResponseWriter, r *http.Request) {
		var request retrieval.InterruptRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_response", "Request body must be valid JSON.")
			return
		}
		var searcher retrieval.InterruptSearcher = registryInterruptSearcher{repo: services.repo}
		if typed, ok := services.search.(retrieval.InterruptSearcher); ok {
			searcher = typed
		}
		service := retrieval.NewInterruptService(searcher)
		handler, ok := service.Search(request)
		handlers := []retrieval.InterruptHit{}
		if ok {
			handlers = append(handlers, handler)
		}
		writeJSON(w, http.StatusOK, map[string]any{"handlers": handlers})
	})
	mux.HandleFunc("POST /api/retrieval/repairs/search", func(w http.ResponseWriter, r *http.Request) {
		var request retrieval.RepairRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_response", "Request body must be valid JSON.")
			return
		}
		var searcher retrieval.RepairSearcher = registryRepairSearcher{repo: services.repo}
		if typed, ok := services.search.(retrieval.RepairSearcher); ok {
			searcher = typed
		}
		service := retrieval.NewRepairService(searcher)
		patch, ok := service.Search(request)
		patches := []retrieval.RepairHit{}
		if ok {
			patches = append(patches, patch)
		}
		writeJSON(w, http.StatusOK, map[string]any{"patches": patches})
	})
	mux.HandleFunc("POST /api/retrieval/index/rebuild", func(w http.ResponseWriter, r *http.Request) {
		if services.indexer == nil {
			writeError(w, http.StatusServiceUnavailable, "qdrant_unavailable", "Workflow indexer is not configured.")
			return
		}
		var request struct {
			ProjectID string `json:"projectId"`
		}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&request)
		}
		count, err := services.indexer.Rebuild(r.Context(), request.ProjectID)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "qdrant_unavailable", err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"status": "indexed", "count": count})
	})
}

type registryInterruptSearcher struct {
	repo registry.Repository
}

func (searcher registryInterruptSearcher) SearchInterrupts(request retrieval.InterruptRequest) ([]retrieval.InterruptHit, error) {
	if searcher.repo == nil {
		return nil, nil
	}
	handlers, err := searcher.repo.ListActiveInterruptHandlers(context.Background(), request.ProjectID)
	if err != nil {
		return nil, err
	}
	hits := make([]retrieval.InterruptHit, 0, len(handlers))
	for _, handler := range handlers {
		if request.Site != "" && handler.Site != request.Site {
			continue
		}
		hits = append(hits, retrieval.InterruptHit{
			HandlerID:           handler.ID,
			WorkflowID:          handler.WorkflowID,
			Version:             handler.Version,
			Score:               0.95,
			Site:                handler.Site,
			RiskLevel:           handler.RiskLevel,
			AppliesToPageStates: handler.AppliesToPageStates,
			RequiredText:        handler.RequiredText,
			TargetControls:      controlsFromSignatures(handler.TargetControls),
			Reasons:             []string{"registry interrupt handler matched"},
		})
	}
	return hits, nil
}

type registryRepairSearcher struct {
	repo registry.Repository
}

func (searcher registryRepairSearcher) SearchRepairs(request retrieval.RepairRequest) ([]retrieval.RepairHit, error) {
	if searcher.repo == nil {
		return nil, nil
	}
	patches, err := searcher.repo.ListActiveRepairPatches(context.Background(), request.ProjectID)
	if err != nil {
		return nil, err
	}
	hits := make([]retrieval.RepairHit, 0, len(patches))
	for _, patch := range patches {
		if request.Site != "" && patch.Site != request.Site {
			continue
		}
		hits = append(hits, retrieval.RepairHit{
			PatchID:             patch.ID,
			WorkflowID:          patch.WorkflowID,
			WorkflowVersion:     patch.WorkflowVersion,
			ChunkID:             patch.ChunkID,
			StepID:              patch.StepID,
			Score:               0.95,
			Site:                patch.Site,
			FailureType:         patch.FailureType,
			AppliesToPageStates: patch.AppliesToPageStates,
			RiskLevel:           patch.RiskLevel,
			Reasons:             []string{"registry repair patch matched"},
		})
	}
	return hits, nil
}

func controlsFromSignatures(values []registry.ControlSignature) []retrieval.Control {
	controls := make([]retrieval.Control, 0, len(values))
	for _, value := range values {
		controls = append(controls, retrieval.Control{Role: value.Role, Name: value.Name})
	}
	return controls
}

func fallbackWorkflowSearch(r *http.Request, repo registry.Repository, normalized retrieval.NormalizedRequest) ([]retrieval.WorkflowCandidate, error) {
	if repo == nil {
		return nil, errors.New("workflow registry is not configured")
	}
	workflows, err := repo.ListActiveWorkflows(r.Context(), normalized.ProjectID)
	if err != nil {
		return nil, err
	}
	raw := make([]retrieval.RawCandidate, 0, len(workflows))
	for _, workflow := range workflows {
		if normalized.Site != "" && workflow.Site != normalized.Site {
			continue
		}
		raw = append(raw, retrieval.RawCandidate{
			WorkflowID:     workflow.ID,
			Version:        workflow.Version,
			Status:         workflow.Status,
			Searchable:     workflow.Searchable,
			Site:           workflow.Site,
			RiskLevel:      workflow.RiskLevel,
			VariableNames:  variableNames(workflow.Variables),
			WorkflowDense:  0.85,
			WorkflowSparse: 0.75,
			BestChunk:      0.7,
			PageState:      0.8,
			SuccessRate:    0.8,
			SelectorHealth: 0.8,
			StepText:       retrieval.WorkflowStepText(workflow),
		})
	}
	candidates := retrieval.Rerank(raw, retrieval.RerankInput{
		CandidateSlots: normalized.CandidateSlots,
		RiskPolicy:     normalized.RiskPolicy,
		Site:           normalized.Site,
		Task:           normalized.Task,
	})
	for index := range candidates {
		workflow, err := repo.GetWorkflow(r.Context(), candidates[index].WorkflowID)
		if err != nil {
			continue
		}
		candidates[index].Name = workflow.Name
		candidates[index].Intent = workflow.Intent
	}
	return candidates, nil
}

type workflowSelectRequest struct {
	ProjectID        string                        `json:"projectId"`
	Task             string                        `json:"task"`
	CurrentPageState string                        `json:"currentPageState"`
	CandidateSlots   map[string]string             `json:"candidateSlots"`
	Candidates       []retrieval.WorkflowCandidate `json:"candidates"`
}

func buildSelectionInput(r *http.Request, repo registry.Repository, request workflowSelectRequest) (selector.SelectionInput, error) {
	projectID := request.ProjectID
	if projectID == "" {
		projectID = "default"
	}
	slots := request.CandidateSlots
	if slots == nil {
		slots = map[string]string{}
	}
	summaries := make([]selector.CandidateSummary, 0, len(request.Candidates))
	for _, candidate := range request.Candidates {
		workflow, err := repo.GetWorkflow(r.Context(), candidate.WorkflowID)
		if err != nil {
			return selector.SelectionInput{}, err
		}
		if workflow.ProjectID != projectID {
			continue
		}
		summaries = append(summaries, selector.CandidateSummary{
			WorkflowID: candidate.WorkflowID,
			Version:    candidate.Version,
			Variables:  workflow.Variables,
			Retrieval:  candidate,
		})
	}
	return selector.SelectionInput{
		Task:             request.Task,
		CurrentPageState: request.CurrentPageState,
		CandidateSlots:   slots,
		Candidates:       summaries,
	}, nil
}

func variableNames(variables []registry.Variable) []string {
	result := make([]string, 0, len(variables))
	for _, variable := range variables {
		if !variable.Sensitive {
			result = append(result, variable.Name)
		}
	}
	return result
}

func inferPageState(request retrieval.NormalizedRequest) string {
	if request.Site == "github.com" {
		if request.CandidateSlots["repo"] != "" {
			return "github_repo_home"
		}
	}
	return ""
}
