import { describe, expect, it } from 'vitest'

import { BrowserCompactManager } from './BrowserCompactManager'

describe('BrowserCompactManager', () => {
	it('summarizes older history without preserving executable DOM indexes', () => {
		const summary = new BrowserCompactManager().compact({
			task: 'Check order 123',
			history: [
				{
					type: 'step',
					stepIndex: 0,
					reflection: {
						evaluation_previous_goal: 'Opened search. Verdict: Success',
						memory: 'Search page is open.',
						next_goal: 'Search order.',
					},
					action: {
						name: 'click_element_by_index',
						input: { index: 12 },
						output: '✅ Clicked element ([12]<button>Search</button>).',
					},
					usage: { promptTokens: 1, completionTokens: 1, totalTokens: 2 },
				},
				{ type: 'observation', content: 'Page navigated to → https://example.com/orders' },
			],
			browserState: {
				url: 'https://example.com/orders',
				title: 'Orders',
				header: 'Current Page: [Orders](https://example.com/orders)',
				content: '[0]<input placeholder=Order ID />',
				footer: '[End of page]',
			},
		})

		expect(summary.userGoal).toBe('Check order 123')
		expect(summary.currentPageSemanticState.url).toBe('https://example.com/orders')
		expect(JSON.stringify(summary)).not.toContain('[12]')
		expect(summary.completed[0]).toContain('click_element_by_index')
	})
})
