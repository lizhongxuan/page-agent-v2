import { type MacroToolInput, type MacroToolResult, PageAgentCore } from '@page-agent/core'
import type { BrowserState, PageController } from '@page-agent/page-controller'
import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

interface PromptMetric {
	step: number
	characters: number
	hasSessionSummary: boolean
	recentStepCount: number
	sessionSummaryHasDomIndex: boolean
	currentStateIndexes: number[]
	actionName: string
	actionIndex?: number
}

interface PlannerAction {
	name: string
	input: Record<string, unknown>
	output: string
}

class FakePageController extends EventTarget {
	private observedStep = 0
	private readonly actionLog: PlannerAction[] = []

	async getBrowserState(): Promise<BrowserState> {
		this.observedStep++
		return {
			url: 'https://console.example.test/ops',
			title: `Ops Console Step ${this.observedStep}`,
			header: [
				`Current Page: [Ops Console Step ${this.observedStep}](https://console.example.test/ops)`,
				'Page info: 1280x720px viewport, 1280x2400px total page size, 0.0 pages above, 2.0 pages below, 3.0 total pages, at 0% of page',
				'',
				'Interactive elements from top layer of the current page inside the viewport:',
				'',
				'[Start of page]',
			].join('\n'),
			content: this.buildContent(this.observedStep),
			footer: '... 1680 pixels below (2.0 pages) - scroll to see more ...',
		}
	}

	async getLastUpdateTime(): Promise<number> {
		return Date.now()
	}

	async clickElement(index: number) {
		const action = {
			name: 'click_element_by_index',
			input: { index },
			output: `✅ Clicked element ([${index}]<button>step ${this.observedStep}</button>).`,
		}
		this.actionLog.push(action)
		return { success: true, message: action.output }
	}

	async inputText(index: number, text: string) {
		const action = {
			name: 'input_text',
			input: { index, text },
			output: `✅ Input text (${text}) into element ([${index}]<input />).`,
		}
		this.actionLog.push(action)
		return { success: true, message: action.output }
	}

	async selectOption(index: number, text: string) {
		const action = {
			name: 'select_dropdown_option',
			input: { index, text },
			output: `✅ Selected option (${text}) in element ([${index}]<select />).`,
		}
		this.actionLog.push(action)
		return { success: true, message: action.output }
	}

	async scroll(options: { down: boolean; numPages: number; pixels?: number; index?: number }) {
		const action = {
			name: 'scroll',
			input: options,
			output: `✅ Scrolled ${options.down ? 'down' : 'up'} by ${options.numPages} pages.`,
		}
		this.actionLog.push(action)
		return { success: true, message: action.output }
	}

	async scrollHorizontally(options: { right: boolean; pixels: number; index?: number }) {
		const action = {
			name: 'scroll_horizontally',
			input: options,
			output: `✅ Scrolled horizontally ${options.right ? 'right' : 'left'} by ${options.pixels}px.`,
		}
		this.actionLog.push(action)
		return { success: true, message: action.output }
	}

	async showMask(): Promise<void> {}
	async hideMask(): Promise<void> {}
	async cleanUpHighlights(): Promise<void> {}
	dispose(): void {}

	private buildContent(step: number): string {
		const base = step * 100
		const rows = Array.from({ length: 35 }, (_, row) => {
			const id = row === 17 ? 'pg-1' : `pg-${row + 2}`
			return `[${base + row + 20}]<button>Open ${id} ${this.longRowEvidence(step, row)}</button>`
		})
		return [
			`[${base + 1}]<button>Overview</button>`,
			`[${base + 2}]<button>PG</button>`,
			`[${base + 3}]<input placeholder="Search PG" />`,
			`[${base + 4}]<select>Status filter</select>`,
			`[${base + 5}]<button>Logs tab</button>`,
			`[${base + 6}]<button>Alerts tab</button>`,
			`[${base + 7}]<button>Create P1 ticket</button>`,
			...rows,
		].join('\n')
	}

	private longRowEvidence(step: number, row: number): string {
		return [
			`status=${row % 3 === 0 ? 'critical' : 'warning'}`,
			`region=cn-${(row % 4) + 1}`,
			`lastError=2026-05-26T${String((step + row) % 24).padStart(2, '0')}:00:00Z`,
			`evidence=${'replica lag '.repeat(8)}`,
		].join(' ')
	}
}

const TASK = [
	'帮我诊断 PG pg-1 的最近失败原因：',
	'1. 在 PG 列表中搜索 pg-1；',
	'2. 打开 pg-1 详情；',
	'3. 查看错误摘要、最近 3 条日志和相关告警；',
	'4. 如果发现最近 1 小时内有高危告警，创建一条 P1 工单；',
	'5. 工单标题包含 PG id 和错误类型，描述中列出证据；',
	'6. 最后用 markdown 总结诊断过程、证据、是否创建工单。',
].join('\n')

describe('Page Agent control acceptance simulation', () => {
	it('TC-07 keeps complex long-running context bounded and avoids stale DOM indexes', async () => {
		const legacy = await runLongTaskSimulation(false)
		const bounded = await runLongTaskSimulation(true)
		const report = buildReport(legacy, bounded)
		mkdirSync('output/page-agent', { recursive: true })
		writeFileSync(
			join('output/page-agent', 'tc-07-long-context-report.json'),
			JSON.stringify(report, null, 2)
		)

		expect(bounded.steps).toBeGreaterThanOrEqual(25)
		expect(bounded.doneSuccess).toBe(true)
		expect(bounded.contextLengthError).toBe(false)
		expect(bounded.metrics.some((metric) => metric.hasSessionSummary)).toBe(true)
		expect(bounded.metrics.every((metric) => !metric.sessionSummaryHasDomIndex)).toBe(true)
		expect(bounded.metrics.every((metric) => metric.recentStepCount <= 6)).toBe(true)
		expect(bounded.metrics.every((metric) => actionIndexIsCurrent(metric))).toBe(true)

		const step12 = bounded.metrics.find((metric) => metric.step === 12)
		const final = bounded.metrics.at(-1)
		expect(step12).toBeDefined()
		expect(final).toBeDefined()
		expect(final!.characters).toBeLessThanOrEqual(Math.floor(step12!.characters * 1.5))
		expect(final!.characters).toBeLessThanOrEqual(64_000 * 3)

		expect(legacy.finalPromptCharacters).toBeGreaterThan(bounded.finalPromptCharacters)
	}, 30_000)
})

async function runLongTaskSimulation(contextEnabled: boolean) {
	const pageController = new FakePageController()
	const plannedActions = makeLongTaskPlan()
	const metrics: PromptMetric[] = []
	let invokeCount = 0

	const agent = new PageAgentCore({
		baseURL: 'https://fake-llm.example.test/v1',
		model: 'fake-planner',
		apiKey: 'test',
		pageController: pageController as unknown as PageController,
		maxSteps: 40,
		stepDelay: 0,
		context: contextEnabled
			? {
					enabled: true,
					maxPromptTokens: 64_000,
					recentStepCount: 6,
					compactAfterSteps: 12,
				}
			: { enabled: false },
		customFetch: async (_url, init) => {
			const rawBody = typeof init?.body === 'string' ? init.body : '{}'
			const body = JSON.parse(rawBody) as {
				messages: { role: string; content: string }[]
			}
			const userPrompt = body.messages.find((message) => message.role === 'user')?.content ?? ''
			const plannedAction = plannedActions[invokeCount] ?? plannedActions.at(-1)!
			metrics.push(buildPromptMetric(invokeCount, userPrompt, plannedAction))
			invokeCount++

			return new Response(
				JSON.stringify({
					choices: [
						{
							finish_reason: 'tool_calls',
							message: {
								tool_calls: [
									{
										id: `call-${invokeCount}`,
										type: 'function',
										function: {
											name: 'AgentOutput',
											arguments: JSON.stringify(toMacroToolInput(plannedAction)),
										},
									},
								],
							},
						},
					],
					usage: {
						prompt_tokens: Math.ceil(userPrompt.length / 3),
						completion_tokens: 20,
						total_tokens: Math.ceil(userPrompt.length / 3) + 20,
					},
				}),
				{ headers: { 'content-type': 'application/json' } }
			)
		},
	})

	const result = await agent.execute(TASK)
	const finalPromptCharacters = metrics.at(-1)?.characters ?? 0

	return {
		contextEnabled,
		steps: metrics.length,
		doneSuccess: result.success,
		contextLengthError: result.history.some(
			(event) => event.type === 'error' && /context/i.test(event.message)
		),
		finalPromptCharacters,
		metrics,
	}
}

function buildReport(
	legacy: Awaited<ReturnType<typeof runLongTaskSimulation>>,
	bounded: Awaited<ReturnType<typeof runLongTaskSimulation>>
) {
	const step12 = bounded.metrics.find((metric) => metric.step === 12)
	const final = bounded.metrics.at(-1)
	return {
		taskName: 'tc-07-pg-diagnosis-long-context',
		legacy: {
			contextEnabled: false,
			steps: legacy.steps,
			finalPromptCharacters: legacy.finalPromptCharacters,
			maxPromptCharacters: Math.max(...legacy.metrics.map((metric) => metric.characters)),
		},
		bounded: {
			contextEnabled: true,
			steps: bounded.steps,
			maxPromptCharacters: Math.max(...bounded.metrics.map((metric) => metric.characters)),
			promptCharactersAtStep12: step12?.characters,
			promptCharactersAtFinalStep: final?.characters,
			finalVsStep12Ratio:
				step12 && final ? Number((final.characters / step12.characters).toFixed(3)) : undefined,
			summaryHasDomIndex: bounded.metrics.some((metric) => metric.sessionSummaryHasDomIndex),
			contextLengthError: bounded.contextLengthError,
			doneSuccess: bounded.doneSuccess,
			hasSessionSummaryAfterCompact: bounded.metrics
				.filter((metric) => metric.step >= 12)
				.every((metric) => metric.hasSessionSummary),
			maxRecentStepCount: Math.max(...bounded.metrics.map((metric) => metric.recentStepCount)),
			allActionIndexesFromCurrentState: bounded.metrics.every((metric) =>
				actionIndexIsCurrent(metric)
			),
		},
	}
}

function makeLongTaskPlan(): PlannerAction[] {
	const actions: PlannerAction[] = [
		{
			name: 'click_element_by_index',
			input: { index: 102 },
			output: 'open PG navigation',
		},
		{
			name: 'input_text',
			input: { index: 203, text: 'pg-1' },
			output: 'search pg-1',
		},
	]

	for (let step = 2; step < 26; step++) {
		const base = (step + 1) * 100
		const cycle = step % 5
		if (cycle === 0) {
			actions.push({
				name: 'click_element_by_index',
				input: { index: base + 5 },
				output: 'open logs tab',
			})
		} else if (cycle === 1) {
			actions.push({
				name: 'click_element_by_index',
				input: { index: base + 6 },
				output: 'open alerts tab',
			})
		} else if (cycle === 2) {
			actions.push({ name: 'scroll', input: { down: true, num_pages: 0.5 }, output: 'scroll list' })
		} else if (cycle === 3) {
			actions.push({
				name: 'click_element_by_index',
				input: { index: base + 37 },
				output: 'open pg-1 detail row',
			})
		} else {
			actions.push({
				name: 'select_dropdown_option',
				input: { index: base + 4, text: 'Critical' },
				output: 'filter critical alerts',
			})
		}
	}

	actions.push({
		name: 'done',
		input: {
			success: true,
			text: [
				'## 诊断结论',
				'PG pg-1 最近失败由 replica lag 和高危告警共同触发。',
				'## 证据',
				'- 日志证据 1：replica lag spike',
				'- 日志证据 2：write timeout',
				'- 告警证据 3：P1 high risk alert',
				'## 工单',
				'已创建 P1 工单。',
			].join('\n'),
		},
		output: 'done',
	})

	return actions
}

function buildPromptMetric(
	step: number,
	userPrompt: string,
	plannedAction: PlannerAction
): PromptMetric {
	const sessionSummary = extractTag(userPrompt, 'session_summary')
	return {
		step,
		characters: userPrompt.length,
		hasSessionSummary: userPrompt.includes('<session_summary>'),
		recentStepCount: (extractTag(userPrompt, 'recent_steps').match(/<step_[0-9]+>/g) ?? []).length,
		sessionSummaryHasDomIndex: /\[[0-9]+\]/.test(sessionSummary),
		currentStateIndexes: extractCurrentStateIndexes(userPrompt),
		actionName: plannedAction.name,
		actionIndex: getActionIndex(plannedAction),
	}
}

function toMacroToolInput(action: PlannerAction): MacroToolInput {
	return {
		evaluation_previous_goal: `Step output reviewed: ${action.output}. Verdict: Success`,
		memory:
			'Continuing PG pg-1 diagnosis with evidence from logs, alerts, filters, and ticket workflow.',
		next_goal: `Run ${action.name} for the next PG diagnosis subtask.`,
		action: {
			[action.name]: action.input,
		},
	}
}

function extractTag(prompt: string, tag: string): string {
	const match = new RegExp(`<${tag}>\\n([\\s\\S]*?)\\n</${tag}>`).exec(prompt)
	return match?.[1] ?? ''
}

function extractCurrentStateIndexes(prompt: string): number[] {
	const browserState = extractTag(prompt, 'browser_state')
	return Array.from(browserState.matchAll(/\[([0-9]+)\]/g), (match) => Number(match[1]))
}

function getActionIndex(action: PlannerAction): number | undefined {
	const rawIndex = action.input.index
	return typeof rawIndex === 'number' ? rawIndex : undefined
}

function actionIndexIsCurrent(metric: PromptMetric): boolean {
	if (metric.actionName === 'done' || metric.actionIndex === undefined) return true
	return metric.currentStateIndexes.includes(metric.actionIndex)
}
