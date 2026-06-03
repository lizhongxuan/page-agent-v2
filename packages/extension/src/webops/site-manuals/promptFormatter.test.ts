import { describe, expect, it } from 'vitest'

import { formatSiteManualKnowledgeForPrompt } from './promptFormatter'

describe('formatSiteManualKnowledgeForPrompt', () => {
	it('formats manual summaries and source refs for LLM context', () => {
		const prompt = formatSiteManualKnowledgeForPrompt([
			{
				id: 'manual_restore',
				confidence: 0.82,
				summary: 'Full backup records expose restore actions.',
				sourceRefs: ['Backup manual / Full restore'],
			},
		])

		expect(prompt).toContain('<site_manual_knowledge>')
		expect(prompt).toContain('manual_restore')
		expect(prompt).toContain('Full backup records expose restore actions.')
		expect(prompt).toContain('Backup manual / Full restore')
	})
})
