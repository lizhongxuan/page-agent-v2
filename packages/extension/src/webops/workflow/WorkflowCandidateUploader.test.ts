import { describe, expect, it, vi } from 'vitest'

import type { RecordedSession } from '../recorder/actionEvents'
import { WorkflowCandidateUploader } from './WorkflowCandidateUploader'

const session: RecordedSession = {
	id: 'session-1',
	task: 'Search docs',
	startUrl: 'https://example.com',
	startedAt: 1,
	endedAt: 2,
	steps: [],
	knowledgeHits: [],
	redactionReport: [],
}

describe('WorkflowCandidateUploader', () => {
	it('uploads successful sessions and returns pending candidate', async () => {
		const fetch = vi.fn(
			async () => new Response(JSON.stringify({ id: 'cand-1', searchable: false }), { status: 201 })
		)
		const uploader = new WorkflowCandidateUploader({
			baseUrl: 'https://backend.test',
			apiKey: 'secret',
			fetch,
		})

		const result = await uploader.uploadSuccessfulSession(session)

		expect(result.ok).toBe(true)
		expect(fetch).toHaveBeenCalledWith(
			'https://backend.test/api/workflow-candidates/from-session',
			expect.objectContaining({
				method: 'POST',
				headers: expect.objectContaining({ Authorization: 'Bearer secret' }),
			})
		)
		if (!result.ok) throw new Error('expected ok')
		expect(result.candidate).toMatchObject({ id: 'cand-1', searchable: false })
	})

	it('uploads manual recordings as user_demo candidates', async () => {
		const fetchMock = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => ({ id: 'cand-2', searchable: false }),
		})
		const uploader = new WorkflowCandidateUploader({
			baseUrl: 'https://backend.test',
			fetch: fetchMock,
		})

		const result = await uploader.uploadManualSession(session)

		expect(result).toEqual({ ok: true, candidate: { id: 'cand-2', searchable: false } })
		expect(JSON.parse(fetchMock.mock.calls[0][1].body).source).toBe('user_demo')
	})

	it('does not throw when upload fails', async () => {
		const uploader = new WorkflowCandidateUploader({
			baseUrl: 'https://backend.test',
			fetch: vi.fn(async () => new Response('nope', { status: 500 })),
		})

		const result = await uploader.uploadSuccessfulSession(session)

		expect(result.ok).toBe(false)
		if (result.ok) throw new Error('expected failure')
		expect(result.error).toContain('500')
	})
})
