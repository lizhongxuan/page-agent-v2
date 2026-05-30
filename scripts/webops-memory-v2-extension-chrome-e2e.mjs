import crypto from 'node:crypto'
import fs from 'node:fs'
import http from 'node:http'
import path from 'node:path'
import { chromium } from 'playwright'

const backendURL = process.env.WEBOPS_MEMORY_V2_BACKEND_URL ?? 'http://127.0.0.1:38433'
const artifactDir = path.resolve(
	process.env.WEBOPS_MEMORY_V2_EXTENSION_ARTIFACT_DIR ??
		'artifacts/webops-memory-v2-extension/manual'
)
const extensionPath = path.resolve(
	process.env.WEBOPS_MEMORY_V2_EXTENSION_PATH ??
		'/Users/lizhongxuan/Desktop/page-agent-ext-1.8.2-chrome'
)
const llmModel = process.env.WEBOPS_MEMORY_V2_LLM_MODEL ?? 'mock-page-agent'
const configuredLLMBaseURL = process.env.WEBOPS_MEMORY_V2_LLM_BASE_URL
const configuredLLMApiKey = process.env.WEBOPS_MEMORY_V2_LLM_API_KEY
const browserChannel = process.env.PLAYWRIGHT_CHROME_CHANNEL ?? 'chromium'

fs.mkdirSync(artifactDir, { recursive: true })

const scenarioID = `wmv2_ext_${Date.now().toString(36)}`
const projectID = `project_${scenarioID}`
const userDataDir = path.join(artifactDir, 'chrome-profile')
const screenshotPath = path.join(artifactDir, 'extension-sidepanel.png')
const pageScreenshotPath = path.join(artifactDir, 'business-page.png')
const reportPath = path.join(artifactDir, 'report.json')
const inspectorPath = path.join(artifactDir, 'memory-inspector.json')

function assert(condition, message) {
	if (!condition) throw new Error(message)
}

function listen(server, host = '127.0.0.1') {
	return new Promise((resolve) => {
		server.listen(0, host, () => {
			const address = server.address()
			resolve(`http://${host}:${address.port}`)
		})
	})
}

function closeServer(server) {
	return new Promise((resolve) => server.close(resolve))
}

async function post(pathname, body) {
	const response = await fetch(backendURL + pathname, {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify(body),
	})
	const data = await response.json().catch(() => ({}))
	if (!response.ok) {
		throw new Error(`${pathname} failed ${response.status}: ${JSON.stringify(data)}`)
	}
	return data
}

async function get(pathname) {
	const response = await fetch(backendURL + pathname)
	const data = await response.json().catch(() => ({}))
	if (!response.ok) {
		throw new Error(`${pathname} failed ${response.status}: ${JSON.stringify(data)}`)
	}
	return data
}

function createBusinessPageServer() {
	return http.createServer((request, response) => {
		if (request.url !== '/' && request.url !== '/services') {
			response.writeHead(404).end('not found')
			return
		}
		response.writeHead(200, {
			'content-type': 'text/html; charset=utf-8',
			'cache-control': 'no-store',
		})
		response.end(`<!doctype html>
<html>
<head>
  <meta charset="utf-8" />
  <title>服务管理</title>
  <style>
    body { font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 32px; background: #f6f8fb; color: #182230; }
    main { max-width: 960px; }
    h1 { margin: 0 0 16px; font-size: 28px; }
    .toolbar { display: flex; gap: 12px; align-items: center; margin-bottom: 16px; }
    input { width: 260px; padding: 10px 12px; border: 1px solid #aeb8c6; border-radius: 6px; font: inherit; }
    button { padding: 10px 14px; border: 1px solid #2457f5; border-radius: 6px; background: #2457f5; color: white; font: inherit; cursor: pointer; }
    #serviceResult { min-height: 360px; padding: 16px; border: 1px solid #d4dce8; border-radius: 8px; background: white; white-space: pre-wrap; line-height: 1.55; }
    .status { display: inline-block; padding: 3px 8px; border-radius: 999px; background: #e8f7ee; color: #197149; }
  </style>
</head>
<body>
<main>
  <h1>服务管理</h1>
  <div class="toolbar">
    <label>服务名称 <input id="serviceName" aria-label="服务名称" placeholder="服务名称" /></label>
    <button id="searchButton" aria-label="搜索">搜索</button>
  </div>
  <section id="serviceResult">请输入服务名称并点击搜索。</section>
</main>
<script>
  document.getElementById('searchButton').addEventListener('click', () => {
    const serviceName = document.getElementById('serviceName').value || 'unknown-service';
    document.getElementById('serviceResult').innerHTML =
      '<h2>服务详情</h2>' +
      '<p>服务名称：' + serviceName + '</p>' +
      '<p>运行状态：<span class="status">running</span></p>' +
      '<p>负责人：SRE Team</p>';
    window.__pageAgentServiceStatus = { serviceName, status: 'running' };
  });
</script>
</body>
</html>`)
	})
}

function createMockLLMServer() {
	const requestSummaries = []
	const server = http.createServer(async (request, response) => {
		if (request.method !== 'POST' || request.url !== '/v1/chat/completions') {
			response.writeHead(404).end('not found')
			return
		}
		const raw = await readBody(request)
		const body = JSON.parse(raw)
		const prompt = String(body.messages?.findLast?.((item) => item.role === 'user')?.content ?? '')
		const summary = summarizePrompt(prompt)
		const action = chooseAction(prompt)
		summary.actionNames = listActionNames(action)
		requestSummaries.push(summary)
		response.writeHead(200, { 'content-type': 'application/json' })
		response.end(
			JSON.stringify({
				id: `chatcmpl_${Date.now()}`,
				object: 'chat.completion',
				created: Math.floor(Date.now() / 1000),
				model: body.model ?? llmModel,
				choices: [
					{
						index: 0,
						finish_reason: 'tool_calls',
						message: {
							role: 'assistant',
							tool_calls: [
								{
									id: `call_${requestSummaries.length}`,
									type: 'function',
									function: {
										name: 'AgentOutput',
										arguments: JSON.stringify(action),
									},
								},
							],
						},
					},
				],
				usage: { prompt_tokens: 10, completion_tokens: 10, total_tokens: 20 },
			})
		)
	})
	return { server, requestSummaries }
}

function readBody(request) {
	return new Promise((resolve, reject) => {
		let data = ''
		request.setEncoding('utf8')
		request.on('data', (chunk) => {
			data += chunk
		})
		request.on('end', () => resolve(data))
		request.on('error', reject)
	})
}

function summarizePrompt(prompt) {
	return {
		hasWebOpsMemory: prompt.includes('<webops_memory>'),
		hasCurrentPage: prompt.includes('<current_page'),
		hasKnowledgeEvidence: prompt.includes('<knowledge_evidence>'),
		hasExperienceHints: prompt.includes('<experience_hints>'),
		hasFailureWarnings: prompt.includes('<failure_warnings>'),
		hasRunningResult: prompt.includes('运行状态') && prompt.includes('running'),
		taskKind: prompt.includes('失败场景')
			? 'failure'
			: prompt.includes('kme-prod-002')
				? 'second_success'
				: 'first_success',
	}
}

function chooseAction(prompt) {
	const inputIndex = findElementIndex(prompt, ['服务名称', 'input'])
	const searchIndex = findElementIndex(prompt, ['搜索', 'button'])
	const isFailureTask = prompt.includes('失败场景')
	const isSecondSuccess = prompt.includes('kme-prod-002')
	const hasRunningResult = prompt.includes('运行状态') && prompt.includes('running')

	if (isFailureTask) {
		if (!prompt.includes('Input text (kme-prod-fail)')) {
			return macroOutput('先复现失败输入分支。', {
				input_text: { index: inputIndex, text: 'kme-prod-fail' },
			})
		}
		return macroOutput('记录失败经验，避免重复错误路径。', {
			done: { text: '失败场景已记录。', success: false },
		})
	}

	if (isSecondSuccess) {
		return macroOutput('使用 WebOps Memory 的经验提示直接完成判断。', {
			done: { text: '根据记忆中的服务状态查询路径，服务状态可按列表搜索后查看。', success: true },
		})
	}

	if (!hasRunningResult) {
		return macroOutput('输入服务名称并提交搜索。', [
			{ input_text: { index: inputIndex, text: 'kme-prod-001' } },
			{ click_element_by_index: { index: searchIndex } },
		])
	}

	return macroOutput('读取页面结果并完成。', {
		done: { text: '服务 kme-prod-001 当前运行状态为 running。', success: true },
	})
}

function listActionNames(actionOutput) {
	const actions = Array.isArray(actionOutput.action) ? actionOutput.action : [actionOutput.action]
	return actions.map((action) => Object.keys(action)[0])
}

function macroOutput(nextGoal, action) {
	return {
		evaluation_previous_goal: 'Previous step was evaluated from the page state.',
		memory: 'Use fixed service search controls and avoid unrelated branches.',
		next_goal: nextGoal,
		action,
	}
}

function findElementIndex(prompt, terms) {
	const lines = prompt.split(/\n/)
	for (const line of lines) {
		const match = /^\*?\[(\d+)\]<([^>]*)>(.*)$/.exec(line.trim())
		if (!match) continue
		const lower = line.toLowerCase()
		if (terms.every((term) => lower.includes(term.toLowerCase()))) {
			return Number(match[1])
		}
	}
	for (const line of lines) {
		const match = /^\*?\[(\d+)\]<([^>]*)>(.*)$/.exec(line.trim())
		if (!match) continue
		const lower = line.toLowerCase()
		if (terms.some((term) => lower.includes(term.toLowerCase()))) {
			return Number(match[1])
		}
	}
	return 0
}

async function waitForExtensionLoaded(context, extensionID, channel) {
	const expectedPrefix = `chrome-extension://${extensionID}/`
	for (let i = 0; i < 30; i += 1) {
		if (context.serviceWorkers().some((worker) => worker.url().startsWith(expectedPrefix))) {
			return
		}
		await new Promise((resolve) => setTimeout(resolve, 500))
	}
	throw new Error(
		`Extension ${extensionID} was not loaded by ${channel}. ` +
			'Use the bundled Chromium default or a browser channel that still supports unpacked extension loading.'
	)
}

async function runTask(sidepanel, task) {
	const textarea = sidepanel.locator('textarea')
	await textarea.waitFor({ state: 'visible', timeout: 30_000 })
	await textarea.fill(task)
	await sidepanel.getByRole('button', { name: /Send|发送/ }).click()
	await waitForStoredSession(sidepanel, task)
}

async function waitForStoredSession(extensionPage, expectedTask) {
	await extensionPage.evaluate(async (expectedTask) => {
		for (let i = 0; i < 180; i += 1) {
			const state = await chrome.storage.local.get(['isAgentRunning', 'lastWebOpsSession'])
			if (
				!state.isAgentRunning &&
				state.lastWebOpsSession &&
				state.lastWebOpsSession.task === expectedTask
			) {
				return
			}
			await new Promise((resolve) => setTimeout(resolve, 500))
		}
		throw new Error(`Timed out waiting for lastWebOpsSession: ${expectedTask}`)
	}, expectedTask)
}

async function main() {
	assert(fs.existsSync(extensionPath), `Extension path not found: ${extensionPath}`)
	const extensionID = extensionIDFromManifest(extensionPath)

	const businessPageServer = createBusinessPageServer()
	const businessURL = await listen(businessPageServer)
	const mockLLM = createMockLLMServer()
	const mockLLMBaseURL = await listen(mockLLM.server)
	const llmBaseURL = configuredLLMBaseURL ?? `${mockLLMBaseURL}/v1`
	const llmApiKey = configuredLLMApiKey ?? 'mock-key'

	let context
	try {
		await seedDocuments()
		const launchOptions = {
			headless: false,
			viewport: { width: 1360, height: 920 },
			args: [
				`--disable-extensions-except=${extensionPath}`,
				`--load-extension=${extensionPath}`,
				'--disable-web-security',
			],
		}
		if (browserChannel !== 'chromium') {
			launchOptions.channel = browserChannel
		}
		context = await chromium.launchPersistentContext(userDataDir, launchOptions)
		await waitForExtensionLoaded(context, extensionID, browserChannel)
		const setupPage = await context.newPage()
		await setupPage.goto(`chrome-extension://${extensionID}/hub.html`)
		await setupPage.evaluate(
			async ({ llmBaseURL, llmModel, llmApiKey, backendURL, projectID }) => {
				await chrome.storage.local.set({
					llmConfig: {
						baseURL: llmBaseURL,
						model: llmModel,
						apiKey: llmApiKey,
					},
					language: 'zh-CN',
					advancedConfig: {
						maxSteps: 5,
					},
					knowledgeSettings: {
						enabled: true,
						allowPageSummary: true,
						baseUrl: backendURL,
						projectKey: projectID,
					},
					workflowBackend: {
						baseUrl: backendURL,
						projectId: projectID,
					},
				})
			},
			{ llmBaseURL, llmModel, llmApiKey, backendURL, projectID }
		)
		await setupPage.close()

		const businessPage = await context.newPage()
		await businessPage.goto(`${businessURL}/services`)
		await businessPage.bringToFront()
		const sidepanel = await context.newPage()
		await sidepanel.goto(`chrome-extension://${extensionID}/sidepanel.html`)
		await sidepanel.waitForSelector('textarea', { timeout: 30_000 })

		await runTask(sidepanel, '查询 kme-prod-001 服务状态')
		await sidepanel.getByRole('button', { name: '新建会话' }).click()
		await businessPage.bringToFront()
		await sidepanel.bringToFront()

		await runTask(sidepanel, '查询 kme-prod-002 服务状态')
		await sidepanel.getByRole('button', { name: '新建会话' }).click()
		await businessPage.bringToFront()
		await sidepanel.bringToFront()

		await runTask(sidepanel, '失败场景：查询 kme-prod-fail 服务状态')
		await sidepanel.getByRole('button', { name: '新建会话' }).click()
		await businessPage.bringToFront()
		await sidepanel.bringToFront()

		await runTask(sidepanel, '失败场景复测：查询 kme-prod-fail 服务状态')

		const inspector = await get(`/api/memory/inspector?projectId=${encodeURIComponent(projectID)}`)
		const lastStorage = await sidepanel.evaluate(async () =>
			chrome.storage.local.get(['workflowBackend', 'lastWebOpsSession'])
		)
		const pageStatus = await businessPage.evaluate(() => window.__pageAgentServiceStatus ?? null)

		await sidepanel.screenshot({ path: screenshotPath, fullPage: true })
		await businessPage.screenshot({ path: pageScreenshotPath, fullPage: true })
		fs.writeFileSync(inspectorPath, JSON.stringify(inspector, null, 2))

		const requestSummaries = mockLLM.requestSummaries
		const report = {
			projectID,
			backendURL,
			artifactDir,
			extensionID,
			extensionPath,
			browserChannel,
			businessURL,
			llmMode: configuredLLMBaseURL ? 'configured' : 'mock',
			pageStatus,
			storedWorkflowBackend: {
				baseUrl: lastStorage.workflowBackend?.baseUrl,
				projectId: lastStorage.workflowBackend?.projectId,
			},
			lastSession: summarizeSession(lastStorage.lastWebOpsSession),
			llmRequestSummaries: requestSummaries,
			inspectorCounts: {
				pageStates: inspector.pageStates?.length ?? 0,
				transitions: inspector.transitions?.length ?? 0,
				experiences: inspector.experiences?.length ?? 0,
				failures: inspector.failures?.length ?? 0,
				workflows: inspector.workflows?.length ?? 0,
			},
		}
		fs.writeFileSync(reportPath, JSON.stringify(report, null, 2))

		assert(
			report.storedWorkflowBackend.baseUrl === backendURL,
			'extension should store Memory Backend URL'
		)
		assert(
			report.storedWorkflowBackend.projectId === projectID,
			'extension should store project ID'
		)
		assert(pageStatus?.status === 'running', 'business page should show queried service status')
		assert(
			requestSummaries.some((item) => item.hasWebOpsMemory),
			'agent should receive webops memory'
		)
		assert(
			requestSummaries.some((item) => item.hasCurrentPage),
			'memory should include current page'
		)
		assert(
			requestSummaries.some((item) => item.hasKnowledgeEvidence),
			'memory should include knowledge evidence'
		)
		assert(inspector.experiences?.length >= 1, 'backend should save experience memory')
		assert(
			requestSummaries.some(
				(item) => item.taskKind === 'second_success' && item.hasExperienceHints
			),
			'second task should receive experience hints'
		)
		assert(inspector.failures?.length >= 1, 'backend should save failure memory')
		assert(
			requestSummaries.some((item) => item.taskKind === 'failure' && item.hasFailureWarnings),
			'failure task should receive failure warning'
		)

		console.log(
			JSON.stringify({ ok: true, artifactDir, reportPath, inspectorPath, screenshotPath }, null, 2)
		)
	} finally {
		await context?.close().catch(() => {})
		await closeServer(businessPageServer).catch(() => {})
		await closeServer(mockLLM.server).catch(() => {})
	}
}

function extensionIDFromManifest(extensionPath) {
	const manifestPath = path.join(extensionPath, 'manifest.json')
	const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'))
	assert(manifest.key, 'Extension manifest must include a stable key for E2E loading.')
	const der = Buffer.from(manifest.key, 'base64')
	const hex = crypto.createHash('sha256').update(der).digest('hex').slice(0, 32)
	return hex
		.split('')
		.map((char) => String.fromCharCode('a'.charCodeAt(0) + Number.parseInt(char, 16)))
		.join('')
}

async function seedDocuments() {
	await post('/api/memory/documents', {
		projectId: projectID,
		documents: [
			{
				id: `doc_service_${scenarioID}`,
				title: '服务管理手册',
				source: 'manual',
				url: 'http://127.0.0.1/services',
				content:
					'# 服务管理\n服务管理页通过服务名称搜索框查询服务。搜索后服务详情展示运行状态、负责人和部署记录。',
				tags: ['服务管理', 'status'],
			},
		],
	})
}

function summarizeSession(session) {
	if (!session) return null
	return {
		id: session.id,
		task: session.task,
		stepCount: Array.isArray(session.steps) ? session.steps.length : 0,
		memoryUpdates: session.memoryUpdates ?? [],
		knowledgeHitCount: Array.isArray(session.knowledgeHits) ? session.knowledgeHits.length : 0,
	}
}

main().catch((error) => {
	console.error(error)
	process.exit(1)
})
