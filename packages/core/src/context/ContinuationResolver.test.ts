import { describe, expect, it } from 'vitest'

import { ContinuationResolver } from './ContinuationResolver'

describe('ContinuationResolver', () => {
	const resolver = new ContinuationResolver()

	it('continues original task when user answers pending question', () => {
		const decision = resolver.resolve({
			previousTask: 'Find failing PG instance',
			userMessage: 'yes, continue',
			pendingQuestion: 'Should I continue checking this instance?',
			currentUrl: 'https://example.com/pg/pg-1',
			currentTitle: 'PG pg-1',
			previousBusinessObjects: [{ type: 'pg', id: 'pg-1', evidence: ['PG pg-1'] }],
		})

		expect(decision.mode).toBe('continue_original_task')
		expect(decision.inheritedSections).toContain('recentSteps')
	})

	it('starts a new task on the same page when intent changes', () => {
		const decision = resolver.resolve({
			previousTask: 'Find failing PG instance',
			userMessage: 'summarize what buttons are on this page',
			currentUrl: 'https://example.com/pg/pg-1',
			currentTitle: 'PG pg-1',
			previousUrl: 'https://example.com/pg/pg-1',
			previousTitle: 'PG pg-1',
			previousBusinessObjects: [{ type: 'pg', id: 'pg-1', evidence: ['PG pg-1'] }],
		})

		expect(decision.mode).toBe('new_task_same_page')
		expect(decision.inheritedSections).toEqual(['currentPage', 'instructions'])
	})

	it('does not treat weak same-page references as continuation when the user asks a new question', () => {
		const decision = resolver.resolve({
			previousTask: '检查 PG pg-1 的故障',
			userMessage: '总结这个页面有哪些按钮',
			currentUrl: 'https://example.com/pg/pg-1',
			currentTitle: 'PG pg-1',
			previousUrl: 'https://example.com/pg/pg-1',
			previousTitle: 'PG pg-1',
			previousBusinessObjects: [{ type: 'pg', id: 'pg-1', evidence: ['PG pg-1'] }],
		})

		expect(decision.mode).toBe('new_task_same_page')
	})

	it('inherits business facts when page changes but object is the same', () => {
		const decision = resolver.resolve({
			previousTask: 'Diagnose PG pg-1 error',
			userMessage: 'continue checking pg-1 logs here',
			currentUrl: 'https://example.com/logs',
			currentTitle: 'Logs',
			previousUrl: 'https://example.com/pg/pg-1',
			previousTitle: 'PG pg-1',
			previousBusinessObjects: [
				{ type: 'pg', id: 'pg-1', aliases: ['pg-1'], evidence: ['PG pg-1'] },
			],
		})

		expect(decision.mode).toBe('same_business_new_page')
		expect(decision.inheritedSections).toContain('businessObjects')
		expect(decision.discardedSections).toContain('recentSteps')
	})

	it('starts fresh when page and problem both change', () => {
		const decision = resolver.resolve({
			previousTask: 'Diagnose PG pg-1 error',
			userMessage: 'find invoice abc',
			currentUrl: 'https://example.com/billing',
			currentTitle: 'Billing',
			previousUrl: 'https://example.com/pg/pg-1',
			previousTitle: 'PG pg-1',
			previousBusinessObjects: [{ type: 'pg', id: 'pg-1', evidence: ['PG pg-1'] }],
		})

		expect(decision.mode).toBe('fresh_task')
		expect(decision.inheritedSections).toEqual(['currentPage', 'instructions'])
	})

	it('requires confirmation before resuming unconfirmed high-risk context', () => {
		const decision = resolver.resolve({
			previousTask: 'Delete stale records',
			userMessage: 'continue',
			currentUrl: 'https://example.com/admin',
			currentTitle: 'Admin',
			hasUnconfirmedRisk: true,
		})

		expect(decision.mode).toBe('ask_user_to_confirm')
		expect(decision.requiresUserConfirmation).toBe(true)
	})
})
