import type {
	MemoryContextDebug,
	MemoryEvidenceRef,
	MemoryPageObservationSignal,
	MemoryPageSummary,
	MemorySurfaceSummary,
} from '../memory/types'

export type RecordedActionType =
	| 'observe'
	| 'click'
	| 'input'
	| 'select'
	| 'wait'
	| 'navigate'
	| 'extract'
	| 'handover'

export interface RecordedActionTarget {
	role?: string
	name?: string
	text?: string
	css?: string
	xpath?: string
	testId?: string
	elementIndex?: number
}

export interface RecordedAction {
	id: string
	type: RecordedActionType
	timestamp: number
	pageUrl: string
	pageTitle: string
	surfaceId?: string
	target?: RecordedActionTarget
	value?: string
	result: 'success' | 'failed' | 'skipped'
	note?: string
	beforeObservation?: MemoryPageObservationSignal
	afterObservation?: MemoryPageObservationSignal
}

export interface RecordedMemoryHit {
	id: string
	title: string
	source: string
	score: number
}

export interface RedactionReportEntry {
	actionId: string
	field: string
}

export interface RecordedMemoryContext {
	contextId?: string
	contextPrompt: string
	recommendedMode?: 'normal' | 'guided'
	currentPageState?: MemoryPageSummary
	currentSurface?: MemorySurfaceSummary
	evidenceRefs: MemoryEvidenceRef[]
	debug?: MemoryContextDebug
}

export interface RecordedSession {
	id: string
	task: string
	startUrl: string
	startedAt: number
	endedAt?: number
	steps: RecordedAction[]
	memoryHits: RecordedMemoryHit[]
	redactionReport: RedactionReportEntry[]
	memoryContext?: RecordedMemoryContext
	memoryUpdates?: string[]
}
