import { type APIRequestContext, expect, test } from '@playwright/test'
import { resolve } from 'node:path'
import { pathToFileURL } from 'node:url'

const REPO_ROOT = resolve(import.meta.dirname, '..')
const FIXTURE_URL = pathToFileURL(
	resolve(REPO_ROOT, 'tests/fixtures/workflow-github-issues.html')
).toString()
const DIALOG_FIXTURE_URL = pathToFileURL(
	resolve(REPO_ROOT, 'tests/fixtures/workflow-github-issues-with-dialog.html')
).toString()
const WORKFLOW_BACKEND_URL = process.env.WORKFLOW_BACKEND_URL

test.describe('PageAgent Playwright workflow user guide acceptance', () => {
	test.skip(!WORKFLOW_BACKEND_URL, 'WORKFLOW_BACKEND_URL is required.')

	test('records a pending candidate, confirms enablement, retrieves it for a similar task, and replays it', async ({
		request,
		browserName,
	}) => {
		test.skip(browserName !== 'chromium', 'Chrome/Chromium is required for this acceptance path.')
		const projectId = `user-guide-${Date.now()}`
		const workflowId = `wf_user_guide_${Date.now()}`
		const candidate = await createCandidate(request, projectId, workflowId)
		expect(candidate.status).toBe('pending_review')
		expect(candidate.searchable).toBe(false)

		const beforeApproval = await searchWorkflows(
			request,
			projectId,
			'example/project-b',
			'user guide'
		)
		expect(beforeApproval.candidates).toHaveLength(0)

		const approved = await postJSON(request, `/api/workflow-candidates/${candidate.id}/approve`, {})
		expect(approved.status).toBe('active')
		expect(approved.searchable).toBe(true)

		const search = await searchWorkflows(request, projectId, 'example/project-b', 'user guide')
		expect(search.candidateSlots.repo).toBe('example/project-b')
		expect(search.candidateSlots.query).toContain('user guide')
		expect(search.candidates[0].workflowId).toBe(workflowId)

		const run = await postJSON(request, '/api/runs/start', {
			projectId,
			selectedWorkflowId: workflowId,
			version: 1,
			bindings: { repo: 'example/project-b', query: 'user guide' },
			currentUrl: FIXTURE_URL,
			pageObservation: { title: 'Fixture', visibleText: [], controls: [] },
		})
		expect(run.status).toBe('succeeded')

		const runLog = await getJSON(request, `/api/runs/${run.runId}`)
		expect(runLog.variables.query).toBe('[redacted]')
		expect(runLog.logs.some((entry: { event: string }) => entry.event === 'chunk_finished')).toBe(
			true
		)
	})

	test('closes an unexpected dialog during backend Playwright replay and keeps the same workflow page', async ({
		request,
		browserName,
	}) => {
		test.skip(browserName !== 'chromium', 'Chrome/Chromium is required for this acceptance path.')
		const projectId = `user-guide-dialog-${Date.now()}`
		const workflowId = `wf_user_guide_dialog_${Date.now()}`
		const candidate = await createCandidate(request, projectId, workflowId)
		await postJSON(request, `/api/workflow-candidates/${candidate.id}/approve`, {})

		const run = await postJSON(request, '/api/runs/start', {
			projectId,
			selectedWorkflowId: workflowId,
			version: 1,
			bindings: { repo: 'example/project-b', query: 'dialog user guide' },
			currentUrl: DIALOG_FIXTURE_URL,
			pageObservation: { title: 'Fixture', visibleText: [], controls: [] },
		})

		expect(run.status).toBe('succeeded')
		const runLog = await getJSON(request, `/api/runs/${run.runId}`)
		expect(
			runLog.logs.some((entry: { event: string }) => entry.event === 'interrupt_detected')
		).toBe(true)
		expect(
			runLog.logs.some((entry: { event: string }) => entry.event === 'interrupt_handler_applied')
		).toBe(true)
	})

	test('approves a repair patch candidate and reuses it on the next replay run', async ({
		request,
		browserName,
	}) => {
		test.skip(browserName !== 'chromium', 'Chrome/Chromium is required for this acceptance path.')
		const projectId = `user-guide-repair-${Date.now()}`
		const workflowId = `wf_user_guide_repair_${Date.now()}`
		const candidate = await createCandidateWithRecipe(
			request,
			projectId,
			workflowId,
			workflowRecipeWithBrokenSearchSelector(projectId, workflowId)
		)
		await postJSON(request, `/api/workflow-candidates/${candidate.id}/approve`, {})

		const patch = await postJSON(request, '/api/repair-patches/candidates', {
			patch: {
				projectId,
				workflowId,
				workflowVersion: 1,
				chunkId: 'search_issues',
				stepId: 'fill_query',
				site: 'github.com',
				failureType: 'locator_not_found',
				failureSignature: 'Old issue search placeholder is no longer visible',
				oldTarget: 'placeholder Old issue search',
				newTargetSummary: 'Current issue search input',
				newTarget: {
					primary: { strategy: 'css', value: 'input[name="q"]' },
				},
				appliesToPageStates: ['github_repo_home'],
				riskLevel: 'read_only',
			},
		})
		expect(patch.status).toBe('pending_review')

		const approvedPatch = await postJSON(request, `/api/repair-patches/${patch.id}/approve`, {})
		expect(approvedPatch.status).toBe('active')

		const run = await postJSON(request, '/api/runs/start', {
			projectId,
			selectedWorkflowId: workflowId,
			version: 1,
			bindings: { repo: 'example/project-b', query: 'patched user guide' },
			currentUrl: FIXTURE_URL,
			pageObservation: { title: 'Fixture', visibleText: [], controls: [] },
		})
		expect(run.status).toBe('succeeded')

		const runLog = await getJSON(request, `/api/runs/${run.runId}`)
		expect(
			runLog.logs.some((entry: { event: string }) => entry.event === 'repair_patch_applied')
		).toBe(true)
	})
})

async function createCandidate(request: APIRequestContext, projectId: string, workflowId: string) {
	return createCandidateWithRecipe(
		request,
		projectId,
		workflowId,
		workflowRecipe(projectId, workflowId)
	)
}

async function createCandidateWithRecipe(
	request: APIRequestContext,
	projectId: string,
	workflowId: string,
	recipe: Record<string, unknown>
) {
	return postJSON(request, '/api/workflow-candidates/from-session', {
		projectId,
		source: 'user_demo',
		task: 'Search example/project-a issues for recorded query',
		startUrl: 'https://github.com/example/project-a',
		recipe,
	})
}

async function searchWorkflows(
	request: APIRequestContext,
	projectId: string,
	repo: string,
	query: string
) {
	return postJSON(request, '/api/retrieval/workflows/search', {
		projectId,
		task: `Search ${repo} issues for ${query}`,
		currentUrl: `https://github.com/${repo}`,
		pageObservation: {
			title: repo,
			visibleText: ['Code', 'Issues', 'Pull requests'],
			controls: [{ role: 'link', name: 'Issues' }],
		},
		riskPolicy: {
			autoAllowed: ['read_only', 'read_or_search'],
			confirmationRequired: ['draft_change', 'external_send'],
			blocked: ['destructive'],
		},
		limit: 8,
	})
}

async function postJSON(request: APIRequestContext, path: string, data: Record<string, unknown>) {
	const response = await request.post(`${backendUrl()}${path}`, { data })
	expect(response.ok()).toBe(true)
	return response.json()
}

async function getJSON(request: APIRequestContext, path: string) {
	const response = await request.get(`${backendUrl()}${path}`)
	expect(response.ok()).toBe(true)
	return response.json()
}

function workflowRecipe(projectId: string, workflowId: string) {
	return {
		id: workflowId,
		version: 1,
		projectId,
		status: 'pending_review',
		searchable: false,
		site: 'github.com',
		app: 'github',
		name: 'Search GitHub issues',
		intent: 'Search issues in a GitHub repository',
		description: 'Open a repository Issues page and search by query.',
		tags: ['github', 'issues', 'search'],
		riskLevel: 'read_or_search',
		requiresConfirmation: false,
		variables: [
			{
				name: 'repo',
				type: 'string',
				required: true,
				source: 'task_or_url',
				examples: ['example/project-a'],
			},
			{
				name: 'query',
				type: 'string',
				required: true,
				source: 'task',
				examples: ['recorded query'],
			},
		],
		startPageStates: ['github_repo_home'],
		endPageStates: ['github_issues_search_results'],
		chunks: [
			{
				id: 'search_issues',
				name: 'Search Issues',
				fromPageState: 'github_repo_home',
				toPageState: 'github_issues_search_results',
				preconditionText: 'Code',
				postconditionText: 'Showing 2 issue results',
				stepSummary: 'Open Issues and search by query',
				riskLevel: 'read_or_search',
				variableNames: ['repo', 'query'],
				steps: [
					{
						id: 'click_issues',
						type: 'click',
						target: {
							primary: { strategy: 'role', role: 'link', name: 'Issues' },
							fallbacks: [{ strategy: 'text', value: 'Issues' }],
						},
						riskLevel: 'read_or_search',
					},
					{
						id: 'fill_query',
						type: 'fill',
						target: {
							primary: { strategy: 'placeholder', value: 'Search all issues' },
							fallbacks: [{ strategy: 'css', value: '[data-testid="issue-search"]' }],
						},
						value: '{{query}}',
						riskLevel: 'read_or_search',
					},
					{
						id: 'submit_search',
						type: 'click',
						target: {
							primary: { strategy: 'role', role: 'button', name: 'Search' },
						},
						riskLevel: 'read_or_search',
					},
				],
			},
		],
	}
}

function workflowRecipeWithBrokenSearchSelector(projectId: string, workflowId: string) {
	const recipe = workflowRecipe(projectId, workflowId)
	recipe.chunks[0].steps[1].target = {
		primary: { strategy: 'placeholder', value: 'Old issue search' },
	}
	return recipe
}

function backendUrl() {
	if (!WORKFLOW_BACKEND_URL) {
		throw new Error('WORKFLOW_BACKEND_URL is required')
	}
	return WORKFLOW_BACKEND_URL.replace(/\/+$/, '')
}
