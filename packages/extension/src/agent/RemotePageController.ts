import type { BrowserState } from '@page-agent/page-controller'

import type { InteractionEvent, InteractionResponse } from '@/webops/interactions/interactionTypes'
import type { RecordedActionTarget, RecordedActionType } from '@/webops/recorder/actionEvents'
import { recordWebOpsAction } from '@/webops/recorder/runtimeSession'

import type { TabsController } from './TabsController'
import { normalizeBrowserStateResponse } from './browserState'

const PREFIX = '[RemotePageController]'

const debug = console.debug.bind(console, `\x1b[90m${PREFIX}\x1b[0m`)

function sendMessage(message: {
	type: 'PAGE_CONTROL'
	action: string
	targetTabId: number
	payload?: any
}): Promise<any> {
	return chrome.runtime.sendMessage(message).catch((error) => {
		console.error(PREFIX, message.action, error)
		return null
	})
}

/**
 * Agent side page controller.
 * - live in the agent env (extension page or content script)
 * - communicates with remote PageController via sw
 */
export class RemotePageController {
	tabsController: TabsController

	constructor(tabsController: TabsController) {
		this.tabsController = tabsController
	}

	get currentTabId(): number | null {
		return this.tabsController.currentTabId
	}

	private async getCurrentUrl(): Promise<string> {
		if (!this.currentTabId) return ''
		const { url } = await this.tabsController.getTabInfo(this.currentTabId)
		return url || ''
	}

	private async getCurrentTitle(): Promise<string> {
		if (!this.currentTabId) return ''
		const { title } = await this.tabsController.getTabInfo(this.currentTabId)
		return title || ''
	}

	async getLastUpdateTime(): Promise<number> {
		if (!this.currentTabId) throw new Error('tabsController not initialized.')
		return sendMessage({
			type: 'PAGE_CONTROL',
			action: 'get_last_update_time',
			targetTabId: this.currentTabId,
		})
	}

	async getBrowserState(): Promise<BrowserState> {
		let browserState: BrowserState
		debug('getBrowserState', this.currentTabId)

		const currentUrl = await this.getCurrentUrl()
		const currentTitle = await this.getCurrentTitle()

		if (!this.currentTabId || !isContentScriptAllowed(currentUrl)) {
			browserState = {
				url: currentUrl,
				title: currentTitle,
				header: '',
				content: '(empty page. either current page is not readable or not loaded yet.)',
				footer: '',
			}
		} else {
			const response = await sendMessage({
				type: 'PAGE_CONTROL',
				action: 'get_browser_state',
				targetTabId: this.currentTabId,
			})
			browserState = normalizeBrowserStateResponse(response, currentUrl, currentTitle)
		}

		const sum = await this.tabsController.summarizeTabs()
		browserState.header = sum + '\n\n' + (browserState.header || '')

		debug('getBrowserState: success', this.currentTabId, browserState)

		return browserState
	}

	async updateTree(): Promise<void> {
		if (!this.currentTabId || !isContentScriptAllowed(await this.getCurrentUrl())) {
			return
		}

		await sendMessage({
			type: 'PAGE_CONTROL',
			action: 'update_tree',
			targetTabId: this.currentTabId,
		})
	}

	async cleanUpHighlights(): Promise<void> {
		if (!this.currentTabId || !isContentScriptAllowed(await this.getCurrentUrl())) {
			return
		}

		await sendMessage({
			type: 'PAGE_CONTROL',
			action: 'clean_up_highlights',
			targetTabId: this.currentTabId,
		})
	}

	async clickElement(...args: any[]): Promise<DomActionReturn> {
		await this.publishDomActionSpotlight('click_element', args)
		const target = await this.getDomActionTarget(args)
		const res = await this.remoteCallDomAction('click_element', args)
		await this.recordDomAction('click_element', args, res, target)
		// @note may cause page navigation, wait for 1 second to ensure the page loading started
		await new Promise((resolve) => setTimeout(resolve, 1000))
		return res
	}

	async inputText(...args: any[]): Promise<DomActionReturn> {
		await this.publishDomActionSpotlight('input_text', args)
		const target = await this.getDomActionTarget(args)
		const res = await this.remoteCallDomAction('input_text', args)
		await this.recordDomAction('input_text', args, res, target)
		return res
	}

	async selectOption(...args: any[]): Promise<DomActionReturn> {
		await this.publishDomActionSpotlight('select_option', args)
		const target = await this.getDomActionTarget(args)
		const res = await this.remoteCallDomAction('select_option', args)
		await this.recordDomAction('select_option', args, res, target)
		return res
	}

	async scroll(...args: any[]): Promise<DomActionReturn> {
		return this.remoteCallDomAction('scroll', args)
	}

	async scrollHorizontally(...args: any[]): Promise<DomActionReturn> {
		return this.remoteCallDomAction('scroll_horizontally', args)
	}

	async executeJavascript(...args: any[]): Promise<DomActionReturn> {
		return this.remoteCallDomAction('execute_javascript', args)
	}

	async requestInteraction(
		event: InteractionEvent,
		timeoutMs: number = 600_000
	): Promise<InteractionResponse> {
		if (!this.currentTabId || !isContentScriptAllowed(await this.getCurrentUrl())) {
			return { type: 'cancelled', reason: 'current_page_not_available' }
		}

		return sendMessage({
			type: 'PAGE_CONTROL',
			action: 'webops_interaction_request',
			targetTabId: this.currentTabId,
			payload: { event, timeoutMs },
		})
	}

	async requestUserHandover(message: string): Promise<InteractionResponse> {
		return this.requestInteraction(
			{
				type: 'handover',
				requestId: crypto.randomUUID(),
				title: '需要你接管页面',
				message,
				resumeButtonLabel: '我已完成，继续',
			},
			600_000
		)
	}

	/** @note Managed by content script via storage polling. */
	async showMask(): Promise<void> {}
	/** @note Managed by content script via storage polling. */
	async hideMask(): Promise<void> {}
	/** @note Managed by content script via storage polling. */
	dispose(): void {}

	private async remoteCallDomAction(action: string, payload: any[]): Promise<DomActionReturn> {
		if (!this.currentTabId) {
			return { success: false, message: 'RemotePageController not initialized.' }
		}

		if (!isContentScriptAllowed(await this.getCurrentUrl())) {
			return {
				success: false,
				message:
					'Operation not allowed on this page. Use open_new_tab to navigate to a web page first.',
			}
		}

		return sendMessage({
			type: 'PAGE_CONTROL',
			action: action,
			targetTabId: this.currentTabId!,
			payload,
		})
	}

	private async publishInteraction(event: InteractionEvent) {
		if (!this.currentTabId) return
		if (!isContentScriptAllowed(await this.getCurrentUrl())) return

		await sendMessage({
			type: 'PAGE_CONTROL',
			action: 'webops_interaction',
			targetTabId: this.currentTabId,
			payload: event,
		})
	}

	private async publishDomActionSpotlight(action: string, payload: any[]) {
		const elementIndex = Number(payload[0])
		if (!Number.isFinite(elementIndex)) return

		const event = getSpotlightEvent(action, elementIndex)
		if (!event) return

		await this.publishInteraction(event)
	}

	private async getElementSnapshot(index: number): Promise<RecordedActionTarget | undefined> {
		if (!this.currentTabId) return undefined

		return sendMessage({
			type: 'PAGE_CONTROL',
			action: 'get_element_snapshot',
			targetTabId: this.currentTabId,
			payload: [index],
		})
	}

	private async getDomActionTarget(payload: any[]): Promise<RecordedActionTarget | undefined> {
		const elementIndex = Number(payload[0])
		if (!Number.isFinite(elementIndex)) return undefined
		return this.getElementSnapshot(elementIndex)
	}

	private async recordDomAction(
		action: string,
		payload: any[],
		result: DomActionReturn,
		target?: RecordedActionTarget
	) {
		const type = toRecordedActionType(action)
		if (!type) return

		const elementIndex = Number(payload[0])
		const currentUrl = await this.getCurrentUrl()
		const currentTitle = await this.getCurrentTitle()

		recordWebOpsAction({
			id: crypto.randomUUID(),
			type,
			timestamp: Date.now(),
			pageUrl: currentUrl,
			pageTitle: currentTitle,
			target: target ?? (Number.isFinite(elementIndex) ? { elementIndex } : undefined),
			value: typeof payload[1] === 'string' ? payload[1] : undefined,
			result: result.success ? 'success' : 'failed',
			note: result.message,
		})
	}
}

function getSpotlightEvent(action: string, elementIndex: number): InteractionEvent | undefined {
	if (action === 'click_element') {
		return {
			type: 'spotlight',
			elementIndex,
			action: 'click',
			message: '准备点击页面元素',
		}
	}

	if (action === 'input_text') {
		return {
			type: 'spotlight',
			elementIndex,
			action: 'input',
			message: '准备在页面输入内容',
		}
	}

	if (action === 'select_option') {
		return {
			type: 'spotlight',
			elementIndex,
			action: 'select',
			message: '准备选择页面选项',
		}
	}

	return undefined
}

function toRecordedActionType(action: string): RecordedActionType | undefined {
	if (action === 'click_element') return 'click'
	if (action === 'input_text') return 'input'
	if (action === 'select_option') return 'select'
	return undefined
}

interface DomActionReturn {
	success: boolean
	message: string
}

/**
 * Check if a URL can run content scripts.
 */
export function isContentScriptAllowed(url: string | undefined): boolean {
	if (!url) return false

	const restrictedPatterns = [
		/^chrome:\/\//,
		/^chrome-extension:\/\//,
		/^about:/,
		/^edge:\/\//,
		/^brave:\/\//,
		/^opera:\/\//,
		/^vivaldi:\/\//,
		/^file:\/\//,
		/^view-source:/,
		/^devtools:\/\//,
	]

	return !restrictedPatterns.some((pattern) => pattern.test(url))
}
