export type WorkflowStatus = 'draft' | 'pending_review' | 'active' | 'disabled' | 'archived'

export type RiskLevel =
	| 'read_only'
	| 'form_fill'
	| 'submit_search'
	| 'state_change'
	| 'production_change'
	| 'permission_change'
	| 'payment'
	| 'delete'
	| 'login_secret'
	| 'captcha'
	| 'mfa'

export type RiskDecision = 'auto' | 'confirm' | 'handover' | 'blocked'

export type VariableBindingPolicy =
	| 'auto'
	| 'ask_if_missing'
	| 'confirm_if_ambiguous'
	| 'always_confirm'
	| 'handover_only'

export interface RiskPolicy {
	auto: RiskLevel[]
	confirm: RiskLevel[]
	handover: RiskLevel[]
	blocked: RiskLevel[]
}

export interface ControlSignature {
	role?: string
	name?: string
	label?: string
	placeholder?: string
	text?: string
	testId?: string
}

export interface SemanticRegion {
	name: string
	requiredText?: string[]
	controlSignatures?: ControlSignature[]
}

export interface PageFingerprint {
	urlPatterns?: string[]
	titleAny?: string[]
	requiredText?: string[]
	forbiddenText?: string[]
	controlSignatures?: ControlSignature[]
	semanticRegions?: SemanticRegion[]
	minMatchScore?: number
}

export interface WorkflowVariable {
	name: string
	description?: string
	required: boolean
	sensitive: boolean
	policy: VariableBindingPolicy
	bindingMode?: VariableBindingPolicy
	examples?: string[]
	defaultValue?: string
}

export type WorkflowStep =
	| {
			type: 'observe'
			id: string
			description?: string
			successWhen?: string[]
	  }
	| {
			type: 'click'
			id: string
			target: WorkflowTarget
			description?: string
	  }
	| {
			type: 'input'
			id: string
			target: WorkflowTarget
			valueVariable: string
			description?: string
	  }
	| {
			type: 'select'
			id: string
			target: WorkflowTarget
			valueVariable: string
			description?: string
	  }
	| {
			type: 'scroll'
			id: string
			direction: 'up' | 'down'
			pages?: number
			description?: string
	  }

export interface WorkflowTarget {
	testId?: string
	role?: string
	name?: string
	label?: string
	placeholder?: string
	text?: string
	css?: string
	xpath?: string
	nearText?: string
}

export interface WorkflowChunk {
	id: string
	name: string
	description?: string
	risk: RiskLevel
	riskLevel?: RiskLevel
	steps: WorkflowStep[]
	successWhen?: string[]
}

export interface WorkflowRecipe {
	id: string
	projectId: string
	site: string
	name: string
	description?: string
	intent: string
	tags?: string[]
	urlPatterns: string[]
	pageFingerprint: PageFingerprint
	variables: WorkflowVariable[]
	chunks: WorkflowChunk[]
	safetyPolicy: RiskPolicy
	status: WorkflowStatus
	version: number
	createdAt: string
	updatedAt: string
}

export interface WorkflowSearchRequest {
	projectId: string
	task: string
	url: string
	domain?: string
	pageFingerprint: PageFingerprint
	limit?: number
}

export interface WorkflowSearchResult {
	recipe: WorkflowRecipe
	score: number
	reasons: string[]
	status?: WorkflowStatus
	risk?: RiskLevel
	historicalSuccessRate?: number
}

export interface WorkflowMatch {
	candidate: WorkflowSearchResult
	score: number
}

export interface WorkflowRejection {
	candidate: WorkflowSearchResult
	reason: 'low_score' | 'forbidden_text' | 'inactive' | 'blocked_risk'
	detail: string
}

export interface VariableBinding {
	value: string
	confidence: number
	source: 'task_slot' | 'default'
	evidence: string
}

export type VariableBindingStatus = 'ready' | 'ask' | 'confirm' | 'handover'

export interface VariableBindingResult {
	status: VariableBindingStatus
	bindings: Record<string, VariableBinding>
	missingVariables: string[]
	ambiguousVariables: string[]
	handoverVariables: string[]
}
