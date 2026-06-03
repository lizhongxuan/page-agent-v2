import { SessionRecorder } from './SessionRecorder'
import type {
	RecordedAction,
	RecordedMemoryContext,
	RecordedMemoryHit,
	RecordedSession,
} from './actionEvents'

let recorder: SessionRecorder | undefined
let lastSession: RecordedSession | undefined

export function startWebOpsSession(input: { id: string; task: string; startUrl: string }) {
	recorder = new SessionRecorder()
	recorder.start(input)
	lastSession = undefined
}

export function recordWebOpsAction(action: RecordedAction) {
	recorder?.record(action)
}

export function addWebOpsMemoryHits(hits: RecordedMemoryHit[]) {
	recorder?.addMemoryHits(
		hits.map((hit) => ({
			id: hit.id,
			title: hit.title,
			source: hit.source,
			score: hit.score,
		}))
	)
}

export function setWebOpsMemoryContext(context: RecordedMemoryContext) {
	recorder?.setMemoryContext(context)
}

export function finishWebOpsSession() {
	if (!recorder) return undefined
	lastSession = recorder.finish()
	recorder = undefined
	return lastSession
}

export function getLastWebOpsSession() {
	return lastSession
}
