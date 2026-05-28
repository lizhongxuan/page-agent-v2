import { describe, expect, it } from 'vitest'

import { manualEventToRecordedAction, mergeManualRecordedAction } from './manualRecording'

describe('manualEventToRecordedAction', () => {
	it('converts a manual click snapshot into a recorded click action', () => {
		const action = manualEventToRecordedAction({
			id: 'manual_1',
			type: 'click',
			timestamp: 10,
			pageUrl: 'https://github.com/microsoft/playwright',
			pageTitle: 'microsoft/playwright',
			target: {
				role: 'link',
				name: 'Issues',
				css: 'a[href$="/issues"]',
				testId: 'repo-tab-issues',
				text: 'Issues',
			},
		})

		expect(action).toEqual({
			id: 'manual_1',
			type: 'click',
			timestamp: 10,
			pageUrl: 'https://github.com/microsoft/playwright',
			pageTitle: 'microsoft/playwright',
			target: {
				role: 'link',
				name: 'Issues',
				css: 'a[href$="/issues"]',
				testId: 'repo-tab-issues',
				text: 'Issues',
			},
			result: 'success',
		})
	})

	it('converts manual input snapshots and redacts password-like values', () => {
		const action = manualEventToRecordedAction({
			id: 'manual_2',
			type: 'change',
			timestamp: 11,
			pageUrl: 'https://github.com/session',
			pageTitle: 'Sign in',
			target: {
				role: 'textbox',
				name: 'Password',
				css: 'input[type="password"]',
				inputType: 'password',
			},
			value: 'super-secret-password',
		})

		expect(action).toEqual({
			id: 'manual_2',
			type: 'input',
			timestamp: 11,
			pageUrl: 'https://github.com/session',
			pageTitle: 'Sign in',
			target: {
				role: 'textbox',
				name: 'Password',
				css: 'input[type="password"]',
				text: undefined,
				testId: undefined,
			},
			value: '[REDACTED]',
			result: 'success',
			note: 'sensitive-value-redacted',
		})
	})

	it('converts Enter keydown snapshots into press actions', () => {
		const action = manualEventToRecordedAction({
			id: 'manual_3',
			type: 'keydown',
			timestamp: 12,
			pageUrl: 'https://github.com/microsoft/playwright/issues',
			pageTitle: 'Issues',
			target: {
				role: 'searchbox',
				name: 'Search all issues',
				css: 'input[name="q"]',
			},
			key: 'Enter',
		})

		expect(action).toEqual({
			id: 'manual_3',
			type: 'input',
			timestamp: 12,
			pageUrl: 'https://github.com/microsoft/playwright/issues',
			pageTitle: 'Issues',
			target: {
				role: 'searchbox',
				name: 'Search all issues',
				css: 'input[name="q"]',
				text: undefined,
				testId: undefined,
			},
			value: 'Enter',
			result: 'success',
			note: 'keydown',
		})
	})

	it('converts select change snapshots into recorded select actions', () => {
		const action = manualEventToRecordedAction({
			id: 'manual_4',
			type: 'change',
			timestamp: 13,
			pageUrl: 'https://ops.example.test/orders',
			pageTitle: 'Orders',
			target: {
				role: 'combobox',
				name: 'Status',
				css: '#status-filter',
				inputType: 'select-one',
			},
			value: 'delayed',
		})

		expect(action).toEqual({
			id: 'manual_4',
			type: 'select',
			timestamp: 13,
			pageUrl: 'https://ops.example.test/orders',
			pageTitle: 'Orders',
			target: {
				role: 'combobox',
				name: 'Status',
				css: '#status-filter',
				text: undefined,
				testId: undefined,
			},
			value: 'delayed',
			result: 'success',
		})
	})

	it('converts live input snapshots into input actions', () => {
		const action = manualEventToRecordedAction({
			id: 'manual_5',
			type: 'input',
			timestamp: 14,
			pageUrl: 'https://ops.example.test/orders',
			pageTitle: 'Orders',
			target: {
				role: 'textbox',
				name: 'Search customers or orders',
				css: '#order-query',
			},
			value: 'Beta outage',
		})

		expect(action).toMatchObject({
			id: 'manual_5',
			type: 'input',
			value: 'Beta outage',
			result: 'success',
		})
	})

	it('coalesces consecutive input updates on the same target', () => {
		const previous = manualEventToRecordedAction({
			id: 'manual_5',
			type: 'input',
			timestamp: 14,
			pageUrl: 'https://ops.example.test/orders',
			pageTitle: 'Orders',
			target: {
				role: 'textbox',
				name: 'Search customers or orders',
				css: '#order-query',
			},
			value: 'Beta',
		})
		const next = { ...previous, id: 'manual_6', timestamp: 15, value: 'Beta outage' }

		expect(mergeManualRecordedAction([previous], next)).toEqual([{ ...next }])
	})
})
