import { describe, expect, it } from 'vitest'

import { defaultRiskPolicy, evaluateRisk } from './riskPolicy'

describe('riskPolicy', () => {
	it('allows read-only and search risks automatically', () => {
		expect(evaluateRisk('read_only', defaultRiskPolicy)).toEqual({ decision: 'allow' })
		expect(evaluateRisk('read_or_search', defaultRiskPolicy)).toEqual({ decision: 'allow' })
	})

	it('requires confirmation for draft changes and external sends', () => {
		expect(evaluateRisk('draft_change', defaultRiskPolicy)).toEqual({
			decision: 'confirm',
			reason: 'risk_requires_confirmation',
		})
		expect(evaluateRisk('external_send', defaultRiskPolicy)).toEqual({
			decision: 'confirm',
			reason: 'risk_requires_confirmation',
		})
	})

	it('blocks destructive workflows', () => {
		expect(evaluateRisk('destructive', defaultRiskPolicy)).toEqual({
			decision: 'block',
			reason: 'risk_blocked',
		})
	})
})
