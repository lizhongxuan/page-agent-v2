import { afterEach, describe, expect, it, vi } from 'vitest'

import { RemotePageController } from './RemotePageController'
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

describe('RemotePageController navigation and keyboard actions', () => {
	afterEach(() => {
		vi.unstubAllGlobals()
	})

	it('forwards keyboard, navigation, reload, and condition waits to the content script', async () => {
		const sendMessage = vi.fn().mockResolvedValue({ success: true, message: 'ok' })
		vi.stubGlobal('chrome', {
			runtime: {
				sendMessage,
			},
		})

		const tabsController = {
			currentTabId: 101,
			getTabInfo: vi.fn().mockResolvedValue({ url: 'https://example.com', title: 'Example' }),
		}
		const controller = new RemotePageController(tabsController as any)

		await controller.pressKey({ key: 'Enter' })
		await controller.goBack()
		await controller.reloadPage()
		await controller.waitForCondition({ type: 'text_present', text: 'Ready' })

		expect(sendMessage).toHaveBeenNthCalledWith(1, {
			type: 'PAGE_CONTROL',
			action: 'press_key',
			targetTabId: 101,
			payload: [{ key: 'Enter' }],
		})
		expect(sendMessage).toHaveBeenNthCalledWith(2, {
			type: 'PAGE_CONTROL',
			action: 'go_back',
			targetTabId: 101,
			payload: [],
		})
		expect(sendMessage).toHaveBeenNthCalledWith(3, {
			type: 'PAGE_CONTROL',
			action: 'reload_page',
			targetTabId: 101,
			payload: [],
		})
		expect(sendMessage).toHaveBeenNthCalledWith(4, {
			type: 'PAGE_CONTROL',
			action: 'wait_for_condition',
			targetTabId: 101,
			payload: [{ type: 'text_present', text: 'Ready' }],
		})
	})
})
