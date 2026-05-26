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

import { hidePageLock, showPageLock, withPageLockBypassed } from './pageLock'

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

		const shouldShowMask = isAgentRunning && agentInTouch && currentTabId === (await myTabIdPromise)

		if (shouldShowMask) {
			showPageLock()
			const pc = getPC()
			pc.initMask()
			await pc.showMask()
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

			case 'get_last_update_time':
			case 'get_browser_state':
			case 'update_tree':
			case 'clean_up_highlights':
			case 'click_element':
			case 'input_text':
			case 'select_option':
			case 'scroll':
			case 'scroll_horizontally':
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

	webOpsInteractionBus.publish(event)

	const requestId = 'requestId' in event ? event.requestId : undefined
	if (!requestId) return Promise.resolve({ type: 'completed' })

	return new Promise((resolve) => {
		const timer = window.setTimeout(() => {
			window.removeEventListener(WEBOPS_INTERACTION_RESPONSE_EVENT, handleResponse)
			resolve({ type: 'cancelled', reason: 'timeout' })
		}, timeoutMs ?? 600_000)

		function handleResponse(nativeEvent: Event) {
			const detail = (nativeEvent as CustomEvent<InteractionResponseDetail>).detail
			if (detail?.requestId !== requestId) return

			window.clearTimeout(timer)
			window.removeEventListener(WEBOPS_INTERACTION_RESPONSE_EVENT, handleResponse)
			resolve(detail.response)
		}

		window.addEventListener(WEBOPS_INTERACTION_RESPONSE_EVENT, handleResponse)
	})
}

function getElementSnapshot(index: number | undefined) {
	if (typeof index !== 'number') return undefined

	const element = getElementByIndex(index)
	if (!element) return undefined

	return {
		elementIndex: index,
		role: element.getAttribute('role') || implicitRole(element),
		name:
			element.getAttribute('aria-label') ||
			element.getAttribute('placeholder') ||
			element.getAttribute('title') ||
			element.getAttribute('name') ||
			element.textContent?.trim().slice(0, 120) ||
			undefined,
		text: element.textContent?.trim().slice(0, 120) || undefined,
		css: stableCssSelector(element),
		testId:
			element.getAttribute('data-testid') ||
			element.getAttribute('data-test') ||
			element.getAttribute('data-cy') ||
			undefined,
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
		case 'execute_javascript':
			return 'executeJavascript' as const

		default:
			return action
	}
}
