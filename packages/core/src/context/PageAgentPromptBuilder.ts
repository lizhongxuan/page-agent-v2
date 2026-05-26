import type { BrowserState } from '@page-agent/page-controller'

import type { HistoricalEvent } from '../types'
import type { ContextPack } from './types'

export interface BuildLegacyPromptInput {
	instructions: string
	task: string
	stepCount: number
	maxSteps: number
	currentTime: string
	history: HistoricalEvent[]
	browserState: BrowserState
	pageContent: string
}

export class PageAgentPromptBuilder {
	buildLegacyPrompt(input: BuildLegacyPromptInput): string {
		let prompt = ''
		prompt += input.instructions
		prompt += '<agent_state>\n'
		prompt += '<user_request>\n'
		prompt += `${input.task}\n`
		prompt += '</user_request>\n'
		prompt += '<step_info>\n'
		prompt += `Step ${input.stepCount + 1} of ${input.maxSteps} max possible steps\n`
		prompt += `Current time: ${input.currentTime}\n`
		prompt += '</step_info>\n'
		prompt += '</agent_state>\n\n'

		prompt += '<agent_history>\n'
		let renderedStepIndex = 0
		for (const event of input.history) {
			if (event.type === 'step') {
				renderedStepIndex++
				prompt += `<step_${renderedStepIndex}>\n`
				prompt += `Evaluation of Previous Step: ${event.reflection.evaluation_previous_goal}\n`
				prompt += `Memory: ${event.reflection.memory}\n`
				prompt += `Next Goal: ${event.reflection.next_goal}\n`
				prompt += `Action Results: ${event.action.output}\n`
				prompt += `</step_${renderedStepIndex}>\n`
			} else if (event.type === 'observation') {
				prompt += `<sys>${event.content}</sys>\n`
			} else if (event.type === 'user_takeover') {
				prompt += '<sys>User took over control and made changes to the page</sys>\n'
			}
		}
		prompt += '</agent_history>\n\n'

		prompt += '<browser_state>\n'
		prompt += input.browserState.header + '\n'
		prompt += input.pageContent + '\n'
		prompt += input.browserState.footer + '\n\n'
		prompt += '</browser_state>\n\n'

		return prompt
	}

	buildContextPrompt(pack: ContextPack): string {
		let prompt = ''
		for (const section of pack.instructions) {
			if (section.content.trim()) prompt += section.content + '\n\n'
		}

		prompt += '<agent_state>\n'
		prompt += pack.agentState.content + '\n'
		prompt += '<context_rules>\n'
		prompt += pack.safetyRules.content + '\n'
		prompt += '</context_rules>\n'
		prompt += '</agent_state>\n\n'

		if (pack.sessionSummary?.content.trim()) {
			prompt += '<session_summary>\n'
			prompt += pack.sessionSummary.content + '\n'
			prompt += '</session_summary>\n\n'
		}

		if (pack.protectedFacts.content.trim()) {
			prompt += '<protected_facts>\n'
			prompt += pack.protectedFacts.content + '\n'
			prompt += '</protected_facts>\n\n'
		}

		prompt += '<recent_steps>\n'
		prompt += pack.recentSteps.content + '\n'
		prompt += '</recent_steps>\n\n'

		prompt += '<browser_state>\n'
		prompt += pack.currentPage.content + '\n'
		prompt += '</browser_state>\n\n'

		if (pack.budgetReport.items.length > 0) {
			prompt += '<context_budget>\n'
			prompt += `Estimated tokens: ${pack.budgetReport.estimatedTokens}/${pack.budgetReport.maxPromptTokens}\n`
			prompt += '</context_budget>\n\n'
		}

		return prompt
	}
}
