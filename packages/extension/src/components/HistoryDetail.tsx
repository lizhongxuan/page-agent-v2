import { ArrowLeft, Database, RotateCcw, Trash2 } from 'lucide-react'
import type { ReactNode } from 'react'
import { useEffect, useState } from 'react'

import { Button } from '@/components/ui/button'
import { type SessionRecord, deleteSession, getSession } from '@/lib/db'
import type { MemorySurfaceSummary } from '@/webops/memory/types'

import { EventCard } from './cards'

export function HistoryDetail({
	sessionId,
	onBack,
	onRerun,
}: {
	sessionId: string
	onBack: () => void
	onRerun: (task: string) => void
}) {
	const [session, setSession] = useState<SessionRecord | null>(null)

	useEffect(() => {
		getSession(sessionId).then((s) => setSession(s ?? null))
	}, [sessionId])

	if (!session) {
		return (
			<div className="flex items-center justify-center h-screen text-xs text-muted-foreground">
				Loading...
			</div>
		)
	}

	return (
		<div className="flex flex-col h-screen bg-background">
			{/* Header */}
			<header className="flex items-center gap-2 border-b px-3 py-2">
				<Button variant="ghost" size="icon-sm" onClick={onBack} className="cursor-pointer">
					<ArrowLeft className="size-3.5" />
				</Button>
				<span className="text-sm font-medium truncate">History</span>
			</header>

			{/* Task */}
			<div className="border-b px-3 py-2 bg-muted/30">
				<div className="text-[10px] text-muted-foreground uppercase tracking-wide">Task</div>
				<div className="text-xs font-medium" title={session.task}>
					{session.task}
				</div>
				<div className="mt-2 flex items-center gap-2">
					<button
						type="button"
						onClick={() => onRerun(session.task)}
						className="flex items-center gap-1 text-[10px] text-muted-foreground hover:text-foreground transition-colors cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
					>
						<RotateCcw className="size-3" />
						Run again
					</button>
					<button
						type="button"
						onClick={async () => {
							await deleteSession(sessionId)
							onBack()
						}}
						className="flex items-center gap-1 text-[10px] text-muted-foreground hover:text-destructive transition-colors cursor-pointer"
					>
						<Trash2 className="size-3" />
						Delete
					</button>
				</div>
			</div>

			{/* Events (read-only) */}
			<div className="flex-1 overflow-y-auto p-3 space-y-2">
				<MemoryContextSection session={session} />
				{session.history.map((event, index) => (
					<EventCard key={index} event={event} />
				))}
			</div>
		</div>
	)
}

function MemoryContextSection({ session }: { session: SessionRecord }) {
	const memoryContext = session.webOpsSession?.memoryContext
	const memoryUpdates = session.webOpsSession?.memoryUpdates ?? []
	if (!memoryContext && memoryUpdates.length === 0) return null

	return (
		<section className="rounded-lg border bg-muted/20 p-3 text-xs">
			<div className="mb-2 flex items-center justify-between gap-2">
				<div className="flex items-center gap-2 font-medium">
					<Database className="size-3.5" />
					<span>Workflow Memory</span>
				</div>
				<div className="text-[10px] text-muted-foreground">
					{memoryContext?.recommendedMode ?? 'normal'}
				</div>
			</div>

			{memoryContext && (
				<div className="space-y-2">
					<div className="grid grid-cols-[84px_1fr] gap-x-2 gap-y-1 text-[11px]">
						<span className="text-muted-foreground">Context</span>
						<span className="truncate" title={memoryContext.contextId}>
							{memoryContext.contextId || '-'}
						</span>
						<span className="text-muted-foreground">Page</span>
						<span className="truncate" title={memoryContext.currentPageState?.id}>
							{memoryContext.currentPageState?.name || memoryContext.currentPageState?.id || '-'}
						</span>
						<span className="text-muted-foreground">Surface</span>
						<span className="truncate" title={memoryContext.currentSurface?.id}>
							{formatSurface(memoryContext.currentSurface)}
						</span>
					</div>

					<MemoryDetails title="Injected prompt">
						<pre className="max-h-64 overflow-auto whitespace-pre-wrap rounded border bg-background p-2 text-[10px] leading-relaxed">
							{memoryContext.contextPrompt || '(empty)'}
						</pre>
					</MemoryDetails>

					<MemoryDetails title={`Evidence refs (${memoryContext.evidenceRefs.length})`}>
						<div className="space-y-2">
							{memoryContext.evidenceRefs.map((ref) => (
								<div key={`${ref.source}:${ref.id}`} className="rounded border bg-background p-2">
									<div className="flex items-center justify-between gap-2">
										<span className="truncate font-medium" title={ref.title || ref.id}>
											{ref.title || ref.id}
										</span>
										<span className="text-[10px] text-muted-foreground">
											{ref.source}
											{ref.score !== undefined ? ` / ${ref.score.toFixed(2)}` : ''}
										</span>
									</div>
									<div className="mt-1 truncate text-[10px] text-muted-foreground" title={ref.id}>
										ID: {ref.id}
									</div>
									{ref.reason && (
										<div className="mt-1 text-[10px] text-muted-foreground">{ref.reason}</div>
									)}
									{ref.matchedRules?.length ? (
										<div className="mt-1 text-[10px] text-muted-foreground">
											Rules: {ref.matchedRules.join(', ')}
										</div>
									) : null}
									{ref.payload && (
										<pre className="mt-2 max-h-40 overflow-auto whitespace-pre-wrap rounded bg-muted/50 p-2 text-[10px]">
											{JSON.stringify(ref.payload, null, 2)}
										</pre>
									)}
								</div>
							))}
						</div>
					</MemoryDetails>

					{memoryContext.debug && (
						<MemoryDetails title="Retrieval debug">
							<pre className="max-h-56 overflow-auto whitespace-pre-wrap rounded border bg-background p-2 text-[10px] leading-relaxed">
								{JSON.stringify(memoryContext.debug, null, 2)}
							</pre>
						</MemoryDetails>
					)}
				</div>
			)}

			{memoryUpdates.length > 0 && (
				<div className="mt-2 text-[10px] text-muted-foreground">
					Updates: {memoryUpdates.join(', ')}
				</div>
			)}
		</section>
	)
}

function MemoryDetails({ title, children }: { title: string; children: ReactNode }) {
	return (
		<details className="group">
			<summary className="cursor-pointer select-none text-[11px] font-medium text-muted-foreground hover:text-foreground">
				{title}
			</summary>
			<div className="mt-2">{children}</div>
		</details>
	)
}

function formatSurface(surface: MemorySurfaceSummary | undefined) {
	if (!surface) return '-'
	return [surface.name, surface.type, surface.id].filter(Boolean).join(' / ') || '-'
}
