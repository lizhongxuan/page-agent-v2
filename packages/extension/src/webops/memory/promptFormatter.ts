import { formatSiteManualKnowledgeForPrompt } from '@/webops/site-manuals/promptFormatter'

import type { MemoryContextResponse } from './types'

export function formatMemoryForPrompt(response: MemoryContextResponse): string {
	if (response.contextPrompt) return response.contextPrompt

	return [
		formatSiteTaskGuidesForPrompt(response),
		formatSiteManualKnowledgeForPrompt(response.siteManualKnowledge),
	]
		.filter(Boolean)
		.join('\n\n')
}

function formatSiteTaskGuidesForPrompt(response: MemoryContextResponse): string {
	if (!response.siteTaskGuides?.length) return ''

	const guides = response.siteTaskGuides
		.map((guide) => {
			const confidence =
				typeof guide.confidence === 'number' ? ` confidence="${guide.confidence.toFixed(2)}"` : ''
			const whenToUse = guide.whenToUse ?? guide.summary ?? ''
			const matchedState = guide.matchedStateName
				? `\n    <matched_state>${escapeXml(guide.matchedStateName)}</matched_state>`
				: ''
			const matchReasons = listBlock('match_reasons', guide.matchReasons)
			const pageGuards = listBlock('page_guards', guide.pageGuards)
			const steps = numberedBlock(
				'remaining_steps',
				guide.steps,
				typeof guide.startStepOffset === 'number' ? guide.startStepOffset : 0
			)
			const abandonRules = listBlock('abandon_if', guide.abandonRules)
			return `  <guide id="${escapeXml(guide.id)}"${confidence}>\n    <when_to_use>\n      ${escapeXml(whenToUse)}\n    </when_to_use>${matchedState}${matchReasons}${pageGuards}${steps}${abandonRules}\n  </guide>`
		})
		.join('\n')

	return `<site_task_guides>\n${guides}\n</site_task_guides>`
}

function listBlock(tag: string, items: string[] | undefined): string {
	if (!items?.length) return ''
	return `\n    <${tag}>\n${items.map((item) => `      ${escapeXml(item)}`).join('\n')}\n    </${tag}>`
}

function numberedBlock(tag: string, items: string[] | undefined, startOffset = 0): string {
	if (!items?.length) return ''
	return `\n    <${tag}>\n${items.map((item, index) => `      ${startOffset + index + 1}. ${escapeXml(item)}`).join('\n')}\n    </${tag}>`
}

function escapeXml(value: string): string {
	return value
		.replaceAll('&', '&amp;')
		.replaceAll('<', '&lt;')
		.replaceAll('>', '&gt;')
		.replaceAll('"', '&quot;')
}
