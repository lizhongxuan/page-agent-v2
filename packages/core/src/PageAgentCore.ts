/**
 * Copyright (C) 2025 Alibaba Group Holding Limited
 * Copyright (C) 2026 SimonLuvRamen
 * All rights reserved.
 */
import { InvokeError, LLM, type Tool } from '@page-agent/llms'
import type { BrowserState, PageController } from '@page-agent/page-controller'
import chalk from 'chalk'
import * as z from 'zod/v4'

import { ContextRuntime, PageAgentPromptBuilder } from './context'
import SYSTEM_PROMPT from './prompts/system_prompt.md?raw'
import {
	createSearchExplorationState,
	observeSearchPage,
	shouldBlockSearchPaginationClick,
} from './searchExplorationGuard'
import { normalizeToolOutput, shouldStopBatchForOutput } from './toolOutput'
import { type PageAgentTool, tools } from './tools'
import type {
	AgentActivity,
	AgentConfig,
	AgentReflection,
	AgentStatus,
	AgentStepEvent,
	ExecutionResult,
	HistoricalEvent,
	MacroToolActionResult,
	MacroToolInput,
	MacroToolResult,
} from './types'
import { assert, fetchLlmsTxt, normalizeResponse, uid, waitFor } from './utils'

export { tool, type PageAgentTool } from './tools'
export * from './context'
export type * from './types'

export type PageAgentCoreConfig = AgentConfig & { pageController: PageController }

/**
 * AI agent for browser automation.
 *
 * @remarks
 * ## Re-act Agent Loop
 * - step
 *    - observe (gather information about current environment and context)
 *    - think (LLM calling)
 *      - reflection (evaluate history, generate memory, short-term planning)
 *      - action (give the action to approach the next goal)
 *    - act (execute the action)
 * - loop
 *
 * ## Event System
 * - `statuschange` - Agent status transitions (idle → running → completed/error)
 * - `historychange` - History events updated (persistent, part of agent memory)
 * - `activity` - Real-time activity feedback (transient, for UI only)
 * - `dispose` - Agent cleanup triggered
 *
 * ## Information Streams
 * 1. **History Events** (`history` array)
 *    - Persistent event stream that forms agent's memory
 *    - Included in LLM context across steps
 *    - Types: steps, observations, user takeovers, llm errors
 *
 * 2. **Activity Events** (via `activity` event)
 *    - Transient UI feedback during task execution
 *    - NOT included in LLM context
 *    - Types: thinking, executing, executed, retrying, error
 */
export class PageAgentCore extends EventTarget {
	readonly id = uid()
	readonly config: PageAgentCoreConfig & { maxSteps: number }
	readonly tools: typeof tools
	/** PageController for DOM operations */
	readonly pageController: PageController

	task = ''
	taskId = ''
	/** History events */
	history: HistoricalEvent[] = []
	/** Whether this agent has been disposed */
	disposed = false

	/**
	 * Callback for when agent needs user input (ask_user tool)
	 * If not set, ask_user tool will be disabled
	 * @example onAskUser: (q) => window.prompt(q) || ''
	 */
	onAskUser?: (question: string) => Promise<string>

	#status: AgentStatus = 'idle'
	#llm: LLM
	#abortController = new AbortController()
	#observations: string[] = []
	#promptBuilder = new PageAgentPromptBuilder()
	#contextRuntime: ContextRuntime | null = null

	/** internal states during a single task execution */
	#states = {
		/** Accumulated wait time in seconds */
		totalWaitTime: 0,
		/** For detecting navigation */
		lastURL: '',
		/** Browser state */
		browserState: null as BrowserState | null,
		/** Prevent open-ended search result pagination from running indefinitely. */
		searchExploration: createSearchExplorationState(),
	}

	constructor(config: PageAgentCoreConfig) {
		super()

		this.config = { ...config, maxSteps: config.maxSteps ?? 40 }

		this.#llm = new LLM(this.config)
		this.tools = new Map(tools)
		this.pageController = config.pageController

		// Listen to LLM retry events
		this.#llm.addEventListener('retry', (e) => {
			const { attempt, maxAttempts } = (e as CustomEvent).detail
			this.#emitActivity({ type: 'retrying', attempt, maxAttempts })
			// Also push to history for panel rendering
			this.history.push({
				type: 'retry',
				message: `LLM retry attempt ${attempt} of ${maxAttempts}`,
				attempt,
				maxAttempts,
			})
			this.#emitHistoryChange()
		})
		this.#llm.addEventListener('error', (e) => {
			const error = (e as CustomEvent).detail.error as Error | InvokeError
			if ((error as any)?.rawError?.name === 'AbortError') return
			const message = String(error)
			this.#emitActivity({ type: 'error', message })
			// Also push to history for panel rendering
			this.history.push({
				type: 'error',
				message,
				rawResponse: (error as InvokeError).rawResponse,
			})
			this.#emitHistoryChange()
		})

		if (this.config.customTools) {
			for (const [name, tool] of Object.entries(this.config.customTools)) {
				if (tool === null) {
					this.tools.delete(name)
					continue
				}
				this.tools.set(name, tool)
			}
		}

		if (!this.config.experimentalScriptExecutionTool) {
			this.tools.delete('execute_javascript')
		}
	}

	/** Get current agent status */
	get status(): AgentStatus {
		return this.#status
	}

	/** Emit statuschange event */
	#emitStatusChange(): void {
		this.dispatchEvent(new Event('statuschange'))
	}

	/** Emit historychange event */
	#emitHistoryChange(): void {
		this.dispatchEvent(new Event('historychange'))
	}

	/**
	 * Emit activity event - for transient UI feedback
	 * @param activity - Current agent activity
	 */
	#emitActivity(activity: AgentActivity): void {
		this.dispatchEvent(new CustomEvent('activity', { detail: activity }))
	}

	/** Update status and emit event */
	#setStatus(status: AgentStatus): void {
		if (this.#status !== status) {
			this.#status = status
			this.#emitStatusChange()
		}
	}

	/**
	 * Push an observation message to the history event stream.
	 * This will be visible in <agent_history> and remain persistent in memory across steps.
	 * @experimental @internal
	 * @note history change will be emitted before next step starts
	 */
	pushObservation(content: string): void {
		this.#observations.push(content)
	}

	/** Stop the current task. Agent remains reusable. */
	stop() {
		this.pageController.cleanUpHighlights()
		this.pageController.hideMask()
		this.#abortController.abort()
	}

	async execute(task: string): Promise<ExecutionResult> {
		if (this.disposed) throw new Error('PageAgent has been disposed. Create a new instance.')
		if (!task) throw new Error('Task is required')
		this.task = task
		this.taskId = uid()
		this.#contextRuntime = this.config.context?.enabled
			? new ContextRuntime({
					config: this.config.context,
					taskId: this.taskId,
				})
			: null

		// Disable ask_user tool if onAskUser is not set
		if (!this.onAskUser) {
			this.tools.delete('ask_user')
		}

		const onBeforeStep = this.config.onBeforeStep
		const onAfterStep = this.config.onAfterStep
		const onBeforeTask = this.config.onBeforeTask
		const onAfterTask = this.config.onAfterTask

		await onBeforeTask?.(this)

		// Show mask
		await this.pageController.showMask()

		if (this.#abortController) {
			this.#abortController.abort()
			this.#abortController = new AbortController()
		}

		this.history = []
		this.#setStatus('running')
		this.#emitHistoryChange()
		this.#observations = []

		// Reset internal states
		this.#states = {
			totalWaitTime: 0,
			lastURL: '',
			browserState: null,
			searchExploration: createSearchExplorationState(),
		}

		while (true) {
			const step = this.#getCompletedStepCount()
			try {
				console.group(`step: ${step}`)

				await onBeforeStep?.(this, step)

				// observe

				console.log(chalk.blue.bold('👀 Observing...'))

				this.#states.browserState = await this.pageController.getBrowserState()
				await this.#handleObservations(step)

				// assemble prompts

				const messages = [
					{ role: 'system' as const, content: this.#getSystemPrompt() },
					{ role: 'user' as const, content: await this.#assembleUserPrompt() },
				]

				const macroTool = { AgentOutput: this.#packMacroTool() }

				// invoke LLM

				console.log(chalk.blue.bold('🧠 Thinking...'))
				this.#emitActivity({ type: 'thinking' })

				const result = await this.#llm.invoke(messages, macroTool, this.#abortController.signal, {
					toolChoiceName: 'AgentOutput',
					normalizeResponse: (res) => normalizeResponse(res, this.tools),
				})

				// assemble history

				const macroResult = result.toolResult as MacroToolResult
				const input = macroResult.input
				const reflection: Partial<AgentReflection> = {
					evaluation_previous_goal: input.evaluation_previous_goal,
					memory: input.memory,
					next_goal: input.next_goal,
				}
				const actionResults =
					macroResult.actions ?? this.#toActionResults(input.action, macroResult.output)
				const stepEvents = actionResults.map((actionResult, index) => {
					return {
						type: 'step',
						stepIndex: step + index,
						reflection,
						action: {
							name: actionResult.name,
							input: actionResult.input,
							output: actionResult.output,
						},
						usage: index === 0 ? result.usage : this.#emptyUsage(),
						rawResponse: index === 0 ? result.rawResponse : undefined,
						rawRequest: index === 0 ? result.rawRequest : undefined,
					} as AgentStepEvent
				})

				for (const stepEvent of stepEvents) {
					this.history.push(stepEvent)
					this.#contextRuntime?.recordStep(stepEvent)
					if (this.#contextRuntime && this.config.contextStore) {
						await this.config.contextStore.save(this.taskId, this.#contextRuntime.toStoredContext())
					}
				}
				this.#emitHistoryChange()

				//

				await onAfterStep?.(this, this.history)

				console.groupEnd()

				// finish task if done

				const doneEvent = stepEvents.find((event) => event.action.name === 'done')
				if (doneEvent) {
					const success = doneEvent.action.input?.success ?? false
					const text = doneEvent.action.input?.text || 'no text provided'
					console.log(chalk.green.bold('Task completed'), success, text)
					this.#onDone(success)
					const result: ExecutionResult = {
						success,
						data: text,
						history: this.history,
					}
					await onAfterTask?.(this, result)
					return result
				}
			} catch (error: unknown) {
				console.groupEnd() // to prevent nested groups
				const isAbortError = (error as any)?.rawError?.name === 'AbortError'

				console.error('Task failed', error)
				const errorMessage = isAbortError ? 'Task stopped' : String(error)
				this.#emitActivity({ type: 'error', message: errorMessage })
				this.history.push({ type: 'error', message: errorMessage, rawResponse: error })
				this.#emitHistoryChange()
				this.#onDone(false)
				const result: ExecutionResult = {
					success: false,
					data: errorMessage,
					history: this.history,
				}
				await onAfterTask?.(this, result)
				return result
			}

			if (this.#getCompletedStepCount() > this.config.maxSteps) {
				const errorMessage = 'Step count exceeded maximum limit'
				this.history.push({ type: 'error', message: errorMessage })
				this.#emitHistoryChange()
				this.#onDone(false)
				const result: ExecutionResult = {
					success: false,
					data: errorMessage,
					history: this.history,
				}
				await onAfterTask?.(this, result)
				return result
			}

			await waitFor(this.config.stepDelay ?? 0.4)
		}
	}

	/**
	 * Merge all tools into a single MacroTool with the following input:
	 * - thinking: string
	 * - evaluation_previous_goal: string
	 * - memory: string
	 * - next_goal: string
	 * - action: { toolName: toolInput }
	 * where action must be selected from tools defined in this.tools
	 */
	#packMacroTool(): Tool<MacroToolInput, MacroToolResult> {
		const tools = this.tools

		const actionSchemas = Array.from(tools.entries()).map(([toolName, tool]) => {
			return z.object({ [toolName]: tool.inputSchema }).describe(tool.description)
		})

		const singleActionSchema = z.union(
			actionSchemas as unknown as [z.ZodType, z.ZodType, ...z.ZodType[]]
		)
		const actionSchema = z.union([singleActionSchema, z.array(singleActionSchema).min(1).max(24)])

		const macroToolSchema = z.object({
			// thinking: z.string().optional(),
			evaluation_previous_goal: z.string().optional(),
			memory: z.string().optional(),
			next_goal: z.string().optional(),
			action: actionSchema,
		})

		return {
			description: 'You MUST call this tool every step!',
			inputSchema: macroToolSchema as z.ZodType<MacroToolInput>,
			execute: async (input: MacroToolInput): Promise<MacroToolResult> => {
				// abort
				if (this.#abortController.signal.aborted) throw new Error('AbortError')

				console.log(chalk.blue.bold('MacroTool input'), input)
				const actions = Array.isArray(input.action) ? input.action : [input.action]
				const containsDone = actions.some((action) => Object.keys(action)[0] === 'done')
				if (containsDone && actions.length > 1) {
					throw new Error('The done action must be returned as a single action.')
				}

				// Build reflection text, only include non-empty fields
				const reflectionLines: string[] = []
				if (input.evaluation_previous_goal)
					reflectionLines.push(`✅: ${input.evaluation_previous_goal}`)
				if (input.memory) reflectionLines.push(`💾: ${input.memory}`)
				if (input.next_goal) reflectionLines.push(`🎯: ${input.next_goal}`)

				const reflectionText = reflectionLines.length > 0 ? reflectionLines.join('\n') : ''

				if (reflectionText) {
					console.log(reflectionText)
				}

				const actionResults: MacroToolActionResult[] = []
				for (const action of actions) {
					const actionResult = await this.#executeAction(action, tools)
					actionResults.push(actionResult)
					if (this.#shouldStopBatch(actionResult.output)) break
				}

				// Return structured result
				return {
					input,
					output: actionResults.map((action) => action.output).join('\n'),
					actions: actionResults,
				}
			},
		}
	}

	async #executeAction(
		action: Record<string, any>,
		tools: Map<string, PageAgentTool>
	): Promise<MacroToolActionResult> {
		const toolName = Object.keys(action)[0]
		const toolInput = action[toolName]

		// Find the corresponding tool
		const tool = tools.get(toolName)
		assert(tool, `Tool ${toolName} not found`)

		console.log(chalk.blue.bold(`Executing tool: ${toolName}`), toolInput)

		if (toolName === 'click_element_by_index') {
			const searchGuardMessage = shouldBlockSearchPaginationClick({
				state: this.#states.searchExploration,
				task: this.task,
				browserContent: this.#states.browserState?.content || '',
				index: Number(toolInput?.index),
			})
			if (searchGuardMessage) {
				this.pushObservation(searchGuardMessage)
				return {
					name: toolName,
					input: toolInput,
					output: `⚠️ ${searchGuardMessage}`,
				}
			}
		}

		// Emit executing activity
		this.#emitActivity({ type: 'executing', tool: toolName, input: toolInput })

		const startTime = Date.now()

		// Execute tool, bind `this` to PageAgent
		const rawResult = await tool.execute.bind(this)(toolInput)
		const result = normalizeToolOutput(rawResult, toolName)

		const duration = Date.now() - startTime
		console.log(chalk.green.bold(`Tool (${toolName}) executed for ${duration}ms`), result)

		// Emit executed activity
		this.#emitActivity({
			type: 'executed',
			tool: toolName,
			input: toolInput,
			output: result,
			duration,
		})

		// counting wait time
		if (toolName === 'wait') {
			this.#states.totalWaitTime += toolInput?.seconds || 0
		} else {
			this.#states.totalWaitTime = 0
		}

		return {
			name: toolName,
			input: toolInput,
			output: result,
		}
	}

	#toActionResults(action: MacroToolInput['action'], output: string): MacroToolActionResult[] {
		const actions = Array.isArray(action) ? action : [action]
		return actions.map((item) => {
			const name = Object.keys(item)[0]
			return {
				name,
				input: item[name],
				output,
			}
		})
	}

	#emptyUsage(): AgentStepEvent['usage'] {
		return {
			promptTokens: 0,
			completionTokens: 0,
			totalTokens: 0,
		}
	}

	#getCompletedStepCount(): number {
		return this.history.filter((event) => event.type === 'step').length
	}

	#shouldStopBatch(output: string): boolean {
		return shouldStopBatchForOutput(output)
	}

	/**
	 * Get system prompt, dynamically replace language settings based on configured language
	 */
	#getSystemPrompt(): string {
		if (this.config.customSystemPrompt) {
			return this.config.customSystemPrompt
		}

		const targetLanguage = this.config.language === 'zh-CN' ? '中文' : 'English'
		const systemPrompt = SYSTEM_PROMPT.replace(
			/Default working language: \*\*.*?\*\*/,
			`Default working language: **${targetLanguage}**`
		)

		return systemPrompt
	}

	/**
	 * Get instructions from config
	 */
	async #getInstructions(): Promise<string> {
		const { instructions, experimentalLlmsTxt } = this.config

		const systemInstructions = instructions?.system?.trim()
		let pageInstructions: string | undefined

		const url = this.#states.browserState?.url || ''
		if (instructions?.getPageInstructions && url) {
			try {
				pageInstructions = instructions.getPageInstructions(url)?.trim()
			} catch (error) {
				console.error(
					chalk.red('[PageAgent] Failed to execute getPageInstructions callback:'),
					error
				)
			}
		}

		const llmsTxt = experimentalLlmsTxt && url ? await fetchLlmsTxt(url) : undefined

		if (!systemInstructions && !pageInstructions && !llmsTxt) return ''

		let result = '<instructions>\n'

		if (systemInstructions) {
			result += `<system_instructions>\n${systemInstructions}\n</system_instructions>\n`
		}

		if (pageInstructions) {
			result += `<page_instructions>\n${pageInstructions}\n</page_instructions>\n`
		}

		if (llmsTxt) {
			result += `<llms_txt>\n${llmsTxt}\n</llms_txt>\n`
		}

		result += '</instructions>\n\n'

		return result
	}

	/**
	 * Generate system observations before each step
	 * @todo loop detection
	 * @todo console error
	 */
	async #handleObservations(step: number): Promise<void> {
		// Accumulated wait time warning
		if (this.#states.totalWaitTime >= 3) {
			this.pushObservation(
				`You have waited ${this.#states.totalWaitTime} seconds accumulatively. ` +
					`DO NOT wait any longer unless you have a good reason.`
			)
		}

		// Detect URL change
		const currentURL = this.#states.browserState?.url || ''
		if (currentURL !== this.#states.lastURL) {
			this.pushObservation(`Page navigated to → ${currentURL}`)
			this.#states.lastURL = currentURL
			await waitFor(0.5) // wait for page to stabilize
		}

		const searchObservation = observeSearchPage(
			this.#states.searchExploration,
			currentURL,
			this.task
		)
		if (searchObservation) {
			this.pushObservation(searchObservation)
		}

		// Remaining steps warning
		const remaining = this.config.maxSteps - step
		if (remaining === 5) {
			this.pushObservation(
				`⚠️ Only ${remaining} steps remaining. ` +
					`Consider wrapping up or calling done with partial results.`
			)
		} else if (remaining === 2) {
			this.pushObservation(
				`⚠️ Critical: Only ${remaining} steps left! You must finish the task or call done immediately.`
			)
		}

		// Push observations to history and emit
		if (this.#observations.length > 0) {
			for (const content of this.#observations) {
				this.history.push({ type: 'observation', content })
				console.log(chalk.cyan('Observation:'), content)
			}
			this.#observations = []
			this.#emitHistoryChange()
		}
	}

	async #assembleUserPrompt(): Promise<string> {
		const browserState = this.#states.browserState!
		let pageContent = browserState.content
		if (this.config.transformPageContent) {
			pageContent = await this.config.transformPageContent(pageContent)
		}

		const instructions = await this.#getInstructions()
		const stepCount = this.history.filter((e) => e.type === 'step').length
		const currentTime = new Date().toLocaleString()
		if (this.#contextRuntime) {
			const pack = this.#contextRuntime.buildPack({
				task: this.task,
				stepCount,
				maxSteps: this.config.maxSteps,
				currentTime,
				instructions,
				history: this.history,
				browserState: {
					...browserState,
					content: pageContent,
				},
			})

			return this.#promptBuilder.buildContextPrompt(pack)
		}

		return this.#promptBuilder.buildLegacyPrompt({
			instructions,
			task: this.task,
			stepCount,
			maxSteps: this.config.maxSteps,
			currentTime,
			history: this.history,
			browserState,
			pageContent,
		})
	}

	#onDone(success = true) {
		this.pageController.cleanUpHighlights()
		this.pageController.hideMask() // No await - fire and forget
		this.#setStatus(success ? 'completed' : 'error')
		this.#abortController.abort()
	}

	dispose() {
		console.log('Disposing PageAgent...')
		this.disposed = true
		this.pageController.dispose()
		// this.history = []
		this.#abortController.abort()

		// Emit dispose event for UI cleanup
		this.dispatchEvent(new Event('dispose'))

		this.config.onDispose?.(this)
	}
}
