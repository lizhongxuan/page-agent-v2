import { describe, expect, it } from 'vitest'

import type { KnowledgeClient } from './KnowledgeClient'
import { KnowledgeContextProvider } from './KnowledgeContextProvider'

describe('KnowledgeContextProvider', () => {
	it('returns empty context and hits without a client', async () => {
		const provider = new KnowledgeContextProvider(undefined)
		const result = await provider.getContext({
			task: '查看 kme',
			url: 'https://app/service',
			title: '服务管理',
			limit: 3,
		})

		expect(result).toEqual({ promptContext: '', hits: [] })
	})

	it('returns formatted context and hits with a client', async () => {
		const client: KnowledgeClient = {
			search: async () => [
				{ id: '1', title: '服务管理', source: 'kb', snippet: '按服务名称搜索。', score: 0.9 },
			],
		}
		const provider = new KnowledgeContextProvider(client)
		const result = await provider.getContext({
			task: '查看 kme',
			url: 'https://app/service',
			title: '服务管理',
			projectKey: 'demo',
			visibleText: '服务管理 搜索',
			hints: ['优先查服务名'],
			limit: 3,
		})

		expect(result.promptContext).toMatch(/<project_knowledge>/)
		expect(result.promptContext).toMatch(/按服务名称搜索。/)
		expect(result.hits).toHaveLength(1)
	})
})
