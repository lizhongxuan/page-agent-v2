import { describe, expect, it } from 'vitest'

import { normalizeToolOutput, shouldStopBatchForOutput } from './toolOutput'

describe('toolOutput', () => {
	it('turns missing tool output into an actionable warning string', () => {
		expect(normalizeToolOutput(undefined, 'scroll')).toBe(
			'⚠️ Tool scroll completed without a text result.'
		)
		expect(normalizeToolOutput(null, 'scroll')).toBe(
			'⚠️ Tool scroll completed without a text result.'
		)
	})

	it('does not throw while checking non-string batch outputs', () => {
		expect(shouldStopBatchForOutput(undefined)).toBe(true)
		expect(shouldStopBatchForOutput('✅ ok')).toBe(false)
		expect(shouldStopBatchForOutput('❌ failed')).toBe(true)
		expect(shouldStopBatchForOutput('⚠️ warning')).toBe(true)
	})

	it('serializes object outputs without default object stringification', () => {
		expect(normalizeToolOutput({ success: true, message: 'ok' }, 'custom')).toBe(
			'{\n  "success": true,\n  "message": "ok"\n}'
		)
	})
})
