import { shouldUseSensitiveHandover } from './sensitiveHandover'

export interface PendingUserQuestion {
	id: string
	question: string
	kind: 'input' | 'handover'
}

interface AskUserBridge {
	ask: (question: string) => Promise<string>
	askWithTaskContext: (task: string, question: string) => Promise<string>
	answer: (answer: string) => boolean
	cancel: (reason: string) => boolean
	getPending: () => PendingUserQuestion | null
}

const DEFAULT_CANCEL_REASON = '用户没有补充答案。'

export function createAskUserBridge(
	onPendingChange: (pending: PendingUserQuestion | null) => void
): AskUserBridge {
	let nextId = 0
	let pending: PendingUserQuestion | null = null
	let resolvePending: ((answer: string) => void) | null = null

	const resolveAndClear = (answer: string) => {
		const resolve = resolvePending
		if (!resolve) return false

		resolvePending = null
		pending = null
		onPendingChange(null)
		resolve(answer)
		return true
	}

	const askWithTaskContext = (task: string, question: string) => {
		resolveAndClear(DEFAULT_CANCEL_REASON)

		pending = {
			id: String(++nextId),
			question,
			kind: shouldUseSensitiveHandover(task, question) ? 'handover' : 'input',
		}
		onPendingChange(pending)

		return new Promise<string>((resolve) => {
			resolvePending = resolve
		})
	}

	return {
		ask(question) {
			return askWithTaskContext('', question)
		},

		askWithTaskContext,

		answer(answer) {
			return resolveAndClear(answer)
		},

		cancel(reason) {
			return resolveAndClear(reason)
		},

		getPending() {
			return pending
		},
	}
}
