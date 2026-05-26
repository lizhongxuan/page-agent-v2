export interface KnowledgeSettings {
	enabled: boolean
	baseUrl: string
	apiKey: string
	projectKey: string
	allowPageSummary: boolean
}

export const defaultKnowledgeSettings: KnowledgeSettings = {
	enabled: false,
	baseUrl: '',
	apiKey: '',
	projectKey: '',
	allowPageSummary: true,
}
