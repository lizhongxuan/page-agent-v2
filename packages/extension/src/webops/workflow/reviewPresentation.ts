import type { WorkflowReviewStatus } from './types'

export type WorkflowReviewKind = 'workflow' | 'repair'

export interface WorkflowReviewPresentation {
	title: string
	statusClassName: string
	cardClassName: string
}

export function workflowReviewPresentation(
	kind: WorkflowReviewKind,
	status: WorkflowReviewStatus | undefined
): WorkflowReviewPresentation {
	if (status === 'active') {
		return {
			title: kind === 'repair' ? '已启用回放修复规则' : '已启用 workflow',
			cardClassName: 'rounded-md border border-green-200 bg-green-50 p-2 text-xs text-green-950',
			statusClassName: 'self-center text-[10px] text-green-700',
		}
	}
	if (status === 'rejected') {
		return {
			title: kind === 'repair' ? '已拒绝回放修复规则' : '已拒绝 workflow',
			cardClassName: 'rounded-md border border-muted bg-muted/40 p-2 text-xs text-muted-foreground',
			statusClassName: 'self-center text-[10px] text-muted-foreground',
		}
	}
	if (kind === 'repair') {
		return {
			title: '发现回放修复规则',
			cardClassName: 'rounded-md border border-blue-200 bg-blue-50 p-2 text-xs text-blue-950',
			statusClassName: 'self-center text-[10px] text-blue-700',
		}
	}
	return {
		title: '发现可复用 workflow',
		cardClassName: 'rounded-md border border-amber-200 bg-amber-50 p-2 text-xs text-amber-950',
		statusClassName: 'self-center text-[10px] text-amber-700',
	}
}
