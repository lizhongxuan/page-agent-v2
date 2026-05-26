// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'

import { WEBOPS_INTERACTION_RESPONSE_EVENT } from '../interactionTypes'
import { ActionSpotlight } from './ActionSpotlight'
import { AnchorBubble } from './AnchorBubble'
import { HandoverBanner } from './HandoverBanner'
import { InputPrompt } from './InputPrompt'
import { ProgressToast } from './ProgressToast'

;(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true

describe('interaction components', () => {
	it('renders ActionSpotlight around a page-agent indexed element', () => {
		document.body.innerHTML = '<button data-page-agent-index="1">搜索</button><div id="root"></div>'
		const button = document.querySelector('button') as HTMLElement
		vi.spyOn(button, 'getBoundingClientRect').mockReturnValue(
			rect({ left: 20, top: 30, width: 80, height: 32 })
		)

		render(
			<ActionSpotlight
				event={{ type: 'spotlight', elementIndex: 1, action: 'click', message: '准备点击搜索' }}
			/>
		)

		expect(document.body.textContent).toContain('准备点击搜索')
		expect(document.querySelector('div[style*="pointer-events: none"]')).toBeTruthy()
	})

	it('renders AnchorBubble near an indexed target with target confirmation actions', () => {
		document.body.innerHTML = '<button data-page-agent-index="0">确认</button><div id="root"></div>'
		const button = document.querySelector('button') as HTMLElement
		vi.spyOn(button, 'getBoundingClientRect').mockReturnValue(
			rect({ left: 40, top: 50, width: 80, height: 32 })
		)

		render(
			<AnchorBubble
				event={{
					type: 'choice',
					elementIndex: 0,
					title: '确认目标',
					message: '要点击这个按钮吗？',
					options: [{ id: 'candidate-0', label: '搜索按钮' }],
				}}
			/>
		)

		expect(document.body.textContent).toContain('确认目标')
		expect(document.body.textContent).toContain('确定')
		expect(document.body.textContent).toContain('错误')
		expect(document.body.textContent).toContain('手动选择')
		expect(document.querySelector('[data-webops-choice-confirm="true"]')).toBeTruthy()
		expect(document.querySelector('[data-webops-choice-reject="true"]')).toBeTruthy()
		expect(document.querySelector('[data-webops-choice-manual="true"]')).toBeTruthy()
		expect(
			document.querySelector<HTMLElement>('[data-webops-anchor-bubble="true"]')?.style.pointerEvents
		).toBe('auto')
	})

	it('emits a choice response when the user clicks a choice button', () => {
		document.body.innerHTML = '<button data-page-agent-index="0">确认</button><div id="root"></div>'
		const responses: unknown[] = []
		window.addEventListener(WEBOPS_INTERACTION_RESPONSE_EVENT, (event) => {
			responses.push((event as CustomEvent).detail)
		})

		render(
			<AnchorBubble
				event={{
					type: 'choice',
					requestId: 'r1',
					elementIndex: 0,
					title: '确认目标',
					message: '要点击这个按钮吗？',
					options: [{ id: 'candidate-0', label: '搜索按钮' }],
				}}
			/>
		)
		act(() =>
			document.querySelector<HTMLButtonElement>('[data-webops-choice-confirm="true"]')?.click()
		)

		expect(responses).toEqual([
			{ requestId: 'r1', response: { type: 'choice', optionId: 'candidate-0' } },
		])
	})

	it('emits a rejected response when the user says the target is wrong', () => {
		document.body.innerHTML = '<button data-page-agent-index="0">确认</button><div id="root"></div>'
		const responses: unknown[] = []
		window.addEventListener(WEBOPS_INTERACTION_RESPONSE_EVENT, (event) => {
			responses.push((event as CustomEvent).detail)
		})

		render(
			<AnchorBubble
				event={{
					type: 'choice',
					requestId: 'r1',
					elementIndex: 0,
					title: '确认目标',
					message: '要点击这个按钮吗？',
					options: [{ id: 'candidate-0', label: '搜索按钮' }],
				}}
			/>
		)
		act(() =>
			document.querySelector<HTMLButtonElement>('[data-webops-choice-reject="true"]')?.click()
		)

		expect(responses).toEqual([
			{ requestId: 'r1', response: { type: 'rejected', reason: 'wrong_target' } },
		])
	})

	it('captures a manual target point without clicking the underlying page control', () => {
		document.body.innerHTML =
			'<button id="real-button" data-page-agent-index="0">确认</button><div id="root"></div>'
		let clicked = false
		document.getElementById('real-button')?.addEventListener('click', () => {
			clicked = true
		})
		const responses: unknown[] = []
		window.addEventListener(WEBOPS_INTERACTION_RESPONSE_EVENT, (event) => {
			responses.push((event as CustomEvent).detail)
		})

		render(
			<AnchorBubble
				event={{
					type: 'choice',
					requestId: 'r1',
					elementIndex: 0,
					title: '确认目标',
					message: '要点击这个按钮吗？',
					options: [{ id: 'candidate-0', label: '搜索按钮' }],
				}}
			/>
		)
		act(() =>
			document.querySelector<HTMLButtonElement>('[data-webops-choice-manual="true"]')?.click()
		)
		act(() => {
			document
				.querySelector<HTMLElement>('[data-webops-manual-select-overlay="true"]')
				?.dispatchEvent(
					new MouseEvent('click', {
						bubbles: true,
						cancelable: true,
						clientX: 123,
						clientY: 45,
					})
				)
		})

		expect(clicked).toBe(false)
		expect(responses).toEqual([
			{ requestId: 'r1', response: { type: 'manual_select', x: 123, y: 45 } },
		])
	})

	it('renders HandoverBanner and ProgressToast messages', () => {
		document.body.innerHTML = '<div id="root"></div>'

		render(
			<>
				<HandoverBanner
					event={{
						type: 'handover',
						title: '需要手动验证',
						message: '请完成 MFA',
						resumeButtonLabel: '我已完成，继续',
					}}
				/>
				<ProgressToast event={{ type: 'toast', message: '正在搜索 kme', level: 'info' }} />
			</>
		)

		expect(document.body.textContent).toContain('需要手动验证')
		expect(document.body.textContent).toContain('我已完成，继续')
		expect(document.body.textContent).toContain('正在搜索 kme')
		expect(document.querySelector('[data-webops-handover-done="true"]')).toBeTruthy()
	})

	it('emits handover and input responses', () => {
		document.body.innerHTML = '<div id="root"></div>'
		const responses: unknown[] = []
		window.addEventListener(WEBOPS_INTERACTION_RESPONSE_EVENT, (event) => {
			responses.push((event as CustomEvent).detail)
		})

		render(
			<>
				<HandoverBanner
					event={{
						type: 'handover',
						requestId: 'handover-1',
						title: '需要手动验证',
						message: '请完成 MFA',
						resumeButtonLabel: '我已完成，继续',
					}}
				/>
				<InputPrompt
					event={{
						type: 'input',
						requestId: 'input-1',
						title: '需要补充信息',
						message: '请输入环境',
						submitButtonLabel: '继续',
					}}
				/>
			</>
		)
		act(() =>
			document.querySelector<HTMLButtonElement>('[data-webops-handover-done="true"]')?.click()
		)
		act(() => {
			const textarea = document.querySelector('textarea')!
			textarea.value = '生产环境'
			textarea.dispatchEvent(new Event('input', { bubbles: true }))
		})
		act(() =>
			document.querySelector<HTMLButtonElement>('[data-webops-input-submit="true"]')?.click()
		)

		expect(responses).toContainEqual({
			requestId: 'handover-1',
			response: { type: 'handover_done' },
		})
		expect(responses).toContainEqual({
			requestId: 'input-1',
			response: { type: 'input', value: '生产环境' },
		})
	})
})

function render(node: React.ReactNode) {
	const container =
		document.getElementById('root') ?? document.body.appendChild(document.createElement('div'))
	const root = createRoot(container)
	act(() => root.render(node))
	return root
}

function rect(input: { left: number; top: number; width: number; height: number }) {
	return {
		...input,
		right: input.left + input.width,
		bottom: input.top + input.height,
		x: input.left,
		y: input.top,
		toJSON: () => input,
	} as DOMRect
}
