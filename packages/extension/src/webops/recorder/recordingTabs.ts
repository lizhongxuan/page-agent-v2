export const WORKFLOW_TARGET_TAB_STORAGE_KEY = 'workflowTargetTabId'

export interface RecordingTabCandidate {
	id?: number
	url?: string
	active?: boolean
}

export function selectRecordableTab(
	activeTabs: RecordingTabCandidate[],
	allTabs: RecordingTabCandidate[],
	extensionOrigin: string,
	rememberedTabId?: number
): RecordingTabCandidate | undefined {
	const active = findRecordableTab(activeTabs, extensionOrigin)
	if (active) return active

	const remembered = allTabs.find(
		(tab) =>
			typeof tab.id === 'number' &&
			tab.id === rememberedTabId &&
			isRecordableTab(tab, extensionOrigin)
	)
	if (remembered) return remembered

	return allTabs.find((tab) => isRecordableTab(tab, extensionOrigin))
}

export function findRecordableTab(
	tabs: RecordingTabCandidate[],
	extensionOrigin: string
): RecordingTabCandidate | undefined {
	return tabs.find((tab) => isRecordableTab(tab, extensionOrigin))
}

export function isRecordableTab(tab: RecordingTabCandidate, extensionOrigin: string): boolean {
	if (typeof tab.id !== 'number') return false
	if (!tab.url) return false
	if (tab.url.startsWith(extensionOrigin)) return false
	return tab.url.startsWith('http://') || tab.url.startsWith('https://')
}
