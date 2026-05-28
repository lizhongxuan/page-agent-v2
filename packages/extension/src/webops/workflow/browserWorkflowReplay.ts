import type {
	WorkflowRecipe,
	WorkflowRecipeChunk,
	WorkflowRecipeStep,
	WorkflowStepTarget,
	WorkflowTargetCandidate,
} from './types'

export interface BrowserWorkflowReplayRequest {
	workflow: WorkflowRecipe
	bindings?: Record<string, string>
}

export type BrowserWorkflowReplayResult =
	| { ok: true }
	| {
			ok: false
			message: string
			failedChunkId?: string
			failedStepId?: string
	  }

export type BrowserWorkflowStepResult = BrowserWorkflowReplayResult

export async function runBrowserWorkflowReplay(
	workflow: WorkflowRecipe,
	bindings: Record<string, string> = {}
): Promise<BrowserWorkflowReplayResult> {
	try {
		for (const chunk of workflow.chunks) {
			await runChunk(chunk, bindings)
		}
		return { ok: true }
	} catch (error) {
		const failure = error as Error & { chunkId?: string; stepId?: string }
		return {
			ok: false,
			message: failure.message || String(error),
			failedChunkId: failure.chunkId,
			failedStepId: failure.stepId,
		}
	}
}

async function runChunk(
	chunk: WorkflowRecipeChunk,
	bindings: Record<string, string>
): Promise<void> {
	for (const step of chunk.steps) {
		try {
			const result = await runBrowserWorkflowStep(step, bindings)
			if (!result.ok) throw new Error(result.message)
		} catch (error) {
			const failure = error as Error & { chunkId?: string; stepId?: string }
			failure.chunkId = failure.chunkId || chunk.id
			failure.stepId = failure.stepId || step.id
			throw failure
		}
	}
}

export async function runBrowserWorkflowStep(
	step: WorkflowRecipeStep,
	bindings: Record<string, string> = {}
): Promise<BrowserWorkflowStepResult> {
	try {
		await dismissCommonDialogs()
		if (hasBlockingDialog()) {
			throw new Error('Workflow replay is blocked by a dialog that could not be dismissed.')
		}
		await runStep(step, bindings)
		await dismissCommonDialogs()
		if (hasBlockingDialog()) {
			throw new Error('Workflow replay is blocked by a dialog that could not be dismissed.')
		}
		return { ok: true }
	} catch (error) {
		return {
			ok: false,
			message: error instanceof Error ? error.message : String(error),
			failedStepId: step.id,
		}
	}
}

async function runStep(step: WorkflowRecipeStep, bindings: Record<string, string>): Promise<void> {
	if (step.type === 'wait') {
		await delay(waitMillis(render(step.value, bindings)))
		return
	}
	const element = resolveElement(step.target, bindings)
	if (!element) throw new Error(`Workflow replay target not found: ${step.id}`)

	scrollIntoView(element)

	if (step.type === 'click') {
		clickElement(element)
		await delay(300)
		return
	}

	if (step.type === 'fill') {
		setElementValue(element, render(step.value, bindings))
		return
	}

	if (step.type === 'press') {
		pressKey(element, render(step.key || step.value, bindings))
		await delay(300)
		return
	}

	if (step.type === 'select') {
		selectElementValue(element, render(step.value, bindings))
		return
	}

	throw new Error(`Unsupported workflow step type: ${step.type}`)
}

function resolveElement(
	target: WorkflowStepTarget | undefined,
	bindings: Record<string, string>
): HTMLElement | undefined {
	const candidates = [target?.primary, ...(target?.fallbacks ?? [])].filter(
		(candidate): candidate is WorkflowTargetCandidate => Boolean(candidate)
	)
	for (const candidate of candidates) {
		const element = elementForCandidate(candidate, bindings)
		if (element && isVisibleElement(element)) return element
	}
	return undefined
}

function elementForCandidate(
	candidate: WorkflowTargetCandidate,
	bindings: Record<string, string>
): HTMLElement | undefined {
	const value = render(candidate.value, bindings)
	const name = render(candidate.name, bindings)

	if (candidate.strategy === 'css' && value) {
		if (!isSafeCssSelector(value)) return undefined
		return document.querySelector<HTMLElement>(value) ?? undefined
	}
	if (candidate.strategy === 'xpath' && value) {
		return xpathElement(value)
	}
	if (candidate.strategy === 'test_id' && (value || name)) {
		return firstVisible(
			[
				`[data-testid="${cssEscape(value || name)}"]`,
				`[data-test="${cssEscape(value || name)}"]`,
				`[data-cy="${cssEscape(value || name)}"]`,
			].flatMap((selector) => Array.from(document.querySelectorAll<HTMLElement>(selector)))
		)
	}
	if (candidate.strategy === 'placeholder' && (value || name)) {
		return firstVisible(
			Array.from(
				document.querySelectorAll<HTMLElement>(
					`input[placeholder="${cssEscape(value || name)}"], textarea[placeholder="${cssEscape(value || name)}"]`
				)
			)
		)
	}
	if (candidate.strategy === 'label' && (value || name)) {
		return labelledControl(value || name)
	}
	if (candidate.strategy === 'role') {
		return byRole(candidate.role, name || value)
	}
	if (candidate.strategy === 'text' && (value || name)) {
		return byText(value || name)
	}
	return undefined
}

function byRole(role: string | undefined, name: string): HTMLElement | undefined {
	const elements = controls().filter((element) => implicitRole(element) === role)
	if (!name) return firstVisible(elements)
	return firstVisible(elements.filter((element) => matchesText(accessibleName(element), name)))
}

function byText(text: string): HTMLElement | undefined {
	const elements = Array.from(
		document.querySelectorAll<HTMLElement>('button,a,[role],label,summary')
	)
	return firstVisible(elements.filter((element) => matchesText(elementText(element), text)))
}

function labelledControl(labelText: string): HTMLElement | undefined {
	const labels = Array.from(document.querySelectorAll<HTMLLabelElement>('label'))
	for (const label of labels) {
		if (!matchesText(elementText(label), labelText)) continue
		const forId = label.getAttribute('for')
		if (forId) {
			const control = document.getElementById(forId)
			if (control instanceof HTMLElement) return control
		}
		const nested = label.querySelector<HTMLElement>('input,textarea,select,button')
		if (nested) return nested
	}
	return byText(labelText)
}

function controls(): HTMLElement[] {
	return Array.from(
		document.querySelectorAll<HTMLElement>(
			'button,a,input,textarea,select,[role],[tabindex]:not([tabindex="-1"])'
		)
	)
}

function xpathElement(xpath: string): HTMLElement | undefined {
	const result = document.evaluate(xpath, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null)
	return result.singleNodeValue instanceof HTMLElement ? result.singleNodeValue : undefined
}

function firstVisible(elements: HTMLElement[]): HTMLElement | undefined {
	return elements.find(isVisibleElement)
}

function isVisibleElement(element: HTMLElement): boolean {
	const style = window.getComputedStyle(element)
	if (style.display === 'none' || style.visibility === 'hidden') return false
	const rect = element.getBoundingClientRect()
	return rect.width > 0 || rect.height > 0 || element.offsetParent !== null || isJsdom()
}

function isJsdom(): boolean {
	return navigator.userAgent.toLowerCase().includes('jsdom')
}

function implicitRole(element: HTMLElement): string | undefined {
	const explicitRole = element.getAttribute('role')
	if (explicitRole) return explicitRole
	const tagName = element.tagName.toLowerCase()
	if (tagName === 'a') return 'link'
	if (tagName === 'button') return 'button'
	if (tagName === 'select') return 'combobox'
	if (tagName === 'textarea') return 'textbox'
	if (tagName === 'input') {
		const type = (element as HTMLInputElement).type
		if (type === 'search') return 'searchbox'
		if (type === 'button' || type === 'submit') return 'button'
		return 'textbox'
	}
	return undefined
}

function accessibleName(element: HTMLElement): string {
	const labelledBy = element.getAttribute('aria-labelledby')
	const labelledText = labelledBy
		?.split(/\s+/)
		.map((id) => document.getElementById(id)?.textContent || '')
		.join(' ')
	return normalizeSpace(
		element.getAttribute('aria-label') ||
			labelledText ||
			associatedLabelText(element) ||
			element.getAttribute('placeholder') ||
			element.getAttribute('title') ||
			element.getAttribute('name') ||
			(element instanceof HTMLInputElement ? element.value : '') ||
			element.textContent ||
			''
	)
}

function associatedLabelText(element: HTMLElement): string {
	const id = element.id
	if (id) {
		const label = document.querySelector<HTMLLabelElement>(`label[for="${cssEscape(id)}"]`)
		if (label?.textContent) return label.textContent
	}
	return element.closest('label')?.textContent || ''
}

function elementText(element: HTMLElement): string {
	return normalizeSpace(element.textContent || '')
}

function matchesText(value: string, expected: string): boolean {
	const normalizedValue = normalizeSpace(value).toLowerCase()
	const normalizedExpected = normalizeSpace(expected).toLowerCase()
	return normalizedValue === normalizedExpected || normalizedValue.includes(normalizedExpected)
}

function normalizeSpace(value: string): string {
	return value.replace(/\s+/g, ' ').trim()
}

function render(value: string | undefined, bindings: Record<string, string>): string {
	if (!value) return ''
	let result = value
	for (const [key, binding] of Object.entries(bindings)) {
		result = result.replaceAll(`{{${key}}}`, binding)
		result = result.replaceAll(`{${key}}`, binding)
	}
	return result
}

function setElementValue(element: HTMLElement, value: string): void {
	if (element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement) {
		element.focus()
		element.value = value
		dispatchInputEvents(element)
		return
	}
	element.textContent = value
	dispatchInputEvents(element)
}

function selectElementValue(element: HTMLElement, value: string): void {
	if (!(element instanceof HTMLSelectElement)) {
		throw new Error('Workflow replay select target is not a select element.')
	}
	element.focus()
	element.value = value
	element.dispatchEvent(new Event('input', { bubbles: true }))
	element.dispatchEvent(new Event('change', { bubbles: true }))
}

function pressKey(element: HTMLElement, key: string): void {
	element.focus()
	const eventInit = { key, bubbles: true, cancelable: true }
	element.dispatchEvent(new KeyboardEvent('keydown', eventInit))
	element.dispatchEvent(new KeyboardEvent('keyup', eventInit))
	if (key === 'Enter') {
		const form = element instanceof HTMLInputElement ? element.form : undefined
		form?.requestSubmit()
	}
}

function clickElement(element: HTMLElement): void {
	element.focus()
	element.click()
}

function dispatchInputEvents(element: HTMLElement): void {
	element.dispatchEvent(new Event('input', { bubbles: true }))
	element.dispatchEvent(new Event('change', { bubbles: true }))
}

function scrollIntoView(element: HTMLElement): void {
	if (typeof element.scrollIntoView !== 'function') return
	element.scrollIntoView({ block: 'center', inline: 'center' })
}

async function dismissCommonDialogs(): Promise<void> {
	const labels = [
		'Save changes',
		'Got it',
		'Close',
		'Dismiss',
		'OK',
		'Ok',
		'Cancel',
		'No thanks',
		'Maybe later',
		'×',
	]
	for (let attempt = 0; attempt < 3; attempt++) {
		let dismissed = false
		for (const label of labels) {
			const button = byActionableRole('button', label)
			if (!button) continue
			clickElement(button)
			await delay(150)
			dismissed = true
			break
		}
		if (!dismissed) return
	}
}

function byActionableRole(role: string | undefined, name: string): HTMLElement | undefined {
	const elements = controls().filter(
		(element) =>
			implicitRole(element) === role && isVisibleElement(element) && !isDisabledControl(element)
	)
	if (!name) return firstVisible(elements)
	return firstVisible(elements.filter((element) => matchesText(accessibleName(element), name)))
}

function hasBlockingDialog(): boolean {
	return Array.from(
		document.querySelectorAll<HTMLElement>(
			'[role="dialog"], [role="alertdialog"], [aria-modal="true"]'
		)
	).some((element) => isVisibleElement(element) && !isKnownNonBlockingDialog(element))
}

function isKnownNonBlockingDialog(_element: HTMLElement): boolean {
	return false
}

function isDisabledControl(element: HTMLElement): boolean {
	if (
		element instanceof HTMLButtonElement ||
		element instanceof HTMLInputElement ||
		element instanceof HTMLSelectElement ||
		element instanceof HTMLTextAreaElement
	) {
		return element.disabled
	}
	return element.getAttribute('aria-disabled') === 'true'
}

function isSafeCssSelector(selector: string): boolean {
	const normalized = selector.trim()
	if (!normalized) return false
	if (/^[a-z][\w-]*$/i.test(normalized)) return false
	if (normalized === '*' || normalized.includes(':has(')) return false
	return true
}

function waitMillis(value: string): number {
	const parsed = Number.parseInt(value || '500', 10)
	return Number.isFinite(parsed) && parsed > 0 ? parsed : 500
}

function delay(ms: number): Promise<void> {
	return new Promise((resolve) => window.setTimeout(resolve, ms))
}

function cssEscape(value: string): string {
	const escape = (globalThis.CSS as { escape?: (input: string) => string } | undefined)?.escape
	return escape ? escape(value) : value.replaceAll('"', '\\"')
}
