import type { MemorySiteManualKnowledge } from '@/webops/memory/types'

export function formatSiteManualKnowledgeForPrompt(
	items: MemorySiteManualKnowledge[] | undefined
): string {
	if (!items?.length) return ''

	const manuals = items
		.map((item) => {
			const id = escapeXml(item.id)
			const confidence =
				typeof item.confidence === 'number' ? ` confidence="${item.confidence.toFixed(2)}"` : ''
			const summary = escapeXml(item.summary ?? item.text ?? '')
			const sourceRefs = item.sourceRefs?.length
				? `\n    <source_refs>\n${item.sourceRefs.map((ref) => `      ${escapeXml(ref)}`).join('\n')}\n    </source_refs>`
				: ''
			return `  <manual id="${id}"${confidence}>\n    <summary>\n      ${summary}\n    </summary>${sourceRefs}\n  </manual>`
		})
		.join('\n')

	return `<site_manual_knowledge>\n${manuals}\n</site_manual_knowledge>`
}

function escapeXml(value: string): string {
	return value
		.replaceAll('&', '&amp;')
		.replaceAll('<', '&lt;')
		.replaceAll('>', '&gt;')
		.replaceAll('"', '&quot;')
}
