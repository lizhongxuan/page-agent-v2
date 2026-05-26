import type { BrowserState } from '@page-agent/page-controller'

import type { HistoricalEvent } from '../types'

export interface ContextConfig {
	enabled?: boolean
	maxPromptTokens?: number
	recentStepCount?: number
	compactAfterSteps?: number
	compactAtBudgetRatio?: number
	preserveFailedSteps?: number
	persistence?: 'none' | 'adapter'
	debug?: boolean
}

export type ContextSectionKey =
	| 'instructions'
	| 'task'
	| 'sessionSummary'
	| 'protectedFacts'
	| 'recentSteps'
	| 'businessObjects'
	| 'currentPage'
	| 'safetyRules'
	| 'debug'

export interface ContextSection {
	key: ContextSectionKey
	title: string
	content: string
	priority: number
	required?: boolean
}

export interface ContextBudgetItem {
	key: ContextSectionKey
	estimatedTokens: number
	characters: number
	status: 'kept' | 'trimmed' | 'dropped'
}

export interface ContextBudgetReport {
	maxPromptTokens: number
	estimatedTokens: number
	items: ContextBudgetItem[]
	triggeredCompact: boolean
}

export interface BusinessObjectRef {
	type: string
	name?: string
	id?: string
	aliases?: string[]
	evidence: string[]
}

export interface BrowserSessionSummary {
	userGoal: string
	currentSubGoal: string
	completed: string[]
	failedAttempts: string[]
	stableFacts: string[]
	businessObjects: BusinessObjectRef[]
	userDecisions: string[]
	riskDecisions: string[]
	currentPageSemanticState: {
		url: string
		title: string
		semanticLocation: string
		visibleEvidence: string[]
	}
	doNotRepeat: string[]
	nextRecommendedActions: string[]
}

export interface ContextPack {
	instructions: ContextSection[]
	agentState: ContextSection
	sessionSummary?: ContextSection
	protectedFacts: ContextSection
	recentSteps: ContextSection
	currentPage: ContextSection
	safetyRules: ContextSection
	budgetReport: ContextBudgetReport
}

export interface BuildContextPackInput {
	task: string
	stepCount: number
	maxSteps: number
	currentTime: string
	instructions: string
	history: HistoricalEvent[]
	browserState: BrowserState
}

export type ContinuationMode =
	| 'continue_original_task'
	| 'new_task_same_page'
	| 'same_business_new_page'
	| 'fresh_task'
	| 'ask_user_to_confirm'

export interface ContinuationDecision {
	mode: ContinuationMode
	reason: string
	inheritedSections: ContextSectionKey[]
	discardedSections: ContextSectionKey[]
	requiresFreshObserve: boolean
	requiresUserConfirmation: boolean
}

export const DEFAULT_CONTEXT_CONFIG: Required<ContextConfig> = {
	enabled: false,
	maxPromptTokens: 32_000,
	recentStepCount: 6,
	compactAfterSteps: 12,
	compactAtBudgetRatio: 0.7,
	preserveFailedSteps: 3,
	persistence: 'none',
	debug: false,
}
