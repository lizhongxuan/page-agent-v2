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

	it('renders workflow memory without old knowledge settings', async () => {
		document.body.innerHTML = '<div id="root"></div>'

		await act(async () => {
			render(
				<ConfigPanel
					config={{
						baseURL: DEMO_BASE_URL,
						model: DEMO_MODEL,
						workflowBackend: {
							baseUrl: 'http://127.0.0.1:38403',
							projectId: 'demo-manuals',
						},
					}}
					onSave={async () => {}}
					onClose={() => {}}
				/>
			)
		})

		expect(document.body.textContent).toContain('Workflow Memory Backend')
		expect(document.body.textContent).toContain('Site Manual Library')
		expect(document.body.textContent).not.toContain('项目知识库')
		const placeholders = Array.from(document.querySelectorAll<HTMLInputElement>('input')).map(
			(input) => input.placeholder
		)
		expect(placeholders).toEqual(
			expect.arrayContaining(['http://127.0.0.1:38402', 'Project ID', 'Workflow Backend API Key'])
		)
		expect(placeholders).not.toContain('Project Key')
		expect(
			document.querySelector<HTMLInputElement>('input[placeholder="http://127.0.0.1:38402"]')!.value
		).toBe('http://127.0.0.1:38403')
		expect(document.querySelector<HTMLInputElement>('input[placeholder="Project ID"]')!.value).toBe(
			'demo-manuals'
		)
	})

	it('saves only workflow backend fields for memory', async () => {
		document.body.innerHTML = '<div id="root"></div>'
		const onSave = vi.fn().mockResolvedValue(undefined)

		await act(async () => {
			render(
				<ConfigPanel
					config={{
						baseURL: DEMO_BASE_URL,
						model: DEMO_MODEL,
					}}
					onSave={onSave}
					onClose={() => {}}
				/>
			)
		})

		await act(async () => {
			setInputValue(
				document.querySelector<HTMLInputElement>('input[placeholder="http://127.0.0.1:38402"]')!,
				'http://127.0.0.1:38403'
			)
			setInputValue(
				document.querySelector<HTMLInputElement>('input[placeholder="Project ID"]')!,
				'demo-manuals'
			)
		})
		await act(async () => {
			Array.from(document.querySelectorAll('button'))
				.find((button) => button.textContent === 'Save')!
				.click()
		})

		expect(onSave).toHaveBeenCalledWith(
			expect.objectContaining({
				workflowBackend: {
					baseUrl: 'http://127.0.0.1:38403',
					projectId: 'demo-manuals',
					apiKey: undefined,
				},
			})
		)
		expect(onSave.mock.calls[0]?.[0]).not.toHaveProperty('knowledgeSettings')
	})
})

function render(node: React.ReactNode) {
	const container = document.getElementById('root')
	if (!container) throw new Error('Missing test root')
	const root = createRoot(container)
	root.render(node)
	return root
}

function setInputValue(input: HTMLInputElement, value: string) {
	const descriptor = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value')
	if (!descriptor?.set) throw new Error('Missing input value setter')
	const setValue = descriptor.set.bind(input)
	setValue(value)
	input.dispatchEvent(new Event('input', { bubbles: true }))
}
