import type { BrowserState } from '@page-agent/page-controller'

import type { AgentStepEvent, HistoricalEvent } from '../types'
import type { BrowserSessionSummary } from './types'

export interface CompactInput {
	task: string
	history: HistoricalEvent[]
	browserState: BrowserState
}

export class BrowserCompactManager {
	compact(input: CompactInput): BrowserSessionSummary {
		const stepEvents = input.history.filter(
			(event): event is AgentStepEvent => event.type === 'step'
		)
		const failedAttempts = stepEvents
			.filter((event) => this.isFailure(event))
			.map((event) => this.sanitizeActionLine(event))
		const completed = stepEvents
			.filter((event) => !this.isFailure(event))
			.map((event) => this.sanitizeActionLine(event))
		const navigationTrail = input.history
			.filter(
				(event) => event.type === 'observation' && event.content.includes('Page navigated to')
			)
			.map((event) => (event.type === 'observation' ? this.redactDomIndexes(event.content) : ''))
			.filter(Boolean)
		const lastGoal = this.redactDomIndexes(stepEvents.at(-1)?.reflection.next_goal ?? '')

		return {
			userGoal: input.task,
			currentSubGoal: lastGoal,
			completed,
			failedAttempts,
			stableFacts: navigationTrail,
			businessObjects: [],
			userDecisions: [],
			riskDecisions: [],
			currentPageSemanticState: {
				url: input.browserState.url,
				title: input.browserState.title,
				semanticLocation: input.browserState.header.split('\n')[0] ?? '',
				visibleEvidence: input.browserState.content
					.split('\n')
					.slice(0, 5)
					.map((line) => this.redactDomIndexes(line, '[current-index-redacted]')),
			},
			doNotRepeat: failedAttempts,
			nextRecommendedActions: lastGoal ? [lastGoal] : [],
		}
	}

	private isFailure(event: AgentStepEvent): boolean {
		const output = event.action.output.toLowerCase()
		return event.action.output.startsWith('❌') || output.includes('failed')
	}

	private sanitizeActionLine(event: AgentStepEvent): string {
		return this.redactDomIndexes(`${event.action.name}: ${event.action.output}`)
	}

	private redactDomIndexes(text: string, replacement = '[stale-index]'): string {
		return text.replace(/\[[0-9]+]/g, replacement)
	}
}
