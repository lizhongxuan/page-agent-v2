import { type InteractionEvent, emitInteractionResponse } from '../interactionTypes'

type HandoverEvent = Extract<InteractionEvent, { type: 'handover' }>

export function HandoverBanner({ event }: { event: HandoverEvent }) {
	return (
		<div
			style={{
				position: 'fixed',
				left: '50%',
				top: 24,
				transform: 'translateX(-50%)',
				width: 'min(760px, calc(100vw - 48px))',
				padding: '14px 16px',
				borderRadius: 8,
				background: '#111827',
				color: '#fff',
				boxShadow: '0 16px 50px rgba(0,0,0,.24)',
				pointerEvents: 'auto',
				fontFamily: 'system-ui, -apple-system, BlinkMacSystemFont, sans-serif',
			}}
		>
			<div style={{ fontWeight: 700, marginBottom: 6 }}>{event.title}</div>
			<div style={{ fontSize: 13, lineHeight: 1.55, color: '#e5e7eb', marginBottom: 12 }}>
				{event.message}
			</div>
			<button
				data-webops-handover-done="true"
				onClick={() => emitInteractionResponse(event.requestId, { type: 'handover_done' })}
				style={{
					padding: '8px 12px',
					borderRadius: 8,
					border: 0,
					background: '#fff',
					color: '#111827',
					fontWeight: 700,
					cursor: 'pointer',
					font: 'inherit',
				}}
			>
				{event.resumeButtonLabel}
			</button>
		</div>
	)
}
