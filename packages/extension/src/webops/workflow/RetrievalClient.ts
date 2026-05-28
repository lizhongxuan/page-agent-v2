import type {
	WorkflowCandidateCreateRequest,
	WorkflowCandidateListRequest,
	WorkflowCandidateListResponse,
	WorkflowCandidateReview,
	WorkflowErrorCode,
	WorkflowErrorResponse,
	WorkflowInterruptSearchRequest,
	WorkflowInterruptSearchResponse,
	WorkflowRepairPatchCandidate,
	WorkflowRepairPatchCandidateCreateRequest,
	WorkflowRepairPatchListRequest,
	WorkflowRepairPatchListResponse,
	WorkflowRepairSearchRequest,
	WorkflowRepairSearchResponse,
	WorkflowResult,
	WorkflowRunStartRequest,
	WorkflowRunStartResponse,
	WorkflowSearchRequest,
	WorkflowSearchResponse,
	WorkflowSelectRequest,
	WorkflowSelectResponse,
} from './types'

export interface RetrievalClientConfig {
	baseUrl: string
	bearerToken?: string
}

export class RetrievalClient {
	private readonly baseUrl: string
	private readonly bearerToken?: string

	constructor(config: RetrievalClientConfig) {
		this.baseUrl = config.baseUrl.replace(/\/+$/, '')
		this.bearerToken = config.bearerToken
	}

	searchWorkflows(request: WorkflowSearchRequest): Promise<WorkflowResult<WorkflowSearchResponse>> {
		return this.post('/api/retrieval/workflows/search', request)
	}

	selectWorkflow(request: WorkflowSelectRequest): Promise<WorkflowResult<WorkflowSelectResponse>> {
		return this.post('/api/retrieval/workflows/select', request)
	}

	searchInterrupts(
		request: WorkflowInterruptSearchRequest
	): Promise<WorkflowResult<WorkflowInterruptSearchResponse>> {
		return this.post('/api/retrieval/interrupts/search', request)
	}

	searchRepairs(
		request: WorkflowRepairSearchRequest
	): Promise<WorkflowResult<WorkflowRepairSearchResponse>> {
		return this.post('/api/retrieval/repairs/search', request)
	}

	startRun(request: WorkflowRunStartRequest): Promise<WorkflowResult<WorkflowRunStartResponse>> {
		return this.post('/api/runs/start', request)
	}

	createWorkflowCandidateFromSession(
		request: WorkflowCandidateCreateRequest
	): Promise<WorkflowResult<WorkflowCandidateReview>> {
		return this.post('/api/workflow-candidates/from-session', request)
	}

	listWorkflowCandidates(
		request: WorkflowCandidateListRequest = {}
	): Promise<WorkflowResult<WorkflowCandidateListResponse>> {
		const params = new URLSearchParams()
		if (request.projectId) params.set('projectId', request.projectId)
		if (request.source) params.set('source', request.source)
		if (request.reviewStatus) params.set('reviewStatus', request.reviewStatus)
		const query = params.toString()
		return this.get(`/api/workflow-candidates${query ? `?${query}` : ''}`)
	}

	approveWorkflowCandidate(id: string): Promise<WorkflowResult<WorkflowCandidateReview>> {
		return this.post(`/api/workflow-candidates/${encodeURIComponent(id)}/approve`, undefined)
	}

	rejectWorkflowCandidate(id: string): Promise<WorkflowResult<WorkflowCandidateReview>> {
		return this.post(`/api/workflow-candidates/${encodeURIComponent(id)}/reject`, undefined)
	}

	createRepairPatchCandidate(
		patch: WorkflowRepairPatchCandidate
	): Promise<WorkflowResult<WorkflowRepairPatchCandidate>> {
		const request: WorkflowRepairPatchCandidateCreateRequest = { patch }
		return this.post('/api/repair-patches/candidates', request)
	}

	listRepairPatches(
		request: WorkflowRepairPatchListRequest = {}
	): Promise<WorkflowResult<WorkflowRepairPatchListResponse>> {
		const params = new URLSearchParams()
		if (request.projectId) params.set('projectId', request.projectId)
		if (request.workflowId) params.set('workflowId', request.workflowId)
		if (request.status) params.set('status', request.status)
		const query = params.toString()
		return this.get(`/api/repair-patches${query ? `?${query}` : ''}`)
	}

	approveRepairPatch(id: string): Promise<WorkflowResult<WorkflowRepairPatchCandidate>> {
		return this.post(`/api/repair-patches/${encodeURIComponent(id)}/approve`, undefined)
	}

	rejectRepairPatch(id: string): Promise<WorkflowResult<WorkflowRepairPatchCandidate>> {
		return this.post(`/api/repair-patches/${encodeURIComponent(id)}/reject`, undefined)
	}

	private async get<TResponse>(path: string): Promise<WorkflowResult<TResponse>> {
		return this.request(path, { method: 'GET' })
	}

	private async post<TResponse>(
		path: string,
		request: unknown
	): Promise<WorkflowResult<TResponse>> {
		return this.request(path, {
			method: 'POST',
			body: request === undefined ? undefined : JSON.stringify(request),
		})
	}

	private async request<TResponse>(
		path: string,
		init: { method: 'GET' | 'POST'; body?: string }
	): Promise<WorkflowResult<TResponse>> {
		try {
			const response = await fetch(`${this.baseUrl}${path}`, {
				method: init.method,
				headers: {
					'content-type': 'application/json',
					...(this.bearerToken ? { Authorization: `Bearer ${this.bearerToken}` } : {}),
				},
				body: init.body,
			})

			if (!response.ok) {
				return {
					ok: false,
					error: await this.errorFromResponse(response),
				}
			}

			return {
				ok: true,
				data: (await response.json()) as TResponse,
			}
		} catch (error) {
			return {
				ok: false,
				error: {
					code: 'network_error',
					message: error instanceof Error ? error.message : 'Network request failed',
					retryable: true,
				},
			}
		}
	}

	private async errorFromResponse(response: Response): Promise<WorkflowErrorResponse> {
		const fallback: WorkflowErrorResponse = {
			code: 'http_error',
			message: `Workflow backend returned HTTP ${response.status}`,
			status: response.status,
			retryable: isRetryableStatus(response.status),
		}

		try {
			const rawBody = await response.text()
			const data = parseErrorBody(rawBody)
			const payload = errorPayload(data)
			return {
				code: isWorkflowErrorCode(payload.code) ? payload.code : fallback.code,
				message:
					typeof payload.message === 'string' && payload.message.trim()
						? payload.message
						: fallback.message,
				status: response.status,
				retryable:
					typeof payload.retryable === 'boolean'
						? payload.retryable
						: isRetryableStatus(response.status),
				details: payload.details,
			}
		} catch {
			return fallback
		}
	}
}

function parseErrorBody(rawBody: string): unknown {
	if (!rawBody.trim()) return undefined
	try {
		return JSON.parse(rawBody)
	} catch {
		return { message: rawBody.trim() }
	}
}

function errorPayload(data: unknown): Partial<WorkflowErrorResponse> {
	if (!isRecord(data)) return {}
	if (isRecord(data.error)) {
		return data.error as Partial<WorkflowErrorResponse>
	}
	return data as Partial<WorkflowErrorResponse>
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null
}

function isRetryableStatus(status: number): boolean {
	return status === 408 || status === 429 || status >= 500
}

function isWorkflowErrorCode(code: unknown): code is WorkflowErrorCode {
	return (
		code === 'network_error' ||
		code === 'http_error' ||
		code === 'invalid_response' ||
		code === 'invalid_json' ||
		code === 'qdrant_unavailable' ||
		code === 'embedding_failed' ||
		code === 'no_match' ||
		code === 'unsafe_risk' ||
		code === 'binding_failed' ||
		code === 'selector_failed' ||
		code === 'candidate_invalid' ||
		code === 'candidate_list_failed' ||
		code === 'candidate_not_found' ||
		code === 'candidate_approve_failed' ||
		code === 'candidate_reject_failed' ||
		code === 'repair_patch_invalid' ||
		code === 'repair_patch_list_failed' ||
		code === 'repair_patch_approve_failed' ||
		code === 'repair_patch_reject_failed' ||
		code === 'replay_failed' ||
		code === 'run_not_found'
	)
}
