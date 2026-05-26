import type { RecordedActionTarget } from '../recorder/actionEvents'

export function toPlaywrightLocator(target: RecordedActionTarget) {
	if (target.testId) return `page.getByTestId('${escapeText(target.testId)}')`
	if (target.role && target.name) {
		return `page.getByRole('${escapeText(target.role)}', { name: '${escapeText(target.name)}' })`
	}
	if (target.name) return `page.getByPlaceholder('${escapeText(target.name)}')`
	if (target.text) return `page.getByText('${escapeText(target.text)}')`
	if (target.css) return `page.locator('${escapeText(target.css)}')`
	if (target.xpath) return `page.locator('xpath=${escapeText(target.xpath)}')`
	return "page.locator('body')"
}

function escapeText(value: string) {
	return value.replace(/\\/g, '\\\\').replace(/'/g, "\\'")
}
