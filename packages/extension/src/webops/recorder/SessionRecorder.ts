import type { RecordedAction, RecordedMemoryContext, RecordedSession } from './actionEvents'
import { redactSensitiveValue } from './redaction'

export class SessionRecorder {
	private session?: RecordedSession

	start(input: { id: string; task: string; startUrl: string }) {
		this.session = {
			id: input.id,
			task: input.task,
			startUrl: input.startUrl,
			startedAt: Date.now(),
			steps: [],
			knowledgeHits: [],
			redactionReport: [],
		}
	}

	record(action: RecordedAction) {
		if (!this.session) return

		const fieldName = getActionValueFieldName(action)
		const value =
			action.value === undefined ? undefined : redactSensitiveValue(fieldName, action.value)

		if (action.value !== undefined && value !== action.value) {
			this.session.redactionReport.push({ actionId: action.id, field: 'value' })
		}

		this.session.steps.push({ ...action, value })
	}

	addKnowledgeHits(hits: RecordedSession['knowledgeHits']) {
		if (!this.session) return
		this.session.knowledgeHits.push(...hits)
	}

	setMemoryContext(context: RecordedMemoryContext) {
		if (!this.session) return
		this.session.memoryContext = context
	}

	finish() {
		if (!this.session) throw new Error('SessionRecorder has not started')
		this.session.endedAt = Date.now()
		return this.session
	}
}

function getActionValueFieldName(action: RecordedAction) {
	return [
		action.target?.name,
		action.target?.testId,
		action.target?.role,
		action.target?.text,
		action.type,
	]
		.filter(Boolean)
		.join(' ')
}
