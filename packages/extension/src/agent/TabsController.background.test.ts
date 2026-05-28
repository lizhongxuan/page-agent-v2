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

	it('returns the remembered web tab when the active tab is the extension page', async () => {
		const extensionTab = {
			id: 20,
			active: true,
			url: 'chrome-extension://abc/sidepanel.html',
		}
		const targetTab = {
			id: 10,
			active: false,
			url: 'http://127.0.0.1:49152/workflow-github-issues.html',
		}
		const query = vi
			.fn()
			.mockResolvedValueOnce([extensionTab])
			.mockResolvedValueOnce([extensionTab])
			.mockResolvedValueOnce([extensionTab, targetTab])
		vi.stubGlobal('chrome', {
			runtime: {
				getURL: () => 'chrome-extension://abc/',
			},
			storage: {
				local: {
					get: vi.fn().mockResolvedValue({ workflowTargetTabId: 10 }),
				},
			},
			tabs: {
				query,
			},
		})

		const response = await new Promise<unknown>((resolve) => {
			handleTabControlMessage(
				{
					type: 'TAB_CONTROL',
					action: 'get_active_tab',
					payload: undefined,
				},
				{} as chrome.runtime.MessageSender,
				resolve
			)
		})

		expect(response).toEqual({ success: true, tab: targetTab })
	})

	it('prefers the current-window active web tab over a remembered workflow target tab', async () => {
		const currentWindowTab = {
			id: 30,
			active: true,
			windowId: 3,
			url: 'https://gemini.google.com/app/chat',
		}
		const rememberedTab = {
			id: 10,
			active: true,
			windowId: 1,
			url: 'https://github.com/browser-use/workflow-use',
		}
		const query = vi.fn(async (queryInfo: chrome.tabs.QueryInfo) => {
			if (queryInfo.active && queryInfo.currentWindow) return [currentWindowTab]
			if (queryInfo.active) return [rememberedTab, currentWindowTab]
			return [rememberedTab, currentWindowTab]
		})
		vi.stubGlobal('chrome', {
			runtime: {
				getURL: () => 'chrome-extension://abc/',
			},
			storage: {
				local: {
					get: vi.fn().mockResolvedValue({ workflowTargetTabId: 10 }),
				},
			},
			tabs: {
				query,
			},
		})

		const response = await new Promise<unknown>((resolve) => {
			handleTabControlMessage(
				{
					type: 'TAB_CONTROL',
					action: 'get_active_tab',
					payload: undefined,
				},
				{} as chrome.runtime.MessageSender,
				resolve
			)
		})

		expect(query).toHaveBeenCalledWith({ active: true, currentWindow: true })
		expect(response).toEqual({ success: true, tab: currentWindowTab })
	})
})
