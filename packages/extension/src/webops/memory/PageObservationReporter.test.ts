import { describe, expect, it, vi } from 'vitest'

import { PageObservationReporter } from './PageObservationReporter'
import type { MemoryClientLike } from './types'

describe('PageObservationReporter', () => {
	it('builds a page observation request from the current page observation', async () => {
		const observePage = vi
			.fn<NonNullable<MemoryClientLike['observePage']>>()
			.mockResolvedValue({ ok: true })
		const reporter = new PageObservationReporter({ observePage })

		await reporter.report({
			projectId: 'default',
			task: '查看服务',
			url: 'https://ops.example.test/service',
			pageObservation: {
				title: '服务管理',
				visibleText: ['服务列表', '搜索'],
				controls: [{ role: 'button', name: '搜索' }],
				breadcrumbs: ['首页', '服务管理'],
				activeTabs: ['运行状态'],
				tables: [{ headers: ['服务名称', '状态', '操作'] }],
				activeSurfaces: [
					{
						surfaceType: 'drawer',
						title: '筛选',
						controls: [{ role: 'button', name: '应用' }],
					},
				],
			},
			allowPageSummary: true,
		})

		expect(observePage).toHaveBeenCalledWith({
			projectId: 'default',
			task: '查看服务',
			url: 'https://ops.example.test/service',
			title: '服务管理',
			visibleText: ['服务列表', '搜索'],
			controls: [{ role: 'button', name: '搜索' }],
			breadcrumbs: ['首页', '服务管理'],
			activeTabs: ['运行状态'],
			tables: [{ headers: ['服务名称', '状态', '操作'] }],
			activeSurfaces: [
				{
					surfaceType: 'drawer',
					title: '筛选',
					controls: [{ role: 'button', name: '应用' }],
				},
			],
		})
	})

	it('omits visible text when page summaries are disabled', async () => {
		const observePage = vi
			.fn<NonNullable<MemoryClientLike['observePage']>>()
			.mockResolvedValue({ ok: true })
		const reporter = new PageObservationReporter({ observePage })

		await reporter.report({
			projectId: 'default',
			task: '查看服务',
			url: 'https://ops.example.test/service',
			pageObservation: {
				title: '服务管理',
				visibleText: ['服务列表', '搜索'],
				controls: [{ role: 'button', name: '搜索' }],
			},
			allowPageSummary: false,
		})

		expect(observePage).toHaveBeenCalledWith(
			expect.objectContaining({
				visibleText: [],
				controls: [{ role: 'button', name: '搜索' }],
			})
		)
	})

	it('warns and does not throw when upload fails', async () => {
		const observePage = vi
			.fn<NonNullable<MemoryClientLike['observePage']>>()
			.mockRejectedValue(new Error('network down'))
		const warn = vi.spyOn(console, 'warn').mockImplementation(() => undefined)
		const reporter = new PageObservationReporter({ observePage })

		await expect(
			reporter.report({
				projectId: 'default',
				task: '查看服务',
				url: 'https://ops.example.test/service',
				pageObservation: { title: '服务管理', visibleText: [], controls: [] },
				allowPageSummary: true,
			})
		).resolves.toBeUndefined()

		expect(warn).toHaveBeenCalledWith(
			'[WebOpsMemory] Failed to report page observation:',
			'network down'
		)
		warn.mockRestore()
	})
})
