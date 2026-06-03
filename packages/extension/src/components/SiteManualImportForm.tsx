import { Upload } from 'lucide-react'
import { useState } from 'react'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import type { SiteManualImportRequest, SiteManualSourceType } from '@/webops/site-manuals/types'

interface SiteManualImportFormProps {
	projectId: string
	defaultSite?: string
	onImport: (request: SiteManualImportRequest) => Promise<void> | void
}

export function SiteManualImportForm({
	projectId,
	defaultSite = '',
	onImport,
}: SiteManualImportFormProps) {
	const [site, setSite] = useState(defaultSite)
	const [module, setModule] = useState('')
	const [title, setTitle] = useState('')
	const [sourceType, setSourceType] = useState<SiteManualSourceType>('markdown')
	const [content, setContent] = useState('')
	const [error, setError] = useState('')
	const [importing, setImporting] = useState(false)

	const handleSubmit = async (event: React.SyntheticEvent<HTMLFormElement>) => {
		event.preventDefault()
		const trimmedSite = site.trim()
		const trimmedTitle = title.trim()
		const trimmedContent = content.trim()
		if (!trimmedSite) {
			setError('Site is required')
			return
		}
		if (!trimmedTitle) {
			setError('Title is required')
			return
		}
		if (!trimmedContent) {
			setError('Manual content is required')
			return
		}

		setError('')
		setImporting(true)
		try {
			await onImport({
				projectId,
				site: trimmedSite,
				module: module.trim() || undefined,
				title: trimmedTitle,
				sourceType,
				content: trimmedContent,
			})
			setTitle('')
			setContent('')
		} catch (reason) {
			setError(reason instanceof Error ? reason.message : 'Failed to import site manual')
		} finally {
			setImporting(false)
		}
	}

	return (
		<form onSubmit={handleSubmit} className="grid gap-2 rounded-md border bg-muted/20 p-3">
			<div className="grid grid-cols-2 gap-2">
				<label className="grid gap-1 text-[11px] text-muted-foreground">
					Site
					<Input
						name="site"
						value={site}
						onChange={(event) => setSite(event.target.value)}
						placeholder="ops.example.test"
						className="h-8 text-xs"
					/>
				</label>
				<label className="grid gap-1 text-[11px] text-muted-foreground">
					Module
					<Input
						name="module"
						value={module}
						onChange={(event) => setModule(event.target.value)}
						placeholder="backup"
						className="h-8 text-xs"
					/>
				</label>
			</div>
			<div className="grid grid-cols-[1fr_120px] gap-2">
				<label className="grid gap-1 text-[11px] text-muted-foreground">
					Title
					<Input
						name="title"
						value={title}
						onChange={(event) => setTitle(event.target.value)}
						placeholder="Backup and restore manual"
						className="h-8 text-xs"
					/>
				</label>
				<label className="grid gap-1 text-[11px] text-muted-foreground">
					Type
					<select
						name="sourceType"
						value={sourceType}
						onChange={(event) => setSourceType(event.target.value as SiteManualSourceType)}
						className="h-8 rounded-md border border-input bg-background px-2 text-xs"
					>
						<option value="markdown">Markdown</option>
						<option value="text">Text</option>
						<option value="html">HTML</option>
						<option value="pdf_text">PDF text</option>
					</select>
				</label>
			</div>
			<label className="grid gap-1 text-[11px] text-muted-foreground">
				Manual Content
				<Textarea
					name="content"
					value={content}
					onChange={(event) => setContent(event.target.value)}
					placeholder="Paste the site manual here..."
					className="min-h-28 text-xs"
				/>
			</label>
			<div className="flex items-center justify-between gap-2">
				<div role="alert" className="min-h-4 text-[11px] text-destructive">
					{error}
				</div>
				<Button type="submit" size="sm" disabled={importing} className="h-8 text-xs">
					<Upload className="size-3" />
					{importing ? 'Importing' : 'Import'}
				</Button>
			</div>
		</form>
	)
}
