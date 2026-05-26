import type { BrowserState } from '@page-agent/page-controller'

export function normalizeBrowserStateResponse(
	response: unknown,
	currentUrl: string,
	currentTitle: string
): BrowserState {
	if (isBrowserState(response)) return response

	const reason = getBrowserStateFailureReason(response)

	return {
		url: currentUrl,
		title: currentTitle,
		header: '',
		content: `(page state unavailable: ${reason})`,
		footer: '',
	}
}

function isBrowserState(value: unknown): value is BrowserState {
	if (!value || typeof value !== 'object') return false

	const candidate = value as BrowserState

	return (
		typeof candidate.url === 'string' &&
		typeof candidate.title === 'string' &&
		typeof candidate.header === 'string' &&
		typeof candidate.content === 'string' &&
		typeof candidate.footer === 'string'
	)
}

function getBrowserStateFailureReason(response: unknown): string {
	if (!response) return 'empty_browser_state_response'

	if (typeof response === 'object' && 'error' in response) {
		const error = (response as { error?: unknown }).error
		if (typeof error === 'string' && error.trim()) return error
	}

	return 'invalid_browser_state_response'
}
