export interface WorkflowRunReporterOptions {
	baseUrl: string
	apiKey: string
	fetch?: typeof fetch
}

export interface WorkflowReportResult {
	ok: boolean
	id?: string
	error?: string
}

export type WorkflowRunReport = Record<string, unknown>

export type SelectorStatsReport = Record<string, unknown>

export class WorkflowRunReporter {
	private readonly baseUrl: string
	private readonly apiKey: string
	private readonly fetchFn: typeof fetch

	constructor(options: WorkflowRunReporterOptions) {
		this.baseUrl = options.baseUrl.replace(/\/+$/, '')
		this.apiKey = options.apiKey
		this.fetchFn = options.fetch ?? fetch
	}

	async reportRun(report: WorkflowRunReport): Promise<WorkflowReportResult> {
		return this.post(`${this.baseUrl}/api/workflow-runs`, report)
	}

	async reportSelectorStats(
		runId: string,
		stats: SelectorStatsReport
	): Promise<WorkflowReportResult> {
		return this.post(
			`${this.baseUrl}/api/workflow-runs/${encodeURIComponent(runId)}/selector-stats`,
			stats
		)
	}

	private async post(url: string, body: Record<string, unknown>): Promise<WorkflowReportResult> {
		try {
			const response = await this.fetchFn(url, {
				method: 'POST',
				headers: {
					Authorization: `Bearer ${this.apiKey}`,
					'Content-Type': 'application/json',
				},
				body: JSON.stringify(body),
			})

			if (!response.ok) {
				return { ok: false, error: `Request failed with status ${response.status}` }
			}

			const payload = (await response.json()) as WorkflowReportResult
			return payload.ok ? payload : { ok: true, id: payload.id }
		} catch (error) {
			return { ok: false, error: errorMessage(error) }
		}
	}
}

function errorMessage(error: unknown): string {
	return error instanceof Error ? error.message : String(error)
}
