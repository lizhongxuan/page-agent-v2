import type { AgentStepEvent } from '../types'
import { BrowserCompactManager } from './BrowserCompactManager'
import type { StoredContext } from './ContextStore'
import { makeBudgetReport } from './tokenBudget'
import {
	type BuildContextPackInput,
	type ContextConfig,
	type ContextPack,
	type ContextSection,
	DEFAULT_CONTEXT_CONFIG,
} from './types'

export interface ContextRuntimeOptions {
	config?: ContextConfig
	taskId: string
}

export class ContextRuntime {
	readonly config: Required<ContextConfig>
	private readonly taskId: string
	private readonly compactManager = new BrowserCompactManager()
	private readonly recordedSteps: AgentStepEvent[] = []

	constructor(options: ContextRuntimeOptions) {
		this.config = { ...DEFAULT_CONTEXT_CONFIG, ...(options.config ?? {}) }
		this.taskId = options.taskId
	}

	buildPack(input: BuildContextPackInput): ContextPack {
		const shouldCompact = input.stepCount >= this.config.compactAfterSteps
		const compactHistory = input.history.slice(
			0,
			Math.max(0, input.history.length - this.config.recentStepCount)
		)
		const summary = shouldCompact
			? this.compactManager.compact({
					task: input.task,
					history: compactHistory,
					browserState: input.browserState,
				})
			: undefined
		const sessionSummary: ContextSection | undefined = summary
			? {
					key: 'sessionSummary',
					title: 'Session Summary',
					priority: 70,
					content: JSON.stringify(summary, null, 2),
				}
			: undefined

		const agentState: ContextSection = {
			key: 'task',
			title: 'Agent State',
			required: true,
			priority: 100,
			content: [
				'<user_request>',
				input.task,
				'</user_request>',
				'<step_info>',
				`Step ${input.stepCount + 1} of ${input.maxSteps} max possible steps`,
				`Current time: ${input.currentTime}`,
				'</step_info>',
			].join('\n'),
		}
		const recentSteps: ContextSection = {
			key: 'recentSteps',
			title: 'Recent Steps',
			priority: 80,
			content: this.renderRecentSteps(input.history),
		}
		const currentPage: ContextSection = {
			key: 'currentPage',
			title: 'Current Page',
			required: true,
			priority: 100,
			content: [
				input.browserState.header,
				input.browserState.content,
				input.browserState.footer,
			].join('\n'),
		}
		const protectedFacts: ContextSection = {
			key: 'protectedFacts',
			title: 'Protected Facts',
			required: true,
			priority: 95,
			content: '',
		}
		const safetyRules: ContextSection = {
			key: 'safetyRules',
			title: 'Context Rules',
			required: true,
			priority: 100,
			content: [
				'Current browser_state is the source of truth for page operations.',
				'Element indexes from history, summaries, or memory are stale and must not be reused.',
				'If summary conflicts with current browser_state, trust current browser_state.',
			].join('\n'),
		}
		const instructions: ContextSection[] = input.instructions
			? [
					{
						key: 'instructions',
						title: 'Instructions',
						content: input.instructions,
						priority: 90,
					},
				]
			: []
		const sections = [
			...instructions,
			agentState,
			...(sessionSummary ? [sessionSummary] : []),
			protectedFacts,
			recentSteps,
			currentPage,
			safetyRules,
		]
		const budgetReport = makeBudgetReport(sections, this.config.maxPromptTokens, shouldCompact)

		return {
			instructions,
			agentState,
			sessionSummary,
			protectedFacts,
			recentSteps,
			currentPage,
			safetyRules,
			budgetReport,
		}
	}

	recordStep(step: AgentStepEvent): void {
		this.recordedSteps.push(step)
	}

	toStoredContext(): StoredContext {
		return {
			taskId: this.taskId,
			updatedAt: Date.now(),
			summary: undefined,
			recentEvents: this.recordedSteps.slice(-this.config.recentStepCount),
			budgetReport: undefined,
		}
	}

	private renderRecentSteps(history: BuildContextPackInput['history']): string {
		const stepEvents = history.filter((event): event is AgentStepEvent => event.type === 'step')
		const recent = stepEvents.slice(-this.config.recentStepCount)
		return recent
			.map((event, index) =>
				[
					`<step_${index + 1}>`,
					`Evaluation of Previous Step: ${event.reflection.evaluation_previous_goal ?? ''}`,
					`Memory: ${event.reflection.memory ?? ''}`,
					`Next Goal: ${event.reflection.next_goal ?? ''}`,
					`Action Results: ${event.action.output}`,
					`</step_${index + 1}>`,
				].join('\n')
			)
			.join('\n')
	}
}
