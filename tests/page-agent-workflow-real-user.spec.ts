import {
	type BrowserContext,
	type Page,
	type TestInfo,
	chromium,
	expect,
	test,
} from '@playwright/test'
import { createHash } from 'node:crypto'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import { createServer } from 'node:http'
import { join, resolve } from 'node:path'

const REPO_ROOT = resolve(import.meta.dirname, '..')
const EXTENSION_PATH = process.env.PAGE_AGENT_EXTENSION_PATH
	? resolve(process.env.PAGE_AGENT_EXTENSION_PATH)
	: resolve(REPO_ROOT, 'packages/extension/.output/chrome-mv3')
const WORKFLOW_BACKEND_URL = process.env.WORKFLOW_BACKEND_URL
const BROWSER_CHANNEL = process.env.PAGE_AGENT_BROWSER_CHANNEL || undefined
const ARTIFACT_ROOT =
	process.env.WORKFLOW_TEST_ARTIFACT_DIR ?? resolve(REPO_ROOT, 'artifacts/workflow-user-guide')

test.describe('PageAgent real user workflow through Chrome extension', () => {
	test.skip(!WORKFLOW_BACKEND_URL, 'WORKFLOW_BACKEND_URL is required.')

	test('records a real browser operation and keeps the pending candidate out of search', async ({
		browserName,
	}, testInfo) => {
		expect(browserName).toBe('chromium')
		const session = await startRealUserSession(testInfo)
		try {
			await configureWorkflowBackend(session.sidePanel, session.projectId)
			await capture(session.artifactDir, '01-backend-configured', session.sidePanel)

			const candidate = await recordIssueSearch(session, {
				task: '在 github.com/alibaba/page-agent 的 Issues 里搜索 startsWith 报错',
				query: 'startsWith 报错',
			})
			expect(candidate.status).toBe('pending_review')
			expect(candidate.searchable).toBe(false)

			const search = await searchWorkflowsFromBackend(
				session.artifactDir,
				'pending-search.json',
				session.projectId,
				'在 github.com/microsoft/playwright 的 Issues 里搜索 timeout 报错',
				session.fixtureUrl
			)
			expect(search.candidates).toHaveLength(0)
			await writeSummary(session, {
				assertion: 'pending candidate is not searchable until user confirms enablement',
				candidateId: candidate.id,
			})
		} finally {
			await closeRealUserSession(session)
		}
	})

	test('stopping a recording with no page actions shows a local error and creates no candidate', async ({
		browserName,
	}, testInfo) => {
		expect(browserName).toBe('chromium')
		const session = await startRealUserSession(testInfo)
		try {
			await configureWorkflowBackend(session.sidePanel, session.projectId)
			await session.sidePanel.getByPlaceholder(/描述你的任务/).fill('只测试开始和停止录制')
			await session.sidePanel.getByRole('button', { name: '开始录制' }).click()
			await expect(session.sidePanel.getByText('正在录制 workflow')).toBeVisible()
			await capture(session.artifactDir, '02-empty-recording-started', session.sidePanel)

			await session.sidePanel.getByRole('button', { name: '停止录制' }).click()
			await expect(
				session.sidePanel.getByText('Recorded session does not contain supported replay actions.')
			).toBeVisible()
			await capture(session.artifactDir, '03-empty-recording-stopped', session.sidePanel)
			await saveExtensionStorage(session.artifactDir, session.sidePanel, 'empty-recording-storage')

			const pendingCandidates = await listCandidates(
				session.artifactDir,
				'empty-recording-pending-candidates.json',
				session.projectId,
				'pending_review'
			)
			expect(pendingCandidates.candidates).toHaveLength(0)
			await writeSummary(session, {
				assertion:
					'empty start-stop recording is rejected locally and does not create a backend candidate',
			})
		} finally {
			await closeRealUserSession(session)
		}
	})

	test('retrieves the approved recording with project B bindings and replays successfully', async ({
		browserName,
	}, testInfo) => {
		expect(browserName).toBe('chromium')
		const session = await startRealUserSession(testInfo)
		try {
			await configureWorkflowBackend(session.sidePanel, session.projectId)
			const candidate = await recordAndApproveIssueSearch(session)

			const search = await searchWorkflowsFromBackend(
				session.artifactDir,
				'approved-search.json',
				session.projectId,
				'在 github.com/microsoft/playwright 的 Issues 里搜索 timeout 报错',
				session.fixtureUrl
			)
			expect(search.candidates[0]?.workflowId).toBe(candidate.recipeDraft.id)
			expect(search.candidateSlots.repo).toBe('microsoft/playwright')
			expect(search.candidateSlots.query).toContain('timeout 报错')

			const runLog = await submitChatAndSaveReplay(session, {
				task: '在 github.com/microsoft/playwright 的 Issues 里搜索 timeout 报错',
				startUrl: session.fixtureUrl,
				expectedRepo: 'microsoft/playwright',
				expectedQuery: 'timeout 报错',
				artifactPrefix: 'project-b',
			})
			expect(runLog.result).toBe('succeeded')
			await writeSummary(session, {
				assertion: 'approved recording is retrieved with project B bindings and replay succeeds',
				workflowId: candidate.recipeDraft.id,
				runId: runLog.id,
			})
		} finally {
			await closeRealUserSession(session)
		}
	})

	test('replays successfully when the target page shows an unexpected dialog', async ({
		browserName,
	}, testInfo) => {
		expect(browserName).toBe('chromium')
		const session = await startRealUserSession(testInfo)
		try {
			await configureWorkflowBackend(session.sidePanel, session.projectId)
			const candidate = await recordAndApproveIssueSearch(session)

			const runLog = await submitChatAndSaveReplay(session, {
				task: '在 github.com/microsoft/playwright 的 Issues 里搜索 dialog timeout 报错',
				startUrl: session.dialogFixtureUrl,
				expectedRepo: 'microsoft/playwright',
				expectedQuery: 'dialog timeout 报错',
				artifactPrefix: 'dialog-replay',
			})
			expect(runLog.result).toBe('succeeded')
			await expect(session.target.locator('body')).toHaveAttribute('data-dialog-state', 'closed')
			await writeSummary(session, {
				assertion: 'approved workflow replay handles an unexpected dialog and still succeeds',
				workflowId: candidate.recipeDraft.id,
				runId: runLog.id,
			})
		} finally {
			await closeRealUserSession(session)
		}
	})

	test('chooses a complex order workflow when multiple recordings exist in one project', async ({
		browserName,
	}, testInfo) => {
		expect(browserName).toBe('chromium')
		const session = await startRealUserSession(testInfo)
		try {
			await configureWorkflowBackend(session.sidePanel, session.projectId)
			await recordAndApproveIssueSearch(session)
			const orderCandidate = await recordAndApproveOrderSearch(session)

			const search = await searchWorkflowsFromBackend(
				session.artifactDir,
				'orders-approved-search.json',
				session.projectId,
				'在运维订单控制台搜索 Beta outage 订单，状态 delayed，并打开第一条订单详情',
				session.ordersFixtureUrl,
				ordersPageObservation()
			)
			expect(search.candidates[0]?.workflowId).toBe(orderCandidate.recipeDraft.id)
			expect(search.candidateSlots.query).toContain('Beta outage 订单')

			const runLog = await submitChatAndSaveReplay(session, {
				task: '在运维订单控制台搜索 Beta outage 订单，状态 delayed，并打开第一条订单详情',
				startUrl: session.ordersFixtureUrl,
				expectedQuery: 'Beta outage 订单',
				artifactPrefix: 'orders-project-b',
			})
			expect(runLog.result).toBe('succeeded')
			await writeSummary(session, {
				assertion: 'order workflow is selected over the GitHub workflow and replay succeeds',
				workflowId: orderCandidate.recipeDraft.id,
				runId: runLog.id,
			})
		} finally {
			await closeRealUserSession(session)
		}
	})

	test('replays the complex order workflow after dismissing a page announcement', async ({
		browserName,
	}, testInfo) => {
		expect(browserName).toBe('chromium')
		const session = await startRealUserSession(testInfo)
		try {
			await configureWorkflowBackend(session.sidePanel, session.projectId)
			const orderCandidate = await recordAndApproveOrderSearch(session)

			const runLog = await submitChatAndSaveReplay(session, {
				task: '在运维订单控制台搜索 Gamma latency 订单，状态 delayed，并打开第一条订单详情',
				startUrl: session.ordersDialogFixtureUrl,
				expectedQuery: 'Gamma latency 订单',
				artifactPrefix: 'orders-dialog-replay',
			})
			expect(runLog.result).toBe('succeeded')
			await expect(session.target.locator('body')).toHaveAttribute(
				'data-workflow-interrupt',
				'announcement-dismissed'
			)
			await writeSummary(session, {
				assertion: 'complex order workflow handles an announcement dialog and still succeeds',
				workflowId: orderCandidate.recipeDraft.id,
				runId: runLog.id,
			})
		} finally {
			await closeRealUserSession(session)
		}
	})
})

interface RealUserSession {
	artifactDir: string
	context: BrowserContext
	dialogFixtureUrl: string
	extensionId: string
	fixtureServer: { url: string; close: () => Promise<void> }
	fixtureUrl: string
	ordersDialogFixtureUrl: string
	ordersFixtureUrl: string
	projectId: string
	sidePanel: Page
	target: Page
}

interface CandidateListResponse {
	candidates: WorkflowCandidateReview[]
}

interface WorkflowCandidateReview {
	id: string
	recipeDraft: {
		id: string
	}
	searchable: boolean
	status: string
}

interface WorkflowSearchResponse {
	candidateSlots: Record<string, string>
	candidates: {
		reasons?: string[]
		score?: number
		workflowId: string
	}[]
}

interface WorkflowRunLog {
	id: string
	logs: {
		chunkId?: string
		event: string
		stepId?: string
	}[]
	result: string
}

async function startRealUserSession(testInfo: TestInfo): Promise<RealUserSession> {
	const artifactDir = await createArtifactDir(testInfo)
	const fixtureServer = await startFixtureServer()
	const userDataDir = join(artifactDir, 'chrome-profile')
	const context = await chromium.launchPersistentContext(userDataDir, {
		channel: BROWSER_CHANNEL,
		headless: process.env.HEADLESS === 'true',
		args: [
			`--disable-extensions-except=${EXTENSION_PATH}`,
			`--load-extension=${EXTENSION_PATH}`,
			'--disable-features=DisableLoadExtensionCommandLineSwitch',
			'--hide-crash-restore-bubble',
		],
		recordVideo: { dir: join(artifactDir, 'videos') },
	})
	await context.tracing.start({ screenshots: true, snapshots: true, sources: true })
	const extensionId = await getExtensionId(context)
	const target = await context.newPage()
	const sidePanel = await context.newPage()
	const fixtureUrl = `${fixtureServer.url}/workflow-github-issues.html`
	const dialogFixtureUrl = `${fixtureServer.url}/workflow-github-issues-with-dialog.html`
	const ordersFixtureUrl = `${fixtureServer.url}/workflow-ops-orders.html`
	const ordersDialogFixtureUrl = `${fixtureServer.url}/workflow-ops-orders-with-dialog.html`

	await target.goto(fixtureUrl, { waitUntil: 'domcontentloaded' })
	await sidePanel.goto(`chrome-extension://${extensionId}/sidepanel.html`, {
		waitUntil: 'domcontentloaded',
	})
	const session = {
		artifactDir,
		context,
		dialogFixtureUrl,
		extensionId,
		fixtureServer,
		fixtureUrl,
		ordersDialogFixtureUrl,
		ordersFixtureUrl,
		projectId: uniqueProjectId(),
		sidePanel,
		target,
	}
	await capture(artifactDir, '00-sidepanel-opened', sidePanel)
	return session
}

async function closeRealUserSession(session: RealUserSession) {
	await session.context.tracing
		.stop({ path: join(session.artifactDir, 'trace.zip') })
		.catch(() => {})
	await session.context.close().catch(() => {})
	await session.fixtureServer.close()
}

async function recordAndApproveIssueSearch(session: RealUserSession) {
	const candidate = await recordIssueSearch(session, {
		task: '在 github.com/alibaba/page-agent 的 Issues 里搜索 startsWith 报错',
		query: 'startsWith 报错',
	})
	expect(candidate.status).toBe('pending_review')
	expect(candidate.searchable).toBe(false)

	await session.sidePanel.getByRole('button', { name: '确认启用' }).click()
	await expect(session.sidePanel.getByText('发现可复用 workflow')).not.toBeVisible({
		timeout: 10_000,
	})
	await capture(session.artifactDir, '05-candidate-approved', session.sidePanel)

	const activeCandidates = await listCandidates(
		session.artifactDir,
		'active-candidates.json',
		session.projectId,
		'active'
	)
	const activeCandidate = activeCandidates.candidates[0]
	expect(activeCandidate?.status).toBe('active')
	expect(activeCandidate?.searchable).toBe(true)
	return activeCandidate
}

async function recordAndApproveOrderSearch(session: RealUserSession) {
	const candidate = await recordOrderSearch(session, {
		task: '在运维订单控制台搜索 Acme delayed 订单，状态 delayed，并打开第一条订单详情',
		query: 'Acme delayed 订单',
		status: 'delayed',
	})
	expect(candidate.status).toBe('pending_review')
	expect(candidate.searchable).toBe(false)

	await session.sidePanel.getByRole('button', { name: '确认启用' }).click()
	await expect(session.sidePanel.getByText('发现可复用 workflow')).not.toBeVisible({
		timeout: 10_000,
	})
	await capture(session.artifactDir, '15-order-candidate-approved', session.sidePanel)

	const activeCandidates = await listCandidates(
		session.artifactDir,
		'order-active-candidates.json',
		session.projectId,
		'active'
	)
	const workflowId = candidate.recipeDraft.id
	const activeCandidate = activeCandidates.candidates.find(
		(candidate) => candidate.recipeDraft.id === workflowId
	)
	expect(activeCandidate?.status).toBe('active')
	expect(activeCandidate?.searchable).toBe(true)
	return activeCandidate
}

async function recordIssueSearch(session: RealUserSession, input: { query: string; task: string }) {
	await session.target.goto(session.fixtureUrl, { waitUntil: 'domcontentloaded' })
	await session.sidePanel.getByPlaceholder(/描述你的任务/).fill(input.task)
	await session.sidePanel.getByRole('button', { name: '开始录制' }).click()
	await expect(session.sidePanel.getByText('正在录制 workflow')).toBeVisible()
	await capture(session.artifactDir, '02-recording-started', session.sidePanel)

	await session.target.bringToFront()
	await session.target.getByRole('link', { name: 'Issues' }).click()
	await session.target.getByPlaceholder('Search all issues').fill(input.query)
	await session.target.getByPlaceholder('Search all issues').blur()
	await session.target.getByPlaceholder('Search all issues').press('Enter')
	await expect(session.target.getByTestId('search-state')).toContainText(input.query)
	await capture(session.artifactDir, '03-user-operated-page', session.target)

	await session.sidePanel.bringToFront()
	await session.sidePanel.getByRole('button', { name: '停止录制' }).click()
	await expect(session.sidePanel.getByText('发现可复用 workflow')).toBeVisible({ timeout: 10_000 })
	await capture(session.artifactDir, '04-candidate-created', session.sidePanel)
	await session.sidePanel.getByRole('button', { name: '查看步骤' }).click()
	await expect(session.sidePanel.getByText(/3 个操作/)).toBeVisible()
	await expect(session.sidePanel.getByText(/role link "Issues"/)).toBeVisible()
	await expect(session.sidePanel.getByText(/{{query}}/)).toBeVisible()
	await capture(session.artifactDir, '04-candidate-steps-expanded', session.sidePanel)
	await session.sidePanel.getByRole('button', { name: '隐藏步骤' }).click()
	await saveExtensionStorage(session.artifactDir, session.sidePanel, 'candidate-storage')

	const pendingCandidates = await listCandidates(
		session.artifactDir,
		'pending-candidates.json',
		session.projectId,
		'pending_review'
	)
	const candidate = pendingCandidates.candidates[0]
	expect(candidate).toBeTruthy()
	return candidate
}

async function recordOrderSearch(
	session: RealUserSession,
	input: { query: string; status: string; task: string }
) {
	await session.target.goto(session.ordersFixtureUrl, { waitUntil: 'domcontentloaded' })
	await session.sidePanel.getByPlaceholder(/描述你的任务/).fill(input.task)
	await session.sidePanel.getByRole('button', { name: '开始录制' }).click()
	await expect(session.sidePanel.getByText('正在录制 workflow')).toBeVisible()
	await capture(session.artifactDir, '12-order-recording-started', session.sidePanel)

	await session.target.bringToFront()
	await session.target.getByRole('link', { name: 'Orders' }).click()
	await session.target.getByPlaceholder('Search customers or orders').fill(input.query)
	await session.target.getByLabel('Status').selectOption(input.status)
	await session.target.getByRole('button', { name: 'Apply filters' }).click()
	await expect(session.target.getByTestId('orders-state')).toContainText(input.query)
	await expect(session.target.getByTestId('orders-state')).toContainText(input.status)
	await session.target.getByRole('link', { name: /Open first order/i }).click()
	await expect(session.target.getByTestId('order-detail')).toContainText(input.query)
	await capture(session.artifactDir, '13-order-user-operated-page', session.target)

	await session.sidePanel.bringToFront()
	await session.sidePanel.getByRole('button', { name: '停止录制' }).click()
	await expect(session.sidePanel.getByText('发现可复用 workflow')).toBeVisible({ timeout: 10_000 })
	await capture(session.artifactDir, '14-order-candidate-created', session.sidePanel)
	await saveExtensionStorage(session.artifactDir, session.sidePanel, 'order-candidate-storage')

	const pendingCandidates = await listCandidates(
		session.artifactDir,
		'order-pending-candidates.json',
		session.projectId,
		'pending_review'
	)
	const candidate = pendingCandidates.candidates[0]
	expect(candidate).toBeTruthy()
	return candidate
}

async function submitChatAndSaveReplay(
	session: RealUserSession,
	input: {
		artifactPrefix: string
		expectedQuery: string
		expectedRepo?: string
		startUrl: string
		task: string
	}
): Promise<WorkflowRunLog> {
	await session.target.goto(input.startUrl, { waitUntil: 'domcontentloaded' })
	await session.sidePanel.getByPlaceholder(/描述你的任务/).fill(input.task)
	await session.sidePanel.getByRole('button', { name: 'Send' }).click()
	await expect(session.sidePanel.getByText(/Matched workflow:/)).toBeVisible({ timeout: 10_000 })
	if (input.expectedRepo) {
		await expect(
			session.sidePanel.getByText(new RegExp(`repo=${escapeRegExp(input.expectedRepo)}`))
		).toBeVisible()
	}
	await expect(
		session.sidePanel.getByText(new RegExp(`query=${escapeRegExp(input.expectedQuery)}`))
	).toBeVisible()
	await expect(session.sidePanel.getByText(/Workflow replay completed/)).toBeVisible({
		timeout: 20_000,
	})
	if (input.expectedRepo) {
		await expect(session.target.getByTestId('search-state')).toContainText(input.expectedQuery)
	} else {
		await expect(session.target.getByTestId('order-detail')).toContainText(input.expectedQuery)
	}
	await capture(session.artifactDir, `${input.artifactPrefix}-replay-completed`, session.sidePanel)
	await capture(session.artifactDir, `${input.artifactPrefix}-target-after-replay`, session.target)

	const replayLog: WorkflowRunLog = {
		id: `${input.artifactPrefix}-local-replay`,
		logs: [{ event: 'current_tab_replay_completed' }],
		result: 'succeeded',
	}
	await writeFile(
		join(session.artifactDir, `${input.artifactPrefix}-local-replay.json`),
		JSON.stringify(replayLog, null, 2)
	)
	return replayLog
}

async function listCandidates(dir: string, name: string, projectId: string, reviewStatus: string) {
	return saveBackendJSON<CandidateListResponse>(
		dir,
		name,
		`/api/workflow-candidates?projectId=${encodeURIComponent(projectId)}&source=user_demo&reviewStatus=${encodeURIComponent(reviewStatus)}`
	)
}

async function searchWorkflowsFromBackend(
	dir: string,
	name: string,
	projectId: string,
	task: string,
	currentUrl: string,
	pageObservation = githubPageObservation()
) {
	const response = await fetch(`${backendUrl()}/api/retrieval/workflows/search`, {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({
			projectId,
			task,
			currentUrl,
			pageObservation,
			riskPolicy: {
				autoAllowed: ['read_only', 'read_or_search'],
				confirmationRequired: ['draft_change', 'external_send'],
				blocked: ['destructive'],
			},
			limit: 8,
		}),
	})
	expect(response.ok).toBe(true)
	const body = (await response.json()) as WorkflowSearchResponse
	await writeFile(join(dir, name), JSON.stringify(body, null, 2))
	return body
}

async function writeSummary(session: RealUserSession, extra: Record<string, unknown>) {
	await writeFile(
		join(session.artifactDir, 'summary.json'),
		JSON.stringify(
			{
				backendUrl: backendUrl(),
				dialogFixtureUrl: session.dialogFixtureUrl,
				extensionId: session.extensionId,
				fixtureUrl: session.fixtureUrl,
				ordersDialogFixtureUrl: session.ordersDialogFixtureUrl,
				ordersFixtureUrl: session.ordersFixtureUrl,
				projectId: session.projectId,
				...extra,
			},
			null,
			2
		)
	)
}

async function configureWorkflowBackend(sidePanel: Page, projectId: string) {
	await sidePanel.getByRole('button', { name: 'Settings' }).click()
	await sidePanel.getByPlaceholder('http://127.0.0.1:38402').fill(backendUrl())
	await sidePanel.getByPlaceholder('Project ID').fill(projectId)
	await sidePanel.getByRole('button', { name: 'Save' }).click()
	await expect(sidePanel.getByText('Playwright workflow')).toBeVisible()
}

async function getExtensionId(context: BrowserContext): Promise<string> {
	let worker = context.serviceWorkers()[0]
	if (!worker) {
		worker = await context.waitForEvent('serviceworker', { timeout: 3_000 }).catch(() => undefined)
	}
	if (worker) {
		const [, , extensionId] = worker.url().split('/')
		if (extensionId) return extensionId
	}
	return extensionIdFromManifest()
}

async function extensionIdFromManifest() {
	const manifest = JSON.parse(await readFile(resolve(EXTENSION_PATH, 'manifest.json'), 'utf8')) as {
		key?: string
	}
	if (!manifest.key) {
		throw new Error(
			'Cannot resolve extension id: service worker was not visible and manifest.key is missing.'
		)
	}
	const hash = createHash('sha256').update(Buffer.from(manifest.key, 'base64')).digest()
	return Array.from(hash.subarray(0, 16))
		.map((byte) => byte.toString(16).padStart(2, '0'))
		.join('')
		.replace(/[0-9a-f]/g, (char) => 'abcdefghijklmnop'[Number.parseInt(char, 16)] ?? '')
}

async function startFixtureServer(): Promise<{ url: string; close: () => Promise<void> }> {
	const server = createServer(async (request, response) => {
		const pathname = new URL(request.url ?? '/', 'http://127.0.0.1').pathname
		const allowedFiles = new Set([
			'workflow-github-issues.html',
			'workflow-github-issues-with-dialog.html',
			'workflow-ops-orders.html',
			'workflow-ops-orders-with-dialog.html',
		])
		const requestedFile =
			pathname.split('/').filter(Boolean).at(-1) || 'workflow-github-issues.html'
		const file = allowedFiles.has(requestedFile) ? requestedFile : 'workflow-github-issues.html'
		const body = await readFile(resolve(REPO_ROOT, 'tests/fixtures', file))
		response.writeHead(200, { 'content-type': 'text/html; charset=utf-8' })
		response.end(body)
	})
	await new Promise<void>((resolveReady) => server.listen(0, '127.0.0.1', resolveReady))
	const address = server.address()
	if (!address || typeof address === 'string')
		throw new Error('Fixture server did not bind a TCP port')
	return {
		url: `http://127.0.0.1:${address.port}`,
		close: () => new Promise((resolveClose) => server.close(() => resolveClose())),
	}
}

async function createArtifactDir(testInfo: TestInfo) {
	const slug = testInfo.title
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/^-|-$/g, '')
		.slice(0, 80)
	const dir = join(ARTIFACT_ROOT, `${new Date().toISOString().replace(/[:.]/g, '-')}-${slug}`)
	await mkdir(dir, { recursive: true })
	return dir
}

async function capture(dir: string, name: string, page: Page) {
	await page.screenshot({ path: join(dir, `${name}.png`), fullPage: true })
	await writeFile(join(dir, `${name}.url.txt`), page.url())
}

async function saveExtensionStorage(dir: string, sidePanel: Page, name: string) {
	const storage = await sidePanel.evaluate(() => chrome.storage.local.get(null))
	await writeFile(join(dir, `${name}.json`), JSON.stringify(storage, null, 2))
}

async function saveBackendJSON<T = unknown>(dir: string, name: string, path: string): Promise<T> {
	const response = await fetch(`${backendUrl()}${path}`)
	expect(response.ok).toBe(true)
	const body = await response.json()
	await writeFile(join(dir, name), JSON.stringify(body, null, 2))
	return body as T
}

function backendUrl() {
	if (!WORKFLOW_BACKEND_URL) throw new Error('WORKFLOW_BACKEND_URL is required')
	return WORKFLOW_BACKEND_URL.replace(/\/+$/, '')
}

function uniqueProjectId() {
	return `real-user-${Date.now()}`
}

function escapeRegExp(value: string) {
	return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function githubPageObservation() {
	return {
		title: 'GitHub fixture',
		visibleText: ['Code', 'Issues', 'Pull requests', 'Search all issues'],
		controls: [
			{ role: 'link', name: 'Issues' },
			{ role: 'textbox', name: 'Search all issues' },
			{ role: 'button', name: 'Search' },
		],
	}
}

function ordersPageObservation() {
	return {
		title: 'Operations order console',
		visibleText: ['Overview', 'Orders', 'Search customers or orders', 'Status', 'Apply filters'],
		controls: [
			{ role: 'link', name: 'Orders' },
			{ role: 'textbox', name: 'Search customers or orders' },
			{ role: 'combobox', name: 'Status' },
			{ role: 'button', name: 'Apply filters' },
			{ role: 'link', name: 'Open first order' },
		],
	}
}
