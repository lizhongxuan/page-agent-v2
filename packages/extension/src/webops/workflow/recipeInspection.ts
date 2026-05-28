import type {
	WorkflowRecipe,
	WorkflowRecipeStep,
	WorkflowStepTarget,
	WorkflowTargetCandidate,
} from './types'

export interface WorkflowInspection {
	stepCount: number
	variableNames: string[]
	chunks: WorkflowInspectionChunk[]
}

export interface WorkflowInspectionChunk {
	id: string
	name: string
	pageState: string
	steps: WorkflowInspectionStep[]
}

export interface WorkflowInspectionStep {
	id: string
	index: number
	action: string
	target: string
	input?: string
	riskLevel: string
}

export function inspectWorkflowRecipe(recipe: WorkflowRecipe): WorkflowInspection {
	let stepIndex = 0
	const variableNames = uniqueStrings([
		...recipe.chunks.flatMap((chunk) => chunk.variableNames ?? []),
		...recipe.variables.map((variable) => variable.name),
	])
	const chunks = recipe.chunks.map((chunk) => {
		const steps = chunk.steps.map((step) => {
			stepIndex += 1
			return inspectWorkflowStep(step, stepIndex)
		})
		return {
			id: chunk.id,
			name: chunk.name,
			pageState: formatPageState(chunk.fromPageState, chunk.toPageState),
			steps,
		}
	})

	return {
		stepCount: stepIndex,
		variableNames,
		chunks,
	}
}

function inspectWorkflowStep(step: WorkflowRecipeStep, index: number): WorkflowInspectionStep {
	const input = formatStepInput(step)
	return {
		id: step.id,
		index,
		action: step.type,
		target: formatStepTarget(step.target),
		...(input ? { input } : {}),
		riskLevel: step.riskLevel,
	}
}

function formatStepInput(step: WorkflowRecipeStep): string | undefined {
	if (step.type === 'press') return step.key
	if (step.type === 'fill' || step.type === 'select' || step.type === 'wait') return step.value
	return undefined
}

function formatStepTarget(target: WorkflowStepTarget | undefined): string {
	if (!target) return 'page'
	const fallbackCount = target.fallbacks?.length ?? 0
	const fallbackLabel = fallbackCount > 0 ? ` (+${fallbackCount} fallback)` : ''
	return `${formatTargetCandidate(target.primary)}${fallbackLabel}`
}

function formatTargetCandidate(candidate: WorkflowTargetCandidate): string {
	switch (candidate.strategy) {
		case 'role':
			return compactJoin([
				'role',
				candidate.role,
				candidate.name ? quote(candidate.name) : undefined,
			])
		case 'label':
			return compactJoin(['label', quote(candidate.value ?? candidate.name ?? '')])
		case 'placeholder':
			return compactJoin(['placeholder', quote(candidate.value ?? candidate.name ?? '')])
		case 'test_id':
			return compactJoin(['test_id', quote(candidate.value ?? '')])
		case 'text':
			return compactJoin(['text', quote(candidate.value ?? candidate.name ?? '')])
		case 'css':
			return compactJoin(['css', candidate.value])
		case 'xpath':
			return compactJoin(['xpath', candidate.value])
		default:
			return candidate.value ?? candidate.name ?? candidate.strategy
	}
}

function formatPageState(
	fromPageState: string | undefined,
	toPageState: string | undefined
): string {
	if (fromPageState && toPageState) return `${fromPageState} -> ${toPageState}`
	return fromPageState ?? toPageState ?? 'recorded page'
}

function uniqueStrings(values: string[]): string[] {
	return [...new Set(values.filter(Boolean))]
}

function compactJoin(parts: (string | undefined)[]): string {
	return parts.filter((part): part is string => Boolean(part)).join(' ')
}

function quote(value: string): string {
	return `"${value}"`
}
