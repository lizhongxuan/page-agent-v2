import { afterEach, describe, expect, it, vi } from 'vitest'

import { MemoryClient } from './MemoryClient'

const originalFetch = globalThis.fetch

describe('MemoryClient', () => {
	afterEach(() => {
		globalThis.fetch = originalFetch
		vi.unstubAllGlobals()
	})

	it('posts page observations to the memory endpoint with trimmed base URL and bearer auth', async () => {
		const calls: { url: string | URL | Request; init?: RequestInit }[] = []
		vi.stubGlobal('fetch', async (url: string | URL | Request, init?: RequestInit) => {
			calls.push({ url, init })
			return jsonResponse({ ok: true, observationId: 'obs-1' })
		})

		const client = new MemoryClient({
			baseUrl: 'https://memory.example.test///',
			bearerToken: 'secret',
		})
		const response = await client.observePage({
			projectId: 'default',
			task: '查看服务',
			url: 'https://ops.example.test/service',
			title: '服务管理',
			visibleText: ['服务列表'],
			controls: [{ role: 'button', name: '搜索' }],
		})

		expect(response).toEqual({ ok: true, observationId: 'obs-1' })
		expect(calls).toHaveLength(1)
		expect(calls[0]?.url).toBe('https://memory.example.test/api/memory/page-observations')
		expect(calls[0]?.init?.method).toBe('POST')
		expect((calls[0]?.init?.headers as Record<string, string>)['content-type']).toBe(
			'application/json'
		)
		expect((calls[0]?.init?.headers as Record<string, string>).Authorization).toBe('Bearer secret')
		expect(JSON.parse(calls[0]?.init?.body as string).title).toBe('服务管理')
	})

	it('posts context requests to the memory endpoint', async () => {
		const calls: { url: string | URL | Request; init?: RequestInit }[] = []
		vi.stubGlobal('fetch', async (url: string | URL | Request, init?: RequestInit) => {
			calls.push({ url, init })
			return jsonResponse({
				contextPrompt: '<webops_memory>Use service search.</webops_memory>',
				evidenceRefs: [{ id: 'ev-1', source: 'experience', title: 'Previous run', score: 0.87 }],
			})
		})

		const client = new MemoryClient({ baseUrl: 'https://memory.example.test' })
		const response = await client.getContext({
			projectId: 'default',
			task: '查看服务',
			currentUrl: 'https://ops.example.test/service',
			title: '服务管理',
			pageObservation: {
				title: '服务管理',
				visibleText: ['服务列表'],
				controls: [{ role: 'button', name: '搜索' }],
				activeTabs: ['运行状态'],
				tables: [{ headers: ['服务名称', '状态', '操作'] }],
				activeSurfaces: [
					{ surfaceType: 'modal', title: '确认', controls: [{ role: 'button', name: '确定' }] },
				],
			},
		})

		expect(calls[0]?.url).toBe('https://memory.example.test/api/memory/context')
		expect(JSON.parse(calls[0]?.init?.body as string).currentUrl).toBe(
			'https://ops.example.test/service'
		)
		expect(JSON.parse(calls[0]?.init?.body as string).pageObservation).toMatchObject({
			activeTabs: ['运行状态'],
			tables: [{ headers: ['服务名称', '状态', '操作'] }],
			activeSurfaces: [{ surfaceType: 'modal', title: '确认' }],
		})
		expect(response.evidenceRefs?.[0]?.id).toBe('ev-1')
	})

	it('posts completed task runs to the memory endpoint', async () => {
		const calls: { url: string | URL | Request; init?: RequestInit }[] = []
		vi.stubGlobal('fetch', async (url: string | URL | Request, init?: RequestInit) => {
			calls.push({ url, init })
			return jsonResponse({ ok: true, runId: 'run-1' })
		})

		const client = new MemoryClient({ baseUrl: 'https://memory.example.test' })
		await client.completeTaskRun({
			id: 'run-1',
			projectId: 'default',
			site: 'ops.example.test',
			taskTemplate: '查看 {{service_name}}',
			summary: '服务运行中',
			originalPath: ['ops.example.test_service'],
			optimizedPath: ['ops.example.test_service'],
			status: 'success',
		})

		expect(calls[0]?.url).toBe('https://memory.example.test/api/memory/task-runs')
		expect(JSON.parse(calls[0]?.init?.body as string).status).toBe('success')
	})

	it('does not expose the old document import endpoint', () => {
		const client = new MemoryClient({ baseUrl: 'https://memory.example.test' })

		expect('importDocuments' in client).toBe(false)
	})
})

function jsonResponse(value: unknown) {
	return new Response(JSON.stringify(value), {
		status: 200,
		headers: { 'content-type': 'application/json' },
	})
}
