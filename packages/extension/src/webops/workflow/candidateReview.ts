import type {
	RecordedAction,
	RecordedActionTarget,
	RecordedSession,
} from '../recorder/actionEvents'
import type {
	WorkflowCandidateCreateRequest,
	WorkflowRecipe,
	WorkflowRecipeStep,
	WorkflowRiskLevel,
	WorkflowStepTarget,
	WorkflowStepType,
	WorkflowTargetCandidate,
	WorkflowVariable,
} from './types'

export type WorkflowCandidateBuildResult =
	| { ok: true; request: WorkflowCandidateCreateRequest }
	| {
			ok: false
			reason: 'high_risk_action' | 'unsupported_session'
			message: string
	  }

export interface WorkflowCandidateBuildOptions {
	projectId: string
	source?: string
	now?: Date
}

const HIGH_RISK_PATTERN =
	/\b(delete|remove|destroy|transfer|make public|publish|send|purchase|buy|pay|merge|close account|revoke)\b/i

export function buildWorkflowCandidateRequest(
	session: RecordedSession,
	options: WorkflowCandidateBuildOptions
): WorkflowCandidateBuildResult {
	if (session.steps.some(isHighRiskAction)) {
		return {
			ok: false,
			reason: 'high_risk_action',
			message:
				'High-risk recorded actions require manual recipe authoring before replay can be enabled.',
		}
	}

	const site = hostFromUrl(session.startUrl)
	if (!site) {
		return {
			ok: false,
			reason: 'unsupported_session',
			message: 'Recorded session start URL must include a valid host.',
		}
	}

	const successfulSteps = session.steps.filter((step) => step.result === 'success')
	const recipeSteps = successfulSteps.reduce<WorkflowRecipeStep[]>((steps, action) => {
		const step = toWorkflowStep(action, steps.length + 1)
		if (step) steps.push(step)
		return steps
	}, [])

	if (recipeSteps.length === 0 || !recipeSteps.some(isExecutableStep)) {
		return {
			ok: false,
			reason: 'unsupported_session',
			message: 'Recorded session does not contain supported replay actions.',
		}
	}

	const githubRepo = extractGithubRepo(session.startUrl) ?? extractGithubRepoFromTask(session.task)
	const queryExample = firstQueryExample(successfulSteps)
	const variables = buildVariables(githubRepo, queryExample)
	const inferredTask = inferTaskForSession(session, site, githubRepo, queryExample, recipeSteps)
	const startPageState = pageStateForUrl(session.startUrl, session.startUrl)
	const endPageState = pageStateForUrl(
		successfulSteps.at(-1)?.pageUrl ?? session.startUrl,
		session.startUrl
	)
	const workflowId = workflowIdForSession(session, site, githubRepo, queryExample)
	const riskLevel: WorkflowRiskLevel = queryExample ? 'read_or_search' : 'read_only'
	const now = (options.now ?? new Date()).toISOString()

	const recipe: WorkflowRecipe = {
		id: workflowId,
		version: 1,
		projectId: options.projectId,
		status: 'pending_review',
		searchable: false,
		site,
		app: site === 'github.com' ? 'github' : undefined,
		name: nameFromTask(inferredTask),
		intent: inferredTask,
		description: descriptionForSession(
			{ ...session, task: inferredTask },
			githubRepo,
			queryExample
		),
		tags: tagsForSession(site, githubRepo, queryExample),
		riskLevel,
		requiresConfirmation: false,
		variables,
		chunks: [
			{
				id: 'chunk_1',
				name: site === 'github.com' ? 'Recorded GitHub workflow' : 'Recorded browser workflow',
				fromPageState: startPageState,
				toPageState: endPageState,
				stepSummary: recipeSteps.map(stepSummary).join(' -> '),
				riskLevel,
				steps: recipeSteps,
				variableNames: variables.map((variable) => variable.name),
				successRate: 1,
				selectorHealth: 1,
			},
		],
		startPageStates: [startPageState],
		endPageStates: [endPageState],
		createdAt: now,
		updatedAt: now,
	}

	return {
		ok: true,
		request: {
			projectId: options.projectId,
			source: options.source ?? 'user_demo',
			task: inferredTask,
			startUrl: session.startUrl,
			recipe,
		},
	}
}

function toWorkflowStep(action: RecordedAction, ordinal: number): WorkflowRecipeStep | undefined {
	if (action.type === 'click') {
		const target = toStepTarget(action.target)
		if (!target) return undefined
		return {
			id: `step_${ordinal}`,
			type: 'click',
			target,
			riskLevel: 'read_or_search',
		}
	}

	if (action.type === 'input') {
		const target = toStepTarget(action.target)
		if (!target) return undefined
		if (action.note === 'keydown' && action.value) {
			return {
				id: `step_${ordinal}`,
				type: 'press',
				target,
				key: action.value,
				riskLevel: 'read_or_search',
			}
		}

		return {
			id: `step_${ordinal}`,
			type: 'fill',
			target,
			value: '{{query}}',
			riskLevel: 'read_or_search',
		}
	}

	if (action.type === 'select') {
		const target = toStepTarget(action.target)
		if (!target || !action.value) return undefined
		return {
			id: `step_${ordinal}`,
			type: 'select',
			target,
			value: action.value,
			riskLevel: 'read_or_search',
		}
	}

	if (action.type === 'wait') {
		return {
			id: `step_${ordinal}`,
			type: 'wait',
			riskLevel: 'read_only',
		}
	}

	return undefined
}

function isExecutableStep(step: WorkflowRecipeStep): boolean {
	return step.type !== 'wait'
}

function inferTaskForSession(
	session: RecordedSession,
	site: string,
	repo: string | undefined,
	queryExample: string | undefined,
	steps: WorkflowRecipeStep[]
): string {
	const explicitTask = session.task.trim()
	if (explicitTask) return explicitTask

	const stepText = steps.map(stepSummary).join(' -> ')
	if (site === 'github.com' && repo && queryExample) {
		return `Replay GitHub workflow on ${repo} with query "${queryExample}": ${stepText}`
	}
	if (site === 'github.com' && repo) {
		return `Replay GitHub workflow on ${repo}: ${stepText}`
	}
	if (queryExample) {
		return `Replay recorded workflow on ${site} with query "${queryExample}": ${stepText}`
	}
	return `Replay recorded workflow on ${site}: ${stepText}`
}

function toStepTarget(target?: RecordedActionTarget): WorkflowStepTarget | undefined {
	const candidates = targetCandidates(target)
	if (candidates.length === 0) return undefined
	const fallbacks = candidates.slice(1)
	return {
		primary: candidates[0]!,
		...(fallbacks.length > 0 ? { fallbacks } : {}),
	}
}

function targetCandidates(target?: RecordedActionTarget): WorkflowTargetCandidate[] {
	if (!target) return []

	const candidates: WorkflowTargetCandidate[] = []
	if (target.role && target.name) {
		candidates.push({ strategy: 'role', role: target.role, name: target.name })
	}
	if (target.name && !target.role) {
		candidates.push({ strategy: 'placeholder', value: target.name })
	}
	if (target.text) {
		candidates.push({ strategy: 'text', value: target.text })
	}
	if (target.testId) {
		candidates.push({ strategy: 'test_id', value: target.testId })
	}
	if (target.css && isSafeCssSelector(target.css)) {
		candidates.push({ strategy: 'css', value: target.css })
	}
	if (target.xpath) {
		candidates.push({ strategy: 'xpath', value: target.xpath })
	}
	return candidates
}

function isSafeCssSelector(selector: string): boolean {
	const normalized = selector.trim()
	if (!normalized) return false
	if (/^[a-z][\w-]*$/i.test(normalized)) return false
	if (normalized === '*' || normalized.includes(':has(')) return false
	return true
}

function buildVariables(
	repo: string | undefined,
	queryExample: string | undefined
): WorkflowVariable[] {
	const variables: WorkflowVariable[] = []
	if (repo) {
		variables.push({
			name: 'repo',
			type: 'string',
			required: true,
			source: 'task_or_url',
			examples: [repo],
		})
	}
	if (queryExample) {
		variables.push({
			name: 'query',
			type: 'string',
			required: true,
			source: 'task',
			examples: [queryExample],
		})
	}
	return variables.length > 0
		? variables
		: [{ name: 'query', type: 'string', required: true, source: 'task' }]
}

function firstQueryExample(actions: RecordedAction[]): string | undefined {
	return actions.find((action) => action.type === 'input' && action.note !== 'keydown')?.value
}

function extractGithubRepo(url: string): string | undefined {
	try {
		const parsed = new URL(url)
		if (parsed.hostname !== 'github.com') return undefined
		const [owner, repo] = parsed.pathname.split('/').filter(Boolean)
		if (!owner || !repo) return undefined
		if (['orgs', 'users', 'settings', 'marketplace', 'topics'].includes(owner)) return undefined
		return `${owner}/${repo}`
	} catch {
		return undefined
	}
}

function extractGithubRepoFromTask(task: string): string | undefined {
	const match = /github\.com\/([A-Za-z0-9_.-]+)\/([A-Za-z0-9_.-]+)/i.exec(task)
	if (!match?.[1] || !match[2]) return undefined
	return `${match[1]}/${match[2].replace(/\.git$/i, '')}`
}

function hostFromUrl(url: string): string | undefined {
	try {
		return new URL(url).hostname
	} catch {
		return undefined
	}
}

function pageStateForUrl(url: string, startUrl: string): string {
	const site = hostFromUrl(startUrl)
	if (site === 'github.com') {
		const parsed = safeUrl(url)
		if (parsed?.pathname.includes('/issues')) return 'github_issues_list'
		if (extractGithubRepo(startUrl)) return 'github_repo_home'
		return 'github_page'
	}
	return `${slug(site ?? 'web')}_page`
}

function workflowIdForSession(
	session: RecordedSession,
	site: string,
	repo: string | undefined,
	queryExample: string | undefined
): string {
	if (site === 'github.com' && repo && queryExample) {
		return `wf_manual_github_${slug(repo)}_issues_search`
	}
	return `wf_manual_${slug(site)}_${slug(session.id)}`
}

function nameFromTask(task: string): string {
	const trimmed = task.trim()
	if (!trimmed) return 'Recorded browser workflow'
	return trimmed.length <= 80 ? trimmed : `${trimmed.slice(0, 77)}...`
}

function descriptionForSession(
	session: RecordedSession,
	repo: string | undefined,
	queryExample: string | undefined
): string {
	const parts = ['Replay a user-recorded browser workflow.']
	if (repo) parts.push(`Recorded GitHub repository: ${repo}.`)
	if (queryExample) parts.push('Search/input values are parameterized as {{query}}.')
	if (session.task) parts.push(`Original task: ${session.task}.`)
	return parts.join(' ')
}

function tagsForSession(
	site: string,
	repo: string | undefined,
	queryExample: string | undefined
): string[] {
	return [site, repo ? 'repo' : undefined, queryExample ? 'search' : undefined].filter(
		(tag): tag is string => Boolean(tag)
	)
}

function stepSummary(step: WorkflowRecipeStep): string {
	const target = step.target?.primary
	const label = target?.name ?? target?.value ?? target?.role ?? 'page'
	return `${step.type} ${label}`
}

function isHighRiskAction(action: RecordedAction): boolean {
	const text = [
		action.target?.name,
		action.target?.text,
		action.target?.css,
		action.value,
		action.note,
		action.pageTitle,
	]
		.filter(Boolean)
		.join(' ')
	return HIGH_RISK_PATTERN.test(text)
}

function safeUrl(url: string): URL | undefined {
	try {
		return new URL(url)
	} catch {
		return undefined
	}
}

function slug(value: string): string {
	return value
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, '_')
		.replace(/^_+|_+$/g, '')
}
