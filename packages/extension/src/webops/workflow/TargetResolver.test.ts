import { describe, expect, it } from 'vitest'

import { resolveWorkflowTarget } from './TargetResolver'
import type { WorkflowTarget } from './types'

const elements = [
	{
		id: 'a',
		role: 'textbox',
		name: 'Search',
		placeholder: 'Search docs',
		visible: true,
		enabled: true,
	},
	{ id: 'b', role: 'button', name: 'Search', text: 'Search', visible: true, enabled: true },
	{ id: 'c', role: 'button', name: 'Hidden', text: 'Hidden', visible: false, enabled: true },
]

describe('resolveWorkflowTarget', () => {
	it('prefers role/name targets with high confidence', () => {
		const result = resolveWorkflowTarget({ role: 'button', name: 'Search' }, elements)

		expect(result.matched).toBe(true)
		expect(result.element?.id).toBe('b')
		expect(result.confidence).toBeGreaterThanOrEqual(0.9)
	})

	it('rejects hidden or disabled elements', () => {
		const result = resolveWorkflowTarget({ role: 'button', name: 'Hidden' }, elements)

		expect(result.matched).toBe(false)
		expect(result.reason).toContain('actionable')
	})

	it('rejects low-confidence text-only duplicate matches', () => {
		const target: WorkflowTarget = { text: 'Search' }
		const result = resolveWorkflowTarget(target, [
			...elements,
			{ id: 'd', role: 'link', text: 'Search', visible: true, enabled: true },
		])

		expect(result.matched).toBe(false)
		expect(result.reason).toContain('multiple')
	})
})
