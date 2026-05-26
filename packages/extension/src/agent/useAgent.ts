/**
 * React hook for using AgentController
 */
import type {
	AgentActivity,
	AgentStatus,
	ExecutionResult,
	HistoricalEvent,
	SupportedLanguage,
} from '@page-agent/core'
import type { LLMConfig } from '@page-agent/llms'
import { useCallback, useEffect, useRef, useState } from 'react'

import {
	type KnowledgeSettings,
	defaultKnowledgeSettings,
} from '@/webops/knowledge/KnowledgeSettings'
import type { RecordedSession } from '@/webops/recorder/actionEvents'

import { MultiPageAgent } from './MultiPageAgent'
import { type PendingUserQuestion, createAskUserBridge } from './askUserBridge'
import { DEMO_CONFIG, migrateLegacyEndpoint } from './constants'

/** Language preference: undefined means follow system */
export type LanguagePreference = SupportedLanguage | undefined

export interface AdvancedConfig {
	maxSteps?: number
	systemInstruction?: string
	experimentalLlmsTxt?: boolean
	experimentalIncludeAllTabs?: boolean
	disableNamedToolChoice?: boolean
}

export interface ExtConfig extends LLMConfig, AdvancedConfig {
	language?: LanguagePreference
	knowledgeSettings?: KnowledgeSettings
}

export interface UseAgentResult {
	status: AgentStatus
	history: HistoricalEvent[]
	activity: AgentActivity | null
	currentTask: string
	config: ExtConfig | null
	webOpsSession: RecordedSession | null | undefined
	pendingQuestion: PendingUserQuestion | null
	execute: (task: string, options?: ExecuteOptions) => Promise<ExecutionResult>
	answerQuestion: (answer: string) => boolean
	newSession: () => void
	stop: () => void
	configure: (config: ExtConfig) => Promise<void>
}

export interface ExecuteOptions {
	displayTask?: string
	carryHistory?: HistoricalEvent[]
}

export function useAgent(): UseAgentResult {
	const agentRef = useRef<MultiPageAgent | null>(null)
	const historyPrefixRef = useRef<HistoricalEvent[]>([])
	const askBridgeRef = useRef<ReturnType<typeof createAskUserBridge> | null>(null)
	const [status, setStatus] = useState<AgentStatus>('idle')
	const [history, setHistory] = useState<HistoricalEvent[]>([])
	const [activity, setActivity] = useState<AgentActivity | null>(null)
	const [currentTask, setCurrentTask] = useState('')
	const [config, setConfig] = useState<ExtConfig | null>(null)
	const [webOpsSession, setWebOpsSession] = useState<RecordedSession | null | undefined>(undefined)
	const [pendingQuestion, setPendingQuestion] = useState<PendingUserQuestion | null>(null)

	useEffect(() => {
		chrome.storage.local
			.get(['llmConfig', 'language', 'advancedConfig', 'knowledgeSettings'])
			.then((result) => {
				let llmConfig = (result.llmConfig as LLMConfig) ?? DEMO_CONFIG
				const language = (result.language as SupportedLanguage) || undefined
				const advancedConfig = (result.advancedConfig as AdvancedConfig) ?? {}
				const knowledgeSettings =
					(result.knowledgeSettings as KnowledgeSettings | undefined) ?? defaultKnowledgeSettings

				// Auto-migrate legacy testing endpoints
				const migrated = migrateLegacyEndpoint(llmConfig)
				if (migrated !== llmConfig) {
					llmConfig = migrated
					chrome.storage.local.set({ llmConfig: migrated })
				} else if (!result.llmConfig) {
					chrome.storage.local.set({ llmConfig: DEMO_CONFIG })
				}

				setConfig({ ...llmConfig, ...advancedConfig, language, knowledgeSettings })
			})
	}, [])

	useEffect(() => {
		if (!config) return

		const { systemInstruction, ...agentConfig } = config
		const askBridge = createAskUserBridge(setPendingQuestion)
		askBridgeRef.current = askBridge
		const agent = new MultiPageAgent({
			...agentConfig,
			instructions: systemInstruction ? { system: systemInstruction } : undefined,
			onAskUser: askBridge.ask,
		})
		agentRef.current = agent

		const handleStatusChange = (e: Event) => {
			const newStatus = agent.status as AgentStatus
			setStatus(newStatus)
			if (newStatus === 'idle' || newStatus === 'completed' || newStatus === 'error') {
				setActivity(null)
			}
		}

		const handleHistoryChange = (e: Event) => {
			setHistory([...historyPrefixRef.current, ...agent.history])
		}

		const handleActivity = (e: Event) => {
			const newActivity = (e as CustomEvent).detail as AgentActivity
			setActivity(newActivity)
		}

		agent.addEventListener('statuschange', handleStatusChange)
		agent.addEventListener('historychange', handleHistoryChange)
		agent.addEventListener('activity', handleActivity)

		return () => {
			askBridge.cancel('Agent 已关闭，未继续等待用户回答。')
			if (askBridgeRef.current === askBridge) {
				askBridgeRef.current = null
			}
			agent.removeEventListener('statuschange', handleStatusChange)
			agent.removeEventListener('historychange', handleHistoryChange)
			agent.removeEventListener('activity', handleActivity)
			agent.dispose()
		}
	}, [config])

	const execute = useCallback(async (task: string, options: ExecuteOptions = {}) => {
		const agent = agentRef.current
		if (!agent) throw new Error('Agent not initialized')

		askBridgeRef.current?.cancel('用户开始了新任务，上一轮问题已取消。')
		historyPrefixRef.current = options.carryHistory ?? []
		setCurrentTask(options.displayTask ?? task)
		setHistory([...historyPrefixRef.current])
		setWebOpsSession(undefined)
		const result = await agent.execute(task)
		setWebOpsSession(agent.getWebOpsSession() ?? null)
		return result
	}, [])

	const answerQuestion = useCallback((answer: string) => {
		return askBridgeRef.current?.answer(answer) ?? false
	}, [])

	const newSession = useCallback(() => {
		askBridgeRef.current?.cancel('用户新建会话，上一轮问题已取消。')
		historyPrefixRef.current = []
		setHistory([])
		setActivity(null)
		setCurrentTask('')
		setWebOpsSession(undefined)
		setPendingQuestion(null)
		setStatus('idle')
	}, [])

	const stop = useCallback(() => {
		askBridgeRef.current?.cancel('用户停止了任务，未提供补充信息。')
		agentRef.current?.stop()
	}, [])

	const configure = useCallback(
		async ({
			language,
			maxSteps,
			systemInstruction,
			experimentalLlmsTxt,
			experimentalIncludeAllTabs,
			disableNamedToolChoice,
			knowledgeSettings,
			...llmConfig
		}: ExtConfig) => {
			await chrome.storage.local.set({ llmConfig })
			await chrome.storage.local.set({
				knowledgeSettings: knowledgeSettings ?? defaultKnowledgeSettings,
			})
			if (language) {
				await chrome.storage.local.set({ language })
			} else {
				await chrome.storage.local.remove('language')
			}
			const advancedConfig: AdvancedConfig = {
				maxSteps,
				systemInstruction,
				experimentalLlmsTxt,
				experimentalIncludeAllTabs,
				disableNamedToolChoice,
			}
			await chrome.storage.local.set({ advancedConfig })
			setConfig({
				...llmConfig,
				...advancedConfig,
				language,
				knowledgeSettings: knowledgeSettings ?? defaultKnowledgeSettings,
			})
		},
		[]
	)

	return {
		status,
		history,
		activity,
		currentTask,
		config,
		webOpsSession,
		pendingQuestion,
		execute,
		answerQuestion,
		newSession,
		stop,
		configure,
	}
}
