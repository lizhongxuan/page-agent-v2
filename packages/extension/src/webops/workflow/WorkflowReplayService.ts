import { LLM, type LLMConfig } from '@page-agent/llms'
import * as z from 'zod/v4'

import type { RemotePageController } from '@/agent/RemotePageController'

import { executeWorkflowChunk } from './ChunkExecutor'
import { WorkflowClient } from './WorkflowClient'
import { matchWorkflowCandidates } from './WorkflowMatcher'
import { WorkflowRunReporter } from './WorkflowRunReporter'
import { buildPageFingerprint } from './pageFingerprint'
import type {
	RiskPolicy,
	VariableBindingResult,
	WorkflowRecipe,
	WorkflowSearchResult,
} from './types'
import { bindWorkflowVariables } from './variableBinding'

export interface WorkflowReplayServiceOptions {
	baseUrl: string
	apiKey?: string
	llmConfig: LLMConfig
	projectId?: string
	fetch?: typeof fetch
}

export type WorkflowReplayResult =
	| { status: 'skipped'; reason: string }
	| { status: 'completed'; workflowId: string; workflowName: string; executedStepIds: string[] }
	| { status: 'fallback'; reason: string; workflowId?: string; workflowName?: string }

export class WorkflowReplayService {
	private readonly client: WorkflowClient
	private readonly llm: LLM
	private readonly reporter: WorkflowRunReporter
	private readonly projectId: string

	constructor(private readonly options: WorkflowReplayServiceOptions) {
		this.client = new WorkflowClient({
			baseUrl: options.baseUrl,
			apiKey: options.apiKey ?? '',
			fetch: options.fetch,
		})
		this.reporter = new WorkflowRunReporter({
			baseUrl: options.baseUrl,
			apiKey: options.apiKey ?? '',
			fetch: options.fetch,
		})
		this.llm = new LLM(options.llmConfig)
		this.projectId = options.projectId || 'default'
	}

	async tryReplay(
		task: string,
		pageController: RemotePageController
	): Promise<WorkflowReplayResult> {
		const browserState = await pageController.getBrowserState()
		const pageFingerprint = buildPageFingerprint({
			url: browserState.url,
			title: browserState.title,
			visibleText: browserState.content,
		})
		const candidates = await this.client.searchWorkflows({
			projectId: this.projectId,
			task,
			url: browserState.url,
			domain: hostFromUrl(browserState.url),
			pageFingerprint,
			limit: 3,
		})
		if (candidates.length === 0) {
			return { status: 'skipped', reason: 'No active workflow matched the current task.' }
		}

		const match = matchWorkflowCandidates({
			candidates,
			pageText: `${browserState.title}\n${browserState.content}`,
			minScore: 0.55,
		}).accepted[0]
		if (!match) {
			return { status: 'skipped', reason: 'Workflow candidates were rejected by local policy.' }
		}

		const recipe = match.candidate.recipe
		const binding = await this.bindVariables(task, recipe, browserState.url)
		if (binding.status !== 'ready') {
			await this.reportRun(
				recipe,
				'fallback',
				`Workflow variables are not ready: ${binding.status}.`,
				[]
			)
			return {
				status: 'fallback',
				workflowId: recipe.id,
				workflowName: recipe.name,
				reason: `Workflow variables are not ready: ${binding.status}.`,
			}
		}

		const elements = await pageController.getWorkflowElements()
		const variables = Object.fromEntries(
			Object.entries(binding.bindings).map(([name, value]) => [name, value.value])
		)
		const executedStepIds: string[] = []

		for (const chunk of recipe.chunks) {
			const result = await executeWorkflowChunk({
				chunk,
				elements,
				variables,
				riskPolicy: recipe.safetyPolicy as RiskPolicy,
				actions: {
					click: (element) => pageController.clickWorkflowElement(element),
					input: (element, value) => pageController.inputWorkflowElement(element, value),
					select: (element, value) => pageController.selectWorkflowElement(element, value),
					scroll: async (direction, pages) => {
						await pageController.scroll({ down: direction === 'down', numPages: pages ?? 1 })
					},
				},
			})
			executedStepIds.push(...result.executedStepIds)
			if (result.status !== 'success') {
				await this.reportRun(
					recipe,
					'failed',
					`Workflow replay stopped: ${result.status} ${result.reason}`,
					executedStepIds
				)
				return {
					status: 'fallback',
					workflowId: recipe.id,
					workflowName: recipe.name,
					reason: `Workflow replay stopped: ${result.status} ${result.reason}`,
				}
			}
		}

		await this.reportRun(recipe, 'success', 'Workflow replay completed.', executedStepIds)
		return {
			status: 'completed',
			workflowId: recipe.id,
			workflowName: recipe.name,
			executedStepIds,
		}
	}

	private async bindVariables(
		task: string,
		recipe: WorkflowRecipe,
		currentUrl: string
	): Promise<VariableBindingResult> {
		const deterministic = bindWorkflowVariables({
			task,
			taskSlots: inferTaskSlots(task, recipe, currentUrl),
			variables: recipe.variables,
		})
		if (deterministic.status === 'ready') return deterministic

		const llmBindings = await bindVariablesWithLLM(this.llm, task, recipe, currentUrl).catch(
			() => ({})
		)
		return bindWorkflowVariables({
			task,
			taskSlots: { ...inferTaskSlots(task, recipe, currentUrl), ...llmBindings },
			variables: recipe.variables,
		})
	}

	private async reportRun(
		recipe: WorkflowRecipe,
		status: 'success' | 'failed' | 'fallback',
		message: string,
		executedStepIds: string[]
	): Promise<void> {
		await this.reporter
			.reportRun({
				projectId: this.projectId,
				workflowId: recipe.id,
				workflowVersion: recipe.version,
				status,
				message,
				executedStepIds,
				createdAt: new Date().toISOString(),
			})
			.catch(() => undefined)
	}
}

async function bindVariablesWithLLM(
	llm: LLM,
	task: string,
	recipe: WorkflowRecipe,
	currentUrl: string
): Promise<Record<string, string>> {
	const schema = z.object({
		bindings: z.record(z.string(), z.string()).describe('Variable name to value bindings.'),
	})
	const result = await llm.invoke(
		[
			{
				role: 'system',
				content:
					'You bind workflow variables for browser replay. Return only values explicitly supported by the user task, current URL, or workflow intent. Do not invent secrets.',
			},
			{
				role: 'user',
				content: JSON.stringify({
					task,
					currentUrl,
					workflow: {
						name: recipe.name,
						intent: recipe.intent,
						variables: recipe.variables.map((variable) => ({
							name: variable.name,
							description: variable.description,
							required: variable.required,
							sensitive: variable.sensitive,
							policy: variable.policy,
						})),
					},
				}),
			},
		],
		{
			bind_variables: {
				inputSchema: schema,
				execute: async (args) => args.bindings,
			},
		},
		new AbortController().signal,
		{ toolChoiceName: 'bind_variables' }
	)
	return result.toolResult as Record<string, string>
}

function inferTaskSlots(
	task: string,
	recipe: WorkflowRecipe,
	currentUrl: string
): Record<string, string> {
	const slots: Record<string, string> = {}
	const repo = extractGithubRepo(task) ?? extractGithubRepo(currentUrl)
	for (const variable of recipe.variables) {
		const name = variable.name.toLowerCase()
		if (repo && (name.includes('repo') || name.includes('project'))) {
			slots[variable.name] = repo
			continue
		}
		const quoted = /[“"']([^“"']{1,120})[”"']/.exec(task)?.[1]
		if (quoted && (name.includes('query') || name.includes('search') || name.includes('keyword'))) {
			slots[variable.name] = quoted
		}
	}
	return slots
}

function extractGithubRepo(value: string): string | undefined {
	const match = /github\.com\/([^/\s]+)\/([^/\s?#]+)/i.exec(value)
	return match ? `${match[1]}/${match[2]}` : undefined
}

function hostFromUrl(rawUrl: string): string {
	try {
		return new URL(rawUrl).hostname
	} catch {
		return ''
	}
}
