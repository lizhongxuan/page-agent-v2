import { Clipboard, Search } from 'lucide-react'
import { useEffect, useState } from 'react'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type {
	SiteManualClientLike,
	SiteManualPreviewContext,
	SiteManualWikiChunk,
	SiteManualWikiPage,
} from '@/webops/site-manuals/types'

interface SiteManualPreviewProps {
	client: SiteManualClientLike
	projectId: string
	defaultSite?: string
	defaultUrl?: string
	module?: string
	pages?: SiteManualWikiPage[]
	chunks?: SiteManualWikiChunk[]
}

export function SiteManualPreview({
	client,
	projectId,
	defaultSite = '',
	defaultUrl = '',
	module = '',
	pages = [],
	chunks = [],
}: SiteManualPreviewProps) {
	const [site, setSite] = useState(defaultSite)
	const [moduleValue, setModuleValue] = useState(module)
	const [task, setTask] = useState('')
	const [url, setUrl] = useState(defaultUrl)
	const [preview, setPreview] = useState<SiteManualPreviewContext | null>(null)
	const [error, setError] = useState('')

	useEffect(() => {
		if (defaultSite) setSite(defaultSite)
		if (defaultUrl) setUrl(defaultUrl)
	}, [defaultSite, defaultUrl])

	useEffect(() => {
		if (defaultSite && defaultUrl) return
		let cancelled = false
		getCurrentTabSiteDefaults()
			.then((defaults) => {
				if (cancelled) return
				if (!defaultSite && defaults.site) setSite((current) => current || defaults.site)
				if (!defaultUrl && defaults.url) setUrl((current) => current || defaults.url)
			})
			.catch(() => {
				// Current tab defaults are opportunistic; manual input remains available.
			})
		return () => {
			cancelled = true
		}
	}, [defaultSite, defaultUrl])

	const handlePreview = async (event: React.SyntheticEvent<HTMLFormElement>) => {
		event.preventDefault()
		if (!client.previewContext) return
		setError('')
		try {
			const result = await client.previewContext({
				projectId,
				site: site.trim(),
				module: moduleValue.trim() || undefined,
				task: task.trim(),
				url: url.trim(),
			})
			setPreview(result)
		} catch (reason) {
			setError(reason instanceof Error ? reason.message : 'Failed to preview site manual context')
		}
	}

	const copyPrompt = async () => {
		if (preview?.prompt) await navigator.clipboard.writeText(preview.prompt)
	}

	return (
		<div className="grid gap-3">
			{pages.length > 0 && (
				<section className="grid gap-2">
					<h3 className="text-xs font-semibold">Wiki Pages</h3>
					<div className="grid gap-2">
						{pages.map((page) => (
							<article key={page.id} className="rounded-md border p-2 text-xs">
								<div className="flex items-center justify-between gap-2">
									<div className="font-medium">{page.title}</div>
									<span className="text-[10px] text-muted-foreground">
										{page.status ?? 'active'}
									</span>
								</div>
								{page.summary && <p className="mt-1 text-muted-foreground">{page.summary}</p>}
								<JsonLine label="Facts" value={page.facts} />
								<JsonLine label="Procedures" value={page.procedures} />
								<JsonLine label="Source refs" value={page.sourceRefs} />
							</article>
						))}
					</div>
				</section>
			)}

			{chunks.length > 0 && (
				<section className="grid gap-2">
					<h3 className="text-xs font-semibold">Retrievable Chunks</h3>
					<div className="grid gap-2">
						{chunks.map((chunk) => (
							<article key={chunk.id} className="rounded-md border p-2 text-xs">
								<div className="flex items-center justify-between gap-2">
									<div className="font-medium">{chunk.chunkType ?? 'chunk'}</div>
									<span className="text-[10px] text-muted-foreground">
										{chunk.status ?? 'active'}
									</span>
								</div>
								<p className="mt-1 text-muted-foreground">{chunk.text}</p>
								<JsonLine label="Page guards" value={chunk.pageGuards} />
								<JsonLine label="Target terms" value={chunk.targetTerms} />
								<JsonLine label="Source refs" value={chunk.sourceRefs} />
							</article>
						))}
					</div>
				</section>
			)}

			<form onSubmit={handlePreview} className="grid gap-2 rounded-md border bg-muted/20 p-3">
				<div className="grid grid-cols-2 gap-2">
					<Input
						name="site"
						value={site}
						onChange={(event) => setSite(event.target.value)}
						placeholder="Site"
						className="h-8 text-xs"
					/>
					<Input
						name="module"
						value={moduleValue}
						onChange={(event) => setModuleValue(event.target.value)}
						placeholder="Module"
						className="h-8 text-xs"
					/>
				</div>
				<Input
					name="task"
					value={task}
					onChange={(event) => setTask(event.target.value)}
					placeholder="Task"
					className="h-8 text-xs"
				/>
				<Input
					name="url"
					value={url}
					onChange={(event) => setUrl(event.target.value)}
					placeholder="Current URL"
					className="h-8 text-xs"
				/>
				<div className="flex items-center justify-between gap-2">
					<div role="alert" className="text-[11px] text-destructive">
						{error}
					</div>
					<Button type="submit" size="sm" className="h-8 text-xs">
						<Search className="size-3" />
						Preview Context
					</Button>
				</div>
			</form>

			{preview && (
				<section className="grid gap-2 rounded-md border p-3 text-xs">
					<div className="flex items-center justify-between gap-2">
						<h3 className="font-semibold">Preview Result</h3>
						<Button
							type="button"
							variant="outline"
							size="sm"
							onClick={copyPrompt}
							disabled={!preview.prompt}
							className="h-7 text-xs"
						>
							<Clipboard className="size-3" />
							Copy Prompt
						</Button>
					</div>
					<div className="grid gap-1">
						<div className="text-[11px] font-medium text-muted-foreground">Matched chunks</div>
						{preview.matchedChunks?.map((chunk) => (
							<div key={chunk.id} className="rounded border bg-muted/20 p-2">
								{chunk.text}
							</div>
						))}
					</div>
					{preview.filtered?.length ? (
						<div className="grid gap-1">
							<div className="text-[11px] font-medium text-muted-foreground">Filtered</div>
							{preview.filtered.map((item, index) => (
								<div
									key={`${item.id ?? 'filtered'}-${index}`}
									className="rounded border bg-muted/20 p-2"
								>
									{item.id ? `${item.id}: ` : ''}
									{item.reason}
								</div>
							))}
						</div>
					) : null}
					{preview.prompt && (
						<pre className="max-h-48 overflow-auto rounded-md bg-muted/40 p-2 text-[11px] whitespace-pre-wrap">
							{preview.prompt}
						</pre>
					)}
				</section>
			)}
		</div>
	)
}

export interface CurrentTabSiteDefaults {
	site: string
	url: string
}

export async function getCurrentTabSiteDefaults(): Promise<CurrentTabSiteDefaults> {
	if (typeof chrome === 'undefined' || !chrome.tabs?.query) return { site: '', url: '' }
	const tabs = await chrome.tabs.query({ active: true, currentWindow: true })
	const url = tabs[0]?.url ?? ''
	return {
		site: siteFromUrl(url),
		url: isHttpUrl(url) ? url : '',
	}
}

function siteFromUrl(value: string): string {
	if (!isHttpUrl(value)) return ''
	try {
		return new URL(value).hostname
	} catch {
		return ''
	}
}

function isHttpUrl(value: string): boolean {
	return value.startsWith('http://') || value.startsWith('https://')
}

function JsonLine({ label, value }: { label: string; value: unknown }) {
	if (value == null || (Array.isArray(value) && value.length === 0)) return null
	return (
		<div className="mt-1 text-[11px] text-muted-foreground">
			<span className="font-medium">{label}: </span>
			<span>{formatValue(value)}</span>
		</div>
	)
}

function formatValue(value: unknown): string {
	if (typeof value === 'string') return value
	return JSON.stringify(value)
}
