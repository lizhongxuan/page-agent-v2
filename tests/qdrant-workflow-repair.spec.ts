import { type APIRequestContext, type Page, expect, test } from '@playwright/test'
import { resolve } from 'node:path'
import { pathToFileURL } from 'node:url'

const REPO_ROOT = resolve(import.meta.dirname, '..')
const DIALOG_FIXTURE_URL = pathToFileURL(
	resolve(REPO_ROOT, 'tests/fixtures/workflow-github-issues-with-dialog.html')
).toString()
const WORKFLOW_BACKEND_URL = process.env.WORKFLOW_BACKEND_URL

test.describe('Qdrant workflow interrupt and repair fixture', () => {
	test('blocks issue search until the New feature dialog is closed', async ({ page }) => {
		await openDialogFixture(page)

		await expect(page.getByRole('dialog', { name: 'New feature' })).toBeVisible()
		await expect(page.locator('body')).toHaveAttribute('data-dialog-state', 'open')
		await expect(page.locator('#page-content')).toHaveAttribute('aria-hidden', 'true')

		await page.getByTestId('dialog-got-it').click()
		await expect(page.getByRole('dialog', { name: 'New feature' })).toHaveCount(0)
		await expect(page.locator('body')).toHaveAttribute('data-dialog-state', 'closed')
		await expect(page.locator('#page-content')).not.toHaveAttribute('aria-hidden', 'true')

		await page.getByRole('link', { name: 'Issues' }).click()
		await page.getByPlaceholder('Search all issues').fill('regression dialog')
		await page.getByRole('button', { name: 'Search' }).click()

		await expect(page.getByTestId('search-state')).toHaveText(
			'Showing 2 issue results for "regression dialog".'
		)
		await expect(page.locator('body')).toHaveAttribute(
			'data-workflow-state',
			'issues-search-results'
		)
	})

	test('allows either dialog action to unblock the same workflow path', async ({ page }) => {
		await openDialogFixture(page)

		await page.getByTestId('dialog-close').click()
		await expect(page.getByRole('dialog', { name: 'New feature' })).toHaveCount(0)

		await page.getByRole('link', { name: 'Issues' }).click()
		await page.getByPlaceholder('Search all issues').fill('selector repair')
		await page.getByRole('button', { name: 'Search' }).click()

		await expect(page.getByTestId('issue-results')).toContainText(
			'Timeout while waiting for locator in selector repair'
		)
	})
})

test.describe('Qdrant workflow repair backend placeholder', () => {
	test.skip(
		!WORKFLOW_BACKEND_URL,
		'WORKFLOW_BACKEND_URL is not set; skipping backend interrupt and repair API assertions.'
	)

	test('can call future interrupt and repair search endpoints when a backend is provided', async ({
		request,
	}) => {
		const interruptResponse = await postInterruptSearch(request, {
			projectId: 'default',
			site: 'github.com',
			currentPageState: 'github_repo_home',
			visibleText: ['New feature', 'Got it', 'Close'],
			controls: [
				{ role: 'button', name: 'Got it' },
				{ role: 'button', name: 'Close' },
			],
			limit: 3,
		})
		expect(interruptResponse.ok()).toBe(true)

		const repairResponse = await postRepairSearch(request, {
			projectId: 'default',
			workflowId: 'wf_github_issue_search',
			chunkId: 'search_issues',
			stepId: 'fill_query',
			failureType: 'locator_not_found',
			currentPageState: 'github_issues_list',
			url: 'https://github.com/microsoft/playwright/issues',
			visibleText: ['Issues', 'Search all issues'],
			controls: [{ role: 'searchbox', name: 'Search all issues' }],
			oldTarget: "getByRole('textbox', { name: /search/i })",
			limit: 3,
		})
		expect(repairResponse.ok()).toBe(true)
	})
})

async function openDialogFixture(page: Page) {
	await page.goto(DIALOG_FIXTURE_URL, { waitUntil: 'domcontentloaded' })
}

async function postInterruptSearch(request: APIRequestContext, body: Record<string, unknown>) {
	return request.post(`${backendUrl()}/api/retrieval/interrupts/search`, { data: body })
}

async function postRepairSearch(request: APIRequestContext, body: Record<string, unknown>) {
	return request.post(`${backendUrl()}/api/retrieval/repairs/search`, { data: body })
}

function backendUrl() {
	if (!WORKFLOW_BACKEND_URL) {
		throw new Error('WORKFLOW_BACKEND_URL is required for backend workflow assertions')
	}
	return WORKFLOW_BACKEND_URL.replace(/\/+$/, '')
}
