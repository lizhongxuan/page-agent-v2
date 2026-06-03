// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { SiteManualClientLike } from '@/webops/site-manuals/types'

import { SiteManualPreview } from './SiteManualPreview'

;(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true
;(globalThis as any).navigator = {
	clipboard: {
		writeText: vi.fn().mockResolvedValue(undefined),
	},
}

describe('SiteManualPreview', () => {
	beforeEach(() => {
		;(globalThis as any).chrome = undefined
	})

	it('renders preview context returned by the backend client', async () => {
		document.body.innerHTML = '<div id="root"></div>'
		const previewContext = vi.fn().mockResolvedValue({
			matchedChunks: [
				{
					id: 'chunk_b',
					wikiPageId: 'page_2',
					site: 'ops.example.test',
					text: 'Second from backend',
				},
				{
					id: 'chunk_a',
					wikiPageId: 'page_1',
					site: 'ops.example.test',
					text: 'First from backend',
				},
			],
			filtered: [{ id: 'chunk_c', reason: 'status disabled' }],
			prompt: '<site_manual_knowledge>backend order</site_manual_knowledge>',
		})
		const client: SiteManualClientLike = {
			previewContext,
		}

		await act(async () => {
			render(
				<SiteManualPreview client={client} projectId="default" defaultSite="ops.example.test" />
			)
		})
		await act(async () => {
			setInputValue(
				document.querySelector<HTMLInputElement>('input[name="task"]')!,
				'restore backup'
			)
			setInputValue(
				document.querySelector<HTMLInputElement>('input[name="url"]')!,
				'https://ops.example.test/backup'
			)
		})
		await act(async () => {
			document.querySelector<HTMLButtonElement>('button[type="submit"]')!.click()
		})

		expect(previewContext).toHaveBeenCalledWith({
			projectId: 'default',
			site: 'ops.example.test',
			module: undefined,
			task: 'restore backup',
			url: 'https://ops.example.test/backup',
		})
		expect(document.body.textContent).toContain('Second from backend')
		expect(document.body.textContent).toContain('First from backend')
		expect(document.body.textContent).toContain('status disabled')
		expect(document.body.textContent).toContain(
			'<site_manual_knowledge>backend order</site_manual_knowledge>'
		)
	})

	it('uses current tab URL and host as preview defaults', async () => {
		document.body.innerHTML = '<div id="root"></div>'
		;(globalThis as any).chrome = {
			tabs: {
				query: vi.fn().mockResolvedValue([
					{
						url: 'https://console.example.test/users?filter=active',
					},
				]),
			},
		}
		const previewContext = vi.fn().mockResolvedValue({
			matchedChunks: [],
			prompt: '<site_manual_knowledge />',
		})
		const client: SiteManualClientLike = {
			previewContext,
		}

		await act(async () => {
			render(<SiteManualPreview client={client} projectId="default" />)
		})
		await act(async () => {})

		expect(document.querySelector<HTMLInputElement>('input[name="site"]')!.value).toBe(
			'console.example.test'
		)
		expect(document.querySelector<HTMLInputElement>('input[name="url"]')!.value).toBe(
			'https://console.example.test/users?filter=active'
		)

		await act(async () => {
			setInputValue(document.querySelector<HTMLInputElement>('input[name="task"]')!, 'find users')
		})
		await act(async () => {
			document.querySelector<HTMLButtonElement>('button[type="submit"]')!.click()
		})

		expect(previewContext).toHaveBeenCalledWith({
			projectId: 'default',
			site: 'console.example.test',
			module: undefined,
			task: 'find users',
			url: 'https://console.example.test/users?filter=active',
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

function setInputValue(input: HTMLInputElement, value: string) {
	const descriptor = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value')
	if (!descriptor?.set) throw new Error('Missing input value setter')
	descriptor.set.call(input, value)
	input.dispatchEvent(new Event('input', { bubbles: true }))
}
