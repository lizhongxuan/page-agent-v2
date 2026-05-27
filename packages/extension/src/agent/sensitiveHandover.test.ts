import { describe, expect, it } from 'vitest'

import {
	formatCompletedSensitiveHandoverResponse,
	formatRepeatedSensitiveHandoverResponse,
	isSensitiveHandoverQuestion,
	isVerificationHandoverQuestion,
	shouldSuppressRepeatedSensitiveHandover,
	shouldUseSensitiveHandover,
} from './sensitiveHandover'

describe('sensitive handover detection', () => {
	it('routes credential and verification-code questions to page handover', () => {
		expect(isSensitiveHandoverQuestion('请提供网易邮箱账号和密码，我会帮你登录')).toBe(true)
		expect(isSensitiveHandoverQuestion('请输入短信验证码后继续')).toBe(true)
		expect(isSensitiveHandoverQuestion('可以继续，但登录163网易邮箱需要账号密码')).toBe(true)
		expect(isSensitiveHandoverQuestion('请接管页面完成登录后继续')).toBe(true)
	})

	it('does not route ordinary clarification questions to page handover', () => {
		expect(isSensitiveHandoverQuestion('请提供要查询的股票名称')).toBe(false)
		expect(isSensitiveHandoverQuestion('你要继续看下一页吗？')).toBe(false)
		expect(isSensitiveHandoverQuestion('请在页面输入股票代码')).toBe(false)
	})

	it('uses the original task as context when the ask_user question is generic', () => {
		expect(
			shouldUseSensitiveHandover(
				'登录邮箱，页面需要邮箱账号和密码，但不要读取用户秘密。',
				'请接管页面，然后点继续。'
			)
		).toBe(true)
		expect(shouldUseSensitiveHandover('查询 A 股新闻', '是否继续看下一页？')).toBe(false)
	})

	it('distinguishes fresh verification challenges from repeated credential handovers', () => {
		expect(isVerificationHandoverQuestion('页面要求短信验证码，请用户完成二次验证')).toBe(true)
		expect(isVerificationHandoverQuestion('Please enter the OTP from your authenticator app')).toBe(
			true
		)
		expect(isVerificationHandoverQuestion('请接管页面输入邮箱账号和密码，然后继续')).toBe(false)
	})

	it('suppresses repeated credential handovers on the same page after user completion', () => {
		const completed = { url: 'https://mail.example.test/login', completedAt: 1000 }

		expect(
			shouldSuppressRepeatedSensitiveHandover({
				lastHandover: completed,
				currentUrl: 'https://mail.example.test/login',
				question: '请再次接管页面输入账号密码后继续。',
				now: 1100,
			})
		).toBe(true)
		expect(
			shouldSuppressRepeatedSensitiveHandover({
				lastHandover: completed,
				currentUrl: 'https://mail.example.test/login',
				question: '页面出现验证码，请用户输入短信验证码。',
				now: 1100,
			})
		).toBe(false)
		expect(
			shouldSuppressRepeatedSensitiveHandover({
				lastHandover: completed,
				currentUrl: 'https://mail.example.test/other-login',
				question: '请接管页面输入账号密码后继续。',
				now: 1100,
			})
		).toBe(false)
	})

	it('tells the LLM to continue instead of asking for the same login again', () => {
		expect(formatCompletedSensitiveHandoverResponse()).toContain('不要再次请求账号、密码')
		expect(formatCompletedSensitiveHandoverResponse()).toContain('点击登录/提交')
		expect(formatRepeatedSensitiveHandoverResponse()).toContain('刚刚已完成页面接管')
		expect(formatRepeatedSensitiveHandoverResponse()).toContain('新的验证码')
	})
})
