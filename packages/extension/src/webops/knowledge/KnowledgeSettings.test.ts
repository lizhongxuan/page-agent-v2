import { describe, expect, it } from 'vitest'

import { defaultKnowledgeSettings } from './KnowledgeSettings'

describe('defaultKnowledgeSettings', () => {
	it('keeps external project knowledge disabled by default', () => {
		expect(defaultKnowledgeSettings.enabled).toBe(false)
		expect(defaultKnowledgeSettings.allowPageSummary).toBe(true)
		expect(defaultKnowledgeSettings.baseUrl).toBe('')
		expect(defaultKnowledgeSettings.apiKey).toBe('')
		expect(defaultKnowledgeSettings.projectKey).toBe('')
	})
})
