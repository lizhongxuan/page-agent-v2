import { useRef } from 'react'

import { type InteractionEvent, emitInteractionResponse } from '../interactionTypes'

type InputEvent = Extract<InteractionEvent, { type: 'input' }>

export function InputPrompt({ event }: { event: InputEvent }) {
	const inputRef = useRef<HTMLTextAreaElement>(null)
	const submitLabel = event.submitButtonLabel ?? '提交并继续'
	const submit = () => {
		emitInteractionResponse(event.requestId, {
			type: 'input',
			value: inputRef.current?.value.trim() ?? '',
		})
	}

	return (
		<div
			data-webops-input-prompt="true"
			style={{
				position: 'fixed',
				left: '50%',
				top: 24,
				transform: 'translateX(-50%)',
				width: 'min(720px, calc(100vw - 48px))',
				padding: '14px 16px',
				borderRadius: 8,
				border: '1px solid #d1d5db',
				background: '#fff',
				color: '#111827',
				boxShadow: '0 16px 50px rgba(0,0,0,.2)',
				pointerEvents: 'auto',
				fontFamily: 'system-ui, -apple-system, BlinkMacSystemFont, sans-serif',
			}}
		>
			<div style={{ fontWeight: 700, marginBottom: 6 }}>{event.title}</div>
			<div style={{ fontSize: 13, lineHeight: 1.55, color: '#4b5563', marginBottom: 10 }}>
				{event.message}
			</div>
			<textarea
				ref={inputRef}
				autoFocus
				placeholder={event.placeholder ?? '请输入补充信息'}
				onKeyDown={(e) => {
					if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
						submit()
					}
				}}
				style={{
					width: '100%',
					minHeight: 78,
					boxSizing: 'border-box',
					resize: 'vertical',
					padding: 10,
					borderRadius: 8,
					border: '1px solid #d1d5db',
					font: 'inherit',
					fontSize: 13,
					marginBottom: 10,
				}}
			/>
			<div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
				<button
					type="button"
					onClick={() =>
						emitInteractionResponse(event.requestId, {
							type: 'cancelled',
							reason: 'user_cancelled',
						})
					}
					style={{
						padding: '8px 12px',
						borderRadius: 8,
						border: '1px solid #d1d5db',
						background: '#fff',
						color: '#111827',
						cursor: 'pointer',
						font: 'inherit',
					}}
				>
					取消
				</button>
				<button
					type="button"
					data-webops-input-submit="true"
					onClick={submit}
					style={{
						padding: '8px 12px',
						borderRadius: 8,
						border: 0,
						background: '#111827',
						color: '#fff',
						cursor: 'pointer',
						font: 'inherit',
						fontWeight: 700,
					}}
				>
					{submitLabel}
				</button>
			</div>
		</div>
	)
}
