import { describe, expect, it } from 'vitest'

import type { RecordedSession } from '../webops/recorder/actionEvents'
import { buildPlaywrightExport } from './history-export'

describe('buildPlaywrightExport', () => {
	it('builds a zip archive for a runnable Playwright project', () => {
		const exportResult = buildPlaywrightExport(
			'查看 kme 服务状态',
			new Date('2026-05-26T01:02:03').getTime(),
			session()
		)

		expect(exportResult.filename).toMatch(/^page-agent-v2-replay-.*\.zip$/)
		expect(exportResult.blob.type).toBe('application/zip')
	})
})

function session(): RecordedSession {
	return {
		id: 's1',
		task: '查看 kme 服务状态',
		startUrl: 'https://example.test/service',
		startedAt: 1,
		steps: [],
		knowledgeHits: [],
		redactionReport: [],
	}
}
