// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { SiteManualClientLike } from '@/webops/site-manuals/types'

import { SiteManualLibrary } from './SiteManualLibrary'

;(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true
;(globalThis as any).confirm = vi.fn(() => true)
;(globalThis as any).navigator = {
	clipboard: {
		writeText: vi.fn().mockResolvedValue(undefined),
	},
}

describe('SiteManualLibrary', () => {
	beforeEach(() => {
		;(globalThis as any).chrome = undefined
	})

	it('lists manuals and runs lifecycle actions through the client', async () => {
		document.body.innerHTML = '<div id="root"></div>'
		const listSiteManuals = vi.fn().mockResolvedValue({
			sources: [
				{
					id: 'manual_1',
					site: 'ops.example.test',
					title: 'Backup manual',
					status: 'active',
					lastCompiledAt: '2026-06-01T00:00:00Z',
				},
			],
		})
		const getSiteManualWiki = vi.fn().mockResolvedValue({
			pages: [{ id: 'page_1', title: 'Restore', summary: 'Restore workflow' }],
			chunks: [{ id: 'chunk_1', wikiPageId: 'page_1', text: 'Click Restore' }],
		})
		const disableSiteManual = vi.fn().mockResolvedValue({ ok: true })
		const deleteSiteManual = vi.fn().mockResolvedValue({ ok: true })
		const client: SiteManualClientLike = {
			listSiteManuals,
			getSiteManualWiki,
			disableSiteManual,
			deleteSiteManual,
		}

		await act(async () => {
			render(
				<SiteManualLibrary client={client} projectId="default" defaultSite="ops.example.test" />
			)
		})
		await act(async () => {})

		expect(document.body.textContent).toContain('Backup manual')
		expect(document.body.textContent).toContain('Restore workflow')

		await act(async () => {
			Array.from(document.querySelectorAll('button'))
				.find((button) => button.textContent === 'Disable')
				?.click()
		})
		expect(disableSiteManual).toHaveBeenCalledWith('manual_1')

		await act(async () => {
			Array.from(document.querySelectorAll('button'))
				.find((button) => button.textContent === 'Delete')
				?.click()
		})
		expect(deleteSiteManual).toHaveBeenCalledWith('manual_1')
	})

	it('uses the current tab URL and host as defaults when explicit defaults are absent', async () => {
		document.body.innerHTML = '<div id="root"></div>'
		;(globalThis as any).chrome = {
			tabs: {
				query: vi.fn().mockResolvedValue([
					{
						url: 'https://docs.example.test/admin/manuals?section=restore',
					},
				]),
			},
		}
		const listSiteManuals = vi.fn().mockResolvedValue({
			sources: [
				{
					id: 'manual_1',
					site: 'docs.example.test',
					title: 'Admin manual',
					status: 'active',
				},
			],
		})
		const getSiteManualWiki = vi.fn().mockResolvedValue({ pages: [], chunks: [] })
		const previewContext = vi.fn().mockResolvedValue({
			matchedChunks: [],
			prompt: '<site_manual_knowledge />',
		})
		const client: SiteManualClientLike = {
			listSiteManuals,
			getSiteManualWiki,
			previewContext,
		}

		await act(async () => {
			render(<SiteManualLibrary client={client} projectId="default" />)
		})
		await act(async () => {})

		expect(listSiteManuals).toHaveBeenLastCalledWith({
			projectId: 'default',
			site: 'docs.example.test',
			module: undefined,
			status: 'active',
		})
		expect(document.querySelector<HTMLInputElement>('input[placeholder="Site"]')!.value).toBe(
			'docs.example.test'
		)
		expect(document.querySelector<HTMLInputElement>('input[name="url"]')!.value).toBe(
			'https://docs.example.test/admin/manuals?section=restore'
		)
	})
})

function render(node: React.ReactNode) {
	const container = document.getElementById('root')
	if (!container) throw new Error('Missing test root')
	const root = createRoot(container)
	root.render(node)
	return root
}
