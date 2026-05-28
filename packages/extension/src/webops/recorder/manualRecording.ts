import { SessionRecorder } from './SessionRecorder'
import type {
	RecordedAction,
	RecordedActionTarget,
	RecordedSession,
	RecordedTargetCandidate,
} from './actionEvents'
import { redactSensitiveValue } from './redaction'

export type ManualRecordingControlMessage =
	| { type: 'WEBOPS_MANUAL_RECORDING'; action: 'start'; task: string }
	| { type: 'WEBOPS_MANUAL_RECORDING'; action: 'stop' }
	| { type: 'WEBOPS_MANUAL_RECORDING'; action: 'cancel' }

export type ManualRecordingControlResponse =
	| { ok: true; status: 'started' | 'cancelled' }
	| { ok: true; status: 'stopped'; session: RecordedSession }
	| { ok: false; error: string }

export class ManualRecordingController {
	private recorder?: SessionRecorder
	private removeListeners: (() => void)[] = []
	private pendingInputs = new Map<string, RecordedAction>()

	get active(): boolean {
		return this.recorder !== undefined
	}

	start(task: string): void {
		this.stopListeners()
		this.pendingInputs.clear()
		this.recorder = new SessionRecorder()
		this.recorder.start({
			id: crypto.randomUUID(),
			task: task || `Manual recording: ${location.hostname}`,
			startUrl: location.href,
		})
		this.installListeners()
	}

	stop(): RecordedSession {
		if (!this.recorder) throw new Error('Manual recording has not started.')
		this.flushInputs()
		const session = this.recorder.finish()
		this.recorder = undefined
		this.stopListeners()
		return session
	}

	cancel(): void {
		this.recorder = undefined
		this.pendingInputs.clear()
		this.stopListeners()
	}

	private installListeners(): void {
		const onClick = (event: Event) => {
			if (!(event.target instanceof HTMLElement)) return
			if (isExtensionElement(event.target)) return
			this.flushInputs()
			this.record({
				type: 'click',
				target: targetFromElement(event.target),
				value: undefined,
				result: 'success',
			})
		}
		const onInput = (event: Event) => {
			if (!isValueElement(event.target)) return
			if (isExtensionElement(event.target)) return
			const target = targetFromElement(event.target)
			const action = this.createAction({
				type: event.target instanceof HTMLSelectElement ? 'select' : 'input',
				target,
				value: event.target.value,
				result: 'success',
			})
			this.pendingInputs.set(targetKey(target), action)
		}
		const onChange = (event: Event) => {
			if (!isValueElement(event.target)) return
			if (isExtensionElement(event.target)) return
			const target = targetFromElement(event.target)
			this.pendingInputs.set(
				targetKey(target),
				this.createAction({
					type: event.target instanceof HTMLSelectElement ? 'select' : 'input',
					target,
					value: event.target.value,
					result: 'success',
				})
			)
			this.flushInputs()
		}
		const onScroll = () => {
			this.flushInputs()
			this.record({
				type: 'scroll' as RecordedAction['type'],
				result: 'success',
				note: `Scrolled to ${Math.round(window.scrollY)}px.`,
			})
		}
		const throttledScroll = throttle(onScroll, 500)

		document.addEventListener('click', onClick, true)
		document.addEventListener('input', onInput, true)
		document.addEventListener('change', onChange, true)
		window.addEventListener('scroll', throttledScroll, true)
		this.removeListeners = [
			() => document.removeEventListener('click', onClick, true),
			() => document.removeEventListener('input', onInput, true),
			() => document.removeEventListener('change', onChange, true),
			() => window.removeEventListener('scroll', throttledScroll, true),
		]
	}

	private stopListeners(): void {
		for (const remove of this.removeListeners) remove()
		this.removeListeners = []
	}

	private flushInputs(): void {
		for (const action of this.pendingInputs.values()) {
			this.recorder?.record(action)
		}
		this.pendingInputs.clear()
	}

	private record(input: Omit<RecordedAction, 'id' | 'timestamp' | 'pageUrl' | 'pageTitle'>): void {
		this.recorder?.record(this.createAction(input))
	}

	private createAction(
		input: Omit<RecordedAction, 'id' | 'timestamp' | 'pageUrl' | 'pageTitle'>
	): RecordedAction {
		return {
			id: crypto.randomUUID(),
			timestamp: Date.now(),
			pageUrl: location.href,
			pageTitle: document.title,
			...input,
		}
	}
}

export interface ManualRecordingCaptureOptions {
	onActions?: (actions: RecordedAction[]) => void
	now?: () => number
	idFactory?: () => string
}

export class ManualRecordingCapture {
	private readonly actions: RecordedAction[] = []
	private readonly pendingInputs = new Map<string, RecordedAction>()
	private readonly now: () => number
	private readonly idFactory: () => string

	constructor(private readonly options: ManualRecordingCaptureOptions = {}) {
		this.now = options.now ?? (() => Date.now())
		this.idFactory = options.idFactory ?? (() => crypto.randomUUID())
	}

	start(): void {
		document.addEventListener('click', this.handleClick, true)
		document.addEventListener('input', this.handleInput, true)
		document.addEventListener('change', this.handleInput, true)
	}

	stop(): RecordedAction[] {
		this.flushInputs()
		document.removeEventListener('click', this.handleClick, true)
		document.removeEventListener('input', this.handleInput, true)
		document.removeEventListener('change', this.handleInput, true)
		return [...this.actions]
	}

	private readonly handleClick = (event: Event): void => {
		const action = normalizeManualDomEvent(event, {
			now: this.now,
			idFactory: this.idFactory,
		})
		if (!action || action.type !== 'click') return
		this.flushInputs()
		this.emit([action])
	}

	private readonly handleInput = (event: Event): void => {
		const action = normalizeManualDomEvent(event, {
			now: this.now,
			idFactory: this.idFactory,
		})
		if (!action || (action.type !== 'input' && action.type !== 'select')) return
		this.pendingInputs.set(targetKey(action.target ?? {}), action)
	}

	private flushInputs(): void {
		const actions = [...this.pendingInputs.values()]
		this.pendingInputs.clear()
		this.emit(actions)
	}

	private emit(actions: RecordedAction[]): void {
		this.actions.push(...actions)
		this.options.onActions?.(actions)
	}
}

export function createRecordedActionTarget(element: Element): RecordedActionTarget {
	if (!(element instanceof HTMLElement)) return {}
	return targetFromElement(element)
}

export function normalizeManualDomEvent(
	event: Event,
	options: { now?: () => number; idFactory?: () => string } = {}
): RecordedAction | undefined {
	if (!(event.target instanceof HTMLElement)) return undefined
	if (isExtensionElement(event.target)) return undefined
	const type = event.type === 'click' ? 'click' : valueActionType(event.target)
	if (!type) return undefined
	const target = targetFromElement(event.target)
	const rawValue = isValueElement(event.target) ? event.target.value : undefined
	const value =
		rawValue === undefined
			? undefined
			: redactSensitiveValue(
					[target.name, target.testId, target.role, target.text, type].filter(Boolean).join(' '),
					rawValue
				)

	return {
		id: options.idFactory?.() ?? crypto.randomUUID(),
		type,
		timestamp: options.now?.() ?? Date.now(),
		pageUrl: location.href,
		pageTitle: document.title,
		target,
		value,
		result: 'success',
	}
}

export function targetFromElement(element: HTMLElement): RecordedActionTarget {
	const role = element.getAttribute('role') || implicitRole(element)
	const label = getElementLabel(element)
	const name =
		label ||
		element.getAttribute('aria-label') ||
		element.getAttribute('placeholder') ||
		element.getAttribute('title') ||
		element.getAttribute('name') ||
		trimText(element.textContent)
	const text = trimText(element.textContent)
	const testId =
		element.getAttribute('data-testid') ||
		element.getAttribute('data-test') ||
		element.getAttribute('data-cy') ||
		undefined
	const css = stableCssSelector(element)
	const xpath = stableXPath(element)
	const candidates: RecordedTargetCandidate[] = (
		[
			testId ? { strategy: 'testId' as const, value: testId, confidence: 1 } : undefined,
			role && name ? { strategy: 'role' as const, role, name, confidence: 0.95 } : undefined,
			label ? { strategy: 'label' as const, value: label, confidence: 0.9 } : undefined,
			element.getAttribute('placeholder')
				? {
						strategy: 'placeholder' as const,
						value: element.getAttribute('placeholder') || undefined,
						confidence: 0.84,
					}
				: undefined,
			text ? { strategy: 'text' as const, value: text, confidence: 0.7 } : undefined,
			css ? { strategy: 'css' as const, value: css, confidence: 0.6 } : undefined,
			xpath ? { strategy: 'xpath' as const, value: xpath, confidence: 0.4 } : undefined,
		] as (RecordedTargetCandidate | undefined)[]
	).filter(isRecordedTargetCandidate)

	return {
		role,
		name,
		text,
		css,
		xpath,
		testId,
		candidates,
	}
}

function isValueElement(
	target: EventTarget | null
): target is HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement {
	return (
		target instanceof HTMLInputElement ||
		target instanceof HTMLTextAreaElement ||
		target instanceof HTMLSelectElement
	)
}

function valueActionType(element: HTMLElement): 'input' | 'select' | undefined {
	if (element instanceof HTMLSelectElement) return 'select'
	if (element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement) return 'input'
	return undefined
}

function isExtensionElement(element: HTMLElement): boolean {
	return Boolean(element.closest('[data-webops-overlay-root], [data-page-agent-extension-ui]'))
}

function implicitRole(element: HTMLElement): string | undefined {
	const tagName = element.tagName.toLowerCase()
	if (tagName === 'button') return 'button'
	if (tagName === 'a') return 'link'
	if (tagName === 'select') return 'combobox'
	if (tagName === 'textarea') return 'textbox'
	if (tagName === 'input') return 'textbox'
	return undefined
}

function getElementLabel(element: HTMLElement): string | undefined {
	const labelledBy = element.getAttribute('aria-labelledby')
	if (labelledBy) {
		const label = labelledBy
			.split(/\s+/)
			.map((id) => document.getElementById(id)?.textContent?.trim())
			.filter(Boolean)
			.join(' ')
		if (label) return label.slice(0, 120)
	}
	if (element.id) {
		const label = document.querySelector<HTMLLabelElement>(`label[for="${escapeCss(element.id)}"]`)
		if (label?.textContent?.trim()) return label.textContent.trim().slice(0, 120)
	}
	return element.closest('label')?.textContent?.trim().slice(0, 120) || undefined
}

function stableCssSelector(element: HTMLElement): string | undefined {
	if (element.id) return `#${escapeCss(element.id)}`
	const testIdAttr = ['data-testid', 'data-test', 'data-cy'].find((attr) =>
		element.hasAttribute(attr)
	)
	if (testIdAttr) return `[${testIdAttr}="${escapeCss(element.getAttribute(testIdAttr) || '')}"]`
	const name = element.getAttribute('name')
	if (name) return `${element.tagName.toLowerCase()}[name="${escapeCss(name)}"]`
	return element.tagName.toLowerCase()
}

function stableXPath(element: HTMLElement): string | undefined {
	const parts: string[] = []
	let current: Element | null = element
	while (current && current.nodeType === Node.ELEMENT_NODE) {
		const tag = current.tagName.toLowerCase()
		const siblings = Array.from(current.parentElement?.children ?? []).filter(
			(sibling) => sibling.tagName === current?.tagName
		)
		const index = siblings.length > 0 ? siblings.indexOf(current) + 1 : 1
		parts.unshift(`${tag}[${index}]`)
		current = current.parentElement
	}
	return parts.length ? `/${parts.join('/')}` : undefined
}

function targetKey(target: RecordedActionTarget): string {
	return target.testId || target.css || target.xpath || target.name || target.text || 'unknown'
}

function trimText(value: string | null | undefined): string | undefined {
	const text = value?.replace(/\s+/g, ' ').trim().slice(0, 120)
	return text || undefined
}

function throttle(fn: () => void, waitMs: number): () => void {
	let last = 0
	return () => {
		const now = Date.now()
		if (now - last < waitMs) return
		last = now
		fn()
	}
}

function escapeCss(value: string): string {
	return globalThis.CSS?.escape ? globalThis.CSS.escape(value) : value.replace(/["\\]/g, '\\$&')
}

function isRecordedTargetCandidate(
	candidate: RecordedTargetCandidate | undefined
): candidate is RecordedTargetCandidate {
	return candidate !== undefined
}
