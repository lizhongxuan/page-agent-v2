import type { InteractionEvent } from '../interactionTypes'

type ToastEvent = Extract<InteractionEvent, { type: 'toast' }>

const levelBackground = {
	info: '#111827',
	success: '#166534',
	warning: '#92400e',
	error: '#991b1b',
} satisfies Record<ToastEvent['level'], string>

export function ProgressToast({ event }: { event: ToastEvent }) {
	return (
		<div
			style={{
				position: 'fixed',
				right: 24,
				bottom: 24,
				maxWidth: 360,
				padding: '10px 12px',
				borderRadius: 8,
				background: levelBackground[event.level],
				color: '#fff',
				fontSize: 13,
				lineHeight: 1.45,
				boxShadow: '0 10px 30px rgba(0,0,0,.18)',
				pointerEvents: 'none',
				fontFamily: 'system-ui, -apple-system, BlinkMacSystemFont, sans-serif',
			}}
		>
			{event.message}
		</div>
	)
}
