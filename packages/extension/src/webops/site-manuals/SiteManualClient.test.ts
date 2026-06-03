import { afterEach, describe, expect, it, vi } from 'vitest'

import { SiteManualClient } from './SiteManualClient'

const originalFetch = globalThis.fetch

describe('SiteManualClient', () => {
	afterEach(() => {
		globalThis.fetch = originalFetch
		vi.unstubAllGlobals()
	})

	it('posts imports to the site manual import endpoint with auth', async () => {
		const calls: { url: string; init?: RequestInit }[] = []
		vi.stubGlobal('fetch', async (url: string | URL | Request, init?: RequestInit) => {
			calls.push({ url: requestURL(url), init })
			return jsonResponse({ ok: true, sourceId: 'manual_1' })
		})

		const client = new SiteManualClient({
			baseUrl: 'https://memory.example.test///',
			bearerToken: 'secret',
		})
		const response = await client.importSiteManual({
			projectId: 'default',
			site: 'ops.example.test',
			title: 'Backup manual',
			sourceType: 'markdown',
			content: '# Restore',
		})

		expect(response.sourceId).toBe('manual_1')
		expect(calls[0]?.url).toBe('https://memory.example.test/api/memory/site-manuals/import')
		expect(calls[0]?.init?.method).toBe('POST')
		expect((calls[0]?.init?.headers as Record<string, string>).Authorization).toBe('Bearer secret')
		expect(JSON.parse(calls[0]?.init?.body as string).site).toBe('ops.example.test')
	})

	it('uses the documented URLs for list, get, wiki, lifecycle, delete, and preview', async () => {
		const calls: { url: string; init?: RequestInit }[] = []
		vi.stubGlobal('fetch', async (url: string | URL | Request, init?: RequestInit) => {
			calls.push({ url: requestURL(url), init })
			return jsonResponse({ ok: true })
		})

		const client = new SiteManualClient({ baseUrl: 'https://memory.example.test' })
		await client.listSiteManuals({
			projectId: 'default',
			site: 'ops.example.test',
			status: 'active',
		})
		await client.getSiteManual('manual_1')
		await client.getSiteManualWiki('manual_1')
		await client.rebuildSiteManual('manual_1')
		await client.disableSiteManual('manual_1')
		await client.enableSiteManual('manual_1')
		await client.deleteSiteManual('manual_1')
		await client.previewContext({
			projectId: 'default',
			site: 'ops.example.test',
			task: 'restore backup',
			url: 'https://ops.example.test/backup',
		})

		expect(calls.map((call) => `${call.init?.method ?? 'GET'} ${call.url}`)).toEqual([
			'GET https://memory.example.test/api/memory/site-manuals?projectId=default&site=ops.example.test&status=active',
			'GET https://memory.example.test/api/memory/site-manuals/manual_1',
			'GET https://memory.example.test/api/memory/site-manuals/manual_1/wiki',
			'POST https://memory.example.test/api/memory/site-manuals/manual_1/rebuild',
			'POST https://memory.example.test/api/memory/site-manuals/manual_1/disable',
			'POST https://memory.example.test/api/memory/site-manuals/manual_1/enable',
			'DELETE https://memory.example.test/api/memory/site-manuals/manual_1',
			'POST https://memory.example.test/api/memory/site-manuals/preview-context',
		])
	})
})

function requestURL(url: string | URL | Request): string {
	if (typeof url === 'string') return url
	if (url instanceof URL) return url.href
	return url.url
}

function jsonResponse(value: unknown) {
	return new Response(JSON.stringify(value), {
		status: 200,
		headers: { 'content-type': 'application/json' },
	})
}
