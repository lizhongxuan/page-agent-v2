import type { KnowledgeClient } from './KnowledgeClient'
import { formatKnowledgeForPrompt } from './promptFormatter'
import type { KnowledgeHit, KnowledgeSearchRequest } from './types'

export class KnowledgeContextProvider {
	private readonly client: KnowledgeClient | undefined

	constructor(client: KnowledgeClient | undefined) {
		this.client = client
	}

	async getContext(
		request: KnowledgeSearchRequest
	): Promise<{ promptContext: string; hits: KnowledgeHit[] }> {
		if (!this.client) return { promptContext: '', hits: [] }

		const hits = await this.client.search(request)
		return { promptContext: formatKnowledgeForPrompt(hits), hits }
	}
}
