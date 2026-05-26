import { describe, expect, it } from 'vitest'

import { formatKnowledgeForPrompt } from './promptFormatter'

describe('formatKnowledgeForPrompt', () => {
	it('formats hits as escaped project knowledge XML', () => {
		const xml = formatKnowledgeForPrompt([
			{
				id: 'k1',
				title: 'KCS "服务" <管理> & 手册',
				source: 'docs/kcs.md',
				snippet: '服务管理页可通过搜索框按服务名称搜索，避免输入 <script> & "raw"。',
				url: 'https://kb.example.test/kcs?x=1&y=2',
				score: 0.914,
				tags: ['service', 'kcs'],
			},
		])

		expect(xml).toMatch(/^<project_knowledge>/)
		expect(xml).toMatch(/<\/project_knowledge>$/)
		expect(xml).toMatch(/source="KCS &quot;服务&quot; &lt;管理&gt; &amp; 手册"/)
		expect(xml).toMatch(/score="0.91"/)
		expect(xml).toMatch(/&lt;script&gt; &amp; &quot;raw&quot;/)
	})

	it('returns an empty prompt context for no hits', () => {
		expect(formatKnowledgeForPrompt([])).toBe('')
	})
})
