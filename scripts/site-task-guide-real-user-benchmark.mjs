import { chromium } from '@playwright/test'
import { spawn } from 'node:child_process'
import { mkdir, readFile, readdir, rm, writeFile } from 'node:fs/promises'
import { createServer } from 'node:http'
import path from 'node:path'

const rootDir = process.cwd()
const artifactDir =
	process.env.SITE_TASK_GUIDE_BENCHMARK_ARTIFACT_DIR ??
	path.join(rootDir, 'artifacts/site-task-guide-real-user-benchmark/manual')
const dataDir = path.join(artifactDir, 'backend-data')
const backendAddr = process.env.SITE_TASK_GUIDE_BENCHMARK_BACKEND_ADDR ?? '127.0.0.1:38432'
const backendURL = `http://${backendAddr}`

const loadTestScenarios = [
	{
		id: 'k8s_restart',
		site: 'k8s.example.com',
		module: 'workloads',
		title: 'Kubernetes deployment restart manual',
		manualLines: [
			'# Kubernetes deployment restart',
			'Open Workloads and search the deployment name.',
			'Click the deployment row, open the Operations tab, click Restart Deployment, then confirm restart.',
			'Do not use this procedure for deleting, scaling, or disabling a deployment.',
		],
		taskRun: {
			id: 'task_run_benchmark_k8s_restart_train',
			taskTemplate: 'restart deployment after config update',
			summary: 'Restarted a Kubernetes deployment after applying a config update.',
			originalPath: ['k8s_workloads', 'k8s_detail', 'k8s_operations'],
			optimizedPath: ['k8s_workloads', 'k8s_detail', 'k8s_operations'],
			steps: [
				[
					'fill',
					'Deployment filter',
					'{{deployment_name}}',
					'Filter the deployment list.',
					'Workloads',
				],
				['click', 'Deployment row', '', 'Open the deployment detail page.', 'Workloads'],
				['click', 'Operations', '', 'Open the Operations tab.', 'Deployment Detail'],
				['click', 'Restart Deployment', '', 'Click Restart Deployment.', 'Deployment Operations'],
				['click', 'Confirm Restart', '', 'Confirm deployment restart.', 'Deployment Operations'],
			],
		},
		positiveCase: {
			task: 'restart deployment after config update',
			url: 'https://k8s.example.com/workloads/api-server/operations',
			title: 'Deployment Operations',
			visibleText: ['Deployment Operations', 'Restart Deployment', 'Confirm Restart'],
			controls: [
				{ role: 'tab', name: 'Operations' },
				{ role: 'button', name: 'Restart Deployment' },
				{ role: 'button', name: 'Confirm Restart' },
			],
			baselineActionCount: 7,
		},
		negativeCases: [
			{
				id: 'delete_same_page',
				name: 'Kubernetes delete deployment on restart page',
				task: 'delete deployment and remove pods',
				url: 'https://k8s.example.com/workloads/api-server/operations',
				title: 'Deployment Operations',
				visibleText: ['Deployment Operations', 'Restart Deployment', 'Confirm Restart'],
				controls: [{ role: 'button', name: 'Restart Deployment' }],
				baselineActionCount: 6,
			},
			{
				id: 'wrong_namespace_page',
				name: 'Kubernetes restart from namespace settings',
				task: 'restart deployment after config update',
				url: 'https://k8s.example.com/namespaces/prod/settings',
				title: 'Namespace Settings',
				visibleText: ['Namespace Settings', 'Quota', 'Save'],
				controls: [{ role: 'button', name: 'Save' }],
				baselineActionCount: 7,
			},
		],
	},
	{
		id: 'billing_refund',
		site: 'billing.example.com',
		module: 'orders',
		title: 'Billing order refund manual',
		manualLines: [
			'# Billing order refund',
			'Open Orders, search by order number, and open the order detail page.',
			'Open Payments, click Refund Payment, select an approved refund reason, then confirm refund.',
			'Do not use this procedure for capturing, voiding, or canceling payments.',
		],
		taskRun: {
			id: 'task_run_benchmark_billing_refund_train',
			taskTemplate: 'refund approved order payment',
			summary: 'Refunded an approved order payment from the payment detail page.',
			originalPath: ['billing_orders', 'billing_detail', 'billing_payments'],
			optimizedPath: ['billing_orders', 'billing_detail', 'billing_payments'],
			steps: [
				['fill', 'Order number filter', '{{order_number}}', 'Filter the order list.', 'Orders'],
				['click', 'Order row', '', 'Open the order detail page.', 'Orders'],
				['click', 'Payments', '', 'Open the Payments tab.', 'Order Detail'],
				['click', 'Refund Payment', '', 'Click Refund Payment.', 'Order Payments'],
				[
					'select',
					'Refund reason',
					'customer_request',
					'Select the refund reason.',
					'Order Payments',
				],
				['click', 'Confirm Refund', '', 'Confirm the refund.', 'Order Payments'],
			],
		},
		positiveCase: {
			task: 'refund approved order payment',
			url: 'https://billing.example.com/orders/ORD-100/payments',
			title: 'Order Payments',
			visibleText: ['Order Payments', 'Refund Payment', 'Refund reason', 'Confirm Refund'],
			controls: [
				{ role: 'tab', name: 'Payments' },
				{ role: 'button', name: 'Refund Payment' },
				{ role: 'button', name: 'Confirm Refund' },
			],
			baselineActionCount: 9,
		},
		negativeCases: [
			{
				id: 'capture_same_page',
				name: 'Billing capture payment on refund page',
				task: 'capture authorized order payment',
				url: 'https://billing.example.com/orders/ORD-100/payments',
				title: 'Order Payments',
				visibleText: ['Order Payments', 'Refund Payment', 'Confirm Refund'],
				controls: [{ role: 'button', name: 'Refund Payment' }],
				baselineActionCount: 6,
			},
			{
				id: 'customer_page',
				name: 'Billing refund from customer profile page',
				task: 'refund approved order payment',
				url: 'https://billing.example.com/customers/C-100',
				title: 'Customer Profile',
				visibleText: ['Customer Profile', 'Subscriptions', 'Invoices'],
				controls: [{ role: 'link', name: 'Invoices' }],
				baselineActionCount: 9,
			},
		],
	},
	{
		id: 'support_escalate',
		site: 'support.example.com',
		module: 'tickets',
		title: 'Support ticket escalation manual',
		manualLines: [
			'# Support ticket escalation',
			'Open Tickets, search by ticket ID, and open the ticket detail page.',
			'Set priority to Critical, click Escalate Ticket, choose the incident queue, then confirm escalation.',
			'Do not use this procedure for closing or merging tickets.',
		],
		taskRun: {
			id: 'task_run_benchmark_support_escalate_train',
			taskTemplate: 'escalate critical support ticket',
			summary: 'Escalated a critical support ticket to the incident queue.',
			originalPath: ['support_tickets', 'support_detail', 'support_escalation'],
			optimizedPath: ['support_tickets', 'support_detail', 'support_escalation'],
			steps: [
				['fill', 'Ticket ID filter', '{{ticket_id}}', 'Filter the ticket list.', 'Tickets'],
				['click', 'Ticket row', '', 'Open the ticket detail page.', 'Tickets'],
				['select', 'Priority', 'Critical', 'Set ticket priority to Critical.', 'Ticket Detail'],
				['click', 'Escalate Ticket', '', 'Click Escalate Ticket.', 'Ticket Escalation'],
				[
					'select',
					'Incident queue',
					'Primary incident queue',
					'Choose the incident queue.',
					'Ticket Escalation',
				],
				['click', 'Confirm Escalation', '', 'Confirm ticket escalation.', 'Ticket Escalation'],
			],
		},
		positiveCase: {
			task: 'escalate critical support ticket',
			url: 'https://support.example.com/tickets/T-100/escalation',
			title: 'Ticket Escalation',
			visibleText: [
				'Ticket Escalation',
				'Priority',
				'Critical',
				'Escalate Ticket',
				'Confirm Escalation',
			],
			controls: [
				{ role: 'button', name: 'Escalate Ticket' },
				{ role: 'button', name: 'Confirm Escalation' },
			],
			baselineActionCount: 8,
		},
		negativeCases: [
			{
				id: 'merge_same_page',
				name: 'Support merge ticket on escalation page',
				task: 'merge duplicate support ticket',
				url: 'https://support.example.com/tickets/T-100/escalation',
				title: 'Ticket Escalation',
				visibleText: ['Ticket Escalation', 'Escalate Ticket', 'Confirm Escalation'],
				controls: [{ role: 'button', name: 'Escalate Ticket' }],
				baselineActionCount: 6,
			},
		],
	},
	{
		id: 'deploy_rollback',
		site: 'deploy.example.com',
		module: 'releases',
		title: 'Release rollback manual',
		manualLines: [
			'# Release rollback',
			'Open Releases, search the service, and open the latest release detail page.',
			'Click Rollback Release, choose the previous stable version, then confirm rollback.',
			'Do not use this procedure for promoting or deleting releases.',
		],
		taskRun: {
			id: 'task_run_benchmark_deploy_rollback_train',
			taskTemplate: 'rollback service release to previous stable version',
			summary: 'Rolled back a service release to the previous stable version.',
			originalPath: ['deploy_releases', 'deploy_detail', 'deploy_rollback'],
			optimizedPath: ['deploy_releases', 'deploy_detail', 'deploy_rollback'],
			steps: [
				['fill', 'Service filter', '{{service_name}}', 'Filter the release list.', 'Releases'],
				['click', 'Release row', '', 'Open the release detail page.', 'Releases'],
				['click', 'Rollback Release', '', 'Click Rollback Release.', 'Release Detail'],
				[
					'select',
					'Previous stable version',
					'{{version}}',
					'Choose the previous stable version.',
					'Rollback Release',
				],
				['click', 'Confirm Rollback', '', 'Confirm release rollback.', 'Rollback Release'],
			],
		},
		positiveCase: {
			task: 'rollback service release to previous stable version',
			url: 'https://deploy.example.com/releases/api/rollback',
			title: 'Rollback Release',
			visibleText: ['Rollback Release', 'Previous stable version', 'Confirm Rollback'],
			controls: [
				{ role: 'combobox', name: 'Previous stable version' },
				{ role: 'button', name: 'Confirm Rollback' },
			],
			baselineActionCount: 8,
		},
		negativeCases: [
			{
				id: 'promote_same_page',
				name: 'Release promote task on rollback page',
				task: 'promote service release to production',
				url: 'https://deploy.example.com/releases/api/rollback',
				title: 'Rollback Release',
				visibleText: ['Rollback Release', 'Confirm Rollback'],
				controls: [{ role: 'button', name: 'Confirm Rollback' }],
				baselineActionCount: 6,
			},
		],
	},
	{
		id: 'analytics_export',
		site: 'analytics.example.com',
		module: 'reports',
		title: 'Analytics report export manual',
		manualLines: [
			'# Analytics report export',
			'Open Reports, choose a report, set the date range, click Export CSV, then confirm export.',
			'Do not use this procedure for deleting or archiving reports.',
		],
		taskRun: {
			id: 'task_run_benchmark_analytics_export_train',
			taskTemplate: 'export monthly analytics report as csv',
			summary: 'Exported a monthly analytics report as CSV.',
			originalPath: ['analytics_reports', 'analytics_report_detail', 'analytics_export'],
			optimizedPath: ['analytics_reports', 'analytics_report_detail', 'analytics_export'],
			steps: [
				['click', 'Report row', '', 'Open the report detail page.', 'Reports'],
				['select', 'Date range', 'Last month', 'Select the date range.', 'Report Detail'],
				['click', 'Export CSV', '', 'Click Export CSV.', 'Report Export'],
				['click', 'Confirm Export', '', 'Confirm report export.', 'Report Export'],
			],
		},
		positiveCase: {
			task: 'export monthly analytics report as csv',
			url: 'https://analytics.example.com/reports/sales/export',
			title: 'Report Export',
			visibleText: ['Report Export', 'Date range', 'Export CSV', 'Confirm Export'],
			controls: [
				{ role: 'button', name: 'Export CSV' },
				{ role: 'button', name: 'Confirm Export' },
			],
			baselineActionCount: 6,
		},
		negativeCases: [
			{
				id: 'delete_same_page',
				name: 'Analytics delete report on export page',
				task: 'delete analytics report',
				url: 'https://analytics.example.com/reports/sales/export',
				title: 'Report Export',
				visibleText: ['Report Export', 'Export CSV', 'Confirm Export'],
				controls: [{ role: 'button', name: 'Export CSV' }],
				baselineActionCount: 5,
			},
		],
	},
]

await rm(artifactDir, { recursive: true, force: true })
await mkdir(artifactDir, { recursive: true })
await mkdir(dataDir, { recursive: true })

const backend = spawn('go', ['run', './cmd/server'], {
	cwd: path.join(rootDir, 'apps/workflow-backend'),
	env: {
		...process.env,
		WORKFLOW_BACKEND_ADDR: backendAddr,
		WORKFLOW_STORAGE_BACKEND: 'file',
		WORKFLOW_DATA_DIR: dataDir,
		LLM_BASE_URL: process.env.LLM_BASE_URL ?? 'https://www.aicodexcn.com/v1',
		LLM_API_KEY: process.env.LLM_API_KEY ?? '',
		LLM_MODEL: process.env.LLM_MODEL ?? 'gpt-5.4',
		NO_PROXY: `127.0.0.1,localhost,${process.env.NO_PROXY ?? ''}`,
		no_proxy: `127.0.0.1,localhost,${process.env.no_proxy ?? ''}`,
	},
	detached: true,
	stdio: ['ignore', 'pipe', 'pipe'],
})
const backendLogs = []
backend.stdout.on('data', (chunk) => backendLogs.push(chunk.toString()))
backend.stderr.on('data', (chunk) => backendLogs.push(chunk.toString()))

const fixture = await createDashboardServer()
const fixtureURL = `http://127.0.0.1:${fixture.port}/`
let browser

try {
	await waitForReady(`${backendURL}/ready`, 'backend')
	browser = await launchChrome()
	const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } })
	await page.goto(fixtureURL)
	await page.screenshot({ path: path.join(artifactDir, '00-dashboard-empty.png'), fullPage: true })

	const seed = await seedTrainingData()
	const report = await runBenchmark(seed)
	report.backendURL = backendURL
	report.fixtureURL = fixtureURL
	report.llm = {
		baseURL: process.env.LLM_BASE_URL ?? 'https://www.aicodexcn.com/v1',
		model: process.env.LLM_MODEL ?? 'gpt-5.4',
		apiKey: process.env.LLM_API_KEY ? 'configured-redacted' : 'not-set',
	}
	report.artifacts = {
		screenshots: ['00-dashboard-empty.png', '01-benchmark-results.png'],
		reportJSON: 'benchmark-report.json',
		reportMarkdown: 'benchmark-report.md',
		backendData: 'backend-data-snapshot.json',
		contexts: report.cases.map((item) => `context-${item.id}.json`),
		prompts: report.cases.map((item) => `prompt-${item.id}.txt`),
	}

	await dumpContexts(report)
	await dumpBackendData()
	await writeJSON('benchmark-report.json', report)
	await writeFile(path.join(artifactDir, 'benchmark-report.md'), markdownReport(report))

	await page.evaluate((value) => {
		window.renderBenchmark(value)
	}, report)
	await page.screenshot({
		path: path.join(artifactDir, '01-benchmark-results.png'),
		fullPage: true,
	})
} finally {
	if (browser) await browser.close()
	await fixture.close()
	try {
		process.kill(-backend.pid, 'SIGTERM')
	} catch {
		backend.kill('SIGTERM')
	}
	await writeFile(path.join(artifactDir, 'backend.log'), backendLogs.join(''))
}

async function seedTrainingData() {
	const manuals = []
	manuals.push(
		await postJSON('/api/memory/site-manuals/import', {
			projectId: 'benchmark',
			site: 'middleware.example.com',
			module: 'instances',
			title: 'Middleware instance backup restore manual',
			sourceType: 'markdown',
			content: [
				'# Middleware instance backup restore',
				'Open the instance list and search by instance name.',
				'Click the instance name to enter the detail page.',
				'Open Data Backup, switch to the Full Backup tab, and click Data Restore.',
				'Choose the latest backup record, select the node IP, then click Start Restore.',
				'If the page does not show Data Backup, Full Backup, or Data Restore, abandon this manual hint.',
			].join('\n'),
		})
	)
	manuals.push(
		await postJSON('/api/memory/site-manuals/import', {
			projectId: 'benchmark',
			site: 'iam.example.com',
			module: 'users',
			title: 'IAM user credential reset manual',
			sourceType: 'markdown',
			content: [
				'# IAM user credential reset',
				'Open Users, search by username, and click the user row.',
				'Open the Security tab, click Reset Credential, choose temporary credential, then confirm.',
				'If the current page is not a user detail security page, abandon this manual hint.',
			].join('\n'),
		})
	)
	for (const scenario of loadTestScenarios) {
		manuals.push(await postJSON('/api/memory/site-manuals/import', scenarioManualRequest(scenario)))
	}

	const taskRuns = []
	taskRuns.push(
		await postJSON('/api/memory/task-runs', {
			id: 'task_run_benchmark_restore_train',
			projectId: 'benchmark',
			site: 'middleware.example.com',
			taskTemplate: 'restore middleware instance from latest full backup',
			summary: 'Restored a middleware instance from the latest full backup.',
			status: 'success',
			originalPath: [
				'mw_instances',
				'mw_monitoring_noise',
				'mw_instances',
				'mw_detail',
				'mw_backups',
			],
			optimizedPath: ['mw_instances', 'mw_detail', 'mw_backups'],
			actionSteps: [
				step(
					'restore_01',
					1,
					'mw_instances',
					'fill',
					'Instance name filter',
					'{{instance_name}}',
					'Filter the instance list.'
				),
				step(
					'restore_noise_01',
					2,
					'mw_instances',
					'click',
					'Monitoring',
					'',
					'Opened the wrong monitoring page.',
					true
				),
				step(
					'restore_noise_02',
					3,
					'mw_monitoring_noise',
					'click',
					'Back to instances',
					'',
					'Returned to the instance list.',
					true
				),
				step(
					'restore_02',
					4,
					'mw_instances',
					'click',
					'Instance name',
					'',
					'Open the instance detail page.'
				),
				step(
					'restore_03',
					5,
					'mw_detail',
					'click',
					'Data Backup',
					'',
					'Open the Data Backup page.'
				),
				step(
					'restore_04',
					6,
					'mw_backups',
					'click',
					'Full Backup',
					'',
					'Switch to the Full Backup tab.'
				),
				step('restore_05', 7, 'mw_backups', 'click', 'Data Restore', '', 'Click Data Restore.'),
				step(
					'restore_06',
					8,
					'mw_backups',
					'click',
					'Latest backup record',
					'',
					'Choose the latest backup record.'
				),
				step(
					'restore_07',
					9,
					'mw_backups',
					'select',
					'Node IP',
					'{{node_ip}}',
					'Select the node IP.'
				),
				step('restore_08', 10, 'mw_backups', 'click', 'Start Restore', '', 'Click Start Restore.'),
			],
		})
	)
	taskRuns.push(
		await postJSON('/api/memory/task-runs', {
			id: 'task_run_benchmark_credential_train',
			projectId: 'benchmark',
			site: 'iam.example.com',
			taskTemplate: 'reset IAM user credential',
			summary: 'Reset a user credential from the IAM user security page.',
			status: 'success',
			originalPath: [
				'iam_users',
				'iam_audit_noise',
				'iam_users',
				'iam_user_detail',
				'iam_security',
			],
			optimizedPath: ['iam_users', 'iam_user_detail', 'iam_security'],
			actionSteps: [
				step(
					'credential_01',
					1,
					'iam_users',
					'fill',
					'Username filter',
					'{{username}}',
					'Filter the user list.'
				),
				step(
					'credential_noise_01',
					2,
					'iam_users',
					'click',
					'Audit Log',
					'',
					'Opened the audit log by mistake.',
					true
				),
				step(
					'credential_noise_02',
					3,
					'iam_audit_noise',
					'click',
					'Back to users',
					'',
					'Returned to the user list.',
					true
				),
				step(
					'credential_02',
					4,
					'iam_users',
					'click',
					'User row',
					'',
					'Open the user detail page.'
				),
				step(
					'credential_03',
					5,
					'iam_user_detail',
					'click',
					'Security',
					'',
					'Open the Security tab.'
				),
				step(
					'credential_04',
					6,
					'iam_security',
					'click',
					'Reset Credential',
					'',
					'Click Reset Credential.'
				),
				step(
					'credential_05',
					7,
					'iam_security',
					'select',
					'Credential mode',
					'Temporary credential',
					'Choose temporary credential mode.'
				),
				step(
					'credential_06',
					8,
					'iam_security',
					'click',
					'Confirm Reset',
					'',
					'Confirm credential reset.'
				),
			],
		})
	)
	for (const scenario of loadTestScenarios) {
		taskRuns.push(await postJSON('/api/memory/task-runs', scenarioTaskRunRequest(scenario)))
	}

	return { manuals, taskRuns }
}

function scenarioManualRequest(scenario) {
	return {
		projectId: 'benchmark',
		site: scenario.site,
		module: scenario.module,
		title: scenario.title,
		sourceType: 'markdown',
		content: scenario.manualLines.join('\n'),
	}
}

function scenarioTaskRunRequest(scenario) {
	const run = scenario.taskRun
	return {
		id: run.id,
		projectId: 'benchmark',
		site: scenario.site,
		taskTemplate: run.taskTemplate,
		summary: run.summary,
		status: 'success',
		originalPath: run.originalPath,
		optimizedPath: run.optimizedPath,
		actionSteps: run.steps.map((item, index) => scenarioStep(scenario, item, index)),
	}
}

function scenarioStep(scenario, item, index) {
	const [actionType, targetName, valueTemplate, resultSummary, pageTitle] = item
	const pageStateId =
		scenario.taskRun.optimizedPath[Math.min(index, scenario.taskRun.optimizedPath.length - 1)]
	return {
		id: `${scenario.id}_${String(index + 1).padStart(2, '0')}`,
		stepIndex: index + 1,
		pageStateId,
		actionType,
		targetName,
		...(valueTemplate ? { valueTemplate } : {}),
		resultSummary,
		beforeObservation: scenarioObservation(scenario, pageStateId, pageTitle, targetName),
	}
}

function scenarioObservation(scenario, pageStateId, title, targetName) {
	const pathKey = pageStateId.replace(/^[^_]+_/, '').replaceAll('_', '/')
	const url = `https://${scenario.site}/${scenario.module}/${pathKey}`
	const controls = scenarioControls(title, targetName)
	return {
		projectId: 'benchmark',
		site: scenario.site,
		url,
		urlPattern: url,
		title,
		visibleTextSample: [title, targetName, ...controls.map((control) => control.name)].join(' '),
		controlSignatures: controls,
		activeTabs: controls.filter((control) => control.role === 'tab').map((control) => control.name),
	}
}

function scenarioControls(title, targetName) {
	const controls = [{ role: controlRoleForTarget(targetName), name: targetName }]
	if (/operations/i.test(title)) {
		controls.push({ role: 'tab', name: 'Operations' }, { role: 'button', name: 'Confirm Restart' })
	}
	if (/payments/i.test(title)) {
		controls.push({ role: 'tab', name: 'Payments' }, { role: 'button', name: 'Confirm Refund' })
	}
	if (/escalation/i.test(title)) {
		controls.push({ role: 'button', name: 'Confirm Escalation' })
	}
	if (/rollback/i.test(title)) {
		controls.push({ role: 'button', name: 'Confirm Rollback' })
	}
	if (/export/i.test(title)) {
		controls.push({ role: 'button', name: 'Confirm Export' })
	}
	return uniqueControls(controls)
}

function uniqueControls(controls) {
	const seen = new Set()
	const result = []
	for (const control of controls) {
		const key = `${control.role}:${control.name}`
		if (seen.has(key)) continue
		seen.add(key)
		result.push(control)
	}
	return result
}

function step(
	id,
	stepIndex,
	pageStateId,
	actionType,
	targetName,
	valueTemplate,
	resultSummary,
	isBranchNoise = false
) {
	return {
		id,
		stepIndex,
		pageStateId,
		actionType,
		targetName,
		...(valueTemplate ? { valueTemplate } : {}),
		resultSummary,
		isBranchNoise,
		beforeObservation: observationForStep(pageStateId, targetName),
	}
}

function observationForStep(pageStateId, targetName) {
	const common = {
		projectId: 'benchmark',
		url: 'https://example.invalid/',
		urlPattern: 'https://example.invalid/',
		visibleTextSample: targetName,
		controlSignatures: [{ role: controlRoleForTarget(targetName), name: targetName }],
	}
	if (pageStateId.startsWith('mw_')) {
		const urlPattern =
			pageStateId === 'mw_backups'
				? 'https://middleware.example.com/instances/pg-prod/backups'
				: pageStateId === 'mw_detail'
					? 'https://middleware.example.com/instances/pg-prod'
					: 'https://middleware.example.com/instances'
		return {
			...common,
			projectId: 'benchmark',
			site: 'middleware.example.com',
			url: urlPattern,
			urlPattern,
			title:
				pageStateId === 'mw_backups'
					? 'Data Backup'
					: pageStateId === 'mw_detail'
						? 'Instance Detail'
						: 'Instances',
			activeTabs: pageStateId === 'mw_backups' ? ['Full Backup'] : [],
			visibleTextSample:
				pageStateId === 'mw_backups'
					? 'Data Backup Full Backup Latest backup Data Restore Start Restore'
					: `${targetName} Data Backup`,
			controlSignatures:
				pageStateId === 'mw_backups'
					? [
							{ role: 'tab', name: 'Full Backup' },
							{ role: 'button', name: 'Data Restore' },
							{ role: 'button', name: 'Start Restore' },
						]
					: [{ role: controlRoleForTarget(targetName), name: targetName }],
		}
	}
	if (pageStateId.startsWith('iam_')) {
		const urlPattern =
			pageStateId === 'iam_security'
				? 'https://iam.example.com/users/alice/security'
				: pageStateId === 'iam_user_detail'
					? 'https://iam.example.com/users/alice'
					: 'https://iam.example.com/users'
		return {
			...common,
			projectId: 'benchmark',
			site: 'iam.example.com',
			url: urlPattern,
			urlPattern,
			title:
				pageStateId === 'iam_security'
					? 'User Security'
					: pageStateId === 'iam_user_detail'
						? 'User Detail'
						: 'Users',
			activeTabs: pageStateId === 'iam_security' ? ['Security'] : [],
			visibleTextSample:
				pageStateId === 'iam_security'
					? 'Users Security Reset Credential Temporary credential Confirm Reset'
					: `${targetName} Security`,
			controlSignatures:
				pageStateId === 'iam_security'
					? [
							{ role: 'tab', name: 'Security' },
							{ role: 'button', name: 'Reset Credential' },
							{ role: 'button', name: 'Confirm Reset' },
						]
					: [{ role: controlRoleForTarget(targetName), name: targetName }],
		}
	}
	return common
}

function controlRoleForTarget(targetName) {
	if (/tab|backup|security/i.test(targetName)) return 'tab'
	if (/filter|input|name|username/i.test(targetName)) return 'textbox'
	return 'button'
}

async function runBenchmark(seed) {
	const cases = [
		{
			id: 'mw_restore_positive',
			name: 'Middleware restore, same site and same task',
			expectedHit: true,
			expectedTerms: ['restore', 'backup'],
			baselineActionCount: 10,
			baselineInvalidClicks: 2,
			contextRequest: contextRequest({
				task: '让 pg-prod 实例基于最新全量备份恢复',
				url: 'https://middleware.example.com/instances/pg-prod/backups',
				module: 'instances',
				title: 'Data Backup',
				visibleText: [
					'Data Backup',
					'Full Backup',
					'Latest backup',
					'Data Restore',
					'Start Restore',
				],
				controls: [
					{ role: 'tab', name: 'Full Backup' },
					{ role: 'button', name: 'Data Restore' },
					{ role: 'button', name: 'Start Restore' },
				],
			}),
		},
		{
			id: 'iam_credential_positive',
			name: 'IAM reset credential, same site and same task',
			expectedHit: true,
			expectedTerms: ['reset', 'credential'],
			baselineActionCount: 8,
			baselineInvalidClicks: 2,
			contextRequest: contextRequest({
				task: '重置 alice 用户密码',
				url: 'https://iam.example.com/users/alice/security',
				module: 'users',
				title: 'User Security',
				visibleText: [
					'Users',
					'Security',
					'Reset Credential',
					'Temporary credential',
					'Confirm Reset',
				],
				controls: [
					{ role: 'tab', name: 'Security' },
					{ role: 'button', name: 'Reset Credential' },
					{ role: 'button', name: 'Confirm Reset' },
				],
			}),
		},
		{
			id: 'cross_site_negative',
			name: 'Same restore wording on another site',
			expectedHit: false,
			expectedTerms: [],
			baselineActionCount: 10,
			baselineInvalidClicks: 0,
			contextRequest: contextRequest({
				task: '让 pg-prod 实例基于最新全量备份恢复',
				url: 'https://finance.example.com/instances/pg-prod/backups',
				module: 'instances',
				title: 'Data Backup',
				visibleText: ['Data Backup', 'Full Backup', 'Data Restore'],
				controls: [{ role: 'button', name: 'Data Restore' }],
			}),
		},
		{
			id: 'wrong_page_negative',
			name: 'Restore task from settings page',
			expectedHit: false,
			expectedTerms: [],
			baselineActionCount: 10,
			baselineInvalidClicks: 0,
			contextRequest: contextRequest({
				task: '让 pg-prod 实例基于最新全量备份恢复',
				url: 'https://middleware.example.com/settings/audit',
				module: 'settings',
				title: 'Audit Settings',
				visibleText: ['Audit Settings', 'Retention days', 'Save'],
				controls: [
					{ role: 'textbox', name: 'Retention days' },
					{ role: 'button', name: 'Save' },
				],
			}),
		},
		{
			id: 'wrong_intent_negative',
			name: 'Same backup page but destructive delete task',
			expectedHit: false,
			expectedTerms: [],
			baselineActionCount: 7,
			baselineInvalidClicks: 0,
			contextRequest: contextRequest({
				task: '删除 pg-prod 实例并释放资源',
				url: 'https://middleware.example.com/instances/pg-prod/backups',
				module: 'instances',
				title: 'Data Backup',
				visibleText: ['Data Backup', 'Full Backup', 'Data Restore', 'Start Restore'],
				controls: [
					{ role: 'tab', name: 'Full Backup' },
					{ role: 'button', name: 'Data Restore' },
				],
			}),
		},
		...scenarioBenchmarkCases(),
	]

	const results = []
	for (const testCase of cases) {
		const started = performance.now()
		const context = await postJSON('/api/memory/context', testCase.contextRequest)
		const contextLatencyMs = Math.round(performance.now() - started)
		const evaluation = evaluateCase(testCase, context, contextLatencyMs)
		results.push(evaluation)
	}
	const metrics = aggregateMetrics(results)
	return {
		generatedAt: new Date().toISOString(),
		projectId: 'benchmark',
		seed,
		dataset: {
			manualSources: seed.manuals.length,
			trainingTaskRuns: seed.taskRuns.length,
			scenarioCount: loadTestScenarios.length + 2,
			caseCount: results.length,
			positiveCases: results.filter((item) => item.expectedHit).length,
			negativeCases: results.filter((item) => !item.expectedHit).length,
		},
		metricDefinitions: {
			retrievalHitRate: 'TP / expected-positive cases',
			precision: 'TP / all actual memory-hit cases',
			accuracy: '(TP + TN) / all benchmark cases',
			misleadingRate: 'FP / all actual memory-hit cases',
			actionReductionPct: '(baseline actions - guided actions) / baseline actions',
		},
		metrics,
		cases: results,
		findings: benchmarkFindings(metrics, results),
	}
}

function scenarioBenchmarkCases() {
	return loadTestScenarios.flatMap((scenario) => {
		const positive = {
			id: `${scenario.id}_positive`,
			name: `${scenario.title.replace(/ manual$/i, '')}, same site and same task`,
			expectedHit: true,
			expectedTerms: expectedTermsFromTask(scenario.taskRun.taskTemplate),
			baselineActionCount: scenario.positiveCase.baselineActionCount,
			baselineInvalidClicks: 1,
			contextRequest: contextRequest({
				task: scenario.positiveCase.task,
				url: scenario.positiveCase.url,
				module: scenario.module,
				title: scenario.positiveCase.title,
				visibleText: scenario.positiveCase.visibleText,
				controls: scenario.positiveCase.controls,
			}),
		}
		const negatives = scenario.negativeCases.map((negative) => ({
			id: `${scenario.id}_${negative.id}`,
			name: negative.name,
			expectedHit: false,
			expectedTerms: [],
			baselineActionCount: negative.baselineActionCount,
			baselineInvalidClicks: 0,
			contextRequest: contextRequest({
				task: negative.task,
				url: negative.url,
				module: scenario.module,
				title: negative.title,
				visibleText: negative.visibleText,
				controls: negative.controls,
			}),
		}))
		return [positive, ...negatives]
	})
}

function expectedTermsFromTask(task) {
	return task
		.split(/\s+/)
		.map((term) => term.trim().toLowerCase())
		.filter((term) => term.length >= 4)
		.slice(0, 2)
}

function contextRequest({ task, url, module, title, visibleText, controls }) {
	return {
		projectId: 'benchmark',
		task,
		currentUrl: url,
		pageObservation: {
			title,
			visibleText,
			controls,
			activeTabs: controls
				.filter((control) => control.role === 'tab')
				.map((control) => control.name),
		},
		metadata: { module },
	}
}

function evaluateCase(testCase, context, contextLatencyMs) {
	const guideHits = context.siteTaskGuides ?? []
	const manualHits = context.siteManualKnowledge ?? []
	const actualHit = guideHits.length > 0 || manualHits.length > 0
	const expectedHit = testCase.expectedHit
	const expectedRelevant =
		testCase.expectedTerms.length === 0 || memoryContainsTerms(context, testCase.expectedTerms)
	const truePositive = expectedHit && actualHit && expectedRelevant
	const falseNegative = expectedHit && !actualHit
	const falsePositive = !expectedHit && actualHit
	const trueNegative = !expectedHit && !actualHit
	const correct = truePositive || trueNegative
	const guidedActionCount =
		truePositive && guideHits[0]?.steps?.length
			? guideHits[0].steps.length
			: testCase.baselineActionCount
	const actionReductionPct =
		testCase.baselineActionCount > 0
			? round2((testCase.baselineActionCount - guidedActionCount) / testCase.baselineActionCount)
			: 0
	const baselineEstimatedSeconds = estimatedTaskSeconds(
		testCase.baselineActionCount,
		testCase.baselineInvalidClicks
	)
	const guidedEstimatedSeconds = estimatedTaskSeconds(guidedActionCount, falsePositive ? 1 : 0)
	return {
		id: testCase.id,
		name: testCase.name,
		expectedHit,
		actualHit,
		correct,
		truePositive,
		falsePositive,
		trueNegative,
		falseNegative,
		misleading: falsePositive,
		contextLatencyMs,
		baselineActionCount: testCase.baselineActionCount,
		guidedActionCount,
		actionReductionPct,
		baselineEstimatedSeconds,
		guidedEstimatedSeconds,
		estimatedSecondsSaved: round2(baselineEstimatedSeconds - guidedEstimatedSeconds),
		expectedTerms: testCase.expectedTerms,
		guideHitCount: guideHits.length,
		manualHitCount: manualHits.length,
		topGuide: guideHits[0]
			? {
					id: guideHits[0].id,
					whenToUse: guideHits[0].whenToUse,
					steps: guideHits[0].steps,
					confidence: guideHits[0].confidence,
				}
			: null,
		manualSummaries: manualHits.map((item) => ({
			id: item.id,
			title: item.title,
			summary: item.summary,
			confidence: item.confidence,
		})),
		filteredEvidence: context.debug?.filteredEvidence ?? [],
		context,
		promptSample: context.contextPrompt ?? '',
	}
}

function memoryContainsTerms(context, terms) {
	const text = JSON.stringify({
		guides: context.siteTaskGuides ?? [],
		manuals: context.siteManualKnowledge ?? [],
	}).toLowerCase()
	return terms.every((term) => text.includes(term.toLowerCase()))
}

function estimatedTaskSeconds(actionCount, invalidClickCount) {
	return round2(actionCount * 1.8 + invalidClickCount * 2.5)
}

function aggregateMetrics(results) {
	const total = results.length
	const tp = results.filter((item) => item.truePositive).length
	const fp = results.filter((item) => item.falsePositive).length
	const tn = results.filter((item) => item.trueNegative).length
	const fn = results.filter((item) => item.falseNegative).length
	const actualHits = results.filter((item) => item.actualHit).length
	const expectedPositive = results.filter((item) => item.expectedHit).length
	const actionCases = results.filter((item) => item.truePositive)
	const averageActionReductionPct =
		actionCases.length > 0
			? round2(
					actionCases.reduce((sum, item) => sum + item.actionReductionPct, 0) / actionCases.length
				)
			: 0
	const averageSecondsSaved =
		actionCases.length > 0
			? round2(
					actionCases.reduce((sum, item) => sum + item.estimatedSecondsSaved, 0) /
						actionCases.length
				)
			: 0
	return {
		totalCases: total,
		truePositive: tp,
		falsePositive: fp,
		trueNegative: tn,
		falseNegative: fn,
		actualHitCases: actualHits,
		expectedPositiveCases: expectedPositive,
		retrievalHitRate: expectedPositive > 0 ? round2(tp / expectedPositive) : 0,
		precision: actualHits > 0 ? round2(tp / actualHits) : 0,
		accuracy: total > 0 ? round2((tp + tn) / total) : 0,
		misleadingRate: actualHits > 0 ? round2(fp / actualHits) : 0,
		averageActionReductionPct,
		averageSecondsSaved,
	}
}

function benchmarkFindings(metrics, results) {
	const findings = []
	if (metrics.retrievalHitRate === 1) {
		findings.push('Both expected positive tasks recalled reusable memory.')
	}
	if (metrics.falsePositive > 0) {
		findings.push(
			'False positives were observed for same-site negative cases; the guide created from task runs has weak page/task gating when no page-state guards exist.'
		)
	}
	for (const result of results) {
		if (result.truePositive) {
			findings.push(
				`${result.name}: guided action count ${result.guidedActionCount}/${result.baselineActionCount}, estimated ${Math.round(
					result.actionReductionPct * 100
				)}% fewer actions.`
			)
		}
		if (result.falsePositive) {
			findings.push(`${result.name}: misleading memory was injected.`)
		}
	}
	return findings
}

async function dumpContexts(report) {
	for (const item of report.cases) {
		await writeJSON(`context-${item.id}.json`, item.context)
		await writeFile(path.join(artifactDir, `prompt-${item.id}.txt`), item.promptSample)
		delete item.context
		delete item.promptSample
	}
}

async function dumpBackendData() {
	const snapshot = {}
	for (const name of await readdir(dataDir)) {
		if (!name.endsWith('.json')) continue
		const raw = await readFile(path.join(dataDir, name), 'utf8')
		snapshot[name] = JSON.parse(raw || '{}')
	}
	await writeJSON('backend-data-snapshot.json', snapshot)
}

async function createDashboardServer() {
	const html = `<!doctype html>
<html>
<head>
  <meta charset="utf-8" />
  <title>Site Task Guide Benchmark</title>
  <style>
    body { margin: 0; font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; color: #17201d; background: #f6f7f4; }
    main { padding: 24px; display: grid; gap: 18px; }
    h1 { margin: 0; font-size: 24px; }
    h2 { margin: 0 0 10px; font-size: 16px; }
    .metrics { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 10px; }
    .metric, section { border: 1px solid #d4dad1; border-radius: 8px; background: #fff; padding: 14px; }
    .metric strong { display: block; font-size: 24px; margin-top: 4px; }
    table { width: 100%; border-collapse: collapse; font-size: 13px; background: #fff; }
    th, td { border-bottom: 1px solid #e0e4dd; padding: 10px; text-align: left; vertical-align: top; }
    th { color: #52605a; font-weight: 600; }
    .ok { color: #16704f; font-weight: 700; }
    .bad { color: #b33a2e; font-weight: 700; }
    pre { white-space: pre-wrap; overflow: auto; max-height: 260px; background: #f8faf8; border: 1px solid #d4dad1; border-radius: 6px; padding: 10px; }
  </style>
</head>
<body>
<main>
  <header>
    <h1>Site Task Guide Memory Benchmark</h1>
    <div id="subtitle">Waiting for benchmark results...</div>
    <div id="dataset"></div>
  </header>
  <div class="metrics" id="metrics"></div>
  <section>
    <h2>Cases</h2>
    <table>
      <thead><tr><th>Case</th><th>Expected</th><th>Actual</th><th>Result</th><th>Efficiency</th><th>Top Guide</th></tr></thead>
      <tbody id="cases"></tbody>
    </table>
  </section>
  <section>
    <h2>Findings</h2>
    <pre id="findings"></pre>
  </section>
</main>
<script>
window.renderBenchmark = (report) => {
  document.querySelector('#subtitle').textContent = 'Generated at ' + report.generatedAt + ' | backend ' + report.backendURL;
  document.querySelector('#dataset').textContent = 'Dataset: ' + report.dataset.manualSources + ' manuals, ' + report.dataset.trainingTaskRuns + ' training task runs, ' + report.dataset.caseCount + ' cases (' + report.dataset.positiveCases + ' positive, ' + report.dataset.negativeCases + ' negative)';
  const metrics = report.metrics;
  document.querySelector('#metrics').innerHTML = [
    ['Hit rate', metrics.retrievalHitRate],
    ['Precision', metrics.precision],
    ['Accuracy', metrics.accuracy],
    ['Misleading', metrics.misleadingRate],
    ['Avg action cut', metrics.averageActionReductionPct],
  ].map(([label, value]) => '<div class="metric">' + label + '<strong>' + Math.round(value * 100) + '%</strong></div>').join('');
  document.querySelector('#cases').innerHTML = report.cases.map((item) => {
    const resultClass = item.correct ? 'ok' : 'bad';
    const topGuide = item.topGuide ? item.topGuide.whenToUse + '<br/><small>' + item.topGuide.id + '</small>' : 'none';
    return '<tr>' +
      '<td>' + item.name + '</td>' +
      '<td>' + (item.expectedHit ? 'memory expected' : 'no memory expected') + '</td>' +
      '<td>guide ' + item.guideHitCount + ', manual ' + item.manualHitCount + '</td>' +
      '<td class="' + resultClass + '">' + (item.correct ? 'correct' : 'wrong / misleading') + '</td>' +
      '<td>' + item.guidedActionCount + '/' + item.baselineActionCount + ' actions<br/>' + Math.round(item.actionReductionPct * 100) + '% fewer</td>' +
      '<td>' + topGuide + '</td>' +
      '</tr>';
  }).join('');
  document.querySelector('#findings').textContent = report.findings.join('\\n');
}
</script>
</body>
</html>`
	const server = createServer((req, res) => {
		if (req.url === '/' || req.url === '/index.html') {
			res.writeHead(200, { 'content-type': 'text/html; charset=utf-8' })
			res.end(html)
			return
		}
		res.writeHead(404)
		res.end('not found')
	})
	await new Promise((resolve, reject) => {
		server.once('error', reject)
		server.listen(0, '127.0.0.1', resolve)
	})
	return {
		port: server.address().port,
		close: () => new Promise((resolve) => server.close(resolve)),
	}
}

async function launchChrome() {
	try {
		return await chromium.launch({ channel: 'chrome', headless: true })
	} catch {
		return chromium.launch({ headless: true })
	}
}

async function waitForReady(url, label) {
	const started = Date.now()
	while (Date.now() - started < 45_000) {
		try {
			const response = await fetch(url)
			if (response.ok) return
		} catch {}
		await new Promise((resolve) => setTimeout(resolve, 500))
	}
	throw new Error(`${label} did not become ready at ${url}`)
}

async function postJSON(pathname, body) {
	const response = await fetch(backendURL + pathname, {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify(body),
	})
	if (!response.ok)
		throw new Error(`${pathname} failed: ${response.status} ${await response.text()}`)
	return response.json()
}

async function writeJSON(name, value) {
	await writeFile(path.join(artifactDir, name), JSON.stringify(value, null, 2))
}

function markdownReport(report) {
	const rows = report.cases
		.map((item) =>
			[
				item.name,
				item.expectedHit ? 'yes' : 'no',
				`${item.guideHitCount}/${item.manualHitCount}`,
				item.correct ? 'correct' : 'wrong',
				item.misleading ? 'yes' : 'no',
				`${item.guidedActionCount}/${item.baselineActionCount}`,
				`${Math.round(item.actionReductionPct * 100)}%`,
			].join(' | ')
		)
		.join('\n')
	return [
		'# Site Task Guide Memory Benchmark',
		'',
		`Generated at: ${report.generatedAt}`,
		`Backend: ${report.backendURL}`,
		`LLM: ${report.llm.baseURL} / ${report.llm.model} / ${report.llm.apiKey}`,
		'',
		'## Dataset',
		'',
		`- Manual sources: ${report.dataset.manualSources}`,
		`- Training task runs: ${report.dataset.trainingTaskRuns}`,
		`- Scenario count: ${report.dataset.scenarioCount}`,
		`- Benchmark cases: ${report.dataset.caseCount}`,
		`- Positive cases: ${report.dataset.positiveCases}`,
		`- Negative cases: ${report.dataset.negativeCases}`,
		'',
		'## Metrics',
		'',
		`- Retrieval hit rate: ${percent(report.metrics.retrievalHitRate)}`,
		`- Precision: ${percent(report.metrics.precision)}`,
		`- Accuracy: ${percent(report.metrics.accuracy)}`,
		`- Misleading rate among hits: ${percent(report.metrics.misleadingRate)}`,
		`- Average positive-case action reduction: ${percent(report.metrics.averageActionReductionPct)}`,
		`- Average estimated seconds saved: ${report.metrics.averageSecondsSaved}s`,
		'',
		'## Cases',
		'',
		'Case | Expected hit | Guide/manual hits | Result | Misleading | Guided/baseline actions | Action reduction',
		'--- | --- | --- | --- | --- | --- | ---',
		rows,
		'',
		'## Findings',
		'',
		...report.findings.map((item) => `- ${item}`),
		'',
	].join('\n')
}

function percent(value) {
	return `${Math.round(value * 100)}%`
}

function round2(value) {
	return Math.round(value * 100) / 100
}
