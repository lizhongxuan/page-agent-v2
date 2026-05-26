import { describe, expect, it } from 'vitest'

import { createAskUserBridge } from './askUserBridge'

describe('createAskUserBridge', () => {
	it('keeps an ask_user prompt pending until the side-panel answer resolves it', async () => {
		const changes: (string | null)[] = []
		const bridge = createAskUserBridge((pending) => changes.push(pending?.question ?? null))

		const answerPromise = bridge.ask('请提供股票名称或代码')

		expect(bridge.getPending()?.question).toBe('请提供股票名称或代码')
		expect(changes).toEqual(['请提供股票名称或代码'])

		expect(bridge.answer('金蝶')).toBe(true)

		await expect(answerPromise).resolves.toBe('金蝶')
		expect(bridge.getPending()).toBeNull()
		expect(changes).toEqual(['请提供股票名称或代码', null])
	})

	it('resolves a pending prompt when the task is cancelled', async () => {
		const changes: (string | null)[] = []
		const bridge = createAskUserBridge((pending) => changes.push(pending?.question ?? null))

		const answerPromise = bridge.ask('需要你确认')

		expect(bridge.cancel('用户停止了任务')).toBe(true)

		await expect(answerPromise).resolves.toBe('用户停止了任务')
		expect(bridge.getPending()).toBeNull()
		expect(changes).toEqual(['需要你确认', null])
	})
})
