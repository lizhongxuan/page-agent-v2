// @vitest-environment jsdom
import { act } from 'react'
import { afterEach, describe, expect, it } from 'vitest'

import { mountWebOpsOverlay, unmountWebOpsOverlay, webOpsInteractionBus } from './overlayRoot'

;(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true

describe('webops overlay root', () => {
	afterEach(() => {
		unmountWebOpsOverlay()
		webOpsInteractionBus.clear()
	})

	it('mounts and unmounts the overlay host', () => {
		act(() => mountWebOpsOverlay())
		expect(document.getElementById('page-agent-v2-webops-overlay')).toBeTruthy()

		act(() => unmountWebOpsOverlay())
		expect(document.getElementById('page-agent-v2-webops-overlay')).toBeNull()
	})

	it('renders published toast events', async () => {
		await act(async () => {
			mountWebOpsOverlay()
			await new Promise((resolve) => window.setTimeout(resolve, 0))
		})

		await act(async () => {
			webOpsInteractionBus.publish({ type: 'toast', message: '正在读取页面', level: 'info' })
			await new Promise((resolve) => window.setTimeout(resolve, 0))
		})

		await waitForText('正在读取页面')
		expect(document.body.textContent).toContain('正在读取页面')
	})

	it('clears pending interaction UI after the user responds', async () => {
		await act(async () => {
			mountWebOpsOverlay()
			await new Promise((resolve) => window.setTimeout(resolve, 0))
		})

		await act(async () => {
			webOpsInteractionBus.publish({
				type: 'choice',
				requestId: 'target-1',
				elementIndex: 0,
				title: '确认目标',
				message: 'Agent 认为这个控件可能是目标，请确认。',
				options: [{ id: 'candidate-0', label: '搜索按钮' }],
			})
			await new Promise((resolve) => window.setTimeout(resolve, 0))
		})
		await waitForText('确认目标')

		await act(async () => {
			document.querySelector<HTMLButtonElement>('[data-webops-choice-confirm="true"]')?.click()
			await new Promise((resolve) => window.setTimeout(resolve, 0))
		})

		expect(document.body.textContent).not.toContain('确认目标')
		expect(document.querySelector('[data-webops-anchor-bubble="true"]')).toBeNull()
	})
})

async function waitForText(text: string) {
	for (let attempt = 0; attempt < 10; attempt++) {
		if (document.body.textContent?.includes(text)) return
		await new Promise((resolve) => window.setTimeout(resolve, 0))
	}
}
