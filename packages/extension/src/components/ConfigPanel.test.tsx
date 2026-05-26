// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'

import { DEMO_BASE_URL, DEMO_MODEL } from '@/agent/constants'

import { ConfigPanel } from './ConfigPanel'

;(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true
;(globalThis as any).__VERSION__ = '1.8.2'
;(globalThis as any).chrome = {
	storage: {
		local: {
			get: vi.fn().mockResolvedValue({ PageAgentExtUserAuthToken: '4521000000000794' }),
		},
	},
}

describe('ConfigPanel', () => {
	it('does not render the testing API notice or footer action links', async () => {
		document.body.innerHTML = '<div id="root"></div>'

		await act(async () => {
			render(
				<ConfigPanel
					config={{
						baseURL: DEMO_BASE_URL,
						model: DEMO_MODEL,
						language: 'zh-CN',
					}}
					onSave={async () => {}}
					onClose={() => {}}
				/>
			)
		})

		expect(document.body.textContent).not.toContain('You are using our testing API')
		expect(document.body.textContent).not.toContain('Terms of Use & Privacy Policy')
		expect(document.body.textContent).not.toContain('Version')
		expect(document.body.textContent).not.toContain('Source Code')
		expect(document.body.textContent).not.toContain('Home Page')
		expect(document.body.textContent).not.toContain('Privacy')
		expect(document.body.textContent).not.toContain('Built with')
		expect(document.body.textContent).not.toContain('@Simon')
		expect(
			document.querySelector('a[href="https://github.com/alibaba/page-agent"]')
		).not.toBeTruthy()
		expect(
			document.querySelector('a[href="https://alibaba.github.io/page-agent/"]')
		).not.toBeTruthy()
	})
})

function render(node: React.ReactNode) {
	const container = document.getElementById('root')
	if (!container) throw new Error('Missing test root')
	const root = createRoot(container)
	root.render(node)
	return root
}
