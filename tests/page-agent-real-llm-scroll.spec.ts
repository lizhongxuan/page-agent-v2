import { chromium, expect, test } from '@playwright/test'
import { existsSync } from 'node:fs'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import { type Server, createServer } from 'node:http'
import { extname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const REPO_ROOT = resolve(fileURLToPath(new URL('.', import.meta.url)), '..')
const FIXTURE_PATH = resolve(REPO_ROOT, 'tests/fixtures/page-agent-scroll-page.html')
const DEMO_BUNDLE_PATH = resolve(REPO_ROOT, 'packages/page-agent/dist/iife/page-agent.demo.js')
const REPORT_PATH = resolve(REPO_ROOT, 'output/page-agent/real-llm-scroll-report.json')
const TASK = [
	'You are validating a PageAgent scroll regression on this local page.',
	'The target marker text SCROLL_TARGET_20260527 is below the visible area.',
	'Regression reproduction requirement: for your first scroll attempt, use the visible textarea index as the scroll index, set pixels to 0, and set num_pages to 0.8.',
	'After that first scroll attempt, continue scrolling normally until the marker text appears in the current browser_state.',
	'Do not type into the textarea and do not click the link.',
	'Finish with done success true and include the exact marker text you found.',
].join('\n')

interface RealScrollReport {
	task: string
	startedAt: string
	finishedAt: string
	durationMs: number
	status: string
	success: boolean
	resultData: unknown
	scrollY: number
	targetVisible: boolean
	usedIndexedZeroPixelScroll: boolean
	usedPageFallback: boolean
	llm: {
		baseURL: string
		model: string
		apiKeyProvided: boolean
		browserChannel: string
	}
	promptCharactersByStep: number[]
	steps: {
		stepIndex: number
		actionName: string
		actionInput: unknown
		output: string
	}[]
	consoleErrors: string[]
	history: unknown[]
}

test.describe('real LLM in real Chrome scroll regression', () => {
	let server: Server
	let origin: string
	let llmBaseURL: string

	test.beforeAll(async () => {
		server = createServer(async (req, res) => {
			try {
				const url = new URL(req.url || '/', 'http://127.0.0.1')
				if (url.pathname === '/') {
					await serveFile(res, FIXTURE_PATH)
					return
				}
				if (url.pathname === '/__page-agent-demo.js') {
					await serveFile(res, DEMO_BUNDLE_PATH)
					return
				}
				if (url.pathname === '/v1/chat/completions') {
					await proxyLlmRequest(req, res)
					return
				}
				if (url.pathname === '/favicon.ico') {
					res.writeHead(204)
					res.end()
					return
				}
				res.writeHead(404, { 'content-type': 'text/plain;charset=utf-8' })
				res.end('not found')
			} catch (error) {
				res.writeHead(500, { 'content-type': 'text/plain;charset=utf-8' })
				res.end(String(error))
			}
		})

		await new Promise<void>((resolveServer) => {
			server.listen(0, '127.0.0.1', () => {
				const address = server.address()
				if (typeof address !== 'object' || !address) {
					throw new Error('Could not allocate fixture HTTP server port')
				}
				origin = `http://127.0.0.1:${address.port}`
				llmBaseURL = `${origin}/v1`
				resolveServer()
			})
		})
	})

	test.afterAll(async () => {
		await new Promise<void>((resolveClose) => server.close(() => resolveClose()))
	})

	test('recovers when the real model sends an indexed zero-pixel scroll', async () => {
		test.setTimeout(180_000)

		const upstreamBaseURL = readRequiredEnv('PAGE_AGENT_TEST_BASE_URL')
		const apiKey = readRequiredEnv('PAGE_AGENT_TEST_API_KEY')
		const model = readRequiredEnv('PAGE_AGENT_TEST_MODEL')
		const browserChannel = process.env.PAGE_AGENT_TEST_BROWSER_CHANNEL || 'chrome'

		if (!existsSync(DEMO_BUNDLE_PATH)) {
			throw new Error(
				`Missing built Page Agent demo bundle at ${DEMO_BUNDLE_PATH}. Run "npm run build:demo --workspace=page-agent" before this real LLM Chrome test.`
			)
		}

		const browser = await chromium.launch({
			channel: browserChannel,
			headless: process.env.PAGE_AGENT_TEST_HEADLESS !== 'false',
		})
		const context = await browser.newContext({ viewport: { width: 1365, height: 900 } })
		const page = await context.newPage()
		const consoleErrors: string[] = []
		page.on('console', (message) => {
			if (message.type() === 'error') consoleErrors.push(message.text())
		})
		page.on('pageerror', (error) => consoleErrors.push(String(error)))

		const startedAt = new Date()
		const startedTime = Date.now()
		let report: RealScrollReport | null = null

		try {
			await page.goto(origin, { waitUntil: 'domcontentloaded' })
			await expect(page.getByRole('heading', { name: 'Scroll Regression Fixture' })).toBeVisible()
			await page.addScriptTag({ url: `${origin}/__page-agent-demo.js?autoInit=false` })

			const result = await page.evaluate(
				async ({ baseURL, model, task }) => {
					const AnyWindow = window as typeof window & {
						PageAgent?: new (config: Record<string, unknown>) => {
							execute: (task: string) => Promise<unknown>
							status: string
							history: unknown[]
							dispose: () => void
						}
					}
					if (!AnyWindow.PageAgent) {
						throw new Error('window.PageAgent was not registered by page-agent.demo.js')
					}

					const promptCharactersByStep: number[] = []
					const agent = new AnyWindow.PageAgent({
						apiKey: 'proxied-by-local-test-server',
						baseURL,
						model,
						language: 'en-US',
						showPanel: false,
						enableMask: true,
						viewportExpansion: 0,
						maxSteps: 6,
						stepDelay: 0.1,
						transformRequestBody: (requestBody: Record<string, unknown>) => {
							const messages = requestBody.messages as
								| { role: string; content?: string }[]
								| undefined
							const userPrompt = messages?.find((message) => message.role === 'user')?.content || ''
							promptCharactersByStep.push(userPrompt.length)
							return requestBody
						},
					})

					const executionResult = await agent.execute(task)
					const target = document.getElementById('target')
					const targetRect = target?.getBoundingClientRect()
					const targetVisible = Boolean(
						targetRect &&
						targetRect.top < window.innerHeight &&
						targetRect.bottom > 0 &&
						window.getComputedStyle(target!).visibility !== 'hidden'
					)
					const history = agent.history
					const status = agent.status
					agent.dispose()

					return {
						executionResult,
						history,
						status,
						scrollY: window.scrollY,
						targetVisible,
						promptCharactersByStep,
					}
				},
				{ baseURL: llmBaseURL, model, task: TASK }
			)

			const steps = extractSteps(result.history)
			const scrollSteps = steps.filter((step) => step.actionName === 'scroll')
			const usedIndexedZeroPixelScroll = scrollSteps.some((step) =>
				isIndexedZeroPixelScroll(step.actionInput)
			)
			const usedPageFallback = scrollSteps.some((step) =>
				/Falling back to page scroll/i.test(step.output)
			)
			const finishedAt = new Date()

			report = {
				task: TASK,
				startedAt: startedAt.toISOString(),
				finishedAt: finishedAt.toISOString(),
				durationMs: Date.now() - startedTime,
				status: result.status,
				success: isSuccessfulResult(result.executionResult),
				resultData: readResultData(result.executionResult),
				scrollY: result.scrollY,
				targetVisible: result.targetVisible,
				usedIndexedZeroPixelScroll,
				usedPageFallback,
				llm: {
					baseURL: upstreamBaseURL,
					model,
					apiKeyProvided: Boolean(apiKey),
					browserChannel,
				},
				promptCharactersByStep: result.promptCharactersByStep,
				steps,
				consoleErrors,
				history: result.history,
			}

			await writeReport(report)

			expect(report.success).toBe(true)
			expect(JSON.stringify(report.resultData)).toContain('SCROLL_TARGET_20260527')
			expect(report.scrollY).toBeGreaterThan(100)
			expect(report.targetVisible).toBe(true)
			expect(report.usedIndexedZeroPixelScroll).toBe(true)
			expect(report.usedPageFallback).toBe(true)
			expect(report.consoleErrors).toEqual([])
		} finally {
			if (!report) {
				await writeReport({
					task: TASK,
					startedAt: startedAt.toISOString(),
					finishedAt: new Date().toISOString(),
					durationMs: Date.now() - startedTime,
					status: 'failed-before-result',
					success: false,
					resultData: null,
					scrollY: await page.evaluate(() => window.scrollY).catch(() => 0),
					targetVisible: false,
					usedIndexedZeroPixelScroll: false,
					usedPageFallback: false,
					llm: {
						baseURL: process.env.PAGE_AGENT_TEST_BASE_URL || '',
						model: process.env.PAGE_AGENT_TEST_MODEL || '',
						apiKeyProvided: Boolean(process.env.PAGE_AGENT_TEST_API_KEY),
						browserChannel: process.env.PAGE_AGENT_TEST_BROWSER_CHANNEL || 'chrome',
					},
					promptCharactersByStep: [],
					steps: [],
					consoleErrors,
					history: [],
				})
			}
			await context.close()
			await browser.close()
		}
	})
})

async function serveFile(res: import('node:http').ServerResponse, filePath: string) {
	const contentType = contentTypeFor(filePath)
	const content = await readFile(filePath)
	res.writeHead(200, { 'content-type': contentType })
	res.end(content)
}

async function proxyLlmRequest(
	req: import('node:http').IncomingMessage,
	res: import('node:http').ServerResponse
) {
	const upstreamBaseURL = readRequiredEnv('PAGE_AGENT_TEST_BASE_URL').replace(/\/$/, '')
	const apiKey = readRequiredEnv('PAGE_AGENT_TEST_API_KEY')
	const body = await readRequestBody(req)
	const upstream = await fetch(`${upstreamBaseURL}/chat/completions`, {
		method: 'POST',
		headers: {
			'content-type': 'application/json',
			authorization: `Bearer ${apiKey}`,
		},
		body,
	})
	const text = await upstream.text()
	res.writeHead(upstream.status, {
		'content-type': upstream.headers.get('content-type') || 'application/json',
	})
	res.end(text)
}

async function readRequestBody(req: import('node:http').IncomingMessage): Promise<string> {
	const chunks: Buffer[] = []
	for await (const chunk of req) {
		chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk))
	}
	return Buffer.concat(chunks).toString('utf8')
}

function contentTypeFor(filePath: string): string {
	switch (extname(filePath)) {
		case '.html':
			return 'text/html;charset=utf-8'
		case '.js':
			return 'text/javascript;charset=utf-8'
		default:
			return 'application/octet-stream'
	}
}

function readRequiredEnv(name: string): string {
	const value = process.env[name]
	if (!value) {
		throw new Error(
			`Missing ${name}. Set PAGE_AGENT_TEST_BASE_URL, PAGE_AGENT_TEST_API_KEY, and PAGE_AGENT_TEST_MODEL to run the real LLM Chrome test.`
		)
	}
	return value
}

function extractSteps(history: unknown[]): RealScrollReport['steps'] {
	return history
		.filter(
			(
				event
			): event is {
				type: 'step'
				stepIndex: number
				action?: { name?: string; input?: unknown; output?: string }
			} =>
				typeof event === 'object' &&
				event !== null &&
				'type' in event &&
				(event as { type?: unknown }).type === 'step'
		)
		.map((event) => ({
			stepIndex: event.stepIndex,
			actionName: event.action?.name || 'unknown',
			actionInput: event.action?.input,
			output: event.action?.output || '',
		}))
}

function isIndexedZeroPixelScroll(input: unknown): boolean {
	if (typeof input !== 'object' || input === null) return false
	const scrollInput = input as { index?: unknown; pixels?: unknown; num_pages?: unknown }
	return (
		typeof scrollInput.index === 'number' &&
		scrollInput.pixels === 0 &&
		typeof scrollInput.num_pages === 'number' &&
		scrollInput.num_pages > 0
	)
}

function isSuccessfulResult(result: unknown): boolean {
	return (
		typeof result === 'object' &&
		result !== null &&
		'success' in result &&
		(result as { success?: unknown }).success === true
	)
}

function readResultData(result: unknown): unknown {
	if (typeof result !== 'object' || result === null || !('data' in result)) return null
	return (result as { data?: unknown }).data
}

async function writeReport(report: RealScrollReport) {
	await mkdir(join(REPO_ROOT, 'output/page-agent'), { recursive: true })
	await writeFile(REPORT_PATH, JSON.stringify(report, null, 2))
}
