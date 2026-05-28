import { WORKFLOW_TARGET_TAB_STORAGE_KEY } from '../recorder/recordingTabs'

export interface WorkflowStorageLike {
	remove(keys: string[]): Promise<void>
}

export async function clearWorkflowSessionStorage(storage: WorkflowStorageLike): Promise<void> {
	await storage.remove([WORKFLOW_TARGET_TAB_STORAGE_KEY])
}

export interface ReplayConfirmationQuestionInput {
	workflowId: string
	version: number
	confidence?: number
	bindings: Record<string, string>
	reason?: string
}

export function formatReplayConfirmationQuestion(input: ReplayConfirmationQuestionInput): string {
	const confidence = typeof input.confidence === 'number' ? input.confidence.toFixed(2) : 'unknown'
	const bindings =
		Object.entries(input.bindings)
			.map(([key, value]) => `${key}=${value}`)
			.join(', ') || 'none'
	const reason = input.reason ? `\n匹配原因：${input.reason}` : ''
	return [
		`发现一个可能可用的 Playwright workflow，需要你确认后再执行。`,
		`Workflow：${input.workflowId} v${input.version}`,
		`置信度：${confidence}`,
		`变量：${bindings}${reason}`,
		`回复“确认”执行，回复“取消”跳过并改用普通 PageAgent。`,
	].join('\n')
}

export function isReplayConfirmationApproved(answer: string): boolean {
	const normalized = answer.trim().toLowerCase()
	if (!normalized) return false
	if (/(取消|拒绝|不要|不用|否|不执行|cancel|no|skip|stop)/i.test(normalized)) {
		return false
	}
	return /(确认|执行|继续|可以|同意|yes|y|ok|run|proceed|confirm)/i.test(normalized)
}
