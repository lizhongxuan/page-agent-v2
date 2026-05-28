import { describe, expect, it, vi } from 'vitest'

import { WorkflowReplayService } from './WorkflowReplayService'
import type { WorkflowPageObservation } from './types'

const observation: WorkflowPageObservation = {
	title: 'microsoft/playwright',
	visibleText: ['Code', 'Issues', 'Pull requests'],
	controls: [{ role: 'link', name: 'Issues' }],
}

describe('WorkflowReplayService', () => {
	it('searches, selects, and starts a replay run', async () => {
		const client = {
			searchWorkflows: vi.fn(async () => ({
				ok: true as const,
				data: {
					currentPageState: 'github_repo_home',
					candidates: [
						{
							workflowId: 'wf_github_issue_search',
							version: 3,
							finalScore: 0.9,
							riskLevel: 'read_or_search' as const,
							variables: ['repo', 'query'],
							reasons: ['site matched'],
						},
					],
				},
			})),
			selectWorkflow: vi.fn(async () => ({
				ok: true as const,
				data: {
					decision: 'replay' as const,
					selectedWorkflowId: 'wf_github_issue_search',
					version: 3,
					bindings: {
						repo: { value: 'microsoft/playwright', source: 'slot', confidence: 0.95 },
						query: 'timeout',
					},
				},
			})),
			startRun: vi.fn(async () => ({
				ok: true as const,
				data: {
					runId: 'run_1',
					status: 'succeeded' as const,
					workflowId: 'wf_github_issue_search',
					version: 3,
				},
			})),
		}
		const statuses: string[] = []
		const service = new WorkflowReplayService({
			client,
			projectId: 'default',
			onStatus: (event) => statuses.push(event.kind),
		})

		const result = await service.tryReplay({
			task: '在 github.com/microsoft/playwright 的 Issues 里搜索 timeout 报错',
			currentUrl: 'https://github.com/microsoft/playwright',
			pageObservation: observation,
		})

		expect(result.status).toBe('completed')
		expect(client.searchWorkflows).toHaveBeenCalledOnce()
		expect(client.selectWorkflow).toHaveBeenCalledOnce()
		expect(client.startRun).toHaveBeenCalledOnce()
		expect(client.startRun).toHaveBeenCalledWith(
			expect.objectContaining({
				bindings: { repo: 'microsoft/playwright', query: 'timeout' },
			})
		)
		expect(statuses).toEqual([
			'search_started',
			'matched_workflow',
			'bindings_ready',
			'replay_started',
			'completed',
		])
		expect(client.searchWorkflows).toHaveBeenCalledWith(expect.objectContaining({ limit: 20 }))
	})

	it('asks for confirmation before executing a medium-confidence selected workflow', async () => {
		const client = {
			searchWorkflows: vi.fn(async () => ({
				ok: true as const,
				data: {
					currentPageState: 'github_repo_home',
					candidates: [
						{
							workflowId: 'wf_github_issue_search',
							version: 3,
							finalScore: 0.58,
							riskLevel: 'read_or_search' as const,
							variables: ['repo'],
							reasons: ['workflow semantic score matched'],
						},
					],
				},
			})),
			selectWorkflow: vi.fn(async () => ({
				ok: true as const,
				data: {
					decision: 'replay' as const,
					selectedWorkflowId: 'wf_github_issue_search',
					version: 3,
					confidence: 0.76,
					needsUserConfirmation: true,
					bindings: {
						repo: { value: 'microsoft/playwright', source: 'slot', confidence: 0.95 },
					},
				},
			})),
			startRun: vi.fn(async () => ({
				ok: true as const,
				data: {
					runId: 'run_confirmed',
					status: 'succeeded' as const,
					workflowId: 'wf_github_issue_search',
					version: 3,
				},
			})),
		}
		const confirmReplay = vi.fn(async () => true)
		const service = new WorkflowReplayService({
			client,
			projectId: 'default',
			confirmReplay,
		})

		const result = await service.tryReplay({
			task: 'open repository issues',
			currentUrl: 'https://github.com/microsoft/playwright',
			pageObservation: observation,
		})

		expect(result.status).toBe('completed')
		expect(confirmReplay).toHaveBeenCalledWith(
			expect.objectContaining({
				workflowId: 'wf_github_issue_search',
				version: 3,
				confidence: 0.76,
			})
		)
		expect(client.startRun).toHaveBeenCalledOnce()
	})

	it('falls back without executing when the user rejects replay confirmation', async () => {
		const client = {
			searchWorkflows: vi.fn(async () => ({
				ok: true as const,
				data: {
					currentPageState: 'github_repo_home',
					candidates: [
						{
							workflowId: 'wf_github_issue_search',
							version: 3,
							finalScore: 0.58,
							riskLevel: 'read_or_search' as const,
							variables: ['repo'],
							reasons: ['workflow semantic score matched'],
						},
					],
				},
			})),
			selectWorkflow: vi.fn(async () => ({
				ok: true as const,
				data: {
					decision: 'replay' as const,
					selectedWorkflowId: 'wf_github_issue_search',
					version: 3,
					confidence: 0.76,
					needsUserConfirmation: true,
					bindings: {
						repo: { value: 'microsoft/playwright', source: 'slot', confidence: 0.95 },
					},
				},
			})),
			startRun: vi.fn(),
		}
		const service = new WorkflowReplayService({
			client,
			projectId: 'default',
			confirmReplay: vi.fn(async () => false),
		})

		const result = await service.tryReplay({
			task: 'open repository issues',
			currentUrl: 'https://github.com/microsoft/playwright',
			pageObservation: observation,
		})

		expect(result).toMatchObject({ status: 'fallback', reason: 'confirmation_rejected' })
		expect(client.startRun).not.toHaveBeenCalled()
	})

	it('executes the selected active workflow in the current tab when a local replay executor is configured', async () => {
		const client = {
			searchWorkflows: vi.fn(async () => ({
				ok: true as const,
				data: {
					currentPageState: 'github_repo_home',
					candidates: [
						{
							workflowId: 'wf_github_issue_search',
							version: 3,
							finalScore: 0.9,
							riskLevel: 'read_or_search' as const,
							variables: ['repo', 'query'],
							reasons: ['site matched'],
						},
					],
				},
			})),
			selectWorkflow: vi.fn(async () => ({
				ok: true as const,
				data: {
					decision: 'replay' as const,
					selectedWorkflowId: 'wf_github_issue_search',
					version: 3,
					bindings: {
						repo: { value: 'microsoft/playwright', source: 'slot', confidence: 0.95 },
						query: 'timeout',
					},
				},
			})),
			listWorkflowCandidates: vi.fn(async () => ({
				ok: true as const,
				data: {
					candidates: [
						{
							id: 'cand_1',
							projectId: 'default',
							source: 'user_demo',
							task: 'Search GitHub issues',
							startUrl: 'https://github.com/microsoft/playwright',
							status: 'active' as const,
							searchable: true,
							recipeDraft: {
								id: 'wf_github_issue_search',
								version: 3,
								projectId: 'default',
								status: 'active' as const,
								searchable: true,
								site: 'github.com',
								name: 'Search GitHub issues',
								intent: 'Search GitHub issues',
								riskLevel: 'read_or_search' as const,
								requiresConfirmation: false,
								variables: [],
								chunks: [],
							},
						},
					],
				},
			})),
			startRun: vi.fn(),
		}
		const executeReplay = vi.fn(async () => ({
			ok: true as const,
		}))
		const service = new WorkflowReplayService({
			client,
			projectId: 'default',
			executeReplay,
		})

		const result = await service.tryReplay({
			task: 'search timeout',
			currentUrl: 'https://github.com/microsoft/playwright',
			pageObservation: observation,
		})

		expect(result).toMatchObject({
			status: 'completed',
			workflowId: 'wf_github_issue_search',
			version: 3,
		})
		expect(client.listWorkflowCandidates).toHaveBeenCalledWith({
			projectId: 'default',
			reviewStatus: 'active',
		})
		expect(executeReplay).toHaveBeenCalledWith(
			expect.objectContaining({
				bindings: { repo: 'microsoft/playwright', query: 'timeout' },
				currentUrl: 'https://github.com/microsoft/playwright',
				workflow: expect.objectContaining({ id: 'wf_github_issue_search' }),
			})
		)
		expect(client.startRun).not.toHaveBeenCalled()
	})

	it('falls back when no workflow candidates match', async () => {
		const service = new WorkflowReplayService({
			projectId: 'default',
			client: {
				searchWorkflows: vi.fn(async () => ({ ok: true as const, data: { candidates: [] } })),
				selectWorkflow: vi.fn(),
				startRun: vi.fn(),
			},
		})

		const result = await service.tryReplay({
			task: 'unknown task',
			currentUrl: 'https://example.com',
			pageObservation: observation,
		})

		expect(result).toMatchObject({ status: 'fallback', reason: 'no_match' })
	})

	it('fails closed when the backend is unavailable', async () => {
		const service = new WorkflowReplayService({
			projectId: 'default',
			client: {
				searchWorkflows: vi.fn(async () => ({
					ok: false as const,
					error: { code: 'network_error' as const, message: 'Failed to fetch', retryable: true },
				})),
				selectWorkflow: vi.fn(),
				startRun: vi.fn(),
			},
		})

		const result = await service.tryReplay({
			task: 'search timeout',
			currentUrl: 'https://github.com/microsoft/playwright',
			pageObservation: observation,
		})

		expect(result).toMatchObject({ status: 'fallback', reason: 'backend_unavailable' })
	})

	it('falls back when selector rejects unsafe risk', async () => {
		const service = new WorkflowReplayService({
			projectId: 'default',
			client: {
				searchWorkflows: vi.fn(async () => ({
					ok: true as const,
					data: {
						currentPageState: 'github_repo_home',
						candidates: [
							{
								workflowId: 'wf_send_email',
								version: 1,
								finalScore: 0.86,
								riskLevel: 'external_send' as const,
								variables: ['recipient'],
								reasons: [],
							},
						],
					},
				})),
				selectWorkflow: vi.fn(async () => ({
					ok: true as const,
					data: {
						decision: 'unsafe_risk' as const,
						rejectReason: 'Risk requires confirmation.',
					},
				})),
				startRun: vi.fn(),
			},
		})

		const result = await service.tryReplay({
			task: 'send the issue list by email',
			currentUrl: 'https://github.com/microsoft/playwright',
			pageObservation: observation,
		})

		expect(result).toMatchObject({ status: 'fallback', reason: 'unsafe_risk' })
	})

	it('falls back when variable binding fails', async () => {
		const service = new WorkflowReplayService({
			projectId: 'default',
			client: {
				searchWorkflows: vi.fn(async () => ({
					ok: true as const,
					data: {
						candidates: [
							{
								workflowId: 'wf_github_issue_search',
								version: 3,
								finalScore: 0.9,
								riskLevel: 'read_or_search' as const,
								variables: ['repo', 'query'],
								reasons: [],
							},
						],
					},
				})),
				selectWorkflow: vi.fn(async () => ({
					ok: false as const,
					error: {
						code: 'binding_failed' as const,
						message: 'required variable missing: query',
						retryable: false,
					},
				})),
				startRun: vi.fn(),
			},
		})

		const result = await service.tryReplay({
			task: 'open issues',
			currentUrl: 'https://github.com/microsoft/playwright',
			pageObservation: observation,
		})

		expect(result).toMatchObject({ status: 'fallback', reason: 'binding_failed' })
	})

	it('searches repair patches before PageAgent fallback when replay fails', async () => {
		const client = {
			searchWorkflows: vi.fn(async () => ({
				ok: true as const,
				data: {
					currentPageState: 'github_issues_list',
					candidateSlots: { repo: 'microsoft/playwright', query: 'timeout' },
					candidates: [
						{
							workflowId: 'wf_github_issue_search',
							version: 3,
							finalScore: 0.9,
							riskLevel: 'read_or_search' as const,
							variables: ['repo', 'query'],
							reasons: [],
						},
					],
				},
			})),
			selectWorkflow: vi.fn(async () => ({
				ok: true as const,
				data: {
					decision: 'replay' as const,
					selectedWorkflowId: 'wf_github_issue_search',
					version: 3,
					bindings: { repo: 'microsoft/playwright', query: 'timeout' },
				},
			})),
			startRun: vi.fn(async () => ({
				ok: true as const,
				data: {
					runId: 'run_failed',
					status: 'failed' as const,
					workflowId: 'wf_github_issue_search',
					version: 3,
					failedChunkId: 'search_issues',
					failedStepId: 'fill_query',
					fallbackReason: 'locator_not_found: searchbox',
				},
			})),
			searchRepairs: vi.fn(async () => ({
				ok: true as const,
				data: {
					patches: [
						{
							patchId: 'patch_searchbox',
							workflowId: 'wf_github_issue_search',
							version: 3,
							chunkId: 'search_issues',
							stepId: 'fill_query',
							failureType: 'locator_not_found',
							riskLevel: 'read_only' as const,
							reasons: ['registry repair patch matched'],
						},
					],
				},
			})),
		}
		const statuses: string[] = []
		const service = new WorkflowReplayService({
			projectId: 'default',
			client,
			onStatus: (event) => statuses.push(event.kind),
		})

		const result = await service.tryReplay({
			task: 'search timeout',
			currentUrl: 'https://github.com/microsoft/playwright/issues',
			pageObservation: observation,
		})

		expect(result).toMatchObject({ status: 'fallback', reason: 'run_failed' })
		expect(result).toMatchObject({
			repairContext: {
				workflowId: 'wf_github_issue_search',
				version: 3,
				chunkId: 'search_issues',
				stepId: 'fill_query',
				fallbackReason: 'locator_not_found: searchbox',
				currentPageState: 'github_issues_list',
			},
		})
		expect(client.searchRepairs).toHaveBeenCalledWith(
			expect.objectContaining({
				workflowId: 'wf_github_issue_search',
				chunkId: 'search_issues',
				stepId: 'fill_query',
				failureType: 'locator_not_found',
			})
		)
		expect(statuses).toContain('repair_candidate')
	})
})
