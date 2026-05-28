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
	candidates?: RecordedTargetCandidate[]
}

export interface RecordedTargetCandidate {
	strategy: 'testId' | 'role' | 'label' | 'placeholder' | 'text' | 'css' | 'xpath'
	value?: string
	role?: string
	name?: string
	nearText?: string
	container?: string
	confidence: number
}

export interface RecordedPageFingerprint {
	url: string
	title: string
	visibleText?: string[]
	controlSignatures?: RecordedControlSignature[]
}

export interface RecordedControlSignature {
	role?: string
	name?: string
	label?: string
	placeholder?: string
	text?: string
	testId?: string
}

export interface RecordedAction {
	id: string
	type: RecordedActionType
	timestamp: number
	pageUrl: string
	pageTitle: string
	target?: RecordedActionTarget
	beforePage?: RecordedPageFingerprint
	afterPage?: RecordedPageFingerprint
	value?: string
	result: 'success' | 'failed' | 'skipped'
	note?: string
}

export interface RecordedKnowledgeHit {
	id: string
	title: string
	source: string
	score: number
}

export interface RedactionReportEntry {
	actionId: string
	field: string
}

export interface RecordedSession {
	id: string
	task: string
	startUrl: string
	startedAt: number
	endedAt?: number
	steps: RecordedAction[]
	knowledgeHits: RecordedKnowledgeHit[]
	redactionReport: RedactionReportEntry[]
}
