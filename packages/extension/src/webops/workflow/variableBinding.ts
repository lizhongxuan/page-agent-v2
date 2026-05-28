import type { VariableBinding, VariableBindingResult, WorkflowVariable } from './types'

export interface VariableBindingInput {
	task: string
	taskSlots?: Record<string, string | undefined>
	variables: WorkflowVariable[]
}

export function bindWorkflowVariables(input: VariableBindingInput): VariableBindingResult {
	const bindings: Record<string, VariableBinding> = {}
	const missingVariables: string[] = []
	const ambiguousVariables: string[] = []
	const handoverVariables: string[] = []

	for (const variable of input.variables) {
		if (variable.sensitive || variable.policy === 'handover_only') {
			handoverVariables.push(variable.name)
			continue
		}

		const slotValue = input.taskSlots?.[variable.name]
		if (slotValue) {
			bindings[variable.name] = {
				value: slotValue,
				confidence: 1,
				source: 'task_slot',
				evidence: variable.name,
			}
			continue
		}

		if (variable.defaultValue && variable.policy === 'auto') {
			bindings[variable.name] = {
				value: variable.defaultValue,
				confidence: 0.8,
				source: 'default',
				evidence: 'defaultValue',
			}
			continue
		}

		if (variable.required) {
			missingVariables.push(variable.name)
		}
	}

	for (const variable of input.variables) {
		if (variable.policy === 'always_confirm' && bindings[variable.name]) {
			ambiguousVariables.push(variable.name)
		}
	}

	return {
		status: getStatus({ handoverVariables, missingVariables, ambiguousVariables }),
		bindings,
		missingVariables,
		ambiguousVariables,
		handoverVariables,
	}
}

function getStatus(input: {
	handoverVariables: string[]
	missingVariables: string[]
	ambiguousVariables: string[]
}): VariableBindingResult['status'] {
	if (input.handoverVariables.length > 0) return 'handover'
	if (input.missingVariables.length > 0) return 'ask'
	if (input.ambiguousVariables.length > 0) return 'confirm'
	return 'ready'
}
