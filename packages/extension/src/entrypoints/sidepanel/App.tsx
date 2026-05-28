import { Check, CircleDot, History, ListTree, Plus, Send, Settings, Square, X } from 'lucide-react'
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

import {
	buildSessionContinuationTask,
	formatSessionDisplayTask,
	getSessionContinuationBaseTask,
} from '../../agent/sessionContinuation'
import { useAgent } from '../../agent/useAgent'
import type { RecordedAction, RecordedSession } from '../../webops/recorder/actionEvents'
import {
	MANUAL_RECORDING_ACTIONS_STORAGE_KEY,
	MANUAL_RECORDING_STATE_STORAGE_KEY,
	type ManualRecordingState,
} from '../../webops/recorder/manualRecording'
import {
	WORKFLOW_TARGET_TAB_STORAGE_KEY,
	selectRecordableTab,
} from '../../webops/recorder/recordingTabs'
import { RetrievalClient } from '../../webops/workflow/RetrievalClient'
import { buildWorkflowCandidateRequest } from '../../webops/workflow/candidateReview'
import { inspectWorkflowRecipe } from '../../webops/workflow/recipeInspection'
import { PENDING_REPAIR_PATCH_STORAGE_KEY } from '../../webops/workflow/repairCandidate'
import { workflowReviewPresentation } from '../../webops/workflow/reviewPresentation'
import type {
	WorkflowCandidateReview,
	WorkflowRepairPatchCandidate,
} from '../../webops/workflow/types'
import { clearWorkflowSessionStorage } from '../../webops/workflow/workflowSessionState'

type View =
	| { name: 'chat' }
	| { name: 'config' }
	| { name: 'history' }
	| { name: 'history-detail'; sessionId: string }

interface RunTaskOptions {
	forceNewSession?: boolean
}

export default function App() {
	const [view, setView] = useState<View>({ name: 'chat' })
	const [inputValue, setInputValue] = useState('')
	const [manualRecording, setManualRecording] = useState<ManualRecordingState | null>(null)
	const [workflowCandidate, setWorkflowCandidate] = useState<WorkflowCandidateReview | null>(null)
	const [workflowStepsExpanded, setWorkflowStepsExpanded] = useState(false)
	const [repairPatchCandidate, setRepairPatchCandidate] =
		useState<WorkflowRepairPatchCandidate | null>(null)
	const [workflowReviewError, setWorkflowReviewError] = useState('')
	const [workflowReviewBusy, setWorkflowReviewBusy] = useState(false)
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
	const workflowBackend = config?.workflowBackend
	const workflowBackendConfigured = Boolean(workflowBackend?.baseUrl)

	useEffect(() => {
		chrome.storage.local.get(MANUAL_RECORDING_STATE_STORAGE_KEY).then((result) => {
			const state = result[MANUAL_RECORDING_STATE_STORAGE_KEY] as ManualRecordingState | undefined
			setManualRecording(state?.active ? state : null)
		})
		chrome.storage.local.get(PENDING_REPAIR_PATCH_STORAGE_KEY).then((result) => {
			const patch = result[PENDING_REPAIR_PATCH_STORAGE_KEY] as
				| WorkflowRepairPatchCandidate
				| undefined
			setRepairPatchCandidate(patch?.status === 'pending_review' ? patch : null)
		})
		const handleStorageChanged = (
			changes: Record<string, chrome.storage.StorageChange>,
			areaName: string
		) => {
			if (areaName !== 'local' || !changes[PENDING_REPAIR_PATCH_STORAGE_KEY]) return
			const patch = changes[PENDING_REPAIR_PATCH_STORAGE_KEY].newValue as
				| WorkflowRepairPatchCandidate
				| undefined
			setRepairPatchCandidate(patch ? patch : null)
		}
		chrome.storage.onChanged.addListener(handleStorageChanged)
		return () => chrome.storage.onChanged.removeListener(handleStorageChanged)
	}, [])

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
		setWorkflowCandidate(null)
		setWorkflowStepsExpanded(false)
		setRepairPatchCandidate(null)
		setWorkflowReviewError('')
		clearWorkflowSessionStorage(chrome.storage.local).catch((error) =>
			console.error('[SidePanel] Failed to clear workflow session storage:', error)
		)
		newSession()
	}, [newSession, status])

	const createWorkflowClient = useCallback(() => {
		if (!workflowBackend?.baseUrl) return undefined
		return new RetrievalClient({
			baseUrl: workflowBackend.baseUrl,
			bearerToken: workflowBackend.apiKey || undefined,
		})
	}, [workflowBackend?.apiKey, workflowBackend?.baseUrl])

	const handleStartRecording = useCallback(async () => {
		if (!workflowBackendConfigured) {
			setWorkflowReviewError('请先在 Settings 配置 Workflow Backend。')
			return
		}
		const [activeTabs, allTabs, stored] = await Promise.all([
			chrome.tabs.query({ active: true, currentWindow: true }),
			chrome.tabs.query({}),
			chrome.storage.local.get(WORKFLOW_TARGET_TAB_STORAGE_KEY),
		])
		const tab = selectRecordableTab(
			activeTabs,
			allTabs,
			chrome.runtime.getURL(''),
			stored[WORKFLOW_TARGET_TAB_STORAGE_KEY] as number | undefined
		)
		if (!tab?.url) {
			setWorkflowReviewError('当前标签页没有可录制的 URL。')
			return
		}
		const state: ManualRecordingState = {
			active: true,
			id: `manual_${Date.now()}`,
			task: inputValue.trim() || currentTask || '',
			startUrl: tab.url,
			startedAt: Date.now(),
		}
		await chrome.storage.local.set({
			[MANUAL_RECORDING_STATE_STORAGE_KEY]: state,
			[MANUAL_RECORDING_ACTIONS_STORAGE_KEY]: [],
			[WORKFLOW_TARGET_TAB_STORAGE_KEY]: tab.id,
		})
		setWorkflowCandidate(null)
		setWorkflowStepsExpanded(false)
		setWorkflowReviewError('')
		setManualRecording(state)
		setView({ name: 'chat' })
	}, [currentTask, inputValue, workflowBackendConfigured])

	const handleStopRecording = useCallback(async () => {
		const client = createWorkflowClient()
		const state = manualRecording
		if (!client || !state) return
		setWorkflowReviewBusy(true)
		setWorkflowReviewError('')
		try {
			const stored = await chrome.storage.local.get(MANUAL_RECORDING_ACTIONS_STORAGE_KEY)
			const steps = Array.isArray(stored[MANUAL_RECORDING_ACTIONS_STORAGE_KEY])
				? (stored[MANUAL_RECORDING_ACTIONS_STORAGE_KEY] as RecordedAction[])
				: []
			const session: RecordedSession = {
				id: state.id,
				task: state.task,
				startUrl: state.startUrl,
				startedAt: state.startedAt,
				endedAt: Date.now(),
				steps,
				knowledgeHits: [],
				redactionReport: steps
					.filter((step) => step.value === '[REDACTED]')
					.map((step) => ({ actionId: step.id, field: 'value' })),
			}
			await chrome.storage.local.remove([
				MANUAL_RECORDING_STATE_STORAGE_KEY,
				MANUAL_RECORDING_ACTIONS_STORAGE_KEY,
				WORKFLOW_TARGET_TAB_STORAGE_KEY,
			])
			setManualRecording(null)
			const request = buildWorkflowCandidateRequest(session, {
				projectId: workflowBackend?.projectId || 'default',
				source: 'user_demo',
			})
			if (!request.ok) {
				setWorkflowReviewError(request.message)
				return
			}
			const result = await client.createWorkflowCandidateFromSession(request.request)
			if (!result.ok) {
				setWorkflowReviewError(result.error.message)
				return
			}
			setWorkflowCandidate(result.data)
			setWorkflowStepsExpanded(false)
		} finally {
			setWorkflowReviewBusy(false)
		}
	}, [createWorkflowClient, manualRecording, workflowBackend?.projectId])

	const handleApproveCandidate = useCallback(async () => {
		const client = createWorkflowClient()
		if (!client || !workflowCandidate) return
		setWorkflowReviewBusy(true)
		setWorkflowReviewError('')
		try {
			const result = await client.approveWorkflowCandidate(workflowCandidate.id)
			if (!result.ok) {
				setWorkflowReviewError(result.error.message)
				return
			}
			setWorkflowCandidate(null)
			setWorkflowStepsExpanded(false)
		} finally {
			setWorkflowReviewBusy(false)
		}
	}, [createWorkflowClient, workflowCandidate])

	const handleRejectCandidate = useCallback(async () => {
		const client = createWorkflowClient()
		if (!client || !workflowCandidate) return
		setWorkflowReviewBusy(true)
		setWorkflowReviewError('')
		try {
			const result = await client.rejectWorkflowCandidate(workflowCandidate.id)
			if (!result.ok) {
				setWorkflowReviewError(result.error.message)
				return
			}
			setWorkflowCandidate(result.data)
			setWorkflowStepsExpanded(false)
		} finally {
			setWorkflowReviewBusy(false)
		}
	}, [createWorkflowClient, workflowCandidate])

	const handleApproveRepairPatch = useCallback(async () => {
		const client = createWorkflowClient()
		if (!client || !repairPatchCandidate?.id) return
		setWorkflowReviewBusy(true)
		setWorkflowReviewError('')
		try {
			const result = await client.approveRepairPatch(repairPatchCandidate.id)
			if (!result.ok) {
				setWorkflowReviewError(result.error.message)
				return
			}
			setRepairPatchCandidate(null)
			await chrome.storage.local.remove(PENDING_REPAIR_PATCH_STORAGE_KEY)
		} finally {
			setWorkflowReviewBusy(false)
		}
	}, [createWorkflowClient, repairPatchCandidate])

	const handleRejectRepairPatch = useCallback(async () => {
		const client = createWorkflowClient()
		if (!client || !repairPatchCandidate?.id) return
		setWorkflowReviewBusy(true)
		setWorkflowReviewError('')
		try {
			const result = await client.rejectRepairPatch(repairPatchCandidate.id)
			if (!result.ok) {
				setWorkflowReviewError(result.error.message)
				return
			}
			setRepairPatchCandidate(result.data)
			await chrome.storage.local.set({ [PENDING_REPAIR_PATCH_STORAGE_KEY]: result.data })
		} finally {
			setWorkflowReviewBusy(false)
		}
	}, [createWorkflowClient, repairPatchCandidate])

	const handleKeyDown = (e: React.KeyboardEvent) => {
		if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
			e.preventDefault()
			handleSubmit()
		}
	}

	const workflowInspection = useMemo(
		() => (workflowCandidate ? inspectWorkflowRecipe(workflowCandidate.recipeDraft) : null),
		[workflowCandidate]
	)

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
	const workflowCandidatePresentation = workflowCandidate
		? workflowReviewPresentation('workflow', workflowCandidate.status)
		: null
	const repairPatchPresentation = repairPatchCandidate
		? workflowReviewPresentation('repair', repairPatchCandidate.status)
		: null

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
				{/* Current task */}
				{currentTask && (
					<div className="border-b px-3 py-2 bg-muted/30">
						<div className="text-[10px] text-muted-foreground uppercase tracking-wide">Task</div>
						<div className="text-xs font-medium truncate" title={currentTask}>
							{currentTask}
						</div>
					</div>
				)}

				{workflowBackendConfigured && (
					<div className="border-b px-3 py-2 bg-muted/20 space-y-2">
						<div className="flex items-center justify-between gap-2">
							<div className="flex min-w-0 items-center gap-1.5 text-xs">
								<CircleDot
									className={
										manualRecording ? 'size-3 text-red-600' : 'size-3 text-muted-foreground'
									}
								/>
								<span className="truncate">
									{manualRecording ? '正在录制 workflow' : 'Playwright workflow'}
								</span>
							</div>
							{manualRecording ? (
								<Button
									variant="destructive"
									size="sm"
									onClick={handleStopRecording}
									disabled={workflowReviewBusy}
									className="h-7 gap-1 px-2 text-xs"
								>
									<Square className="size-3" />
									停止录制
								</Button>
							) : (
								<Button
									variant="outline"
									size="sm"
									onClick={handleStartRecording}
									disabled={isRunning || workflowReviewBusy}
									className="h-7 gap-1 px-2 text-xs"
								>
									<CircleDot className="size-3" />
									开始录制
								</Button>
							)}
						</div>
						{workflowCandidate && workflowCandidatePresentation && (
							<div className={workflowCandidatePresentation.cardClassName}>
								<div className="font-medium">{workflowCandidatePresentation.title}</div>
								<div className="mt-1 truncate" title={workflowCandidate.recipeDraft.intent}>
									{workflowCandidate.recipeDraft.name}
								</div>
								<div className="mt-2 flex flex-wrap items-center gap-2">
									<Button
										size="sm"
										onClick={handleApproveCandidate}
										disabled={workflowReviewBusy || workflowCandidate.status === 'active'}
										className="h-7 gap-1 px-2 text-xs"
									>
										<Check className="size-3" />
										确认启用
									</Button>
									<Button
										variant="outline"
										size="sm"
										onClick={handleRejectCandidate}
										disabled={workflowReviewBusy || workflowCandidate.status === 'rejected'}
										className="h-7 gap-1 px-2 text-xs"
									>
										<X className="size-3" />
										拒绝
									</Button>
									<Button
										variant="outline"
										size="sm"
										onClick={() => setWorkflowStepsExpanded((expanded) => !expanded)}
										className="h-7 gap-1 px-2 text-xs"
										aria-expanded={workflowStepsExpanded}
									>
										<ListTree className="size-3" />
										{workflowStepsExpanded ? '隐藏步骤' : '查看步骤'}
									</Button>
									<span className={workflowCandidatePresentation.statusClassName}>
										{workflowCandidate.status}
									</span>
								</div>
								{workflowStepsExpanded && workflowInspection && (
									<div className="mt-2 space-y-2 rounded-md border border-current/10 bg-background/70 p-2 text-[11px] text-foreground">
										<div className="flex flex-wrap gap-x-3 gap-y-1 text-muted-foreground">
											<span>{workflowInspection.stepCount} 个操作</span>
											{workflowInspection.variableNames.length > 0 && (
												<span>变量: {workflowInspection.variableNames.join(', ')}</span>
											)}
										</div>
										{workflowInspection.chunks.map((chunk) => (
											<div key={chunk.id} className="space-y-1">
												<div className="font-medium">{chunk.name}</div>
												<div className="text-[10px] text-muted-foreground">{chunk.pageState}</div>
												<ol className="space-y-1">
													{chunk.steps.map((step) => (
														<li
															key={step.id}
															className="grid grid-cols-[1.25rem_3.5rem_minmax(0,1fr)] items-start gap-1"
														>
															<span className="text-muted-foreground">{step.index}.</span>
															<span className="font-medium">{step.action}</span>
															<span className="min-w-0 break-words" title={step.target}>
																{step.target}
																{step.input && (
																	<span className="text-muted-foreground"> · {step.input}</span>
																)}
															</span>
														</li>
													))}
												</ol>
											</div>
										))}
									</div>
								)}
							</div>
						)}
						{repairPatchCandidate && repairPatchPresentation && (
							<div className={repairPatchPresentation.cardClassName}>
								<div className="font-medium">{repairPatchPresentation.title}</div>
								<div className="mt-1 truncate" title={repairPatchCandidate.newTargetSummary}>
									{repairPatchCandidate.newTargetSummary}
								</div>
								<div className="mt-2 flex gap-2">
									<Button
										size="sm"
										onClick={handleApproveRepairPatch}
										disabled={workflowReviewBusy || repairPatchCandidate.status === 'active'}
										className="h-7 gap-1 px-2 text-xs"
									>
										<Check className="size-3" />
										确认启用
									</Button>
									<Button
										variant="outline"
										size="sm"
										onClick={handleRejectRepairPatch}
										disabled={workflowReviewBusy || repairPatchCandidate.status === 'rejected'}
										className="h-7 gap-1 px-2 text-xs"
									>
										<X className="size-3" />
										拒绝
									</Button>
									<span className={repairPatchPresentation.statusClassName}>
										{repairPatchCandidate.status}
									</span>
								</div>
							</div>
						)}
						{workflowReviewError && (
							<div className="rounded-md border border-red-200 bg-red-50 p-2 text-xs text-red-700">
								{workflowReviewError}
							</div>
						)}
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
