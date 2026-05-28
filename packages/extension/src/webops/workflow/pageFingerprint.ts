import type { ControlSignature, PageFingerprint } from './types'

export interface BrowserStateInput {
	url?: string
	title?: string
	visibleText?: string
	controls?: ControlSignature[]
}

const importantControlRoles = new Set([
	'button',
	'textbox',
	'combobox',
	'link',
	'checkbox',
	'radio',
])

export function buildPageFingerprint(state: BrowserStateInput): PageFingerprint {
	const controls = normalizeControls(state.controls ?? [])
	const controlText = controls
		.flatMap((control) => [control.name, control.label, control.text])
		.filter(isPresent)

	return {
		urlPatterns: state.url ? [toOriginPattern(state.url)] : undefined,
		titleAny: state.title ? [state.title] : undefined,
		requiredText: pickRequiredText(state.visibleText ?? '', controlText),
		controlSignatures: controls,
		minMatchScore: 0.7,
	}
}

function toOriginPattern(url: string): string {
	try {
		const parsed = new URL(url)
		return `${parsed.origin}/*`
	} catch {
		return url
	}
}

function normalizeControls(controls: ControlSignature[]): ControlSignature[] {
	return controls
		.filter((control) => {
			if (!control.role) return true
			return importantControlRoles.has(control.role)
		})
		.map((control) => ({
			role: clean(control.role),
			name: clean(control.name),
			label: clean(control.label),
			placeholder: clean(control.placeholder),
			text: clean(control.text),
			testId: clean(control.testId),
		}))
		.map(removeEmptyKeys)
}

function pickRequiredText(visibleText: string, controlText: string[]): string[] {
	const normalizedText = visibleText.replace(/\s+/g, ' ').trim()
	const headings = normalizedText.match(/\b[A-Z][a-zA-Z]{2,}\b/g)?.slice(0, 1) ?? []

	return [...new Set([...headings, ...controlText].filter((text) => normalizedText.includes(text)))]
}

function clean(value: string | undefined): string | undefined {
	const cleaned = value?.replace(/\s+/g, ' ').trim()
	return cleaned || undefined
}

function removeEmptyKeys(control: ControlSignature): ControlSignature {
	return Object.fromEntries(
		Object.entries(control).filter(([, value]) => value !== undefined)
	) as ControlSignature
}

function isPresent(value: string | undefined): value is string {
	return typeof value === 'string' && value.length > 0
}
