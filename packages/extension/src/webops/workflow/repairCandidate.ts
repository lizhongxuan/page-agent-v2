import type {
	RecordedAction,
	RecordedActionTarget,
	RecordedSession,
} from '../recorder/actionEvents'
import type {
	WorkflowRepairPatchCandidateCreateRequest,
	WorkflowStepTarget,
	WorkflowTargetCandidate,
} from './types'

export const PENDING_REPAIR_PATCH_STORAGE_KEY = 'pendingWorkflowRepairPatch'

export interface RepairCandidateBuildContext {
	projectId: string
	workflowId: string
	version: number
	chunkId: string
	stepId: string
	fallbackReason?: string
	currentPageState?: string
	currentUrl?: string
	runId?: string
}

export type RepairCandidateBuildResult =
	| { ok: true; request: WorkflowRepairPatchCandidateCreateRequest }
	| { ok: false; reason: 'missing_context' | 'missing_successful_target'; message: string }

export function buildRepairPatchCandidateRequest(
	session: RecordedSession,
	context: RepairCandidateBuildContext
): RepairCandidateBuildResult {
	if (!context.workflowId || !context.chunkId || !context.stepId) {
		return {
			ok: false,
			reason: 'missing_context',
			message: 'Replay failure context is incomplete; repair patch candidate was not created.',
		}
	}
	const action = selectRepairAction(session.steps, context.fallbackReason)
	const target = toStepTarget(action?.target)
	if (!action || !target) {
		return {
			ok: false,
			reason: 'missing_successful_target',
			message:
				'PageAgent completed the fallback but did not expose a stable target for repair reuse.',
		}
	}
	const site =
		hostFromUrl(context.currentUrl) ?? hostFromUrl(action.pageUrl) ?? hostFromUrl(session.startUrl)
	if (!site) {
		return {
			ok: false,
			reason: 'missing_context',
			message: 'Repair patch candidate requires a stable site.',
		}
	}

	return {
		ok: true,
		request: {
			patch: {
				projectId: context.projectId,
				workflowId: context.workflowId,
				workflowVersion: context.version,
				chunkId: context.chunkId,
				stepId: context.stepId,
				site,
				failureType: failureTypeFromReason(context.fallbackReason),
				failureSignature: context.fallbackReason || 'workflow replay failed',
				oldTarget: context.stepId,
				newTargetSummary: summarizeTarget(action.target),
				newTarget: target,
				appliesToPageStates: context.currentPageState ? [context.currentPageState] : undefined,
				riskLevel: 'read_or_search',
				sourceRunId: context.runId,
				sourceFailureReason: context.fallbackReason,
			},
		},
	}
}

function selectRepairAction(actions: RecordedAction[], failureReason?: string) {
	const successful = actions.filter((action) => action.result === 'success' && action.target)
	if (successful.length === 0) return undefined
	if (/\b(fill|input|search|textbox|searchbox)\b/i.test(failureReason || '')) {
		return successful.find((action) => action.type === 'input') ?? successful[0]
	}
	if (/\b(click|button|link)\b/i.test(failureReason || '')) {
		return successful.find((action) => action.type === 'click') ?? successful[0]
	}
	return successful.find((action) => action.type === 'input') ?? successful[0]
}

function toStepTarget(target?: RecordedActionTarget): WorkflowStepTarget | undefined {
	const candidates = targetCandidates(target)
	if (candidates.length === 0) return undefined
	return {
		primary: candidates[0]!,
		fallbacks: candidates.slice(1),
	}
}

function targetCandidates(target?: RecordedActionTarget): WorkflowTargetCandidate[] {
	if (!target) return []

	const candidates: WorkflowTargetCandidate[] = []
	if (target.role && target.name) {
		candidates.push({ strategy: 'role', role: target.role, name: target.name })
	}
	if (target.name && !target.role) {
		candidates.push({ strategy: 'placeholder', value: target.name })
	}
	if (target.text) {
		candidates.push({ strategy: 'text', value: target.text })
	}
	if (target.testId) {
		candidates.push({ strategy: 'test_id', value: target.testId })
	}
	if (target.css) {
		candidates.push({ strategy: 'css', value: target.css })
	}
	if (target.xpath) {
		candidates.push({ strategy: 'xpath', value: target.xpath })
	}
	return candidates
}

function summarizeTarget(target?: RecordedActionTarget) {
	if (!target) return 'PageAgent repaired target'
	return (
		[target.role, target.name, target.text, target.css, target.xpath, target.testId]
			.filter(Boolean)
			.join(' ')
			.trim() || 'PageAgent repaired target'
	)
}

function failureTypeFromReason(reason: string | undefined) {
	if (!reason) return 'replay_failed'
	const [prefix] = reason.split(':')
	return prefix?.trim() || 'replay_failed'
}

function hostFromUrl(url: string | undefined) {
	if (!url) return undefined
	try {
		return new URL(url).hostname || undefined
	} catch {
		return undefined
	}
}
