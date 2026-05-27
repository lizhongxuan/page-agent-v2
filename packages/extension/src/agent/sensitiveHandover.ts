const sensitiveQuestionPattern =
	/(账号.*密码|密码.*账号|账号密码|用户名.*密码|密码.*用户名|邮箱.*密码|密码.*邮箱|登录.*密码|密码.*登录|验证码|校验码|动态码|短信码|二次验证|两步验证|接管.*页面|页面.*接管|接管.*网页|网页.*接管|手动.*登录|完成.*登录|登录.*完成|自己输入|自行输入|2fa|mfa|otp|password|passwd|passcode|verification code|auth code|take over|handover|two[- ]?factor|multi[- ]?factor)/i

const verificationQuestionPattern =
	/(验证码|校验码|动态码|短信码|二次验证|两步验证|安全验证|人机验证|图形验证|滑块|扫码|otp|captcha|verification code|auth code|2fa|mfa|two[- ]?factor|multi[- ]?factor|authenticator)/i

export interface CompletedSensitiveHandover {
	url: string
	completedAt: number
}

export function isSensitiveHandoverQuestion(question: string): boolean {
	return sensitiveQuestionPattern.test(question)
}

export function shouldUseSensitiveHandover(task: string, question: string): boolean {
	return isSensitiveHandoverQuestion(`${task}\n${question}`)
}

export function isVerificationHandoverQuestion(question: string): boolean {
	return verificationQuestionPattern.test(question)
}

export function shouldSuppressRepeatedSensitiveHandover({
	lastHandover,
	currentUrl,
	question,
	now,
	windowMs = 120_000,
}: {
	lastHandover: CompletedSensitiveHandover | null
	currentUrl: string
	question: string
	now: number
	windowMs?: number
}): boolean {
	if (!lastHandover) return false
	if (isVerificationHandoverQuestion(question)) return false
	if (currentUrl !== lastHandover.url) return false
	return now - lastHandover.completedAt <= windowMs
}

export function formatCompletedSensitiveHandoverResponse(): string {
	return [
		'用户已完成页面接管并交回控制权。',
		'请立即重新观察当前页面状态并继续下一步：如果登录按钮或提交按钮仍可见，请点击登录/提交；如果已经进入目标页面，请继续任务或调用 done。',
		'不要再次请求账号、密码或同一登录接管；只有页面明确出现新的验证码、MFA、CAPTCHA 或二次验证挑战时，才可以再次请求用户接管。',
	].join(' ')
}

export function formatRepeatedSensitiveHandoverResponse(): string {
	return [
		'用户刚刚已完成页面接管并交回控制权。',
		'不要再次请求账号、密码或同一登录接管；请先重新观察当前页面，继续点击登录/提交、等待登录结果或完成任务。',
		'只有页面明确出现新的验证码、MFA、CAPTCHA 或二次验证挑战时，才可以再次请求用户接管。',
	].join(' ')
}
