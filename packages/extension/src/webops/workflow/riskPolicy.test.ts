import { describe, expect, it } from 'vitest'

import { evaluateChunkRisk } from './riskPolicy'
import type { RiskPolicy, WorkflowChunk } from './types'

const policy: RiskPolicy = {
	auto: ['read_only', 'form_fill', 'submit_search'],
	confirm: ['state_change', 'production_change'],
	handover: ['login_secret', 'captcha', 'mfa'],
	blocked: ['delete', 'payment', 'permission_change'],
}

const chunk = (risk: WorkflowChunk['risk']): WorkflowChunk => ({
	id: `chunk-${risk}`,
	name: risk,
	risk,
	steps: [],
})

describe('evaluateChunkRisk', () => {
	it('allows auto execution for safe chunks', () => {
		expect(evaluateChunkRisk(chunk('submit_search'), policy).decision).toBe('auto')
	})

	it('requires confirmation for configured risky chunks', () => {
		expect(evaluateChunkRisk(chunk('production_change'), policy).decision).toBe('confirm')
	})

	it('requests handover for login secrets and MFA', () => {
		expect(evaluateChunkRisk(chunk('login_secret'), policy).decision).toBe('handover')
		expect(evaluateChunkRisk(chunk('mfa'), policy).decision).toBe('handover')
	})

	it('blocks configured destructive chunks', () => {
		const result = evaluateChunkRisk(chunk('delete'), policy)

		expect(result.decision).toBe('blocked')
		expect(result.reason).toContain('delete')
	})
})
