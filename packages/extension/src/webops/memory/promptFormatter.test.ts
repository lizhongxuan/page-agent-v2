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
})
