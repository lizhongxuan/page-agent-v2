import { afterEach, describe, expect, it, vi } from 'vitest'

import type { RecordedSession } from '../recorder/actionEvents'
import { RetrievalClient } from './RetrievalClient'
import { buildWorkflowCandidateRequest } from './candidateReview'

const originalFetch = globalThis.fetch

describe('buildWorkflowCandidateRequest', () => {
	it('builds a pending GitHub workflow candidate request and generalizes repo/query values', () => {
		const result = buildWorkflowCandidateRequest(githubIssueSearchSession(), {
			projectId: 'default',
		})

		expect(result.ok).toBe(true)
		if (!result.ok) return

		expect(result.request).toMatchObject({
			projectId: 'default',
			source: 'user_demo',
			task: 'Search timeout issues',
			startUrl: 'https://github.com/microsoft/playwright',
			recipe: {
				projectId: 'default',
				status: 'pending_review',
				searchable: false,
				site: 'github.com',
				app: 'github',
				riskLevel: 'read_or_search',
				requiresConfirmation: false,
			},
		})
		expect(result.request.recipe.id).toBe('wf_manual_github_microsoft_playwright_issues_search')
		expect(result.request.recipe.variables).toEqual(
			expect.arrayContaining([
				expect.objectContaining({
					name: 'repo',
					type: 'string',
					required: true,
					source: 'task_or_url',
					examples: ['microsoft/playwright'],
				}),
				expect.objectContaining({
					name: 'query',
					type: 'string',
					required: true,
					source: 'task',
					examples: ['timeout error'],
				}),
			])
		)
		expect(result.request.recipe.chunks[0]).toMatchObject({
			id: 'chunk_1',
			name: 'Recorded GitHub workflow',
			fromPageState: 'github_repo_home',
			toPageState: 'github_issues_list',
			riskLevel: 'read_or_search',
			variableNames: ['repo', 'query'],
		})
		expect(result.request.recipe.chunks[0]?.steps).toEqual([
			expect.objectContaining({
				id: 'step_1',
				type: 'click',
				target: {
					primary: { strategy: 'role', role: 'link', name: 'Issues' },
					fallbacks: [{ strategy: 'text', value: 'Issues' }],
				},
			}),
			expect.objectContaining({
				id: 'step_2',
				type: 'fill',
				value: '{{query}}',
				target: {
					primary: { strategy: 'role', role: 'searchbox', name: 'Search all issues' },
					fallbacks: [{ strategy: 'css', value: 'input[name="q"]' }],
				},
			}),
			expect.objectContaining({
				id: 'step_3',
				type: 'press',
				key: 'Enter',
			}),
		])
	})

	it('does not generate an auto-executable recipe for high-risk recorded actions', () => {
		const session = githubIssueSearchSession()
		session.steps.push({
			id: 'delete_repo',
			type: 'click',
			timestamp: 4,
			pageUrl: 'https://github.com/microsoft/playwright/settings',
			pageTitle: 'Settings',
			target: { role: 'button', name: 'Delete this repository' },
			result: 'success',
		})

		const result = buildWorkflowCandidateRequest(session, { projectId: 'default' })

		expect(result).toEqual({
			ok: false,
			reason: 'high_risk_action',
			message:
				'High-risk recorded actions require manual recipe authoring before replay can be enabled.',
		})
	})

	it('generalizes GitHub repo from the task when the recorded page is a local fixture', () => {
		const session = githubIssueSearchSession()
		session.task = '在 github.com/alibaba/page-agent 的 Issues 里搜索 startsWith 报错'
		session.startUrl = 'http://127.0.0.1:49152/workflow-github-issues.html'
		session.steps = session.steps.map((step) => ({
			...step,
			pageUrl: 'http://127.0.0.1:49152/workflow-github-issues.html',
		}))

		const result = buildWorkflowCandidateRequest(session, { projectId: 'default' })

		expect(result.ok).toBe(true)
		if (!result.ok) return
		expect(result.request.recipe.site).toBe('127.0.0.1')
		expect(result.request.recipe.variables).toEqual(
			expect.arrayContaining([
				expect.objectContaining({
					name: 'repo',
					examples: ['alibaba/page-agent'],
				}),
			])
		)
	})

	it('infers a useful task description when the user records without typing a prompt', () => {
		const session = githubIssueSearchSession()
		session.task = ''
		session.startUrl = 'https://github.com/browser-use/workflow-use'

		const result = buildWorkflowCandidateRequest(session, { projectId: 'default' })

		expect(result.ok).toBe(true)
		if (!result.ok) return
		expect(result.request.task).toContain('browser-use/workflow-use')
		expect(result.request.task).toContain('timeout error')
		expect(result.request.recipe.name).toContain('browser-use/workflow-use')
		expect(result.request.recipe.intent).toBe(result.request.task)
	})

	it('rejects recordings whose successful DOM actions do not have replayable targets', () => {
		const session = githubIssueSearchSession()
		session.steps = [
			{
				id: 'unstable_click',
				type: 'click',
				timestamp: 1,
				pageUrl: 'https://github.com/browser-use/workflow-use/actions',
				pageTitle: 'Actions',
				result: 'success',
			},
		]

		const result = buildWorkflowCandidateRequest(session, { projectId: 'default' })

		expect(result).toEqual({
			ok: false,
			reason: 'unsupported_session',
			message: 'Recorded session does not contain supported replay actions.',
		})
	})

	it('rejects recordings that only contain wait steps', () => {
		const session = githubIssueSearchSession()
		session.steps = [
			{
				id: 'wait_for_page',
				type: 'wait',
				timestamp: 1,
				pageUrl: 'https://github.com/browser-use/workflow-use/actions',
				pageTitle: 'Actions',
				result: 'success',
			},
		]

		const result = buildWorkflowCandidateRequest(session, { projectId: 'default' })

		expect(result).toEqual({
			ok: false,
			reason: 'unsupported_session',
			message: 'Recorded session does not contain supported replay actions.',
		})
	})

	it('does not persist unsafe generic css selectors as replay fallbacks', () => {
		const session = githubIssueSearchSession()
		session.steps[0]!.target = {
			text: 'Issues',
			css: 'span',
		}

		const result = buildWorkflowCandidateRequest(session, { projectId: 'default' })

		expect(result.ok).toBe(true)
		if (!result.ok) return
		expect(result.request.recipe.chunks[0]?.steps[0]?.target).toEqual({
			primary: { strategy: 'text', value: 'Issues' },
		})
	})
})

describe('RetrievalClient workflow candidate APIs', () => {
	afterEach(() => {
		globalThis.fetch = originalFetch
		vi.restoreAllMocks()
	})

	it('creates, lists, approves, and rejects workflow candidates through backend paths', async () => {
		const calls: { url: string; init?: RequestInit }[] = []
		vi.stubGlobal('fetch', async (url: string | URL | Request, init?: RequestInit) => {
			const normalizedUrl = url instanceof Request ? url.url : String(url)
			calls.push({ url: normalizedUrl, init })
			return jsonResponse(
				normalizedUrl.endsWith('/api/workflow-candidates')
					? { candidates: [{ id: 'cand_1', status: 'pending_review' }] }
					: { id: 'cand_1', status: 'pending_review' },
				normalizedUrl.endsWith('/from-session') ? 201 : 200
			)
		})

		const request = buildWorkflowCandidateRequest(githubIssueSearchSession(), {
			projectId: 'default',
		})
		expect(request.ok).toBe(true)
		if (!request.ok) return

		const client = new RetrievalClient({
			baseUrl: 'http://127.0.0.1:38422/',
			bearerToken: 'token-123',
		})

		const created = await client.createWorkflowCandidateFromSession(request.request)
		const listed = await client.listWorkflowCandidates({
			projectId: 'default',
			source: 'user_demo',
			reviewStatus: 'pending_review',
		})
		const approved = await client.approveWorkflowCandidate('cand_1')
		const rejected = await client.rejectWorkflowCandidate('cand_1')

		expect(created.ok).toBe(true)
		expect(listed.ok).toBe(true)
		expect(approved.ok).toBe(true)
		expect(rejected.ok).toBe(true)
		expect(calls.map((call) => `${call.init?.method ?? 'GET'} ${call.url}`)).toEqual([
			'POST http://127.0.0.1:38422/api/workflow-candidates/from-session',
			'GET http://127.0.0.1:38422/api/workflow-candidates?projectId=default&source=user_demo&reviewStatus=pending_review',
			'POST http://127.0.0.1:38422/api/workflow-candidates/cand_1/approve',
			'POST http://127.0.0.1:38422/api/workflow-candidates/cand_1/reject',
		])
		expect((calls[0]?.init?.headers as Record<string, string>).Authorization).toBe(
			'Bearer token-123'
		)
	})

	it('returns a retryable workflow result when the candidate backend is unavailable', async () => {
		vi.stubGlobal('fetch', async () => {
			throw new TypeError('Failed to fetch')
		})

		const client = new RetrievalClient({ baseUrl: 'http://127.0.0.1:38422' })
		const result = await client.listWorkflowCandidates({ projectId: 'default' })

		expect(result).toEqual({
			ok: false,
			error: {
				code: 'network_error',
				message: 'Failed to fetch',
				retryable: true,
			},
		})
	})
})

function githubIssueSearchSession(): RecordedSession {
	return {
		id: 'manual_session_1',
		task: 'Search timeout issues',
		startUrl: 'https://github.com/microsoft/playwright',
		startedAt: 1,
		endedAt: 4,
		knowledgeHits: [],
		redactionReport: [],
		steps: [
			{
				id: 'click_issues',
				type: 'click',
				timestamp: 1,
				pageUrl: 'https://github.com/microsoft/playwright',
				pageTitle: 'microsoft/playwright',
				target: { role: 'link', name: 'Issues', text: 'Issues' },
				result: 'success',
			},
			{
				id: 'search_issues',
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
			{
				id: 'submit_search',
				type: 'input',
				timestamp: 3,
				pageUrl: 'https://github.com/microsoft/playwright/issues',
				pageTitle: 'Issues',
				target: {
					role: 'searchbox',
					name: 'Search all issues',
					css: 'input[name="q"]',
				},
				value: 'Enter',
				note: 'keydown',
				result: 'success',
			},
		],
	}
}

function jsonResponse(body: unknown, status = 200): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'content-type': 'application/json' },
	})
}
