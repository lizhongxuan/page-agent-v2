import type { RecordedAction, RecordedActionTarget, RecordedActionType } from './actionEvents'
import { redactSensitiveValue } from './redaction'

export const MANUAL_RECORDING_STATE_STORAGE_KEY = 'manualWorkflowRecording'
export const MANUAL_RECORDING_ACTIONS_STORAGE_KEY = 'manualWorkflowRecordedActions'

export type ManualRecordedEventType = 'click' | 'change' | 'input' | 'keydown'

export interface ManualRecordingState {
	active: boolean
	id: string
	task: string
	startUrl: string
	startedAt: number
}

export interface ManualRecordedTargetSnapshot {
	role?: string
	name?: string
	text?: string
	css?: string
	testId?: string
	inputType?: string
}

export interface ManualRecordedEventSnapshot {
	id: string
	type: ManualRecordedEventType
	timestamp: number
	pageUrl: string
	pageTitle: string
	target: ManualRecordedTargetSnapshot
	value?: string
	key?: string
}

export function manualEventToRecordedAction(snapshot: ManualRecordedEventSnapshot): RecordedAction {
	const actionType = toRecordedActionType(snapshot)
	const value = snapshot.type === 'keydown' ? snapshot.key : snapshot.value
	const fieldName = sensitiveFieldName(snapshot.target, snapshot.type)
	const safeValue = value === undefined ? undefined : redactSensitiveValue(fieldName, value)
	const redacted = value !== undefined && safeValue !== value

	return {
		id: snapshot.id,
		type: actionType,
		timestamp: snapshot.timestamp,
		pageUrl: snapshot.pageUrl,
		pageTitle: snapshot.pageTitle,
		target: toRecordedTarget(snapshot.target),
		value: safeValue,
		result: 'success',
		note:
			snapshot.type === 'keydown' ? 'keydown' : redacted ? 'sensitive-value-redacted' : undefined,
	}
}

function toRecordedActionType(snapshot: ManualRecordedEventSnapshot): RecordedActionType {
	if (snapshot.type === 'click') return 'click'
	if (
		snapshot.type === 'change' &&
		(snapshot.target.role === 'combobox' || snapshot.target.inputType === 'select-one')
	) {
		return 'select'
	}
	return 'input'
}

export function mergeManualRecordedAction(
	actions: RecordedAction[],
	action: RecordedAction
): RecordedAction[] {
	const previous = actions.at(-1)
	if (previous && shouldCoalesceInput(previous, action)) {
		return [...actions.slice(0, -1), action]
	}
	return [...actions, action]
}

function shouldCoalesceInput(previous: RecordedAction, next: RecordedAction): boolean {
	return (
		previous.type === 'input' &&
		next.type === 'input' &&
		previous.note !== 'keydown' &&
		next.note !== 'keydown' &&
		previous.pageUrl === next.pageUrl &&
		sameTarget(previous.target, next.target)
	)
}

function sameTarget(
	left: RecordedActionTarget | undefined,
	right: RecordedActionTarget | undefined
): boolean {
	if (!left || !right) return false
	return stableTargetKey(left) !== '' && stableTargetKey(left) === stableTargetKey(right)
}

function stableTargetKey(target: RecordedActionTarget): string {
	return (
		[
			target.testId ? `test:${target.testId}` : '',
			target.css ? `css:${target.css}` : '',
			target.role && target.name ? `role:${target.role}:${target.name}` : '',
			target.name ? `name:${target.name}` : '',
		].find(Boolean) ?? ''
	)
}

function toRecordedTarget(target: ManualRecordedTargetSnapshot): RecordedActionTarget {
	return {
		role: target.role,
		name: target.name,
		css: target.css,
		text: target.text,
		testId: target.testId,
	}
}

function sensitiveFieldName(
	target: ManualRecordedTargetSnapshot,
	eventType: ManualRecordedEventType
): string {
	return [target.name, target.testId, target.role, target.text, target.inputType, eventType]
		.filter(Boolean)
		.join(' ')
}
