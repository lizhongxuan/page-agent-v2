import { chromium } from '@playwright/test'
import { spawn } from 'node:child_process'
import { mkdir, readFile, readdir, rm, writeFile } from 'node:fs/promises'
import { createServer } from 'node:http'
import path from 'node:path'

const rootDir = process.cwd()
const artifactDir =
	process.env.SITE_TASK_GUIDE_E2E_ARTIFACT_DIR ??
	path.join(rootDir, 'artifacts/site-task-guide-manual-wiki-chrome/manual')
const dataDir = path.join(artifactDir, 'backend-data')
const backendAddr = process.env.SITE_TASK_GUIDE_E2E_BACKEND_ADDR ?? '127.0.0.1:38412'
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

const fixture = await createFixtureServer(backendURL)
const fixtureURL = `http://127.0.0.1:${fixture.port}/fixture.html`

let browser
const report = {
	backendURL,
	fixtureURL,
	steps: [],
	metrics: {
		firstRunActionCount: 0,
		secondRunSuggestedGuideStepCount: 0,
		firstRunInvalidClickCount: 0,
		secondRunInvalidClickCount: 0,
		feedbackLabels: [],
		extensionLoaded: false,
	},
	artifacts: {},
}

try {
	await waitForReady(`${backendURL}/ready`, 'backend')
	browser = await launchChrome()
	await loadBuiltExtensionInChrome()
	const page = await browser.newPage({ viewport: { width: 1440, height: 980 } })
	await page.goto(fixtureURL)
	await page.fill('[name="site"]', 'middleware.example.com')
	await page.fill('[name="module"]', 'backup')
	await page.fill('[name="title"]', 'Middleware backup restore manual')
	await page.fill(
		'[name="content"]',
		[
			'# Middleware backup restore / 中间件备份恢复',
			'Open the instance list / 实例列表 and click the instance name / 实例名称 to enter the detail page.',
			'Open Data Backup / 数据备份, switch to the Full Backup / 全量备份 tab, and click Data Restore / 数据恢复.',
			'Choose the latest backup record / 最新备份记录, select the node IP / 节点 IP, then click Start Restore / 开始恢复.',
			'If the current page does not show Data Backup / 数据备份 or Full Backup / 全量备份, abandon this manual hint.',
		].join('\n')
	)
	await page.click('[data-testid="import"]')
	await expectText(page, '[data-testid="manual-count"]', '1')
	await expectText(page, '[data-testid="wiki-count"]', '5')
	await page.screenshot({ path: path.join(artifactDir, '01-manual-import.png'), fullPage: true })
	report.steps.push('Imported a site-scoped manual and rendered source/wiki counts.')

	await page.fill('[name="task"]', '让实例基于最新全量备份恢复')
	await page.fill('[name="currentUrl"]', 'https://middleware.example.com/instances/pg/backups')
	await page.click('[data-testid="preview"]')
	await expectText(page, '[data-testid="preview-prompt"]', '<site_manual_knowledge>')
	await page.screenshot({ path: path.join(artifactDir, '02-manual-preview.png'), fullPage: true })
	report.steps.push('Previewed runtime manual context before any reusable task guide exists.')

	const beforeContext = await postJSON('/api/memory/context', {
		projectId: 'default',
		task: '让实例基于最新全量备份恢复',
		currentUrl: 'https://middleware.example.com/instances/pg/backups',
		pageObservation: {
			title: 'Data Backup',
			visibleText: ['Data Backup', 'Full Backup', 'Data Restore', 'Start Restore'],
			controls: [
				{ role: 'tab', name: 'Full Backup' },
				{ role: 'button', name: 'Data Restore' },
			],
		},
		metadata: { module: 'backup' },
	})
	await writeJSON('before-context.json', beforeContext)
	assert(beforeContext.siteManualKnowledge?.length > 0, 'manual should be injected before guide')
	assert(!beforeContext.siteTaskGuides?.length, 'guide should not exist before successful task run')
	report.steps.push('Captured first context: manual knowledge only.')

	await page.click('[data-testid="complete-task"]')
	await expectText(page, '[data-testid="guide-count"]', '1')
	await page.screenshot({ path: path.join(artifactDir, '03-guide-created.png'), fullPage: true })
	report.metrics.firstRunActionCount = 7
	report.steps.push('Uploaded a successful task run and verified a site task guide was saved.')

	await page.click('[data-testid="context"]')
	await expectText(page, '[data-testid="context-prompt"]', '<site_task_guides>')
	await expectText(page, '[data-testid="context-prompt"]', '<site_manual_knowledge>')
	await page.screenshot({
		path: path.join(artifactDir, '04-context-with-guide.png'),
		fullPage: true,
	})
	const afterContext = JSON.parse(await page.locator('[data-testid="context-json"]').textContent())
	await writeJSON('after-context.json', afterContext)
	report.metrics.secondRunSuggestedGuideStepCount =
		afterContext.siteTaskGuides?.[0]?.steps?.length ?? 0
	report.metrics.contextPromptHasGuide =
		afterContext.contextPrompt?.includes('<site_task_guides>') ?? false
	report.metrics.contextPromptHasManual =
		afterContext.contextPrompt?.includes('<site_manual_knowledge>') ?? false
	report.steps.push('Captured second context: guide plus manual injected for a similar task.')

	await page.click('[data-testid="disable-manual"]')
	await expectText(page, '[data-testid="manual-count"]', '0')
	await page.click('[data-testid="context"]')
	await page.waitForFunction(() => {
		try {
			const value = JSON.parse(
				document.querySelector('[data-testid="context-json"]')?.textContent || '{}'
			)
			return value.siteTaskGuides?.length > 0 && !value.siteManualKnowledge?.length
		} catch {
			return false
		}
	})
	const disabledContext = JSON.parse(
		await page.locator('[data-testid="context-json"]').textContent()
	)
	await writeJSON('after-disable-context.json', disabledContext)
	assert(disabledContext.siteTaskGuides?.length > 0, 'guide should remain after manual disable')
	assert(
		!disabledContext.siteManualKnowledge?.length,
		'manual should not be injected after disable'
	)
	report.metrics.manualDisabledRemovesManual = !disabledContext.siteManualKnowledge?.length
	report.steps.push(
		'Disabled the manual and verified context keeps the guide but removes manual knowledge.'
	)

	const wrongSiteContext = await postJSON('/api/memory/context', {
		projectId: 'default',
		task: '让实例基于最新全量备份恢复',
		currentUrl: 'https://other.example.com/instances/pg/backups',
		pageObservation: {
			title: 'Data Backup',
			visibleText: ['Data Backup', 'Full Backup', 'Data Restore'],
			controls: [{ role: 'button', name: 'Data Restore' }],
		},
		metadata: { module: 'backup' },
	})
	await writeJSON('wrong-site-context.json', wrongSiteContext)
	assert(!wrongSiteContext.siteTaskGuides?.length, 'cross-site guide must not be injected')
	assert(!wrongSiteContext.siteManualKnowledge?.length, 'cross-site manual must not be injected')
	report.metrics.crossSiteGuideInjected = Boolean(wrongSiteContext.siteTaskGuides?.length)
	report.metrics.crossSiteManualInjected = Boolean(wrongSiteContext.siteManualKnowledge?.length)
	report.steps.push('Verified cross-site requests do not receive the saved site guide or manual.')

	const guideID = afterContext.siteTaskGuides?.[0]?.id
	assert(guideID, 'guide id should be available for feedback')
	await postJSON(`/api/memory/site-task-guides/${encodeURIComponent(guideID)}/feedback`, {
		label: 'abandoned_mismatch',
		reason: 'Modified entry page did not match the saved guide.',
		taskRunId: 'task_run_restore_e2e_mismatch',
		contextId: afterContext.contextId,
	})
	report.metrics.feedbackLabels.push('abandoned_mismatch')
	report.steps.push('Recorded abandoned_mismatch feedback for a changed page entry.')

	await dumpBackendData()
	report.artifacts = {
		screenshots: [
			'00-extension-manual-library.png',
			'01-manual-import.png',
			'02-manual-preview.png',
			'03-guide-created.png',
			'04-context-with-guide.png',
		],
		contexts: [
			'before-context.json',
			'after-context.json',
			'after-disable-context.json',
			'wrong-site-context.json',
		],
		backendData: 'backend-data-snapshot.json',
	}
	await writeJSON('e2e-report.json', report)
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

async function createFixtureServer(baseURL) {
	const html = fixtureHTML('')
	const server = createServer(async (req, res) => {
		if (req.url === '/fixture.html' || req.url === '/') {
			res.writeHead(200, { 'content-type': 'text/html; charset=utf-8' })
			res.end(html)
			return
		}
		if (req.url?.startsWith('/api/')) {
			try {
				const chunks = []
				for await (const chunk of req) chunks.push(chunk)
				const upstream = await fetch(baseURL + req.url, {
					method: req.method,
					headers: req.headers['content-type']
						? { 'content-type': String(req.headers['content-type']) }
						: {},
					body: chunks.length > 0 ? Buffer.concat(chunks) : undefined,
				})
				res.writeHead(upstream.status, {
					'content-type': upstream.headers.get('content-type') ?? 'application/json',
				})
				res.end(Buffer.from(await upstream.arrayBuffer()))
			} catch (error) {
				res.writeHead(502, { 'content-type': 'text/plain; charset=utf-8' })
				res.end(error instanceof Error ? error.message : String(error))
			}
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

function fixtureHTML(baseURL) {
	return `<!doctype html>
<html>
<head>
  <meta charset="utf-8" />
  <title>Site Manual Library Fixture</title>
  <style>
    body { margin: 0; font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; color: #151515; background: #f7f7f4; }
    main { display: grid; grid-template-columns: 360px 1fr; min-height: 100vh; }
    aside { border-right: 1px solid #d6d6ce; padding: 16px; background: #ffffff; }
    section { padding: 16px; }
    label { display: grid; gap: 4px; margin-bottom: 10px; font-size: 12px; color: #555; }
    input, textarea { font: inherit; border: 1px solid #c9c9c0; border-radius: 6px; padding: 8px; background: #fff; color: #151515; }
    textarea { min-height: 160px; resize: vertical; }
    button { border: 1px solid #1f6f68; border-radius: 6px; background: #1f6f68; color: #fff; font: inherit; font-size: 13px; padding: 8px 10px; margin-right: 8px; cursor: pointer; }
    button.secondary { background: #fff; color: #1f6f68; }
    pre { white-space: pre-wrap; overflow: auto; max-height: 340px; border: 1px solid #d6d6ce; border-radius: 6px; padding: 12px; background: #fff; font-size: 12px; }
    .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
    .metric { display: inline-flex; gap: 6px; margin: 4px 10px 8px 0; font-size: 12px; }
    .metric strong { font-variant-numeric: tabular-nums; }
  </style>
</head>
<body>
<main>
  <aside>
    <h1>Site Manual Library</h1>
    <label>Site <input name="site" /></label>
    <label>Module <input name="module" /></label>
    <label>Title <input name="title" /></label>
    <label>Manual Content <textarea name="content"></textarea></label>
    <button data-testid="import">Import Manual</button>
    <button class="secondary" data-testid="disable-manual">Disable Manual</button>
    <hr />
    <label>Task <input name="task" /></label>
    <label>Current URL <input name="currentUrl" /></label>
    <button data-testid="preview">Preview Manual Context</button>
    <button data-testid="complete-task">Upload Successful Task Run</button>
    <button data-testid="context">Get Runtime Context</button>
  </aside>
  <section>
    <div class="metric">manual sources <strong data-testid="manual-count">0</strong></div>
    <div class="metric">wiki chunks <strong data-testid="wiki-count">0</strong></div>
    <div class="metric">site guides <strong data-testid="guide-count">0</strong></div>
    <h2>Preview Prompt</h2>
    <pre data-testid="preview-prompt"></pre>
    <h2>Runtime Prompt</h2>
    <pre data-testid="context-prompt"></pre>
    <h2>Runtime JSON</h2>
    <pre data-testid="context-json">{}</pre>
  </section>
</main>
<script>
const baseURL = ${JSON.stringify(baseURL)};
const state = { sourceId: '', guideId: '' };
const $ = (selector) => document.querySelector(selector);
const field = (name) => document.querySelector('[name="' + name + '"]').value;
async function request(method, path, body) {
  const response = await fetch(baseURL + path, {
    method,
    headers: body ? { 'content-type': 'application/json' } : {},
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!response.ok) throw new Error(method + ' ' + path + ' failed: ' + response.status + ' ' + await response.text());
  if (response.status === 204) return {};
  return response.json();
}
async function refresh() {
  const site = field('site');
  const moduleName = field('module');
  const manuals = await request('GET', '/api/memory/site-manuals?projectId=default&site=' + encodeURIComponent(site) + '&module=' + encodeURIComponent(moduleName));
  const sources = manuals.sources || [];
  state.sourceId = sources[0]?.id || state.sourceId;
  $('[data-testid="manual-count"]').textContent = String(sources.length);
  if (state.sourceId) {
    const wiki = await request('GET', '/api/memory/site-manuals/' + encodeURIComponent(state.sourceId) + '/wiki');
    $('[data-testid="wiki-count"]').textContent = String((wiki.chunks || []).length);
  }
  const guides = await request('GET', '/api/memory/site-task-guides?projectId=default&site=' + encodeURIComponent(site));
  state.guideId = guides.guides?.[0]?.id || state.guideId;
  $('[data-testid="guide-count"]').textContent = String((guides.guides || []).length);
}
function observation(url, title, visibleTextSample, controls, extra = {}) {
  return {
    projectId: 'default',
    site: field('site'),
    url,
    urlPattern: url,
    title,
    visibleTextSample,
    controlSignatures: controls,
    ...extra,
  };
}
document.querySelector('[data-testid="import"]').onclick = async () => {
  const result = await request('POST', '/api/memory/site-manuals/import', {
    projectId: 'default',
    site: field('site'),
    module: field('module'),
    title: field('title'),
    sourceType: 'markdown',
    content: field('content'),
  });
  state.sourceId = result.source.id;
  await refresh();
};
document.querySelector('[data-testid="preview"]').onclick = async () => {
  const result = await request('POST', '/api/memory/site-manuals/preview-context', {
    projectId: 'default',
    site: field('site'),
    module: field('module'),
    task: field('task'),
    url: field('currentUrl'),
    title: 'Data Backup',
    visibleTextSample: 'Data Backup Full Backup Data Restore Start Restore',
  });
  $('[data-testid="preview-prompt"]').textContent = result.prompt || '';
};
document.querySelector('[data-testid="complete-task"]').onclick = async () => {
  const result = await request('POST', '/api/memory/task-runs', {
    id: 'task_run_restore_e2e',
    projectId: 'default',
    site: field('site'),
    taskTemplate: 'restore {{instance_name}} from latest full backup',
    summary: 'Restored instance from the latest full backup.',
    status: 'success',
    originalPath: ['instances', 'detail', 'backups'],
    optimizedPath: ['instances', 'detail', 'backups'],
    actionSteps: [
      {
        id: 'step_open_instance',
        stepIndex: 1,
        pageStateId: 'instances',
        actionType: 'click',
        targetName: 'Instance name',
        resultSummary: 'Open the instance detail page.',
        beforeObservation: observation('https://middleware.example.com/instances', 'Instances', 'Instances Instance name Status', [
          { role: 'button', name: 'Instance name' },
        ]),
      },
      {
        id: 'step_open_backup',
        stepIndex: 2,
        pageStateId: 'detail',
        actionType: 'click',
        targetName: 'Data Backup',
        resultSummary: 'Open the Data Backup page.',
        beforeObservation: observation('https://middleware.example.com/instances/pg', 'Instance Detail', 'Instance Detail Data Backup Metrics', [
          { role: 'link', name: 'Data Backup' },
        ]),
      },
      {
        id: 'step_full_backup',
        stepIndex: 3,
        pageStateId: 'backups',
        actionType: 'click',
        targetName: 'Full Backup',
        resultSummary: 'Switch to the Full Backup tab.',
        beforeObservation: observation('https://middleware.example.com/instances/pg/backups', 'Data Backup', 'Data Backup Incremental Backup Full Backup', [
          { role: 'tab', name: 'Full Backup' },
        ]),
      },
      {
        id: 'step_restore',
        stepIndex: 4,
        pageStateId: 'backups',
        actionType: 'click',
        targetName: 'Data Restore',
        resultSummary: 'Click Data Restore.',
        beforeObservation: observation(
          'https://middleware.example.com/instances/pg/backups',
          'Data Backup',
          'Data Backup Full Backup Data Restore Latest backup record',
          [
            { role: 'tab', name: 'Full Backup', selected: true },
            { role: 'button', name: 'Data Restore' },
          ],
          { activeTabs: ['Full Backup'] }
        ),
      },
      {
        id: 'step_latest',
        stepIndex: 5,
        pageStateId: 'backups',
        actionType: 'click',
        targetName: 'Latest backup record',
        resultSummary: 'Choose the latest backup record.',
        beforeObservation: observation(
          'https://middleware.example.com/instances/pg/backups',
          'Data Restore',
          'Data Restore Latest backup record Node IP Start Restore',
          [
            { role: 'row', name: 'Latest backup record' },
            { role: 'button', name: 'Start Restore' },
          ],
          {
            activeSurfaces: [
				{
					surfaceType: 'modal',
					title: 'Data Restore',
					text: ['Latest backup record', 'Node IP', 'Start Restore'],
					controls: [
						{ role: 'row', name: 'Latest backup record' },
						{ role: 'button', name: 'Start Restore' },
                ],
              },
            ],
          }
        ),
      },
      {
        id: 'step_node',
        stepIndex: 6,
        pageStateId: 'backups',
        actionType: 'select',
        targetName: 'Node IP',
        valueTemplate: '{{node_ip}}',
        resultSummary: 'Select the node IP.',
        beforeObservation: observation('https://middleware.example.com/instances/pg/backups', 'Data Restore', 'Data Restore Node IP Start Restore', [
          { role: 'combobox', name: 'Node IP' },
          { role: 'button', name: 'Start Restore' },
        ]),
      },
      {
        id: 'step_start',
        stepIndex: 7,
        pageStateId: 'backups',
        actionType: 'click',
        targetName: 'Start Restore',
        resultSummary: 'Click Start Restore.',
        beforeObservation: observation('https://middleware.example.com/instances/pg/backups', 'Data Restore', 'Data Restore Start Restore', [
          { role: 'button', name: 'Start Restore' },
        ]),
      }
    ]
  });
  state.guideId = result.siteTaskGuideId || state.guideId;
  await refresh();
};
document.querySelector('[data-testid="context"]').onclick = async () => {
  const result = await request('POST', '/api/memory/context', {
    projectId: 'default',
    task: field('task'),
    currentUrl: field('currentUrl'),
    pageObservation: {
      title: 'Data Backup',
      visibleText: ['Data Backup', 'Full Backup', 'Data Restore', 'Start Restore'],
      controls: [
        { role: 'tab', name: 'Full Backup' },
        { role: 'button', name: 'Data Restore' },
        { role: 'button', name: 'Start Restore' },
      ],
    },
    metadata: { module: field('module') },
  });
  $('[data-testid="context-prompt"]').textContent = result.contextPrompt || '';
  $('[data-testid="context-json"]').textContent = JSON.stringify(result, null, 2);
};
document.querySelector('[data-testid="disable-manual"]').onclick = async () => {
  if (!state.sourceId) await refresh();
  await request('POST', '/api/memory/site-manuals/' + encodeURIComponent(state.sourceId) + '/disable');
  await refresh();
};
</script>
</body>
</html>`
}

async function launchChrome() {
	try {
		return await chromium.launch({ channel: 'chrome', headless: true })
	} catch {
		return chromium.launch({ headless: true })
	}
}

async function loadBuiltExtensionInChrome() {
	const extensionPath = await ensureBuiltExtensionPath()
	const hubPath = await firstExistingExtensionPage(extensionPath, ['hub.html', 'hub/index.html'])
	const profileDir = path.join(artifactDir, 'chrome-extension-profile')
	await rm(profileDir, { recursive: true, force: true })
	const { context, extensionId, browserName } = await launchExtensionContext(
		profileDir,
		extensionPath
	)
	try {
		const page = await context.newPage()
		const hubURL = `chrome-extension://${extensionId}/${hubPath}?view=site-manuals&site=middleware.example.com&url=${encodeURIComponent(
			'https://middleware.example.com/instances/pg/backups'
		)}`
		await page.goto(hubURL)
		await page.evaluate(
			async (workflowBackend) => {
				await chrome.storage.local.set({ workflowBackend })
			},
			{ baseUrl: backendURL, projectId: 'default' }
		)
		await page.reload()
		await expectText(page, 'body', 'Site Manual Library')
		await expectText(page, 'body', 'No site manuals match the current filters.')
		await page.screenshot({
			path: path.join(artifactDir, '00-extension-manual-library.png'),
			fullPage: true,
		})
		report.metrics.extensionLoaded = true
		report.metrics.extensionBrowser = browserName
		report.metrics.extensionId = extensionId
		report.steps.push('Loaded the built Chrome extension and opened the site manual library hub.')
	} finally {
		await context.close()
	}
}

async function ensureBuiltExtensionPath() {
	const explicit = process.env.SITE_TASK_GUIDE_E2E_EXTENSION_PATH
	if (explicit) return assertExtensionPath(explicit)
	const found = await findBuiltExtensionPath()
	if (found) return found
	report.steps.push('Built the Chrome extension because no existing build output was found.')
	await runCommand('npm', ['run', 'build:ext'], rootDir)
	const rebuilt = await findBuiltExtensionPath()
	if (!rebuilt) {
		throw new Error('Chrome extension build output was not found after npm run build:ext.')
	}
	return rebuilt
}

async function findBuiltExtensionPath() {
	for (const candidate of [
		path.join(rootDir, 'packages/extension/.output/chrome-mv3'),
		path.join(rootDir, 'packages/extension/.output/chrome-mv3-dev'),
		path.join(rootDir, '.output/chrome-mv3'),
		path.join(rootDir, '.output/chrome-mv3-dev'),
	]) {
		try {
			return await assertExtensionPath(candidate)
		} catch {}
	}
	return ''
}

async function assertExtensionPath(extensionPath) {
	await readFile(path.join(extensionPath, 'manifest.json'), 'utf8')
	return extensionPath
}

async function firstExistingExtensionPage(extensionPath, candidates) {
	for (const candidate of candidates) {
		try {
			await readFile(path.join(extensionPath, candidate), 'utf8')
			return candidate
		} catch {}
	}
	throw new Error(
		`None of these extension pages exist in ${extensionPath}: ${candidates.join(', ')}`
	)
}

async function launchExtensionContext(profileDir, extensionPath) {
	const args = [
		`--disable-extensions-except=${extensionPath}`,
		`--load-extension=${extensionPath}`,
		'--no-first-run',
		'--no-default-browser-check',
	]
	const attempts = [
		{ name: 'google-chrome', options: { channel: 'chrome', headless: false } },
		{ name: 'playwright-chromium', options: { headless: false } },
		{ name: 'playwright-chromium-headless', options: { headless: true } },
	]
	const errors = []
	for (const [index, attempt] of attempts.entries()) {
		const attemptProfile = `${profileDir}-${index}`
		await rm(attemptProfile, { recursive: true, force: true })
		let context
		try {
			context = await chromium.launchPersistentContext(attemptProfile, {
				...attempt.options,
				ignoreDefaultArgs: ['--disable-extensions'],
				args,
			})
			const worker = await waitForExtensionWorker(context)
			if (worker) {
				return {
					context,
					extensionId: new URL(worker.url()).host,
					browserName: attempt.name,
				}
			}
			await context.close()
			errors.push(`${attempt.name}: extension service worker did not start`)
		} catch (error) {
			if (context) await context.close().catch(() => {})
			errors.push(`${attempt.name}: ${error instanceof Error ? error.message : String(error)}`)
		}
	}
	throw new Error(`Unable to load built extension:\n${errors.join('\n')}`)
}

async function waitForExtensionWorker(context) {
	let worker = context.serviceWorkers()[0]
	if (!worker) {
		worker = await context.waitForEvent('serviceworker', { timeout: 5_000 }).catch(() => null)
	}
	if (worker?.url().startsWith('chrome-extension://')) return worker
	return null
}

async function runCommand(command, args, cwd) {
	await new Promise((resolve, reject) => {
		const child = spawn(command, args, {
			cwd,
			env: process.env,
			stdio: ['ignore', 'pipe', 'pipe'],
		})
		const chunks = []
		child.stdout.on('data', (chunk) => chunks.push(chunk.toString()))
		child.stderr.on('data', (chunk) => chunks.push(chunk.toString()))
		child.once('error', reject)
		child.once('exit', (code) => {
			if (code === 0) {
				resolve()
				return
			}
			reject(new Error(`${command} ${args.join(' ')} failed with code ${code}\n${chunks.join('')}`))
		})
	})
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

async function expectText(page, selector, expected) {
	await page.waitForFunction(
		([sel, value]) => document.querySelector(sel)?.textContent?.includes(value),
		[selector, expected],
		{ timeout: 10_000 }
	)
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

async function dumpBackendData() {
	const snapshot = {}
	for (const name of await readdir(dataDir)) {
		if (!name.endsWith('.json')) continue
		const raw = await readFile(path.join(dataDir, name), 'utf8')
		snapshot[name] = JSON.parse(raw || '{}')
	}
	await writeJSON('backend-data-snapshot.json', snapshot)
}

async function writeJSON(name, value) {
	await writeFile(path.join(artifactDir, name), JSON.stringify(value, null, 2))
}

function assert(condition, message) {
	if (!condition) throw new Error(message)
}
