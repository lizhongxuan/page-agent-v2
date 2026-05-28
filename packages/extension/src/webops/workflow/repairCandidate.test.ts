import { describe, expect, it } from 'vitest'

import type { RecordedSession } from '../recorder/actionEvents'
import { buildRepairPatchCandidateRequest } from './repairCandidate'

describe('buildRepairPatchCandidateRequest', () => {
	it('builds a pending repair patch candidate from a successful PageAgent fallback session', () => {
		const result = buildRepairPatchCandidateRequest(sampleFallbackSession(), {
			projectId: 'default',
			workflowId: 'wf_github_issue_search',
			version: 3,
			chunkId: 'search_issues',
			stepId: 'fill_query',
			fallbackReason: 'locator_not_found: searchbox',
			currentPageState: 'github_issues_list',
			currentUrl: 'https://github.com/microsoft/playwright/issues',
			runId: 'run_failed',
		})

		expect(result.ok).toBe(true)
		if (!result.ok) return
		expect(result.request.patch).toMatchObject({
			projectId: 'default',
			workflowId: 'wf_github_issue_search',
			workflowVersion: 3,
			chunkId: 'search_issues',
			stepId: 'fill_query',
			site: 'github.com',
			failureType: 'locator_not_found',
			appliesToPageStates: ['github_issues_list'],
			riskLevel: 'read_or_search',
			sourceRunId: 'run_failed',
		})
		expect(result.request.patch.newTarget.primary).toEqual({
			strategy: 'role',
			role: 'searchbox',
			name: 'Search all issues',
		})
		expect(result.request.patch.newTarget.fallbacks).toContainEqual({
			strategy: 'css',
			value: 'input[name="q"]',
		})
	})

	it('does not create a candidate when the fallback session has no stable successful target', () => {
		const session = sampleFallbackSession()
		session.steps = session.steps.map((step) => ({ ...step, target: undefined }))

		const result = buildRepairPatchCandidateRequest(session, {
			projectId: 'default',
			workflowId: 'wf_github_issue_search',
			version: 3,
			chunkId: 'search_issues',
			stepId: 'fill_query',
		})

		expect(result).toMatchObject({ ok: false, reason: 'missing_successful_target' })
	})
})

function sampleFallbackSession(): RecordedSession {
	return {
		id: 'fallback_session_1',
		task: 'Search timeout issues',
		startUrl: 'https://github.com/microsoft/playwright/issues',
		startedAt: 1,
		endedAt: 3,
		knowledgeHits: [],
		redactionReport: [],
		steps: [
			{
				id: 'fill_query',
				type: 'input',
				timestamp: 2,
				pageUrl: 'https://github.com/microsoft/playwright/issues',
				pageTitle: 'Issues',
				target: {
					role: 'searchbox',
					name: 'Search all issues',
					css: 'input[name="q"]',
				},
				value: 'timeout error',
				result: 'success',
			},
		],
	}
}
