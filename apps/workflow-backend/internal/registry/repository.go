package registry

import "context"

type Repository interface {
	SaveCandidate(context.Context, WorkflowCandidate) error
	GetCandidate(context.Context, string) (WorkflowCandidate, error)
	ListCandidates(context.Context, CandidateListQuery) ([]WorkflowCandidate, error)
	SaveWorkflow(context.Context, WorkflowRecipe) error
	GetWorkflow(context.Context, string) (WorkflowRecipe, error)
	ListActiveWorkflows(context.Context, string) ([]WorkflowRecipe, error)
	SaveVersion(context.Context, WorkflowVersion) error
	GetVersion(context.Context, string, int) (WorkflowVersion, error)
	SaveRun(context.Context, WorkflowRun) error
	GetRun(context.Context, string) (WorkflowRun, error)
	SaveSelectorStats(context.Context, SelectorStats) error
	ListSelectorStats(context.Context, string, int) ([]SelectorStats, error)
	SaveInterruptHandler(context.Context, InterruptHandler) error
	ListActiveInterruptHandlers(context.Context, string) ([]InterruptHandler, error)
	SaveRepairPatch(context.Context, RepairPatch) error
	GetRepairPatch(context.Context, string) (RepairPatch, error)
	ListRepairPatches(context.Context, RepairPatchListQuery) ([]RepairPatch, error)
	ListActiveRepairPatches(context.Context, string) ([]RepairPatch, error)
	AppendOutboxEvent(context.Context, OutboxEvent) (OutboxEvent, error)
	ListOutboxEvents(context.Context, Status) ([]OutboxEvent, error)
}

type CandidateListQuery struct {
	ProjectID string
	Source    string
	Status    Status
}

type RepairPatchListQuery struct {
	ProjectID  string
	WorkflowID string
	Status     Status
}
