import { describe, expect, it } from 'vitest'

import { inspectWorkflowRecipe } from './recipeInspection'
import type { WorkflowRecipe } from './types'

describe('inspectWorkflowRecipe', () => {
	it('formats recorded workflow steps into readable inspection rows', () => {
		const inspection = inspectWorkflowRecipe(workflowRecipe())

		expect(inspection.stepCount).toBe(3)
		expect(inspection.variableNames).toEqual(['query', 'repo'])
		expect(inspection.chunks[0]).toMatchObject({
			name: 'GitHub issue search',
			pageState: 'github_repo_home -> github_issues_list',
		})
		expect(inspection.chunks[0]?.steps).toEqual([
			expect.objectContaining({
				index: 1,
				action: 'click',
				target: 'role link "Issues" (+1 fallback)',
			}),
			expect.objectContaining({
				index: 2,
				action: 'fill',
				target: 'role searchbox "Search all issues" (+1 fallback)',
				input: '{{query}}',
			}),
			expect.objectContaining({
				index: 3,
				action: 'press',
				target: 'role searchbox "Search all issues"',
				input: 'Enter',
			}),
		])
	})
})

function workflowRecipe(): WorkflowRecipe {
	return {
		id: 'wf_github_issue_search',
		version: 1,
		projectId: 'default',
		status: 'pending_review',
		searchable: false,
		site: 'github.com',
		name: 'Replay GitHub issue search',
		intent: 'Search issues in a GitHub repository',
		riskLevel: 'read_or_search',
		requiresConfirmation: false,
		variables: [
			{ name: 'repo', type: 'string', required: true, source: 'task_or_url' },
			{ name: 'query', type: 'string', required: true, source: 'task' },
		],
		chunks: [
			{
				id: 'chunk_1',
				name: 'GitHub issue search',
				fromPageState: 'github_repo_home',
				toPageState: 'github_issues_list',
				riskLevel: 'read_or_search',
				variableNames: ['query'],
				steps: [
					{
						id: 'step_1',
						type: 'click',
						riskLevel: 'read_or_search',
						target: {
							primary: { strategy: 'role', role: 'link', name: 'Issues' },
							fallbacks: [{ strategy: 'text', value: 'Issues' }],
						},
					},
					{
						id: 'step_2',
						type: 'fill',
						value: '{{query}}',
						riskLevel: 'read_or_search',
						target: {
							primary: {
								strategy: 'role',
								role: 'searchbox',
								name: 'Search all issues',
							},
							fallbacks: [{ strategy: 'css', value: 'input[name="q"]' }],
						},
					},
					{
						id: 'step_3',
						type: 'press',
						key: 'Enter',
						riskLevel: 'read_or_search',
						target: {
							primary: {
								strategy: 'role',
								role: 'searchbox',
								name: 'Search all issues',
							},
						},
					},
				],
			},
		],
	}
}
