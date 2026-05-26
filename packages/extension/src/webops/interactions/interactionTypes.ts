export type InteractionAction = 'click' | 'input' | 'select' | 'read'

export type InteractionLevel = 'info' | 'success' | 'warning' | 'error'

export interface InteractionChoiceOption {
	id: string
	label: string
}

export type InteractionEvent =
	| {
			type: 'spotlight'
			elementIndex: number
			action: InteractionAction
			message: string
			timeoutMs?: number
	  }
	| {
			type: 'choice'
			requestId?: string
			elementIndex?: number
			title: string
			message: string
			options: InteractionChoiceOption[]
	  }
	| {
			type: 'handover'
			requestId?: string
			title: string
			message: string
			resumeButtonLabel: string
	  }
	| {
			type: 'input'
			requestId?: string
			title: string
			message: string
			placeholder?: string
			submitButtonLabel?: string
	  }
	| {
			type: 'toast'
			message: string
			level: InteractionLevel
			timeoutMs?: number
	  }

export type InteractionResponse =
	| { type: 'completed' }
	| { type: 'choice'; optionId: string }
	| { type: 'rejected'; reason: 'wrong_target' }
	| { type: 'manual_select'; x: number; y: number }
	| { type: 'handover_done' }
	| { type: 'input'; value: string }
	| { type: 'cancelled'; reason: string }

type SpotlightEvent = Extract<InteractionEvent, { type: 'spotlight' }>
type ToastEvent = Extract<InteractionEvent, { type: 'toast' }>
type ChoiceEvent = Extract<InteractionEvent, { type: 'choice' }>
type HandoverEvent = Extract<InteractionEvent, { type: 'handover' }>
type InputEvent = Extract<InteractionEvent, { type: 'input' }>

export function normalizeInteractionEvent(
	event: SpotlightEvent
): SpotlightEvent & { timeoutMs: number }
export function normalizeInteractionEvent(event: ToastEvent): ToastEvent & { timeoutMs: number }
export function normalizeInteractionEvent(event: ChoiceEvent): ChoiceEvent
export function normalizeInteractionEvent(event: HandoverEvent): HandoverEvent
export function normalizeInteractionEvent(
	event: InputEvent
): InputEvent & { submitButtonLabel: string }
export function normalizeInteractionEvent(event: InteractionEvent): InteractionEvent
export function normalizeInteractionEvent(event: InteractionEvent): InteractionEvent {
	if (event.type === 'spotlight') {
		return { ...event, timeoutMs: event.timeoutMs ?? 1200 }
	}

	if (event.type === 'toast') {
		return { ...event, timeoutMs: event.timeoutMs ?? 1800 }
	}

	if (event.type === 'input') {
		return { ...event, submitButtonLabel: event.submitButtonLabel ?? '提交并继续' }
	}

	return event
}

export interface InteractionResponseDetail {
	requestId: string
	response: InteractionResponse
}

export const WEBOPS_INTERACTION_RESPONSE_EVENT = 'webops-interaction-response'

export function emitInteractionResponse(
	requestId: string | undefined,
	response: InteractionResponse
) {
	if (!requestId) return

	window.dispatchEvent(
		new CustomEvent<InteractionResponseDetail>(WEBOPS_INTERACTION_RESPONSE_EVENT, {
			detail: { requestId, response },
		})
	)
}
