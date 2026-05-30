import type { RecordedSession } from '../recorder/actionEvents'
import { isRedactedValue } from '../recorder/redaction'
import type {
	MemoryClientLike,
	MemoryEvidenceRef,
	MemoryTaskRunActionStep,
	MemoryTaskRunActionType,
	MemoryTaskRunRequest,
	MemoryTaskRunResponse,
} from './types'

export interface MemoryTaskRunBuildInput {
	projectId: string
	session: RecordedSession
	result: { success: boolean; summary?: string }
	reasoning?: { nextGoal?: string; memory?: string }[]
	pageStateAliases?: Record<string, string>
	memoryContextId?: string
	memoryEvidenceRefs?: MemoryEvidenceRef[]
}

export class TaskRunReporter {
	private readonly client: MemoryClientLike | undefined

	constructor(client: MemoryClientLike | undefined) {
		this.client = client
	}

	async complete(input: MemoryTaskRunBuildInput): Promise<MemoryTaskRunResponse | undefined> {
		if (!this.client?.completeTaskRun) return

		const payload = buildMemoryTaskRunPayload(input)
		if (!payload) return

		try {
			return await this.client.completeTaskRun(payload)
		} catch (error) {
			console.warn('[WebOpsMemory] Failed to upload task run:', errorMessage(error))
		}
	}
}

export function buildMemoryTaskRunPayload(
	input: MemoryTaskRunBuildInput
): MemoryTaskRunRequest | null {
	if (containsSensitiveMaterial(input.session.task)) return null

	const site = siteFromURL(input.session.startUrl || input.session.steps[0]?.pageUrl || '')
	const valueTemplates = valueTemplatesFromSession(input.session)
	const taskTemplate = truncateSummary(applyTaskTemplates(input.session.task, valueTemplates))
	const summary = truncateSummary(
		applyTaskTemplates(sanitizeSummary(input.result.summary || taskTemplate), valueTemplates)
	)
	const originalPath = pagePathFromSession(input.session, input.pageStateAliases)
	const optimizedPath = optimizeRepeatedPagePath(originalPath)
	const actionSteps = input.session.steps
		.map((action, index): MemoryTaskRunActionStep | undefined => {
			const actionType = toMemoryActionType(action.type)
			if (!actionType) return undefined
			const rawTargetName = targetNameForAction(action)
			const targetName = truncateSummary(applyTaskTemplates(rawTargetName, valueTemplates))
			const valueTemplate = valueTemplateForAction(action, rawTargetName, valueTemplates)
			if ((actionType === 'fill' || actionType === 'select') && !valueTemplate) return undefined
			const pageStateId = applyPageStateAlias(
				pageStateID(action.pageUrl, action.pageTitle),
				input.pageStateAliases
			)
			const surfaceId = surfaceIdForAction(action, pageStateId, input.session)
			const relatedReasoning = input.reasoning?.[index]
			const reasoningSummary = applyTaskTemplates(
				sanitizeSummary(relatedReasoning?.nextGoal ?? relatedReasoning?.memory),
				valueTemplates
			)
			const resultSummary = applyTaskTemplates(
				sanitizeSummary(action.note ?? action.result),
				valueTemplates
			)
			return {
				id: action.id,
				pageStateId,
				...(surfaceId ? { surfaceId } : {}),
				stepIndex: index + 1,
				actionType,
				targetName,
				valueTemplate,
				reasoningSummary: truncateSummary(reasoningSummary),
				resultSummary: truncateSummary(resultSummary),
				isBranchNoise: !optimizedPath.includes(pageStateId),
			}
		})
		.filter((step): step is MemoryTaskRunActionStep => Boolean(step))

	return {
		id: input.session.id,
		projectId: input.projectId,
		site,
		taskTemplate,
		summary,
		originalPath,
		optimizedPath,
		status: input.result.success ? 'success' : 'failed',
		memoryContextId: input.memoryContextId,
		memoryEvidenceRefs: input.memoryEvidenceRefs,
		actionSteps,
	}
}

function valueTemplatesFromSession(session: RecordedSession): Map<string, string> {
	const templates = new Map<string, string>()
	for (const action of session.steps) {
		if (!action.value || isRedactedValue(action.value) || containsSensitiveMaterial(action.value)) {
			continue
		}
		const targetName = targetNameForAction(action)
		templates.set(action.value, `{{${variableNameForTarget(targetName)}}}`)
	}
	return templates
}

function applyTaskTemplates(task: string, templates: Map<string, string>): string {
	let result = task
	for (const [value, template] of templates) {
		result = result.split(value).join(template)
	}
	if (hasTemplate(templates, '{{service_name}}')) {
		result = result.replace(
			/\b[A-Za-z][A-Za-z0-9]*(?:-[A-Za-z0-9]+)*(?:-api|-service|-svc)\b/g,
			'{{service_name}}'
		)
	}
	return result
		.replace(/\b\d{3,}\b/g, '{{value}}')
		.replace(/\b[A-Za-z]+-[A-Za-z0-9]+-[A-Za-z0-9]+\b/g, '{{value}}')
}

function hasTemplate(templates: Map<string, string>, templateName: string): boolean {
	for (const template of templates.values()) {
		if (template === templateName) return true
	}
	return false
}

function valueTemplateForAction(
	action: RecordedSession['steps'][number],
	targetName: string,
	templates: Map<string, string>
): string | undefined {
	if (!action.value || isRedactedValue(action.value) || containsSensitiveMaterial(action.value)) {
		return undefined
	}
	return templates.get(action.value) ?? `{{${variableNameForTarget(targetName)}}}`
}

function variableNameForTarget(targetName: string): string {
	const normalized = targetName.toLowerCase()
	if (normalized.includes('服务') || normalized.includes('service')) return 'service_name'
	if (
		normalized.includes('搜索') ||
		normalized.includes('search') ||
		normalized.includes('query')
	) {
		return 'query'
	}
	if (normalized.includes('issue')) return 'issue_id'
	if (normalized.includes('订单') || normalized.includes('order')) return 'order_id'
	return 'input_value'
}

function pagePathFromSession(
	session: RecordedSession,
	pageStateAliases?: Record<string, string>
): string[] {
	const values = [
		pageStateID(session.startUrl, ''),
		...session.steps.map((action) => pageStateID(action.pageUrl, action.pageTitle)),
	]
		.map((id) => applyPageStateAlias(id, pageStateAliases))
		.filter(Boolean)
	const result: string[] = []
	for (const value of values) {
		if (result[result.length - 1] !== value) result.push(value)
	}
	return result
}

function optimizeRepeatedPagePath(path: string[]): string[] {
	const result: string[] = []
	for (const page of path) {
		const existingIndex = result.indexOf(page)
		if (existingIndex >= 0) {
			result.splice(existingIndex + 1)
			continue
		}
		result.push(page)
	}
	return result.length > 0 ? result : path
}

function pageStateID(rawURL: string, title: string): string {
	const site = siteFromURL(rawURL)
	const path = pathPatternFromURL(rawURL)
	const fallback = slug(title) || 'page'
	return [site || 'unknown', path || fallback].filter(Boolean).join('_')
}

function pathPatternFromURL(rawURL: string): string {
	try {
		const parsed = new URL(rawURL)
		return parsed.pathname
			.split('/')
			.filter(Boolean)
			.map((part) => (looksLikeInstancePathPart(part) ? 'param' : slug(part)))
			.filter(Boolean)
			.join('_')
	} catch {
		return ''
	}
}

function looksLikeInstancePathPart(value: string): boolean {
	return (
		/^\d{3,}$/.test(value) ||
		/^[a-f0-9]{8,}$/i.test(value) ||
		/^[A-Za-z]+-[A-Za-z0-9-]{4,}$/.test(value)
	)
}

function targetNameForAction(action: RecordedSession['steps'][number]): string {
	return truncateSummary(
		sanitizeSummary(
			action.target?.name ||
				action.target?.role ||
				action.target?.testId ||
				(Number.isFinite(action.target?.elementIndex)
					? `element_${action.target?.elementIndex}`
					: action.type)
		)
	)
}

function toMemoryActionType(
	type: RecordedSession['steps'][number]['type']
): MemoryTaskRunActionType | undefined {
	if (type === 'click') return 'click'
	if (type === 'input') return 'fill'
	if (type === 'select') return 'select'
	if (type === 'wait') return 'wait'
	return undefined
}

function siteFromURL(rawURL: string): string {
	try {
		return new URL(rawURL).host
	} catch {
		return ''
	}
}

function applyPageStateAlias(id: string, aliases?: Record<string, string>): string {
	if (!id) return id
	return aliases?.[id] || id
}

function surfaceIdForAction(
	action: RecordedSession['steps'][number],
	pageStateId: string,
	session: RecordedSession
): string | undefined {
	if (action.surfaceId) return action.surfaceId
	const surface = session.memoryContext?.currentSurface
	if (!surface?.id) return undefined
	const currentPageIds = [
		session.memoryContext?.currentPageState?.id,
		surface.parentPageStateId,
	].filter((value): value is string => Boolean(value))
	if (currentPageIds.length === 0 || currentPageIds.includes(pageStateId)) {
		return surface.id
	}
	return undefined
}

function slug(value: string): string {
	return value
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, '_')
		.replace(/^_+|_+$/g, '')
}

function sanitizeSummary(value: unknown): string {
	const text =
		typeof value === 'string'
			? value.trim()
			: typeof value === 'number' || typeof value === 'boolean'
				? String(value)
				: ''
	if (!text) return ''
	if (containsSensitiveMaterial(text)) return 'Sensitive content omitted.'
	return text
}

function truncateSummary(value: string): string {
	let result = ''
	let count = 0
	for (const char of value) {
		if (count >= 500) break
		result += char
		count += 1
	}
	return result
}

function containsSensitiveMaterial(value: string): boolean {
	return /(password|passwd|pwd|token|access[_-]?token|refresh[_-]?token|cookie|captcha|secret|api[_-]?key|authorization|bearer|验证码|密码|密钥|令牌)/i.test(
		value
	)
}

function errorMessage(error: unknown): string {
	return error instanceof Error ? error.message : String(error)
}
