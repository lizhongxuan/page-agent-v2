import type {
	BusinessObjectRef,
	ContinuationDecision,
	HistoricalEvent,
	ResolveContinuationInput,
} from '@page-agent/core'

export interface SessionContinuationInput {
	previousTask: string
	userMessage: string
}

export type SessionContinuationDecisionContext = Omit<ResolveContinuationInput, 'userMessage'> & {
	userMessage?: string
}

export interface ContinuationResolverLike {
	resolve(input: ResolveContinuationInput): ContinuationDecision
}

export interface ResolvedSessionContinuationInput {
	task: string
	displayTask?: string
	carryHistory?: HistoricalEvent[]
	context?: SessionContinuationDecisionContext
	decision?: ContinuationDecision
	resolver?: ContinuationResolverLike
}

export interface ResolvedSessionContinuation {
	task: string
	displayTask?: string
	carryHistory?: HistoricalEvent[]
	decision?: ContinuationDecision
}

export function buildSessionContinuationTask({
	previousTask,
	userMessage,
}: SessionContinuationInput): string {
	return [
		'继续当前浏览器会话，不要把下面的用户输入当成完全独立的新任务。',
		'',
		`上一轮会话任务：${previousTask}`,
		`用户新的补充要求：${userMessage}`,
		'',
		'请结合当前浏览器页面状态、已有步骤历史和用户新的补充要求继续工作。',
		'如果用户只是要求“多看一点 / 多搜索一下 / 继续找”，请在合理范围内补充查看，不要无上限循环翻页。',
	].join('\n')
}

export function formatSessionDisplayTask(previousTask: string, userMessage: string): string {
	return `${getSessionContinuationBaseTask(previousTask)}\n补充：${userMessage}`
}

export function getSessionContinuationBaseTask(task: string): string {
	return task.split(/\n补充：/)[0]?.trim() || task.trim()
}

export function toContinuationResolverInput(
	context: SessionContinuationDecisionContext
): ResolveContinuationInput {
	return {
		previousTask: context.previousTask,
		userMessage: context.userMessage ?? '',
		pendingQuestion: context.pendingQuestion,
		currentUrl: context.currentUrl,
		currentTitle: context.currentTitle,
		previousUrl: context.previousUrl,
		previousTitle: context.previousTitle,
		previousBusinessObjects: context.previousBusinessObjects as BusinessObjectRef[] | undefined,
		hasUnconfirmedRisk: context.hasUnconfirmedRisk,
	}
}

export function buildResolvedSessionContinuation({
	task,
	displayTask,
	carryHistory,
	context,
	decision,
	resolver,
}: ResolvedSessionContinuationInput): ResolvedSessionContinuation {
	const resolvedDecision = decision ?? resolveContinuationDecision(context, resolver)
	if (!resolvedDecision) {
		return { task, displayTask, carryHistory }
	}

	if (
		resolvedDecision.mode === 'new_task_same_page' ||
		resolvedDecision.mode === 'fresh_task' ||
		resolvedDecision.requiresUserConfirmation
	) {
		const freshTask = context?.userMessage?.trim() || task
		return {
			task: freshTask,
			displayTask: freshTask,
			decision: resolvedDecision,
		}
	}

	return {
		task,
		displayTask,
		carryHistory,
		decision: resolvedDecision,
	}
}

function resolveContinuationDecision(
	context: SessionContinuationDecisionContext | undefined,
	resolver: ContinuationResolverLike | undefined
): ContinuationDecision | undefined {
	if (!context || !resolver) return undefined
	return resolver.resolve(toContinuationResolverInput(context))
}
