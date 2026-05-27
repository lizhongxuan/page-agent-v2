import { type BrowserContext, type Worker, chromium, expect, test } from '@playwright/test'
import { mkdtemp, rm } from 'node:fs/promises'
import { type Server, createServer } from 'node:http'
import os from 'node:os'
import path from 'node:path'

let server: Server
let origin: string
const SIDE_PANEL_HANDOVER_ACTIVE_STORAGE_KEY = 'pageAgentSidePanelHandoverActive'

test.beforeAll(async () => {
	server = createServer((req, res) => {
		if (req.url !== '/') {
			res.writeHead(404)
			res.end('not found')
			return
		}

		res.setHeader('content-type', 'text/html;charset=utf-8')
		res.end(`<!doctype html>
			<html>
				<head><title>WebOps Fixture</title></head>
				<body>
					<label>
						Search
						<input placeholder="按照服务名称或容器IP搜索" />
					</label>
					<button onclick="window.searchClicked = true">搜索</button>
				</body>
			</html>`)
	})

	await new Promise<void>((resolve) => {
		server.listen(0, '127.0.0.1', () => {
			const address = server.address()
			if (typeof address === 'object' && address) {
				origin = `http://127.0.0.1:${address.port}`
			}
			resolve()
		})
	})
})

test.afterAll(async () => {
	await new Promise<void>((resolve) => server.close(() => resolve()))
})

test('content script injects WebOps overlay host on normal web pages', async () => {
	const extensionPath = path.resolve('packages/extension/.output/chrome-mv3')
	const userDataDir = await mkdtemp(path.join(os.tmpdir(), 'webops-extension-'))
	const context = await chromium.launchPersistentContext(userDataDir, {
		headless: false,
		args: [`--disable-extensions-except=${extensionPath}`, `--load-extension=${extensionPath}`],
	})

	try {
		const page = await context.newPage()
		await page.goto(origin)
		const extensionWorker = await getExtensionWorker(context)
		const tabId = await getFixtureTabId(extensionWorker, origin)

		await expect(page.locator('#page-agent-v2-webops-overlay')).toBeAttached()
		await expect(page.getByPlaceholder('按照服务名称或容器IP搜索')).toBeVisible()

		await setAgentRunning(extensionWorker, tabId, true)
		await expect(page.locator('#page-agent-v2-runtime-page-lock')).toBeVisible()
		const lockedBrowserState = await getBrowserState(extensionWorker, tabId)
		expect(String(lockedBrowserState.content)).toContain('按照服务名称或容器IP搜索')
		expect(String(lockedBrowserState.content)).not.toContain('undefined')
		await page
			.getByRole('button', { name: '搜索' })
			.click({ timeout: 700 })
			.catch(() => {})
		await expect(page.evaluate(() => window.searchClicked ?? false)).resolves.toBe(false)

		await setSidePanelHandover(extensionWorker, true)
		await setAgentRunning(extensionWorker, tabId, true)
		await expect(page.locator('#page-agent-v2-runtime-page-lock')).toHaveCount(0, {
			timeout: 1000,
		})
		await page.getByRole('button', { name: '搜索' }).click()
		await expect(page.evaluate(() => window.searchClicked ?? false)).resolves.toBe(true)
		await page.evaluate(() => {
			window.searchClicked = false
		})
		await setSidePanelHandover(extensionWorker, false)
		await setAgentRunning(extensionWorker, tabId, true)
		await expect(page.locator('#page-agent-v2-runtime-page-lock')).toBeVisible()

		await setAgentRunning(extensionWorker, tabId, false)
		await expect(page.locator('#page-agent-v2-runtime-page-lock')).toHaveCount(0)

		await publishInteraction(extensionWorker, origin, {
			type: 'spotlight',
			elementIndex: 0,
			action: 'input',
			message: '准备输入搜索关键词',
			timeoutMs: 5_000,
		})
		await expect(page.getByText('准备输入搜索关键词')).toBeVisible()

		const responsePromise = requestInteraction(extensionWorker, origin, {
			event: {
				type: 'input',
				requestId: 'playwright-input',
				title: '需要补充信息',
				message: '请输入搜索关键词',
				submitButtonLabel: '提交并继续',
			},
			timeoutMs: 5_000,
		})
		await expect(page.locator('[data-webops-input-prompt="true"]')).toBeVisible()
		await page.locator('[data-webops-input-prompt="true"] textarea').fill('page-agent')
		await page.screenshot({ path: 'test-results/webops-extension-fixture.png', fullPage: true })
		await page.locator('[data-webops-input-submit="true"]').click()
		await expect(responsePromise).resolves.toEqual({ type: 'input', value: 'page-agent' })

		const handoverPromise = requestInteraction(extensionWorker, origin, {
			event: {
				type: 'handover',
				requestId: 'playwright-handover',
				title: '需要你接管页面',
				message: '请完成页面上的安全验证，然后把控制权交还给 Agent。',
				resumeButtonLabel: '我已完成，继续',
			},
			timeoutMs: 5_000,
		})
		await expect(page.locator('[data-webops-handover-done="true"]')).toBeVisible()
		await page.locator('[data-webops-handover-done="true"]').click()
		await expect(handoverPromise).resolves.toEqual({ type: 'handover_done' })

		const choicePromise = requestInteraction(extensionWorker, origin, {
			event: {
				type: 'choice',
				requestId: 'playwright-choice',
				elementIndex: 1,
				title: '确认目标',
				message: '要点击这个搜索按钮吗？',
				options: [{ id: 'yes', label: '确认' }],
			},
			timeoutMs: 5_000,
		})
		await expect(page.locator('[data-webops-anchor-bubble="true"]')).toBeVisible()
		await page.locator('[data-webops-choice-confirm="true"]').click()
		await expect(choicePromise).resolves.toEqual({ type: 'choice', optionId: 'yes' })
	} finally {
		await context.close()
		await rm(userDataDir, { recursive: true, force: true })
	}
})

async function getExtensionWorker(context: BrowserContext) {
	return context.serviceWorkers()[0] ?? (await context.waitForEvent('serviceworker'))
}

async function getFixtureTabId(worker: Worker, originUrl: string): Promise<number> {
	return worker.evaluate(async (originUrl) => {
		const [tab] = await chrome.tabs.query({ url: `${originUrl}/*` })
		if (!tab?.id) throw new Error('Fixture tab not found')
		return tab.id
	}, originUrl)
}

async function setAgentRunning(worker: Worker, tabId: number, running: boolean) {
	await worker.evaluate(
		async ({ running, tabId }) => {
			await chrome.storage.local.set({
				isAgentRunning: running,
				agentHeartbeat: Date.now(),
				currentTabId: running ? tabId : null,
			})
		},
		{ running, tabId }
	)
}

async function setSidePanelHandover(worker: Worker, active: boolean) {
	await worker.evaluate(
		async ({ active, key }) => {
			await chrome.storage.local.set({ [key]: active })
		},
		{ active, key: SIDE_PANEL_HANDOVER_ACTIVE_STORAGE_KEY }
	)
}

async function getBrowserState(worker: Worker, tabId: number): Promise<{ content?: unknown }> {
	return worker.evaluate(async (tabId) => {
		return chrome.tabs.sendMessage(tabId, {
			type: 'PAGE_CONTROL',
			action: 'get_browser_state',
		})
	}, tabId)
}

async function publishInteraction(worker: Worker, originUrl: string, payload: unknown) {
	await worker.evaluate(
		async ({ originUrl, payload }) => {
			const [tab] = await chrome.tabs.query({ url: `${originUrl}/*` })
			if (!tab?.id) throw new Error('Fixture tab not found')
			await chrome.tabs.sendMessage(tab.id, {
				type: 'PAGE_CONTROL',
				action: 'webops_interaction',
				payload,
			})
		},
		{ originUrl, payload }
	)
}

async function requestInteraction(
	worker: Worker,
	originUrl: string,
	payload: unknown
): Promise<unknown> {
	return worker.evaluate(
		async ({ originUrl, payload }) => {
			const [tab] = await chrome.tabs.query({ url: `${originUrl}/*` })
			if (!tab?.id) throw new Error('Fixture tab not found')
			return chrome.tabs.sendMessage(tab.id, {
				type: 'PAGE_CONTROL',
				action: 'webops_interaction_request',
				payload,
			})
		},
		{ originUrl, payload }
	)
}
