import type { ContextBudgetReport, ContextSection } from './types'

export function estimateTokens(text: string): number {
	if (!text) return 0
	return Math.ceil(text.length / 3)
}

export function makeBudgetReport(
	sections: ContextSection[],
	maxPromptTokens: number,
	triggeredCompact: boolean
): ContextBudgetReport {
	const items = sections.map((section) => ({
		key: section.key,
		characters: section.content.length,
		estimatedTokens: estimateTokens(section.content),
		status: 'kept' as const,
	}))

	return {
		maxPromptTokens,
		estimatedTokens: items.reduce((sum, item) => sum + item.estimatedTokens, 0),
		items,
		triggeredCompact,
	}
}
