import { describe, expect, it } from 'vitest'

import { redactSensitiveValue } from './redaction'

describe('redactSensitiveValue', () => {
	it('redacts sensitive field names', () => {
		expect(redactSensitiveValue('password', 'secret123')).toBe('[REDACTED]')
		expect(redactSensitiveValue('apiKey', 'sk-1234567890abcdef')).toBe('[REDACTED]')
		expect(redactSensitiveValue('api key', 'plain-secret')).toBe('[REDACTED]')
		expect(redactSensitiveValue('captcha', '849201')).toBe('[REDACTED]')
		expect(redactSensitiveValue('mfaCode', '123456')).toBe('[REDACTED]')
		expect(redactSensitiveValue('验证码', '123456')).toBe('[REDACTED]')
	})

	it('redacts token-like values even when the field name looks safe', () => {
		expect(redactSensitiveValue('notes', 'Bearer abcdefghijklmnopqrstuvwxyz123456')).toBe(
			'[REDACTED]'
		)
		expect(redactSensitiveValue('search', 'sk-1234567890abcdef')).toBe('[REDACTED]')
		expect(redactSensitiveValue('session', 'a'.repeat(32))).toBe('[REDACTED]')
	})

	it('keeps normal search values', () => {
		expect(redactSensitiveValue('search', 'kme')).toBe('kme')
	})
})
