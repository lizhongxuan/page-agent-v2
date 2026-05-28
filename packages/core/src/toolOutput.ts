export function normalizeToolOutput(output: unknown, toolName: string): string {
	if (typeof output === 'string') return output
	if (output == null) return `⚠️ Tool ${toolName} completed without a text result.`
	if (typeof output === 'object') return JSON.stringify(output, null, 2)
	if (typeof output === 'number' || typeof output === 'boolean' || typeof output === 'bigint') {
		return output.toString()
	}
	if (typeof output === 'symbol') return output.description ?? output.toString()
	return `⚠️ Tool ${toolName} returned unsupported output type: ${typeof output}.`
}

export function shouldStopBatchForOutput(output: unknown): boolean {
	const normalized = normalizeToolOutput(output, 'unknown')
	return normalized.startsWith('❌') || normalized.startsWith('⚠️')
}
