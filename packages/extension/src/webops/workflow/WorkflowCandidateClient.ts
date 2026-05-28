import type { WorkflowCandidateSummary } from './WorkflowCandidateUploader'

export interface WorkflowCandidateClientOptions {
	baseUrl: string
	apiKey?: string
	fetch?: typeof fetch
}

export type CandidateActionResult =
	| { ok: true; candidate: WorkflowCandidateSummary }
	| { ok: false; error: string }

export class WorkflowCandidateClient {
	private readonly baseUrl: string
	private readonly apiKey?: string
	private readonly fetchFn: typeof fetch

	constructor(options: WorkflowCandidateClientOptions) {
		this.baseUrl = options.baseUrl.replace(/\/+$/, '')
		this.apiKey = options.apiKey
		this.fetchFn = options.fetch ?? fetch
	}

	async approve(id: string): Promise<CandidateActionResult> {
		return this.postCandidateAction(id, 'approve')
	}

	async reject(id: string): Promise<CandidateActionResult> {
		return this.postCandidateAction(id, 'reject')
	}

	private async postCandidateAction(
		id: string,
		action: 'approve' | 'reject'
	): Promise<CandidateActionResult> {
		try {
			const response = await this.fetchFn(
				`${this.baseUrl}/api/workflow-candidates/${encodeURIComponent(id)}/${action}`,
				{
					method: 'POST',
					headers: {
						...(this.apiKey ? { Authorization: `Bearer ${this.apiKey}` } : {}),
					},
				}
			)
			if (!response.ok) {
				return { ok: false, error: `candidate ${action} failed: ${response.status}` }
			}
			const candidate = (await response.json()) as WorkflowCandidateSummary
			return { ok: true, candidate }
		} catch (error) {
			return { ok: false, error: error instanceof Error ? error.message : String(error) }
		}
	}
}
