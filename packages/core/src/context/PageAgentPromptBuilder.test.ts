import { describe, expect, it } from 'vitest'

import { PageAgentPromptBuilder } from './PageAgentPromptBuilder'

describe('PageAgentPromptBuilder', () => {
	it('renders a legacy-compatible prompt with agent history and browser state', () => {
		const prompt = new PageAgentPromptBuilder().buildLegacyPrompt({
			instructions: '<instructions>Use the app carefully.</instructions>\n\n',
			task: 'Find the latest invoice',
			stepCount: 1,
			maxSteps: 40,
			currentTime: '2026/5/26 20:00:00',
			history: [
				{
					type: 'step',
					stepIndex: 0,
					reflection: {
						evaluation_previous_goal: 'Clicked search. Verdict: Success',
						memory: 'Search panel is open.',
						next_goal: 'Type invoice id.',
					},
					action: {
						name: 'click_element_by_index',
						input: { index: 3 },
						output: '✅ Clicked element ([3]<button>Search</button>).',
					},
					usage: {
						promptTokens: 1,
						completionTokens: 1,
						totalTokens: 2,
					},
				},
				{ type: 'observation', content: 'Page navigated to → https://example.com' },
			],
			browserState: {
				url: 'https://example.com',
				title: 'Example',
				header: 'Current Page: [Example](https://example.com)',
				content: '[0]<input placeholder=Search />',
				footer: '[End of page]',
			},
			pageContent: '[0]<input placeholder=Search />',
		})

		expect(prompt).toContain('<instructions>Use the app carefully.</instructions>')
		expect(prompt).toContain('<agent_state>')
		expect(prompt).toContain('<agent_history>')
		expect(prompt).toContain('<step_1>')
		expect(prompt).toContain('Action Results: ✅ Clicked element')
		expect(prompt).toContain('<sys>Page navigated to → https://example.com</sys>')
		expect(prompt).toContain('<browser_state>')
		expect(prompt).toContain('[0]<input placeholder=Search />')
	})

	it('renders sectioned context with stale-index safety rules', () => {
		const prompt = new PageAgentPromptBuilder().buildContextPrompt({
			instructions: [],
			agentState: {
				key: 'task',
				title: 'Agent State',
				content: '<user_request>Find invoice</user_request>',
				priority: 100,
				required: true,
			},
			protectedFacts: {
				key: 'protectedFacts',
				title: 'Protected Facts',
				content: '- User confirmed read-only mode.',
				priority: 95,
				required: true,
			},
			recentSteps: {
				key: 'recentSteps',
				title: 'Recent Steps',
				content: '<step_1>...</step_1>',
				priority: 80,
			},
			currentPage: {
				key: 'currentPage',
				title: 'Current Page',
				content: '[0]<button>Search</button>',
				priority: 100,
				required: true,
			},
			safetyRules: {
				key: 'safetyRules',
				title: 'Context Rules',
				content: 'Only use indexes in current browser_state.',
				priority: 100,
				required: true,
			},
			budgetReport: {
				maxPromptTokens: 32_000,
				estimatedTokens: 100,
				items: [],
				triggeredCompact: false,
			},
		})

		expect(prompt).toContain('<context_rules>')
		expect(prompt).toContain('Only use indexes in current browser_state.')
		expect(prompt).toContain('<protected_facts>')
		expect(prompt).toContain('<recent_steps>')
		expect(prompt).toContain('<browser_state>')
	})
})
