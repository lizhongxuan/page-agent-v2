import { describe, expect, it } from 'vitest'

import { buildPageFingerprint } from './pageFingerprint'

describe('buildPageFingerprint', () => {
	it('builds a fingerprint from browser state text and controls without screenshots', () => {
		const fingerprint = buildPageFingerprint({
			url: 'https://console.example.test/services?region=us',
			title: 'Service Console',
			visibleText: 'Services Search service Create service Production',
			controls: [
				{ role: 'textbox', name: 'Search service', placeholder: 'Search' },
				{ role: 'button', name: 'Create service' },
			],
		})

		expect(fingerprint.urlPatterns).toEqual(['https://console.example.test/*'])
		expect(fingerprint.titleAny).toContain('Service Console')
		expect(fingerprint.requiredText).toEqual(['Services', 'Search service', 'Create service'])
		expect(fingerprint.controlSignatures).toEqual([
			{ role: 'textbox', name: 'Search service', placeholder: 'Search' },
			{ role: 'button', name: 'Create service' },
		])
		expect(Object.keys(fingerprint)).not.toContain('screenshot')
	})
})
