import {
	type AgentConfig,
	type AgentStepEvent,
	type ExecutionResult,
	type HistoricalEvent,
	PageAgentCore,
} from '@page-agent/core'

import {
	type KnowledgeSettings,
	defaultKnowledgeSettings,
} from '@/webops/knowledge/KnowledgeSettings'
import { MemoryClient } from '@/webops/memory/MemoryClient'
import { MemoryContextProvider } from '@/webops/memory/MemoryContextProvider'
import { PageObservationReporter } from '@/webops/memory/PageObservationReporter'
import { TaskRunReporter } from '@/webops/memory/TaskRunReporter'
import type {
	MemoryContextResponse,
	MemoryEvidenceRef,
	MemoryTaskRunActionType,
	MemoryTaskRunRequest,
	MemoryTaskRunResponse,
} from '@/webops/memory/types'
import type { RecordedMemoryContext, RecordedSession } from '@/webops/recorder/actionEvents'
import { isRedactedValue } from '@/webops/recorder/redaction'
import {
	addWebOpsKnowledgeHits,
	finishWebOpsSession,
	recordWebOpsAction,
	setWebOpsMemoryContext,
	startWebOpsSession,
} from '@/webops/recorder/runtimeSession'

import { RemotePageController } from './RemotePageController'
import { TabsController } from './TabsController'
import { formatAskUserResponse } from './askUserResponse'
import {
	type CompletedSensitiveHandover,
	formatCompletedSensitiveHandoverResponse,
	formatRepeatedSensitiveHandoverResponse,
	isSensitiveHandoverQuestion,
	shouldSuppressRepeatedSensitiveHandover,
	shouldUseSensitiveHandover,
} from './sensitiveHandover'
import SYSTEM_PROMPT from './system_prompt.md?raw'
import { createTabTools } from './tabTools'

/** Detect user language from browser settings */
function detectLanguage(): 'en-US' | 'zh-CN' {
	const lang = navigator.language || navigator.languages?.[0] || 'en-US'
	return lang.startsWith('zh') ? 'zh-CN' : 'en-US'
}

interface MultiPageAgentConfig extends AgentConfig {
	includeInitialTab?: boolean
	experimentalIncludeAllTabs?: boolean
	knowledgeSettings?: KnowledgeSettings
	workflowBackend?: {
		baseUrl?: string
		apiKey?: string
		projectId?: string
	}
	onAskUser?: (question: string) => Promise<string>
}

/**
 * MultiPageAgent
 * - use with extension
 * - can be used from a side panel or a content script
 */
export class MultiPageAgent extends PageAgentCore {
	private getWebOpsSessionRef: () => RecordedSession | undefined = () => undefined
	private tabsController: TabsController
	private remotePageController: RemotePageController

	getWebOpsSession() {
		return this.getWebOpsSessionRef()
	}

	async getCurrentPageObservation(task = 'PageAgent page observation') {
		if (!this.tabsController.currentTabId) {
			await this.tabsController.init(task, {
				includeInitialTab: true,
				experimentalIncludeAllTabs: false,
			})
		}
		const tabInfo = this.tabsController.currentTabId
			? await this.tabsController.getTabInfo(this.tabsController.currentTabId)
			: { url: '', title: '' }
		const elements = await this.remotePageController.getWorkflowElements()
		return {
			url: tabInfo.url,
			title: tabInfo.title,
			visibleText: elements.visibleText,
			controls: elements.controls,
		}
	}

	constructor(config: MultiPageAgentConfig) {
		// multi page controller
		const tabsController = new TabsController()
		const pageController = new RemotePageController(tabsController)
		const customTools = createTabTools(tabsController)

		// system prompt - auto-detect language if not specified
		const language = config.language ?? detectLanguage()
		const targetLanguage = language === 'zh-CN' ? '中文' : 'English'
		const systemPrompt = SYSTEM_PROMPT.replace(
			/Default working language: \*\*.*?\*\*/,
			`Default working language: **${targetLanguage}**`
		)

		const includeInitialTab = config.includeInitialTab ?? true
		const experimentalIncludeAllTabs = config.experimentalIncludeAllTabs ?? false

		/**
		 * When the agent is in side-panel and user closed the side-panel.
		 * There is no chance for isAgentRunning to be set false.
		 * (unload event doesn't work well in side panel.)
		 * (I'm trying not to use long-lived connection because the lifecycle of a sw is hard to predict.)
		 * This heartbeat mechanism acts as a backup.
		 */
		let heartBeatInterval: null | number = null
		let pendingMemoryContext = ''
		let pendingMemoryAttributionContext:
			| {
					contextId?: string
					evidenceRefs?: MemoryEvidenceRef[]
			  }
			| undefined
		let pageStateAliases: Record<string, string> = {}
		let webOpsSession: RecordedSession | undefined
		let lastRecordedStepIndex = -1
		let lastSensitiveHandover: CompletedSensitiveHandover | null = null

		super({
			...config,
			pageController: pageController as any,
			customTools: customTools,
			customSystemPrompt: systemPrompt,

			onBeforeTask: async (agent) => {
				await tabsController.init(agent.task, { includeInitialTab, experimentalIncludeAllTabs })

				const tabInfo = tabsController.currentTabId
					? await tabsController.getTabInfo(tabsController.currentTabId)
					: { url: '', title: '' }
				const elements = await pageController.getWorkflowElements()
				const pageObservation = {
					url: tabInfo.url,
					title: tabInfo.title,
					visibleText: elements.visibleText,
					controls: elements.controls,
				}

				startWebOpsSession({
					id: agent.taskId || crypto.randomUUID(),
					task: agent.task,
					startUrl: tabInfo.url,
				})
				webOpsSession = undefined
				pendingMemoryAttributionContext = undefined
				pageStateAliases = {}

				const memoryClient = createMemoryClient(config.workflowBackend)
				if (memoryClient) {
					const projectId = config.workflowBackend?.projectId || 'default'
					const knowledgeSettings = config.knowledgeSettings ?? defaultKnowledgeSettings
					const observationResponse = await new PageObservationReporter(memoryClient).report({
						projectId,
						task: agent.task,
						url: tabInfo.url,
						pageObservation: {
							title: pageObservation.title,
							visibleText: pageObservation.visibleText,
							controls: pageObservation.controls,
						},
						allowPageSummary: knowledgeSettings.enabled && knowledgeSettings.allowPageSummary,
					})
					if (observationResponse?.pageStateId) {
						pageStateAliases[pageStateID(tabInfo.url, '')] = observationResponse.pageStateId
						pageStateAliases[pageStateID(tabInfo.url, tabInfo.title)] =
							observationResponse.pageStateId
					}
					try {
						const memory = await new MemoryContextProvider(memoryClient).getContext({
							projectId,
							task: agent.task,
							currentUrl: tabInfo.url,
							title: tabInfo.title,
							pageObservation: {
								title: pageObservation.title,
								visibleText: pageObservation.visibleText,
								controls: pageObservation.controls,
							},
						})
						pendingMemoryContext = memory.promptContext
						const memoryEvidenceRefs = toMemoryEvidenceRefs(memory.response?.evidenceRefs)
						pendingMemoryAttributionContext = {
							contextId: memory.response?.contextId,
							evidenceRefs: memoryEvidenceRefs,
						}
						if (memory.response) {
							setWebOpsMemoryContext(
								toRecordedMemoryContext(memory.response, memory.promptContext, memoryEvidenceRefs)
							)
						}
						addWebOpsKnowledgeHits(
							memoryEvidenceRefs.map((item) => ({
								id: item.id,
								title: item.title || item.source,
								source: item.source,
								snippet: item.title || item.source,
								score: item.score ?? 0,
							}))
						)
					} catch (error) {
						console.warn('[WebOpsMemory] Failed to load memory context:', error)
					}
				}

				heartBeatInterval = window.setInterval(() => {
					chrome.storage.local.set({
						agentHeartbeat: Date.now(),
					})
				}, 1_000)

				await chrome.storage.local.set({
					isAgentRunning: true,
				})
			},

			onAfterTask: async (agent, result) => {
				if (heartBeatInterval) {
					window.clearInterval(heartBeatInterval)
					heartBeatInterval = null
				}

				webOpsSession = finishWebOpsSession()
				if (webOpsSession) {
					const uploadResponse = await uploadTaskRunToWorkflowBackend({
						session: webOpsSession,
						result,
						history: agent.history,
						workflowBackend: config.workflowBackend,
						memoryContextId: pendingMemoryAttributionContext?.contextId,
						memoryEvidenceRefs: pendingMemoryAttributionContext?.evidenceRefs,
						pageStateAliases,
					})
					webOpsSession = attachMemoryUpdates(webOpsSession, uploadResponse)
					await chrome.storage.local.set({ lastWebOpsSession: webOpsSession })
				}

				await chrome.storage.local.set({
					isAgentRunning: false,
				})
			},

			onBeforeStep: async (agent, step) => {
				if (!tabsController.currentTabId) return
				// make sure the current tab is loaded before the step starts
				await tabsController.waitUntilTabLoaded(tabsController.currentTabId!)
				if (step === 0 && pendingMemoryContext) {
					agent.pushObservation(pendingMemoryContext)
					pendingMemoryContext = ''
				}
			},

			onAfterStep: async (agent, history) => {
				const stepEvent = [...history].reverse().find((event) => event.type === 'step') as
					| AgentStepEvent
					| undefined
				if (!stepEvent || stepEvent.stepIndex === lastRecordedStepIndex) return

				lastRecordedStepIndex = stepEvent.stepIndex
				await recordNonDomStep(stepEvent, tabsController)
			},

			onDispose: () => {
				if (heartBeatInterval) {
					window.clearInterval(heartBeatInterval)
					heartBeatInterval = null
				}

				chrome.storage.local.set({
					isAgentRunning: false,
				})

				tabsController.dispose()
			},
		})

		const pageInteractionAskUser = createPageInteractionAskUser(tabsController, pageController)
		this.tabsController = tabsController
		this.remotePageController = pageController
		this.onAskUser = async (question: string) => {
			if (shouldUseSensitiveHandover(this.task, question)) {
				const tabInfo = await getCurrentTabInfo(tabsController)
				if (
					shouldSuppressRepeatedSensitiveHandover({
						lastHandover: lastSensitiveHandover,
						currentUrl: tabInfo.url,
						question,
						now: Date.now(),
					})
				) {
					return formatRepeatedSensitiveHandoverResponse()
				}
				const answer = await requestSensitivePageHandover(
					question,
					tabsController,
					pageController,
					tabInfo
				)
				if (answer.completed) {
					lastSensitiveHandover = { url: tabInfo.url, completedAt: Date.now() }
				}
				return answer.message
			}
			return config.onAskUser ? config.onAskUser(question) : pageInteractionAskUser(question)
		}

		this.getWebOpsSessionRef = () => webOpsSession
	}
}

export function buildTaskRunPayload(input: {
	session: RecordedSession
	result: ExecutionResult
	history: HistoricalEvent[]
	projectId: string
}): MemoryTaskRunRequest | null {
	if (containsSensitiveMaterial(input.session.task)) return null
	const site = siteFromURL(input.session.startUrl || input.session.steps[0]?.pageUrl || '')
	const valueTemplates = valueTemplatesFromSession(input.session)
	const taskTemplate = truncateSummary(applyTaskTemplates(input.session.task, valueTemplates))
	const originalPath = pagePathFromSession(input.session)
	const optimizedPath = optimizeRepeatedPagePath(originalPath)
	const reasoning = input.history.filter((event): event is AgentStepEvent => event.type === 'step')
	const actionSteps = input.session.steps
		.map((action, index) => {
			const actionType = toWorkflowActionType(action.type)
			if (!actionType) return undefined
			const rawTargetName = targetNameForAction(action)
			const targetName = truncateSummary(applyTaskTemplates(rawTargetName, valueTemplates))
			const valueTemplate = valueTemplateForAction(action, rawTargetName, valueTemplates)
			if ((actionType === 'fill' || actionType === 'select') && !valueTemplate) return undefined
			const pageStateId = pageStateID(action.pageUrl, action.pageTitle)
			const relatedReasoning = reasoning[index]
			const reasoningSummary = applyTaskTemplates(
				sanitizeSummary(
					relatedReasoning?.reflection.next_goal ?? relatedReasoning?.reflection.memory
				),
				valueTemplates
			)
			const resultSummary = applyTaskTemplates(
				sanitizeSummary(action.note ?? action.result),
				valueTemplates
			)
			const surfaceId = surfaceIdForRecordedAction(action, pageStateId, input.session)
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
		.filter((step): step is NonNullable<typeof step> => Boolean(step))

	return {
		id: input.session.id,
		projectId: input.projectId,
		site,
		taskTemplate,
		summary: truncateSummary(
			applyTaskTemplates(sanitizeSummary(input.result.data || taskTemplate), valueTemplates)
		),
		originalPath,
		optimizedPath,
		status: input.result.success ? 'success' : 'failed',
		actionSteps,
	}
}

async function uploadTaskRunToWorkflowBackend(input: {
	session: RecordedSession
	result: ExecutionResult
	history: HistoricalEvent[]
	workflowBackend?: MultiPageAgentConfig['workflowBackend']
	memoryContextId?: string
	memoryEvidenceRefs?: MemoryEvidenceRef[]
	pageStateAliases?: Record<string, string>
}): Promise<MemoryTaskRunResponse | undefined> {
	const client = createMemoryClient(input.workflowBackend)
	if (!client) return
	return new TaskRunReporter(client).complete({
		projectId: input.workflowBackend?.projectId || 'default',
		session: input.session,
		result: {
			success: input.result.success,
			summary: typeof input.result.data === 'string' ? input.result.data : undefined,
		},
		reasoning: input.history
			.filter((event): event is AgentStepEvent => event.type === 'step')
			.map((event) => ({
				nextGoal: event.reflection.next_goal,
				memory: event.reflection.memory,
			})),
		memoryContextId: input.memoryContextId,
		memoryEvidenceRefs: input.memoryEvidenceRefs,
		pageStateAliases: input.pageStateAliases,
	})
}

function toMemoryEvidenceRefs(evidence: MemoryEvidenceRef[] | undefined): MemoryEvidenceRef[] {
	if (!evidence?.length) return []
	return evidence.map((item) => ({ ...item, payload: cloneEvidencePayload(item.payload) }))
}

function toRecordedMemoryContext(
	response: MemoryContextResponse,
	contextPrompt: string,
	evidenceRefs: MemoryEvidenceRef[] | undefined
): RecordedMemoryContext {
	return {
		contextId: response.contextId,
		contextPrompt,
		recommendedMode: response.recommendedMode,
		currentPageState: response.currentPageState,
		currentSurface: response.currentSurface,
		evidenceRefs: evidenceRefs ?? [],
		debug: response.debug,
	}
}

function cloneEvidencePayload(payload: MemoryEvidenceRef['payload']) {
	if (!payload) return undefined
	return JSON.parse(JSON.stringify(payload)) as Record<string, unknown>
}

function attachMemoryUpdates(
	session: RecordedSession,
	response: MemoryTaskRunResponse | undefined
): RecordedSession {
	if (!response?.memoryUpdates?.length) return session
	return {
		...session,
		memoryUpdates: response.memoryUpdates,
	}
}

function createMemoryClient(
	workflowBackend?: MultiPageAgentConfig['workflowBackend']
): MemoryClient | undefined {
	if (!workflowBackend?.baseUrl) return undefined
	return new MemoryClient({
		baseUrl: workflowBackend.baseUrl,
		bearerToken: workflowBackend.apiKey || undefined,
	})
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

function pagePathFromSession(session: RecordedSession): string[] {
	const values = [
		pageStateID(session.startUrl, ''),
		...session.steps.map((action) => pageStateID(action.pageUrl, action.pageTitle)),
	].filter(Boolean)
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

function toWorkflowActionType(
	type: RecordedSession['steps'][number]['type']
): MemoryTaskRunActionType | undefined {
	if (type === 'click') return 'click'
	if (type === 'input') return 'fill'
	if (type === 'select') return 'select'
	if (type === 'wait') return 'wait'
	return undefined
}

function surfaceIdForRecordedAction(
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

function siteFromURL(rawURL: string): string {
	try {
		return new URL(rawURL).host
	} catch {
		return ''
	}
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

async function requestSensitivePageHandover(
	question: string,
	tabsController: TabsController,
	pageController: RemotePageController,
	tabInfo?: { url: string; title: string }
) {
	const currentTabInfo = tabInfo ?? (await getCurrentTabInfo(tabsController))
	const response = await pageController.requestUserHandover(
		'请直接在网页中输入账号、密码、验证码或完成登录验证。PageAgent 不会读取或保存这些敏感信息。完成后点击这里继续。'
	)

	recordWebOpsAction({
		id: crypto.randomUUID(),
		type: 'handover',
		timestamp: Date.now(),
		pageUrl: currentTabInfo.url,
		pageTitle: currentTabInfo.title,
		result: response.type === 'cancelled' ? 'skipped' : 'success',
		note: question,
	})

	return {
		completed: response.type === 'handover_done',
		message:
			response.type === 'handover_done'
				? formatCompletedSensitiveHandoverResponse()
				: formatAskUserResponse(response),
	}
}

function createPageInteractionAskUser(
	tabsController: TabsController,
	pageController: RemotePageController
) {
	return async (question: string) => {
		if (isSensitiveHandoverQuestion(question)) {
			const answer = await requestSensitivePageHandover(question, tabsController, pageController)
			return answer.message
		}

		const tabInfo = await getCurrentTabInfo(tabsController)
		const response = await pageController.requestInteraction(
			{
				type: 'input',
				requestId: crypto.randomUUID(),
				title: '需要你补充信息',
				message: question,
				placeholder: '请直接输入答案，提交后 Agent 会继续执行。',
				submitButtonLabel: '提交并继续',
			},
			600_000
		)

		recordWebOpsAction({
			id: crypto.randomUUID(),
			type: 'handover',
			timestamp: Date.now(),
			pageUrl: tabInfo.url,
			pageTitle: tabInfo.title,
			result: response.type === 'cancelled' ? 'skipped' : 'success',
			note: question,
		})

		return formatAskUserResponse(response)
	}
}

async function getCurrentTabInfo(
	tabsController: TabsController
): Promise<{ url: string; title: string }> {
	return tabsController.currentTabId
		? tabsController.getTabInfo(tabsController.currentTabId)
		: { url: '', title: '' }
}

async function recordNonDomStep(stepEvent: AgentStepEvent, tabsController: TabsController) {
	const action = stepEvent.action
	if (['click_element_by_index', 'input_text', 'select_dropdown_option'].includes(action.name)) {
		return
	}

	const tabInfo = tabsController.currentTabId
		? await tabsController.getTabInfo(tabsController.currentTabId)
		: { url: '', title: '' }
	const result = action.output.startsWith('❌') ? 'failed' : 'success'
	const base = {
		id: crypto.randomUUID(),
		timestamp: Date.now(),
		pageUrl: tabInfo.url,
		pageTitle: tabInfo.title,
		result,
		note: action.output,
	} as const

	if (action.name === 'wait') {
		recordWebOpsAction({
			...base,
			type: 'wait',
			value: String(action.input?.seconds ?? 1),
		})
		return
	}

	if (action.name === 'open_new_tab') {
		recordWebOpsAction({
			...base,
			type: 'navigate',
			value: String(action.input?.url ?? tabInfo.url),
		})
		return
	}

	if (action.name === 'done') {
		recordWebOpsAction({
			...base,
			type: 'extract',
			target: action.input?.text ? { text: String(action.input.text).slice(0, 120) } : undefined,
			value: action.input?.text ? String(action.input.text) : undefined,
		})
		return
	}

	if (action.name === 'ask_user') return

	recordWebOpsAction({
		...base,
		type: 'observe',
	})
}
