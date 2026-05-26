export interface SessionContinuationInput {
	previousTask: string
	userMessage: string
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
	return `${previousTask}\n补充：${userMessage}`
}
