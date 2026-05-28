/**
 * content script for RemotePageController
 */
import { PageController } from '@page-agent/page-controller'

import type {
	InteractionEvent,
	InteractionResponse,
	InteractionResponseDetail,
} from '@/webops/interactions/interactionTypes'
import { WEBOPS_INTERACTION_RESPONSE_EVENT } from '@/webops/interactions/interactionTypes'
import { webOpsInteractionBus } from '@/webops/interactions/overlayRoot'

import {
	hidePageLock,
	isPageLockSuspended,
	resumePageLock,
	showPageLock,
	suspendPageLock,
	withPageLockBypassed,
} from './pageLock'
import { SIDE_PANEL_HANDOVER_ACTIVE_STORAGE_KEY } from './sidePanelHandover'

export function initPageController() {
	let pageController: PageController | null = null
	let intervalID: number | null = null

	const myTabIdPromise = chrome.runtime
		.sendMessage({ type: 'PAGE_CONTROL', action: 'get_my_tab_id' })
		.then((response) => {
			return (response as { tabId: number | null }).tabId
		})
		.catch((error) => {
			console.error('[RemotePageController.ContentScript]: Failed to get my tab id', error)
			return null
		})

	function getPC(): PageController {
		if (!pageController) {
			pageController = new PageController({
				enableMask: false,
				viewportExpansion: 400,
			})
		}
		return pageController
	}

	intervalID = window.setInterval(async () => {
		const agentHeartbeat = (await chrome.storage.local.get('agentHeartbeat')).agentHeartbeat
		const now = Date.now()
		const agentInTouch = typeof agentHeartbeat === 'number' && now - agentHeartbeat < 2_000

		const isAgentRunning = (await chrome.storage.local.get('isAgentRunning')).isAgentRunning
		const currentTabId = (await chrome.storage.local.get('currentTabId')).currentTabId
		const sidePanelHandoverActive = Boolean(
			(await chrome.storage.local.get(SIDE_PANEL_HANDOVER_ACTIVE_STORAGE_KEY))[
				SIDE_PANEL_HANDOVER_ACTIVE_STORAGE_KEY
			]
		)

		const shouldShowMask = isAgentRunning && agentInTouch && currentTabId === (await myTabIdPromise)

		if (shouldShowMask) {
			const pc = getPC()
			pc.initMask()
			if (isPageLockSuspended() || sidePanelHandoverActive) {
				hidePageLock()
				await pc.hideMask()
			} else {
				showPageLock()
				await pc.showMask()
			}
		} else {
			hidePageLock()
			// await getPC().hideMask()
			if (pageController) {
				pageController.hideMask()
				pageController.cleanUpHighlights()
			}
		}

		if (!isAgentRunning && agentInTouch) {
			if (pageController) {
				pageController.dispose()
				pageController = null
			}
		}
	}, 500)

	chrome.runtime.onMessage.addListener((message, sender, sendResponse): true | undefined => {
		if (message.type !== 'PAGE_CONTROL') {
			// sendResponse({
			// 	success: false,
			// 	error: `[RemotePageController.ContentScript]: Invalid message type: ${message.type}`,
			// })
			return
		}

		const { action, payload } = message
		const methodName = getMethodName(action)

		const pc = getPC() as any

		switch (action) {
			case 'webops_interaction':
				webOpsInteractionBus.publish(payload)
				sendResponse({ success: true })
				break

			case 'webops_interaction_request':
				waitForInteractionResponse(payload?.event, payload?.timeoutMs)
					.then((response) => sendResponse(response))
					.catch((error: unknown) =>
						sendResponse({
							type: 'cancelled',
							reason: error instanceof Error ? error.message : String(error),
						})
					)
				break

			case 'get_element_snapshot':
				sendResponse(getElementSnapshot(payload?.[0]))
				break

			case 'get_workflow_elements':
				sendResponse(getWorkflowElements())
				break

			case 'workflow_click_element':
				sendResponse(executeWorkflowElementAction(payload?.[0], 'click'))
				break

			case 'workflow_input_text':
				sendResponse(executeWorkflowElementAction(payload?.[0], 'input', payload?.[1]))
				break

			case 'workflow_select_option':
				sendResponse(executeWorkflowElementAction(payload?.[0], 'select', payload?.[1]))
				break

			case 'get_last_update_time':
			case 'get_browser_state':
			case 'update_tree':
			case 'clean_up_highlights':
			case 'click_element':
			case 'input_text':
			case 'select_option':
			case 'scroll':
			case 'scroll_horizontally':
			case 'press_key':
			case 'go_back':
			case 'reload_page':
			case 'wait_for_condition':
			case 'execute_javascript':
				executePageControllerMethod(pc, methodName, payload, shouldBypassPageLock(action))
					.then((result: any) => sendResponse(result))
					.catch((error: any) =>
						sendResponse({
							success: false,
							error: error instanceof Error ? error.message : String(error),
						})
					)
				break

			default:
				sendResponse({
					success: false,
					error: `Unknown PAGE_CONTROL action: ${action}`,
				})
		}

		return true
	})
}

function getWorkflowElements() {
	return getInteractiveElements().map((element, index) => {
		const snapshot = getElementSnapshotFromElement(element, index)
		return {
			id: index,
			...snapshot,
			label: getElementLabel(element),
			placeholder: element.getAttribute('placeholder') || undefined,
			visible: isElementVisible(element),
			enabled: !isElementDisabled(element),
			covered: false,
		}
	})
}

function executeWorkflowElementAction(
	target: any,
	action: 'click' | 'input' | 'select',
	value?: string
) {
	const element = resolveWorkflowElement(target)
	if (!element) {
		return { success: false, message: 'Workflow target no longer exists on the current page.' }
	}
	if (isElementDisabled(element)) {
		return { success: false, message: 'Workflow target is disabled.' }
	}

	try {
		if (action === 'click') {
			element.click()
			return { success: true, message: '✅ Workflow clicked target.' }
		}

		if (action === 'input') {
			if (!isTextInputElement(element)) {
				return { success: false, message: 'Workflow target is not an input element.' }
			}
			element.focus()
			element.value = value ?? ''
			element.dispatchEvent(new InputEvent('input', { bubbles: true, data: value ?? '' }))
			element.dispatchEvent(new Event('change', { bubbles: true }))
			return { success: true, message: '✅ Workflow filled target.' }
		}

		if (!(element instanceof HTMLSelectElement)) {
			return { success: false, message: 'Workflow target is not a select element.' }
		}
		element.value = value ?? ''
		element.dispatchEvent(new Event('input', { bubbles: true }))
		element.dispatchEvent(new Event('change', { bubbles: true }))
		return { success: true, message: '✅ Workflow selected option.' }
	} catch (error) {
		return {
			success: false,
			message: error instanceof Error ? error.message : String(error),
		}
	}
}

function resolveWorkflowElement(target: any): HTMLElement | null {
	if (!target) return null
	const byIndex = getInteractiveElements()[Number(target.id)]
	if (byIndex && elementMatchesWorkflowTarget(byIndex, target)) return byIndex
	if (target.css) {
		const element = document.querySelector<HTMLElement>(target.css)
		if (element) return element
	}
	if (target.xpath) {
		const element = document.evaluate(
			String(target.xpath).replace(/^xpath=/, ''),
			document,
			null,
			XPathResult.FIRST_ORDERED_NODE_TYPE,
			null
		).singleNodeValue
		if (element instanceof HTMLElement) return element
	}
	return (
		getInteractiveElements().find((element) => elementMatchesWorkflowTarget(element, target)) ??
		null
	)
}

function elementMatchesWorkflowTarget(element: HTMLElement, target: any): boolean {
	const snapshot = getElementSnapshotFromElement(element, Number(target.id) || 0)
	const label = getElementLabel(element)
	const placeholder = element.getAttribute('placeholder') || undefined
	return (
		equals(target.testId, snapshot.testId) ||
		(!!target.role &&
			!!target.name &&
			equals(target.role, snapshot.role) &&
			equals(target.name, snapshot.name)) ||
		equals(target.label, label) ||
		equals(target.placeholder, placeholder) ||
		equals(target.text, snapshot.text)
	)
}

function getInteractiveElements() {
	return Array.from(
		document.querySelectorAll<HTMLElement>(
			'button,a,input,textarea,select,[role="button"],[role="link"],[role="textbox"],[role="combobox"],[tabindex]:not([tabindex="-1"])'
		)
	).filter(isElementVisible)
}

function executePageControllerMethod(
	pc: any,
	methodName: string,
	payload: any[] | undefined,
	bypassPageLock: boolean
) {
	const execute = () => pc[methodName](...(payload || []))
	return bypassPageLock ? withPageLockBypassed(execute) : execute()
}

function shouldBypassPageLock(action: string) {
	return action === 'get_browser_state' || action === 'update_tree'
}

function waitForInteractionResponse(
	event: InteractionEvent | undefined,
	timeoutMs: number | undefined
): Promise<InteractionResponse> {
	if (!event) return Promise.resolve({ type: 'cancelled', reason: 'missing_interaction_event' })

	const shouldSuspendLock = event.type === 'handover'
	if (shouldSuspendLock) suspendPageLock()

	webOpsInteractionBus.publish(event)

	const requestId = 'requestId' in event ? event.requestId : undefined
	if (!requestId) {
		if (shouldSuspendLock) resumePageLock()
		return Promise.resolve({ type: 'completed' })
	}

	return new Promise((resolve) => {
		const finish = (response: InteractionResponse) => {
			if (shouldSuspendLock) resumePageLock()
			resolve(response)
		}
		const timer = window.setTimeout(() => {
			window.removeEventListener(WEBOPS_INTERACTION_RESPONSE_EVENT, handleResponse)
			finish({ type: 'cancelled', reason: 'timeout' })
		}, timeoutMs ?? 600_000)

		function handleResponse(nativeEvent: Event) {
			const detail = (nativeEvent as CustomEvent<InteractionResponseDetail>).detail
			if (detail?.requestId !== requestId) return

			window.clearTimeout(timer)
			window.removeEventListener(WEBOPS_INTERACTION_RESPONSE_EVENT, handleResponse)
			finish(detail.response)
		}

		window.addEventListener(WEBOPS_INTERACTION_RESPONSE_EVENT, handleResponse)
	})
}

function getElementSnapshot(index: number | undefined) {
	if (typeof index !== 'number') return undefined

	const element = getElementByIndex(index)
	if (!element) return undefined

	return getElementSnapshotFromElement(element, index)
}

function getElementSnapshotFromElement(element: HTMLElement, index: number) {
	const role = element.getAttribute('role') || implicitRole(element)
	const name =
		getElementLabel(element) ||
		element.getAttribute('aria-label') ||
		element.getAttribute('placeholder') ||
		element.getAttribute('title') ||
		element.getAttribute('name') ||
		element.textContent?.trim().slice(0, 120) ||
		undefined
	const text = element.textContent?.trim().slice(0, 120) || undefined
	const css = stableCssSelector(element)
	const testId =
		element.getAttribute('data-testid') ||
		element.getAttribute('data-test') ||
		element.getAttribute('data-cy') ||
		undefined
	const xpath = stableXPath(element)
	const candidates = [
		testId ? { strategy: 'testId' as const, value: testId, confidence: 1 } : undefined,
		role && name ? { strategy: 'role' as const, role, name, confidence: 0.95 } : undefined,
		getElementLabel(element)
			? { strategy: 'label' as const, value: getElementLabel(element), confidence: 0.9 }
			: undefined,
		name ? { strategy: 'placeholder' as const, value: name, confidence: 0.8 } : undefined,
		text ? { strategy: 'text' as const, value: text, confidence: 0.7 } : undefined,
		css ? { strategy: 'css' as const, value: css, confidence: 0.6 } : undefined,
		xpath ? { strategy: 'xpath' as const, value: xpath, confidence: 0.4 } : undefined,
	].filter(Boolean)

	return {
		elementIndex: index,
		role,
		name,
		text,
		css,
		xpath,
		testId,
		candidates,
	}
}

function getElementByIndex(index: number) {
	const selector = `[data-page-agent-index="${index}"], [data-highlight-index="${index}"]`
	const indexedElement = document.querySelector<HTMLElement>(selector)
	const fallbackElement = document.querySelectorAll<HTMLElement>(
		'button,a,input,textarea,select,[role="button"],[role="link"],[tabindex]:not([tabindex="-1"])'
	)[index]

	return indexedElement ?? fallbackElement
}

function implicitRole(element: HTMLElement) {
	const tagName = element.tagName.toLowerCase()
	if (tagName === 'button') return 'button'
	if (tagName === 'a') return 'link'
	if (tagName === 'select') return 'combobox'
	if (tagName === 'textarea') return 'textbox'
	if (tagName === 'input') return 'textbox'
	return undefined
}

function stableCssSelector(element: HTMLElement) {
	if (element.id) return `#${CSS.escape(element.id)}`
	const testIdAttr = ['data-testid', 'data-test', 'data-cy'].find((attr) =>
		element.hasAttribute(attr)
	)
	if (testIdAttr) return `[${testIdAttr}="${CSS.escape(element.getAttribute(testIdAttr) || '')}"]`
	return element.tagName.toLowerCase()
}

function stableXPath(element: HTMLElement): string | undefined {
	if (element.id) return `//*[@id="${element.id.replace(/"/g, '\\"')}"]`
	const parts: string[] = []
	let current: Element | null = element
	while (current && current.nodeType === Node.ELEMENT_NODE && current !== document.body) {
		const tag = current.tagName.toLowerCase()
		const siblings = Array.from(current.parentElement?.children ?? []).filter(
			(sibling) => sibling.tagName === current?.tagName
		)
		const index = siblings.indexOf(current) + 1
		parts.unshift(`${tag}[${index}]`)
		current = current.parentElement
	}
	return parts.length ? `xpath=//${parts.join('/')}` : undefined
}

function getElementLabel(element: HTMLElement): string | undefined {
	const ariaLabelledBy = element.getAttribute('aria-labelledby')
	if (ariaLabelledBy) {
		const label = ariaLabelledBy
			.split(/\s+/)
			.map((id) => document.getElementById(id)?.textContent?.trim())
			.filter(Boolean)
			.join(' ')
		if (label) return label.slice(0, 120)
	}
	if (element.id) {
		const label = document.querySelector<HTMLLabelElement>(`label[for="${CSS.escape(element.id)}"]`)
		if (label?.textContent?.trim()) return label.textContent.trim().slice(0, 120)
	}
	const parentLabel = element.closest('label')
	return parentLabel?.textContent?.trim().slice(0, 120) || undefined
}

function isElementVisible(element: HTMLElement): boolean {
	const style = window.getComputedStyle(element)
	const rect = element.getBoundingClientRect()
	return (
		style.visibility !== 'hidden' && style.display !== 'none' && rect.width > 0 && rect.height > 0
	)
}

function isElementDisabled(element: HTMLElement): boolean {
	return (
		element.hasAttribute('disabled') ||
		element.getAttribute('aria-disabled') === 'true' ||
		(element as HTMLInputElement).disabled
	)
}

function isTextInputElement(
	element: HTMLElement
): element is HTMLInputElement | HTMLTextAreaElement {
	return element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement
}

function equals(left?: string, right?: string): boolean {
	if (!left || !right) return false
	return left.trim().toLowerCase() === right.trim().toLowerCase()
}

function getMethodName(action: string): string {
	switch (action) {
		case 'get_last_update_time':
			return 'getLastUpdateTime' as const
		case 'get_browser_state':
			return 'getBrowserState' as const
		case 'update_tree':
			return 'updateTree' as const
		case 'clean_up_highlights':
			return 'cleanUpHighlights' as const

		// DOM actions

		case 'click_element':
			return 'clickElement' as const
		case 'input_text':
			return 'inputText' as const
		case 'select_option':
			return 'selectOption' as const
		case 'scroll':
			return 'scroll' as const
		case 'scroll_horizontally':
			return 'scrollHorizontally' as const
		case 'press_key':
			return 'pressKey' as const
		case 'go_back':
			return 'goBack' as const
		case 'reload_page':
			return 'reloadPage' as const
		case 'wait_for_condition':
			return 'waitForCondition' as const
		case 'execute_javascript':
			return 'executeJavascript' as const

		default:
			return action
	}
}
