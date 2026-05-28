package replay

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/page-agent/workflow-backend/internal/registry"
)

const (
	ResultSucceeded = "succeeded"
	ResultFailed    = "failed"
)

var ErrChunkFailed = errors.New("workflow chunk failed")

type Runner interface {
	RunChunk(context.Context, ChunkRunRequest) error
}

type WorkflowRunner interface {
	RunWorkflow(context.Context, WorkflowRunRequest) error
}

type DetailedWorkflowRunner interface {
	RunWorkflowDetailed(context.Context, WorkflowRunRequest) (RunExecution, error)
}

type RunExecution struct {
	Logs          []registry.WorkflowRunLog `json:"logs,omitempty"`
	SelectorStats []registry.SelectorStats  `json:"selectorStats,omitempty"`
	ArtifactRefs  []registry.ArtifactRef    `json:"artifactRefs,omitempty"`
	FailedChunkID string                    `json:"failedChunkId,omitempty"`
	FailedStepID  string                    `json:"failedStepId,omitempty"`
	FailureReason string                    `json:"failureReason,omitempty"`
}

type ChunkRunRequest struct {
	Workflow registry.WorkflowRecipe
	Chunk    registry.WorkflowChunk
	Bindings map[string]string
	URL      string
}

type WorkflowRunRequest struct {
	Workflow           registry.WorkflowRecipe
	Bindings           map[string]string
	URL                string
	RepairPatches      []registry.RepairPatch
	InterruptHandlers  []registry.InterruptHandler
	InterruptWorkflows map[string]registry.WorkflowRecipe
}

type NoopPlaywrightRunner struct{}

func (NoopPlaywrightRunner) RunChunk(context.Context, ChunkRunRequest) error {
	return nil
}

type StartRequest struct {
	ProjectID  string
	WorkflowID string
	Version    int
	Task       string
	URL        string
	Bindings   map[string]string
}

type Service struct {
	repo   registry.Repository
	runner Runner
}

func NewService(repo registry.Repository, runner Runner) *Service {
	return &Service{repo: repo, runner: runner}
}

func (service *Service) Start(ctx context.Context, request StartRequest) (registry.WorkflowRun, error) {
	workflow, err := service.repo.GetWorkflow(ctx, request.WorkflowID)
	if err != nil {
		return registry.WorkflowRun{}, err
	}
	run := registry.WorkflowRun{
		ID:         newRunID(),
		ProjectID:  request.ProjectID,
		WorkflowID: workflow.ID,
		Version:    workflow.Version,
		Task:       request.Task,
		URL:        request.URL,
		Variables:  redactBindings(request.Bindings),
		Result:     ResultSucceeded,
		CreatedAt:  time.Now().UTC(),
	}
	startedAt := time.Now()
	defer func() {
		run.DurationMillis = int(time.Since(startedAt) / time.Millisecond)
	}()
	if detailedRunner, ok := service.runner.(DetailedWorkflowRunner); ok {
		interruptHandlers, interruptWorkflows := service.activeInterruptHandlers(ctx, workflow)
		execution, err := detailedRunner.RunWorkflowDetailed(ctx, WorkflowRunRequest{
			Workflow:           workflow,
			Bindings:           request.Bindings,
			URL:                request.URL,
			RepairPatches:      service.activeRepairPatches(ctx, workflow),
			InterruptHandlers:  interruptHandlers,
			InterruptWorkflows: interruptWorkflows,
		})
		run.Logs = ensureLogTimestamps(execution.Logs)
		run.ArtifactRefs = append(run.ArtifactRefs, execution.ArtifactRefs...)
		if err != nil {
			run.Result = ResultFailed
			run.FailedChunkID = execution.FailedChunkID
			run.FailedStepID = execution.FailedStepID
			run.FallbackReason = execution.FailureReason
			if run.FallbackReason == "" {
				run.FallbackReason = err.Error()
			}
			if run.FailedChunkID == "" {
				run.FailedChunkID = "workflow"
			}
		}
		service.persistExecution(ctx, &workflow, run, execution.SelectorStats)
		return run, nil
	}
	if workflowRunner, ok := service.runner.(WorkflowRunner); ok {
		interruptHandlers, interruptWorkflows := service.activeInterruptHandlers(ctx, workflow)
		if err := workflowRunner.RunWorkflow(ctx, WorkflowRunRequest{
			Workflow:           workflow,
			Bindings:           request.Bindings,
			URL:                request.URL,
			RepairPatches:      service.activeRepairPatches(ctx, workflow),
			InterruptHandlers:  interruptHandlers,
			InterruptWorkflows: interruptWorkflows,
		}); err != nil {
			run.Result = ResultFailed
			run.FailedChunkID = "workflow"
			run.FallbackReason = err.Error()
			service.persistExecution(ctx, &workflow, run, nil)
			return run, nil
		}
		service.persistExecution(ctx, &workflow, run, nil)
		return run, nil
	}
	var selectorStats []registry.SelectorStats
	for _, chunk := range workflow.Chunks {
		run.Logs = append(run.Logs, registry.WorkflowRunLog{
			Timestamp: time.Now().UTC(),
			Event:     "chunk_started",
			ChunkID:   chunk.ID,
			PageState: chunk.FromPageState,
		})
		if err := service.runner.RunChunk(ctx, ChunkRunRequest{
			Workflow: workflow,
			Chunk:    chunk,
			Bindings: request.Bindings,
			URL:      request.URL,
		}); err != nil {
			run.Result = ResultFailed
			run.FailedChunkID = chunk.ID
			run.FallbackReason = err.Error()
			run.Logs = append(run.Logs, registry.WorkflowRunLog{
				Timestamp: time.Now().UTC(),
				Event:     "chunk_failed",
				ChunkID:   chunk.ID,
				Reason:    err.Error(),
			})
			selectorStats = append(selectorStats, selectorStatsFromChunk(workflow, chunk, false, err.Error())...)
			service.persistExecution(ctx, &workflow, run, selectorStats)
			return run, nil
		}
		run.Logs = append(run.Logs, registry.WorkflowRunLog{
			Timestamp: time.Now().UTC(),
			Event:     "chunk_finished",
			ChunkID:   chunk.ID,
			PageState: chunk.ToPageState,
		})
		selectorStats = append(selectorStats, selectorStatsFromChunk(workflow, chunk, true, "")...)
	}
	service.persistExecution(ctx, &workflow, run, selectorStats)
	return run, nil
}

func (service *Service) activeRepairPatches(ctx context.Context, workflow registry.WorkflowRecipe) []registry.RepairPatch {
	patches, err := service.repo.ListActiveRepairPatches(ctx, workflow.ProjectID)
	if err != nil {
		return nil
	}
	result := make([]registry.RepairPatch, 0, len(patches))
	for _, patch := range patches {
		if patch.WorkflowID != workflow.ID || patch.WorkflowVersion != workflow.Version {
			continue
		}
		if patch.RiskLevel == registry.RiskDestructive || patch.RiskLevel == registry.RiskExternalSend {
			continue
		}
		result = append(result, patch)
	}
	return result
}

func (service *Service) activeInterruptHandlers(ctx context.Context, workflow registry.WorkflowRecipe) ([]registry.InterruptHandler, map[string]registry.WorkflowRecipe) {
	handlers, err := service.repo.ListActiveInterruptHandlers(ctx, workflow.ProjectID)
	if err != nil {
		return nil, nil
	}
	result := make([]registry.InterruptHandler, 0, len(handlers))
	workflows := map[string]registry.WorkflowRecipe{}
	for _, handler := range handlers {
		if handler.Site != "" && handler.Site != workflow.Site {
			continue
		}
		if handler.RiskLevel == registry.RiskDestructive || handler.RiskLevel == registry.RiskExternalSend {
			continue
		}
		if handler.WorkflowID == "" {
			continue
		}
		handlerWorkflow, err := service.repo.GetWorkflow(ctx, handler.WorkflowID)
		if err != nil {
			continue
		}
		if handler.Version > 0 && handlerWorkflow.Version != handler.Version {
			continue
		}
		if handlerWorkflow.ProjectID != workflow.ProjectID {
			continue
		}
		result = append(result, handler)
		workflows[handler.WorkflowID] = handlerWorkflow
	}
	return result, workflows
}

func (service *Service) persistExecution(
	ctx context.Context,
	workflow *registry.WorkflowRecipe,
	run registry.WorkflowRun,
	stats []registry.SelectorStats,
) {
	now := time.Now().UTC()
	for _, item := range stats {
		if item.WorkflowID == "" {
			item.WorkflowID = workflow.ID
		}
		if item.Version == 0 {
			item.Version = workflow.Version
		}
		if item.SuccessCount > 0 && item.LastSuccessAt.IsZero() {
			item.LastSuccessAt = now
		}
		if item.FailCount > 0 && item.LastFailAt.IsZero() {
			item.LastFailAt = now
		}
		_ = service.repo.SaveSelectorStats(ctx, item)
	}
	if len(stats) > 0 {
		service.updateWorkflowSelectorHealth(ctx, workflow, stats)
		_, _ = service.repo.AppendOutboxEvent(ctx, registry.OutboxEvent{
			Type:           "run_stats_updated",
			IdempotencyKey: "run_stats_updated:" + run.ID,
			Payload: map[string]any{
				"workflowId": workflow.ID,
				"version":    workflow.Version,
				"runId":      run.ID,
			},
		})
	}
	_ = service.repo.SaveRun(ctx, run)
}

func (service *Service) updateWorkflowSelectorHealth(
	ctx context.Context,
	workflow *registry.WorkflowRecipe,
	current []registry.SelectorStats,
) {
	existing, err := service.repo.ListSelectorStats(ctx, workflow.ID, workflow.Version)
	if err != nil {
		existing = nil
	}
	all := append(existing, current...)
	byChunk := map[string][]registry.SelectorStats{}
	stepToChunk := map[string]string{}
	for _, chunk := range workflow.Chunks {
		for _, step := range chunk.Steps {
			stepToChunk[step.ID] = chunk.ID
		}
	}
	for _, stats := range all {
		chunkID := stepToChunk[stats.StepID]
		if chunkID == "" {
			continue
		}
		byChunk[chunkID] = append(byChunk[chunkID], stats)
	}
	for index := range workflow.Chunks {
		stats := byChunk[workflow.Chunks[index].ID]
		if len(stats) == 0 {
			continue
		}
		successes := 0
		failures := 0
		for _, item := range stats {
			successes += item.SuccessCount
			failures += item.FailCount
		}
		total := successes + failures
		if total == 0 {
			continue
		}
		health := float64(successes) / float64(total)
		workflow.Chunks[index].SelectorHealth = health
		workflow.Chunks[index].SuccessRate = health
		workflow.Chunks[index].RecentFailureCount = failures
	}
	workflow.UpdatedAt = time.Now().UTC()
	_ = service.repo.SaveWorkflow(ctx, *workflow)
}

func selectorStatsFromChunk(
	workflow registry.WorkflowRecipe,
	chunk registry.WorkflowChunk,
	success bool,
	failureReason string,
) []registry.SelectorStats {
	now := time.Now().UTC()
	result := make([]registry.SelectorStats, 0, len(chunk.Steps))
	for _, step := range chunk.Steps {
		selector := step.Target.Primary.Text()
		if selector == "" {
			selector = string(step.Type)
		}
		stats := registry.SelectorStats{
			WorkflowID: workflow.ID,
			Version:    workflow.Version,
			StepID:     step.ID,
			Strategy:   string(step.Target.Primary.Strategy),
			Selector:   selector,
		}
		if success {
			stats.SuccessCount = 1
			stats.LastSuccessAt = now
		} else {
			stats.FailCount = 1
			stats.LastFailAt = now
			stats.LastFailureReason = failureReason
		}
		result = append(result, stats)
	}
	return result
}

func redactBindings(bindings map[string]string) map[string]string {
	if len(bindings) == 0 {
		return nil
	}
	redacted := make(map[string]string, len(bindings))
	for key := range bindings {
		redacted[key] = "[redacted]"
	}
	return redacted
}

func ensureLogTimestamps(logs []registry.WorkflowRunLog) []registry.WorkflowRunLog {
	now := time.Now().UTC()
	for index := range logs {
		if logs[index].Timestamp.IsZero() {
			logs[index].Timestamp = now
		}
	}
	return logs
}

func newRunID() string {
	return "run_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "") + "_" + strconv.FormatInt(time.Now().UnixNano()%1_000_000, 10)
}
