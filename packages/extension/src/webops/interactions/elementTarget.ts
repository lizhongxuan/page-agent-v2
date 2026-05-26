export function getElementRectByIndex(index: number) {
	const target = getElementByIndex(index)
	if (!target) return undefined

	const rect = target.getBoundingClientRect()
	if (rect.width <= 0 || rect.height <= 0) return undefined

	return rect
}

export function getElementByIndex(index: number) {
	const selector = `[data-page-agent-index="${index}"], [data-highlight-index="${index}"]`
	const indexedElement = document.querySelector<HTMLElement>(selector)
	const fallbackElement = document.querySelectorAll<HTMLElement>(
		'button,a,input,textarea,select,[role="button"],[role="link"],[tabindex]:not([tabindex="-1"])'
	)[index]

	return indexedElement ?? fallbackElement
}
