import { describe, expect, it } from 'vitest'

import { createInteractionBus } from './interactionBus'

describe('createInteractionBus', () => {
	it('delivers normalized interaction events to subscribers', () => {
		const bus = createInteractionBus()
		const received: { type: string; timeoutMs?: number }[] = []

		const unsubscribe = bus.subscribe((event) =>
			received.push({
				type: event.type,
				timeoutMs: 'timeoutMs' in event ? event.timeoutMs : undefined,
			})
		)
		bus.publish({ type: 'toast', message: '正在搜索', level: 'info' })
		unsubscribe()
		bus.publish({ type: 'toast', message: '不会收到', level: 'info' })

		expect(received).toEqual([{ type: 'toast', timeoutMs: 1800 }])
	})

	it('clears all subscribers', () => {
		const bus = createInteractionBus()
		const received: string[] = []

		bus.subscribe((event) => received.push(event.type))
		bus.clear()
		bus.publish({ type: 'toast', message: '不会收到', level: 'info' })

		expect(received).toEqual([])
	})
})
