import type { KnowledgeHit, KnowledgeSearchRequest } from './types'

export interface KnowledgeClient {
	search(request: KnowledgeSearchRequest): Promise<KnowledgeHit[]>
}
