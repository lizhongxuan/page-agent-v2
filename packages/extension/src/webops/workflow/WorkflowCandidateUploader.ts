import type { RecordedSession } from '../recorder/actionEvents'

export interface WorkflowCandidateUploaderOptions {
	baseUrl: string
	apiKey?: string
	fetch?: typeof fetch
}

export interface WorkflowCandidateSummary {
	id: string
	searchable: boolean
	reviewStatus?: string
	notificationStatus?: string
}

export type UploadCandidateResult =
	| { ok: true; candidate: WorkflowCandidateSummary }
	| { ok: false; error: string }

export class WorkflowCandidateUploader {
	private readonly baseUrl: string
	private readonly apiKey?: string
	private readonly fetchFn: typeof fetch

	constructor(options: WorkflowCandidateUploaderOptions) {
		this.baseUrl = options.baseUrl.replace(/\/+$/, '')
		this.apiKey = options.apiKey
		this.fetchFn = options.fetch ?? fetch
	}

	async uploadSuccessfulSession(session: RecordedSession): Promise<UploadCandidateResult> {
		return this.uploadSession(session, 'agent_run')
	}

	async uploadManualSession(session: RecordedSession): Promise<UploadCandidateResult> {
		return this.uploadSession(session, 'user_demo')
	}

	private async uploadSession(
		session: RecordedSession,
		source: 'agent_run' | 'user_demo'
	): Promise<UploadCandidateResult> {
		try {
			const response = await this.fetchFn(`${this.baseUrl}/api/workflow-candidates/from-session`, {
				method: 'POST',
				headers: {
					'content-type': 'application/json',
					...(this.apiKey ? { Authorization: `Bearer ${this.apiKey}` } : {}),
				},
				body: JSON.stringify(toBackendRecordedSession(session, source)),
			})
			if (!response.ok) {
				return { ok: false, error: `candidate upload failed: ${response.status}` }
			}
			const candidate = (await response.json()) as WorkflowCandidateSummary
			return { ok: true, candidate }
		} catch (error) {
			return { ok: false, error: error instanceof Error ? error.message : String(error) }
		}
	}
}

function toBackendRecordedSession(session: RecordedSession, source: 'agent_run' | 'user_demo') {
	return {
		id: session.id,
		projectId: 'default',
		source,
		task: session.task,
		startUrl: session.startUrl,
		site: hostFromUrl(session.startUrl),
		pageBefore: {
			url: session.startUrl,
			title: session.steps[0]?.pageTitle ?? '',
			visibleText: session.steps[0]?.beforePage?.visibleText ?? [],
			controlSignatures: session.steps[0]?.beforePage?.controlSignatures ?? [],
		},
		events: session.steps.map((step) => ({
			id: step.id,
			type: step.type,
			label: step.target?.name ?? step.target?.text ?? step.note ?? step.type,
			value: step.value,
			sensitive: step.value === '[REDACTED]',
			targetCandidates:
				step.target?.candidates?.map((candidate) => ({
					strategy: candidate.strategy === 'testId' ? 'test_id' : candidate.strategy,
					value: candidate.value,
					role: candidate.role,
					name: candidate.name,
					nearText: candidate.nearText,
					container: candidate.container,
				})) ?? [],
		})),
		artifactRefs: [],
	}
}

function hostFromUrl(rawUrl: string): string {
	try {
		return new URL(rawUrl).hostname
	} catch {
		return ''
	}
}
