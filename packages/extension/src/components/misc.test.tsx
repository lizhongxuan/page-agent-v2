// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it } from 'vitest'

import { EmptyState } from './misc'

;(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true
;(globalThis as any).IntersectionObserver = class {
	observe() {}
	unobserve() {}
	disconnect() {}
}

describe('EmptyState', () => {
	it('shows only the configured GitHub link in the new-session empty state', () => {
		document.body.innerHTML = '<div id="root"></div>'

		render(<EmptyState />)

		const links = Array.from(document.querySelectorAll<HTMLAnchorElement>('a[target="_blank"]'))
		expect(links).toHaveLength(1)
		expect(links[0]?.getAttribute('href')).toBe('https://github.com/lizhongxuan')
		expect(links[0]?.getAttribute('title')).toBe('GitHub')
	})
})

function render(node: React.ReactNode) {
	const container = document.getElementById('root')
	if (!container) throw new Error('Missing test root')
	const root = createRoot(container)
	act(() => root.render(node))
	return root
}
