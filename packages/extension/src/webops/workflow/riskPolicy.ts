import type { RiskDecision, RiskPolicy, WorkflowChunk } from './types'

export interface RiskPolicyDecision {
	decision: RiskDecision
	reason: string
}

export function evaluateChunkRisk(chunk: WorkflowChunk, policy: RiskPolicy): RiskPolicyDecision {
	if (policy.blocked.includes(chunk.risk)) {
		return { decision: 'blocked', reason: `Risk level ${chunk.risk} is blocked.` }
	}
	if (policy.handover.includes(chunk.risk)) {
		return { decision: 'handover', reason: `Risk level ${chunk.risk} requires handover.` }
	}
	if (policy.confirm.includes(chunk.risk)) {
		return { decision: 'confirm', reason: `Risk level ${chunk.risk} requires confirmation.` }
	}
	if (policy.auto.includes(chunk.risk)) {
		return { decision: 'auto', reason: `Risk level ${chunk.risk} is allowed for auto replay.` }
	}

	return { decision: 'blocked', reason: `Risk level ${chunk.risk} is not configured.` }
}
