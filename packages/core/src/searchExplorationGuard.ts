const DEFAULT_MAX_SEARCH_RESULT_PAGES = 2

export interface SearchExplorationState {
	signature: string | null
	visitedPageKeys: Set<string>
	limitObservationEmitted: boolean
	blockedPaginationAttempts: number
}

interface SearchPageInfo {
	signature: string
	pageKey: string
}

export function createSearchExplorationState(): SearchExplorationState {
	return {
		signature: null,
		visitedPageKeys: new Set(),
		limitObservationEmitted: false,
		blockedPaginationAttempts: 0,
	}
}

export function observeSearchPage(
	state: SearchExplorationState,
	url: string,
	task: string
): string | null {
	const page = parseSearchPage(url)
	if (!page) return null

	if (state.signature !== page.signature) {
		state.signature = page.signature
		state.visitedPageKeys = new Set()
		state.limitObservationEmitted = false
		state.blockedPaginationAttempts = 0
	}

	state.visitedPageKeys.add(page.pageKey)

	if (
		shouldApplyDefaultSearchLimit(task) &&
		state.visitedPageKeys.size >= DEFAULT_MAX_SEARCH_RESULT_PAGES &&
		!state.limitObservationEmitted
	) {
		state.limitObservationEmitted = true
		return (
			`Search exploration limit reached after ${state.visitedPageKeys.size} result pages. ` +
			`If the collected visible results are enough, call done with a concise summary. ` +
			`If the user really wants more pages, call ask_user before continuing.`
		)
	}

	return null
}

export function shouldBlockSearchPaginationClick({
	state,
	task,
	browserContent,
	index,
}: {
	state: SearchExplorationState
	task: string
	browserContent: string
	index: number
}): string | null {
	if (!shouldApplyDefaultSearchLimit(task)) return null
	if (state.visitedPageKeys.size < DEFAULT_MAX_SEARCH_RESULT_PAGES) return null

	const elementText = getIndexedElementText(browserContent, index)
	if (!isNextPaginationText(elementText)) return null

	state.blockedPaginationAttempts++

	return (
		`Search exploration limit reached. Do not click "${elementText || 'Next'}" again for this ` +
		`open-ended search request. Summarize what you have found with done, or ask_user whether ` +
		`the user wants to inspect more result pages.`
	)
}

function shouldApplyDefaultSearchLimit(task: string): boolean {
	return !hasExplicitSearchScope(task)
}

function hasExplicitSearchScope(task: string): boolean {
	const normalized = task.toLowerCase()
	return (
		/\d+\s*(条|篇|个|项|页|pages?|results?|items?)/i.test(normalized) ||
		/(前|top)\s*\d+/i.test(normalized) ||
		/[一二三四五六七八九十百]+\s*(条|篇|个|项|页)/.test(normalized)
	)
}

function parseSearchPage(rawUrl: string): SearchPageInfo | null {
	let url: URL
	try {
		url = new URL(rawUrl)
	} catch {
		return null
	}

	const host = url.hostname.toLowerCase()
	if (host.includes('google.') && url.pathname === '/search') {
		const query = url.searchParams.get('q') || ''
		const mode = url.searchParams.get('tbm') || 'all'
		const page = url.searchParams.get('start') || '0'
		if (!query) return null
		return {
			signature: `google:${mode}:${query}`,
			pageKey: `google:${mode}:${query}:${page}`,
		}
	}

	if (host.endsWith('baidu.com') && url.pathname.startsWith('/s')) {
		const query = url.searchParams.get('wd') || url.searchParams.get('word') || ''
		const page = url.searchParams.get('pn') || '0'
		if (!query) return null
		return {
			signature: `baidu:${query}`,
			pageKey: `baidu:${query}:${page}`,
		}
	}

	if (host.endsWith('bing.com') && url.pathname === '/search') {
		const query = url.searchParams.get('q') || ''
		const page = url.searchParams.get('first') || '0'
		if (!query) return null
		return {
			signature: `bing:${query}`,
			pageKey: `bing:${query}:${page}`,
		}
	}

	return null
}

function getIndexedElementText(browserContent: string, index: number): string {
	const escapedIndex = String(index).replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
	const line = browserContent
		.split('\n')
		.find((contentLine) => new RegExp(`\\[${escapedIndex}\\]`).test(contentLine))
	if (!line) return ''

	return line
		.replace(/^\s*\*?\[\d+\]\s*/, '')
		.replace(/<[^>]*>/g, ' ')
		.replace(/\s+/g, ' ')
		.trim()
}

function isNextPaginationText(text: string): boolean {
	const normalized = text.trim().toLowerCase()
	if (!normalized) return false

	return ['next', 'next page', '下一页', '下页', '更多结果', 'more results', '更多'].some(
		(pattern) => normalized === pattern || normalized.includes(pattern)
	)
}
