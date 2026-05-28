import type { WorkflowSearchRequest, WorkflowSearchResult } from './types'

export interface WorkflowClientOptions {
	baseUrl: string
	apiKey: string
	fetch?: typeof fetch
}

export class WorkflowClient {
	private readonly baseUrl: string
	private readonly apiKey: string
	private readonly fetchFn: typeof fetch

	constructor(options: WorkflowClientOptions) {
		this.baseUrl = options.baseUrl.replace(/\/+$/, '')
		this.apiKey = options.apiKey
		this.fetchFn = options.fetch ?? fetch
	}

	async searchWorkflows(request: WorkflowSearchRequest): Promise<WorkflowSearchResult[]> {
		try {
			const response = await this.fetchFn(`${this.baseUrl}/api/workflows/search`, {
				method: 'POST',
				headers: {
					Authorization: `Bearer ${this.apiKey}`,
					'Content-Type': 'application/json',
				},
				body: JSON.stringify(request),
			})

			if (!response.ok) return []

			const payload = (await response.json()) as { workflows?: WorkflowSearchResult[] }
			return Array.isArray(payload.workflows)
				? payload.workflows.map(normalizeWorkflowSearchResult)
				: []
		} catch {
			return []
		}
	}
}

function normalizeWorkflowSearchResult(result: WorkflowSearchResult): WorkflowSearchResult {
	const recipe = result.recipe as any
	return {
		...result,
		status: result.status ?? recipe.status,
		risk: result.risk ?? recipe.chunks?.[0]?.riskLevel ?? recipe.chunks?.[0]?.risk,
		recipe: {
			...recipe,
			variables: (recipe.variables ?? []).map((variable: any) => ({
				...variable,
				policy: variable.policy ?? variable.bindingMode ?? 'ask_if_missing',
			})),
			chunks: (recipe.chunks ?? []).map((chunk: any) => ({
				...chunk,
				risk: chunk.risk ?? chunk.riskLevel ?? 'read_only',
				steps: (chunk.steps ?? []).map(normalizeWorkflowStep),
			})),
			safetyPolicy: normalizeSafetyPolicy(recipe.safetyPolicy),
		},
	}
}

function normalizeWorkflowStep(step: any) {
	const target = normalizeWorkflowTarget(step.target)
	if (step.type === 'input' || step.type === 'select') {
		return {
			...step,
			target,
			valueVariable: step.valueVariable ?? variableNameFromTemplate(step.value),
		}
	}
	if (step.type === 'scroll') {
		return {
			...step,
			direction: step.direction ?? 'down',
			pages: step.pages ?? 1,
		}
	}
	return { ...step, target }
}

function normalizeWorkflowTarget(target: any) {
	if (!target) return {}
	const preferred = target.preferred ?? target
	const normalized: Record<string, string> = {}
	if (preferred.strategy === 'test_id' || preferred.strategy === 'testId')
		normalized.testId = preferred.value
	if (preferred.strategy === 'role') {
		normalized.role = preferred.role
		normalized.name = preferred.name ?? preferred.value
	}
	if (preferred.strategy === 'label') normalized.label = preferred.value
	if (preferred.strategy === 'placeholder') normalized.placeholder = preferred.value
	if (preferred.strategy === 'text') normalized.text = preferred.value
	if (preferred.strategy === 'css') normalized.css = preferred.value
	if (preferred.strategy === 'xpath') normalized.xpath = preferred.value
	return { ...target, ...normalized }
}

function normalizeSafetyPolicy(policy: any) {
	return {
		auto: policy?.auto ?? policy?.allowedRiskLevels ?? ['read_only', 'form_fill', 'submit_search'],
		confirm: policy?.confirm ?? policy?.confirmationRiskLevels ?? ['state_change'],
		handover: policy?.handover ?? policy?.handoverRiskLevels ?? ['login_secret', 'captcha', 'mfa'],
		blocked: policy?.blocked ?? policy?.blockedRiskLevels ?? ['delete', 'payment'],
	}
}

function variableNameFromTemplate(value: unknown): string {
	const match = typeof value === 'string' ? /^\{\{\s*([a-zA-Z0-9_]+)\s*\}\}$/.exec(value) : null
	return match?.[1] ?? ''
}
