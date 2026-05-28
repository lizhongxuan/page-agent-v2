import { ContinuationResolver } from '@page-agent/core'
import { Circle, History, Plus, Send, Settings, Square } from 'lucide-react'
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
import { startManualRecording, stopManualRecording } from '@/webops/recorder/manualRecordingClient'
import { WorkflowCandidateClient } from '@/webops/workflow/WorkflowCandidateClient'
import {
	type WorkflowCandidateSummary,
	WorkflowCandidateUploader,
} from '@/webops/workflow/WorkflowCandidateUploader'

import {
	buildSessionContinuationDecisionContext,
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

export default function App() {
	const [view, setView] = useState<View>({ name: 'chat' })
	const [inputValue, setInputValue] = useState('')
	const [manualRecordingStatus, setManualRecordingStatus] = useState<
		'idle' | 'recording' | 'uploading' | 'approving' | 'enabled'
	>('idle')
	const [manualRecordingError, setManualRecordingError] = useState('')
	const [pendingManualCandidate, setPendingManualCandidate] =
		useState<WorkflowCandidateSummary | null>(null)
	const historyRef = useRef<HTMLDivElement>(null)
	const textareaRef = useRef<HTMLTextAreaElement>(null)
	const continuationResolver = useMemo(() => new ContinuationResolver(), [])

	const {
		status,
		history,
		activity,
		currentTask,
		config,
		webOpsSession,
		pendingWorkflowCandidate,
		pendingQuestion,
		execute,
		answerQuestion,
		newSession,
		stop,
		configure,
	} = useAgent()

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
			const continuationContext = shouldContinueSession
				? buildSessionContinuationDecisionContext({
						previousTask,
						userMessage: normalizedTask,
						previousSession: webOpsSession,
					})
				: undefined

			execute(taskToExecute, {
				displayTask,
				carryHistory,
				continuationContext,
				continuationResolver,
			}).catch((error) => {
				console.error('[SidePanel] Failed to execute task:', error)
			})
		},
		[continuationResolver, currentTask, execute, history, status, webOpsSession]
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

	const handleToggleRecording = useCallback(async () => {
		if (!config?.knowledgeSettings?.enabled || !config.knowledgeSettings.baseUrl) {
			setManualRecordingError('请先在设置里启用项目知识库并填写后端 URL。')
			return
		}
		setManualRecordingError('')
		if (manualRecordingStatus !== 'recording') {
			const result = await startManualRecording(inputValue.trim() || 'Manual workflow recording')
			if (!result.ok) {
				setManualRecordingError(result.error)
				return
			}
			setPendingManualCandidate(null)
			setManualRecordingStatus('recording')
			return
		}

		setManualRecordingStatus('uploading')
		const result = await stopManualRecording()
		if (!result.ok || result.status !== 'stopped') {
			setManualRecordingStatus('idle')
			setManualRecordingError(!result.ok ? result.error : '录制停止失败。')
			return
		}
		const uploader = new WorkflowCandidateUploader({
			baseUrl: config.knowledgeSettings.baseUrl,
			apiKey: config.knowledgeSettings.apiKey || undefined,
		})
		const upload = await uploader.uploadManualSession(result.session)
		if (!upload.ok) {
			setManualRecordingStatus('idle')
			setManualRecordingError(upload.error)
			return
		}
		setPendingManualCandidate(upload.candidate)
		setManualRecordingStatus('idle')
	}, [config?.knowledgeSettings, inputValue, manualRecordingStatus])

	const handleApproveCandidate = useCallback(
		async (candidate: WorkflowCandidateSummary) => {
			if (!config?.knowledgeSettings?.baseUrl) return
			setManualRecordingStatus('approving')
			setManualRecordingError('')
			const client = new WorkflowCandidateClient({
				baseUrl: config.knowledgeSettings.baseUrl,
				apiKey: config.knowledgeSettings.apiKey || undefined,
			})
			const result = await client.approve(candidate.id)
			if (!result.ok) {
				setManualRecordingStatus('idle')
				setManualRecordingError(result.error)
				return
			}
			setPendingManualCandidate(result.candidate)
			setManualRecordingStatus('enabled')
		},
		[config?.knowledgeSettings]
	)

	const handleNewSession = useCallback(() => {
		if (status === 'running') return
		setInputValue('')
		setView({ name: 'chat' })
		newSession()
	}, [newSession, status])

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
					<Button
						variant={manualRecordingStatus === 'recording' ? 'destructive' : 'ghost'}
						size="icon-sm"
						onClick={handleToggleRecording}
						disabled={
							isRunning ||
							manualRecordingStatus === 'uploading' ||
							manualRecordingStatus === 'approving'
						}
						className="cursor-pointer"
						aria-label={
							manualRecordingStatus === 'recording' ? 'Stop recording' : 'Start recording'
						}
						title={manualRecordingStatus === 'recording' ? '停止录制' : '开始录制'}
					>
						{manualRecordingStatus === 'recording' ? (
							<Square className="size-3.5" />
						) : (
							<Circle className="size-3.5" />
						)}
					</Button>
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

				{/* History */}
				<div ref={historyRef} className="flex-1 overflow-y-auto p-3 space-y-2">
					{showEmptyState && <EmptyState />}

					{manualRecordingStatus === 'recording' && (
						<div className="rounded-lg border border-red-200 bg-red-50 p-3 text-xs text-red-950">
							<div className="font-semibold">正在录制手动流程</div>
							<div className="mt-1 text-red-700">
								请在当前网页完成操作，然后点击顶部方形按钮停止。
							</div>
						</div>
					)}

					{(pendingManualCandidate || pendingWorkflowCandidate) && (
						<div className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-950">
							<div className="font-semibold">发现可复用 workflow</div>
							<div className="mt-1 text-amber-700">
								录制结果已保存为候选，确认启用后才会用于后续相似任务回放。
							</div>
							<div className="mt-2 flex items-center gap-2">
								<Button
									size="sm"
									className="h-7 px-2 text-xs"
									disabled={
										manualRecordingStatus === 'approving' ||
										Boolean((pendingManualCandidate ?? pendingWorkflowCandidate)?.searchable)
									}
									onClick={() => {
										const candidate = pendingManualCandidate ?? pendingWorkflowCandidate
										if (candidate) void handleApproveCandidate(candidate)
									}}
								>
									{(pendingManualCandidate ?? pendingWorkflowCandidate)?.searchable
										? '已启用'
										: manualRecordingStatus === 'approving'
											? '启用中'
											: '确认启用'}
								</Button>
								<span className="text-[10px] text-amber-700">
									ID: {(pendingManualCandidate ?? pendingWorkflowCandidate)?.id}
								</span>
							</div>
						</div>
					)}

					{manualRecordingError && (
						<div className="rounded-lg border border-red-200 bg-red-50 p-3 text-xs text-red-950">
							{manualRecordingError}
						</div>
					)}

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
