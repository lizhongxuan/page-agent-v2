import { describe, expect, it } from 'vitest'

import { MemoryContextProvider } from './MemoryContextProvider'
import type { MemoryClientLike } from './types'

describe('MemoryContextProvider', () => {
	it('returns empty prompt context and evidence without a client', async () => {
		const provider = new MemoryContextProvider(undefined)

		const result = await provider.getContext({
			projectId: 'default',
			task: '查看服务',
			currentUrl: 'https://ops.example.test/service',
			title: '服务管理',
			pageObservation: { title: '服务管理', visibleText: [], controls: [] },
		})

		expect(result).toEqual({ promptContext: '', evidence: [] })
	})

	it('injects contextPrompt unchanged and exposes evidence metadata', async () => {
		const client: MemoryClientLike = {
			getContext: async () => ({
				contextPrompt: '<webops_memory>Use service search.</webops_memory>',
				evidenceRefs: [{ id: 'ev-1', source: 'experience', title: 'Previous run', score: 0.91 }],
			}),
		}
		const provider = new MemoryContextProvider(client)

		const result = await provider.getContext({
			projectId: 'default',
			task: '查看服务',
			currentUrl: 'https://ops.example.test/service',
			title: '服务管理',
			pageObservation: { title: '服务管理', visibleText: [], controls: [] },
		})

		expect(result.promptContext).toBe('<webops_memory>Use service search.</webops_memory>')
		expect(result.evidence).toEqual([
			{ id: 'ev-1', source: 'experience', title: 'Previous run', score: 0.91 },
		])
	})

	it('prefers backend evidenceRefs without rebuilding evidence from legacy fields', async () => {
		const client: MemoryClientLike = {
			getContext: async () => ({
				contextId: 'ctx_1',
				contextPrompt: '<webops_memory>Use service search.</webops_memory>',
				evidenceRefs: [
					{ source: 'knowledge', id: 'chunk_1', rank: 1, score: 0.97 },
					{ source: 'experience', id: 'exp_2', rank: 2, score: 0.88 },
				],
				knowledgeEvidence: [{ chunkId: 'legacy_chunk', title: 'Legacy chunk', score: 0.4 }],
			}),
		}
		const provider = new MemoryContextProvider(client)

		const result = await provider.getContext({
			projectId: 'default',
			task: '查看服务',
			currentUrl: 'https://ops.example.test/service',
			title: '服务管理',
			pageObservation: { title: '服务管理', visibleText: [], controls: [] },
		})

		expect(result.response?.contextId).toBe('ctx_1')
		expect(result.evidence).toEqual([
			{ source: 'knowledge', id: 'chunk_1', rank: 1, score: 0.97 },
			{ source: 'experience', id: 'exp_2', rank: 2, score: 0.88 },
		])
	})

	it('returns an empty prompt when the memory response has no contextPrompt', async () => {
		const client: MemoryClientLike = {
			getContext: async () => ({
				evidenceRefs: [{ id: 'ev-1', source: 'experience', title: 'Previous run' }],
			}),
		}
		const provider = new MemoryContextProvider(client)

		const result = await provider.getContext({
			projectId: 'default',
			task: '查看服务',
			currentUrl: 'https://ops.example.test/service',
			title: '服务管理',
			pageObservation: { title: '服务管理', visibleText: [], controls: [] },
		})

		expect(result.promptContext).toBe('')
		expect(result.evidence).toEqual([{ id: 'ev-1', source: 'experience', title: 'Previous run' }])
	})

	it('does not synthesize compatibility evidence when backend evidenceRefs are missing', async () => {
		const client: MemoryClientLike = {
			getContext: async () => ({
				knowledgeEvidence: [{ chunkId: 'chunk_1', title: 'Knowledge item', score: 0.9 }],
				experienceHints: [{ id: 'exp_1', summary: 'Use search first', confidence: 0.8 }],
				failureWarnings: [{ id: 'fail_1', summary: 'Avoid archived services' }],
			}),
		}
		const provider = new MemoryContextProvider(client)

		const result = await provider.getContext({
			projectId: 'default',
			task: '查看服务',
			currentUrl: 'https://ops.example.test/service',
			title: '服务管理',
			pageObservation: { title: '服务管理', visibleText: [], controls: [] },
		})

		expect(result.evidence).toEqual([])
	})
})
