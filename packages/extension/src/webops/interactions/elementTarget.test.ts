// @vitest-environment jsdom
import { describe, expect, it } from 'vitest'

import { getElementByIndex } from './elementTarget'

describe('getElementByIndex', () => {
	it('prefers data-page-agent-index', () => {
		document.body.innerHTML = `
			<button>fallback</button>
			<input data-page-agent-index="2" />
		`

		expect(getElementByIndex(2)).toBe(document.querySelector('[data-page-agent-index="2"]'))
	})

	it('uses data-highlight-index', () => {
		document.body.innerHTML = `<input data-highlight-index="3" />`

		expect(getElementByIndex(3)).toBe(document.querySelector('[data-highlight-index="3"]'))
	})

	it('falls back to interactive element order', () => {
		document.body.innerHTML = `
			<button>first</button>
			<input placeholder="second" />
		`

		expect(getElementByIndex(1)).toBe(document.querySelector('input'))
	})
})
