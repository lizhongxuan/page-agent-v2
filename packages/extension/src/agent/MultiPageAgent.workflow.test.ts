import { describe, expect, it, vi } from 'vitest'

import { MultiPageAgent } from './MultiPageAgent'

const mocks = vi.hoisted(() => {
	const tabsControllerInstances: {
		currentTabId: number | null
		init: ReturnType<typeof vi.fn>
		getTabInfo: ReturnType<typeof vi.fn>
	}[] = []
	const remotePageControllerInstances: {
		getWorkflowElements: ReturnType<typeof vi.fn>
	}[] = []

	return { tabsControllerInstances, remotePageControllerInstances }
})

vi.mock('@page-agent/core', () => ({
	PageAgentCore: class {
		addEventListener = vi.fn()
		removeEventListener = vi.fn()
		dispose = vi.fn()
	},
}))

vi.mock('./TabsController', () => ({
	TabsController: class {
		currentTabId: number | null = null
		init = vi.fn(async () => {
			this.currentTabId = 123
		})
		getTabInfo = vi.fn(async () => ({
			title: 'GitHub fixture',
			url: 'http://127.0.0.1:60231/workflow-github-issues.html',
		}))
		constructor() {
			mocks.tabsControllerInstances.push(this)
		}
	},
}))

vi.mock('./RemotePageController', () => ({
	RemotePageController: class {
		getWorkflowElements = vi.fn(async () => ({
			visibleText: ['Code', 'Issues', 'Search all issues'],
			controls: [{ role: 'link', name: 'Issues' }],
		}))
		constructor() {
			mocks.remotePageControllerInstances.push(this)
		}
	},
}))

vi.mock('./tabTools', () => ({
	createTabTools: () => [],
}))

describe('MultiPageAgent workflow observation', () => {
	it('initializes the current web tab before workflow replay reads the page observation', async () => {
		const agent = new MultiPageAgent({} as never)

		const observation = await agent.getCurrentPageObservation(
			'在 github.com/microsoft/playwright 的 Issues 里搜索 timeout 报错'
		)

		const tabsController = mocks.tabsControllerInstances[0]
		expect(tabsController?.init).toHaveBeenCalledWith(
			'在 github.com/microsoft/playwright 的 Issues 里搜索 timeout 报错',
			{
				includeInitialTab: true,
				experimentalIncludeAllTabs: false,
			}
		)
		expect(tabsController?.getTabInfo).toHaveBeenCalledWith(123)
		expect(observation).toEqual({
			url: 'http://127.0.0.1:60231/workflow-github-issues.html',
			title: 'GitHub fixture',
			visibleText: ['Code', 'Issues', 'Search all issues'],
			controls: [{ role: 'link', name: 'Issues' }],
		})
	})
})
