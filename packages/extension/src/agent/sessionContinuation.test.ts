import { describe, expect, it } from 'vitest'

import { buildSessionContinuationTask, formatSessionDisplayTask } from './sessionContinuation'

describe('session continuation', () => {
	it('turns user input into a continuation of the current session', () => {
		const task = buildSessionContinuationTask({
			previousTask: '搜索百度最近新闻',
			userMessage: '多搜索一下',
		})

		expect(task).toContain('继续当前浏览器会话')
		expect(task).toContain('上一轮会话任务：搜索百度最近新闻')
		expect(task).toContain('用户新的补充要求：多搜索一下')
		expect(formatSessionDisplayTask('搜索百度最近新闻', '多搜索一下')).toBe(
			'搜索百度最近新闻\n补充：多搜索一下'
		)
	})
})
