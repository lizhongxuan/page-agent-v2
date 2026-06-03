import { chromium } from '@playwright/test'
import { spawn } from 'node:child_process'
import { mkdir, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'

const rootDir = process.cwd()
const artifactDir =
	process.env.SITE_TASK_GUIDE_UI_STATE_E2E_ARTIFACT_DIR ??
	path.join(rootDir, 'artifacts/site-task-guide-ui-state-memory-e2e/manual')
const dataDir = path.join(artifactDir, 'backend-data')
const backendAddr = process.env.SITE_TASK_GUIDE_UI_STATE_E2E_BACKEND_ADDR ?? '127.0.0.1:38433'
const backendURL = `http://${backendAddr}`

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
		NO_PROXY: `127.0.0.1,localhost,${process.env.NO_PROXY ?? ''}`,
		no_proxy: `127.0.0.1,localhost,${process.env.no_proxy ?? ''}`,
	},
	detached: true,
	stdio: ['ignore', 'pipe', 'pipe'],
})
const backendLogs = []
backend.stdout.on('data', (chunk) => backendLogs.push(chunk.toString()))
backend.stderr.on('data', (chunk) => backendLogs.push(chunk.toString()))

let browser
try {
	await waitForReady(`${backendURL}/ready`)
	await seedTaskRun()
	const cases = await runCases()
	assertCases(cases)
	await dumpCaseArtifacts(cases)
	await writeFile(path.join(artifactDir, 'ui-state-e2e-report.md'), markdownReport(cases))
	await writeJSON('ui-state-e2e-report.json', {
		backendURL,
		generatedAt: new Date().toISOString(),
		cases,
	})

	browser = await chromium.launch({ headless: true })
	const page = await browser.newPage({ viewport: { width: 1280, height: 820 } })
	await page.setContent(reportHTML(cases), { waitUntil: 'domcontentloaded' })
	await page.screenshot({ path: path.join(artifactDir, 'ui-state-e2e-report.png'), fullPage: true })
} finally {
	if (browser) await browser.close()
	try {
		process.kill(-backend.pid, 'SIGTERM')
	} catch {
		backend.kill('SIGTERM')
	}
	await writeFile(path.join(artifactDir, 'backend.log'), backendLogs.join(''))
}

console.log(`UI state E2E artifacts: ${artifactDir}`)

async function seedTaskRun() {
	await postJSON('/api/memory/task-runs', {
		id: 'task_run_ui_state_restore',
		projectId: 'ui-state-e2e',
		site: 'middleware.example.com',
		taskTemplate: 'restore middleware instance from latest full backup',
		summary: 'Restored a middleware instance from latest full backup.',
		status: 'success',
		actionSteps: [
			step(
				'open_detail',
				1,
				'click',
				'Instance name',
				'Open the instance detail page.',
				listState(),
				detailState()
			),
			step(
				'open_backup',
				2,
				'click',
				'Data Backup',
				'Open the Data Backup page.',
				detailState(),
				backupState()
			),
			step(
				'open_restore',
				3,
				'click',
				'Data Restore',
				'Click Data Restore.',
				backupState(),
				restoreModalState()
			),
			step(
				'start_restore',
				4,
				'click',
				'Start Restore',
				'Click Start Restore.',
				restoreModalState()
			),
		],
	})
}

function step(
	id,
	stepIndex,
	actionType,
	targetName,
	resultSummary,
	beforeObservation,
	afterObservation
) {
	return {
		id,
		stepIndex,
		actionType,
		targetName,
		resultSummary,
		beforeObservation,
		...(afterObservation ? { afterObservation } : {}),
	}
}

async function runCases() {
	const definitions = [
		{
			id: 'from_list',
			name: 'Start from instance list',
			request: contextRequest('https://middleware.example.com/instances', listState()),
			expectedStateName: 'Instances',
			expectedFirstStep: 'Open the instance detail page.',
		},
		{
			id: 'from_backup_tab',
			name: 'Start from Full Backup tab',
			request: contextRequest(
				'https://middleware.example.com/instances/pg-prod/backups',
				backupState()
			),
			expectedStateName: 'Data Backup',
			expectedFirstStep: 'Click Data Restore.',
		},
		{
			id: 'from_restore_modal',
			name: 'Start from restore modal',
			request: contextRequest(
				'https://middleware.example.com/instances/pg-prod/backups',
				restoreModalState()
			),
			expectedStateName: 'Data Restore',
			expectedFirstStep: 'Click Start Restore.',
		},
		{
			id: 'wrong_settings',
			name: 'Wrong settings page',
			request: contextRequest('https://middleware.example.com/settings/audit', {
				title: 'Audit Settings',
				visibleTextSample: 'Audit Settings Retention days Save',
				controlSignatures: [
					{ role: 'textbox', name: 'Retention days' },
					{ role: 'button', name: 'Save' },
				],
			}),
			expectedNoHit: true,
		},
	]
	const results = []
	for (const definition of definitions) {
		const response = await postJSON('/api/memory/context', definition.request)
		const guide = response.siteTaskGuides?.[0]
		results.push({
			...definition,
			response,
			matchedStateId: guide?.matchedStateId,
			matchedStateName: guide?.matchedStateName,
			remainingSteps: guide?.steps ?? [],
			prompt: response.contextPrompt ?? '',
			passed: definition.expectedNoHit
				? !guide
				: Boolean(
						guide &&
						guide.matchedStateName === definition.expectedStateName &&
						guide.steps?.[0] === definition.expectedFirstStep
					),
		})
	}
	return results
}

function assertCases(cases) {
	const failed = cases.filter((item) => !item.passed)
	if (failed.length > 0) {
		throw new Error(`UI state E2E failed: ${failed.map((item) => item.id).join(', ')}`)
	}
}

function contextRequest(currentUrl, state) {
	return {
		projectId: 'ui-state-e2e',
		task: '让实例基于最新全量备份恢复',
		currentUrl,
		pageObservation: {
			title: state.title,
			visibleText: [state.visibleTextSample],
			controls: state.controlSignatures ?? [],
			activeTabs: state.activeTabs ?? [],
			activeSurfaces: state.activeSurfaces ?? [],
		},
		metadata: { module: currentUrl.includes('/settings') ? 'settings' : 'instances' },
	}
}

function listState() {
	return {
		projectId: 'ui-state-e2e',
		site: 'middleware.example.com',
		url: 'https://middleware.example.com/instances',
		urlPattern: 'https://middleware.example.com/instances',
		title: 'Instances',
		visibleTextSample: 'Instances Instance name Data Backup',
		controlSignatures: [{ role: 'button', name: 'Instance name' }],
	}
}

function detailState() {
	return {
		...listState(),
		url: 'https://middleware.example.com/instances/pg-prod',
		urlPattern: 'https://middleware.example.com/instances/pg-prod',
		title: 'Instance Detail',
		visibleTextSample: 'Instance Detail Data Backup',
		controlSignatures: [{ role: 'tab', name: 'Data Backup' }],
	}
}

function backupState() {
	return {
		...listState(),
		url: 'https://middleware.example.com/instances/pg-prod/backups',
		urlPattern: 'https://middleware.example.com/instances/pg-prod/backups',
		title: 'Data Backup',
		activeTabs: ['Full Backup'],
		visibleTextSample: 'Data Backup Full Backup Data Restore Start Restore',
		controlSignatures: [
			{ role: 'tab', name: 'Full Backup' },
			{ role: 'button', name: 'Data Restore' },
			{ role: 'button', name: 'Start Restore' },
		],
	}
}

function restoreModalState() {
	return {
		...backupState(),
		title: 'Data Restore',
		visibleTextSample: 'Data Restore Start Restore',
		activeSurfaces: [
			{
				surfaceType: 'modal',
				title: 'Data Restore',
				controls: [{ role: 'button', name: 'Start Restore' }],
			},
		],
	}
}

async function dumpCaseArtifacts(cases) {
	for (const item of cases) {
		await writeJSON(`context-${item.id}.json`, item.response)
		await writeFile(path.join(artifactDir, `prompt-${item.id}.txt`), item.prompt)
	}
}

function markdownReport(cases) {
	const lines = [
		'# Site Task Guide UI State E2E',
		'',
		`Generated at: ${new Date().toISOString()}`,
		`Backend: ${backendURL}`,
		'',
		'Case | Passed | Matched state | Remaining first step',
		'--- | --- | --- | ---',
	]
	for (const item of cases) {
		lines.push(
			`${item.name} | ${item.passed ? 'yes' : 'no'} | ${item.matchedStateName ?? ''} | ${item.remainingSteps[0] ?? ''}`
		)
	}
	return `${lines.join('\n')}\n`
}

function reportHTML(cases) {
	return `<!doctype html>
<html>
<head>
<meta charset="utf-8" />
<title>UI State E2E</title>
<style>
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 32px; color: #17202a; }
table { border-collapse: collapse; width: 100%; }
th, td { border: 1px solid #d5dce4; padding: 10px; text-align: left; vertical-align: top; }
th { background: #f3f6f9; }
.ok { color: #087443; font-weight: 700; }
</style>
</head>
<body>
<h1>Site Task Guide UI State E2E</h1>
<table>
<thead><tr><th>Case</th><th>Status</th><th>Matched state</th><th>Remaining steps</th></tr></thead>
<tbody>
${cases
	.map(
		(item) =>
			`<tr><td>${escapeHTML(item.name)}</td><td class="ok">${item.passed ? 'passed' : 'failed'}</td><td>${escapeHTML(item.matchedStateName ?? '')}</td><td>${escapeHTML(item.remainingSteps.join(' / '))}</td></tr>`
	)
	.join('')}
</tbody>
</table>
</body>
</html>`
}

async function postJSON(pathname, body) {
	const response = await fetch(`${backendURL}${pathname}`, {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify(body),
	})
	if (!response.ok) {
		throw new Error(`${pathname} failed: ${response.status} ${await response.text()}`)
	}
	return response.json()
}

async function waitForReady(url) {
	const started = Date.now()
	while (Date.now() - started < 15_000) {
		try {
			const response = await fetch(url)
			if (response.ok) return
		} catch {}
		await new Promise((resolve) => setTimeout(resolve, 200))
	}
	throw new Error(`Timed out waiting for ${url}`)
}

async function writeJSON(filename, value) {
	await writeFile(path.join(artifactDir, filename), `${JSON.stringify(value, null, 2)}\n`)
}

function escapeHTML(value) {
	return String(value)
		.replaceAll('&', '&amp;')
		.replaceAll('<', '&lt;')
		.replaceAll('>', '&gt;')
		.replaceAll('"', '&quot;')
}
