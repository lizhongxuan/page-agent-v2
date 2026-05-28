import { describe, expect, it, vi } from 'vitest'

import { WorkflowCandidateClient } from './WorkflowCandidateClient'

describe('WorkflowCandidateClient', () => {
	it('approves a pending candidate', async () => {
		const fetchMock = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => ({ id: 'cand-1', searchable: true, reviewStatus: 'approved' }),
		})
		const client = new WorkflowCandidateClient({
			baseUrl: 'https://workflow.example.test/',
			apiKey: 'key',
			fetch: fetchMock,
		})

		const result = await client.approve('cand-1')

		expect(result).toEqual({
			ok: true,
			candidate: { id: 'cand-1', searchable: true, reviewStatus: 'approved' },
		})
		expect(fetchMock).toHaveBeenCalledWith(
			'https://workflow.example.test/api/workflow-candidates/cand-1/approve',
			{
				method: 'POST',
				headers: { Authorization: 'Bearer key' },
			}
		)
	})
})
