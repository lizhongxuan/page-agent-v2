import { type AgentConfig, type AgentStepEvent, PageAgentCore } from '@page-agent/core'

import { HttpKnowledgeClient } from '@/webops/knowledge/HttpKnowledgeClient'
import { KnowledgeContextProvider } from '@/webops/knowledge/KnowledgeContextProvider'
import {
	type KnowledgeSettings,
	defaultKnowledgeSettings,
} from '@/webops/knowledge/KnowledgeSettings'
import type { RecordedSession } from '@/webops/recorder/actionEvents'
import {
	addWebOpsKnowledgeHits,
	finishWebOpsSession,
	recordWebOpsAction,
	startWebOpsSession,
} from '@/webops/recorder/runtimeSession'
import type {
	WorkflowReplayResult,
	WorkflowReplayService,
} from '@/webops/workflow/WorkflowReplayService'

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
	onAskUser?: (question: string) => Promise<string>
}

/**
 * MultiPageAgent
 * - use with extension
 * - can be used from a side panel or a content script
 */
export class MultiPageAgent extends PageAgentCore {
	private getWebOpsSessionRef: () => RecordedSession | undefined = () => undefined
	private replayWorkflowRef: (
		task: string,
		service: WorkflowReplayService
	) => Promise<WorkflowReplayResult> = async () => ({
		status: 'skipped',
		reason: 'Agent has not initialized workflow replay.',
	})

	getWebOpsSession() {
		return this.getWebOpsSessionRef()
	}

	tryReplayWorkflow(task: string, service: WorkflowReplayService): Promise<WorkflowReplayResult> {
		return this.replayWorkflowRef(task, service)
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
		let pendingKnowledgeContext = ''
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

				startWebOpsSession({
					id: agent.taskId || crypto.randomUUID(),
					task: agent.task,
					startUrl: tabInfo.url,
				})
				webOpsSession = undefined

				const knowledgeSettings = config.knowledgeSettings ?? defaultKnowledgeSettings
				if (knowledgeSettings.enabled && knowledgeSettings.baseUrl) {
					const provider = new KnowledgeContextProvider(
						new HttpKnowledgeClient({
							baseUrl: knowledgeSettings.baseUrl,
							apiKey: knowledgeSettings.apiKey || undefined,
						})
					)
					const knowledge = await provider.getContext({
						task: agent.task,
						url: tabInfo.url,
						title: tabInfo.title,
						projectKey: knowledgeSettings.projectKey || undefined,
						limit: 3,
					})
					pendingKnowledgeContext = knowledge.promptContext
					addWebOpsKnowledgeHits(knowledge.hits)
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

			onAfterTask: async () => {
				if (heartBeatInterval) {
					window.clearInterval(heartBeatInterval)
					heartBeatInterval = null
				}

				webOpsSession = finishWebOpsSession()
				if (webOpsSession) {
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
				if (step === 0 && pendingKnowledgeContext) {
					agent.pushObservation(pendingKnowledgeContext)
					pendingKnowledgeContext = ''
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
		this.replayWorkflowRef = async (task, service) => {
			await tabsController.init(task, { includeInitialTab, experimentalIncludeAllTabs })
			const requestedUrl = extractReplayStartUrl(task)
			if (requestedUrl) {
				const current = tabsController.currentTabId
					? await tabsController.getTabInfo(tabsController.currentTabId)
					: { url: '' }
				if (!sameUrlWithoutHash(current.url, requestedUrl)) {
					await tabsController.openNewTab(requestedUrl)
				}
			}
			if (tabsController.currentTabId) {
				await tabsController.waitUntilTabLoaded(tabsController.currentTabId)
			}
			return service.tryReplay(task, pageController)
		}
	}
}

function extractReplayStartUrl(task: string): string | undefined {
	const match = /https?:\/\/[^\s"'<>，。)）]+/i.exec(task)
	return match?.[0]
}

function sameUrlWithoutHash(left: string, right: string): boolean {
	try {
		const leftUrl = new URL(left)
		const rightUrl = new URL(right)
		leftUrl.hash = ''
		rightUrl.hash = ''
		return leftUrl.toString() === rightUrl.toString()
	} catch {
		return left === right
	}
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
