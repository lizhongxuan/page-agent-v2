import { useState } from 'react'

import { getElementRectByIndex } from '../elementTarget'
import { type InteractionEvent, emitInteractionResponse } from '../interactionTypes'

type ChoiceEvent = Extract<InteractionEvent, { type: 'choice' }>

export function AnchorBubble({ event }: { event: ChoiceEvent }) {
	const [manualSelect, setManualSelect] = useState(false)
	const rect =
		typeof event.elementIndex === 'number' ? getElementRectByIndex(event.elementIndex) : undefined
	const position = getBubblePosition(rect)
	const candidate = event.options[0]

	if (manualSelect) {
		return (
			<div
				data-webops-manual-select-overlay="true"
				onClick={(nativeEvent) => {
					nativeEvent.preventDefault()
					nativeEvent.stopPropagation()
					emitInteractionResponse(event.requestId, {
						type: 'manual_select',
						x: nativeEvent.clientX,
						y: nativeEvent.clientY,
					})
				}}
				style={{
					position: 'fixed',
					inset: 0,
					zIndex: 1,
					cursor: 'crosshair',
					pointerEvents: 'auto',
					background: 'rgba(15, 23, 42, .08)',
					fontFamily: 'system-ui, -apple-system, BlinkMacSystemFont, sans-serif',
				}}
			>
				<div
					style={{
						position: 'fixed',
						left: '50%',
						top: 24,
						transform: 'translateX(-50%)',
						padding: '10px 14px',
						borderRadius: 8,
						background: '#111827',
						color: '#fff',
						boxShadow: '0 16px 50px rgba(0,0,0,.24)',
						fontSize: 13,
						fontWeight: 700,
					}}
				>
					请点击目标控件的位置
				</div>
			</div>
		)
	}

	return (
		<div
			data-webops-anchor-bubble="true"
			style={{
				position: 'fixed',
				left: position.left,
				top: position.top,
				width: 340,
				maxWidth: 'calc(100vw - 48px)',
				padding: 14,
				borderRadius: 8,
				border: '1px solid #d1d5db',
				background: '#fff',
				color: '#111827',
				boxShadow: '0 16px 40px rgba(0,0,0,.18)',
				pointerEvents: 'auto',
				fontFamily: 'system-ui, -apple-system, BlinkMacSystemFont, sans-serif',
			}}
		>
			<div style={{ fontWeight: 700, marginBottom: 8 }}>{event.title}</div>
			<div style={{ fontSize: 13, lineHeight: 1.5, color: '#4b5563', marginBottom: 12 }}>
				{event.message}
			</div>
			{candidate?.label && (
				<div
					style={{
						fontSize: 12,
						lineHeight: 1.4,
						color: '#6b7280',
						padding: '6px 8px',
						borderRadius: 6,
						background: '#f3f4f6',
						marginBottom: 12,
					}}
				>
					候选目标：{candidate.label}
				</div>
			)}
			<div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
				<button
					type="button"
					data-webops-choice-confirm="true"
					onClick={() =>
						emitInteractionResponse(event.requestId, {
							type: 'choice',
							optionId: candidate?.id ?? 'confirmed',
						})
					}
					style={buttonStyle({ primary: true })}
				>
					确定
				</button>
				<button
					type="button"
					data-webops-choice-reject="true"
					onClick={() =>
						emitInteractionResponse(event.requestId, {
							type: 'rejected',
							reason: 'wrong_target',
						})
					}
					style={buttonStyle()}
				>
					错误
				</button>
				<button
					type="button"
					data-webops-choice-manual="true"
					onClick={() => setManualSelect(true)}
					style={buttonStyle()}
				>
					手动选择
				</button>
			</div>
		</div>
	)
}

function buttonStyle(options: { primary?: boolean } = {}): React.CSSProperties {
	return {
		padding: '8px 10px',
		borderRadius: 8,
		border: options.primary ? 0 : '1px solid #d1d5db',
		background: options.primary ? '#111827' : '#f9fafb',
		color: options.primary ? '#fff' : '#111827',
		cursor: 'pointer',
		font: 'inherit',
		fontWeight: options.primary ? 700 : 400,
	}
}

function getBubblePosition(rect: DOMRect | undefined) {
	const width = 340
	const margin = 12

	if (!rect) {
		return {
			left: Math.max(margin, window.innerWidth - width - 24),
			top: 96,
		}
	}

	const rightSideLeft = rect.right + margin
	const leftSideLeft = rect.left - width - margin
	const left =
		rightSideLeft + width + margin <= window.innerWidth
			? rightSideLeft
			: Math.max(margin, leftSideLeft)
	const top = Math.max(margin, Math.min(rect.top, window.innerHeight - 220))

	return { left, top }
}
