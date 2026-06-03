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
	ListVersions(context.Context, string) ([]WorkflowVersion, error)
	SaveRun(context.Context, WorkflowRun) error
	GetRun(context.Context, string) (WorkflowRun, error)
	SaveTaskRun(context.Context, TaskRun) error
	GetTaskRun(context.Context, string) (TaskRun, error)
	ListTaskRuns(context.Context, TaskRunListQuery) ([]TaskRun, error)
	SavePageState(context.Context, PageState) error
	GetPageState(context.Context, string) (PageState, error)
	ListPageStates(context.Context, PageStateListQuery) ([]PageState, error)
	SavePageSurface(context.Context, PageSurface) error
	ListPageSurfaces(context.Context, PageSurfaceListQuery) ([]PageSurface, error)
	SavePageTransition(context.Context, PageTransition) error
	ListPageTransitions(context.Context, PageTransitionListQuery) ([]PageTransition, error)
	SavePageObservationEvent(context.Context, PageObservationEvent) error
	ListPageObservationEvents(context.Context, PageObservationEventListQuery) ([]PageObservationEvent, error)
	SaveExperienceMemory(context.Context, ExperienceMemory) error
	GetExperienceMemory(context.Context, string) (ExperienceMemory, error)
	SearchExperienceMemories(context.Context, ExperienceMemorySearchQuery) ([]ExperienceMemory, error)
	ListExperienceMemories(context.Context, ExperienceMemorySearchQuery) ([]ExperienceMemory, error)
	SaveFailureMemory(context.Context, FailureMemory) error
	SearchFailureMemories(context.Context, FailureMemorySearchQuery) ([]FailureMemory, error)
	ListFailureMemories(context.Context, FailureMemorySearchQuery) ([]FailureMemory, error)
	SaveMemoryContextEvent(context.Context, MemoryContextEvent) error
	GetMemoryContextEvent(context.Context, string) (MemoryContextEvent, error)
	SaveMemoryAttributionEvent(context.Context, MemoryAttributionEvent) error
	ListMemoryAttributionEvents(context.Context, MemoryAttributionEventListQuery) ([]MemoryAttributionEvent, error)
	SaveMemoryEvidenceStats(context.Context, MemoryEvidenceStats) error
	GetMemoryEvidenceStats(context.Context, string, MemoryEvidenceSource, string) (MemoryEvidenceStats, error)
	ListMemoryEvidenceStats(context.Context, MemoryEvidenceStatsListQuery) ([]MemoryEvidenceStats, error)
	SaveMemoryReview(context.Context, MemoryReview) error
	ListMemoryReviews(context.Context, MemoryReviewListQuery) ([]MemoryReview, error)
	ApproveMemoryReview(context.Context, string) error
	RejectMemoryReview(context.Context, string) error
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

type SiteTaskGuideRepository interface {
	SaveSiteTaskGuide(context.Context, SiteTaskGuide) error
	GetSiteTaskGuide(context.Context, string) (SiteTaskGuide, error)
	ListSiteTaskGuides(context.Context, SiteTaskGuideListQuery) ([]SiteTaskGuide, error)
	SearchSiteTaskGuides(context.Context, SiteTaskGuideSearchQuery) ([]SiteTaskGuide, error)
	UpdateSiteTaskGuideStatus(context.Context, string, Status) error
	SaveSiteTaskGuideFeedback(context.Context, SiteTaskGuideFeedback) error
}

type SiteManualRepository interface {
	SaveSiteManualSource(context.Context, SiteManualSource) error
	FindSiteManualSourceByHash(context.Context, SiteManualSourceHashQuery) (SiteManualSource, error)
	GetSiteManualSource(context.Context, string) (SiteManualSource, error)
	ListSiteManualSources(context.Context, SiteManualSourceListQuery) ([]SiteManualSource, error)
	UpdateSiteManualSourceStatus(context.Context, string, Status) error
	DeleteSiteManualSource(context.Context, string) error
	SaveSiteManualWiki(context.Context, []SiteManualWikiPage, []SiteManualWikiChunk) error
	GetSiteManualWikiForSource(context.Context, string) (SiteManualWiki, error)
	SearchSiteManualWikiChunks(context.Context, SiteManualWikiSearchQuery) ([]SiteManualKnowledgeMatch, error)
}

type CandidateListQuery struct {
	ProjectID string
	Source    string
	Status    Status
}

type SiteTaskGuideListQuery struct {
	ProjectID string
	Site      string
	Module    string
	Status    Status
}

type SiteTaskGuideSearchQuery struct {
	ProjectID string
	Site      string
	Module    string
	Task      string
	Limit     int
}

type RepairPatchListQuery struct {
	ProjectID  string
	WorkflowID string
	Status     Status
}
