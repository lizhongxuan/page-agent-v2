import { describe, expect, it } from 'vitest'

import { formatWorkflowReplayStatus } from './replayStatus'

describe('formatWorkflowReplayStatus', () => {
	it('formats matched workflow score and reasons', () => {
		const message = formatWorkflowReplayStatus({
			kind: 'matched_workflow',
			candidate: {
				workflowId: 'wf_github_issue_search',
				version: 3,
				name: 'Search GitHub issues',
				finalScore: 0.913,
				riskLevel: 'read_or_search',
				variables: ['repo', 'query'],
				reasons: ['site matched', 'variables bindable'],
			},
		})

		expect(message).toContain('Search GitHub issues')
		expect(message).toContain('score=0.91')
		expect(message).toContain('site matched')
	})

	it('redacts sensitive binding values', () => {
		const message = formatWorkflowReplayStatus({
			kind: 'bindings_ready',
			bindings: {
				repo: { value: 'microsoft/playwright', source: 'slot', confidence: 0.95 },
				token: 'sk-secretvalue',
			},
		})

		expect(message).toContain('repo=microsoft/playwright (slot 0.95)')
		expect(message).toContain('token=[redacted]')
		expect(message).not.toContain('sk-secretvalue')
	})

	it('formats fallback and repair candidate status', () => {
		expect(
			formatWorkflowReplayStatus({
				kind: 'fallback',
				reason: 'binding_failed',
				message: 'required variable missing: query',
			})
		).toContain('binding_failed')
		expect(
			formatWorkflowReplayStatus({
				kind: 'repair_candidate',
				patchName: 'Use new Issues tab selector',
			})
		).toContain('Use new Issues tab selector')
	})
})
