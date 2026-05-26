import type { RecordedAction, RecordedSession } from '../../recorder/actionEvents'
import { toPlaywrightLocator } from '../selectorStrategy'

export function testFileTemplate(session: RecordedSession) {
	const lines = session.steps.map(actionToCode).filter(Boolean).join('\n')

	return `import { expect, test } from '@playwright/test'

test('${escapeText(session.task)}', async ({ page }) => {
\tawait page.goto('${escapeText(session.startUrl)}')
${lines}
})
`
}

function actionToCode(action: RecordedAction) {
	if (action.result === 'failed') {
		return `\t// Skipped failed ${action.type} action ${action.id}: ${escapeText(
			action.note ?? 'no note recorded'
		)}`
	}

	if (action.type === 'handover') {
		return `\t// Manual handover: ${escapeText(action.note ?? 'complete this step manually')}`
	}

	if (action.type === 'navigate') {
		return `\tawait page.goto('${escapeText(action.value ?? action.pageUrl)}')`
	}

	if (action.type === 'input' && action.target && action.value !== undefined) {
		return `\tawait ${toPlaywrightLocator(action.target)}.fill('${escapeText(action.value)}')`
	}

	if (action.type === 'select' && action.target && action.value !== undefined) {
		return `\tawait ${toPlaywrightLocator(action.target)}.selectOption('${escapeText(action.value)}')`
	}

	if (action.type === 'click' && action.target) {
		return `\tawait ${toPlaywrightLocator(action.target)}.click()`
	}

	if (action.type === 'extract' && action.target?.text) {
		return `\tawait expect(page.getByText('${escapeText(action.target.text)}')).toBeVisible()`
	}

	if (action.type === 'wait') {
		return `\tawait page.waitForLoadState('networkidle')`
	}

	if (action.type === 'observe') {
		return `\t// Observed page state: ${escapeText(action.note ?? action.pageTitle)}`
	}

	return ''
}

function escapeText(value: string) {
	return value.replace(/\\/g, '\\\\').replace(/'/g, "\\'")
}
