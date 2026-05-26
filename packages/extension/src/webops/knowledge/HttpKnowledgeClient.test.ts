import { afterEach, describe, expect, it, vi } from 'vitest'

import { HttpKnowledgeClient } from './HttpKnowledgeClient'

const originalFetch = globalThis.fetch

describe('HttpKnowledgeClient', () => {
	afterEach(() => {
		globalThis.fetch = originalFetch
	})

	it('posts search request to configured endpoint with bearer auth', async () => {
		const calls: { url: string | URL | Request; init?: RequestInit }[] = []
		vi.stubGlobal('fetch', async (url: string | URL | Request, init?: RequestInit) => {
			calls.push({ url, init })
			return new Response(
				JSON.stringify({
					hits: [
						{
							id: '1',
							title: '服务管理',
							source: 'kb',
							snippet: '使用搜索框查询服务。',
							score: 0.9,
						},
						{
							id: '2',
							title: '服务详情',
							source: 'kb',
							snippet: '详情页包含环境信息。',
							score: 0.8,
						},
					],
				}),
				{ status: 200, headers: { 'content-type': 'application/json' } }
			)
		})

		const client = new HttpKnowledgeClient({
			baseUrl: 'https://kb.example.test/',
			apiKey: 'secret',
		})
		const hits = await client.search({
			task: '查看 kme',
			url: 'https://app/service',
			title: '服务管理',
			limit: 1,
		})

		expect(calls).toHaveLength(1)
		expect(calls[0]?.url).toBe('https://kb.example.test/search')
		expect(calls[0]?.init?.method).toBe('POST')
		expect((calls[0]?.init?.headers as Record<string, string>)['content-type']).toBe(
			'application/json'
		)
		expect((calls[0]?.init?.headers as Record<string, string>).Authorization).toBe('Bearer secret')
		expect(JSON.parse(calls[0]?.init?.body as string).task).toBe('查看 kme')
		expect(hits.map((hit) => hit.id)).toEqual(['1'])
	})

	it('returns an empty list when the request fails', async () => {
		vi.stubGlobal('fetch', async () => new Response('server error', { status: 500 }))

		const client = new HttpKnowledgeClient({ baseUrl: 'https://kb.example.test', apiKey: 'secret' })
		const hits = await client.search({
			task: '查看 kme',
			url: 'https://app/service',
			title: '服务管理',
			limit: 3,
		})

		expect(hits).toEqual([])
	})
})
