import { describe, expect, it } from 'vitest'

import { workflowReviewPresentation } from './reviewPresentation'

describe('workflowReviewPresentation', () => {
	it('uses distinct visual states for pending and active workflow candidates', () => {
		expect(workflowReviewPresentation('workflow', 'pending_review')).toMatchObject({
			title: '发现可复用 workflow',
			cardClassName: expect.stringContaining('amber'),
		})
		expect(workflowReviewPresentation('workflow', 'active')).toMatchObject({
			title: '已启用 workflow',
			cardClassName: expect.stringContaining('green'),
		})
	})

	it('uses distinct visual states for pending and active repair patches', () => {
		expect(workflowReviewPresentation('repair', 'pending_review')).toMatchObject({
			title: '发现回放修复规则',
			cardClassName: expect.stringContaining('blue'),
		})
		expect(workflowReviewPresentation('repair', 'active')).toMatchObject({
			title: '已启用回放修复规则',
			cardClassName: expect.stringContaining('green'),
		})
	})
})
