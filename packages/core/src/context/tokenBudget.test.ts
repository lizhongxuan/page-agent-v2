import { describe, expect, it } from 'vitest'

import { estimateTokens, makeBudgetReport } from './tokenBudget'

describe('tokenBudget', () => {
	it('estimates tokens conservatively from characters', () => {
		expect(estimateTokens('abcdef')).toBe(2)
		expect(estimateTokens('')).toBe(0)
	})

	it('reports section sizes and total estimated tokens', () => {
		const report = makeBudgetReport(
			[
				{ key: 'task', title: 'Task', content: 'abc', priority: 100, required: true },
				{
					key: 'currentPage',
					title: 'Page',
					content: 'abcdef',
					priority: 90,
					required: true,
				},
			],
			32_000,
			false
		)

		expect(report.estimatedTokens).toBe(3)
		expect(report.items).toHaveLength(2)
		expect(report.items[0]).toMatchObject({ key: 'task', status: 'kept' })
	})
})
