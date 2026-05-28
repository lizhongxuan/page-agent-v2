import { type APIRequestContext, type Page, expect, test } from '@playwright/test'
import { resolve } from 'node:path'
import { pathToFileURL } from 'node:url'

const REPO_ROOT = resolve(import.meta.dirname, '..')
const FIXTURE_URL = pathToFileURL(
	resolve(REPO_ROOT, 'tests/fixtures/workflow-github-issues.html')
).toString()
const WORKFLOW_BACKEND_URL = process.env.WORKFLOW_BACKEND_URL

test.describe('Qdrant workflow retrieval fixture', () => {
	test('simulates GitHub issue search workflow behavior locally', async ({ page }) => {
		await openIssuesFixture(page, FIXTURE_URL)

		await expect(page.getByRole('heading', { name: 'playwright', exact: true })).toBeVisible()
		await expect(page.getByRole('link', { name: 'Code' })).toHaveAttribute('aria-current', 'page')
		await page.getByRole('link', { name: 'Issues' }).click()
		await expect(page.getByRole('link', { name: 'Issues' })).toHaveAttribute('aria-current', 'page')
		await expect(page.getByRole('heading', { name: 'Issues' })).toBeVisible()

		await page.getByPlaceholder('Search all issues').fill('is:issue timeout')
		await page.getByRole('button', { name: 'Search' }).click()

		await expect(page.getByTestId('search-state')).toHaveText(
			'Showing 2 issue results for "is:issue timeout".'
		)
		await expect(page.getByTestId('issue-results').getByRole('listitem')).toHaveCount(2)
		await expect(
			page.getByText('Timeout while waiting for locator in is:issue timeout')
		).toBeVisible()
		await expect(page.locator('body')).toHaveAttribute(
			'data-workflow-state',
			'issues-search-results'
		)
		await expect(page.locator('body')).toHaveAttribute('data-workflow-query', 'is:issue timeout')
	})

	test('keeps query submission deterministic when Enter submits the search form', async ({
		page,
	}) => {
		await openIssuesFixture(page, `${FIXTURE_URL}#issues`)

		await expect(page.getByRole('heading', { name: 'Issues' })).toBeVisible()
		await page.getByPlaceholder('Search all issues').fill('label:bug qdrant')
		await page.getByPlaceholder('Search all issues').press('Enter')

		await expect(page.getByTestId('search-state')).toHaveText(
			'Showing 2 issue results for "label:bug qdrant".'
		)
		await expect(page.getByTestId('issue-results')).toContainText(
			'Document flaky issue search workflow for label:bug qdrant'
		)
	})
})

test.describe('Qdrant workflow retrieval backend placeholder', () => {
	test.skip(
		!WORKFLOW_BACKEND_URL,
		'WORKFLOW_BACKEND_URL is not set; skipping backend retrieval API assertions.'
	)

	test('can call the future workflow search endpoint when a backend is provided', async ({
		request,
	}) => {
		const response = await postWorkflowSearch(request, {
			projectId: 'default',
			task: 'Search microsoft/playwright issues for timeout',
			currentUrl: 'https://github.com/microsoft/playwright',
			pageObservation: {
				title: 'microsoft/playwright',
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

		expect(response.ok()).toBe(true)
	})
})

async function openIssuesFixture(page: Page, url: string) {
	await page.goto(url, { waitUntil: 'domcontentloaded' })
}

async function postWorkflowSearch(request: APIRequestContext, body: Record<string, unknown>) {
	return request.post(`${backendUrl()}/api/retrieval/workflows/search`, { data: body })
}

function backendUrl() {
	if (!WORKFLOW_BACKEND_URL) {
		throw new Error('WORKFLOW_BACKEND_URL is required for backend workflow assertions')
	}
	return WORKFLOW_BACKEND_URL.replace(/\/+$/, '')
}
