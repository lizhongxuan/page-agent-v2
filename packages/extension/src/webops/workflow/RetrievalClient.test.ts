import { afterEach, describe, expect, it, vi } from 'vitest'

import { RetrievalClient } from './RetrievalClient'

const originalFetch = globalThis.fetch

describe('RetrievalClient', () => {
	afterEach(() => {
		globalThis.fetch = originalFetch
		vi.restoreAllMocks()
	})

	it('posts workflow search request with bearer auth and normalized base URL', async () => {
		const calls: { url: string | URL | Request; init?: RequestInit }[] = []
		vi.stubGlobal('fetch', async (url: string | URL | Request, init?: RequestInit) => {
			calls.push({ url, init })
			return jsonResponse({
				currentPageState: 'github_repo_home',
				candidates: [
					{
						workflowId: 'wf_github_issue_search',
						version: 3,
						name: 'Search GitHub issues',
						finalScore: 0.89,
						riskLevel: 'read_or_search',
						variables: ['repo', 'query'],
						score: {
							workflowDenseScore: 0.9,
							workflowSparseScore: 0.8,
							bestChunkScore: 0.88,
							pageStateScore: 0.91,
							variableBindability: 0.95,
							historicalSuccessRate: 0.94,
							selectorHealth: 0.9,
							recencyScore: 0.7,
							userPreferenceScore: 0.5,
							riskPenalty: 0,
							recentFailurePenalty: 0,
							finalScore: 0.89,
						},
						reasons: ['site matched'],
					},
				],
			})
		})

		const client = new RetrievalClient({
			baseUrl: 'https://workflow.example.test///',
			bearerToken: 'token-123',
		})
		const result = await client.searchWorkflows({
			projectId: 'default',
			task: 'search timeout',
			currentUrl: 'https://github.com/microsoft/playwright',
			pageObservation: {
				title: 'microsoft/playwright',
				visibleText: ['Code', 'Issues'],
				controls: [{ role: 'link', name: 'Issues' }],
			},
			riskPolicy: {
				autoAllowed: ['read_only', 'read_or_search'],
				confirmationRequired: ['draft_change', 'external_send'],
				blocked: ['destructive'],
			},
			limit: 8,
		})

		expect(calls).toHaveLength(1)
		expect(calls[0]?.url).toBe('https://workflow.example.test/api/retrieval/workflows/search')
		expect(calls[0]?.init?.method).toBe('POST')
		expect((calls[0]?.init?.headers as Record<string, string>)['content-type']).toBe(
			'application/json'
		)
		expect((calls[0]?.init?.headers as Record<string, string>).Authorization).toBe(
			'Bearer token-123'
		)
		expect(JSON.parse(calls[0]?.init?.body as string).currentUrl).toBe(
			'https://github.com/microsoft/playwright'
		)
		expect(result.ok).toBe(true)
		if (result.ok) {
			expect(result.data.candidates[0]?.workflowId).toBe('wf_github_issue_search')
		}
	})

	it('posts selector, interrupt, repair, repair candidate, and run requests to the expected endpoints', async () => {
		const urls: string[] = []
		vi.stubGlobal('fetch', async (url: string | URL | Request) => {
			urls.push(url instanceof Request ? url.url : String(url))
			return jsonResponse({})
		})

		const client = new RetrievalClient({ baseUrl: 'http://127.0.0.1:38402' })
		await client.selectWorkflow({
			projectId: 'default',
			task: 'search timeout',
			currentPageState: 'github_repo_home',
			candidates: [],
		})
		await client.searchInterrupts({
			projectId: 'default',
			workflowId: 'wf',
			version: 1,
			currentPageState: 'github_dialog',
			pageObservation: { title: 'Dialog', visibleText: [], controls: [] },
			riskPolicy: {
				autoAllowed: ['read_only', 'read_or_search'],
				confirmationRequired: ['draft_change', 'external_send'],
				blocked: ['destructive'],
			},
		})
		await client.searchRepairs({
			projectId: 'default',
			workflowId: 'wf',
			version: 1,
			chunkId: 'open_issues',
			stepId: 'click_issues',
			failureType: 'locator_not_found',
			currentPageState: 'github_repo_home',
			currentUrl: 'https://github.com/microsoft/playwright',
			pageObservation: { title: 'Repo', visibleText: [], controls: [] },
			oldTarget: { role: 'link', name: 'Issues' },
		})
		await client.createRepairPatchCandidate({
			projectId: 'default',
			workflowId: 'wf',
			workflowVersion: 1,
			chunkId: 'open_issues',
			stepId: 'click_issues',
			site: 'github.com',
			failureType: 'locator_not_found',
			failureSignature: 'old selector failed',
			oldTarget: 'role link Issues',
			newTargetSummary: 'new query field',
			newTarget: { primary: { strategy: 'css', value: '#new-query' } },
			riskLevel: 'read_only',
		})
		await client.approveRepairPatch('patch_1')
		await client.startRun({
			projectId: 'default',
			selectedWorkflowId: 'wf',
			version: 1,
			bindings: { query: 'timeout' },
			currentUrl: 'https://github.com/microsoft/playwright',
			pageObservation: { title: 'Repo', visibleText: [], controls: [] },
		})

		expect(urls).toEqual([
			'http://127.0.0.1:38402/api/retrieval/workflows/select',
			'http://127.0.0.1:38402/api/retrieval/interrupts/search',
			'http://127.0.0.1:38402/api/retrieval/repairs/search',
			'http://127.0.0.1:38402/api/repair-patches/candidates',
			'http://127.0.0.1:38402/api/repair-patches/patch_1/approve',
			'http://127.0.0.1:38402/api/runs/start',
		])
	})

	it('returns a typed failure for non-2xx responses', async () => {
		vi.stubGlobal(
			'fetch',
			async () =>
				new Response(JSON.stringify({ code: 'qdrant_unavailable', message: 'Qdrant is down' }), {
					status: 503,
					headers: { 'content-type': 'application/json' },
				})
		)

		const client = new RetrievalClient({ baseUrl: 'http://127.0.0.1:38402' })
		const result = await client.searchWorkflows(minimalWorkflowSearchRequest())

		expect(result).toEqual({
			ok: false,
			error: {
				code: 'qdrant_unavailable',
				message: 'Qdrant is down',
				status: 503,
				retryable: true,
			},
		})
	})

	it('returns a typed failure for network errors', async () => {
		vi.stubGlobal('fetch', async () => {
			throw new TypeError('Failed to fetch')
		})

		const client = new RetrievalClient({ baseUrl: 'http://127.0.0.1:38402' })
		const result = await client.searchWorkflows(minimalWorkflowSearchRequest())

		expect(result).toEqual({
			ok: false,
			error: {
				code: 'network_error',
				message: 'Failed to fetch',
				retryable: true,
			},
		})
	})

	it('surfaces nested backend error messages instead of a generic HTTP status', async () => {
		vi.stubGlobal(
			'fetch',
			async () =>
				new Response(
					JSON.stringify({
						error: {
							code: 'candidate_invalid',
							message: 'task is required',
						},
					}),
					{
						status: 400,
						headers: { 'content-type': 'application/json' },
					}
				)
		)

		const client = new RetrievalClient({ baseUrl: 'http://127.0.0.1:38402' })
		const result = await client.searchWorkflows(minimalWorkflowSearchRequest())

		expect(result).toEqual({
			ok: false,
			error: {
				code: 'candidate_invalid',
				message: 'task is required',
				status: 400,
				retryable: false,
			},
		})
	})
})

function jsonResponse(body: unknown): Response {
	return new Response(JSON.stringify(body), {
		status: 200,
		headers: { 'content-type': 'application/json' },
	})
}

function minimalWorkflowSearchRequest() {
	return {
		projectId: 'default',
		task: 'search timeout',
		currentUrl: 'https://github.com/microsoft/playwright',
		pageObservation: { title: 'Repo', visibleText: [], controls: [] },
		riskPolicy: {
			autoAllowed: ['read_only', 'read_or_search'] as const,
			confirmationRequired: ['draft_change', 'external_send'] as const,
			blocked: ['destructive'] as const,
		},
		limit: 8,
	}
}
