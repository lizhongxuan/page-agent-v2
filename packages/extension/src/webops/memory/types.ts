export interface MemoryClientConfig {
	baseUrl: string
	bearerToken?: string
}

export interface MemoryPageControl {
	role: string
	name: string
	value?: string
	disabled?: boolean
}

export interface MemoryPageObservation {
	title: string
	visibleText: string[]
	controls: MemoryPageControl[]
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

export type MemoryEvidenceSource = 'knowledge' | 'experience' | 'failure' | 'navigation'

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
	filteredEvidence?: {
		source: string
		id?: string
		reason: string
	}[]
}

export interface MemoryContextResponse {
	contextId?: string
	projectId?: string
	contextPrompt?: string
	evidenceRefs?: MemoryEvidenceRef[]
	currentPageState?: MemoryPageSummary
	currentSurface?: MemorySurfaceSummary
	businessContext?: {
		summary?: string
	}
	knowledgeEvidence?: {
		chunkId: string
		title?: string
		snippet?: string
		score?: number
	}[]
	experienceHints?: {
		id: string
		summary?: string
		confidence?: number
	}[]
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

export interface MemoryDocument {
	id?: string
	projectId?: string
	projectKey?: string
	title: string
	source: string
	url?: string
	content: string
	tags?: string[]
}

export interface MemoryDocumentImportRequest {
	projectId?: string
	projectKey?: string
	documents: MemoryDocument[]
}

export interface MemoryDocumentImportResponse {
	ok?: boolean
	imported?: number
	[key: string]: unknown
}

export interface MemoryClientLike {
	observePage?(request: MemoryPageObservationRequest): Promise<MemoryPageObservationResponse>
	getContext?(request: MemoryContextRequest): Promise<MemoryContextResponse>
	completeTaskRun?(request: MemoryTaskRunRequest): Promise<MemoryTaskRunResponse>
	importDocuments?(request: MemoryDocumentImportRequest): Promise<MemoryDocumentImportResponse>
}
