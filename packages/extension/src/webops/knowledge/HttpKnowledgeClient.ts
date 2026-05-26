import type { KnowledgeClient } from './KnowledgeClient'
import type { KnowledgeHit, KnowledgeSearchRequest } from './types'

export class HttpKnowledgeClient implements KnowledgeClient {
	private readonly config: { baseUrl: string; apiKey?: string }

	constructor(config: { baseUrl: string; apiKey?: string }) {
		this.config = config
	}

	async search(request: KnowledgeSearchRequest): Promise<KnowledgeHit[]> {
		try {
			const response = await fetch(`${this.config.baseUrl.replace(/\/+$/, '')}/search`, {
				method: 'POST',
				headers: {
					'content-type': 'application/json',
					...(this.config.apiKey ? { Authorization: `Bearer ${this.config.apiKey}` } : {}),
				},
				body: JSON.stringify(request),
			})

			if (!response.ok) return []

			const data = (await response.json()) as { hits?: KnowledgeHit[] }
			return data.hits?.slice(0, request.limit) ?? []
		} catch {
			return []
		}
	}
}
