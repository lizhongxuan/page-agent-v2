import { formatMemoryForPrompt } from './promptFormatter'
import type {
	MemoryClientLike,
	MemoryContextRequest,
	MemoryContextResponse,
	MemoryEvidenceRef,
} from './types'

export class MemoryContextProvider {
	private readonly client: MemoryClientLike | undefined

	constructor(client: MemoryClientLike | undefined) {
		this.client = client
	}

	async getContext(request: MemoryContextRequest): Promise<{
		promptContext: string
		evidence: MemoryEvidenceRef[]
		response?: MemoryContextResponse
	}> {
		if (!this.client?.getContext) return { promptContext: '', evidence: [] }

		const response = await this.client.getContext(request)
		return {
			promptContext: formatMemoryForPrompt(response),
			evidence: evidenceFromContextResponse(response),
			response,
		}
	}
}

function evidenceFromContextResponse(response: MemoryContextResponse): MemoryEvidenceRef[] {
	return response.evidenceRefs?.map((item) => ({ ...item })) ?? []
}
