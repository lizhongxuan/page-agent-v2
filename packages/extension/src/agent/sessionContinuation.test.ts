import { ContinuationResolver } from '@page-agent/core'
import { describe, expect, it } from 'vitest'

import {
	buildResolvedSessionContinuation,
	buildSessionContinuationTask,
	formatSessionDisplayTask,
	getSessionContinuationBaseTask,
	toContinuationResolverInput,
} from './sessionContinuation'

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

	it('keeps repeated continuation task display bounded to the latest supplement', () => {
		const firstDisplay = formatSessionDisplayTask('搜索A股今天行情', '分析什么股好')
		const secondDisplay = formatSessionDisplayTask(firstDisplay, '那pcb板块呢?')
		const thirdDisplay = formatSessionDisplayTask(secondDisplay, '再看财报')

		expect(getSessionContinuationBaseTask(thirdDisplay)).toBe('搜索A股今天行情')
		expect(secondDisplay).toBe('搜索A股今天行情\n补充：那pcb板块呢?')
		expect(thirdDisplay).toBe('搜索A股今天行情\n补充：再看财报')
		expect(thirdDisplay).not.toContain('分析什么股好')
		expect(thirdDisplay).not.toContain('那pcb板块呢?')
	})

	it('builds resolver input from extension continuation context', () => {
		expect(
			toContinuationResolverInput({
				previousTask: '检查 PG pg-1 的故障',
				userMessage: '总结这个页面有哪些按钮',
				currentUrl: 'https://example.com/pg/pg-1',
				currentTitle: 'PG pg-1',
				previousUrl: 'https://example.com/pg/pg-1',
				previousTitle: 'PG pg-1',
				previousBusinessObjects: [{ type: 'pg', id: 'pg-1', evidence: ['PG pg-1'] }],
			})
		).toEqual({
			previousTask: '检查 PG pg-1 的故障',
			userMessage: '总结这个页面有哪些按钮',
			currentUrl: 'https://example.com/pg/pg-1',
			currentTitle: 'PG pg-1',
			previousUrl: 'https://example.com/pg/pg-1',
			previousTitle: 'PG pg-1',
			previousBusinessObjects: [{ type: 'pg', id: 'pg-1', evidence: ['PG pg-1'] }],
		})
	})

	it('can classify same-page new questions without forcing old history continuation', () => {
		const resolver = new ContinuationResolver()
		const decision = resolver.resolve({
			previousTask: '检查 PG pg-1 的故障',
			userMessage: '总结这个页面有哪些按钮',
			currentUrl: 'https://example.com/pg/pg-1',
			currentTitle: 'PG pg-1',
			previousUrl: 'https://example.com/pg/pg-1',
			previousTitle: 'PG pg-1',
			previousBusinessObjects: [{ type: 'pg', id: 'pg-1', evidence: ['PG pg-1'] }],
		})

		expect(decision.mode).toBe('new_task_same_page')
	})

	it('drops carried history for same-page new tasks', () => {
		const resolved = buildResolvedSessionContinuation({
			task: '总结这个页面有哪些按钮',
			carryHistory: [{ type: 'observation', content: 'previous' }],
			context: {
				previousTask: '检查 PG pg-1 的故障',
				userMessage: '总结这个页面有哪些按钮',
				currentUrl: 'https://example.com/pg/pg-1',
				currentTitle: 'PG pg-1',
			},
			decision: {
				mode: 'new_task_same_page',
				reason: 'Same page but user intent changed.',
				inheritedSections: ['currentPage', 'instructions'],
				discardedSections: ['task', 'sessionSummary', 'recentSteps'],
				requiresFreshObserve: true,
				requiresUserConfirmation: false,
			},
		})

		expect(resolved.task).toBe('总结这个页面有哪些按钮')
		expect(resolved.displayTask).toBe('总结这个页面有哪些按钮')
		expect(resolved.carryHistory).toBeUndefined()
		expect(resolved.decision?.mode).toBe('new_task_same_page')
	})
})
