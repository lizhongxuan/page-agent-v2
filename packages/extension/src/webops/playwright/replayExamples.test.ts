import { describe, expect, it } from 'vitest'

import { exportPlaywrightProject } from './PlaywrightExporter'
import { createReplayExampleSessions } from './replayExamples'

describe('createReplayExampleSessions', () => {
	it('provides three named replay examples', () => {
		const examples = createReplayExampleSessions('https://example.test')

		expect(examples.map((example) => example.id)).toEqual([
			'service-search-detail',
			'settings-filter-export',
			'manual-handover-review',
		])
		expect(examples.every((example) => example.session.steps.length > 0)).toBe(true)
	})

	it('exports replay code covering common operation types', () => {
		const examples = createReplayExampleSessions('https://example.test')
		const combinedReplayCode = examples
			.map((example) => exportPlaywrightProject(example.session)['tests/replay.spec.ts'])
			.join('\n')

		expect(combinedReplayCode).toContain("page.getByPlaceholder('Service name').fill('checkout')")
		expect(combinedReplayCode).toContain("page.getByRole('button', { name: 'Search' }).click()")
		expect(combinedReplayCode).toContain(
			"page.getByRole('combobox', { name: 'Environment' }).selectOption('prod')"
		)
		expect(combinedReplayCode).toContain(
			"await page.goto('https://example.test/services/checkout')"
		)
		expect(combinedReplayCode).toContain("await expect(page.getByText('Healthy')).toBeVisible()")
		expect(combinedReplayCode).toContain('await page.waitForLoadState')
		expect(combinedReplayCode).toContain('// Observed page state: Confirmed service detail page')
		expect(combinedReplayCode).toContain('// Manual handover: Complete MFA approval in the browser')
	})
})
