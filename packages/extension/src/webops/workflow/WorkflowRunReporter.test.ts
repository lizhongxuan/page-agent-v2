import { beforeEach, describe, expect, it, vi } from 'vitest'

import { WorkflowRunReporter } from './WorkflowRunReporter'

describe('WorkflowRunReporter', () => {
	beforeEach(() => {
		vi.restoreAllMocks()
	})

	it('posts workflow run reports with bearer auth and preserves run fields', async () => {
		const fetchMock = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => ({ ok: true, id: 'run-1' }),
		})
		const reporter = new WorkflowRunReporter({
			baseUrl: 'https://workflow.example.test/',
			apiKey: 'api-key-1',
			fetch: fetchMock,
		})
		const report = {
			projectId: 'project-1',
			workflowId: 'workflow-1',
			workflowVersion: 2,
			task: 'Find a service',
			url: 'https://console.example.test/services',
			status: 'fallback' as const,
			startedAt: '2026-05-27T00:00:00.000Z',
			finishedAt: '2026-05-27T00:00:04.000Z',
			executedChunkIds: ['chunk-1'],
			variables: {
				serviceName: 'billing',
			},
			artifactRefs: [
				{
					type: 'dom_snapshot',
					id: 'artifact-1',
				},
			],
			fallbackReason: 'selector_not_found',
		}

		const result = await reporter.reportRun(report)

		expect(result).toEqual({ ok: true, id: 'run-1' })
		expect(fetchMock).toHaveBeenCalledWith('https://workflow.example.test/api/workflow-runs', {
			method: 'POST',
			headers: {
				Authorization: 'Bearer api-key-1',
				'Content-Type': 'application/json',
			},
			body: JSON.stringify(report),
		})
	})

	it('returns a closed failure when workflow run reporting has a network error', async () => {
		const reporter = new WorkflowRunReporter({
			baseUrl: 'https://workflow.example.test',
			apiKey: 'api-key-1',
			fetch: vi.fn().mockRejectedValue(new Error('offline')),
		})

		await expect(reporter.reportRun({ projectId: 'project-1' })).resolves.toEqual({
			ok: false,
			error: 'offline',
		})
	})

	it('posts selector stats to the run-scoped endpoint', async () => {
		const fetchMock = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => ({ ok: true }),
		})
		const reporter = new WorkflowRunReporter({
			baseUrl: 'https://workflow.example.test',
			apiKey: 'api-key-1',
			fetch: fetchMock,
		})
		const stats = {
			projectId: 'project-1',
			workflowId: 'workflow-1',
			stepId: 'step-1',
			target: {
				role: 'button',
				name: 'Search',
			},
			strategy: 'role_name',
			success: false,
			fallbackReason: 'ambiguous_match',
		}

		const result = await reporter.reportSelectorStats('run-1', stats)

		expect(result).toEqual({ ok: true })
		expect(fetchMock).toHaveBeenCalledWith(
			'https://workflow.example.test/api/workflow-runs/run-1/selector-stats',
			{
				method: 'POST',
				headers: {
					Authorization: 'Bearer api-key-1',
					'Content-Type': 'application/json',
				},
				body: JSON.stringify(stats),
			}
		)
	})
})
