import type { RecordedSession } from '../recorder/actionEvents'

export interface ReplayExampleSession {
	id: string
	title: string
	description: string
	session: RecordedSession
}

export function createReplayExampleSessions(
	baseUrl = 'https://example.test'
): ReplayExampleSession[] {
	const normalizedBaseUrl = baseUrl.replace(/\/+$/, '')
	return [
		createServiceSearchDetailExample(normalizedBaseUrl),
		createSettingsFilterExportExample(normalizedBaseUrl),
		createManualHandoverReviewExample(normalizedBaseUrl),
	]
}

function createServiceSearchDetailExample(baseUrl: string): ReplayExampleSession {
	return {
		id: 'service-search-detail',
		title: 'Service search and detail check',
		description: 'Searches for a service, opens its detail page, and checks the health status.',
		session: {
			id: 'example-service-search-detail',
			task: 'Search checkout service and verify health',
			startUrl: `${baseUrl}/services`,
			startedAt: 1_700_000_000_000,
			steps: [
				{
					id: 'svc-1',
					type: 'input',
					timestamp: 1_700_000_000_001,
					pageUrl: `${baseUrl}/services`,
					pageTitle: 'Service Console',
					target: { name: 'Service name' },
					value: 'checkout',
					result: 'success',
				},
				{
					id: 'svc-2',
					type: 'click',
					timestamp: 1_700_000_000_002,
					pageUrl: `${baseUrl}/services`,
					pageTitle: 'Service Console',
					target: { role: 'button', name: 'Search' },
					result: 'success',
				},
				{
					id: 'svc-3',
					type: 'wait',
					timestamp: 1_700_000_000_003,
					pageUrl: `${baseUrl}/services`,
					pageTitle: 'Service Console',
					result: 'success',
					note: 'Wait for search results',
				},
				{
					id: 'svc-4',
					type: 'click',
					timestamp: 1_700_000_000_004,
					pageUrl: `${baseUrl}/services`,
					pageTitle: 'Service Console',
					target: { role: 'link', name: 'checkout' },
					result: 'success',
				},
				{
					id: 'svc-5',
					type: 'navigate',
					timestamp: 1_700_000_000_005,
					pageUrl: `${baseUrl}/services`,
					pageTitle: 'Service Console',
					value: `${baseUrl}/services/checkout`,
					result: 'success',
				},
				{
					id: 'svc-6',
					type: 'observe',
					timestamp: 1_700_000_000_006,
					pageUrl: `${baseUrl}/services/checkout`,
					pageTitle: 'Service Detail',
					result: 'success',
					note: 'Confirmed service detail page',
				},
				{
					id: 'svc-7',
					type: 'extract',
					timestamp: 1_700_000_000_007,
					pageUrl: `${baseUrl}/services/checkout`,
					pageTitle: 'Service Detail',
					target: { text: 'Healthy' },
					result: 'success',
				},
			],
			knowledgeHits: [],
			redactionReport: [],
		},
	}
}

function createSettingsFilterExportExample(baseUrl: string): ReplayExampleSession {
	return {
		id: 'settings-filter-export',
		title: 'Settings filter and export',
		description: 'Chooses an environment, applies the filter, then exports the filtered table.',
		session: {
			id: 'example-settings-filter-export',
			task: 'Filter production settings and export table',
			startUrl: `${baseUrl}/settings`,
			startedAt: 1_700_000_010_000,
			steps: [
				{
					id: 'set-1',
					type: 'select',
					timestamp: 1_700_000_010_001,
					pageUrl: `${baseUrl}/settings`,
					pageTitle: 'Settings',
					target: { role: 'combobox', name: 'Environment' },
					value: 'prod',
					result: 'success',
				},
				{
					id: 'set-2',
					type: 'click',
					timestamp: 1_700_000_010_002,
					pageUrl: `${baseUrl}/settings`,
					pageTitle: 'Settings',
					target: { testId: 'apply-filter' },
					result: 'success',
				},
				{
					id: 'set-3',
					type: 'extract',
					timestamp: 1_700_000_010_003,
					pageUrl: `${baseUrl}/settings`,
					pageTitle: 'Settings',
					target: { text: 'Production configuration' },
					result: 'success',
				},
				{
					id: 'set-4',
					type: 'click',
					timestamp: 1_700_000_010_004,
					pageUrl: `${baseUrl}/settings`,
					pageTitle: 'Settings',
					target: { role: 'button', name: 'Export CSV' },
					result: 'success',
				},
			],
			knowledgeHits: [],
			redactionReport: [],
		},
	}
}

function createManualHandoverReviewExample(baseUrl: string): ReplayExampleSession {
	return {
		id: 'manual-handover-review',
		title: 'Manual approval handover',
		description: 'Documents an approval flow where MFA must remain a human step.',
		session: {
			id: 'example-manual-handover-review',
			task: 'Review release approval without automating MFA',
			startUrl: `${baseUrl}/approval`,
			startedAt: 1_700_000_020_000,
			steps: [
				{
					id: 'mfa-1',
					type: 'observe',
					timestamp: 1_700_000_020_001,
					pageUrl: `${baseUrl}/approval`,
					pageTitle: 'Release Approval',
					result: 'success',
					note: 'Approval page is visible',
				},
				{
					id: 'mfa-2',
					type: 'handover',
					timestamp: 1_700_000_020_002,
					pageUrl: `${baseUrl}/approval`,
					pageTitle: 'Release Approval',
					result: 'skipped',
					note: 'Complete MFA approval in the browser',
				},
				{
					id: 'mfa-3',
					type: 'extract',
					timestamp: 1_700_000_020_003,
					pageUrl: `${baseUrl}/approval`,
					pageTitle: 'Release Approval',
					target: { text: 'Approved' },
					result: 'success',
				},
			],
			knowledgeHits: [],
			redactionReport: [],
		},
	}
}
