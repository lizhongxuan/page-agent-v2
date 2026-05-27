import { expect, test } from '@playwright/test'
import { existsSync } from 'node:fs'
import { readFile } from 'node:fs/promises'
import { type Server, createServer } from 'node:http'
import { extname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const REPO_ROOT = resolve(fileURLToPath(new URL('.', import.meta.url)), '..')
const FIXTURE_PATH = resolve(REPO_ROOT, 'tests/fixtures/page-agent-scroll-page.html')
const DEMO_BUNDLE_PATH = resolve(REPO_ROOT, 'packages/page-agent/dist/iife/page-agent.demo.js')
let fakeLlmCallCount = 0
let firstTextareaIndex = 0

test.describe('PageAgent scroll action regression', () => {
	let server: Server
	let origin: string

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
					await serveFakeLlmResponse(req, res, ++fakeLlmCallCount)
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
				resolveServer()
			})
		})
	})

	test.afterAll(async () => {
		await new Promise<void>((resolveClose) => server.close(() => resolveClose()))
	})

	test('falls back to page scroll when the model targets a non-scrollable element index', async ({
		page,
	}) => {
		fakeLlmCallCount = 0
		firstTextareaIndex = 0

		if (!existsSync(DEMO_BUNDLE_PATH)) {
			throw new Error(
				`Missing built Page Agent demo bundle at ${DEMO_BUNDLE_PATH}. Run "npm run build:demo --workspace=page-agent" before this test.`
			)
		}

		await page.goto(origin, { waitUntil: 'domcontentloaded' })
		await page.addScriptTag({ url: `${origin}/__page-agent-demo.js?autoInit=false` })

		const result = await page.evaluate(async () => {
			const AnyWindow = window as typeof window & {
				PageAgent?: new (config: Record<string, unknown>) => {
					execute: (task: string) => Promise<unknown>
					history: unknown[]
					dispose: () => void
				}
			}
			if (!AnyWindow.PageAgent) throw new Error('window.PageAgent was not registered')

			const agent = new AnyWindow.PageAgent({
				apiKey: 'test',
				baseURL: `${window.location.origin}/v1`,
				model: 'fake-scroll-planner',
				language: 'en-US',
				showPanel: false,
				enableMask: false,
				maxSteps: 3,
				stepDelay: 0,
			})

			const executionResult = await agent.execute('Scroll down to the target marker.')
			const history = agent.history
			return { executionResult, history, scrollY: window.scrollY }
		})

		const steps = extractSteps(result.history)
		expect(result.executionResult, JSON.stringify(result, null, 2)).toMatchObject({
			success: true,
		})
		expect(result.scrollY, JSON.stringify(result, null, 2)).toBeGreaterThan(100)
		expect(steps[0]?.actionName).toBe('scroll')
		expect(steps[0]?.output).toContain('Falling back to page scroll')
		expect(steps[0]?.output).toContain('Scrolled page')
	})
})

async function serveFakeLlmResponse(
	req: import('node:http').IncomingMessage,
	res: import('node:http').ServerResponse,
	callCount: number
) {
	const requestBody = JSON.parse(await readRequestBody(req)) as {
		messages?: { role: string; content?: string }[]
	}
	const userPrompt = requestBody.messages?.find((message) => message.role === 'user')?.content || ''
	const payload =
		callCount === 1
			? buildScrollPayload(readAndRememberTextareaIndex(userPrompt))
			: callCount === 2
				? buildScrollPayload(firstTextareaIndex)
				: {
						evaluation_previous_goal: 'The page moved after fallback scrolls. Verdict: Success',
						memory: 'The scroll fallback allowed the page to move downward despite indexed inputs.',
						next_goal: 'Finish.',
						action: {
							done: {
								success: true,
								text: 'Scroll completed.',
							},
						},
					}

	res.writeHead(200, { 'content-type': 'application/json' })
	res.end(
		JSON.stringify({
			choices: [
				{
					finish_reason: 'tool_calls',
					message: {
						tool_calls: [
							{
								id: `call-${crypto.randomUUID()}`,
								type: 'function',
								function: {
									name: 'AgentOutput',
									arguments: JSON.stringify(payload),
								},
							},
						],
					},
				},
			],
			usage: {
				prompt_tokens: 100,
				completion_tokens: 20,
				total_tokens: 120,
			},
		})
	)
}

function buildScrollPayload(index: number) {
	return {
		evaluation_previous_goal: 'Ready to reproduce the indexed scroll issue. Verdict: Success',
		memory: 'The model is using a textarea index even though the page itself should scroll.',
		next_goal: 'Scroll downward using the textarea index.',
		action: {
			scroll: {
				down: true,
				num_pages: 0.8,
				pixels: 0,
				index,
			},
		},
	}
}

function readAndRememberTextareaIndex(userPrompt: string): number {
	const index = Number(/\[(\d+)]<textarea/.exec(userPrompt)?.[1] ?? NaN)
	if (!Number.isFinite(index)) {
		throw new Error(`Could not find textarea index in prompt:\n${userPrompt}`)
	}
	firstTextareaIndex = index
	return index
}

function extractSteps(history: unknown[]) {
	return history
		.filter(
			(
				event
			): event is {
				type: 'step'
				action?: { name?: string; output?: string }
			} =>
				typeof event === 'object' &&
				event !== null &&
				'type' in event &&
				(event as { type?: unknown }).type === 'step'
		)
		.map((event) => ({
			actionName: event.action?.name || 'unknown',
			output: event.action?.output || '',
		}))
}

async function serveFile(res: import('node:http').ServerResponse, filePath: string) {
	const contentType = contentTypeFor(filePath)
	const content = await readFile(filePath)
	res.writeHead(200, { 'content-type': contentType })
	res.end(content)
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
