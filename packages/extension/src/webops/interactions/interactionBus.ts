import type { InteractionEvent } from './interactionTypes'
import { normalizeInteractionEvent } from './interactionTypes'

export type InteractionListener = (event: InteractionEvent) => void

export type InteractionBus = ReturnType<typeof createInteractionBus>

export function createInteractionBus() {
	const listeners = new Set<InteractionListener>()

	return {
		subscribe(listener: InteractionListener) {
			listeners.add(listener)

			return () => {
				listeners.delete(listener)
			}
		},
		publish(event: InteractionEvent) {
			const normalized = normalizeInteractionEvent(event)

			for (const listener of listeners) {
				listener(normalized)
			}
		},
		clear() {
			listeners.clear()
		},
	}
}
