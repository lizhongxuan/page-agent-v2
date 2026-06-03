import { describe, expect, it } from 'vitest'

import { formatMemoryForPrompt } from './promptFormatter'

describe('formatMemoryForPrompt', () => {
	it('returns contextPrompt unchanged when present', () => {
		const prompt = '<webops_memory>\nUse service search.\n</webops_memory>'

		expect(formatMemoryForPrompt({ contextPrompt: prompt })).toBe(prompt)
	})

	it('returns an empty string when contextPrompt is missing', () => {
		expect(formatMemoryForPrompt({ evidence: [] })).toBe('')
	})

	it('formats site task guides and site manual knowledge without old memory tags', () => {
		const prompt = formatMemoryForPrompt({
			siteTaskGuides: [
				{
					id: 'guide_restore',
					confidence: 0.86,
					summary: 'Restore from latest backup.',
					matchedStateId: 'state_full_backup_tab',
					matchedStateName: 'Full Backup tab',
					startStepOffset: 3,
					matchReasons: ['active_tab', 'controls_all'],
					pageGuards: ['Backup page must be visible.'],
					steps: ['Open backup tab.', 'Click Restore.'],
					abandonRules: ['Abandon if the backup tab is missing.'],
				},
			],
			siteManualKnowledge: [
				{
					id: 'manual_restore',
					confidence: 0.82,
					summary: 'Full backups expose the restore action.',
					sourceRefs: ['Backup manual / Full restore'],
				},
			],
		})

		expect(prompt).toContain('<site_task_guides>')
		expect(prompt).toContain('<site_manual_knowledge>')
		expect(prompt).toContain('<matched_state>Full Backup tab</matched_state>')
		expect(prompt).toContain('<remaining_steps>')
		expect(prompt).toContain('4. Open backup tab.')
		expect(prompt).toContain('Abandon if the backup tab is missing.')
		expect(prompt).toContain('Backup manual / Full restore')
	})
})
