import { type ResolvableElement, resolveWorkflowTarget } from './TargetResolver'
import { evaluateChunkRisk } from './riskPolicy'
import type { RiskPolicy, WorkflowChunk, WorkflowStep } from './types'

export interface ChunkActions {
	click: (element: ResolvableElement) => Promise<void>
	input: (element: ResolvableElement, value: string) => Promise<void>
	select?: (element: ResolvableElement, value: string) => Promise<void>
	scroll?: (direction: 'up' | 'down', pages?: number) => Promise<void>
}

export interface ExecuteWorkflowChunkInput {
	chunk: WorkflowChunk
	elements: ResolvableElement[]
	variables: Record<string, string>
	riskPolicy: RiskPolicy
	actions: ChunkActions
}

export type ChunkExecutionResult =
	| { status: 'success'; executedStepIds: string[] }
	| { status: 'blocked' | 'confirm' | 'handover'; reason: string; executedStepIds: string[] }
	| { status: 'failed'; reason: string; failedStepId: string; executedStepIds: string[] }

export async function executeWorkflowChunk(
	input: ExecuteWorkflowChunkInput
): Promise<ChunkExecutionResult> {
	const risk = evaluateChunkRisk(input.chunk, input.riskPolicy)
	if (risk.decision !== 'auto') {
		return { status: risk.decision, reason: risk.reason, executedStepIds: [] }
	}

	const executedStepIds: string[] = []
	for (const step of input.chunk.steps) {
		const result = await executeStep(step, input)
		if (!result.ok) {
			return {
				status: 'failed',
				reason: result.reason,
				failedStepId: step.id,
				executedStepIds,
			}
		}
		executedStepIds.push(step.id)
	}

	return { status: 'success', executedStepIds }
}

async function executeStep(
	step: WorkflowStep,
	input: ExecuteWorkflowChunkInput
): Promise<{ ok: true } | { ok: false; reason: string }> {
	if (step.type === 'observe') return { ok: true }
	if (step.type === 'scroll') {
		await input.actions.scroll?.(step.direction, step.pages)
		return { ok: true }
	}

	const target = resolveWorkflowTarget(step.target, input.elements)
	if (!target.matched || !target.element) {
		return { ok: false, reason: target.reason }
	}

	if (step.type === 'click') {
		await input.actions.click(target.element)
		return { ok: true }
	}

	const value = input.variables[step.valueVariable]
	if (value === undefined) {
		return { ok: false, reason: `Missing variable ${step.valueVariable}.` }
	}
	if (step.type === 'input') {
		await input.actions.input(target.element, value)
		return { ok: true }
	}
	if (step.type === 'select') {
		await input.actions.select?.(target.element, value)
		return { ok: true }
	}

	return { ok: false, reason: `Unsupported step type ${(step as WorkflowStep).type}.` }
}
