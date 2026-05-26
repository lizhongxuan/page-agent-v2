import { describe, expect, it } from 'vitest'

import { ContextRuntime } from './ContextRuntime'

describe('ContextRuntime', () => {
	it('builds a context pack with current page and budget report', () => {
		const runtime = new ContextRuntime({
			config: { enabled: true },
			taskId: 'task-1',
		})

		const pack = runtime.buildPack({
			task: 'Find invoice',
			stepCount: 0,
			maxSteps: 40,
			currentTime: '2026/5/26 20:00:00',
			instructions: '',
			history: [],
			browserState: {
				url: 'https://example.com',
				title: 'Example',
				header: 'Current Page: [Example](https://example.com)',
				content: '[0]<button>Search</button>',
				footer: '[End of page]',
			},
		})

		expect(pack.agentState.content).toContain('Find invoice')
		expect(pack.currentPage.content).toContain('[0]<button>Search</button>')
		expect(pack.safetyRules.content).toContain('current browser_state')
		expect(pack.budgetReport.estimatedTokens).toBeGreaterThan(0)
	})

	it('keeps only recent steps in recentSteps section after compact threshold', () => {
		const runtime = new ContextRuntime({
			config: { enabled: true, recentStepCount: 2, compactAfterSteps: 3 },
			taskId: 'task-1',
		})

		const history = Array.from({ length: 5 }, (_, stepIndex) => ({
			type: 'step' as const,
			stepIndex,
			reflection: {
				evaluation_previous_goal: `step ${stepIndex} done`,
				memory: `memory ${stepIndex}`,
				next_goal: `next ${stepIndex}`,
			},
			action: {
				name: 'wait',
				input: { seconds: 1 },
				output: `✅ Waited ${stepIndex}`,
			},
			usage: { promptTokens: 1, completionTokens: 1, totalTokens: 2 },
		}))

		const pack = runtime.buildPack({
			task: 'Long task',
			stepCount: 5,
			maxSteps: 40,
			currentTime: '2026/5/26 20:00:00',
			instructions: '',
			history,
			browserState: {
				url: 'https://example.com',
				title: 'Example',
				header: 'Current Page: [Example](https://example.com)',
				content: '[0]<button>Search</button>',
				footer: '[End of page]',
			},
		})

		expect(pack.recentSteps.content).toContain('Waited 3')
		expect(pack.recentSteps.content).toContain('Waited 4')
		expect(pack.recentSteps.content).not.toContain('Waited 0')
		expect(pack.sessionSummary?.content).toContain('Long task')
	})

	it('exports a lightweight stored context snapshot', () => {
		const runtime = new ContextRuntime({ config: { enabled: true }, taskId: 'task-1' })
		const snapshot = runtime.toStoredContext()

		expect(snapshot.taskId).toBe('task-1')
		expect(snapshot.recentEvents).toEqual([])
		expect(snapshot.updatedAt).toEqual(expect.any(Number))
	})
})
