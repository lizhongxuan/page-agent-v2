import type { WorkflowReplayStatusEvent } from './replayStatus'
import { defaultRiskPolicy } from './riskPolicy'
import type {
	WorkflowCandidateListResponse,
	WorkflowPageObservation,
	WorkflowRecipe,
	WorkflowRepairSearchResponse,
	WorkflowResult,
	WorkflowRunStartResponse,
	WorkflowSearchResponse,
	WorkflowSelectResponse,
} from './types'

interface ReplayClient {
	searchWorkflows: (
		request: Parameters<import('./RetrievalClient').RetrievalClient['searchWorkflows']>[0]
	) => Promise<WorkflowResult<WorkflowSearchResponse>>
	selectWorkflow: (
		request: Parameters<import('./RetrievalClient').RetrievalClient['selectWorkflow']>[0]
	) => Promise<WorkflowResult<WorkflowSelectResponse>>
	searchRepairs?: (
		request: Parameters<import('./RetrievalClient').RetrievalClient['searchRepairs']>[0]
	) => Promise<WorkflowResult<WorkflowRepairSearchResponse>>
	startRun: (
		request: Parameters<import('./RetrievalClient').RetrievalClient['startRun']>[0]
	) => Promise<WorkflowResult<WorkflowRunStartResponse>>
	listWorkflowCandidates?: (
		request: Parameters<import('./RetrievalClient').RetrievalClient['listWorkflowCandidates']>[0]
	) => Promise<WorkflowResult<WorkflowCandidateListResponse>>
}

export interface WorkflowReplayExecutorRequest {
	workflow: WorkflowRecipe
	bindings: Record<string, string>
	currentUrl: string
	currentPageState?: string
}

export type WorkflowReplayExecutorResult =
	| { ok: true }
	| {
			ok: false
			message: string
			failedChunkId?: string
			failedStepId?: string
	  }

export type WorkflowReplayExecutor = (
	request: WorkflowReplayExecutorRequest
) => Promise<WorkflowReplayExecutorResult>

export interface WorkflowReplayConfirmationRequest {
	workflowId: string
	version: number
	confidence?: number
	bindings: Record<string, string>
	reason?: string
}

export type WorkflowReplayConfirmation = (
	request: WorkflowReplayConfirmationRequest
) => Promise<boolean>

export interface WorkflowReplayServiceConfig {
	client: ReplayClient
	projectId?: string
	onStatus?: (event: WorkflowReplayStatusEvent) => void
	executeReplay?: WorkflowReplayExecutor
	confirmReplay?: WorkflowReplayConfirmation
}

export interface WorkflowReplayRequest {
	task: string
	currentUrl: string
	pageObservation: WorkflowPageObservation
}

export interface WorkflowRepairContext {
	workflowId: string
	version: number
	chunkId: string
	stepId: string
	fallbackReason?: string
	currentPageState?: string
	currentUrl: string
	runId?: string
}

export type WorkflowReplayResult =
	| {
			status: 'completed'
			runId?: string
			workflowId?: string
			version?: number
	  }
	| {
			status: 'fallback'
			reason:
				| 'backend_unavailable'
				| 'no_match'
				| 'selector_rejected'
				| 'binding_failed'
				| 'unsafe_risk'
				| 'confirmation_rejected'
				| 'run_failed'
			message: string
			repairContext?: WorkflowRepairContext
	  }

export class WorkflowReplayService {
	private readonly client: ReplayClient
	private readonly projectId: string
	private readonly onStatus?: (event: WorkflowReplayStatusEvent) => void
	private readonly executeReplay?: WorkflowReplayExecutor
	private readonly confirmReplay?: WorkflowReplayConfirmation

	constructor(config: WorkflowReplayServiceConfig) {
		this.client = config.client
		this.projectId = config.projectId ?? 'default'
		this.onStatus = config.onStatus
		this.executeReplay = config.executeReplay
		this.confirmReplay = config.confirmReplay
	}

	async tryReplay(request: WorkflowReplayRequest): Promise<WorkflowReplayResult> {
		this.emit({ kind: 'search_started', task: request.task })
		const search = await this.client.searchWorkflows({
			projectId: this.projectId,
			task: request.task,
			currentUrl: request.currentUrl,
			pageObservation: request.pageObservation,
			riskPolicy: defaultRiskPolicy,
			limit: 20,
		})
		if (!search.ok) {
			this.emit({ kind: 'fallback', reason: 'backend_unavailable', message: search.error.message })
			return {
				status: 'fallback',
				reason: 'backend_unavailable',
				message: search.error.message,
			}
		}
		if (search.data.candidates.length === 0) {
			this.emit({ kind: 'fallback', reason: 'no_match', message: 'No matching workflow found.' })
			return { status: 'fallback', reason: 'no_match', message: 'No matching workflow found.' }
		}
		this.emit({ kind: 'matched_workflow', candidate: search.data.candidates[0] })

		const selection = await this.client.selectWorkflow({
			projectId: this.projectId,
			task: request.task,
			currentPageState: search.data.currentPageState,
			candidateSlots: search.data.candidateSlots,
			candidates: search.data.candidates,
		})
		if (!selection.ok) {
			this.emit({
				kind: 'fallback',
				reason: selection.error.code === 'binding_failed' ? 'binding_failed' : 'selector_rejected',
				message: selection.error.message,
			})
			return {
				status: 'fallback',
				reason: selection.error.code === 'binding_failed' ? 'binding_failed' : 'selector_rejected',
				message: selection.error.message,
			}
		}
		if (selection.data.decision !== 'replay' || !selection.data.selectedWorkflowId) {
			const reason = workflowSelectionFallbackReason(selection.data.decision)
			this.emit({
				kind: 'fallback',
				reason,
				message: selection.data.rejectReason || 'Workflow selector did not choose a replay.',
			})
			return {
				status: 'fallback',
				reason,
				message: selection.data.rejectReason || 'Workflow selector did not choose a replay.',
			}
		}

		const version = selection.data.version ?? selection.data.selectedVersion ?? 1
		const bindings = flattenBindings(selection.data.bindings ?? {})
		this.emit({ kind: 'bindings_ready', bindings: selection.data.bindings ?? {} })
		if (selection.data.needsUserConfirmation) {
			const confirmationRequest = {
				workflowId: selection.data.selectedWorkflowId,
				version,
				confidence: selection.data.confidence,
				bindings,
				reason: selection.data.rejectReason || search.data.candidates[0]?.reasons.join('; '),
			}
			this.emit({ kind: 'confirmation_required', ...confirmationRequest })
			const approved = this.confirmReplay ? await this.confirmReplay(confirmationRequest) : false
			if (!approved) {
				const message = 'Workflow replay confirmation was rejected or unavailable.'
				this.emit({ kind: 'fallback', reason: 'confirmation_rejected', message })
				return { status: 'fallback', reason: 'confirmation_rejected', message }
			}
		}
		this.emit({ kind: 'replay_started', workflowId: selection.data.selectedWorkflowId, version })
		if (this.executeReplay) {
			return this.executeSelectedWorkflow({
				request,
				workflowId: selection.data.selectedWorkflowId,
				version,
				bindings,
				currentPageState: search.data.currentPageState,
			})
		}
		const run = await this.client.startRun({
			projectId: this.projectId,
			selectedWorkflowId: selection.data.selectedWorkflowId,
			version,
			bindings,
			currentUrl: request.currentUrl,
			pageObservation: request.pageObservation,
		})
		if (!run.ok) {
			this.emit({ kind: 'fallback', reason: 'run_failed', message: run.error.message })
			return { status: 'fallback', reason: 'run_failed', message: run.error.message }
		}
		if (run.data.status === 'failed') {
			const repairContext = {
				workflowId: selection.data.selectedWorkflowId,
				version,
				chunkId: run.data.failedChunkId || 'workflow',
				stepId: run.data.failedStepId || '',
				fallbackReason: run.data.fallbackReason,
				currentPageState: search.data.currentPageState,
				currentUrl: request.currentUrl,
				runId: run.data.runId,
			}
			await this.searchRepairBeforeFallback({
				request,
				workflowId: selection.data.selectedWorkflowId,
				version,
				currentPageState: search.data.currentPageState,
				run: run.data,
			})
			this.emit({
				kind: 'fallback',
				reason: 'run_failed',
				message: 'Workflow replay failed and will fall back to PageAgent.',
			})
			return {
				status: 'fallback',
				reason: 'run_failed',
				message: 'Workflow replay failed and will fall back to PageAgent.',
				repairContext,
			}
		}
		this.emit({
			kind: 'completed',
			runId: run.data.runId,
			workflowId: run.data.workflowId ?? selection.data.selectedWorkflowId,
		})
		return {
			status: 'completed',
			runId: run.data.runId,
			workflowId: run.data.workflowId ?? selection.data.selectedWorkflowId,
			version: run.data.version ?? version,
		}
	}

	private emit(event: WorkflowReplayStatusEvent) {
		this.onStatus?.(event)
	}

	private async executeSelectedWorkflow(input: {
		request: WorkflowReplayRequest
		workflowId: string
		version: number
		bindings: Record<string, string>
		currentPageState?: string
	}): Promise<WorkflowReplayResult> {
		const workflowResult = await this.loadActiveWorkflow(input.workflowId, input.version)
		if (!workflowResult.ok) {
			this.emit({ kind: 'fallback', reason: 'run_failed', message: workflowResult.error.message })
			return {
				status: 'fallback',
				reason: 'run_failed',
				message: workflowResult.error.message,
				repairContext: {
					workflowId: input.workflowId,
					version: input.version,
					chunkId: 'workflow',
					stepId: '',
					currentPageState: input.currentPageState,
					currentUrl: input.request.currentUrl,
				},
			}
		}
		const result = await this.executeReplay!({
			workflow: workflowResult.data,
			bindings: input.bindings,
			currentUrl: input.request.currentUrl,
			currentPageState: input.currentPageState,
		})
		if (!result.ok) {
			const repairContext = {
				workflowId: input.workflowId,
				version: input.version,
				chunkId: result.failedChunkId || 'workflow',
				stepId: result.failedStepId || '',
				fallbackReason: result.message,
				currentPageState: input.currentPageState,
				currentUrl: input.request.currentUrl,
			}
			this.emit({ kind: 'fallback', reason: 'run_failed', message: result.message })
			return {
				status: 'fallback',
				reason: 'run_failed',
				message: result.message,
				repairContext,
			}
		}
		this.emit({
			kind: 'completed',
			workflowId: input.workflowId,
		})
		return {
			status: 'completed',
			workflowId: input.workflowId,
			version: input.version,
		}
	}

	private async loadActiveWorkflow(
		workflowId: string,
		version: number
	): Promise<WorkflowResult<WorkflowRecipe>> {
		if (!this.client.listWorkflowCandidates) {
			return {
				ok: false,
				error: {
					code: 'invalid_response',
					message: 'Workflow recipe lookup is not available.',
					retryable: false,
				},
			}
		}
		const candidates = await this.client.listWorkflowCandidates({
			projectId: this.projectId,
			reviewStatus: 'active',
		})
		if (!candidates.ok) return candidates
		const matched = candidates.data.candidates.find(
			(candidate) =>
				candidate.recipeDraft.id === workflowId &&
				(version <= 0 || candidate.recipeDraft.version === version)
		)
		if (!matched) {
			return {
				ok: false,
				error: {
					code: 'no_match',
					message: `Selected workflow recipe was not found: ${workflowId}`,
					retryable: false,
				},
			}
		}
		return { ok: true, data: matched.recipeDraft }
	}

	private async searchRepairBeforeFallback(input: {
		request: WorkflowReplayRequest
		workflowId: string
		version: number
		currentPageState?: string
		run: WorkflowRunStartResponse
	}) {
		if (!this.client.searchRepairs) return
		const failureType = failureTypeFromReason(input.run.fallbackReason)
		const result = await this.client.searchRepairs({
			projectId: this.projectId,
			workflowId: input.workflowId,
			version: input.version,
			chunkId: input.run.failedChunkId || 'workflow',
			stepId: input.run.failedStepId || '',
			failureType,
			currentPageState: input.currentPageState || '',
			currentUrl: input.request.currentUrl,
			pageObservation: input.request.pageObservation,
		})
		if (!result.ok || result.data.patches.length === 0) return
		const patch = result.data.patches[0]
		this.emit({ kind: 'repair_candidate', patchName: patch.patchId })
	}
}

function failureTypeFromReason(reason: string | undefined) {
	if (!reason) return 'replay_failed'
	const [prefix] = reason.split(':')
	return prefix?.trim() || 'replay_failed'
}

function flattenBindings(bindings: Record<string, unknown>): Record<string, string> {
	const result: Record<string, string> = {}
	for (const [key, value] of Object.entries(bindings)) {
		if (typeof value === 'string') {
			result[key] = value
			continue
		}
		if (
			value &&
			typeof value === 'object' &&
			typeof (value as { value?: unknown }).value === 'string'
		) {
			result[key] = (value as { value: string }).value
		}
	}
	return result
}

function workflowSelectionFallbackReason(
	decision: string
): Extract<WorkflowReplayResult, { status: 'fallback' }>['reason'] {
	if (decision === 'unsafe_risk') return 'unsafe_risk'
	if (decision === 'no_match') return 'no_match'
	if (decision === 'binding_failed') return 'binding_failed'
	return 'selector_rejected'
}
