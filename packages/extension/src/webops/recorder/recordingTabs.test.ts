import { describe, expect, it } from 'vitest'

import { selectRecordableTab } from './recordingTabs'

describe('selectRecordableTab', () => {
	it('uses the active web tab when the current active tab is recordable', () => {
		const tab = selectRecordableTab(
			[{ id: 10, url: 'http://127.0.0.1:49152/workflow-github-issues.html', active: true }],
			[],
			'chrome-extension://abc'
		)

		expect(tab?.id).toBe(10)
	})

	it('falls back to the last remembered web tab when the side panel is opened as an extension tab', () => {
		const tab = selectRecordableTab(
			[{ id: 20, url: 'chrome-extension://abc/sidepanel.html', active: true }],
			[
				{ id: 20, url: 'chrome-extension://abc/sidepanel.html', active: true },
				{ id: 10, url: 'http://127.0.0.1:49152/workflow-github-issues.html', active: false },
			],
			'chrome-extension://abc',
			10
		)

		expect(tab?.id).toBe(10)
	})
})
