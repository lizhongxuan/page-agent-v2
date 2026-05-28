import type { WorkflowTarget } from './types'

export interface ResolvableElement {
	id: string | number
	testId?: string
	role?: string
	name?: string
	label?: string
	placeholder?: string
	text?: string
	css?: string
	xpath?: string
	nearText?: string
	visible?: boolean
	enabled?: boolean
	covered?: boolean
}

export interface TargetResolution {
	matched: boolean
	element?: ResolvableElement
	confidence: number
	strategy?: string
	reason: string
}

export function resolveWorkflowTarget(
	target: WorkflowTarget,
	elements: ResolvableElement[]
): TargetResolution {
	const candidates = elements
		.map((element) => ({ element, score: scoreElement(target, element) }))
		.filter((candidate) => candidate.score > 0)
		.sort((left, right) => right.score - left.score)

	if (candidates.length === 0) {
		return { matched: false, confidence: 0, reason: 'No target matched.' }
	}

	const bestScore = candidates[0].score
	const tied = candidates.filter((candidate) => candidate.score === bestScore)
	if (tied.length > 1 && bestScore < 0.9) {
		return {
			matched: false,
			confidence: bestScore,
			reason: 'Found multiple low-confidence matches.',
		}
	}

	const best = candidates[0].element
	if (!isActionable(best)) {
		return {
			matched: false,
			element: best,
			confidence: bestScore,
			reason: 'Matched target is not actionable.',
		}
	}

	return {
		matched: true,
		element: best,
		confidence: bestScore,
		strategy: bestStrategy(target),
		reason: 'Target matched.',
	}
}

function scoreElement(target: WorkflowTarget, element: ResolvableElement): number {
	if (target.testId && equals(target.testId, element.testId)) return 1
	if (
		target.role &&
		target.name &&
		equals(target.role, element.role) &&
		equals(target.name, element.name)
	) {
		return 0.95
	}
	if (target.label && equals(target.label, element.label)) return 0.88
	if (target.placeholder && equals(target.placeholder, element.placeholder)) return 0.84
	if (target.text && equals(target.text, element.text)) return 0.7
	if (target.css && target.css === element.css) return 0.65
	if (target.xpath && target.xpath === element.xpath) return 0.6
	return 0
}

function isActionable(element: ResolvableElement): boolean {
	return element.visible !== false && element.enabled !== false && element.covered !== true
}

function equals(left?: string, right?: string): boolean {
	return normalize(left) === normalize(right)
}

function normalize(value?: string): string {
	return (value ?? '').trim().toLowerCase()
}

function bestStrategy(target: WorkflowTarget): string {
	if (target.testId) return 'testId'
	if (target.role && target.name) return 'role+name'
	if (target.label) return 'label'
	if (target.placeholder) return 'placeholder'
	if (target.text) return 'text'
	if (target.css) return 'css'
	if (target.xpath) return 'xpath'
	return 'unknown'
}
