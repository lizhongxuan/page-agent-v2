import { BookOpen, History, Plus, Send, Settings, Square } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import { ConfigPanel } from '@/components/ConfigPanel'
import { HistoryDetail } from '@/components/HistoryDetail'
import { HistoryList } from '@/components/HistoryList'
import { ActivityCard, EventCard } from '@/components/cards'
import { EmptyState, MotionOverlay, StatusDot } from '@/components/misc'
import { Button } from '@/components/ui/button'
import {
	InputGroup,
	InputGroupAddon,
	InputGroupButton,
	InputGroupTextarea,
} from '@/components/ui/input-group'
import { saveSession } from '@/lib/db'
import { SiteManualClient } from '@/webops/site-manuals/SiteManualClient'
import type { SiteManualSource } from '@/webops/site-manuals/types'

import {
	buildSessionContinuationTask,
	formatSessionDisplayTask,
	getSessionContinuationBaseTask,
} from '../../agent/sessionContinuation'
import { useAgent } from '../../agent/useAgent'

type View =
	| { name: 'chat' }
	| { name: 'config' }
	| { name: 'history' }
	| { name: 'history-detail'; sessionId: string }

interface RunTaskOptions {
	forceNewSession?: boolean
}

interface CurrentTabSite {
	site: string
	url: string
}

export default function App() {
	const [view, setView] = useState<View>({ name: 'chat' })
	const [inputValue, setInputValue] = useState('')
	const [currentTabSite, setCurrentTabSite] = useState<CurrentTabSite>({ site: '', url: '' })
	const [siteManualSources, setSiteManualSources] = useState<SiteManualSource[]>([])
	const [siteManualStatus, setSiteManualStatus] = useState<'idle' | 'loading' | 'error'>('idle')
	const historyRef = useRef<HTMLDivElement>(null)
	const textareaRef = useRef<HTMLTextAreaElement>(null)

	const {
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
	} = useAgent()

	const siteManualClient = useMemo(
		() =>
			config?.workflowBackend?.baseUrl
				? new SiteManualClient({
						baseUrl: config.workflowBackend.baseUrl,
						bearerToken: config.workflowBackend.apiKey,
					})
				: undefined,
		[config?.workflowBackend?.apiKey, config?.workflowBackend?.baseUrl]
	)
	const projectId = config?.workflowBackend?.projectId ?? 'default'

	// Persist session when task finishes
	const savedSessionKeyRef = useRef('')
	useEffect(() => {
		if (
			(status === 'completed' || status === 'error') &&
			history.length > 0 &&
			currentTask &&
			webOpsSession !== undefined
		) {
			const saveKey = `${currentTask}:${status}:${history.length}`
			if (savedSessionKeyRef.current === saveKey) return
			savedSessionKeyRef.current = saveKey

			saveSession({
				task: currentTask,
				history,
				status,
				webOpsSession: webOpsSession ?? undefined,
			}).catch((err) => console.error('[SidePanel] Failed to save session:', err))
		}
	}, [status, history, currentTask, webOpsSession])

	// Auto-scroll to bottom on new events
	useEffect(() => {
		if (historyRef.current) {
			historyRef.current.scrollTop = historyRef.current.scrollHeight
		}
	}, [history, activity])

	useEffect(() => {
		let cancelled = false
		async function loadCurrentTabSite() {
			if (typeof chrome === 'undefined' || !chrome.tabs?.query) return
			const tabs = await chrome.tabs.query({ active: true, currentWindow: true })
			const url = tabs[0]?.url ?? ''
			if (cancelled || !isHttpUrl(url)) return
			setCurrentTabSite({
				site: new URL(url).hostname,
				url,
			})
		}
		loadCurrentTabSite().catch((error) => {
			console.warn('[SidePanel] Failed to detect current tab site:', error)
		})
		return () => {
			cancelled = true
		}
	}, [])

	useEffect(() => {
		if (!siteManualClient?.listSiteManuals || !currentTabSite.site) {
			setSiteManualSources([])
			return
		}
		let cancelled = false
		setSiteManualStatus('loading')
		siteManualClient
			.listSiteManuals({
				projectId,
				site: currentTabSite.site,
				status: 'active',
			})
			.then((response) => {
				if (cancelled) return
				setSiteManualSources(response.sources ?? response.items ?? [])
				setSiteManualStatus('idle')
			})
			.catch((error) => {
				if (cancelled) return
				console.warn('[SidePanel] Failed to load current site manuals:', error)
				setSiteManualSources([])
				setSiteManualStatus('error')
			})
		return () => {
			cancelled = true
		}
	}, [currentTabSite.site, projectId, siteManualClient])

	const runTask = useCallback(
		(task: string, options: RunTaskOptions = {}) => {
			const normalizedTask = task.trim()
			if (!normalizedTask || status === 'running') return

			setInputValue('')
			setView({ name: 'chat' })

			const shouldContinueSession =
				!options.forceNewSession && (!!currentTask || history.length > 0)
			const previousTask = getSessionContinuationBaseTask(currentTask || '上一轮浏览器任务')
			const taskToExecute = shouldContinueSession
				? buildSessionContinuationTask({
						previousTask,
						userMessage: normalizedTask,
					})
				: normalizedTask
			const displayTask = shouldContinueSession
				? formatSessionDisplayTask(previousTask, normalizedTask)
				: normalizedTask
			const carryHistory = shouldContinueSession ? history : undefined

			execute(taskToExecute, { displayTask, carryHistory }).catch((error) => {
				console.error('[SidePanel] Failed to execute task:', error)
			})
		},
		[currentTask, execute, history, status]
	)

	const handleSubmit = useCallback(
		(e?: React.SyntheticEvent) => {
			e?.preventDefault()
			const answer = inputValue.trim()
			if (pendingQuestion) {
				if (!answer) return
				if (answerQuestion(answer)) {
					setInputValue('')
					setView({ name: 'chat' })
				}
				return
			}
			runTask(inputValue)
		},
		[answerQuestion, inputValue, pendingQuestion, runTask]
	)

	const handleStop = useCallback(() => {
		console.log('[SidePanel] Stopping task...')
		stop()
	}, [stop])

	const handleNewSession = useCallback(() => {
		if (status === 'running') return
		setInputValue('')
		setView({ name: 'chat' })
		newSession()
	}, [newSession, status])

	const openSiteManualLibrary = useCallback(() => {
		const params = new URLSearchParams({ view: 'site-manuals' })
		if (currentTabSite.site) params.set('site', currentTabSite.site)
		if (currentTabSite.url) params.set('url', currentTabSite.url)
		chrome.tabs.create({ url: chrome.runtime.getURL(`/hub.html?${params.toString()}`) })
	}, [currentTabSite])

	const handleKeyDown = (e: React.KeyboardEvent) => {
		if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
			e.preventDefault()
			handleSubmit()
		}
	}

	// --- View routing ---

	if (view.name === 'config') {
		return (
			<ConfigPanel
				config={config}
				onSave={async (newConfig) => {
					await configure(newConfig)
					setView({ name: 'chat' })
				}}
				onClose={() => setView({ name: 'chat' })}
			/>
		)
	}

	if (view.name === 'history') {
		return (
			<HistoryList
				onSelect={(id) => setView({ name: 'history-detail', sessionId: id })}
				onBack={() => setView({ name: 'chat' })}
				onRerun={(task) => runTask(task, { forceNewSession: true })}
			/>
		)
	}

	if (view.name === 'history-detail') {
		return (
			<HistoryDetail
				sessionId={view.sessionId}
				onBack={() => setView({ name: 'history' })}
				onRerun={(task) => runTask(task, { forceNewSession: true })}
			/>
		)
	}

	// --- Chat view ---

	const isRunning = status === 'running'
	const isAnsweringQuestion = isRunning && !!pendingQuestion
	const isHandoverQuestion = pendingQuestion?.kind === 'handover'
	const showEmptyState = !currentTask && history.length === 0 && !isRunning
	const inputPlaceholder = pendingQuestion
		? isHandoverQuestion
			? '请先在网页里完成输入，完成后点击“继续”'
			: '请在这里回答 Agent 的问题，Enter 提交后继续'
		: isRunning
			? 'Agent 正在执行。点击右侧停止按钮可取消'
			: '描述你的任务...（Enter 发送）'
	return (
		<div className="relative flex flex-col h-screen bg-background">
			<MotionOverlay active={isRunning} />
			{/* Header actions. Chrome already renders the extension title above the side panel. */}
			<header className="flex items-center justify-between border-b px-3 py-2">
				<Button
					variant="outline"
					size="sm"
					onClick={handleNewSession}
					disabled={isRunning}
					className="h-7 gap-1 px-2 text-xs"
					aria-label="新建会话"
					title={isRunning ? '请先停止当前任务' : '新建会话'}
				>
					<Plus className="size-3.5" />
					新建会话
				</Button>
				<div className="flex items-center gap-1">
					<StatusDot status={status} />
					<Button
						variant="ghost"
						size="icon-sm"
						onClick={openSiteManualLibrary}
						className="cursor-pointer"
						aria-label="Site Manual Library"
						title="Site Manual Library"
					>
						<BookOpen className="size-3.5" />
					</Button>
					<Button
						variant="ghost"
						size="icon-sm"
						onClick={() => setView({ name: 'history' })}
						className="cursor-pointer"
						aria-label="History"
						title="History"
					>
						<History className="size-3.5" />
					</Button>
					<Button
						variant="ghost"
						size="icon-sm"
						onClick={() => setView({ name: 'config' })}
						className="cursor-pointer"
						aria-label="Settings"
						title="Settings"
					>
						<Settings className="size-3.5" />
					</Button>
				</div>
			</header>

			{/* Content */}
			<main className="flex-1 overflow-hidden flex flex-col">
				<CurrentSiteManualStatus
					site={currentTabSite.site}
					manualCount={siteManualSources.length}
					status={siteManualStatus}
					backendConfigured={!!siteManualClient}
					onOpen={openSiteManualLibrary}
				/>

				{/* Current task */}
				{currentTask && (
					<div className="border-b px-3 py-2 bg-muted/30">
						<div className="text-[10px] text-muted-foreground uppercase tracking-wide">Task</div>
						<div className="text-xs font-medium truncate" title={currentTask}>
							{currentTask}
						</div>
					</div>
				)}

				{/* History */}
				<div ref={historyRef} className="flex-1 overflow-y-auto p-3 space-y-2">
					{showEmptyState && <EmptyState />}

					{history.map((event, index) => (
						<EventCard key={index} event={event} />
					))}

					{pendingQuestion && (
						<div className="rounded-lg border border-blue-200 bg-blue-50 p-3 text-xs text-blue-950">
							<div className="mb-1 font-semibold">Agent 需要你补充信息</div>
							<div className="leading-relaxed">{pendingQuestion.question}</div>
							<div className="mt-2 text-blue-700">
								{isHandoverQuestion
									? '请直接在左侧网页中输入账号、密码或验证码；完成后点击底部继续按钮。'
									: '请在底部输入框回复，提交后会继续当前任务。'}
							</div>
						</div>
					)}

					{/* Activity indicator at bottom */}
					{activity && <ActivityCard activity={activity} />}
				</div>
			</main>

			{/* Input */}
			<footer className="border-t p-3">
				<InputGroup className="relative rounded-lg">
					<InputGroupTextarea
						ref={textareaRef}
						placeholder={inputPlaceholder}
						value={inputValue}
						onChange={(e) => setInputValue(e.target.value)}
						onKeyDown={handleKeyDown}
						readOnly={(isRunning && !isAnsweringQuestion) || isHandoverQuestion}
						className={isAnsweringQuestion ? 'text-xs pr-20 min-h-10' : 'text-xs pr-12 min-h-10'}
					/>
					<InputGroupAddon align="inline-end" className="absolute bottom-0 right-0 gap-1">
						{isAnsweringQuestion && (
							<InputGroupButton
								size="icon-sm"
								variant="destructive"
								onClick={handleStop}
								className="size-7 cursor-pointer"
								aria-label="Stop task"
								title="停止任务"
							>
								<Square className="size-3" />
							</InputGroupButton>
						)}
						{isRunning && !isAnsweringQuestion ? (
							<InputGroupButton
								size="icon-sm"
								variant="destructive"
								onClick={handleStop}
								className="size-7 cursor-pointer"
								aria-label="Stop task"
								title="Stop task"
							>
								<Square className="size-3" />
							</InputGroupButton>
						) : (
							<InputGroupButton
								size="icon-sm"
								variant="default"
								onClick={() => {
									if (isHandoverQuestion) {
										if (answerQuestion('我已完成页面接管，可以继续。')) {
											setInputValue('')
											setView({ name: 'chat' })
										}
										return
									}
									handleSubmit()
								}}
								disabled={!isHandoverQuestion && !inputValue.trim()}
								className="size-7 cursor-pointer"
								aria-label={
									isHandoverQuestion
										? 'Continue after handover'
										: pendingQuestion
											? 'Submit answer'
											: 'Send'
								}
								title={
									isHandoverQuestion ? '我已完成，继续' : pendingQuestion ? '提交回答' : '发送'
								}
							>
								<Send className="size-3" />
							</InputGroupButton>
						)}
					</InputGroupAddon>
				</InputGroup>
			</footer>
		</div>
	)
}

function CurrentSiteManualStatus({
	site,
	manualCount,
	status,
	backendConfigured,
	onOpen,
}: {
	site: string
	manualCount: number
	status: 'idle' | 'loading' | 'error'
	backendConfigured: boolean
	onOpen: () => void
}) {
	const summary = !backendConfigured
		? 'Memory backend not configured'
		: !site
			? 'No web tab detected'
			: status === 'loading'
				? 'Checking manuals...'
				: status === 'error'
					? 'Manual status unavailable'
					: `${manualCount} active ${manualCount === 1 ? 'manual' : 'manuals'}`

	return (
		<section className="border-b bg-muted/20 px-3 py-2">
			<div className="flex items-center justify-between gap-2">
				<div className="min-w-0">
					<div className="text-[10px] text-muted-foreground uppercase tracking-wide">
						Current Site
					</div>
					<div className="truncate text-xs font-medium" title={site || summary}>
						{site || summary}
					</div>
					{site && <div className="text-[11px] text-muted-foreground">{summary}</div>}
				</div>
				<Button
					type="button"
					variant="outline"
					size="sm"
					onClick={onOpen}
					className="h-7 shrink-0 gap-1 px-2 text-xs"
					aria-label="Open current site manual library"
					title="Open current site manual library"
				>
					<BookOpen className="size-3" />
					Open Library
				</Button>
			</div>
		</section>
	)
}

function isHttpUrl(value: string): boolean {
	return value.startsWith('http://') || value.startsWith('https://')
}
