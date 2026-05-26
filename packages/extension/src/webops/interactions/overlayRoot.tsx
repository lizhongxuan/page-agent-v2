import { type Root, createRoot } from 'react-dom/client'

import { ActionSpotlight } from './components/ActionSpotlight'
import { AnchorBubble } from './components/AnchorBubble'
import { HandoverBanner } from './components/HandoverBanner'
import { InputPrompt } from './components/InputPrompt'
import { ProgressToast } from './components/ProgressToast'
import { createInteractionBus } from './interactionBus'
import { type InteractionEvent, WEBOPS_INTERACTION_RESPONSE_EVENT } from './interactionTypes'

export const webOpsInteractionBus = createInteractionBus()

let root: Root | undefined
let host: HTMLDivElement | undefined
let unsubscribe: (() => void) | undefined
let clearTimer: number | undefined
let responseListener: (() => void) | undefined

export function mountWebOpsOverlay() {
	if (root) return

	host = document.createElement('div')
	host.id = 'page-agent-v2-webops-overlay'
	host.style.position = 'fixed'
	host.style.inset = '0'
	host.style.pointerEvents = 'none'
	host.style.zIndex = '2147483647'
	;(document.body ?? document.documentElement).appendChild(host)

	root = createRoot(host)
	renderInteraction(undefined)
	unsubscribe = webOpsInteractionBus.subscribe((event) => {
		renderInteraction(event)

		if ('timeoutMs' in event && event.timeoutMs) {
			if (clearTimer) window.clearTimeout(clearTimer)
			clearTimer = window.setTimeout(() => renderInteraction(undefined), event.timeoutMs)
		}
	})
	responseListener = () => renderInteraction(undefined)
	window.addEventListener(WEBOPS_INTERACTION_RESPONSE_EVENT, responseListener)
}

export function unmountWebOpsOverlay() {
	if (clearTimer) window.clearTimeout(clearTimer)
	clearTimer = undefined
	if (responseListener)
		window.removeEventListener(WEBOPS_INTERACTION_RESPONSE_EVENT, responseListener)
	responseListener = undefined
	unsubscribe?.()
	unsubscribe = undefined
	root?.unmount()
	root = undefined
	host?.remove()
	host = undefined
}

function renderInteraction(event: InteractionEvent | undefined) {
	root?.render(<OverlayApp event={event} />)
}

function OverlayApp({ event }: { event?: InteractionEvent }) {
	if (!event) return null

	if (event.type === 'spotlight') return <ActionSpotlight event={event} />
	if (event.type === 'choice') return <AnchorBubble event={event} />
	if (event.type === 'handover') return <HandoverBanner event={event} />
	if (event.type === 'input') return <InputPrompt event={event} />
	if (event.type === 'toast') return <ProgressToast event={event} />

	return null
}
