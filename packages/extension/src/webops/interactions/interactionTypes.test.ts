// @vitest-environment jsdom
import { describe, expect, it } from 'vitest'

import {
	WEBOPS_INTERACTION_RESPONSE_EVENT,
	emitInteractionResponse,
	normalizeInteractionEvent,
} from './interactionTypes'

describe('normalizeInteractionEvent', () => {
	it('normalizes spotlight timeout and message', () => {
		const event = normalizeInteractionEvent({
			type: 'spotlight',
			elementIndex: 3,
			action: 'input',
			message: '正在输入搜索关键词',
		})

		expect(event.timeoutMs).toBe(1200)
		expect(event.message).toBe('正在输入搜索关键词')
	})

	it('normalizes toast timeout', () => {
		const event = normalizeInteractionEvent({
			type: 'toast',
			message: '正在搜索',
			level: 'info',
		})

		expect(event.timeoutMs).toBe(1800)
	})

	it('keeps handover resume label explicit', () => {
		const event = normalizeInteractionEvent({
			type: 'handover',
			title: '需要手动验证',
			message: '请完成验证码',
			resumeButtonLabel: '我已完成，继续',
		})

		expect(event.resumeButtonLabel).toBe('我已完成，继续')
	})

	it('normalizes input submit label and emits request-scoped responses', () => {
		const event = normalizeInteractionEvent({
			type: 'input',
			requestId: 'r1',
			title: '需要补充信息',
			message: '请输入环境',
		})
		const responses: unknown[] = []
		window.addEventListener(WEBOPS_INTERACTION_RESPONSE_EVENT, (nativeEvent) => {
			responses.push((nativeEvent as CustomEvent).detail)
		})

		emitInteractionResponse(event.requestId, { type: 'input', value: '生产环境' })

		expect(event.submitButtonLabel).toBe('提交并继续')
		expect(responses).toEqual([{ requestId: 'r1', response: { type: 'input', value: '生产环境' } }])
	})
})
