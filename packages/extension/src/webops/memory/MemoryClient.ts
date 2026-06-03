import type {
	MemoryClientConfig,
	MemoryContextRequest,
	MemoryContextResponse,
	MemoryPageObservationRequest,
	MemoryPageObservationResponse,
	MemoryTaskRunRequest,
	MemoryTaskRunResponse,
} from './types'

export class MemoryClient {
	private readonly baseUrl: string
	private readonly bearerToken?: string

	constructor(config: MemoryClientConfig) {
		this.baseUrl = config.baseUrl.replace(/\/+$/, '')
		this.bearerToken = config.bearerToken
	}

	observePage(request: MemoryPageObservationRequest): Promise<MemoryPageObservationResponse> {
		return this.post('/api/memory/page-observations', request)
	}

	getContext(request: MemoryContextRequest): Promise<MemoryContextResponse> {
		return this.post('/api/memory/context', request)
	}

	completeTaskRun(request: MemoryTaskRunRequest): Promise<MemoryTaskRunResponse> {
		return this.post('/api/memory/task-runs', request)
	}

	private async post<TResponse>(path: string, request: unknown): Promise<TResponse> {
		const response = await fetch(`${this.baseUrl}${path}`, {
			method: 'POST',
			headers: {
				'content-type': 'application/json',
				...(this.bearerToken ? { Authorization: `Bearer ${this.bearerToken}` } : {}),
			},
			body: JSON.stringify(request),
		})

		if (!response.ok) {
			throw new Error(`Memory request failed with status ${response.status}`)
		}

		return (await response.json()) as TResponse
	}
}
