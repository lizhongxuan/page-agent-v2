import { describe, expect, it, vi } from 'vitest'

import { WORKFLOW_TARGET_TAB_STORAGE_KEY } from '../recorder/recordingTabs'
import {
	clearWorkflowSessionStorage,
	formatReplayConfirmationQuestion,
	isReplayConfirmationApproved,
} from './workflowSessionState'

describe('workflow session state helpers', () => {
	it('clears stale workflow target state when a chat session is reset', async () => {
		const remove = vi.fn().mockResolvedValue(undefined)

		await clearWorkflowSessionStorage({ remove })

		expect(remove).toHaveBeenCalledWith([WORKFLOW_TARGET_TAB_STORAGE_KEY])
	})

	it('parses explicit replay confirmation answers', () => {
		expect(isReplayConfirmationApproved('确认执行')).toBe(true)
		expect(isReplayConfirmationApproved('yes, run it')).toBe(true)
		expect(isReplayConfirmationApproved('取消')).toBe(false)
		expect(isReplayConfirmationApproved('不要执行')).toBe(false)
	})

	it('formats a confirmation question with workflow evidence', () => {
		const question = formatReplayConfirmationQuestion({
			workflowId: 'wf_github_issue_search',
			version: 3,
			confidence: 0.76,
			bindings: { repo: 'browser-use/workflow-use' },
			reason: 'workflow semantic score matched',
		})

		expect(question).toContain('wf_github_issue_search')
		expect(question).toContain('0.76')
		expect(question).toContain('repo=browser-use/workflow-use')
	})
})
