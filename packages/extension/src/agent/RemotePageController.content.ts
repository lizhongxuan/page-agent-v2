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
import type { RecordedAction } from '@/webops/recorder/actionEvents'
import {
	MANUAL_RECORDING_ACTIONS_STORAGE_KEY,
	MANUAL_RECORDING_STATE_STORAGE_KEY,
	type ManualRecordedEventSnapshot,
	type ManualRecordedEventType,
	type ManualRecordingState,
	manualEventToRecordedAction,
	mergeManualRecordedAction,
} from '@/webops/recorder/manualRecording'
import {
	runBrowserWorkflowReplay,
	runBrowserWorkflowStep,
} from '@/webops/workflow/browserWorkflowReplay'

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
	initManualWorkflowRecorder()

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

			case 'run_workflow_recipe':
				runBrowserWorkflowReplay(payload?.workflow, payload?.bindings)
					.then((result) => sendResponse(result))
					.catch((error: unknown) =>
						sendResponse({
							ok: false,
							message: error instanceof Error ? error.message : String(error),
						})
					)
				break

			case 'run_workflow_step':
				runBrowserWorkflowStep(payload?.step, payload?.bindings)
					.then((result) => sendResponse(result))
					.catch((error: unknown) =>
						sendResponse({
							ok: false,
							message: error instanceof Error ? error.message : String(error),
						})
					)
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

function initManualWorkflowRecorder() {
	document.addEventListener(
		'click',
		(event) => {
			void recordManualDomEvent('click', event)
		},
		true
	)
	document.addEventListener(
		'input',
		(event) => {
			void recordManualDomEvent('input', event)
		},
		true
	)
	document.addEventListener(
		'change',
		(event) => {
			void recordManualDomEvent('change', event)
		},
		true
	)
	document.addEventListener(
		'keydown',
		(event) => {
			if (event.key !== 'Enter') return
			void recordManualDomEvent('keydown', event)
		},
		true
	)
}

async function recordManualDomEvent(type: ManualRecordedEventType, event: Event) {
	const state = await getManualRecordingState()
	if (!state?.active) return
	const rawTarget = event.target instanceof HTMLElement ? event.target : undefined
	const target = rawTarget ? manualRecordingTargetForEvent(type, rawTarget) : undefined
	if (!target || shouldIgnoreManualRecordingTarget(target)) return
	if (type === 'input' && !isTextEntryTarget(target)) return
	const snapshot = toManualEventSnapshot(state, type, event, target)
	const action = manualEventToRecordedAction(snapshot)
	await appendManualRecordedAction(action)
}

async function getManualRecordingState(): Promise<ManualRecordingState | undefined> {
	const stored = await chrome.storage.local.get(MANUAL_RECORDING_STATE_STORAGE_KEY)
	return stored[MANUAL_RECORDING_STATE_STORAGE_KEY] as ManualRecordingState | undefined
}

function toManualEventSnapshot(
	state: ManualRecordingState,
	type: ManualRecordedEventType,
	event: Event,
	target: HTMLElement
): ManualRecordedEventSnapshot {
	return {
		id: `${state.id}_${Date.now()}_${Math.random().toString(16).slice(2)}`,
		type,
		timestamp: Date.now(),
		pageUrl: location.href,
		pageTitle: document.title,
		target: {
			role: target.getAttribute('role') || implicitRole(target),
			name: getAccessibleName(target),
			text: targetText(target),
			css: stableCssSelector(target),
			testId:
				target.getAttribute('data-testid') ||
				target.getAttribute('data-test') ||
				target.getAttribute('data-cy') ||
				undefined,
			inputType: target instanceof HTMLInputElement ? target.type : undefined,
		},
		value: valueFromTarget(target),
		key: event instanceof KeyboardEvent ? event.key : undefined,
	}
}

function manualRecordingTargetForEvent(
	type: ManualRecordedEventType,
	target: HTMLElement
): HTMLElement {
	if (type !== 'click') return target
	return (
		target.closest<HTMLElement>(
			'button,a,input,textarea,select,[role],[tabindex]:not([tabindex="-1"])'
		) ?? target
	)
}

async function appendManualRecordedAction(action: RecordedAction) {
	const stored = await chrome.storage.local.get(MANUAL_RECORDING_ACTIONS_STORAGE_KEY)
	const actions = Array.isArray(stored[MANUAL_RECORDING_ACTIONS_STORAGE_KEY])
		? (stored[MANUAL_RECORDING_ACTIONS_STORAGE_KEY] as RecordedAction[])
		: []
	await chrome.storage.local.set({
		[MANUAL_RECORDING_ACTIONS_STORAGE_KEY]: mergeManualRecordedAction(actions, action),
	})
}

function shouldIgnoreManualRecordingTarget(target: HTMLElement) {
	return target.closest('[data-page-agent-ignore-recording="true"]') !== null
}

function isTextEntryTarget(target: HTMLElement): boolean {
	if (target instanceof HTMLTextAreaElement) return true
	if (target.isContentEditable) return true
	if (!(target instanceof HTMLInputElement)) return false
	return ['', 'text', 'search', 'email', 'url', 'tel', 'number', 'password'].includes(target.type)
}

function valueFromTarget(target: HTMLElement) {
	if (target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement) {
		return target.value
	}
	if (target instanceof HTMLSelectElement) {
		return target.value
	}
	return undefined
}

function targetText(target: HTMLElement) {
	const text = target.textContent?.trim()
	return text ? text.slice(0, 120) : undefined
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
	return (
		action === 'get_browser_state' || action === 'update_tree' || action === 'get_workflow_elements'
	)
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

function getWorkflowElements() {
	return {
		visibleText: getVisibleTextLines(),
		controls: getVisibleControls(),
	}
}

function getVisibleTextLines() {
	const text = document.body?.innerText || ''
	return text
		.split(/\n+/)
		.map((line) => normalizeSpace(line).slice(0, 240))
		.filter((line) => line.length > 0)
		.slice(0, 80)
}

function getVisibleControls() {
	return Array.from(
		document.querySelectorAll<HTMLElement>(
			'button,a,input,textarea,select,[role],[tabindex]:not([tabindex="-1"])'
		)
	)
		.filter(isVisibleElement)
		.map((element) => ({
			role: element.getAttribute('role') || implicitRole(element) || 'control',
			name: getAccessibleName(element),
		}))
		.filter((control) => control.name.length > 0)
		.slice(0, 120)
}

function isVisibleElement(element: HTMLElement) {
	const style = window.getComputedStyle(element)
	if (style.display === 'none' || style.visibility === 'hidden' || Number(style.opacity) === 0) {
		return false
	}

	const rect = element.getBoundingClientRect()
	return rect.width > 0 && rect.height > 0
}

function getAccessibleName(element: HTMLElement) {
	const labelledBy = element.getAttribute('aria-labelledby')
	const labelledText = labelledBy
		?.split(/\s+/)
		.map((id) => document.getElementById(id)?.textContent || '')
		.join(' ')

	const directLabel =
		element.getAttribute('aria-label') ||
		labelledText ||
		getAssociatedLabelText(element) ||
		element.getAttribute('placeholder') ||
		element.getAttribute('title') ||
		element.getAttribute('name') ||
		(element instanceof HTMLInputElement ? element.value : '') ||
		element.textContent ||
		''

	return normalizeSpace(directLabel).slice(0, 160)
}

function getAssociatedLabelText(element: HTMLElement) {
	const id = element.id
	if (id) {
		const label = document.querySelector<HTMLLabelElement>(`label[for="${CSS.escape(id)}"]`)
		if (label?.textContent) return label.textContent
	}

	const parentLabel = element.closest('label')
	return parentLabel?.textContent || ''
}

function normalizeSpace(value: string) {
	return value.replace(/\s+/g, ' ').trim()
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
	const tagName = element.tagName.toLowerCase()
	if (
		(element instanceof HTMLInputElement ||
			element instanceof HTMLTextAreaElement ||
			element instanceof HTMLSelectElement) &&
		element.getAttribute('name')
	) {
		return `${tagName}[name="${CSS.escape(element.getAttribute('name') || '')}"]`
	}
	if (element instanceof HTMLAnchorElement) {
		const href = element.getAttribute('href')
		if (href && href !== '#') return `a[href="${CSS.escape(href)}"]`
	}
	return undefined
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
