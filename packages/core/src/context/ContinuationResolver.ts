import type { BusinessObjectRef, ContextSectionKey, ContinuationDecision } from './types'

export interface ResolveContinuationInput {
	previousTask: string
	userMessage: string
	pendingQuestion?: string
	currentUrl: string
	currentTitle: string
	previousUrl?: string
	previousTitle?: string
	previousBusinessObjects?: BusinessObjectRef[]
	hasUnconfirmedRisk?: boolean
}

export class ContinuationResolver {
	resolve(input: ResolveContinuationInput): ContinuationDecision {
		if (input.hasUnconfirmedRisk) {
			return this.confirm('Unconfirmed high-risk context must not be resumed automatically.')
		}

		if (input.pendingQuestion && this.looksLikeAnswer(input.userMessage)) {
			return {
				mode: 'continue_original_task',
				reason: 'User message answers the pending agent question.',
				inheritedSections: this.continuationSections(),
				discardedSections: [],
				requiresFreshObserve: true,
				requiresUserConfirmation: false,
			}
		}

		const samePage =
			!!input.previousUrl &&
			input.previousUrl === input.currentUrl &&
			(!input.previousTitle || input.previousTitle === input.currentTitle)
		const sameBusinessObject = this.referencesBusinessObject(
			input.userMessage,
			input.previousBusinessObjects ?? []
		)
		const continuationIntent = this.looksLikeContinuation(input.previousTask, input.userMessage)

		if (samePage && !continuationIntent) {
			return {
				mode: 'new_task_same_page',
				reason: 'Same page but user intent changed.',
				inheritedSections: ['currentPage', 'instructions'],
				discardedSections: [
					'task',
					'sessionSummary',
					'recentSteps',
					'businessObjects',
					'protectedFacts',
				],
				requiresFreshObserve: true,
				requiresUserConfirmation: false,
			}
		}

		if (!samePage && sameBusinessObject) {
			return {
				mode: 'same_business_new_page',
				reason: 'Page changed but user referenced the same business object.',
				inheritedSections: ['businessObjects', 'protectedFacts', 'currentPage', 'instructions'],
				discardedSections: ['recentSteps'],
				requiresFreshObserve: true,
				requiresUserConfirmation: false,
			}
		}

		if (samePage && continuationIntent) {
			return {
				mode: 'continue_original_task',
				reason: 'Same page and message appears to continue the original task.',
				inheritedSections: this.continuationSections(),
				discardedSections: [],
				requiresFreshObserve: true,
				requiresUserConfirmation: false,
			}
		}

		return {
			mode: 'fresh_task',
			reason: 'Page and task signals do not match previous context.',
			inheritedSections: ['currentPage', 'instructions'],
			discardedSections: [
				'task',
				'sessionSummary',
				'recentSteps',
				'businessObjects',
				'protectedFacts',
			],
			requiresFreshObserve: true,
			requiresUserConfirmation: false,
		}
	}

	private confirm(reason: string): ContinuationDecision {
		return {
			mode: 'ask_user_to_confirm',
			reason,
			inheritedSections: [],
			discardedSections: [],
			requiresFreshObserve: true,
			requiresUserConfirmation: true,
		}
	}

	private continuationSections(): ContextSectionKey[] {
		return [
			'task',
			'sessionSummary',
			'recentSteps',
			'protectedFacts',
			'businessObjects',
			'currentPage',
			'instructions',
		]
	}

	private looksLikeAnswer(message: string): boolean {
		return /^(yes|no|ok|continue|stop|cancel|继续|可以|确认|不用|不要|是|否)\b/i.test(
			message.trim()
		)
	}

	private looksLikeContinuation(previousTask: string, message: string): boolean {
		const lowerMessage = message.toLowerCase()
		if (this.looksLikeNewSamePageQuestion(lowerMessage)) return false
		const continuationWords = ['continue', '继续', '接着', '再看', '多看', 'same']
		return continuationWords.some((word) => lowerMessage.includes(word.toLowerCase()))
	}

	private looksLikeNewSamePageQuestion(message: string): boolean {
		return /(summari[sz]e|list|describe|explain|what .* page|总结|列出|说明|解释|有哪些|这个页面)/i.test(
			message
		)
	}

	private referencesBusinessObject(message: string, objects: BusinessObjectRef[]): boolean {
		const normalized = message.toLowerCase()
		return objects.some((object) => {
			const candidates = [object.id, object.name, ...(object.aliases ?? [])].filter(Boolean)
			return candidates.some((candidate) => normalized.includes(String(candidate).toLowerCase()))
		})
	}
}
