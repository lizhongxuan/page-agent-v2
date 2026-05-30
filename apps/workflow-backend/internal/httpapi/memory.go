package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/page-agent/workflow-backend/internal/config"
	"github.com/page-agent/workflow-backend/internal/knowledge"
	"github.com/page-agent/workflow-backend/internal/memory"
	"github.com/page-agent/workflow-backend/internal/registry"
)

type memoryPruner interface {
	PruneMemory(context.Context, registry.MemoryPrunePolicy) (registry.MemoryPruneResult, error)
}

func registerMemoryRoutes(mux *http.ServeMux, cfg config.Config, repo registry.Repository, vectorizer KnowledgeVectorizer) {
	mux.HandleFunc("POST /api/memory/page-observations", func(w http.ResponseWriter, r *http.Request) {
		if repo == nil {
			writeError(w, http.StatusServiceUnavailable, "memory_failed", "Workflow registry is not configured.")
			return
		}
		var request memory.PageObservationRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
			return
		}
		response, err := memory.NewPageObservationService(repo).ObservePage(r.Context(), request)
		if err != nil {
			writeError(w, http.StatusBadRequest, "memory_observation_failed", err.Error())
			return
		}
		pruneMemoryBestEffort(r.Context(), repo, registry.MemoryPrunePolicy{
			ProjectID:                request.ProjectID,
			Site:                     siteFromRawURL(request.URL),
			MaxPageObservationEvents: cfg.MemoryMaxPageObservationEventsPerSite,
		})
		writeJSON(w, http.StatusOK, response)
	})

	mux.HandleFunc("POST /api/memory/context", func(w http.ResponseWriter, r *http.Request) {
		if repo == nil {
			writeError(w, http.StatusServiceUnavailable, "memory_failed", "Workflow registry is not configured.")
			return
		}
		var request memory.MemoryContextRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
			return
		}
		response, err := memory.NewMemoryContextService(repo).GetContext(r.Context(), request)
		if err != nil {
			writeError(w, http.StatusBadRequest, "memory_context_failed", err.Error())
			return
		}
		pruneMemoryBestEffort(r.Context(), repo, registry.MemoryPrunePolicy{
			ProjectID:              defaultProjectID(request.ProjectID),
			MaxMemoryContextEvents: cfg.MemoryMaxContextEventsPerProject,
		})
		writeJSON(w, http.StatusOK, response)
	})

	mux.HandleFunc("POST /api/memory/documents", func(w http.ResponseWriter, r *http.Request) {
		if repo == nil {
			writeError(w, http.StatusServiceUnavailable, "memory_failed", "Workflow registry is not configured.")
			return
		}
		documents, err := decodeMemoryDocuments(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		service := knowledge.NewServiceWithVectorizer(repo, vectorizer)
		ids, err := service.Ingest(r.Context(), documents)
		if err != nil {
			writeError(w, http.StatusBadRequest, "memory_documents_failed", err.Error())
			return
		}
		profile, updated, err := memory.NewBusinessProfileService(repo).UpdateFromDocuments(r.Context(), documents)
		if err != nil {
			writeError(w, http.StatusBadRequest, "memory_profile_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"documentIds":            ids,
			"updatedBusinessProfile": updated,
			"businessProfile":        profile,
		})
	})

	mux.HandleFunc("POST /api/memory/task-runs", func(w http.ResponseWriter, r *http.Request) {
		if repo == nil {
			writeError(w, http.StatusServiceUnavailable, "memory_failed", "Workflow registry is not configured.")
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be readable JSON.")
			return
		}
		if hasActionInstanceValue(body) {
			writeError(w, http.StatusBadRequest, "memory_task_run_invalid", "action steps must use valueTemplate instead of value")
			return
		}
		var run registry.TaskRun
		if err := json.Unmarshal(body, &run); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
			return
		}
		prepareTaskRun(&run)
		if err := validateTaskRunRequest(run); err != nil {
			writeError(w, http.StatusBadRequest, "memory_task_run_invalid", err.Error())
			return
		}
		result, err := memory.NewMemoryConsolidationService(repo).ConsolidateTaskRun(r.Context(), run)
		if err != nil {
			writeError(w, http.StatusBadRequest, "memory_task_run_failed", err.Error())
			return
		}
		pruneMemoryBestEffort(r.Context(), repo, registry.MemoryPrunePolicy{
			ProjectID:          run.ProjectID,
			Site:               run.Site,
			MaxTaskRuns:        cfg.MemoryMaxTaskRunsPerSite,
			MaxFailureMemories: cfg.MemoryMaxFailureMemoriesPerSite,
		})
		writeJSON(w, http.StatusCreated, result)
	})

	mux.HandleFunc("GET /api/memory/reviews", func(w http.ResponseWriter, r *http.Request) {
		if repo == nil {
			writeError(w, http.StatusServiceUnavailable, "memory_failed", "Workflow registry is not configured.")
			return
		}
		reviews, err := repo.ListMemoryReviews(r.Context(), registry.MemoryReviewListQuery{
			ProjectID:  r.URL.Query().Get("projectId"),
			TargetType: registry.ReviewTargetType(r.URL.Query().Get("targetType")),
			Status:     registry.ReviewStatus(r.URL.Query().Get("status")),
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, "memory_reviews_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"reviews": reviews})
	})
	mux.HandleFunc("POST /api/memory/reviews/{id}/approve", func(w http.ResponseWriter, r *http.Request) {
		if repo == nil {
			writeError(w, http.StatusServiceUnavailable, "memory_failed", "Workflow registry is not configured.")
			return
		}
		reviewID := r.PathValue("id")
		review, err := findMemoryReview(r, repo, reviewID)
		if err != nil {
			writeError(w, http.StatusNotFound, "memory_review_not_found", err.Error())
			return
		}
		if err := repo.ApproveMemoryReview(r.Context(), reviewID); err != nil {
			writeError(w, http.StatusNotFound, "memory_review_not_found", err.Error())
			return
		}
		if err := applyApprovedMemoryReview(r, repo, review); err != nil {
			writeError(w, http.StatusBadRequest, "memory_review_apply_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": registry.ReviewStatusApproved, "targetType": review.TargetType, "targetId": review.TargetID})
	})
	mux.HandleFunc("POST /api/memory/reviews/{id}/reject", func(w http.ResponseWriter, r *http.Request) {
		if repo == nil {
			writeError(w, http.StatusServiceUnavailable, "memory_failed", "Workflow registry is not configured.")
			return
		}
		if err := repo.RejectMemoryReview(r.Context(), r.PathValue("id")); err != nil {
			writeError(w, http.StatusNotFound, "memory_review_not_found", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": registry.ReviewStatusRejected})
	})

	mux.HandleFunc("POST /api/memory/maintenance/prune", func(w http.ResponseWriter, r *http.Request) {
		if repo == nil {
			writeError(w, http.StatusServiceUnavailable, "memory_failed", "Workflow registry is not configured.")
			return
		}
		var policy registry.MemoryPrunePolicy
		if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
			return
		}
		result, err := pruneMemory(r.Context(), repo, policy)
		if err != nil {
			writeError(w, http.StatusBadRequest, "memory_prune_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("GET /api/memory/inspector", func(w http.ResponseWriter, r *http.Request) {
		if repo == nil {
			writeError(w, http.StatusServiceUnavailable, "memory_failed", "Workflow registry is not configured.")
			return
		}
		projectID := r.URL.Query().Get("projectId")
		site := r.URL.Query().Get("site")
		module := r.URL.Query().Get("module")
		contextID := r.URL.Query().Get("contextId")
		profile, _ := repo.GetBusinessSystemProfile(r.Context(), registry.BusinessSystemProfileQuery{
			ProjectID:  projectID,
			Site:       site,
			Module:     module,
			SourceType: registry.MemorySourceProduction,
		})
		pageStates, _ := repo.ListPageStates(r.Context(), registry.PageStateListQuery{ProjectID: projectID, Site: site})
		pageSurfaces, _ := repo.ListPageSurfaces(r.Context(), registry.PageSurfaceListQuery{ProjectID: projectID, Site: site})
		transitions, _ := repo.ListPageTransitions(r.Context(), registry.PageTransitionListQuery{ProjectID: projectID, Site: site})
		experiences, _ := repo.ListExperienceMemories(r.Context(), registry.ExperienceMemorySearchQuery{ProjectID: projectID, Site: site})
		failures, _ := repo.ListFailureMemories(r.Context(), registry.FailureMemorySearchQuery{ProjectID: projectID, Site: site})
		workflows, _ := repo.ListActiveWorkflows(r.Context(), projectID)
		reviews, _ := repo.ListMemoryReviews(r.Context(), registry.MemoryReviewListQuery{ProjectID: projectID})
		taskRuns, _ := repo.ListTaskRuns(r.Context(), registry.TaskRunListQuery{ProjectID: projectID, Site: site})
		attributionEvents, _ := repo.ListMemoryAttributionEvents(r.Context(), registry.MemoryAttributionEventListQuery{
			ProjectID: projectID,
			Site:      site,
			TaskRunID: r.URL.Query().Get("taskRunId"),
			ContextID: r.URL.Query().Get("contextId"),
		})
		evidenceStats, _ := repo.ListMemoryEvidenceStats(r.Context(), registry.MemoryEvidenceStatsListQuery{
			ProjectID: projectID,
			Site:      site,
		})
		var contextEvent *registry.MemoryContextEvent
		if contextID != "" {
			if event, err := repo.GetMemoryContextEvent(r.Context(), contextID); err == nil {
				contextEvent = &event
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"businessProfile":   profile,
			"contextEvent":      contextEvent,
			"pageStates":        pageStates,
			"pageSurfaces":      pageSurfaces,
			"transitions":       transitions,
			"experiences":       experiences,
			"failures":          failures,
			"workflows":         workflows,
			"reviews":           reviews,
			"taskRuns":          taskRuns,
			"attributionEvents": attributionEvents,
			"evidenceStats":     evidenceStats,
		})
	})
}

func findMemoryReview(r *http.Request, repo registry.Repository, id string) (registry.MemoryReview, error) {
	reviews, err := repo.ListMemoryReviews(r.Context(), registry.MemoryReviewListQuery{})
	if err != nil {
		return registry.MemoryReview{}, err
	}
	for _, review := range reviews {
		if review.ID == id {
			return review, nil
		}
	}
	return registry.MemoryReview{}, errString("memory review not found")
}

func applyApprovedMemoryReview(r *http.Request, repo registry.Repository, review registry.MemoryReview) error {
	switch review.TargetType {
	case registry.ReviewTargetWorkflow:
		workflow, err := repo.GetWorkflow(r.Context(), review.TargetID)
		if err != nil {
			return nil
		}
		workflow.Status = registry.StatusActive
		workflow.Searchable = true
		return repo.SaveWorkflow(r.Context(), workflow)
	case registry.ReviewTargetExperience:
		experience, err := repo.GetExperienceMemory(r.Context(), review.TargetID)
		if err != nil {
			return nil
		}
		experience.ReviewStatus = registry.ReviewStatusApproved
		experience.Searchable = true
		return repo.SaveExperienceMemory(r.Context(), experience)
	default:
		return nil
	}
}

func decodeMemoryDocuments(r *http.Request) ([]knowledge.DocumentInput, error) {
	var raw struct {
		Documents []memory.MemoryDocumentInput `json:"documents"`
		memory.MemoryDocumentInput
	}
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		return nil, err
	}
	items := raw.Documents
	if len(items) == 0 && raw.Content != "" {
		items = []memory.MemoryDocumentInput{raw.MemoryDocumentInput}
	}
	projectID := defaultProjectID(raw.ProjectID)
	documents := make([]knowledge.DocumentInput, 0, len(items))
	for _, item := range items {
		itemProjectID := item.ProjectID
		if itemProjectID == "" {
			itemProjectID = projectID
		}
		documents = append(documents, knowledge.DocumentInput{
			ID:         item.ID,
			ProjectID:  defaultProjectID(itemProjectID),
			Title:      item.Title,
			Source:     item.Source,
			URL:        item.URL,
			Content:    item.Content,
			Tags:       item.Tags,
			SourceType: item.SourceType,
		})
	}
	return documents, nil
}

func defaultProjectID(projectID string) string {
	if projectID == "" {
		return "default"
	}
	return projectID
}

func pruneMemoryBestEffort(ctx context.Context, repo registry.Repository, policy registry.MemoryPrunePolicy) {
	if memoryPrunePolicyEmpty(policy) {
		return
	}
	_, _ = pruneMemory(ctx, repo, policy)
}

func pruneMemory(ctx context.Context, repo registry.Repository, policy registry.MemoryPrunePolicy) (registry.MemoryPruneResult, error) {
	pruner, ok := repo.(memoryPruner)
	if !ok {
		return registry.MemoryPruneResult{}, nil
	}
	return pruner.PruneMemory(ctx, policy)
}

func memoryPrunePolicyEmpty(policy registry.MemoryPrunePolicy) bool {
	return policy.MaxTaskRuns <= 0 &&
		policy.MaxPageObservationEvents <= 0 &&
		policy.MaxMemoryContextEvents <= 0 &&
		policy.MaxFailureMemories <= 0
}

func siteFromRawURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Host)
}
