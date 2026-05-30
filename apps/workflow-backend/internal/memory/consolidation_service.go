package memory

import (
	"context"
	"errors"
	"strings"

	"github.com/page-agent/workflow-backend/internal/registry"
)

type MemoryConsolidationService struct {
	repo       registry.Repository
	experience *ExperienceService
}

type ConsolidationResult struct {
	TaskRunID              string   `json:"taskRunId"`
	ExperienceID           string   `json:"experienceId,omitempty"`
	FailureMemoryID        string   `json:"failureMemoryId,omitempty"`
	OptimizedPath          []string `json:"optimizedPath,omitempty"`
	BranchNoise            []string `json:"branchNoise,omitempty"`
	AutoPromotedWorkflowID string   `json:"autoPromotedWorkflowId,omitempty"`
	ReviewRequired         bool     `json:"reviewRequired"`
	MemoryUpdates          []string `json:"memoryUpdates,omitempty"`
}

func NewMemoryConsolidationService(repo registry.Repository) *MemoryConsolidationService {
	return &MemoryConsolidationService{repo: repo, experience: NewExperienceService(repo)}
}

func (service *MemoryConsolidationService) ConsolidateTaskRun(ctx context.Context, run registry.TaskRun) (ConsolidationResult, error) {
	if service.repo == nil {
		return ConsolidationResult{}, errors.New("workflow registry is not configured")
	}
	if len(run.OptimizedPath) == 0 {
		optimization := OptimizePath(OptimizePathInput{
			Path:              run.OriginalPath,
			Steps:             pathActionSteps(run),
			PageStates:        pathPageStates(ctx, service.repo, run.ProjectID, run.Site, run.OriginalPath),
			DirectTransitions: directTransitions(ctx, service.repo, run.ProjectID, run.Site),
		})
		run.OptimizedPath = optimization.OptimizedPath
		run.ActionSteps = markRunBranchNoise(run.ActionSteps, optimization.NoiseStepIDs)
	}
	if len(run.OptimizedPath) == 0 {
		run.OptimizedPath = append([]string(nil), run.OriginalPath...)
	}
	if err := service.repo.SaveTaskRun(ctx, run); err != nil {
		return ConsolidationResult{}, err
	}
	result := ConsolidationResult{
		TaskRunID:     run.ID,
		OptimizedPath: run.OptimizedPath,
		BranchNoise:   noisyStepIDs(run.ActionSteps),
	}
	attributionResult, err := service.attributeMemoryEvidence(ctx, run)
	if err != nil {
		return ConsolidationResult{}, err
	}
	if len(attributionResult.Events) > 0 {
		result.MemoryUpdates = append(result.MemoryUpdates, "updated_memory_attribution")
	}
	if len(attributionResult.Stats) > 0 {
		result.MemoryUpdates = append(result.MemoryUpdates, "updated_evidence_stats")
	}
	switch run.Status {
	case registry.TaskRunSuccess:
		experience, err := service.experience.UpsertExperienceFromTaskRun(ctx, run)
		if err != nil {
			return ConsolidationResult{}, err
		}
		result.ExperienceID = experience.ID
		result.ReviewRequired = experience.ReviewStatus == registry.ReviewStatusPending
		result.MemoryUpdates = append(result.MemoryUpdates, "updated_experience_memory")
		if err := incrementTransitionCounts(ctx, service.repo, run, true); err != nil {
			return ConsolidationResult{}, err
		}
		result.MemoryUpdates = append(result.MemoryUpdates, "updated_page_transition")
	case registry.TaskRunFailed, registry.TaskRunPartial:
		failure, err := service.experience.RecordFailureMemory(ctx, run)
		if err != nil {
			return ConsolidationResult{}, err
		}
		result.FailureMemoryID = failure.ID
		result.MemoryUpdates = append(result.MemoryUpdates, "updated_failure_memory")
		_ = incrementTransitionCounts(ctx, service.repo, run, false)
	}
	return result, nil
}

func (service *MemoryConsolidationService) attributeMemoryEvidence(ctx context.Context, run registry.TaskRun) (AttributionResult, error) {
	if run.MemoryContextID == "" && len(run.MemoryEvidenceRefs) == 0 {
		return AttributionResult{}, nil
	}
	var contextEvent registry.MemoryContextEvent
	if run.MemoryContextID != "" {
		event, err := service.repo.GetMemoryContextEvent(ctx, run.MemoryContextID)
		if err != nil && len(run.MemoryEvidenceRefs) == 0 {
			return AttributionResult{}, err
		}
		if err == nil {
			contextEvent = event
		}
	}
	refs := run.MemoryEvidenceRefs
	if len(refs) == 0 {
		refs = contextEvent.EvidenceRefs
	}
	if len(refs) == 0 {
		return AttributionResult{}, nil
	}
	return NewAttributionService(service.repo).EvaluateAndPersist(ctx, AttributionInput{
		TaskRun:         run,
		ContextEvent:    contextEvent,
		EvidenceRefs:    refs,
		PageObservation: service.attributionPageObservation(ctx, run, contextEvent),
	})
}

func (service *MemoryConsolidationService) attributionPageObservation(ctx context.Context, run registry.TaskRun, contextEvent registry.MemoryContextEvent) *NormalizedPageObservation {
	pageStateIDs := []string{
		contextEvent.CurrentPageState,
		firstActionStepPageState(run.ActionSteps),
		firstString(run.OptimizedPath),
		firstString(run.OriginalPath),
	}
	for _, pageStateID := range pageStateIDs {
		pageStateID = strings.TrimSpace(pageStateID)
		if pageStateID == "" {
			continue
		}
		page, err := service.repo.GetPageState(ctx, pageStateID)
		if err != nil {
			continue
		}
		controls := page.RequiredControls
		if len(controls) == 0 {
			controls = page.HardRules.ControlsAll
		}
		return &NormalizedPageObservation{
			PageStateID:       page.ID,
			ProjectID:         page.ProjectID,
			Site:              page.Site,
			URLPattern:        page.URLPattern,
			Title:             page.CanonicalTitle,
			Controls:          controls,
			VisibleTextSample: strings.Join(page.RequiredText, "\n"),
			HardRules:         page.HardRules,
		}
	}
	return nil
}

func firstActionStepPageState(steps []registry.ActionStep) string {
	for _, step := range steps {
		if strings.TrimSpace(step.PageStateID) != "" {
			return step.PageStateID
		}
	}
	return ""
}

func pathActionSteps(run registry.TaskRun) []PathActionStep {
	result := make([]PathActionStep, 0, len(run.ActionSteps))
	for index, step := range run.ActionSteps {
		from := step.PageStateID
		to := step.PageStateID
		if index+1 < len(run.OriginalPath) {
			from = run.OriginalPath[index]
			to = run.OriginalPath[index+1]
		}
		result = append(result, PathActionStep{ID: step.ID, FromPageID: from, ToPageID: to, IsBranchNoise: step.IsBranchNoise})
	}
	return result
}

func pathPageStates(ctx context.Context, repo registry.Repository, projectID, site string, path []string) map[string]PathPageState {
	result := map[string]PathPageState{}
	for _, pageID := range path {
		page, err := repo.GetPageState(ctx, pageID)
		if err != nil {
			result[pageID] = PathPageState{}
			continue
		}
		result[pageID] = PathPageState{
			HasOutput:            len(page.RequiredText) > 3,
			HighRiskConfirmation: page.HardRules.TextAny != nil && containsString(page.HardRules.TextAny, "确认"),
			GuardDependency:      page.Status == registry.StatusPendingReview,
		}
	}
	return result
}

func directTransitions(ctx context.Context, repo registry.Repository, projectID, site string) []DirectTransition {
	transitions, err := repo.ListPageTransitions(ctx, registry.PageTransitionListQuery{ProjectID: projectID, Site: site})
	if err != nil {
		return nil
	}
	result := make([]DirectTransition, 0, len(transitions))
	for _, transition := range transitions {
		result = append(result, DirectTransition{FromPageID: transition.FromPageState, ToPageID: transition.ToPageState})
	}
	return result
}

func markRunBranchNoise(steps []registry.ActionStep, noiseStepIDs []string) []registry.ActionStep {
	noise := map[string]bool{}
	for _, stepID := range noiseStepIDs {
		noise[stepID] = true
	}
	result := append([]registry.ActionStep(nil), steps...)
	for index := range result {
		if noise[result[index].ID] {
			result[index].IsBranchNoise = true
		}
	}
	return result
}

func noisyStepIDs(steps []registry.ActionStep) []string {
	result := []string{}
	for _, step := range steps {
		if step.IsBranchNoise {
			result = append(result, step.ID)
		}
	}
	return result
}

func incrementTransitionCounts(ctx context.Context, repo registry.Repository, run registry.TaskRun, success bool) error {
	for i := 0; i < len(run.OptimizedPath)-1; i++ {
		transitions, err := repo.ListPageTransitions(ctx, registry.PageTransitionListQuery{
			ProjectID:       run.ProjectID,
			Site:            run.Site,
			FromPageStateID: run.OptimizedPath[i],
			ToPageStateID:   run.OptimizedPath[i+1],
		})
		if err != nil {
			return err
		}
		for _, transition := range transitions {
			if success {
				transition.SuccessCount++
			} else {
				transition.FailureCount++
			}
			if err := repo.SavePageTransition(ctx, transition); err != nil {
				return err
			}
		}
	}
	return nil
}
