export type WorkflowRiskLevel =
	| 'read_only'
	| 'read_or_search'
	| 'draft_change'
	| 'external_send'
	| 'destructive'

export type WorkflowReviewStatus =
	| 'pending_review'
	| 'active'
	| 'disabled'
	| 'archived'
	| 'deleted'
	| 'indexed'
	| 'index_failed'
	| 'rejected'

export type WorkflowVariableType = 'string' | 'number' | 'boolean'

export type WorkflowVariableSource = 'task' | 'url' | 'task_or_url' | 'page_state' | 'user'

export type WorkflowStepType = 'click' | 'fill' | 'press' | 'select' | 'wait'

export type WorkflowTargetStrategy =
	| 'role'
	| 'label'
	| 'placeholder'
	| 'test_id'
	| 'text'
	| 'css'
	| 'xpath'

export interface WorkflowVariable {
	name: string
	type: WorkflowVariableType
	required: boolean
	source: WorkflowVariableSource
	examples?: string[]
	sensitive?: boolean
}

export interface WorkflowTargetCandidate {
	strategy: WorkflowTargetStrategy
	value?: string
	role?: string
	name?: string
}

export interface WorkflowStepTarget {
	primary: WorkflowTargetCandidate
	fallbacks?: WorkflowTargetCandidate[]
}

export interface WorkflowRecipeStep {
	id: string
	type: WorkflowStepType
	target?: WorkflowStepTarget
	value?: string
	key?: string
	riskLevel: WorkflowRiskLevel
}

export interface WorkflowRecipeChunk {
	id: string
	name: string
	fromPageState?: string
	toPageState?: string
	preconditionText?: string
	postconditionText?: string
	stepSummary?: string
	riskLevel: WorkflowRiskLevel
	steps: WorkflowRecipeStep[]
	variableNames?: string[]
	successRate?: number
	selectorHealth?: number
}

export interface WorkflowRecipe {
	id: string
	version: number
	projectId: string
	tenantId?: string
	status: WorkflowReviewStatus
	searchable: boolean
	site: string
	app?: string
	name: string
	intent: string
	description?: string
	tags?: string[]
	riskLevel: WorkflowRiskLevel
	requiresConfirmation: boolean
	variables: WorkflowVariable[]
	chunks: WorkflowRecipeChunk[]
	startPageStates?: string[]
	endPageStates?: string[]
	createdAt?: string
	updatedAt?: string
}

export interface WorkflowCandidateReview {
	id: string
	projectId: string
	source: string
	task: string
	startUrl: string
	recipeDraft: WorkflowRecipe
	status: WorkflowReviewStatus
	searchable: boolean
	createdAt?: string
	reviewedAt?: string
}

export interface WorkflowCandidateCreateRequest {
	projectId: string
	source: string
	task: string
	startUrl: string
	recipe: WorkflowRecipe
}

export interface WorkflowCandidateListRequest {
	projectId?: string
	source?: string
	reviewStatus?: WorkflowReviewStatus
}

export interface WorkflowCandidateListResponse {
	candidates: WorkflowCandidateReview[]
}

export interface WorkflowRiskPolicy {
	autoAllowed: readonly WorkflowRiskLevel[]
	confirmationRequired: readonly WorkflowRiskLevel[]
	blocked: readonly WorkflowRiskLevel[]
}

export type WorkflowRiskDecision =
	| { decision: 'allow' }
	| { decision: 'confirm'; reason: 'risk_requires_confirmation' }
	| { decision: 'block'; reason: 'risk_blocked' | 'risk_not_allowed' }

export interface WorkflowControlObservation {
	role: string
	name: string
	value?: string
	disabled?: boolean
}

export interface WorkflowPageObservation {
	title: string
	visibleText: string[]
	controls: WorkflowControlObservation[]
}

export interface WorkflowScoreBreakdown {
	workflowDenseScore?: number
	workflowSparseScore?: number
	bestChunkScore?: number
	stepCoverage?: number
	pageStateScore?: number
	variableBindability?: number
	historicalSuccessRate?: number
	selectorHealth?: number
	recencyScore?: number
	userPreferenceScore?: number
	riskPenalty?: number
	recentFailurePenalty?: number
	finalScore: number
}

export interface WorkflowCandidate {
	workflowId: string
	version: number
	name?: string
	intent?: string
	finalScore: number
	riskLevel: WorkflowRiskLevel
	variables: string[]
	score?: WorkflowScoreBreakdown
	reasons: string[]
}

export interface WorkflowSearchRequest {
	projectId: string
	task: string
	currentUrl: string
	pageObservation: WorkflowPageObservation
	riskPolicy: WorkflowRiskPolicy
	limit?: number
}

export interface WorkflowSearchResponse {
	currentPageState?: string
	candidateSlots?: Record<string, string>
	candidates: WorkflowCandidate[]
}

export interface WorkflowSelectRequest {
	projectId: string
	task: string
	currentPageState?: string
	candidateSlots?: Record<string, string>
	candidates: WorkflowCandidate[]
}

export type WorkflowSelectDecision = 'replay' | 'no_match' | 'unsafe_risk' | 'binding_failed'

export interface WorkflowSelectResponse {
	selectedWorkflowId?: string
	version?: number
	selectedVersion?: number
	confidence?: number
	bindings?: Record<string, unknown>
	needsUserConfirmation?: boolean
	rejectReason?: string
	decision: WorkflowSelectDecision
}

export interface WorkflowInterruptSearchRequest {
	projectId: string
	workflowId: string
	version: number
	currentPageState: string
	pageObservation: WorkflowPageObservation
	riskPolicy: WorkflowRiskPolicy
}

export interface WorkflowInterruptHandler {
	handlerId: string
	workflowId?: string
	version?: number
	name?: string
	interruptType: string
	riskLevel: WorkflowRiskLevel
	score?: WorkflowScoreBreakdown
	reasons: string[]
}

export interface WorkflowInterruptSearchResponse {
	handlers: WorkflowInterruptHandler[]
}

export interface WorkflowTargetSummary {
	role?: string
	name?: string
	text?: string
	selector?: string
}

export interface WorkflowRepairSearchRequest {
	projectId: string
	workflowId: string
	version: number
	chunkId: string
	stepId: string
	failureType: string
	currentPageState: string
	currentUrl: string
	pageObservation: WorkflowPageObservation
	oldTarget?: WorkflowTargetSummary
}

export interface WorkflowRepairPatch {
	patchId: string
	workflowId: string
	version: number
	chunkId?: string
	stepId?: string
	failureType: string
	riskLevel: WorkflowRiskLevel
	score?: WorkflowScoreBreakdown
	reasons: string[]
}

export interface WorkflowRepairSearchResponse {
	patches: WorkflowRepairPatch[]
}

export interface WorkflowRepairPatchCandidate {
	id?: string
	projectId: string
	status?: WorkflowReviewStatus
	workflowId: string
	workflowVersion: number
	chunkId: string
	stepId: string
	site: string
	app?: string
	failureType: string
	failureSignature: string
	oldTarget?: string
	newTargetSummary: string
	newTarget: WorkflowStepTarget
	appliesToPageStates?: string[]
	riskLevel: WorkflowRiskLevel
	successRate?: number
	sourceRunId?: string
	sourceFailureReason?: string
	createdAt?: string
	reviewedAt?: string
	updatedAt?: string
}

export interface WorkflowRepairPatchCandidateCreateRequest {
	patch: WorkflowRepairPatchCandidate
}

export interface WorkflowRepairPatchListRequest {
	projectId?: string
	workflowId?: string
	status?: WorkflowReviewStatus
}

export interface WorkflowRepairPatchListResponse {
	patches: WorkflowRepairPatchCandidate[]
}

export interface WorkflowRunStartRequest {
	projectId: string
	selectedWorkflowId: string
	version: number
	bindings: Record<string, unknown>
	currentUrl: string
	pageObservation: WorkflowPageObservation
}

export type WorkflowRunStatus = 'queued' | 'running' | 'succeeded' | 'failed'

export interface WorkflowRunStartResponse {
	runId?: string
	status?: WorkflowRunStatus
	workflowId?: string
	version?: number
	failedChunkId?: string
	failedStepId?: string
	fallbackReason?: string
}

export type WorkflowErrorCode =
	| 'network_error'
	| 'http_error'
	| 'invalid_response'
	| 'invalid_json'
	| 'qdrant_unavailable'
	| 'embedding_failed'
	| 'no_match'
	| 'unsafe_risk'
	| 'binding_failed'
	| 'selector_failed'
	| 'candidate_invalid'
	| 'candidate_list_failed'
	| 'candidate_not_found'
	| 'candidate_approve_failed'
	| 'candidate_reject_failed'
	| 'repair_patch_invalid'
	| 'repair_patch_list_failed'
	| 'repair_patch_approve_failed'
	| 'repair_patch_reject_failed'
	| 'replay_failed'
	| 'run_not_found'

export interface WorkflowErrorResponse {
	code: WorkflowErrorCode
	message: string
	status?: number
	retryable?: boolean
	details?: unknown
}

export type WorkflowResult<T> =
	| {
			ok: true
			data: T
	  }
	| {
			ok: false
			error: WorkflowErrorResponse
	  }
