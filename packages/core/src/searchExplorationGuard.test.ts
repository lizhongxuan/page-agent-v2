import { describe, expect, it } from 'vitest'

import {
	createSearchExplorationState,
	observeSearchPage,
	shouldBlockSearchPaginationClick,
} from './searchExplorationGuard'

describe('searchExplorationGuard', () => {
	it('blocks another search-result next click after the default exploration limit', () => {
		const state = createSearchExplorationState()

		observeSearchPage(
			state,
			'https://www.google.com/search?q=%E7%99%BE%E5%BA%A6%E6%9C%80%E8%BF%91%E6%96%B0%E9%97%BB&tbm=nws',
			'能多搜索一下吗?'
		)
		observeSearchPage(
			state,
			'https://www.google.com/search?q=%E7%99%BE%E5%BA%A6%E6%9C%80%E8%BF%91%E6%96%B0%E9%97%BB&tbm=nws&start=10',
			'能多搜索一下吗?'
		)

		const message = shouldBlockSearchPaginationClick({
			state,
			task: '能多搜索一下吗?',
			browserContent: '[56]<a>Next</a>',
			index: 56,
		})

		expect(message).toContain('Search exploration limit reached')
	})

	it('does not block when the user provided an explicit result count', () => {
		const state = createSearchExplorationState()

		observeSearchPage(state, 'https://www.google.com/search?q=baidu&tbm=nws', '搜索前 50 条新闻')
		observeSearchPage(
			state,
			'https://www.google.com/search?q=baidu&tbm=nws&start=10',
			'搜索前 50 条新闻'
		)

		const message = shouldBlockSearchPaginationClick({
			state,
			task: '搜索前 50 条新闻',
			browserContent: '[56]<a>Next</a>',
			index: 56,
		})

		expect(message).toBeNull()
	})
})
