const REDACTED_VALUE = '[REDACTED]'

const sensitiveNamePattern =
	/(password|passwd|pwd|token|secret|apikey|api_key|api-key|api\s+key|authorization|auth|cookie|captcha|mfa|otp|2fa|verification|验证码|校验码|密码|密钥|令牌)/i

const tokenLikePattern = /(sk-[a-zA-Z0-9_-]{12,}|Bearer\s+[a-zA-Z0-9._-]{12,}|[a-zA-Z0-9_-]{32,})/

export function redactSensitiveValue(fieldName: string, value: string) {
	if (sensitiveNamePattern.test(fieldName)) return REDACTED_VALUE
	if (tokenLikePattern.test(value)) return REDACTED_VALUE
	return value
}

export function isRedactedValue(value: string) {
	return value === REDACTED_VALUE
}
