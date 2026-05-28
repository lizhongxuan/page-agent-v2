import { describe, expect, it } from 'vitest'

import type { WorkflowVariable } from './types'
import { bindWorkflowVariables } from './variableBinding'

describe('bindWorkflowVariables', () => {
	it('binds obvious non-sensitive values from task slots', () => {
		const result = bindWorkflowVariables({
			task: 'Search service kme in prod',
			taskSlots: { serviceName: 'kme' },
			variables: [
				{
					name: 'serviceName',
					description: 'Service name',
					required: true,
					sensitive: false,
					policy: 'ask_if_missing',
				},
			],
		})

		expect(result.status).toBe('ready')
		expect(result.bindings.serviceName?.value).toBe('kme')
		expect(result.bindings.serviceName?.confidence).toBe(1)
	})

	it('returns ask when a required variable is missing', () => {
		const result = bindWorkflowVariables({
			task: 'Search service',
			variables: [
				{
					name: 'serviceName',
					required: true,
					sensitive: false,
					policy: 'ask_if_missing',
				},
			],
		})

		expect(result.status).toBe('ask')
		expect(result.missingVariables).toEqual(['serviceName'])
	})

	it('returns handover for sensitive variables and does not bind their values', () => {
		const variables: WorkflowVariable[] = [
			{
				name: 'password',
				required: true,
				sensitive: true,
				policy: 'handover_only',
			},
		]

		const result = bindWorkflowVariables({
			task: 'Login with password hunter2',
			taskSlots: { password: 'hunter2' },
			variables,
		})

		expect(result.status).toBe('handover')
		expect(result.bindings.password).toBeUndefined()
		expect(result.handoverVariables).toEqual(['password'])
	})
})
