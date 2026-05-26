// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest'

import { PAGE_LOCK_ID, hidePageLock, showPageLock, withPageLockBypassed } from './pageLock'

describe('page lock', () => {
	afterEach(() => {
		hidePageLock()
	})

	it('blocks page interaction while the agent is running', () => {
		showPageLock()

		const lock = document.getElementById(PAGE_LOCK_ID)

		expect(lock).toBeTruthy()
		expect(lock?.style.pointerEvents).toBe('auto')
	})

	it('temporarily bypasses pointer events while reading page state', async () => {
		showPageLock()
		const lock = document.getElementById(PAGE_LOCK_ID)

		const pointerEventsDuringRead = await withPageLockBypassed(
			async () => lock?.style.pointerEvents
		)

		expect(pointerEventsDuringRead).toBe('none')
		expect(lock?.style.pointerEvents).toBe('auto')
	})
})
