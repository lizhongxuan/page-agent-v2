import type { WorkflowCandidate } from './types'

export type WorkflowReplayStatusEvent =
	| { kind: 'search_started'; task: string }
	| { kind: 'matched_workflow'; candidate: WorkflowCandidate }
	| { kind: 'bindings_ready'; bindings: Record<string, unknown> }
	| {
			kind: 'confirmation_required'
			workflowId: string
			version: number
			confidence?: number
			bindings: Record<string, string>
			reason?: string
	  }
	| { kind: 'replay_started'; workflowId: string; version: number }
	| { kind: 'chunk_progress'; chunkName: string; index: number; total: number }
	| { kind: 'interrupt_applied'; handlerName: string }
	| { kind: 'repair_patch_applied'; patchName: string }
	| { kind: 'pending_candidate'; workflowName: string }
	| { kind: 'repair_candidate'; patchName: string }
	| { kind: 'fallback'; reason: string; message: string }
	| { kind: 'completed'; workflowId?: string; runId?: string }

export function formatWorkflowReplayStatus(event: WorkflowReplayStatusEvent): string {
	switch (event.kind) {
		case 'search_started':
			return `Workflow replay search started: ${event.task}`
		case 'matched_workflow':
			return formatMatchedWorkflow(event.candidate)
		case 'bindings_ready':
			return `Workflow variables bound: ${formatBindings(event.bindings)}`
		case 'confirmation_required':
			return `Workflow replay confirmation required: ${event.workflowId} v${event.version}${typeof event.confidence === 'number' ? ` confidence=${event.confidence.toFixed(2)}` : ''}`
		case 'replay_started':
			return `Workflow replay started: ${event.workflowId} v${event.version}`
		case 'chunk_progress':
			return `Workflow chunk ${event.index}/${event.total}: ${event.chunkName}`
		case 'interrupt_applied':
			return `Workflow interrupt handler applied: ${event.handlerName}`
		case 'repair_patch_applied':
			return `Workflow repair patch applied: ${event.patchName}`
		case 'pending_candidate':
			return `Pending workflow candidate created for review: ${event.workflowName}`
		case 'repair_candidate':
			return `Repair patch candidate created for review: ${event.patchName}`
		case 'fallback':
			return `Workflow replay fallback: ${event.reason}. ${event.message}`
		case 'completed':
			return `Workflow replay completed: ${event.workflowId ?? 'unknown workflow'}${event.runId ? ` (${event.runId})` : ''}`
	}
}

function formatMatchedWorkflow(candidate: WorkflowCandidate): string {
	const name = candidate.name || candidate.intent || candidate.workflowId
	const score = Number.isFinite(candidate.finalScore)
		? ` score=${candidate.finalScore.toFixed(2)}`
		: ''
	const reasons = candidate.reasons.length > 0 ? ` reasons: ${candidate.reasons.join('; ')}` : ''
	return `Matched workflow: ${name}${score}.${reasons}`
}

function formatBindings(bindings: Record<string, unknown>): string {
	const entries = Object.entries(bindings)
	if (entries.length === 0) return 'none'
	return entries.map(([key, value]) => `${key}=${formatBindingValue(value)}`).join(', ')
}

function formatBindingValue(value: unknown): string {
	if (typeof value === 'string') return redactValue(value)
	if (!value || typeof value !== 'object') return ''

	const binding = value as { value?: unknown; source?: unknown; confidence?: unknown }
	const text = typeof binding.value === 'string' ? redactValue(binding.value) : ''
	const details: string[] = []
	if (typeof binding.source === 'string' && binding.source) {
		details.push(binding.source)
	}
	if (typeof binding.confidence === 'number' && Number.isFinite(binding.confidence)) {
		details.push(binding.confidence.toFixed(2))
	}
	return details.length > 0 ? `${text} (${details.join(' ')})` : text
}

function redactValue(value: string): string {
	if (/sk-[a-z0-9_-]{8,}|token[:=]\S+|api[_-]?key[:=]\S+|password[:=]\S+/i.test(value)) {
		return '[redacted]'
	}
	return value
}
