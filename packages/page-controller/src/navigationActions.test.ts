// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { PageController } from './PageController'

describe('PageController navigation and keyboard actions', () => {
	beforeEach(() => {
		document.body.innerHTML = ''
		vi.restoreAllMocks()
	})

	it('presses a keyboard key on the focused element', async () => {
		const input = document.createElement('input')
		const events: string[] = []
		document.body.append(input)
		input.focus()

		for (const eventName of ['keydown', 'keyup']) {
			input.addEventListener(eventName, (event) => {
				events.push(`${event.type}:${(event as KeyboardEvent).key}`)
			})
		}

		const result = await new PageController().pressKey({ key: 'Enter' })

		expect(result.success).toBe(true)
		expect(events).toEqual(['keydown:Enter', 'keyup:Enter'])
	})

	it('goes back in browser history', async () => {
		const back = vi.spyOn(window.history, 'back').mockImplementation(() => {})

		const result = await new PageController().goBack()

		expect(result.success).toBe(true)
		expect(back).toHaveBeenCalledOnce()
	})

	it('reloads the current page', async () => {
		const result = await new PageController().reloadPage()

		expect(result.success).toBe(true)
		expect(result.message).toContain('Reloaded')
	})

	it('waits until text appears on the page', async () => {
		setTimeout(() => {
			document.body.textContent = 'Service is healthy'
		}, 10)

		const result = await new PageController().waitForCondition({
			type: 'text_present',
			text: 'healthy',
			timeoutMs: 500,
			pollIntervalMs: 10,
		})

		expect(result.success).toBe(true)
		expect(result.message).toContain('text_present')
	})

	it('returns a timeout message when a condition is not met', async () => {
		const result = await new PageController().waitForCondition({
			type: 'text_present',
			text: 'never appears',
			timeoutMs: 20,
			pollIntervalMs: 10,
		})

		expect(result.success).toBe(false)
		expect(result.message).toContain('Timed out')
	})
})
