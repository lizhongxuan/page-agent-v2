import { describe, expect, it } from 'vitest'

import { formatAskUserResponse } from './askUserResponse'

describe('formatAskUserResponse', () => {
	it('does not turn a missing answer into user approval', () => {
		expect(formatAskUserResponse({ type: 'completed' })).toContain('用户没有提供补充信息')
		expect(formatAskUserResponse({ type: 'completed' })).not.toContain('确认继续')
	})

	it('returns actual text input as the user answer', () => {
		expect(formatAskUserResponse({ type: 'input', value: '查股价' })).toBe('查股价')
	})
})
