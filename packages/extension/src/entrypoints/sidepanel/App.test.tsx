// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import App from './App'

;(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true

const listSiteManuals = vi.fn()

vi.mock('@/agent/useAgent', () => ({
	useAgent: () => ({
		status: 'idle',
		history: [],
		activity: null,
		currentTask: null,
		config: {
			workflowBackend: {
				baseUrl: 'http://127.0.0.1:38402',
				projectId: 'default',
			},
		},
		webOpsSession: undefined,
		pendingQuestion: null,
		execute: vi.fn(),
		answerQuestion: vi.fn(),
		newSession: vi.fn(),
		stop: vi.fn(),
		configure: vi.fn(),
	}),
}))

vi.mock('@/lib/db', () => ({
	saveSession: vi.fn(),
}))

vi.mock('@/components/misc', () => ({
	EmptyState: () => <div>Empty</div>,
	MotionOverlay: () => null,
	StatusDot: ({ status }: { status: string }) => <span>{status}</span>,
}))

vi.mock('@/webops/site-manuals/SiteManualClient', () => ({
	SiteManualClient: vi.fn().mockImplementation(function SiteManualClient() {
		return {
			listSiteManuals,
		}
	}),
}))

describe('SidePanel App site manuals status', () => {
	beforeEach(() => {
		document.body.innerHTML = '<div id="root"></div>'
		listSiteManuals.mockReset()
		listSiteManuals.mockResolvedValue({
			sources: [
				{
					id: 'manual_1',
					site: 'ops.example.test',
					title: 'Ops manual',
					status: 'active',
				},
			],
		})
		;(globalThis as any).chrome = {
			runtime: {
				getURL: vi.fn((path: string) => `chrome-extension://extension-id${path}`),
			},
			tabs: {
				create: vi.fn().mockResolvedValue({}),
				query: vi.fn().mockResolvedValue([
					{
						url: 'https://ops.example.test/dashboard?tab=manuals',
					},
				]),
			},
		}
	})

	it('shows current site status and opens the library with current tab defaults', async () => {
		await act(async () => {
			render(<App />)
		})
		await act(async () => {})

		expect(listSiteManuals).toHaveBeenCalledWith({
			projectId: 'default',
			site: 'ops.example.test',
			status: 'active',
		})
		expect(document.body.textContent).toContain('Current Site')
		expect(document.body.textContent).toContain('ops.example.test')
		expect(document.body.textContent).toContain('1 active manual')

		await act(async () => {
			Array.from(document.querySelectorAll('button'))
				.find((button) => button.textContent?.includes('Open Library'))
				?.click()
		})

		expect(chrome.tabs.create).toHaveBeenCalledWith({
			url: 'chrome-extension://extension-id/hub.html?view=site-manuals&site=ops.example.test&url=https%3A%2F%2Fops.example.test%2Fdashboard%3Ftab%3Dmanuals',
		})
	})
})

function render(node: React.ReactNode) {
	const container = document.getElementById('root')
	if (!container) throw new Error('Missing test root')
	const root = createRoot(container)
	root.render(node)
	return root
}
