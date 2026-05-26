import { describe, expect, it } from 'vitest'
import * as z from 'zod/v4'

import { OpenAIClient } from './OpenAIClient'
import { InvokeError } from './errors'
import type { LLMConfig, Tool } from './types'

describe('OpenAIClient', () => {
	it('reports non-JSON API responses clearly', async () => {
		const client = new OpenAIClient({
			...baseConfig(),
			customFetch: async () =>
				new Response('<!DOCTYPE html><html><body>gateway page</body></html>', {
					status: 200,
					headers: { 'content-type': 'text/html' },
				}),
		})

		await expect(client.invoke([], tools(), new AbortController().signal)).rejects.toMatchObject({
			name: 'InvokeError',
			message: expect.stringContaining('LLM API returned non-JSON response'),
		})

		await expect(client.invoke([], tools(), new AbortController().signal)).rejects.toBeInstanceOf(
			InvokeError
		)
	})
})

function baseConfig(): Required<LLMConfig> {
	return {
		baseURL: 'https://example.com/v1',
		model: 'test-model',
		apiKey: 'test-key',
		temperature: 0,
		maxRetries: 0,
		transformRequestBody: (body) => body,
		disableNamedToolChoice: false,
		customFetch: fetch.bind(globalThis),
	}
}

function tools(): Record<string, Tool> {
	return {
		done: {
			description: 'finish',
			inputSchema: z.object({}),
			execute: async () => 'done',
		},
	}
}
