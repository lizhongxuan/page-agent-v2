import { describe, expect, it } from 'vitest'

import { SessionRecorder } from './SessionRecorder'

describe('SessionRecorder', () => {
	it('records input action with redacted sensitive values and report', () => {
		const recorder = new SessionRecorder()
		recorder.start({ id: 's1', task: '登录', startUrl: 'https://example.test' })
		recorder.record({
			id: 'a1',
			type: 'input',
			timestamp: 1,
			pageUrl: 'https://example.test',
			pageTitle: 'Login',
			target: { name: 'password' },
			value: 'secret123',
			result: 'success',
		})

		const session = recorder.finish()
		expect(session.steps[0].value).toBe('[REDACTED]')
		expect(session.redactionReport).toEqual([{ actionId: 'a1', field: 'value' }])
	})

	it('tracks knowledge hits and finish timestamp', () => {
		const recorder = new SessionRecorder()
		recorder.start({ id: 's1', task: '排障', startUrl: 'https://example.test' })
		recorder.addKnowledgeHits([{ id: 'k1', title: 'Runbook', source: 'local', score: 0.9 }])

		const session = recorder.finish()
		expect(session.knowledgeHits).toEqual([
			{ id: 'k1', title: 'Runbook', source: 'local', score: 0.9 },
		])
		expect(session.endedAt).toEqual(expect.any(Number))
	})

	it('records all supported action types', () => {
		const recorder = new SessionRecorder()
		recorder.start({ id: 's1', task: '复放', startUrl: 'https://example.test' })

		const types = [
			'observe',
			'click',
			'input',
			'select',
			'wait',
			'navigate',
			'extract',
			'handover',
		] as const
		for (const [index, type] of types.entries()) {
			recorder.record({
				id: `a${index}`,
				type,
				timestamp: index,
				pageUrl: 'https://example.test',
				pageTitle: 'Example',
				result: type === 'handover' ? 'skipped' : 'success',
				note: type,
			})
		}

		const session = recorder.finish()
		expect(session.steps.map((step) => step.type)).toEqual(types)
	})

	it('throws when finishing before start', () => {
		const recorder = new SessionRecorder()
		expect(() => recorder.finish()).toThrow('SessionRecorder has not started')
	})
})
