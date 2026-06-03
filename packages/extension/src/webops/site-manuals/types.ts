export type SiteManualSourceType = 'markdown' | 'html' | 'pdf_text' | 'text'

export type SiteManualStatus = 'active' | 'replaced' | 'disabled' | 'stale'

export interface SiteManualClientConfig {
	baseUrl: string
	bearerToken?: string
}

export interface SiteManualSource {
	id: string
	projectId?: string
	site: string
	module?: string
	title: string
	sourceType?: SiteManualSourceType
	contentHash?: string
	metadata?: Record<string, unknown>
	status: SiteManualStatus
	createdAt?: string
	updatedAt?: string
	lastCompiledAt?: string
	compileStatus?: string
	compileError?: string
}

export interface SiteManualWikiPage {
	id: string
	projectId?: string
	site?: string
	module?: string
	pageKey?: string
	title: string
	summary?: string
	facts?: unknown[]
	procedures?: unknown[]
	relatedPages?: string[]
	sourceRefs?: unknown[]
	confidence?: number
	status?: SiteManualStatus
	updatedAt?: string
}

export interface SiteManualWikiChunk {
	id: string
	wikiPageId: string
	projectId?: string
	site?: string
	module?: string
	chunkType?: string
	text: string
	pageGuards?: unknown
	targetTerms?: string[]
	sourceRefs?: unknown[]
	status?: SiteManualStatus
	confidence?: number
	score?: number
}

export interface SiteManualPreviewContext {
	matchedChunks?: SiteManualWikiChunk[]
	filtered?: {
		id?: string
		reason: string
	}[]
	prompt?: string
	siteManualKnowledge?: {
		id: string
		summary?: string
		sourceRefs?: string[]
		confidence?: number
	}[]
	[key: string]: unknown
}

export interface SiteManualImportRequest {
	projectId: string
	site: string
	module?: string
	title: string
	sourceType: SiteManualSourceType
	content: string
	metadata?: Record<string, unknown>
}

export interface SiteManualImportResponse {
	ok?: boolean
	sourceId?: string
	source?: SiteManualSource
	wikiPages?: SiteManualWikiPage[]
	[key: string]: unknown
}

export interface SiteManualListQuery {
	projectId?: string
	site?: string
	module?: string
	status?: SiteManualStatus | 'all'
}

export interface SiteManualListResponse {
	sources?: SiteManualSource[]
	items?: SiteManualSource[]
	[key: string]: unknown
}

export interface SiteManualWikiResponse {
	source?: SiteManualSource
	pages?: SiteManualWikiPage[]
	chunks?: SiteManualWikiChunk[]
	[key: string]: unknown
}

export interface SiteManualPreviewRequest {
	projectId: string
	site: string
	module?: string
	task: string
	url: string
}

export interface SiteManualClientLike {
	importSiteManual?(request: SiteManualImportRequest): Promise<SiteManualImportResponse>
	listSiteManuals?(query?: SiteManualListQuery): Promise<SiteManualListResponse>
	getSiteManual?(id: string): Promise<SiteManualSource>
	getSiteManualWiki?(id: string): Promise<SiteManualWikiResponse>
	rebuildSiteManual?(id: string): Promise<Record<string, unknown>>
	disableSiteManual?(id: string): Promise<Record<string, unknown>>
	enableSiteManual?(id: string): Promise<Record<string, unknown>>
	deleteSiteManual?(id: string): Promise<Record<string, unknown>>
	previewContext?(request: SiteManualPreviewRequest): Promise<SiteManualPreviewContext>
}
