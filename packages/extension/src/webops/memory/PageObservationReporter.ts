import type {
	MemoryClientLike,
	MemoryPageObservation,
	MemoryPageObservationRequest,
	MemoryPageObservationResponse,
} from './types'

export class PageObservationReporter {
	private readonly client: MemoryClientLike | undefined

	constructor(client: MemoryClientLike | undefined) {
		this.client = client
	}

	async report(input: {
		projectId: string
		task: string
		url: string
		pageObservation: MemoryPageObservation
		allowPageSummary: boolean
	}): Promise<MemoryPageObservationResponse | undefined> {
		if (!this.client?.observePage) return

		const request: MemoryPageObservationRequest = {
			projectId: input.projectId,
			task: input.task,
			url: input.url,
			title: input.pageObservation.title,
			visibleText: input.allowPageSummary ? input.pageObservation.visibleText : [],
			controls: input.pageObservation.controls,
			breadcrumbs: input.pageObservation.breadcrumbs,
			activeTabs: input.pageObservation.activeTabs,
			tables: input.pageObservation.tables,
			activeSurfaces: input.pageObservation.activeSurfaces,
		}

		try {
			return await this.client.observePage(request)
		} catch (error) {
			console.warn('[WebOpsMemory] Failed to report page observation:', errorMessage(error))
		}
	}
}

function errorMessage(error: unknown): string {
	return error instanceof Error ? error.message : String(error)
}
