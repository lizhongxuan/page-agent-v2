export interface MemoryClientConfig {
	baseUrl: string
	bearerToken?: string
}

export interface MemoryPageControl {
	role: string
	name: string
	value?: string
	disabled?: boolean
	selected?: boolean
	enabled?: boolean
}

export interface MemoryPageTable {
	caption?: string
	headers?: string[]
}

export interface MemoryPageSurface {
	surfaceType?:
		| 'modal'
		| 'drawer'
		| 'popover'
		| 'dropdown'
		| 'carousel'
		| 'toast'
		| 'wizard'
		| 'unknown'
	title?: string
	text?: string[]
	controls?: MemoryPageControl[]
}

export interface MemoryPageObservation {
	title: string
	visibleText: string[]
	controls: MemoryPageControl[]
	breadcrumbs?: string[]
	activeTabs?: string[]
	tables?: MemoryPageTable[]
	activeSurfaces?: MemoryPageSurface[]
}

export interface MemoryPageObservationRequest extends MemoryPageObservation {
	projectId: string
	task: string
	url: string
}

export interface MemoryPageObservationResponse {
	ok?: boolean
	observationId?: string
	pageStateId?: string
	matched?: boolean
	confidence?: number
	pageSummary?: string
	[key: string]: unknown
}

export interface MemoryContextRequest {
	projectId: string
	task: string
	currentUrl: string
	title: string
	pageObservation: MemoryPageObservation
	hints?: string[]
	limit?: number
}

export type MemoryEvidenceSource =
	| 'manual'
	| 'guide'
	| 'experience'
	| 'failure'
	| 'navigation'
	| (string & {})

export type MemoryAttributionLabel = 'helpful' | 'unused' | 'misleading' | 'stale' | 'neutral'

export interface MemoryEvidenceRef {
	source: MemoryEvidenceSource
	id: string
	title?: string
	rank?: number
	score?: number
	label?: MemoryAttributionLabel
	pageStateId?: string
	matchedRules?: string[]
	reason?: string
	payload?: Record<string, unknown>
}

export interface MemoryPageSummary {
	id: string
	name?: string
	summary?: string
}

export interface MemorySurfaceSummary {
	id: string
	type: string
	name?: string
	parentPageStateId?: string
}

export interface MemoryContextDebug {
	promptChars?: number
	promptBudget?: {
		maxChars?: number
		usedChars?: number
	}
	candidateSiteTaskGuides?: {
		id: string
		taskScore?: number
		passed?: boolean
		reason?: string
		matchedStateId?: string
	}[]
	uiStateMatches?: {
		guideId?: string
		stateId?: string
		score?: number
		minimumScore?: number
		passed?: boolean
		matched?: string[]
		missing?: string[]
		reason?: string
	}[]
	filteredEvidence?: {
		source: string
		id?: string
		reason: string
	}[]
}

export interface MemorySiteTaskGuide {
	id: string
	confidence?: number
	summary?: string
	whenToUse?: string
	matchedStateId?: string
	matchedStateName?: string
	startStepOffset?: number
	matchReasons?: string[]
	pageGuards?: string[]
	steps?: string[]
	abandonRules?: string[]
	sourceRefs?: string[]
	[key: string]: unknown
}

export interface MemorySiteManualKnowledge {
	id: string
	wikiPageId?: string
	chunkId?: string
	confidence?: number
	summary?: string
	text?: string
	sourceRefs?: string[]
	pageGuards?: Record<string, unknown>
	targetTerms?: string[]
	[key: string]: unknown
}

export interface MemoryPageObservationSignal {
	id?: string
	pageKey?: string
	summary?: string
	confidence?: number
	site?: string
	url?: string
	urlPattern?: string
	urlFamily?: string
	title?: string
	breadcrumbs?: string[]
	activeTabs?: string[]
	visibleTextSample?: string
	controlSignatures?: MemoryPageControl[]
	tables?: MemoryPageTable[]
	activeSurfaces?: MemoryPageSurface[]
	[key: string]: unknown
}

export interface MemoryContextResponse {
	contextId?: string
	projectId?: string
	contextPrompt?: string
	evidenceRefs?: MemoryEvidenceRef[]
	currentPageState?: MemoryPageSummary
	currentSurface?: MemorySurfaceSummary
	siteTaskGuides?: MemorySiteTaskGuide[]
	siteManualKnowledge?: MemorySiteManualKnowledge[]
	pageObservationSignal?: MemoryPageObservationSignal
	failureWarnings?: {
		id?: string
		summary?: string
	}[]
	recommendedMode?: 'normal' | 'guided'
	debug?: MemoryContextDebug
	[key: string]: unknown
}

export type MemoryTaskRunStatus = 'success' | 'failed' | 'partial'

export type MemoryTaskRunActionType = 'click' | 'fill' | 'select' | 'wait'

export interface MemoryTaskRunActionStep {
	id?: string
	pageStateId: string
	surfaceId?: string
	stepIndex: number
	actionType: MemoryTaskRunActionType
	targetName: string
	valueTemplate?: string
	reasoningSummary?: string
	resultSummary?: string
	isBranchNoise?: boolean
	beforeObservation?: MemoryPageObservationSignal
	afterObservation?: MemoryPageObservationSignal
}

export interface MemoryTaskRunRequest {
	id?: string
	projectId: string
	site: string
	taskTemplate: string
	summary: string
	originalPath: string[]
	optimizedPath: string[]
	status: MemoryTaskRunStatus
	memoryContextId?: string
	memoryEvidenceRefs?: MemoryEvidenceRef[]
	actionSteps?: MemoryTaskRunActionStep[]
}

export interface MemoryTaskRunResponse {
	ok?: boolean
	runId?: string
	taskRunId?: string
	memoryUpdates?: string[]
	[key: string]: unknown
}

export interface MemoryClientLike {
	observePage?(request: MemoryPageObservationRequest): Promise<MemoryPageObservationResponse>
	getContext?(request: MemoryContextRequest): Promise<MemoryContextResponse>
	completeTaskRun?(request: MemoryTaskRunRequest): Promise<MemoryTaskRunResponse>
}
