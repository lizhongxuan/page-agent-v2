import { chromium, expect, test } from '@playwright/test'
import { existsSync } from 'node:fs'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import { type Server, createServer } from 'node:http'
import { extname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const REPO_ROOT = resolve(fileURLToPath(new URL('.', import.meta.url)), '..')
const FIXTURE_PATH = resolve(REPO_ROOT, 'tests/fixtures/page-agent-ops-console.html')
const DEMO_BUNDLE_PATH = resolve(REPO_ROOT, 'packages/page-agent/dist/iife/page-agent.demo.js')
const REPORT_PATH = resolve(REPO_ROOT, 'output/page-agent/real-llm-chrome-report.json')
const TASK = [
	'You are operating a local PG recovery console fixture.',
	'Diagnose PG pg-1 and complete the workflow:',
	'1. Search for pg-1 in the PG search box.',
	'2. Open the pg-1 details from the PG inventory.',
	'3. Inspect the error summary.',
	'4. Open every evidence card from Evidence 1 through Evidence 18.',
	'5. Open every recent log line from Log 1 through Log 6.',
	'6. Use action arrays for the evidence-card and log-line review batches when all target indexes are visible in the current browser_state.',
	'7. Because pg-1 has high-risk alerts within the last hour, create a P1 ticket.',
	'8. Keep the ticket title specific to pg-1 and the WAL replay stalled failure.',
	'9. Submit the ticket, then finish with a concise markdown summary including the ticket id.',
].join('\n')

interface RealRunReport {
	task: string
	startedAt: string
	finishedAt: string
	durationMs: number
	status: string
	success: boolean
	resultData: unknown
	ticketId: string | null
	reviewedEvidenceCount: number
	reviewedLogCount: number
	prompt: {
		taskCharacters: number
		maxPromptCharacters: number
		finalPromptCharacters: number
		promptCharactersByStep: number[]
	}
	context: {
		llmCallCount: number
		sessionSummaryAppeared: boolean
		summaryHasDomIndex: boolean
		maxRecentStepCount: number
		contextLengthError: boolean
		firstCompactedPromptCharacters: number | null
		finalVsFirstCompactedRatio: number | null
		llmCalls: {
			callIndex: number
			completedStepCount: number
			promptCharacters: number
			hasSessionSummary: boolean
			recentStepCount: number
			summaryHasDomIndex: boolean
		}[]
	}
	llm: {
		baseURL: string
		model: string
		apiKeyProvided: boolean
	}
	steps: {
		stepIndex: number
		actionName: string
		actionInput: unknown
		output: string
		usage: unknown
		promptCharacters: number
	}[]
	history: unknown[]
	consoleErrors: string[]
}

test.describe('real LLM in real Chromium against ops console fixture', () => {
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
				if (url.pathname === '/favicon.ico') {
					res.writeHead(204)
					res.end()
					return
				}
				if (url.pathname === '/v1/chat/completions') {
					await proxyLlmRequest(req, res)
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

	test('creates a P1 ticket without exceeding the long-task budget', async () => {
		test.setTimeout(240_000)

		const upstreamBaseURL = readRequiredEnv('PAGE_AGENT_TEST_BASE_URL')
		const apiKey = readRequiredEnv('PAGE_AGENT_TEST_API_KEY')
		const model = readRequiredEnv('PAGE_AGENT_TEST_MODEL')

		if (!existsSync(DEMO_BUNDLE_PATH)) {
			throw new Error(
				`Missing built Page Agent demo bundle at ${DEMO_BUNDLE_PATH}. Run "npm run build:demo --workspace=page-agent" before this real LLM Chrome test.`
			)
		}

		const browser = await chromium.launch({
			channel: process.env.PAGE_AGENT_TEST_BROWSER_CHANNEL || undefined,
			headless: process.env.PAGE_AGENT_TEST_HEADLESS !== 'false',
		})
		const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
		const page = await context.newPage()
		const consoleErrors: string[] = []
		page.on('console', (message) => {
			if (message.type() === 'error') consoleErrors.push(message.text())
		})
		page.on('pageerror', (error) => consoleErrors.push(String(error)))

		const startedAt = new Date()
		const startedTime = Date.now()
		let report: RealRunReport | null = null

		try {
			await page.goto(origin, { waitUntil: 'domcontentloaded' })
			await expect(page.getByRole('heading', { name: 'PG Recovery Console' })).toBeVisible()

			await page.addScriptTag({
				url: `${origin}/__page-agent-demo.js?autoInit=false`,
			})

			const result = await page.evaluate(
				async ({ apiKey, baseURL, model, task }) => {
					const AnyWindow = window as typeof window & {
						PageAgent?: new (config: Record<string, unknown>) => {
							execute: (task: string) => Promise<unknown>
							status: string
							history: unknown[]
							dispose: () => void
						}
						pageAgent?: {
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
					const promptMetrics: {
						callIndex: number
						completedStepCount: number
						promptCharacters: number
						hasSessionSummary: boolean
						recentStepCount: number
						summaryHasDomIndex: boolean
					}[] = []
					AnyWindow.pageAgent = new AnyWindow.PageAgent({
						apiKey,
						baseURL,
						model,
						language: 'en-US',
						showPanel: false,
						enableMask: true,
						maxSteps: 40,
						stepDelay: 0.2,
						context: {
							enabled: true,
							maxPromptTokens: 64000,
							recentStepCount: 6,
							compactAfterSteps: 12,
						},
						transformRequestBody: (requestBody: Record<string, unknown>) => {
							const messages = requestBody.messages as
								| { role: string; content?: string }[]
								| undefined
							const userPrompt = messages?.find((message) => message.role === 'user')?.content || ''
							promptCharactersByStep.push(userPrompt.length)
							const sessionSummary = extractTag(userPrompt, 'session_summary')
							const recentSteps = extractTag(userPrompt, 'recent_steps')
							promptMetrics.push({
								callIndex: promptMetrics.length,
								completedStepCount:
									AnyWindow.pageAgent?.history.filter(
										(event: unknown) =>
											typeof event === 'object' &&
											event !== null &&
											'type' in event &&
											(event as { type?: unknown }).type === 'step'
									).length ?? 0,
								promptCharacters: userPrompt.length,
								hasSessionSummary: userPrompt.includes('<session_summary>'),
								recentStepCount: (recentSteps.match(/<step_[0-9]+>/g) || []).length,
								summaryHasDomIndex: /\[[0-9]+\]/.test(sessionSummary),
							})
							return requestBody
						},
					})

					const executionResult = await AnyWindow.pageAgent.execute(task)
					const ticketId = document.body.dataset.ticketId || null
					const reviewedEvidenceCount = Number(document.body.dataset.reviewedEvidenceCount || '0')
					const reviewedLogCount = Number(document.body.dataset.reviewedLogCount || '0')
					const history = AnyWindow.pageAgent.history

					return {
						executionResult,
						history,
						status: AnyWindow.pageAgent.status,
						ticketId,
						reviewedEvidenceCount,
						reviewedLogCount,
						promptCharactersByStep,
						promptMetrics,
					}

					function extractTag(prompt: string, tag: string): string {
						const match = new RegExp(`<${tag}>\\n([\\s\\S]*?)\\n</${tag}>`).exec(prompt)
						return match?.[1] ?? ''
					}
				},
				{ apiKey: 'proxied-by-local-test-server', baseURL: llmBaseURL, model, task: TASK }
			)

			const finishedAt = new Date()
			const steps = extractSteps(result.history, result.promptCharactersByStep)
			const firstCompactedPrompt = result.promptMetrics.find(
				(metric) => metric.completedStepCount >= 12
			)
			const contextLengthError = result.history.some(
				(event) =>
					typeof event === 'object' &&
					event !== null &&
					'type' in event &&
					(event as { type?: unknown }).type === 'error' &&
					/message/i.test(JSON.stringify(event)) &&
					/context/i.test(JSON.stringify(event))
			)
			report = {
				task: TASK,
				startedAt: startedAt.toISOString(),
				finishedAt: finishedAt.toISOString(),
				durationMs: Date.now() - startedTime,
				status: result.status,
				success: isSuccessfulResult(result.executionResult),
				resultData: readResultData(result.executionResult),
				ticketId: result.ticketId,
				reviewedEvidenceCount: result.reviewedEvidenceCount,
				reviewedLogCount: result.reviewedLogCount,
				prompt: {
					taskCharacters: TASK.length,
					maxPromptCharacters: Math.max(0, ...result.promptCharactersByStep),
					finalPromptCharacters: result.promptCharactersByStep.at(-1) ?? 0,
					promptCharactersByStep: result.promptCharactersByStep,
				},
				context: {
					llmCallCount: result.promptMetrics.length,
					sessionSummaryAppeared: result.promptMetrics.some((metric) => metric.hasSessionSummary),
					summaryHasDomIndex: result.promptMetrics.some((metric) => metric.summaryHasDomIndex),
					maxRecentStepCount: Math.max(
						0,
						...result.promptMetrics.map((metric) => metric.recentStepCount)
					),
					contextLengthError,
					firstCompactedPromptCharacters: firstCompactedPrompt?.promptCharacters ?? null,
					finalVsFirstCompactedRatio: firstCompactedPrompt
						? Number(
								(
									(result.promptCharactersByStep.at(-1) ?? 0) /
									firstCompactedPrompt.promptCharacters
								).toFixed(3)
							)
						: null,
					llmCalls: result.promptMetrics,
				},
				llm: {
					baseURL: upstreamBaseURL,
					model,
					apiKeyProvided: Boolean(apiKey),
				},
				steps,
				history: result.history,
				consoleErrors,
			}

			await writeReport(report)

			expect(report.success).toBe(true)
			expect(report.ticketId).toMatch(/^P1-\d{8}-PG1-0427$/)
			expect(report.reviewedEvidenceCount).toBe(18)
			expect(report.reviewedLogCount).toBe(6)
			expect(report.durationMs).toBeLessThan(240_000)
			expect(report.steps.length).toBeGreaterThanOrEqual(25)
			expect(report.steps.length).toBeLessThanOrEqual(40)
			expect(report.context.contextLengthError).toBe(false)
			expect(report.context.sessionSummaryAppeared).toBe(true)
			expect(report.context.summaryHasDomIndex).toBe(false)
			expect(report.context.maxRecentStepCount).toBeLessThanOrEqual(6)
			expect(report.context.finalVsFirstCompactedRatio).not.toBeNull()
			expect(report.context.finalVsFirstCompactedRatio!).toBeLessThanOrEqual(1.5)
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
					ticketId: await page
						.evaluate(() => document.body.dataset.ticketId || null)
						.catch(() => null),
					reviewedEvidenceCount: await page
						.evaluate(() => Number(document.body.dataset.reviewedEvidenceCount || '0'))
						.catch(() => 0),
					reviewedLogCount: await page
						.evaluate(() => Number(document.body.dataset.reviewedLogCount || '0'))
						.catch(() => 0),
					prompt: {
						taskCharacters: TASK.length,
						maxPromptCharacters: 0,
						finalPromptCharacters: 0,
						promptCharactersByStep: [],
					},
					context: {
						llmCallCount: 0,
						sessionSummaryAppeared: false,
						summaryHasDomIndex: false,
						maxRecentStepCount: 0,
						contextLengthError: false,
						firstCompactedPromptCharacters: null,
						finalVsFirstCompactedRatio: null,
						llmCalls: [],
					},
					llm: {
						baseURL: process.env.PAGE_AGENT_TEST_BASE_URL || '',
						model: process.env.PAGE_AGENT_TEST_MODEL || '',
						apiKeyProvided: Boolean(process.env.PAGE_AGENT_TEST_API_KEY),
					},
					steps: [],
					history: [],
					consoleErrors,
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

function extractSteps(
	history: unknown[],
	promptCharactersByStep: number[]
): RealRunReport['steps'] {
	return history
		.filter(
			(
				event
			): event is {
				type: 'step'
				stepIndex: number
				action?: { name?: string; input?: unknown; output?: string }
				usage?: unknown
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
			usage: event.usage,
			promptCharacters: promptCharactersByStep[event.stepIndex] ?? 0,
		}))
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

async function writeReport(report: RealRunReport) {
	await mkdir(join(REPO_ROOT, 'output/page-agent'), { recursive: true })
	await writeFile(REPORT_PATH, JSON.stringify(report, null, 2))
}
