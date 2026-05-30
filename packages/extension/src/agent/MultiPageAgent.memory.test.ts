import { beforeEach, describe, expect, it, vi } from 'vitest'

import { MultiPageAgent, buildTaskRunPayload } from './MultiPageAgent'

const mocks = vi.hoisted(() => {
	const tabsControllerInstances: {
		currentTabId: number | null
		init: ReturnType<typeof vi.fn>
		getTabInfo: ReturnType<typeof vi.fn>
		waitUntilTabLoaded: ReturnType<typeof vi.fn>
	}[] = []
	const remotePageControllerInstances: {
		getWorkflowElements: ReturnType<typeof vi.fn>
	}[] = []
	const coreConfigs: any[] = []

	return { tabsControllerInstances, remotePageControllerInstances, coreConfigs }
})

vi.mock('@page-agent/core', () => ({
	PageAgentCore: class {
		history = []
		addEventListener = vi.fn()
		removeEventListener = vi.fn()
		dispose = vi.fn()
		pushObservation = vi.fn()
		constructor(config: any) {
			mocks.coreConfigs.push(config)
		}
	},
}))

vi.mock('./TabsController', () => ({
	TabsController: class {
		currentTabId: number | null = null
		init = vi.fn(async () => {
			this.currentTabId = 123
		})
		getTabInfo = vi.fn(async () => ({
			title: 'GitHub fixture',
			url: 'http://127.0.0.1:60231/workflow-github-issues.html',
		}))
		waitUntilTabLoaded = vi.fn(async () => undefined)
		constructor() {
			mocks.tabsControllerInstances.push(this)
		}
	},
}))

vi.mock('./RemotePageController', () => ({
	RemotePageController: class {
		getWorkflowElements = vi.fn(async () => ({
			visibleText: ['Code', 'Issues', 'Search all issues'],
			controls: [{ role: 'link', name: 'Issues' }],
		}))
		constructor() {
			mocks.remotePageControllerInstances.push(this)
		}
	},
}))

vi.mock('./tabTools', () => ({
	createTabTools: () => [],
}))

describe('MultiPageAgent memory observation', () => {
	beforeEach(() => {
		mocks.coreConfigs.length = 0
		mocks.tabsControllerInstances.length = 0
		mocks.remotePageControllerInstances.length = 0
		vi.stubGlobal('chrome', {
			storage: {
				local: {
					set: vi.fn(async () => undefined),
				},
			},
		})
		vi.stubGlobal('window', {
			setInterval: vi.fn(() => 1),
			clearInterval: vi.fn(),
		})
	})

	it('initializes the current web tab before memory reads the page observation', async () => {
		const agent = new MultiPageAgent({} as never)

		const observation = await agent.getCurrentPageObservation(
			'在 github.com/microsoft/typescript 的 Issues 里搜索 timeout 报错'
		)

		const tabsController = mocks.tabsControllerInstances[0]
		expect(tabsController?.init).toHaveBeenCalledWith(
			'在 github.com/microsoft/typescript 的 Issues 里搜索 timeout 报错',
			{
				includeInitialTab: true,
				experimentalIncludeAllTabs: false,
			}
		)
		expect(tabsController?.getTabInfo).toHaveBeenCalledWith(123)
		expect(observation).toEqual({
			url: 'http://127.0.0.1:60231/workflow-github-issues.html',
			title: 'GitHub fixture',
			visibleText: ['Code', 'Issues', 'Search all issues'],
			controls: [{ role: 'link', name: 'Issues' }],
		})
	})

	it('reports page observation and injects webops memory before the first step', async () => {
		const fetchCalls: { url: string | URL | Request; init?: RequestInit }[] = []
		vi.stubGlobal('fetch', async (url: string | URL | Request, init?: RequestInit) => {
			fetchCalls.push({ url, init })
			const path = requestURLToString(url)
			if (path.endsWith('/api/memory/context')) {
				return new Response(
					JSON.stringify({
						contextPrompt: '<webops_memory>Use service search.</webops_memory>',
						knowledgeEvidence: [{ chunkId: 'chunk_1', title: 'Service manual', score: 0.9 }],
					}),
					{ status: 200, headers: { 'content-type': 'application/json' } }
				)
			}
			return new Response(JSON.stringify({ pageStateId: 'page_service_list' }), {
				status: 200,
				headers: { 'content-type': 'application/json' },
			})
		})
		const agent = new MultiPageAgent({
			workflowBackend: {
				baseUrl: 'https://memory.example.test',
				projectId: 'default',
			},
			knowledgeSettings: { enabled: true },
		} as never)
		const coreConfig = mocks.coreConfigs[0]
		const runtimeAgent = {
			task: '查看服务状态',
			taskId: 'task_1',
			pushObservation: vi.fn(),
		}

		await coreConfig.onBeforeTask(runtimeAgent)
		await coreConfig.onBeforeStep(runtimeAgent, 0)

		expect(fetchCalls.map((call) => requestURLToString(call.url))).toEqual([
			'https://memory.example.test/api/memory/page-observations',
			'https://memory.example.test/api/memory/context',
		])
		expect(JSON.parse(fetchCalls[1]?.init?.body as string)).toMatchObject({
			projectId: 'default',
			task: '查看服务状态',
			currentUrl: 'http://127.0.0.1:60231/workflow-github-issues.html',
		})
		expect(runtimeAgent.pushObservation).toHaveBeenCalledWith(
			'<webops_memory>Use service search.</webops_memory>'
		)
		expect(agent.getWebOpsSession()).toBeUndefined()
	})

	it('loads memory context without legacy retrieval request fields', async () => {
		const fetchCalls: { url: string | URL | Request; init?: RequestInit }[] = []
		vi.stubGlobal('fetch', async (url: string | URL | Request, init?: RequestInit) => {
			fetchCalls.push({ url, init })
			const path = requestURLToString(url)
			if (path.endsWith('/api/memory/context')) {
				return new Response(
					JSON.stringify({
						recommendedMode: 'guided',
						contextPrompt: '<webops_memory>Use prior hints.</webops_memory>',
					}),
					{ status: 200, headers: { 'content-type': 'application/json' } }
				)
			}
			return new Response(JSON.stringify({ pageStateId: 'page_service_list' }), {
				status: 200,
				headers: { 'content-type': 'application/json' },
			})
		})
		const agent = new MultiPageAgent({
			workflowBackend: {
				baseUrl: 'https://memory.example.test',
				projectId: 'default',
			},
		} as never)
		const coreConfig = mocks.coreConfigs[0]

		await coreConfig.onBeforeTask({
			task: '查看服务状态',
			taskId: 'task_1',
			pushObservation: vi.fn(),
		})

		expect(fetchCalls.map((call) => requestURLToString(call.url))).toEqual([
			'https://memory.example.test/api/memory/page-observations',
			'https://memory.example.test/api/memory/context',
		])
		expect(JSON.parse(fetchCalls[1]?.init?.body as string)).toMatchObject({
			task: '查看服务状态',
		})
	})

	it('warms workflow elements before the first step when memory is disabled', async () => {
		const agent = new MultiPageAgent({} as never)
		const coreConfig = mocks.coreConfigs[0]

		await coreConfig.onBeforeTask({
			task: '查看服务状态',
			taskId: 'task_1',
			pushObservation: vi.fn(),
		})

		expect(agent.getWebOpsSession()).toBeUndefined()
		expect(mocks.remotePageControllerInstances[0]?.getWorkflowElements).toHaveBeenCalled()
	})

	it('builds a templated task run payload from a completed webops session', () => {
		const payload = buildTaskRunPayload({
			projectId: 'default',
			result: {
				success: true,
				data: '服务状态是 running',
				history: [],
			},
			history: [
				{
					type: 'step',
					stepIndex: 0,
					reflection: {
						next_goal: '使用服务名称搜索框定位服务。',
					},
					action: {
						name: 'input_text',
						input: {},
						output: 'ok',
					},
					usage: {
						promptTokens: 0,
						completionTokens: 0,
						totalTokens: 0,
					},
				},
			],
			session: {
				id: 'task_run_1',
				task: '查看 kme-prod-001 运行状态',
				startUrl: 'https://ops.example.com/service',
				startedAt: Date.now(),
				knowledgeHits: [],
				redactionReport: [],
				steps: [
					{
						id: 'a',
						type: 'click',
						timestamp: Date.now(),
						pageUrl: 'https://ops.example.com/service',
						pageTitle: '服务管理',
						target: { name: '服务名称搜索框' },
						result: 'success',
					},
					{
						id: 'b',
						type: 'input',
						timestamp: Date.now(),
						pageUrl: 'https://ops.example.com/service/search',
						pageTitle: '搜索页',
						target: { name: '服务名称搜索框' },
						value: 'kme-prod-001',
						result: 'success',
						note: '搜索已提交。',
					},
					{
						id: 'c',
						type: 'click',
						timestamp: Date.now(),
						pageUrl: 'https://ops.example.com/service/detail/12345',
						pageTitle: '详情页',
						target: { name: '返回' },
						result: 'success',
					},
					{
						id: 'd',
						type: 'click',
						timestamp: Date.now(),
						pageUrl: 'https://ops.example.com/service',
						pageTitle: '服务管理',
						target: { name: '详情链接' },
						result: 'success',
					},
					{
						id: 'e',
						type: 'click',
						timestamp: Date.now(),
						pageUrl: 'https://ops.example.com/service/status',
						pageTitle: '运行状态',
						target: { name: '状态列' },
						result: 'success',
					},
				],
			},
		})

		expect(payload).toMatchObject({
			id: 'task_run_1',
			projectId: 'default',
			site: 'ops.example.com',
			taskTemplate: '查看 {{service_name}} 运行状态',
			status: 'success',
		})
		expect(payload?.taskTemplate).not.toContain('kme-prod-001')
		expect(payload?.actionSteps?.[1]?.valueTemplate).toBe('{{service_name}}')
		expect(payload?.optimizedPath).toEqual([
			'ops.example.com_service',
			'ops.example.com_service_status',
		])
	})

	it('templates stale service-like target labels in the task run payload', () => {
		const payload = buildTaskRunPayload({
			projectId: 'default',
			result: {
				success: true,
				data: 'payment-api 已找到负责人。',
				history: [],
			},
			history: [],
			session: {
				id: 'task_run_1',
				task: '查看 payment-api 运行状态',
				startUrl: 'https://ops.example.com/service',
				startedAt: Date.now(),
				knowledgeHits: [],
				redactionReport: [],
				steps: [
					{
						id: 'a',
						type: 'input',
						timestamp: Date.now(),
						pageUrl: 'https://ops.example.com/service/search',
						pageTitle: '搜索页',
						target: { name: '服务名称搜索框' },
						value: 'payment-api',
						result: 'success',
						note: '已搜索 payment-api。',
					},
					{
						id: 'b',
						type: 'click',
						timestamp: Date.now(),
						pageUrl: 'https://ops.example.com/service/search',
						pageTitle: '搜索页',
						target: { name: '查询 checkout-api' },
						result: 'success',
						note: '打开 checkout-api 详情。',
					},
				],
			},
		})

		const payloadText = JSON.stringify(payload)
		expect(payloadText).not.toContain('payment-api')
		expect(payloadText).not.toContain('checkout-api')
		expect(payload?.summary).toBe('{{service_name}} 已找到负责人。')
		expect(payload?.actionSteps?.[1]).toMatchObject({
			targetName: '查询 {{service_name}}',
			resultSummary: '打开 {{service_name}} 详情。',
		})
	})

	it('records backend memoryUpdates from completed task runs into session metadata', async () => {
		const storageSet = vi.fn(async () => undefined)
		const fetchCalls: { url: string | URL | Request; init?: RequestInit }[] = []
		vi.stubGlobal('chrome', {
			storage: {
				local: {
					set: storageSet,
				},
			},
		})
		vi.stubGlobal('fetch', async (url: string | URL | Request, init?: RequestInit) => {
			fetchCalls.push({ url, init })
			const path = requestURLToString(url)
			if (path.endsWith('/api/memory/task-runs')) {
				return new Response(
					JSON.stringify({
						taskRunId: 'task_run_1',
						memoryUpdates: ['experience_created', 'transition_updated'],
					}),
					{ status: 201, headers: { 'content-type': 'application/json' } }
				)
			}
			if (path.endsWith('/api/memory/context')) {
				return new Response(
					JSON.stringify({
						contextId: 'ctx_1',
						recommendedMode: 'normal',
						contextPrompt: '',
						evidenceRefs: [
							{ source: 'knowledge', id: 'chunk_1', rank: 1, score: 0.9 },
							{ source: 'experience', id: 'exp_1', rank: 2, score: 0.8 },
						],
					}),
					{
						status: 200,
						headers: { 'content-type': 'application/json' },
					}
				)
			}
			return new Response(JSON.stringify({ pageStateId: 'page_service_list' }), {
				status: 200,
				headers: { 'content-type': 'application/json' },
			})
		})
		const agent = new MultiPageAgent({
			workflowBackend: {
				baseUrl: 'https://memory.example.test',
				projectId: 'default',
			},
		} as never)
		const coreConfig = mocks.coreConfigs[0]

		await coreConfig.onBeforeTask({
			task: '查看服务状态',
			taskId: 'task_1',
			pushObservation: vi.fn(),
		})
		await coreConfig.onAfterTask(
			{ history: [] },
			{ success: true, data: '服务状态是 running', history: [] }
		)

		const taskRunCall = fetchCalls.find((call) =>
			requestURLToString(call.url).endsWith('/api/memory/task-runs')
		)
		const taskRunBody = taskRunCall?.init?.body as string
		expect(JSON.parse(taskRunBody)).toMatchObject({
			memoryContextId: 'ctx_1',
			memoryEvidenceRefs: [
				{ source: 'knowledge', id: 'chunk_1', rank: 1, score: 0.9 },
				{ source: 'experience', id: 'exp_1', rank: 2, score: 0.8 },
			],
		})
		expect(taskRunBody).not.toContain('kme-prod-001')
		expect(storageSet).toHaveBeenCalledWith({
			lastWebOpsSession: expect.objectContaining({
				memoryUpdates: ['experience_created', 'transition_updated'],
			}),
		})
	})

	it('keeps memory context attribution refs for task-run upload', async () => {
		const fetchCalls: { url: string | URL | Request; init?: RequestInit }[] = []
		vi.stubGlobal('fetch', async (url: string | URL | Request, init?: RequestInit) => {
			fetchCalls.push({ url, init })
			const path = requestURLToString(url)
			if (path.endsWith('/api/memory/context')) {
				return new Response(
					JSON.stringify({
						contextId: 'ctx_memory_1',
						recommendedMode: 'guided',
						evidenceRefs: [
							{ source: 'knowledge', id: 'chunk_1', rank: 1, score: 0.8 },
							{ source: 'experience', id: 'exp_1', rank: 2, score: 0.7 },
						],
					}),
					{ status: 200, headers: { 'content-type': 'application/json' } }
				)
			}
			if (path.endsWith('/api/memory/task-runs')) {
				return new Response(JSON.stringify({ taskRunId: 'task_run_1' }), {
					status: 201,
					headers: { 'content-type': 'application/json' },
				})
			}
			return new Response(JSON.stringify({ pageStateId: 'page_service_list' }), {
				status: 200,
				headers: { 'content-type': 'application/json' },
			})
		})
		const agent = new MultiPageAgent({
			workflowBackend: {
				baseUrl: 'https://memory.example.test',
				projectId: 'default',
			},
		} as never)
		const coreConfig = mocks.coreConfigs[0]

		await coreConfig.onBeforeTask({
			task: '搜索 timeout',
			taskId: 'task_1',
			pushObservation: vi.fn(),
		})
		await coreConfig.onAfterTask(
			{ history: [] },
			{ success: true, data: '搜索结果已展示', history: [] }
		)

		const taskRunCall = fetchCalls.find((call) =>
			requestURLToString(call.url).endsWith('/api/memory/task-runs')
		)
		expect(JSON.parse(taskRunCall?.init?.body as string)).toMatchObject({
			memoryContextId: 'ctx_memory_1',
			memoryEvidenceRefs: [
				{ source: 'knowledge', id: 'chunk_1', rank: 1, score: 0.8 },
				{ source: 'experience', id: 'exp_1', rank: 2, score: 0.7 },
			],
		})
	})

	it('records injected memory debug data and uploads full evidence refs', async () => {
		const storageSet = vi.fn(async () => undefined)
		const fetchCalls: { url: string | URL | Request; init?: RequestInit }[] = []
		vi.stubGlobal('chrome', {
			storage: {
				local: {
					set: storageSet,
				},
			},
		})
		vi.stubGlobal('fetch', async (url: string | URL | Request, init?: RequestInit) => {
			fetchCalls.push({ url, init })
			const path = requestURLToString(url)
			if (path.endsWith('/api/memory/context')) {
				return new Response(
					JSON.stringify({
						contextId: 'ctx_debug_1',
						recommendedMode: 'guided',
						contextPrompt: '<webops_memory>Use the service search box.</webops_memory>',
						currentSurface: {
							id: 'surface_filter_drawer',
							type: 'drawer',
							name: 'Filter drawer',
							parentPageStateId: 'page_service_list',
						},
						debug: {
							promptChars: 64,
							promptBudget: { maxChars: 5000, usedChars: 64 },
							filteredEvidence: [
								{ source: 'knowledge', id: 'chunk_wrong', reason: 'hard_gate_failed' },
							],
						},
						evidenceRefs: [
							{
								source: 'experience',
								id: 'exp_service_search',
								rank: 1,
								score: 0.91,
								title: 'Search service from service list',
								reason: 'Matched a reusable successful experience for this task.',
								matchedRules: ['start_page_state', 'task_semantics'],
								payload: {
									targetNames: ['Service search'],
									actionTypes: ['fill'],
									stepTargets: [
										{
											actionType: 'fill',
											targetName: 'Service search',
											valueTemplate: '{{service_name}}',
											pageStateId: 'page_service_list',
											surfaceId: 'surface_filter_drawer',
										},
									],
								},
							},
						],
					}),
					{ status: 200, headers: { 'content-type': 'application/json' } }
				)
			}
			if (path.endsWith('/api/memory/task-runs')) {
				return new Response(JSON.stringify({ taskRunId: 'task_run_1' }), {
					status: 201,
					headers: { 'content-type': 'application/json' },
				})
			}
			return new Response(JSON.stringify({ pageStateId: 'page_service_list' }), {
				status: 200,
				headers: { 'content-type': 'application/json' },
			})
		})
		const agent = new MultiPageAgent({
			workflowBackend: {
				baseUrl: 'https://memory.example.test',
				projectId: 'default',
			},
		} as never)
		const coreConfig = mocks.coreConfigs[0]

		await coreConfig.onBeforeTask({
			task: '查看 service 状态',
			taskId: 'task_1',
			pushObservation: vi.fn(),
		})
		await coreConfig.onAfterTask(
			{ history: [] },
			{ success: true, data: '搜索结果已展示', history: [] }
		)

		const taskRunCall = fetchCalls.find((call) =>
			requestURLToString(call.url).endsWith('/api/memory/task-runs')
		)
		expect(JSON.parse(taskRunCall?.init?.body as string)).toMatchObject({
			memoryContextId: 'ctx_debug_1',
			memoryEvidenceRefs: [
				{
					source: 'experience',
					id: 'exp_service_search',
					reason: 'Matched a reusable successful experience for this task.',
					matchedRules: ['start_page_state', 'task_semantics'],
					payload: {
						targetNames: ['Service search'],
						stepTargets: [
							{
								targetName: 'Service search',
								surfaceId: 'surface_filter_drawer',
							},
						],
					},
				},
			],
		})
		expect(storageSet).toHaveBeenCalledWith({
			lastWebOpsSession: expect.objectContaining({
				memoryContext: expect.objectContaining({
					contextId: 'ctx_debug_1',
					contextPrompt: '<webops_memory>Use the service search box.</webops_memory>',
					currentSurface: expect.objectContaining({ id: 'surface_filter_drawer' }),
					debug: expect.objectContaining({
						promptChars: 64,
						filteredEvidence: [
							{ source: 'knowledge', id: 'chunk_wrong', reason: 'hard_gate_failed' },
						],
					}),
					evidenceRefs: [
						expect.objectContaining({
							id: 'exp_service_search',
							payload: expect.objectContaining({
								targetNames: ['Service search'],
							}),
						}),
					],
				}),
			}),
		})
	})

	it('skips memory task-run upload when no workflow backend is configured', async () => {
		const fetchMock = vi.fn(async () => new Response('{}', { status: 200 }))
		vi.stubGlobal('fetch', fetchMock)
		const agent = new MultiPageAgent({} as never)
		const coreConfig = mocks.coreConfigs[0]

		await coreConfig.onBeforeTask({
			task: '查看服务状态',
			taskId: 'task_1',
			pushObservation: vi.fn(),
		})
		await coreConfig.onAfterTask(
			{ history: [] },
			{ success: true, data: '服务状态是 running', history: [] }
		)

		expect(fetchMock).not.toHaveBeenCalled()
	})
})

function requestURLToString(url: string | URL | Request): string {
	if (typeof url === 'string') return url
	if (url instanceof URL) return url.href
	return url.url
}
