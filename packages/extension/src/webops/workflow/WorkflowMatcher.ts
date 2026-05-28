import type { RiskPolicy, WorkflowMatch, WorkflowRejection, WorkflowSearchResult } from './types'

export interface WorkflowMatcherInput {
	candidates: WorkflowSearchResult[]
	pageText: string
	minScore?: number
	riskPolicy?: RiskPolicy
}

export interface WorkflowMatcherResult {
	accepted: WorkflowMatch[]
	rejected: WorkflowRejection[]
}

export function matchWorkflowCandidates(input: WorkflowMatcherInput): WorkflowMatcherResult {
	const minScore = input.minScore ?? 0.7
	const pageText = normalize(input.pageText)
	const accepted: WorkflowMatch[] = []
	const rejected: WorkflowRejection[] = []

	for (const candidate of input.candidates) {
		const recipe = candidate.recipe
		const effectiveStatus = candidate.status ?? recipe.status
		const effectiveMinScore = recipe.pageFingerprint.minMatchScore ?? minScore

		if (effectiveStatus !== 'active') {
			rejected.push({ candidate, reason: 'inactive', detail: `Workflow is ${effectiveStatus}.` })
			continue
		}

		if (candidate.score < effectiveMinScore || candidate.score < minScore) {
			rejected.push({
				candidate,
				reason: 'low_score',
				detail: `Score ${candidate.score} is below minimum ${Math.max(effectiveMinScore, minScore)}.`,
			})
			continue
		}

		const forbiddenText = recipe.pageFingerprint.forbiddenText?.find((text) =>
			pageText.includes(normalize(text))
		)
		if (forbiddenText) {
			rejected.push({
				candidate,
				reason: 'forbidden_text',
				detail: `Visible page contains forbidden text: ${forbiddenText}`,
			})
			continue
		}

		if (candidate.risk && input.riskPolicy?.blocked.includes(candidate.risk)) {
			rejected.push({
				candidate,
				reason: 'blocked_risk',
				detail: `Risk level ${candidate.risk} is blocked.`,
			})
			continue
		}

		accepted.push({ candidate, score: candidate.score })
	}

	accepted.sort((left, right) => {
		if (right.score !== left.score) return right.score - left.score
		return (
			(right.candidate.historicalSuccessRate ?? 0) - (left.candidate.historicalSuccessRate ?? 0)
		)
	})

	return { accepted, rejected }
}

function normalize(text: string): string {
	return text.replace(/\s+/g, ' ').trim().toLowerCase()
}
