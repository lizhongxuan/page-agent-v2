package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/page-agent/workflow-backend/internal/config"
	"github.com/page-agent/workflow-backend/internal/memory"
	"github.com/page-agent/workflow-backend/internal/registry"
)

type memoryPruner interface {
	PruneMemory(context.Context, registry.MemoryPrunePolicy) (registry.MemoryPruneResult, error)
}

func registerMemoryRoutes(mux *http.ServeMux, cfg config.Config, repo registry.Repository, vectorizer KnowledgeVectorizer) {
	_ = vectorizer
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
		contextID := r.URL.Query().Get("contextId")
		pageStates, _ := repo.ListPageStates(r.Context(), registry.PageStateListQuery{ProjectID: projectID, Site: site})
		pageSurfaces, _ := repo.ListPageSurfaces(r.Context(), registry.PageSurfaceListQuery{ProjectID: projectID, Site: site})
		transitions, _ := repo.ListPageTransitions(r.Context(), registry.PageTransitionListQuery{ProjectID: projectID, Site: site})
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
		attributionEvents = filterLegacyExperienceAttributionEvents(attributionEvents)
		evidenceStats, _ := repo.ListMemoryEvidenceStats(r.Context(), registry.MemoryEvidenceStatsListQuery{
			ProjectID: projectID,
			Site:      site,
		})
		evidenceStats = filterLegacyExperienceEvidenceStats(evidenceStats)
		var contextEvent *registry.MemoryContextEvent
		if contextID != "" {
			if event, err := repo.GetMemoryContextEvent(r.Context(), contextID); err == nil {
				contextEvent = &event
			}
		}
		candidateGuides := evidenceRefsBySource(contextEvent, registry.MemoryEvidenceSourceGuide)
		candidateManuals := evidenceRefsBySource(contextEvent, registry.MemoryEvidenceSourceManual)
		guideFeedback, _ := repo.ListMemoryAttributionEvents(r.Context(), registry.MemoryAttributionEventListQuery{
			ProjectID:      projectID,
			Site:           site,
			ContextID:      contextID,
			EvidenceSource: registry.MemoryEvidenceSourceGuide,
		})
		manualFeedback, _ := repo.ListMemoryAttributionEvents(r.Context(), registry.MemoryAttributionEventListQuery{
			ProjectID:      projectID,
			Site:           site,
			ContextID:      contextID,
			EvidenceSource: registry.MemoryEvidenceSourceManual,
		})
		writeJSON(w, http.StatusOK, map[string]any{
			"contextEvent":                 contextEvent,
			"pageObservationSignal":        contextPayloadValue(contextEvent, "pageObservationSignal"),
			"candidateSiteTaskGuides":      candidateGuides,
			"candidateSiteManualKnowledge": candidateManuals,
			"filteredEvidence":             contextDebugValue(contextEvent, "filteredEvidence"),
			"guideFeedback":                guideFeedback,
			"manualFeedback":               manualFeedback,
			"pageStates":                   pageStates,
			"pageSurfaces":                 pageSurfaces,
			"transitions":                  transitions,
			"failures":                     failures,
			"workflows":                    workflows,
			"reviews":                      reviews,
			"taskRuns":                     taskRuns,
			"attributionEvents":            attributionEvents,
			"evidenceStats":                evidenceStats,
		})
	})
}

func evidenceRefsBySource(contextEvent *registry.MemoryContextEvent, source registry.MemoryEvidenceSource) []registry.MemoryEvidenceRef {
	if contextEvent == nil {
		return nil
	}
	result := []registry.MemoryEvidenceRef{}
	for _, ref := range contextEvent.EvidenceRefs {
		if ref.Source == source {
			result = append(result, ref)
		}
	}
	return result
}

func filterLegacyExperienceAttributionEvents(events []registry.MemoryAttributionEvent) []registry.MemoryAttributionEvent {
	result := make([]registry.MemoryAttributionEvent, 0, len(events))
	for _, event := range events {
		if event.EvidenceSource == registry.MemoryEvidenceSourceExperience {
			continue
		}
		result = append(result, event)
	}
	return result
}

func filterLegacyExperienceEvidenceStats(stats []registry.MemoryEvidenceStats) []registry.MemoryEvidenceStats {
	result := make([]registry.MemoryEvidenceStats, 0, len(stats))
	for _, item := range stats {
		if item.EvidenceSource == registry.MemoryEvidenceSourceExperience {
			continue
		}
		result = append(result, item)
	}
	return result
}

func contextPayloadValue(contextEvent *registry.MemoryContextEvent, key string) any {
	if contextEvent == nil || contextEvent.Payload == nil {
		return nil
	}
	return contextEvent.Payload[key]
}

func contextDebugValue(contextEvent *registry.MemoryContextEvent, key string) any {
	debug, ok := contextPayloadValue(contextEvent, "debug").(map[string]any)
	if !ok {
		return nil
	}
	return debug[key]
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
	result, err := pruner.PruneMemory(ctx, policy)
	if err != nil {
		return registry.MemoryPruneResult{}, err
	}
	guideUpdates, err := maintainSiteTaskGuideStatuses(ctx, repo, policy)
	if err != nil {
		return registry.MemoryPruneResult{}, err
	}
	manualUpdates, err := maintainSiteManualStatuses(ctx, repo, policy)
	if err != nil {
		return registry.MemoryPruneResult{}, err
	}
	result.UpdatedSiteTaskGuides = guideUpdates
	result.UpdatedSiteManualSources = manualUpdates
	return result, nil
}

func memoryPrunePolicyEmpty(policy registry.MemoryPrunePolicy) bool {
	return policy.MaxTaskRuns <= 0 &&
		policy.MaxPageObservationEvents <= 0 &&
		policy.MaxMemoryContextEvents <= 0 &&
		policy.MaxFailureMemories <= 0
}

func maintainSiteTaskGuideStatuses(ctx context.Context, repo registry.Repository, policy registry.MemoryPrunePolicy) (int, error) {
	guideRepo, ok := repo.(registry.SiteTaskGuideRepository)
	if !ok {
		return 0, nil
	}
	guides, err := guideRepo.ListSiteTaskGuides(ctx, registry.SiteTaskGuideListQuery{
		ProjectID: policy.ProjectID,
		Site:      policy.Site,
	})
	if err != nil {
		return 0, err
	}
	updated := 0
	for _, guide := range guides {
		if guide.Status != registry.StatusActive {
			continue
		}
		nextStatus := registry.Status("")
		switch {
		case guide.StaleCount >= 3:
			nextStatus = registry.StatusStale
		case guide.MisleadingCount >= 3:
			nextStatus = registry.StatusHidden
		case guide.UnusedCount >= 5 && guide.SuccessCount == 0:
			nextStatus = registry.StatusHidden
		}
		if nextStatus == "" {
			continue
		}
		if err := guideRepo.UpdateSiteTaskGuideStatus(ctx, guide.ID, nextStatus); err != nil {
			return updated, err
		}
		updated++
	}
	return updated, nil
}

func maintainSiteManualStatuses(ctx context.Context, repo registry.Repository, policy registry.MemoryPrunePolicy) (int, error) {
	manualRepo, ok := repo.(registry.SiteManualRepository)
	if !ok {
		return 0, nil
	}
	sources, err := manualRepo.ListSiteManualSources(ctx, registry.SiteManualSourceListQuery{
		ProjectID:     policy.ProjectID,
		Site:          policy.Site,
		IncludeHidden: true,
	})
	if err != nil {
		return 0, err
	}
	stats, err := repo.ListMemoryEvidenceStats(ctx, registry.MemoryEvidenceStatsListQuery{
		ProjectID:      policy.ProjectID,
		Site:           policy.Site,
		EvidenceSource: registry.MemoryEvidenceSourceManual,
	})
	if err != nil {
		return 0, err
	}
	statsByID := map[string]registry.MemoryEvidenceStats{}
	for _, item := range stats {
		statsByID[item.EvidenceID] = item
	}
	updated := 0
	for _, source := range sources {
		if source.Status != registry.StatusActive {
			continue
		}
		wiki, err := manualRepo.GetSiteManualWikiForSource(ctx, source.ID)
		if err != nil {
			continue
		}
		staleCount := 0
		helpfulCount := 0
		for _, chunk := range wiki.Chunks {
			if !sourceRefsContainID(chunk.SourceRefs, source.ID) {
				continue
			}
			stat := statsByID[chunk.ID]
			staleCount += stat.StaleCount
			helpfulCount += stat.HelpfulCount
		}
		if staleCount < 3 || helpfulCount > 0 {
			continue
		}
		if err := manualRepo.UpdateSiteManualSourceStatus(ctx, source.ID, registry.StatusStale); err != nil {
			return updated, err
		}
		updated++
	}
	return updated, nil
}

func sourceRefsContainID(refs []registry.MemorySourceRef, id string) bool {
	for _, ref := range refs {
		if ref.ID == id {
			return true
		}
	}
	return false
}

func siteFromRawURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Host)
}
