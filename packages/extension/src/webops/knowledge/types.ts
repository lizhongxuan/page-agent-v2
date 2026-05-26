export interface KnowledgeSearchRequest {
	task: string
	url: string
	title: string
	projectKey?: string
	visibleText?: string
	hints?: string[]
	limit: number
}

export interface KnowledgeHit {
	id: string
	title: string
	source: string
	snippet: string
	url?: string
	score: number
	tags?: string[]
}
