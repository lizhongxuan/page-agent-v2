export const SIDE_PANEL_HANDOVER_ACTIVE_STORAGE_KEY = 'pageAgentSidePanelHandoverActive'

export async function setSidePanelHandoverActive(active: boolean): Promise<void> {
	await chrome.storage.local.set({ [SIDE_PANEL_HANDOVER_ACTIVE_STORAGE_KEY]: active })
}
