// @vitest-environment jsdom
import { beforeEach, describe, expect, it } from 'vitest'

import { runBrowserWorkflowReplay } from './browserWorkflowReplay'
import type { WorkflowRecipe } from './types'

describe('runBrowserWorkflowReplay', () => {
	beforeEach(() => {
		window.location.hash = ''
	})

	it('executes click and fill steps against the current document', async () => {
		document.body.innerHTML = `
			<a href="#issues">Issues</a>
			<input placeholder="Search all issues" />
		`

		const result = await runBrowserWorkflowReplay(workflowRecipe(), { query: 'timeout error' })

		expect(result.ok).toBe(true)
		expect(window.location.hash).toBe('#issues')
		expect(document.querySelector<HTMLInputElement>('input')?.value).toBe('timeout error')
	})

	it('reports the failed chunk and step when a target cannot be resolved', async () => {
		document.body.innerHTML = `<button>Code</button>`
		const recipe = workflowRecipe()
		recipe.chunks[0]!.steps[0]!.target = {
			primary: { strategy: 'role', role: 'link', name: 'Issues' },
		}

		const result = await runBrowserWorkflowReplay(recipe)

		expect(result).toMatchObject({
			ok: false,
			failedChunkId: 'chunk_1',
			failedStepId: 'click_issues',
		})
	})

	it('dismisses cookie preference dialogs before replaying a step', async () => {
		document.body.innerHTML = `
			<div role="dialog" aria-label="Manage cookie preferences">
				<h2>Manage cookie preferences</h2>
				<button id="save-cookie-preferences">Save changes</button>
			</div>
			<a href="#issues">Issues</a>
			<input placeholder="Search all issues" />
		`
		document.getElementById('save-cookie-preferences')?.addEventListener('click', () => {
			document.querySelector('[role="dialog"]')?.remove()
		})

		const result = await runBrowserWorkflowReplay(workflowRecipe(), { query: 'timeout error' })

		expect(result.ok).toBe(true)
		expect(document.querySelector('[role="dialog"]')).toBeNull()
		expect(window.location.hash).toBe('#issues')
	})

	it('skips disabled dialog buttons and uses the close button instead', async () => {
		document.body.innerHTML = `
			<div role="dialog" aria-label="Manage cookie preferences">
				<h2>Manage cookie preferences</h2>
				<button disabled>Save changes</button>
				<button aria-label="Close">×</button>
			</div>
			<a href="#issues">Issues</a>
			<input placeholder="Search all issues" />
		`
		document.querySelector('[aria-label="Close"]')?.addEventListener('click', () => {
			document.querySelector('[role="dialog"]')?.remove()
		})

		const result = await runBrowserWorkflowReplay(workflowRecipe(), { query: 'timeout error' })

		expect(result.ok).toBe(true)
		expect(document.querySelector('[role="dialog"]')).toBeNull()
		expect(window.location.hash).toBe('#issues')
	})

	it('fails instead of reporting completion when a blocking dialog cannot be dismissed', async () => {
		document.body.innerHTML = `
			<div role="dialog" aria-label="Manage cookie preferences">
				<h2>Manage cookie preferences</h2>
				<button disabled>Save changes</button>
			</div>
			<a href="#issues">Issues</a>
			<input placeholder="Search all issues" />
		`

		const result = await runBrowserWorkflowReplay(workflowRecipe(), { query: 'timeout error' })

		expect(result).toMatchObject({
			ok: false,
			failedStepId: 'click_issues',
		})
		expect(window.location.hash).not.toBe('#issues')
	})

	it('does not use unsafe generic css fallbacks for click targets', async () => {
		document.body.innerHTML = `
			<span id="unrelated-cookie-link">Manage cookies</span>
			<a href="#code">Code</a>
		`
		let unrelatedClicked = false
		document.getElementById('unrelated-cookie-link')?.addEventListener('click', () => {
			unrelatedClicked = true
		})
		const recipe = workflowRecipe()
		recipe.chunks[0]!.steps[0]!.target = {
			primary: { strategy: 'text', value: 'Missing nav item' },
			fallbacks: [{ strategy: 'css', value: 'span' }],
		}

		const result = await runBrowserWorkflowReplay(recipe)

		expect(result).toMatchObject({
			ok: false,
			failedStepId: 'click_issues',
		})
		expect(unrelatedClicked).toBe(false)
	})
})

function workflowRecipe(): WorkflowRecipe {
	return {
		id: 'wf_github_issue_search',
		version: 1,
		projectId: 'default',
		status: 'active',
		searchable: true,
		site: 'github.com',
		name: 'Search issues',
		intent: 'Search issues',
		riskLevel: 'read_or_search',
		requiresConfirmation: false,
		variables: [],
		chunks: [
			{
				id: 'chunk_1',
				name: 'Search',
				riskLevel: 'read_or_search',
				steps: [
					{
						id: 'click_issues',
						type: 'click',
						target: { primary: { strategy: 'role', role: 'link', name: 'Issues' } },
						riskLevel: 'read_or_search',
					},
					{
						id: 'fill_query',
						type: 'fill',
						target: { primary: { strategy: 'placeholder', value: 'Search all issues' } },
						value: '{{query}}',
						riskLevel: 'read_or_search',
					},
				],
			},
		],
	}
}
