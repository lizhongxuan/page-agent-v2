import type { CSSProperties } from 'react'

import { getElementRectByIndex } from '../elementTarget'
import type { InteractionEvent } from '../interactionTypes'

type SpotlightEvent = Extract<InteractionEvent, { type: 'spotlight' }>

export function ActionSpotlight({ event }: { event: SpotlightEvent }) {
	const rect = getElementRectByIndex(event.elementIndex)

	if (!rect) {
		return <div style={fallbackMessageStyle}>{event.message}</div>
	}

	const messageLeft = Math.max(12, Math.min(rect.left, window.innerWidth - 344))
	const messageTop =
		rect.bottom + 12 + 52 < window.innerHeight ? rect.bottom + 12 : Math.max(12, rect.top - 64)

	return (
		<>
			<div
				style={{
					position: 'fixed',
					left: rect.left - 4,
					top: rect.top - 4,
					width: rect.width + 8,
					height: rect.height + 8,
					border: '2px solid #2563eb',
					borderRadius: 8,
					boxShadow: '0 0 0 4px rgba(37, 99, 235, 0.14)',
					pointerEvents: 'none',
					transition: 'all 120ms ease',
				}}
			/>
			<div
				style={{
					...fallbackMessageStyle,
					left: messageLeft,
					top: messageTop,
				}}
			>
				{event.message}
			</div>
		</>
	)
}

const fallbackMessageStyle: CSSProperties = {
	position: 'fixed',
	left: 24,
	top: 24,
	maxWidth: 320,
	padding: '10px 12px',
	borderRadius: 8,
	background: '#111827',
	color: '#fff',
	fontSize: 13,
	lineHeight: 1.45,
	boxShadow: '0 10px 30px rgba(0,0,0,.18)',
	pointerEvents: 'none',
	fontFamily: 'system-ui, -apple-system, BlinkMacSystemFont, sans-serif',
}
