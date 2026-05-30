import type { MemoryContextResponse } from './types'

export function formatMemoryForPrompt(response: MemoryContextResponse): string {
	return response.contextPrompt ?? ''
}
