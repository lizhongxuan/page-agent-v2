import { beforeEach, describe, expect, it, vi } from 'vitest'

import { WorkflowClient } from './WorkflowClient'
import type { WorkflowSearchRequest, WorkflowSearchResult } from './types'

const searchRequest: WorkflowSearchRequest = {
	projectId: 'project-1',
	task: 'Find service',
	url: 'https://console.example.test/services',
	domain: 'console.example.test',
	pageFingerprint: {
		requiredText: ['Services'],
	},
	limit: 3,
}

const searchResult: WorkflowSearchResult = {
	recipe: {
		id: 'recipe-1',
		projectId: 'project-1',
		site: 'console.example.test',
		name: 'Find service',
		intent: 'Find a service by name',
		urlPatterns: ['https://console.example.test/*'],
		pageFingerprint: {
			requiredText: ['Services'],
		},
		variables: [],
		chunks: [],
		safetyPolicy: {
			auto: ['read_only'],
			confirm: ['state_change'],
			handover: ['login_secret'],
			blocked: ['delete'],
		},
		status: 'active',
		version: 1,
		createdAt: '2026-05-27T00:00:00.000Z',
		updatedAt: '2026-05-27T00:00:00.000Z',
	},
	score: 0.91,
	reasons: ['semantic match'],
}

describe('WorkflowClient', () => {
	beforeEach(() => {
		vi.restoreAllMocks()
	})

	it('posts workflow search requests with bearer auth and returns workflows', async () => {
		const fetchMock = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => ({ workflows: [searchResult] }),
		})
		const client = new WorkflowClient({
			baseUrl: 'https://workflow.example.test',
			apiKey: 'api-key-1',
			fetch: fetchMock,
		})

		const workflows = await client.searchWorkflows(searchRequest)

		expect(workflows[0]).toMatchObject(searchResult)
		expect(fetchMock).toHaveBeenCalledWith('https://workflow.example.test/api/workflows/search', {
			method: 'POST',
			headers: {
				Authorization: 'Bearer api-key-1',
				'Content-Type': 'application/json',
			},
			body: JSON.stringify(searchRequest),
		})
	})

	it('returns an empty list when workflow search returns a non-2xx response', async () => {
		const client = new WorkflowClient({
			baseUrl: 'https://workflow.example.test',
			apiKey: 'api-key-1',
			fetch: vi.fn().mockResolvedValue({ ok: false, status: 500 }),
		})

		await expect(client.searchWorkflows(searchRequest)).resolves.toEqual([])
	})

	it('returns an empty list when workflow search has a network error', async () => {
		const client = new WorkflowClient({
			baseUrl: 'https://workflow.example.test',
			apiKey: 'api-key-1',
			fetch: vi.fn().mockRejectedValue(new Error('offline')),
		})

		await expect(client.searchWorkflows(searchRequest)).resolves.toEqual([])
	})

	it('normalizes backend workflow recipes for the extension executor', async () => {
		const fetchMock = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => ({
				workflows: [
					{
						score: 0.9,
						reasons: ['match'],
						recipe: {
							...searchResult.recipe,
							chunks: [
								{
									id: 'chunk-1',
									name: 'Chunk',
									riskLevel: 'submit_search',
									steps: [
										{
											id: 'input-query',
											type: 'input',
											value: '{{query}}',
											target: {
												preferred: {
													strategy: 'role',
													role: 'textbox',
													name: 'Search',
												},
											},
										},
									],
								},
							],
							variables: [
								{
									name: 'query',
									required: true,
									sensitive: false,
									bindingMode: 'ask_if_missing',
								},
							],
							safetyPolicy: {
								allowedRiskLevels: ['submit_search'],
								confirmationRiskLevels: [],
								handoverRiskLevels: [],
								blockedRiskLevels: [],
							},
						},
					},
				],
			}),
		})
		const client = new WorkflowClient({
			baseUrl: 'https://workflow.example.test',
			apiKey: 'api-key-1',
			fetch: fetchMock,
		})

		const [workflow] = await client.searchWorkflows(searchRequest)

		expect(workflow.recipe.chunks[0]).toMatchObject({ risk: 'submit_search' })
		expect(workflow.recipe.chunks[0]?.steps[0]).toMatchObject({
			type: 'input',
			valueVariable: 'query',
			target: { role: 'textbox', name: 'Search' },
		})
		expect(workflow.recipe.variables[0]).toMatchObject({ policy: 'ask_if_missing' })
		expect(workflow.recipe.safetyPolicy.auto).toEqual(['submit_search'])
	})
})
