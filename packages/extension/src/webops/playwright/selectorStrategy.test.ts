import { describe, expect, it } from 'vitest'

import { toPlaywrightLocator } from './selectorStrategy'

describe('toPlaywrightLocator', () => {
	it('uses test id before any other target hint', () => {
		expect(
			toPlaywrightLocator({
				testId: 'service-search',
				role: 'textbox',
				name: '按照服务名称或容器IP搜索',
			})
		).toBe("page.getByTestId('service-search')")
	})

	it('uses role and name before placeholder-like name', () => {
		expect(toPlaywrightLocator({ role: 'button', name: '刷新' })).toBe(
			"page.getByRole('button', { name: '刷新' })"
		)
	})

	it('uses placeholder-like name when role is absent', () => {
		expect(toPlaywrightLocator({ name: '按照服务名称或容器IP搜索' })).toBe(
			"page.getByPlaceholder('按照服务名称或容器IP搜索')"
		)
	})

	it('falls back through text, css, xpath, then body', () => {
		expect(toPlaywrightLocator({ text: '服务详情' })).toBe("page.getByText('服务详情')")
		expect(toPlaywrightLocator({ css: '#service-list' })).toBe("page.locator('#service-list')")
		expect(toPlaywrightLocator({ xpath: '//button[1]' })).toBe("page.locator('xpath=//button[1]')")
		expect(toPlaywrightLocator({})).toBe("page.locator('body')")
	})
})
