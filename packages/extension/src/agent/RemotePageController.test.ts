import { describe, expect, it } from 'vitest'

import { normalizeBrowserStateResponse } from './browserState'

describe('normalizeBrowserStateResponse', () => {
	it('returns a diagnostic browser state instead of undefined content', () => {
		const browserState = normalizeBrowserStateResponse(
			{ success: false, error: 'content script returned undefined' },
			'https://example.com/service',
			'Service'
		)

		expect(browserState.url).toBe('https://example.com/service')
		expect(browserState.title).toBe('Service')
		expect(browserState.content).toContain('page state unavailable')
		expect(browserState.content).toContain('content script returned undefined')
		expect(browserState.header).toBe('')
		expect(browserState.footer).toBe('')
	})
})
