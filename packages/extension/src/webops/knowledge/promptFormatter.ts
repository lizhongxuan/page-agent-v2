import type { KnowledgeHit } from './types'

export function formatKnowledgeForPrompt(hits: KnowledgeHit[]) {
	if (!hits.length) return ''

	const body = hits
		.map((hit) => {
			return `  <hit source="${escapeXml(hit.title)}" score="${hit.score.toFixed(2)}">${escapeXml(hit.snippet)}</hit>`
		})
		.join('\n')

	return `<project_knowledge>\n${body}\n</project_knowledge>`
}

function escapeXml(value: string) {
	return value
		.replace(/&/g, '&amp;')
		.replace(/</g, '&lt;')
		.replace(/>/g, '&gt;')
		.replace(/"/g, '&quot;')
		.replace(/'/g, '&apos;')
}
