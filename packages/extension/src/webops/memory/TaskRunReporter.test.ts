import { describe, expect, it, vi } from 'vitest'

import type { RecordedSession } from '../recorder/actionEvents'
import { TaskRunReporter, buildMemoryTaskRunPayload } from './TaskRunReporter'
import type { MemoryClientLike } from './types'

describe('buildMemoryTaskRunPayload', () => {
	it('builds a successful task run payload from a recorded session', () => {
		const payload = buildMemoryTaskRunPayload({
			projectId: 'default',
			session: sampleSession(),
			result: { success: true, summary: '服务状态是 running' },
			reasoning: [{ nextGoal: '使用服务名称搜索框定位服务。' }],
			memoryContextId: 'ctx_1',
			memoryEvidenceRefs: [
				{ source: 'manual', id: 'chunk_1', rank: 1, score: 0.9 },
				{ source: 'experience', id: 'exp_1', rank: 2, score: 0.82 },
			],
		})

		expect(payload).toMatchObject({
			id: 'task_run_1',
			projectId: 'default',
			site: 'ops.example.test',
			taskTemplate: '查看 {{service_name}} 运行状态',
			status: 'success',
			summary: '服务状态是 running',
			memoryContextId: 'ctx_1',
			memoryEvidenceRefs: [
				{ source: 'manual', id: 'chunk_1', rank: 1, score: 0.9 },
				{ source: 'experience', id: 'exp_1', rank: 2, score: 0.82 },
			],
		})
		expect(payload?.actionSteps?.[1]).toMatchObject({
			actionType: 'fill',
			targetName: '服务名称搜索框',
			valueTemplate: '{{service_name}}',
		})
		expect(JSON.stringify(payload)).not.toContain('kme-prod-001')
	})

	it('omits sensitive task runs and sensitive action values', () => {
		expect(
			buildMemoryTaskRunPayload({
				projectId: 'default',
				session: { ...sampleSession(), task: '输入 password 登录' },
				result: { success: true },
			})
		).toBeNull()

		const session = sampleSession()
		session.steps[1] = {
			...session.steps[1],
			value: '[REDACTED]',
			target: { name: 'password' },
		}

		const payload = buildMemoryTaskRunPayload({
			projectId: 'default',
			session,
			result: { success: false, summary: 'Sensitive token abc was omitted.' },
		})

		expect(JSON.stringify(payload)).not.toContain('[REDACTED]')
		expect(JSON.stringify(payload)).not.toContain('Sensitive token abc')
		expect(payload?.status).toBe('failed')
	})

	it('templates variable instances in summaries, targets, and action results', () => {
		const session = sampleSession()
		session.steps[1] = {
			...session.steps[1],
			note: '已搜索 kme-prod-001。',
		}
		session.steps.push({
			id: 'c',
			type: 'click',
			timestamp: Date.now(),
			pageUrl: 'https://ops.example.test/service/search',
			pageTitle: '搜索页',
			target: { name: '查询 kme-prod-001' },
			result: 'success',
			note: '打开 kme-prod-001 详情。',
		})

		const payload = buildMemoryTaskRunPayload({
			projectId: 'default',
			session,
			result: { success: true, summary: 'kme-prod-001 已找到负责人。' },
		})

		expect(JSON.stringify(payload)).not.toContain('kme-prod-001')
		expect(payload?.summary).toBe('{{service_name}} 已找到负责人。')
		expect(payload?.actionSteps?.[1]).toMatchObject({
			targetName: '服务名称搜索框',
			resultSummary: '已搜索 {{service_name}}。',
		})
		expect(payload?.actionSteps?.[2]).toMatchObject({
			targetName: '查询 {{service_name}}',
			resultSummary: '打开 {{service_name}} 详情。',
		})
	})

	it('templates stale service-like target labels that are not the current action value', () => {
		const session = sampleSession()
		session.task = '查看 payment-api 运行状态'
		session.steps[1] = {
			...session.steps[1],
			value: 'payment-api',
			note: '已搜索 payment-api。',
		}
		session.steps.push({
			id: 'c',
			type: 'click',
			timestamp: Date.now(),
			pageUrl: 'https://ops.example.test/service/search',
			pageTitle: '搜索页',
			target: { name: '查询 checkout-api' },
			result: 'success',
			note: '打开 checkout-api 详情。',
		})

		const payload = buildMemoryTaskRunPayload({
			projectId: 'default',
			session,
			result: { success: true, summary: 'payment-api 已找到负责人。' },
		})

		const payloadText = JSON.stringify(payload)
		expect(payloadText).not.toContain('payment-api')
		expect(payloadText).not.toContain('checkout-api')
		expect(payload?.summary).toBe('{{service_name}} 已找到负责人。')
		expect(payload?.actionSteps?.[2]).toMatchObject({
			targetName: '查询 {{service_name}}',
			resultSummary: '打开 {{service_name}} 详情。',
		})
	})

	it('uses backend page state aliases from page observation responses', () => {
		const payload = buildMemoryTaskRunPayload({
			projectId: 'default',
			session: sampleSession(),
			result: { success: true, summary: '服务状态是 running' },
			pageStateAliases: {
				'ops.example.test_service': 'page_service_management_abc123',
				'ops.example.test_service_search': 'page_service_search_def456',
			},
		})

		expect(payload?.originalPath).toEqual([
			'page_service_management_abc123',
			'page_service_search_def456',
		])
		expect(payload?.optimizedPath).toEqual([
			'page_service_management_abc123',
			'page_service_search_def456',
		])
		expect(payload?.actionSteps?.map((step) => step.pageStateId)).toEqual([
			'page_service_management_abc123',
			'page_service_search_def456',
		])
	})

	it('attaches the current surface id to action steps on the matched current page', () => {
		const session = {
			...sampleSession(),
			memoryContext: {
				contextId: 'ctx_surface',
				contextPrompt: '<webops_memory />',
				currentPageState: { id: 'page_service_management_abc123', name: '服务管理' },
				currentSurface: {
					id: 'surface_filter_drawer',
					type: 'drawer',
					parentPageStateId: 'page_service_management_abc123',
				},
				evidenceRefs: [],
			},
		}

		const payload = buildMemoryTaskRunPayload({
			projectId: 'default',
			session,
			result: { success: true, summary: '服务状态是 running' },
			pageStateAliases: {
				'ops.example.test_service': 'page_service_management_abc123',
				'ops.example.test_service_search': 'page_service_search_def456',
			},
		})

		expect(payload?.actionSteps?.[0]).toMatchObject({
			pageStateId: 'page_service_management_abc123',
			surfaceId: 'surface_filter_drawer',
		})
		expect(payload?.actionSteps?.[1]).not.toHaveProperty('surfaceId')
	})

	it('preserves an explicitly recorded surface id for later dynamic surfaces', () => {
		const session = sampleSession()
		session.steps[1] = { ...session.steps[1], surfaceId: 'surface_search_popover' }

		const payload = buildMemoryTaskRunPayload({
			projectId: 'default',
			session,
			result: { success: true, summary: '服务状态是 running' },
		})

		expect(payload?.actionSteps?.[1]).toMatchObject({
			surfaceId: 'surface_search_popover',
		})
	})

	it('preserves before and after observations on action steps', () => {
		const session = sampleSession()
		session.steps[0] = {
			...session.steps[0],
			beforeObservation: {
				site: 'ops.example.test',
				title: '服务管理',
				activeTabs: ['运行状态'],
				controlSignatures: [{ role: 'button', name: '搜索' }],
			},
			afterObservation: {
				site: 'ops.example.test',
				title: '筛选抽屉',
				activeSurfaces: [
					{
						surfaceType: 'drawer',
						title: '筛选',
						controls: [{ role: 'button', name: '应用' }],
					},
				],
			},
		}

		const payload = buildMemoryTaskRunPayload({
			projectId: 'default',
			session,
			result: { success: true, summary: '服务状态是 running' },
		})

		expect(payload?.actionSteps?.[0]?.beforeObservation).toMatchObject({
			activeTabs: ['运行状态'],
		})
		expect(payload?.actionSteps?.[0]?.afterObservation).toMatchObject({
			activeSurfaces: [{ surfaceType: 'drawer', title: '筛选' }],
		})
	})

	it('keeps URL ports in site keys so local page observations and task runs match', () => {
		const session = { ...sampleSession(), startUrl: 'http://127.0.0.1:38403/service' }
		session.steps = session.steps.map((step) => ({
			...step,
			pageUrl: step.pageUrl.replace('https://ops.example.test', 'http://127.0.0.1:38403'),
		}))

		const payload = buildMemoryTaskRunPayload({
			projectId: 'default',
			session,
			result: { success: true },
		})

		expect(payload?.site).toBe('127.0.0.1:38403')
	})
})

describe('TaskRunReporter', () => {
	it('uploads the built task run payload', async () => {
		const completeTaskRun = vi
			.fn<NonNullable<MemoryClientLike['completeTaskRun']>>()
			.mockResolvedValue({ ok: true, runId: 'task_run_1' })
		const reporter = new TaskRunReporter({ completeTaskRun })

		await reporter.complete({
			projectId: 'default',
			session: sampleSession(),
			result: { success: true, summary: '服务状态是 running' },
			memoryContextId: 'ctx_1',
			memoryEvidenceRefs: [{ source: 'manual', id: 'chunk_1', rank: 1, score: 0.9 }],
		})

		expect(completeTaskRun).toHaveBeenCalledWith(
			expect.objectContaining({
				id: 'task_run_1',
				status: 'success',
				memoryContextId: 'ctx_1',
				memoryEvidenceRefs: [{ source: 'manual', id: 'chunk_1', rank: 1, score: 0.9 }],
			})
		)
	})

	it('warns and does not throw when upload fails', async () => {
		const completeTaskRun = vi
			.fn<NonNullable<MemoryClientLike['completeTaskRun']>>()
			.mockRejectedValue(new Error('network down'))
		const warn = vi.spyOn(console, 'warn').mockImplementation(() => undefined)
		const reporter = new TaskRunReporter({ completeTaskRun })

		await expect(
			reporter.complete({
				projectId: 'default',
				session: sampleSession(),
				result: { success: true },
			})
		).resolves.toBeUndefined()

		expect(warn).toHaveBeenCalledWith('[WebOpsMemory] Failed to upload task run:', 'network down')
		warn.mockRestore()
	})
})

function sampleSession(): RecordedSession {
	return {
		id: 'task_run_1',
		task: '查看 kme-prod-001 运行状态',
		startUrl: 'https://ops.example.test/service',
		startedAt: Date.now(),
		memoryHits: [],
		redactionReport: [],
		steps: [
			{
				id: 'a',
				type: 'click',
				timestamp: Date.now(),
				pageUrl: 'https://ops.example.test/service',
				pageTitle: '服务管理',
				target: { name: '服务名称搜索框' },
				result: 'success',
			},
			{
				id: 'b',
				type: 'input',
				timestamp: Date.now(),
				pageUrl: 'https://ops.example.test/service/search',
				pageTitle: '搜索页',
				target: { name: '服务名称搜索框' },
				value: 'kme-prod-001',
				result: 'success',
				note: '搜索已提交。',
			},
		],
	}
}
