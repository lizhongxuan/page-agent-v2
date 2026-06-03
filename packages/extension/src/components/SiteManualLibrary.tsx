import { Eye, FileText, RefreshCw, RotateCcw, Search, Trash2 } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'

import { SiteManualImportForm } from '@/components/SiteManualImportForm'
import { SiteManualPreview, getCurrentTabSiteDefaults } from '@/components/SiteManualPreview'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type {
	SiteManualClientLike,
	SiteManualSource,
	SiteManualStatus,
	SiteManualWikiResponse,
} from '@/webops/site-manuals/types'

interface SiteManualLibraryProps {
	client: SiteManualClientLike
	projectId: string
	defaultSite?: string
	defaultUrl?: string
}

export function SiteManualLibrary({
	client,
	projectId,
	defaultSite = '',
	defaultUrl = '',
}: SiteManualLibraryProps) {
	const [site, setSite] = useState(defaultSite)
	const [module, setModule] = useState('')
	const [status, setStatus] = useState<SiteManualStatus | 'all'>('active')
	const [sources, setSources] = useState<SiteManualSource[]>([])
	const [selectedSource, setSelectedSource] = useState<SiteManualSource | null>(null)
	const [wiki, setWiki] = useState<SiteManualWikiResponse | null>(null)
	const [loading, setLoading] = useState(false)
	const [error, setError] = useState('')
	const [previewDefaultUrl, setPreviewDefaultUrl] = useState(defaultUrl)

	const activeSource = selectedSource ?? sources[0] ?? null

	const loadSources = useCallback(async () => {
		if (!client.listSiteManuals) return
		setLoading(true)
		setError('')
		try {
			const response = await client.listSiteManuals({
				projectId,
				site: site.trim() || undefined,
				module: module.trim() || undefined,
				status,
			})
			const nextSources = response.sources ?? response.items ?? []
			setSources(nextSources)
			setSelectedSource((current) => {
				if (current && nextSources.some((source) => source.id === current.id)) return current
				return nextSources[0] ?? null
			})
		} catch (reason) {
			setError(reason instanceof Error ? reason.message : 'Failed to load site manuals')
		} finally {
			setLoading(false)
		}
	}, [client, module, projectId, site, status])

	useEffect(() => {
		if (defaultSite) setSite(defaultSite)
		if (defaultUrl) setPreviewDefaultUrl(defaultUrl)
	}, [defaultSite, defaultUrl])

	useEffect(() => {
		if (defaultSite && defaultUrl) return
		let cancelled = false
		getCurrentTabSiteDefaults()
			.then((defaults) => {
				if (cancelled) return
				if (!defaultSite && defaults.site) setSite((current) => current || defaults.site)
				if (!defaultUrl && defaults.url) setPreviewDefaultUrl((current) => current || defaults.url)
			})
			.catch(() => {
				// Current tab defaults are a convenience only; filters can still be typed manually.
			})
		return () => {
			cancelled = true
		}
	}, [defaultSite, defaultUrl])

	useEffect(() => {
		void loadSources()
	}, [loadSources])

	useEffect(() => {
		if (!activeSource || !client.getSiteManualWiki) {
			setWiki(null)
			return
		}
		client
			.getSiteManualWiki(activeSource.id)
			.then(setWiki)
			.catch((reason) =>
				setError(reason instanceof Error ? reason.message : 'Failed to load site manual wiki')
			)
	}, [activeSource?.id])

	const selectedStatus = activeSource?.status ?? 'active'
	const canDisable = activeSource && selectedStatus !== 'disabled'
	const canEnable = activeSource && selectedStatus === 'disabled'

	const runAction = async (action: () => Promise<Record<string, unknown>> | undefined) => {
		if (!action) return
		setError('')
		try {
			await action()
			await loadSources()
		} catch (reason) {
			setError(reason instanceof Error ? reason.message : 'Site manual action failed')
		}
	}

	const deleteActive = async () => {
		if (!activeSource || !client.deleteSiteManual) return
		if (!confirm(`Delete site manual "${activeSource.title}"?`)) return
		await runAction(() => client.deleteSiteManual?.(activeSource.id))
	}

	return (
		<div className="flex h-full min-h-0 flex-col bg-background">
			<header className="flex items-center justify-between border-b px-4 py-3">
				<div>
					<h1 className="text-sm font-semibold">Site Manual Library</h1>
					<div className="text-[11px] text-muted-foreground">
						Project <span className="font-mono">{projectId}</span>
					</div>
				</div>
				<Button
					type="button"
					variant="outline"
					size="sm"
					onClick={loadSources}
					className="h-8 text-xs"
				>
					<RefreshCw className="size-3" />
					Refresh
				</Button>
			</header>

			<div className="grid min-h-0 flex-1 grid-cols-[360px_1fr]">
				<aside className="min-h-0 overflow-y-auto border-r p-3">
					<div className="mb-3 grid gap-2">
						<div className="grid grid-cols-2 gap-2">
							<Input
								value={site}
								onChange={(event) => setSite(event.target.value)}
								placeholder="Site"
								className="h-8 text-xs"
							/>
							<Input
								value={module}
								onChange={(event) => setModule(event.target.value)}
								placeholder="Module"
								className="h-8 text-xs"
							/>
						</div>
						<div className="grid grid-cols-[1fr_auto] gap-2">
							<select
								value={status}
								onChange={(event) => setStatus(event.target.value as SiteManualStatus | 'all')}
								className="h-8 rounded-md border border-input bg-background px-2 text-xs"
							>
								<option value="active">Active</option>
								<option value="disabled">Disabled</option>
								<option value="replaced">Replaced</option>
								<option value="stale">Stale</option>
								<option value="all">All</option>
							</select>
							<Button type="button" size="sm" onClick={loadSources} className="h-8 text-xs">
								<Search className="size-3" />
								Filter
							</Button>
						</div>
					</div>

					<SiteManualImportForm
						projectId={projectId}
						defaultSite={site || defaultSite}
						onImport={async (request) => {
							await client.importSiteManual?.(request)
							await loadSources()
						}}
					/>

					<div className="mt-3 grid gap-2">
						{loading && <div className="text-xs text-muted-foreground">Loading manuals...</div>}
						{error && (
							<div className="rounded-md border border-destructive/30 p-2 text-xs text-destructive">
								{error}
							</div>
						)}
						{sources.map((source) => (
							<button
								key={source.id}
								type="button"
								onClick={() => setSelectedSource(source)}
								className={`rounded-md border p-2 text-left text-xs hover:bg-muted/30 ${
									activeSource?.id === source.id ? 'border-foreground/30 bg-muted/40' : ''
								}`}
							>
								<div className="flex items-center justify-between gap-2">
									<span className="font-medium">{source.title}</span>
									<span className="text-[10px] text-muted-foreground">{source.status}</span>
								</div>
								<div className="mt-1 text-[11px] text-muted-foreground">
									{source.site}
									{source.module ? ` / ${source.module}` : ''}
								</div>
								<div className="mt-1 text-[10px] text-muted-foreground">
									Compiled:{' '}
									{source.lastCompiledAt ?? source.updatedAt ?? source.createdAt ?? 'pending'}
								</div>
								{source.compileError && (
									<div className="mt-1 text-[10px] text-destructive">{source.compileError}</div>
								)}
							</button>
						))}
					</div>
				</aside>

				<main className="min-h-0 overflow-y-auto p-4">
					{activeSource ? (
						<div className="grid gap-3">
							<section className="rounded-md border p-3">
								<div className="flex items-start justify-between gap-3">
									<div>
										<div className="flex items-center gap-2">
											<FileText className="size-4" />
											<h2 className="text-sm font-semibold">{activeSource.title}</h2>
										</div>
										<div className="mt-1 text-xs text-muted-foreground">
											{activeSource.site}
											{activeSource.module ? ` / ${activeSource.module}` : ''} ·{' '}
											{activeSource.status}
										</div>
									</div>
									<div className="flex flex-wrap justify-end gap-2">
										<Button
											type="button"
											variant="outline"
											size="sm"
											onClick={() => runAction(() => client.rebuildSiteManual?.(activeSource.id))}
											className="h-8 text-xs"
										>
											<RotateCcw className="size-3" />
											Rebuild
										</Button>
										{canDisable && (
											<Button
												type="button"
												variant="outline"
												size="sm"
												onClick={() => runAction(() => client.disableSiteManual?.(activeSource.id))}
												className="h-8 text-xs"
											>
												<Eye className="size-3" />
												Disable
											</Button>
										)}
										{canEnable && (
											<Button
												type="button"
												variant="outline"
												size="sm"
												onClick={() => runAction(() => client.enableSiteManual?.(activeSource.id))}
												className="h-8 text-xs"
											>
												<Eye className="size-3" />
												Enable
											</Button>
										)}
										<Button
											type="button"
											variant="destructive"
											size="sm"
											onClick={deleteActive}
											className="h-8 text-xs"
										>
											<Trash2 className="size-3" />
											Delete
										</Button>
									</div>
								</div>
							</section>

							<SiteManualPreview
								client={client}
								projectId={projectId}
								defaultSite={activeSource.site}
								defaultUrl={previewDefaultUrl}
								module={activeSource.module}
								pages={wiki?.pages ?? []}
								chunks={wiki?.chunks ?? []}
							/>
						</div>
					) : (
						<div className="flex h-full items-center justify-center text-sm text-muted-foreground">
							No site manuals match the current filters.
						</div>
					)}
				</main>
			</div>
		</div>
	)
}
