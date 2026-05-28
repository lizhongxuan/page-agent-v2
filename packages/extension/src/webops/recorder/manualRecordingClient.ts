import type { RecordedSession } from './actionEvents'
import type {
	ManualRecordingControlMessage,
	ManualRecordingControlResponse,
} from './manualRecording'

export type ManualRecordingClientResult =
	| { ok: true; status: 'started' | 'cancelled' }
	| { ok: true; status: 'stopped'; session: RecordedSession }
	| { ok: false; error: string }

export async function startManualRecording(task: string): Promise<ManualRecordingClientResult> {
	return sendToActiveTab({ type: 'WEBOPS_MANUAL_RECORDING', action: 'start', task })
}

export async function stopManualRecording(): Promise<ManualRecordingClientResult> {
	return sendToActiveTab({ type: 'WEBOPS_MANUAL_RECORDING', action: 'stop' })
}

export async function cancelManualRecording(): Promise<ManualRecordingClientResult> {
	return sendToActiveTab({ type: 'WEBOPS_MANUAL_RECORDING', action: 'cancel' })
}

async function sendToActiveTab(
	message: ManualRecordingControlMessage
): Promise<ManualRecordingClientResult> {
	const [tab] = await chrome.tabs.query({ active: true, currentWindow: true })
	if (!tab?.id) return { ok: false, error: 'No active tab is available for recording.' }

	try {
		const response = (await chrome.tabs.sendMessage(
			tab.id,
			message
		)) as ManualRecordingControlResponse
		return response ?? { ok: false, error: 'Manual recorder did not respond.' }
	} catch (error) {
		return {
			ok: false,
			error: error instanceof Error ? error.message : String(error),
		}
	}
}
