import type {
	SiteManualClientConfig,
	SiteManualImportRequest,
	SiteManualImportResponse,
	SiteManualListQuery,
	SiteManualListResponse,
	SiteManualPreviewContext,
	SiteManualPreviewRequest,
	SiteManualSource,
	SiteManualWikiResponse,
} from './types'

export class SiteManualClient {
	private readonly baseUrl: string
	private readonly bearerToken?: string

	constructor(config: SiteManualClientConfig) {
		this.baseUrl = config.baseUrl.replace(/\/+$/, '')
		this.bearerToken = config.bearerToken
	}

	importSiteManual(request: SiteManualImportRequest): Promise<SiteManualImportResponse> {
		return this.request('POST', '/api/memory/site-manuals/import', request)
	}

	listSiteManuals(query: SiteManualListQuery = {}): Promise<SiteManualListResponse> {
		const search = new URLSearchParams()
		for (const key of ['projectId', 'site', 'module', 'status'] as const) {
			const value = query[key]
			if (value && value !== 'all') search.set(key, value)
		}
		const suffix = search.toString() ? `?${search.toString()}` : ''
		return this.request('GET', `/api/memory/site-manuals${suffix}`)
	}

	getSiteManual(id: string): Promise<SiteManualSource> {
		return this.request('GET', `/api/memory/site-manuals/${encodeURIComponent(id)}`)
	}

	getSiteManualWiki(id: string): Promise<SiteManualWikiResponse> {
		return this.request('GET', `/api/memory/site-manuals/${encodeURIComponent(id)}/wiki`)
	}

	rebuildSiteManual(id: string): Promise<Record<string, unknown>> {
		return this.request('POST', `/api/memory/site-manuals/${encodeURIComponent(id)}/rebuild`)
	}

	disableSiteManual(id: string): Promise<Record<string, unknown>> {
		return this.request('POST', `/api/memory/site-manuals/${encodeURIComponent(id)}/disable`)
	}

	enableSiteManual(id: string): Promise<Record<string, unknown>> {
		return this.request('POST', `/api/memory/site-manuals/${encodeURIComponent(id)}/enable`)
	}

	deleteSiteManual(id: string): Promise<Record<string, unknown>> {
		return this.request('DELETE', `/api/memory/site-manuals/${encodeURIComponent(id)}`)
	}

	previewContext(request: SiteManualPreviewRequest): Promise<SiteManualPreviewContext> {
		return this.request('POST', '/api/memory/site-manuals/preview-context', request)
	}

	private async request<TResponse>(
		method: 'GET' | 'POST' | 'DELETE',
		path: string,
		body?: unknown
	): Promise<TResponse> {
		const response = await fetch(`${this.baseUrl}${path}`, {
			method,
			headers: {
				...(body ? { 'content-type': 'application/json' } : {}),
				...(this.bearerToken ? { Authorization: `Bearer ${this.bearerToken}` } : {}),
			},
			...(body ? { body: JSON.stringify(body) } : {}),
		})

		if (!response.ok) {
			throw new Error(`Site manual request failed with status ${response.status}`)
		}

		if (response.status === 204) return {} as TResponse
		return (await response.json()) as TResponse
	}
}
