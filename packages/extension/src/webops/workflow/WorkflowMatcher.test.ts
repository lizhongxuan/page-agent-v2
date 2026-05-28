import { describe, expect, it } from 'vitest'

import { matchWorkflowCandidates } from './WorkflowMatcher'
import type { WorkflowSearchResult } from './types'

const candidate = (overrides: Partial<WorkflowSearchResult> = {}): WorkflowSearchResult => ({
	recipe: {
		id: 'recipe-1',
		projectId: 'project-1',
		site: 'console.example.test',
		name: 'Find service',
		intent: 'Find a service by name',
		urlPatterns: ['https://console.example.test/*'],
		pageFingerprint: {
			requiredText: ['Services'],
			forbiddenText: ['Access denied'],
			minMatchScore: 0.7,
		},
		variables: [],
		chunks: [],
		safetyPolicy: {
			auto: ['read_only', 'form_fill', 'submit_search'],
			confirm: ['state_change'],
			handover: ['login_secret', 'captcha', 'mfa'],
			blocked: ['delete', 'payment', 'permission_change', 'production_change'],
		},
		status: 'active',
		version: 1,
		createdAt: '2026-05-27T00:00:00.000Z',
		updatedAt: '2026-05-27T00:00:00.000Z',
	},
	score: 0.92,
	reasons: ['semantic match'],
	historicalSuccessRate: 0.8,
	...overrides,
})

describe('matchWorkflowCandidates', () => {
	it('rejects candidates below the minimum score', () => {
		const result = matchWorkflowCandidates({
			candidates: [candidate({ score: 0.59 })],
			pageText: 'Services',
			minScore: 0.7,
		})

		expect(result.accepted).toEqual([])
		expect(result.rejected[0]?.reason).toBe('low_score')
	})

	it('rejects candidates when forbidden text is visible', () => {
		const result = matchWorkflowCandidates({
			candidates: [candidate()],
			pageText: 'Services Access denied',
			minScore: 0.7,
		})

		expect(result.accepted).toEqual([])
		expect(result.rejected[0]?.reason).toBe('forbidden_text')
	})

	it('prefers accepted candidates with higher historical success after score', () => {
		const result = matchWorkflowCandidates({
			candidates: [
				candidate({
					recipe: { ...candidate().recipe, id: 'lower-history' },
					score: 0.9,
					historicalSuccessRate: 0.4,
				}),
				candidate({
					recipe: { ...candidate().recipe, id: 'higher-history' },
					score: 0.9,
					historicalSuccessRate: 0.9,
				}),
			],
			pageText: 'Services',
			minScore: 0.7,
		})

		expect(result.accepted.map((match) => match.candidate.recipe.id)).toEqual([
			'higher-history',
			'lower-history',
		])
	})
})
