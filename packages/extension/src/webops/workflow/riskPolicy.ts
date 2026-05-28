import type { WorkflowRiskDecision, WorkflowRiskLevel, WorkflowRiskPolicy } from './types'

export const defaultRiskPolicy: WorkflowRiskPolicy = {
	autoAllowed: ['read_only', 'read_or_search'],
	confirmationRequired: ['draft_change', 'external_send'],
	blocked: ['destructive'],
}

export function evaluateRisk(
	riskLevel: WorkflowRiskLevel,
	policy: WorkflowRiskPolicy = defaultRiskPolicy
): WorkflowRiskDecision {
	if (policy.blocked.includes(riskLevel)) {
		return { decision: 'block', reason: 'risk_blocked' }
	}

	if (policy.autoAllowed.includes(riskLevel)) {
		return { decision: 'allow' }
	}

	if (policy.confirmationRequired.includes(riskLevel)) {
		return { decision: 'confirm', reason: 'risk_requires_confirmation' }
	}

	return { decision: 'block', reason: 'risk_not_allowed' }
}
