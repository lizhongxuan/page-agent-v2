import { PageAgentCore } from '@page-agent/core'
import { type BrowserState, type PageController } from '@page-agent/page-controller'
import { describe, expect, it } from 'vitest'

class BatchPageController extends EventTarget {
	readonly actions: string[] = []
	private observes = 0

	async getBrowserState(): Promise<BrowserState> {
		this.observes++
		return {
			url: 'https://console.example.test/batch',
			title: `Batch Fixture ${this.observes}`,
			header: [
				`Current Page: [Batch Fixture ${this.observes}](https://console.example.test/batch)`,
				'Page info: 1280x720px viewport, 1280x720px total page size, 0.0 pages above, 0.0 pages below, 1.0 total pages, at 0% of page',
				'',
				'Interactive elements from top layer of the current page (full page):',
				'',
				'[Start of page]',
			].join('\n'),
			content: [
				'[1]<button>Open evidence 1: replica lag spike</button>',
				'[2]<button>Open evidence 2: write timeout</button>',
				'[3]<input placeholder="Ticket title" />',
				'[4]<select>Priority</select>',
			].join('\n'),
			footer: '[End of page]',
		}
	}

	async getLastUpdateTime(): Promise<number> {
		return Date.now()
	}

	async clickElement(index: number) {
		this.actions.push(`click:${index}`)
		return { success: true, message: `✅ Clicked ${index}.` }
	}

	async inputText(index: number, text: string) {
		this.actions.push(`input:${index}:${text}`)
		return { success: true, message: `✅ Input ${text} into ${index}.` }
	}

	async selectOption(index: number, text: string) {
		this.actions.push(`select:${index}:${text}`)
		return { success: true, message: `✅ Selected ${text} in ${index}.` }
	}

	async scroll() {
		this.actions.push('scroll')
		return { success: true, message: '✅ Scrolled.' }
	}

	async scrollHorizontally() {
		this.actions.push('scroll-horizontally')
		return { success: true, message: '✅ Scrolled horizontally.' }
	}

	async showMask(): Promise<void> {}
	async hideMask(): Promise<void> {}
	async cleanUpHighlights(): Promise<void> {}
	dispose(): void {}
}

describe('PageAgentCore multi-action execution', () => {
	it('executes a bounded batch from one LLM decision and records every action', async () => {
		const pageController = new BatchPageController()
		let invokeCount = 0

		const agent = new PageAgentCore({
			baseURL: 'https://fake-llm.example.test/v1',
			model: 'fake-planner',
			apiKey: 'test',
			pageController: pageController as unknown as PageController,
			maxSteps: 10,
			stepDelay: 0,
			customFetch: async () => {
				invokeCount++
				const payload =
					invokeCount === 1
						? {
								evaluation_previous_goal: 'Ready to inspect visible evidence. Verdict: Success',
								memory: 'Evidence controls and ticket controls are visible in the current state.',
								next_goal: 'Inspect visible evidence and prepare ticket fields.',
								action: [
									{ click_element_by_index: { index: 1 } },
									{ click_element_by_index: { index: 2 } },
									{ input_text: { index: 3, text: 'pg-1 replica lag P1' } },
									{ select_dropdown_option: { index: 4, text: 'P1' } },
								],
							}
						: {
								evaluation_previous_goal: 'The batched page actions succeeded. Verdict: Success',
								memory: 'Evidence was inspected and ticket fields were prepared.',
								next_goal: 'Finish with the observed result.',
								action: {
									done: {
										success: true,
										text: 'Batch completed.',
									},
								},
							}

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
					}),
					{ headers: { 'content-type': 'application/json' } }
				)
			},
		})

		const result = await agent.execute('Inspect evidence cards and prepare a P1 ticket.')

		expect(result.success).toBe(true)
		expect(invokeCount).toBe(2)
		expect(pageController.actions).toEqual([
			'click:1',
			'click:2',
			'input:3:pg-1 replica lag P1',
			'select:4:P1',
		])
		expect(result.history.filter((event) => event.type === 'step')).toHaveLength(5)
		expect(
			result.history.filter((event) => event.type === 'step').map((event) => event.action.name)
		).toEqual([
			'click_element_by_index',
			'click_element_by_index',
			'input_text',
			'select_dropdown_option',
			'done',
		])
	})
})
