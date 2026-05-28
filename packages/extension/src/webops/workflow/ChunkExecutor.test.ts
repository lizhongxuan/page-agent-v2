import { describe, expect, it } from 'vitest'

import { executeWorkflowChunk } from './ChunkExecutor'
import type { WorkflowChunk } from './types'

const elements = [
	{ id: 'searchbox', role: 'textbox', name: 'Search', visible: true, enabled: true },
	{ id: 'submit', role: 'button', name: 'Search', visible: true, enabled: true },
]

describe('executeWorkflowChunk', () => {
	it('executes safe chunk steps in order', async () => {
		const actions: string[] = []
		const chunk: WorkflowChunk = {
			id: 'chunk_search',
			name: 'Search',
			risk: 'submit_search',
			steps: [
				{
					id: 'input',
					type: 'input',
					target: { role: 'textbox', name: 'Search' },
					valueVariable: 'query',
				},
				{ id: 'click', type: 'click', target: { role: 'button', name: 'Search' } },
			],
		}

		const result = await executeWorkflowChunk({
			chunk,
			elements,
			variables: { query: 'page agent' },
			riskPolicy: { auto: ['submit_search'], confirm: [], handover: [], blocked: [] },
			actions: {
				input: async (element, value) => {
					actions.push(`input:${element.id}:${value}`)
				},
				click: async (element) => {
					actions.push(`click:${element.id}`)
				},
			},
		})

		expect(result.status).toBe('success')
		expect(actions).toEqual(['input:searchbox:page agent', 'click:submit'])
	})

	it('blocks chunks with blocked risk before executing steps', async () => {
		const actions: string[] = []
		const result = await executeWorkflowChunk({
			chunk: {
				id: 'delete',
				name: 'Delete',
				risk: 'delete',
				steps: [{ id: 'click', type: 'click', target: { role: 'button', name: 'Search' } }],
			},
			elements,
			variables: {},
			riskPolicy: { auto: ['submit_search'], confirm: [], handover: [], blocked: ['delete'] },
			actions: {
				click: async (element) => {
					actions.push(String(element.id))
				},
				input: async () => {
					actions.push('input')
				},
			},
		})

		expect(result.status).toBe('blocked')
		expect(actions).toEqual([])
	})

	it('stops remaining steps on target failure', async () => {
		const actions: string[] = []
		const result = await executeWorkflowChunk({
			chunk: {
				id: 'chunk',
				name: 'Chunk',
				risk: 'submit_search',
				steps: [
					{ id: 'missing', type: 'click', target: { role: 'button', name: 'Missing' } },
					{ id: 'click', type: 'click', target: { role: 'button', name: 'Search' } },
				],
			},
			elements,
			variables: {},
			riskPolicy: { auto: ['submit_search'], confirm: [], handover: [], blocked: [] },
			actions: {
				click: async (element) => {
					actions.push(String(element.id))
				},
				input: async () => {
					actions.push('input')
				},
			},
		})

		expect(result.status).toBe('failed')
		if (result.status !== 'failed') throw new Error(`expected failed result, got ${result.status}`)
		expect(result.failedStepId).toBe('missing')
		expect(actions).toEqual([])
	})
})
