// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'

import { SiteManualImportForm } from './SiteManualImportForm'

;(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true

describe('SiteManualImportForm', () => {
	it('requires a site before importing', async () => {
		document.body.innerHTML = '<div id="root"></div>'
		const onImport = vi.fn()

		await act(async () => {
			render(<SiteManualImportForm projectId="default" onImport={onImport} />)
		})
		await act(async () => {
			setInputValue(
				document.querySelector<HTMLInputElement>('input[name="title"]')!,
				'Backup manual'
			)
			setTextareaValue(
				document.querySelector<HTMLTextAreaElement>('textarea[name="content"]')!,
				'# Restore'
			)
		})
		await act(async () => {
			document.querySelector<HTMLButtonElement>('button[type="submit"]')!.click()
		})

		expect(onImport).not.toHaveBeenCalled()
		expect(document.body.textContent).toContain('Site is required')
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

function setTextareaValue(textarea: HTMLTextAreaElement, value: string) {
	const descriptor = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value')
	if (!descriptor?.set) throw new Error('Missing textarea value setter')
	descriptor.set.call(textarea, value)
	textarea.dispatchEvent(new Event('input', { bubbles: true }))
}
