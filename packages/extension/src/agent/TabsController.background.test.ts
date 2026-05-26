import { afterEach, describe, expect, it, vi } from 'vitest'

import { handleTabControlMessage } from './TabsController.background'

describe('handleTabControlMessage', () => {
	afterEach(() => {
		vi.unstubAllGlobals()
	})

	it('opens new tabs as active so users can see what the agent is controlling', async () => {
		const create = vi.fn().mockResolvedValue({ id: 123 })
		vi.stubGlobal('chrome', {
			tabs: {
				create,
			},
		})

		const response = await new Promise<unknown>((resolve) => {
			handleTabControlMessage(
				{
					type: 'TAB_CONTROL',
					action: 'open_new_tab',
					payload: { url: 'https://example.com' },
				},
				{} as chrome.runtime.MessageSender,
				resolve
			)
		})

		expect(create).toHaveBeenCalledWith({ url: 'https://example.com', active: true })
		expect(response).toEqual({ success: true, tabId: 123 })
	})
})
