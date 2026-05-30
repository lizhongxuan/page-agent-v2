import crypto from 'node:crypto'
import fs from 'node:fs'
import http from 'node:http'
import path from 'node:path'
import { chromium } from 'playwright'

const backendURL = process.env.WEBOPS_MEMORY_V2_BACKEND_URL ?? 'http://127.0.0.1:38434'
const artifactDir = path.resolve(
	process.env.WEBOPS_MEMORY_V2_COMPLEX_ARTIFACT_DIR ?? 'artifacts/webops-memory-v2-complex/manual'
)
const extensionPath = path.resolve(
	process.env.WEBOPS_MEMORY_V2_EXTENSION_PATH ??
		'/Users/lizhongxuan/Desktop/page-agent-ext-1.8.2-chrome'
)
const browserChannel = process.env.PLAYWRIGHT_CHROME_CHANNEL ?? 'chromium'
const llmModel = process.env.WEBOPS_MEMORY_V2_LLM_MODEL ?? 'mock-page-agent-complex-effect'

fs.mkdirSync(artifactDir, { recursive: true })

const scenarioID = `wmv2_complex_${Date.now().toString(36)}`
const projectID = `project_${scenarioID}`
const userDataDir = path.join(artifactDir, 'chrome-profile')
const reportPath = path.join(artifactDir, 'report.json')
const processLogPath = path.join(artifactDir, 'process-log.md')
const inspectorPath = path.join(artifactDir, 'memory-inspector.json')
const eventsPath = path.join(artifactDir, 'process-events.json')
const injectedMemoryPath = path.join(artifactDir, 'memory-context-injected.md')
const injectedMemoryJSONPath = path.join(artifactDir, 'memory-context-injected.json')
const screenshotPath = path.join(artifactDir, 'chrome-user-flow.png')
const htmlReportPath = path.join(artifactDir, 'user-flow-report.html')
const dbSnapshotPath = path.join(artifactDir, 'backend-saved-data.json')
const memoryContextBeforePath = path.join(artifactDir, 'memory-context-before.json')
const taskRunUploadPath = path.join(artifactDir, 'task-run-upload.json')
const attributionEventsPath = path.join(artifactDir, 'attribution-events.json')
const memoryContextAfterPath = path.join(artifactDir, 'memory-context-after.json')

const events = []

function assert(condition, message) {
	if (!condition) throw new Error(message)
}

function record(event, details = {}) {
	events.push({ at: new Date().toISOString(), event, ...details })
}

function writeJSON(filePath, value) {
	fs.writeFileSync(filePath, JSON.stringify(value, null, 2))
}

function firstArray(source, keys) {
	for (const key of keys) {
		if (Array.isArray(source?.[key])) return source[key]
	}
	return []
}

function lastItem(items) {
	return Array.isArray(items) && items.length > 0 ? items[items.length - 1] : null
}

function escapeHTML(value) {
	return String(value)
		.replaceAll('&', '&amp;')
		.replaceAll('<', '&lt;')
		.replaceAll('>', '&gt;')
		.replaceAll('"', '&quot;')
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

async function tryPost(pathname, body, fallback) {
	try {
		return await post(pathname, body)
	} catch (error) {
		return {
			ok: false,
			pathname,
			error: String(error?.message ?? error),
			fallback,
		}
	}
}

async function fetchInspectorSnapshot() {
	const inspector = await get(`/api/memory/inspector?projectId=${encodeURIComponent(projectID)}`)
	const attributionEvents = firstArray(inspector, [
		'attributionEvents',
		'memory_attribution_events',
	])
	const evidenceStats = firstArray(inspector, ['evidenceStats', 'memory_evidence_stats'])
	return {
		projectID,
		fetchedAt: new Date().toISOString(),
		compatibility: {
			hasAttributionEvents: attributionEvents.length > 0,
			hasEvidenceStats: evidenceStats.length > 0,
			notes: [
				attributionEvents.length === 0
					? 'Inspector did not expose attributionEvents; recorded empty array for compatibility.'
					: null,
				evidenceStats.length === 0
					? 'Inspector did not expose evidenceStats; recorded empty array for compatibility.'
					: null,
			].filter(Boolean),
		},
		...inspector,
		attributionEvents,
		evidenceStats,
	}
}

async function captureMemoryContextSnapshot(label) {
	const body = {
		projectId: projectID,
		task: '复杂任务D：确认 checkout-api 发布冻结窗口，不执行回滚',
		currentUrl: 'http://127.0.0.1/console/deployments',
		pageObservation: {
			title: '发布管理 - WebOps 控制台',
			visibleText: ['发布管理', '发布服务名称', '查询发布'],
			controls: [
				{ role: 'textbox', name: '发布服务名称' },
				{ role: 'button', name: '查询发布' },
			],
		},
		riskPolicy: { blocked: ['destructive'] },
		mode: 'before_task',
	}
	const response = await tryPost('/api/memory/context', body, `${label} snapshot unavailable`)
	return {
		label,
		capturedAt: new Date().toISOString(),
		request: body,
		response,
		hasContextPrompt: typeof response?.contextPrompt === 'string',
		evidenceRefs: Array.isArray(response?.evidenceRefs) ? response.evidenceRefs : [],
	}
}

async function runAttributionEffectProbe() {
	const pageObservation = await post('/api/memory/page-observations', {
		projectId: projectID,
		task: 'Attribution probe',
		url: 'http://probe.local/probe-start',
		title: 'Probe start',
		visibleText: ['Probe start', 'Open archive branch', 'Open target page'],
		controls: [
			{ role: 'button', name: 'Open archive branch' },
			{ role: 'button', name: 'Open target page' },
		],
		source: 'attribution_probe',
	})
	const pageA = pageObservation.pageStateId
	assert(pageA, 'attribution probe must create a start page state')
	const pageB = `${pageA}_wrong_b`
	const pageD = `${pageA}_correct_d`
	const correctSeed = await post('/api/memory/task-runs', {
		id: `probe_correct_seed_${scenarioID}`,
		projectId: projectID,
		site: 'probe.local',
		taskTemplate: 'Probe choose correct path direct',
		summary: 'Correct path opens the target page directly.',
		originalPath: [pageA, pageD],
		optimizedPath: [pageA, pageD],
		status: 'success',
		actionSteps: [
			{
				id: 'probe_correct_step',
				pageStateId: pageA,
				stepIndex: 1,
				actionType: 'click',
				targetName: 'Open target page',
				resultSummary: 'Opened the target page.',
			},
		],
	})
	const wrongSeed = await post('/api/memory/task-runs', {
		id: `probe_wrong_seed_${scenarioID}`,
		projectId: projectID,
		site: 'probe.local',
		taskTemplate: 'Probe choose correct path wrong legacy',
		summary: 'Wrong historical path opens the archive branch first.',
		originalPath: [pageA, pageB],
		optimizedPath: [pageA, pageB],
		status: 'success',
		actionSteps: [
			{
				id: 'probe_wrong_step',
				pageStateId: pageA,
				stepIndex: 1,
				actionType: 'click',
				targetName: 'Open archive branch',
				resultSummary: 'Opened the archive branch.',
			},
		],
	})
	assert(correctSeed.experienceId, 'correct probe seed must create an experience')
	assert(wrongSeed.experienceId, 'wrong probe seed must create an experience')
	const contextRequest = {
		projectId: projectID,
		task: 'Probe choose correct path',
		currentPageStateId: pageA,
		mode: 'before_task',
	}
	const before = await post('/api/memory/context', contextRequest)
	const beforeExperienceIDs = (before.evidenceRefs ?? [])
		.filter((ref) => ref.source === 'experience')
		.map((ref) => ref.id)
	assert(
		beforeExperienceIDs.includes(correctSeed.experienceId) &&
			beforeExperienceIDs.includes(wrongSeed.experienceId),
		`probe context should recall both experiences, got ${JSON.stringify(beforeExperienceIDs)}`
	)
	const attributionRun = await post('/api/memory/task-runs', {
		id: `probe_attribution_run_${scenarioID}`,
		projectId: projectID,
		site: 'probe.local',
		taskTemplate: 'Probe choose correct path',
		summary: 'The agent tried the archive branch, returned to start, then opened the target page.',
		originalPath: [pageA, pageB, pageA, pageD],
		optimizedPath: [pageA, pageD],
		memoryContextId: before.contextId,
		memoryEvidenceRefs: before.evidenceRefs,
		status: 'success',
		actionSteps: [
			{
				id: 'probe_wrong_branch',
				pageStateId: pageA,
				stepIndex: 1,
				actionType: 'click',
				targetName: 'Open archive branch',
				resultSummary: 'Opened the archive branch and hit a dead end.',
				isBranchNoise: true,
			},
			{
				id: 'probe_correct_branch',
				pageStateId: pageA,
				stepIndex: 2,
				actionType: 'click',
				targetName: 'Open target page',
				resultSummary: 'Opened the target page.',
			},
		],
	})
	const after = await post('/api/memory/context', contextRequest)
	const afterExperienceIDs = (after.evidenceRefs ?? [])
		.filter((ref) => ref.source === 'experience')
		.map((ref) => ref.id)
	const inspector = await fetchInspectorSnapshot()
	const wrongEvent = (inspector.attributionEvents ?? []).find(
		(event) =>
			event.taskRunId === `probe_attribution_run_${scenarioID}` &&
			event.evidenceId === wrongSeed.experienceId
	)
	const correctEvent = (inspector.attributionEvents ?? []).find(
		(event) =>
			event.taskRunId === `probe_attribution_run_${scenarioID}` &&
			event.evidenceId === correctSeed.experienceId
	)
	const wrongStats = (inspector.evidenceStats ?? []).find(
		(stats) => stats.evidenceId === wrongSeed.experienceId
	)
	const correctStats = (inspector.evidenceStats ?? []).find(
		(stats) => stats.evidenceId === correctSeed.experienceId
	)
	assert(
		wrongEvent?.label === 'misleading',
		`wrong probe evidence should be misleading: ${JSON.stringify(wrongEvent)}`
	)
	assert(
		correctEvent?.label === 'helpful',
		`correct probe evidence should be helpful: ${JSON.stringify(correctEvent)}`
	)
	assert(
		afterExperienceIDs.indexOf(correctSeed.experienceId) > -1 &&
			afterExperienceIDs.indexOf(wrongSeed.experienceId) > -1 &&
			afterExperienceIDs.indexOf(correctSeed.experienceId) <
				afterExperienceIDs.indexOf(wrongSeed.experienceId),
		`correct probe evidence should rank ahead of wrong evidence after attribution: ${JSON.stringify(afterExperienceIDs)}`
	)
	return {
		pageA,
		pageB,
		pageD,
		correctExperienceId: correctSeed.experienceId,
		wrongExperienceId: wrongSeed.experienceId,
		before,
		attributionRun,
		after,
		wrongEvent,
		correctEvent,
		wrongStats,
		correctStats,
		afterExperienceIDs,
	}
}

function buildTaskRunUploadArtifact(inspectorSnapshot, scenario) {
	const taskRuns = firstArray(inspectorSnapshot, ['taskRuns', 'task_runs'])
	const scenarioSessionID = scenario?.lastSession?.id
	const matchedTaskRun =
		taskRuns.find((run) => (run.id ?? run.taskRunId) === scenarioSessionID) ??
		taskRuns.find((run) => run.taskTemplate === scenario?.task) ??
		lastItem(taskRuns)
	if (matchedTaskRun) {
		return {
			capturedAt: new Date().toISOString(),
			source: 'inspector-matched-task-run',
			selectedBy: scenarioSessionID ? 'scenario-session-id' : 'scenario-task-template-or-last',
			taskRunId: matchedTaskRun.id ?? matchedTaskRun.taskRunId ?? null,
			memoryContextId: matchedTaskRun.memoryContextId ?? matchedTaskRun.memory_context_id ?? null,
			memoryEvidenceRefs:
				matchedTaskRun.memoryEvidenceRefs ?? matchedTaskRun.memory_evidence_refs ?? [],
			status: matchedTaskRun.status ?? null,
			summary: matchedTaskRun.summary ?? null,
			compatibility: {
				note: 'Captured from inspector snapshot because the extension upload request is not intercepted directly in this E2E.',
			},
			taskRun: matchedTaskRun,
			allTaskRunSummaries: taskRuns.map((run) => ({
				id: run.id ?? run.taskRunId ?? null,
				taskTemplate: run.taskTemplate ?? null,
				status: run.status ?? null,
				summary: run.summary ?? null,
				memoryContextId: run.memoryContextId ?? run.memory_context_id ?? null,
			})),
		}
	}
	return {
		capturedAt: new Date().toISOString(),
		source: 'scenario-session-summary-fallback',
		taskRunId: null,
		memoryContextId: null,
		memoryEvidenceRefs: [],
		status: scenario?.lastSession ? 'unknown' : null,
		summary: scenario?.lastSession ?? null,
		compatibility: {
			note: 'Inspector did not expose task runs, so this artifact contains only the sanitized session summary fallback.',
		},
	}
}

function buildBackendSavedDataSnapshot(inspectorSnapshot, seededKnowledge) {
	const inspectorBusinessProfile = inspectorSnapshot.businessProfile ?? null
	const seededBusinessProfile = seededKnowledge?.response?.businessProfile ?? null
	const businessProfile = inspectorBusinessProfile?.projectId
		? inspectorBusinessProfile
		: seededBusinessProfile
	return {
		projectID,
		savedAt: new Date().toISOString(),
		source: 'memory-inspector',
		compatibility: {
			note: 'Runtime memory comes from GET /api/memory/inspector. Seeded knowledge includes the document request and /api/memory/documents response because the inspector does not list raw knowledge documents.',
		},
		seededKnowledge: seededKnowledge ?? null,
		knowledgeDocuments: seededKnowledge?.request?.documents ?? [],
		knowledgeDocumentIds: seededKnowledge?.response?.documentIds ?? [],
		businessProfile: businessProfile ?? null,
		businessProfiles: businessProfile
			? [businessProfile]
			: firstArray(inspectorSnapshot, ['businessProfiles', 'business_profiles']),
		pageStates: firstArray(inspectorSnapshot, ['pageStates', 'page_states']),
		pageTransitions: firstArray(inspectorSnapshot, ['pageTransitions', 'page_transitions']),
		experiences: firstArray(inspectorSnapshot, ['experiences', 'experience_memories']),
		failures: firstArray(inspectorSnapshot, ['failures', 'failure_memories']),
		workflows: firstArray(inspectorSnapshot, ['workflows', 'workflow_recipes']),
		memoryContextEvents: firstArray(inspectorSnapshot, [
			'memoryContextEvents',
			'memory_context_events',
		]),
		taskRuns: firstArray(inspectorSnapshot, ['taskRuns', 'task_runs']),
		attributionEvents: firstArray(inspectorSnapshot, [
			'attributionEvents',
			'memory_attribution_events',
		]),
		evidenceStats: firstArray(inspectorSnapshot, ['evidenceStats', 'memory_evidence_stats']),
		rawInspector: inspectorSnapshot,
	}
}

function createBusinessPageServer() {
	return http.createServer((request, response) => {
		if (!request.url?.startsWith('/console')) {
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
  <title>WebOps 控制台</title>
  <style>
    body { font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 0; background: #f5f7fb; color: #172033; }
    header { display: flex; align-items: center; justify-content: space-between; padding: 18px 28px; background: #182033; color: #fff; }
    nav { display: flex; gap: 10px; padding: 14px 28px; background: #fff; border-bottom: 1px solid #d7deea; }
    button { padding: 9px 13px; border: 1px solid #2f63d6; border-radius: 6px; background: #2f63d6; color: #fff; cursor: pointer; font: inherit; }
    button.secondary { background: #fff; color: #24324a; border-color: #b9c5d8; }
    button.danger { background: #b42318; border-color: #b42318; }
    main { padding: 24px 28px; max-width: 1120px; }
    .panel { background: #fff; border: 1px solid #d7deea; border-radius: 8px; padding: 18px; margin-bottom: 16px; }
    .grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px; }
    label { display: inline-flex; gap: 8px; align-items: center; margin-right: 10px; }
    input { padding: 9px 10px; border: 1px solid #aab6c8; border-radius: 6px; font: inherit; min-width: 220px; }
    .metric { display: inline-flex; gap: 6px; padding: 4px 8px; border-radius: 999px; background: #edf7ee; color: #166534; margin-right: 8px; }
    .warning { color: #b42318; font-weight: 650; }
    .muted { color: #667085; }
  </style>
</head>
<body>
<header>
  <h1>WebOps 控制台</h1>
  <span>生产环境 · ap-southeast-1</span>
</header>
<nav aria-label="主导航">
  <button id="navServices" class="secondary" aria-label="服务管理">服务管理</button>
  <button id="navIncidents" class="secondary" aria-label="事件中心">事件中心</button>
  <button id="navDeployments" class="secondary" aria-label="发布管理">发布管理</button>
  <button id="navRunbook" class="secondary" aria-label="运行手册">运行手册</button>
</nav>
<main id="app"></main>
<script>
  const metricsKey = '__webops_complex_metrics';
  const defaultMetrics = {
    serviceQueries: 0,
    wrongIncidentVisits: 0,
    rollbackClicks: 0,
    freezeChecks: 0,
    lastService: '',
    lastView: '',
  };
  function loadMetrics() {
    try {
      return { ...defaultMetrics, ...JSON.parse(localStorage.getItem(metricsKey) || '{}') };
    } catch {
      return { ...defaultMetrics };
    }
  }
  function saveMetrics() {
    localStorage.setItem(metricsKey, JSON.stringify(window.__webops));
  }
  function bumpMetric(key) {
    window.__webops[key] = (window.__webops[key] || 0) + 1;
    saveMetrics();
  }
  function setMetric(key, value) {
    window.__webops[key] = value;
    saveMetrics();
  }
  window.__webops = loadMetrics();

  const app = document.getElementById('app');
  const services = {
    'checkout-api': { owner: 'Team Checkout', status: 'degraded', errorRate: '4.8%', latency: '820ms' },
    'payment-api': { owner: 'Team Payments', status: 'warning', errorRate: '2.1%', latency: '430ms' },
  };

  function setView(view) {
    setMetric('lastView', view);
    if (view === 'services') renderServices();
    else if (view === 'incidents') renderIncidents();
    else if (view === 'deployments') renderDeployments();
    else if (view === 'runbook') renderRunbook();
    else renderHome();
  }

  function renderHome() {
    document.title = 'WebOps 控制台首页';
    app.innerHTML = '<section class="panel"><h2>WebOps 控制台首页</h2><p>请选择业务模块。服务健康巡检从服务管理进入；发布冻结确认从发布管理进入。</p></section>' +
      '<section class="grid">' +
      '<article class="panel"><h3>服务管理</h3><p>查询服务画像、错误率、负责人和健康状态。</p></article>' +
      '<article class="panel"><h3>事件中心</h3><p>用于事件列表筛选，不适合直接查询服务画像。</p></article>' +
      '<article class="panel"><h3>发布管理</h3><p>查看发布批次、冻结窗口和回滚风险。</p></article>' +
      '</section>';
  }

  function renderIncidents() {
    document.title = '事件中心 - WebOps 控制台';
    bumpMetric('wrongIncidentVisits');
    app.innerHTML = '<section class="panel"><h2>事件中心</h2><p>这里按事件号和告警级别筛选，无法直接查看服务负责人。</p>' +
      '<label>事件类型 <input aria-label="事件类型" placeholder="incident type" /></label></section>';
  }

  function renderServices() {
    document.title = '服务管理 - WebOps 控制台';
    app.innerHTML = '<section class="panel"><h2>服务管理</h2><label>服务名称 <input id="serviceName" aria-label="服务名称" placeholder="service name" /></label>' +
      '<button id="queryService" aria-label="查询服务">查询服务</button> ' +
      '<button id="quickCheckout" class="secondary" aria-label="查询 checkout-api">查询 checkout-api</button> ' +
      '<button id="quickPayment" class="secondary" aria-label="查询 payment-api">查询 payment-api</button></section>' +
      '<section id="serviceResult" class="panel">请输入服务名称查询服务画像。</section>';
    function queryService(name) {
      const service = services[name] || { owner: 'Unknown', status: 'unknown', errorRate: '0%', latency: '0ms' };
      bumpMetric('serviceQueries');
      setMetric('lastService', name);
      document.getElementById('serviceResult').innerHTML =
        '<h2>服务画像</h2>' +
        '<p>服务名称：' + name + '</p>' +
        '<p>运行状态：<span class="metric">' + service.status + '</span></p>' +
        '<p>错误率：<span class="metric">' + service.errorRate + '</span></p>' +
        '<p>P95 延迟：' + service.latency + '</p>' +
        '<p>负责人：' + service.owner + '</p>';
    }
    document.getElementById('queryService').addEventListener('click', () => {
      queryService(document.getElementById('serviceName').value || 'checkout-api');
    });
    document.getElementById('quickCheckout').addEventListener('click', () => queryService('checkout-api'));
    document.getElementById('quickPayment').addEventListener('click', () => queryService('payment-api'));
  }

  function renderDeployments() {
    document.title = '发布管理 - WebOps 控制台';
    app.innerHTML = '<section class="panel"><h2>发布管理</h2><label>发布服务名称 <input id="deployService" aria-label="发布服务名称" placeholder="service name" /></label>' +
      '<button id="queryDeploy" aria-label="查询发布">查询发布</button></section><section id="deployResult" class="panel">请输入服务名称查询发布批次。</section>';
    document.getElementById('queryDeploy').addEventListener('click', () => {
      const name = document.getElementById('deployService').value || 'checkout-api';
      document.getElementById('deployResult').innerHTML =
        '<h2>当前发布</h2><p>服务名称：' + name + '</p><p>发布批次：release-2026-05-30-17</p>' +
        '<p>状态：canary 观察中</p><button id="freezeWindow" aria-label="查看冻结窗口">查看冻结窗口</button> ' +
        '<button id="rollbackNow" class="danger" aria-label="立即回滚">立即回滚</button><div id="deployMessage"></div>';
      document.getElementById('freezeWindow').addEventListener('click', () => {
        bumpMetric('freezeChecks');
        document.getElementById('deployMessage').innerHTML = '<p>冻结窗口：22:00-23:00，只读确认完成，未执行回滚。</p>';
      });
      document.getElementById('rollbackNow').addEventListener('click', () => {
        bumpMetric('rollbackClicks');
        document.getElementById('deployMessage').innerHTML = '<p class="warning">危险操作已拦截：立即回滚需要人工审批。</p>';
      });
    });
  }

  function renderRunbook() {
    document.title = '运行手册 - WebOps 控制台';
    app.innerHTML = '<section class="panel"><h2>运行手册</h2><p>服务巡检手册：服务管理 -> 输入服务名称 -> 查询服务。</p><p>发布安全手册：发布管理 -> 查询发布 -> 查看冻结窗口。</p></section>';
  }

  document.getElementById('navServices').addEventListener('click', () => { location.href = '/console/services'; });
  document.getElementById('navIncidents').addEventListener('click', () => { location.href = '/console/incidents'; });
  document.getElementById('navDeployments').addEventListener('click', () => { location.href = '/console/deployments'; });
  document.getElementById('navRunbook').addEventListener('click', () => { location.href = '/console/runbook'; });
  const pathView = location.pathname.split('/').filter(Boolean).at(-1);
  setView(pathView === 'console' ? 'home' : (pathView || 'home'));
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
		const action = chooseAction(prompt)
		const summary = summarizePrompt(prompt, action)
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

function summarizePrompt(prompt, actionOutput) {
	const actionNames = listActionNames(actionOutput)
	const memoryContext = extractWebOpsMemory(prompt)
	return {
		taskKind: taskKind(prompt),
		taskText: taskText(prompt),
		hasWebOpsMemory: memoryContext !== '',
		hasCurrentPage: prompt.includes('<current_page'),
		hasKnowledgeEvidence: prompt.includes('<knowledge_evidence>'),
		hasExperienceHints: prompt.includes('<experience_hints>'),
		hasFailureWarnings: prompt.includes('<failure_warnings>'),
		hasServiceResult: hasServiceResult(prompt),
		hasFreezeResult: prompt.includes('冻结窗口：22:00-23:00'),
		actionNames,
		nextAction: actionNames.join('+'),
		nextGoal: actionOutput.next_goal,
		actionInput: actionOutput.action,
		indexedElements: indexedElementLines(prompt).slice(0, 40),
		memoryContext,
		memoryContextLength: memoryContext.length,
	}
}

function chooseAction(prompt) {
	const kind = taskKind(prompt)
	if (kind === 'service_triage') return chooseServiceTriageAction(prompt, 'checkout-api')
	if (kind === 'experience_reuse') return chooseServiceTriageAction(prompt, 'payment-api')
	if (kind === 'rollback_failure_seed') return chooseRollbackSeedAction(prompt)
	if (kind === 'rollback_warning_check') return chooseRollbackWarningAction(prompt)
	return macroOutput('任务已完成。', { done: { text: '任务已完成。', success: true } })
}

function chooseServiceTriageAction(prompt, serviceName) {
	const hasMemory = prompt.includes('<webops_memory>')
	const onIncidentPage =
		hasElement(prompt, ['事件类型', 'input']) || prompt.includes('aria-label=事件类型')
	const onServicePage = prompt.includes('id=serviceName') && prompt.includes('id=queryService')
	if (hasServiceResult(prompt, serviceName)) {
		return macroOutput('读取服务画像并完成。', {
			done: { text: `${serviceName} 已找到错误率、运行状态和负责人。`, success: true },
		})
	}
	if (onServicePage) {
		if (!prompt.includes(`Input text (${serviceName})`)) {
			return macroOutput('输入服务名称并提交查询服务画像。', [
				{ input_text: { index: serviceInputIndex(prompt), text: serviceName } },
				{ click_element_by_index: { index: findElementIndex(prompt, ['查询服务', 'button']) } },
			])
		}
		return macroOutput('提交服务画像查询。', {
			click_element_by_index: { index: findElementIndex(prompt, ['查询服务', 'button']) },
		})
	}
	if (onIncidentPage) {
		return macroOutput('事件中心不适合服务画像查询，返回服务管理。', {
			click_element_by_index: { index: findElementIndex(prompt, ['服务管理']) },
		})
	}
	if (!hasMemory && hasElement(prompt, ['事件中心']) && prompt.includes('WebOps 控制台首页')) {
		return macroOutput('无知识库时先探索事件中心。', {
			click_element_by_index: { index: findElementIndex(prompt, ['事件中心']) },
		})
	}
	return macroOutput('进入服务管理模块。', {
		click_element_by_index: { index: findElementIndex(prompt, ['服务管理']) },
	})
}

function chooseRollbackSeedAction(prompt) {
	if (!hasElement(prompt, ['查询发布', 'button'])) {
		return macroOutput('进入发布管理做风险演练。', {
			click_element_by_index: { index: findElementIndex(prompt, ['发布管理']) },
		})
	}
	if (!prompt.includes('当前发布')) {
		if (!prompt.includes('Input text (checkout-api)')) {
			return macroOutput('输入发布服务名称。', {
				input_text: { index: deploymentInputIndex(prompt), text: 'checkout-api' },
			})
		}
		return macroOutput('查询发布批次。', {
			click_element_by_index: { index: findElementIndex(prompt, ['查询发布', 'button']) },
		})
	}
	if (!prompt.includes('危险操作已拦截')) {
		return macroOutput('复现错误分支：点击立即回滚。', {
			click_element_by_index: { index: findElementIndex(prompt, ['立即回滚', 'button']) },
		})
	}
	return macroOutput('记录失败路径。', {
		done: { text: 'checkout-api 发布回滚失败，避免立即回滚。', success: false },
	})
}

function chooseRollbackWarningAction(prompt) {
	if (!hasElement(prompt, ['查询发布', 'button'])) {
		return macroOutput('进入发布管理，只做只读确认。', {
			click_element_by_index: { index: findElementIndex(prompt, ['发布管理']) },
		})
	}
	if (!prompt.includes('当前发布')) {
		if (!prompt.includes('Input text (checkout-api)')) {
			return macroOutput('输入发布服务名称。', {
				input_text: { index: deploymentInputIndex(prompt), text: 'checkout-api' },
			})
		}
		return macroOutput('查询发布批次。', {
			click_element_by_index: { index: findElementIndex(prompt, ['查询发布', 'button']) },
		})
	}
	if (!prompt.includes('冻结窗口：22:00-23:00')) {
		if (prompt.includes('<failure_warnings>')) {
			return macroOutput('根据失败 warning 避免立即回滚，改为查看冻结窗口。', {
				click_element_by_index: { index: findElementIndex(prompt, ['查看冻结窗口', 'button']) },
			})
		}
		if (prompt.includes('危险操作已拦截')) {
			return macroOutput('缺少失败 warning，错误分支已复现。', {
				done: { text: '缺少失败 warning 时误入回滚分支。', success: false },
			})
		}
		return macroOutput('没有失败 warning 时可能误点立即回滚。', {
			click_element_by_index: { index: findElementIndex(prompt, ['立即回滚', 'button']) },
		})
	}
	return macroOutput('冻结窗口已只读确认。', {
		done: { text: '已确认冻结窗口，未执行回滚。', success: true },
	})
}

function taskKind(prompt) {
	const task = taskText(prompt)
	if (task.includes('复杂任务C') || task.includes('风险演练：回滚')) return 'rollback_failure_seed'
	if (task.includes('复杂任务D') || task.includes('发布冻结窗口')) return 'rollback_warning_check'
	if (task.includes('复杂任务B') || task.includes('payment-api')) return 'experience_reuse'
	if (task.includes('复杂任务A') || (task.includes('checkout-api') && task.includes('错误率'))) {
		return 'service_triage'
	}
	return 'unknown'
}

function taskText(prompt) {
	const taskMatch =
		/(?:Task|任务|User task|用户任务)[^\n:：]*[:：]\s*([^\n]+)/i.exec(prompt) ??
		/(复杂任务[A-D][^\n]+)/.exec(prompt)
	return taskMatch?.[1]?.trim() ?? ''
}

function hasServiceResult(prompt, serviceName = '') {
	const hasResultFields = prompt.includes('负责人：') && prompt.includes('P95 延迟：')
	return hasResultFields && (serviceName === '' || prompt.includes(`服务名称：${serviceName}`))
}

function listActionNames(actionOutput) {
	const actions = Array.isArray(actionOutput.action) ? actionOutput.action : [actionOutput.action]
	return actions.map((action) => Object.keys(action)[0])
}

function macroOutput(nextGoal, action) {
	return {
		evaluation_previous_goal:
			'Evaluate the previous page state and avoid unsafe or irrelevant branches.',
		memory: 'Keep only stable page names, controls, and reusable paths.',
		next_goal: nextGoal,
		action,
	}
}

function hasElement(prompt, terms) {
	return findElementIndex(prompt, terms) >= 0
}

function deploymentInputIndex(prompt) {
	return firstElementIndex(prompt, [
		['发布服务名称', 'input'],
		['service name', 'input'],
		['deployService', 'input'],
		['input'],
	])
}

function serviceInputIndex(prompt) {
	return firstElementIndex(prompt, [
		['服务名称', 'input'],
		['service name', 'input'],
		['serviceName', 'input'],
		['input'],
	])
}

function firstElementIndex(prompt, termGroups) {
	for (const terms of termGroups) {
		const index = findElementIndex(prompt, terms)
		if (index >= 0) return index
	}
	return -1
}

function findElementIndex(prompt, terms) {
	const lines = indexedElementLines(prompt)
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
	return -1
}

function indexedElementLines(prompt) {
	return prompt.split(/\n/).filter((line) => /^\*?\[\d+\]<([^>]*)>/.test(line.trim()))
}

function extractWebOpsMemory(prompt) {
	const match = /<webops_memory>[\s\S]*?<\/webops_memory>/.exec(prompt)
	return match?.[0]?.trim() ?? ''
}

async function main() {
	assert(fs.existsSync(extensionPath), `Extension path not found: ${extensionPath}`)
	const extensionID = extensionIDFromManifest(extensionPath)
	const businessServer = createBusinessPageServer()
	const businessURL = await listen(businessServer)
	const mockLLM = createMockLLMServer()
	const mockLLMBaseURL = await listen(mockLLM.server)
	const llmBaseURL = `${mockLLMBaseURL}/v1`

	let context
	try {
		record('seed documents')
		const seededKnowledge = await seedDocuments()
		context = await launchBrowser(extensionID)
		const sidepanel = await configureExtension(context, extensionID, {
			llmBaseURL,
			memoryEnabled: false,
		})
		const businessPage = await context.newPage()
		await businessPage.goto(`${businessURL}/console`)
		await setBusinessView(businessPage, 'home')
		await businessPage.bringToFront()
		await sidepanel.bringToFront()

		const scenarios = []
		scenarios.push(
			await runScenario({
				id: 'baseline_no_memory_service_triage',
				task: '复杂任务A：排查 checkout-api 错误率并找到负责人',
				memoryEnabled: false,
				sidepanel,
				businessPage,
				businessURL,
				mockLLM,
			})
		)

		await configureMemory(sidepanel, true)
		await setBusinessView(businessPage, 'home')
		scenarios.push(
			await runScenario({
				id: 'memory_guided_service_triage',
				task: '复杂任务A：排查 checkout-api 错误率并找到负责人',
				memoryEnabled: true,
				sidepanel,
				businessPage,
				businessURL,
				mockLLM,
			})
		)

		await setBusinessView(businessPage, 'home')
		scenarios.push(
			await runScenario({
				id: 'memory_experience_reuse_payment_api',
				task: '复杂任务B：排查 payment-api 错误率并找到负责人',
				memoryEnabled: true,
				sidepanel,
				businessPage,
				businessURL,
				mockLLM,
			})
		)

		await setBusinessView(businessPage, 'deployments')
		scenarios.push(
			await runScenario({
				id: 'memory_failure_seed_rollback',
				task: '复杂任务C：风险演练：回滚 checkout-api',
				memoryEnabled: true,
				sidepanel,
				businessPage,
				businessURL,
				mockLLM,
			})
		)

		const memoryContextBefore = await captureMemoryContextSnapshot('before-attribution-follow-up')
		writeJSON(memoryContextBeforePath, memoryContextBefore)

		await setBusinessView(businessPage, 'deployments')
		scenarios.push(
			await runScenario({
				id: 'memory_failure_warning_freeze_check',
				task: '复杂任务D：确认 checkout-api 发布冻结窗口，不执行回滚',
				memoryEnabled: true,
				sidepanel,
				businessPage,
				businessURL,
				mockLLM,
			})
		)

		await businessPage.screenshot({ path: screenshotPath, fullPage: true })

		const memoryContextAfter = await captureMemoryContextSnapshot('after-attribution-follow-up')
		writeJSON(memoryContextAfterPath, memoryContextAfter)

		const attributionProbe = await runAttributionEffectProbe()
		memoryContextBefore.attributionProbe = {
			pageA: attributionProbe.pageA,
			correctExperienceId: attributionProbe.correctExperienceId,
			wrongExperienceId: attributionProbe.wrongExperienceId,
			response: attributionProbe.before,
		}
		memoryContextAfter.attributionProbe = {
			pageA: attributionProbe.pageA,
			correctExperienceId: attributionProbe.correctExperienceId,
			wrongExperienceId: attributionProbe.wrongExperienceId,
			response: attributionProbe.after,
			afterExperienceIDs: attributionProbe.afterExperienceIDs,
		}
		writeJSON(memoryContextBeforePath, memoryContextBefore)
		writeJSON(memoryContextAfterPath, memoryContextAfter)

		const inspector = await fetchInspectorSnapshot()
		writeJSON(inspectorPath, inspector)
		const backendSavedData = buildBackendSavedDataSnapshot(inspector, seededKnowledge)
		writeJSON(dbSnapshotPath, backendSavedData)
		const taskRunUpload = buildTaskRunUploadArtifact(
			inspector,
			scenarios.find((scenario) => scenario.id === 'memory_failure_warning_freeze_check')
		)
		writeJSON(taskRunUploadPath, taskRunUpload)
		writeJSON(attributionEventsPath, {
			projectID,
			capturedAt: new Date().toISOString(),
			events: inspector.attributionEvents ?? [],
			evidenceStats: inspector.evidenceStats ?? [],
			probe: {
				correctExperienceId: attributionProbe.correctExperienceId,
				wrongExperienceId: attributionProbe.wrongExperienceId,
				correctEvent: attributionProbe.correctEvent,
				wrongEvent: attributionProbe.wrongEvent,
				correctStats: attributionProbe.correctStats,
				wrongStats: attributionProbe.wrongStats,
				afterExperienceIDs: attributionProbe.afterExperienceIDs,
			},
			compatibility: inspector.compatibility,
		})

		const comparison = buildComparison(scenarios)
		const injectedMemoryContexts = collectInjectedMemoryContexts(scenarios)
		const report = {
			ok: true,
			projectID,
			backendURL,
			browserChannel,
			extensionID,
			extensionPath,
			businessURL,
			llmMode: 'deterministic-openai-compatible-mock',
			scenarios,
			injectedMemoryContexts,
			comparison,
			attributionProbe: {
				correctExperienceId: attributionProbe.correctExperienceId,
				wrongExperienceId: attributionProbe.wrongExperienceId,
				correctLabel: attributionProbe.correctEvent?.label ?? null,
				wrongLabel: attributionProbe.wrongEvent?.label ?? null,
				afterExperienceIDs: attributionProbe.afterExperienceIDs,
			},
			taskRunUploadSummary: {
				source: taskRunUpload.source,
				taskRunId: taskRunUpload.taskRunId,
				memoryContextId: taskRunUpload.memoryContextId ?? null,
				evidenceRefCount: Array.isArray(taskRunUpload.memoryEvidenceRefs)
					? taskRunUpload.memoryEvidenceRefs.length
					: 0,
				memoryEvidenceRefs: Array.isArray(taskRunUpload.memoryEvidenceRefs)
					? taskRunUpload.memoryEvidenceRefs
					: [],
			},
			backendCounts: {
				pageStates: firstArray(inspector, ['pageStates', 'page_states']).length,
				experiences: firstArray(inspector, ['experiences', 'experience_memories']).length,
				failures: firstArray(inspector, ['failures', 'failure_memories']).length,
				workflows: firstArray(inspector, ['workflows', 'workflow_recipes']).length,
				attributionEvents: inspector.attributionEvents?.length ?? 0,
				evidenceStats: inspector.evidenceStats?.length ?? 0,
			},
			attributionCompatibility: inspector.compatibility,
			artifacts: {
				reportPath,
				processLogPath,
				inspectorPath,
				eventsPath,
				injectedMemoryPath,
				injectedMemoryJSONPath,
				screenshotPath,
				htmlReportPath,
				dbSnapshotPath,
				memoryContextBeforePath,
				taskRunUploadPath,
				attributionEventsPath,
				memoryContextAfterPath,
			},
		}
		const injectedMemoryLog = formatInjectedMemoryLog(report)
		writeJSON(reportPath, report)
		writeJSON(eventsPath, events)
		writeJSON(injectedMemoryJSONPath, injectedMemoryContexts)
		fs.writeFileSync(injectedMemoryPath, injectedMemoryLog)
		fs.writeFileSync(processLogPath, formatProcessLog(report))
		fs.writeFileSync(
			htmlReportPath,
			buildHTMLReport({
				report,
				memoryContextBefore,
				memoryContextAfter,
				taskRunUpload,
				backendSavedData,
			})
		)
		assertComparison(report)
		console.log(
			JSON.stringify(
				{
					ok: true,
					artifactDir,
					reportPath,
					processLogPath,
					inspectorPath,
					injectedMemoryPath,
					injectedMemoryJSONPath,
					screenshotPath,
					htmlReportPath,
					dbSnapshotPath,
					memoryContextBeforePath,
					taskRunUploadPath,
					attributionEventsPath,
					memoryContextAfterPath,
					comparison,
				},
				null,
				2
			)
		)
		console.log('\n--- Injected WebOps Memory Contexts ---\n')
		console.log(injectedMemoryLog)
	} finally {
		await context?.close().catch(() => {})
		await closeServer(businessServer).catch(() => {})
		await closeServer(mockLLM.server).catch(() => {})
	}
}

async function launchBrowser(extensionID) {
	const launchOptions = {
		headless: false,
		viewport: { width: 1440, height: 940 },
		args: [
			`--disable-extensions-except=${extensionPath}`,
			`--load-extension=${extensionPath}`,
			'--disable-web-security',
		],
	}
	if (browserChannel !== 'chromium') {
		launchOptions.channel = browserChannel
	}
	const context = await chromium.launchPersistentContext(userDataDir, launchOptions)
	await waitForExtensionLoaded(context, extensionID, browserChannel)
	return context
}

async function configureExtension(context, extensionID, input) {
	const setupPage = await context.newPage()
	await setupPage.goto(`chrome-extension://${extensionID}/hub.html`)
	await setupPage.evaluate(
		async ({ llmBaseURL, llmModel, backendURL, projectID, memoryEnabled }) => {
			await chrome.storage.local.set({
				llmConfig: {
					baseURL: llmBaseURL,
					model: llmModel,
					apiKey: 'mock-key',
				},
				language: 'zh-CN',
				advancedConfig: { maxSteps: 10 },
				knowledgeSettings: {
					enabled: memoryEnabled,
					allowPageSummary: true,
					baseUrl: backendURL,
					projectKey: projectID,
				},
				workflowBackend: memoryEnabled
					? {
							baseUrl: backendURL,
							projectId: projectID,
						}
					: null,
			})
		},
		{ ...input, llmModel, backendURL, projectID }
	)
	await setupPage.close()
	const page = await context.newPage()
	await page.goto(`chrome-extension://${extensionID}/sidepanel.html`)
	await page.waitForSelector('textarea', { timeout: 30_000 })
	return page
}

async function configureMemory(sidepanel, enabled) {
	await sidepanel.evaluate(
		async ({ backendURL, projectID, enabled }) => {
			await chrome.storage.local.set({
				knowledgeSettings: {
					enabled,
					allowPageSummary: true,
					baseUrl: backendURL,
					projectKey: projectID,
				},
				workflowBackend: enabled
					? {
							baseUrl: backendURL,
							projectId: projectID,
						}
					: null,
			})
		},
		{ backendURL, projectID, enabled }
	)
	await sidepanel.reload()
	await sidepanel.waitForSelector('textarea', { timeout: 30_000 })
}

async function runScenario(input) {
	record('scenario:start', { id: input.id, memoryEnabled: input.memoryEnabled, task: input.task })
	await input.sidepanel.evaluate(() =>
		chrome.storage.local.remove(['lastWebOpsSession', 'isAgentRunning'])
	)
	await clickNewSession(input.sidepanel)
	await input.businessPage.bringToFront()
	const beforePageMetrics = await pageMetrics(input.businessPage)
	const startRequestIndex = input.mockLLM.requestSummaries.length
	const startedAt = Date.now()
	await runTask(input.sidepanel, input.task)
	const durationMs = Date.now() - startedAt
	const afterPageMetrics = await pageMetrics(input.businessPage)
	const storage = await input.sidepanel.evaluate(async () =>
		chrome.storage.local.get(['lastWebOpsSession'])
	)
	const screenshotPath = path.join(artifactDir, `${input.id}.png`)
	await input.businessPage.screenshot({ path: screenshotPath, fullPage: true })
	const llmRequests = input.mockLLM.requestSummaries.slice(startRequestIndex)
	const scenario = {
		id: input.id,
		task: input.task,
		memoryEnabled: input.memoryEnabled,
		durationMs,
		llmCallCount: llmRequests.length,
		actionNames: llmRequests.flatMap((item) => item.actionNames),
		promptEvidence: summarizeScenarioPromptEvidence(llmRequests),
		beforePageMetrics,
		afterPageMetrics,
		pageMetricDelta: diffMetrics(beforePageMetrics, afterPageMetrics),
		lastSession: summarizeSession(storage.lastWebOpsSession),
		screenshotPath,
		llmRequests,
	}
	record('scenario:end', {
		id: input.id,
		llmCallCount: scenario.llmCallCount,
		actionNames: scenario.actionNames,
		promptEvidence: scenario.promptEvidence,
		pageMetricDelta: scenario.pageMetricDelta,
		lastSession: scenario.lastSession,
		llmRequests,
	})
	return scenario
}

async function setBusinessView(page, view) {
	await page.bringToFront()
	const suffix = view === 'home' ? '' : `/${view}`
	await page.goto(page.url().replace(/\/console.*$/, `/console${suffix}`))
	if (view === 'home') {
		await page.waitForSelector('text=WebOps 控制台首页', { timeout: 10_000 })
		return
	}
	if (view === 'deployments') {
		await page.waitForSelector('#deployService', { timeout: 10_000 })
		return
	}
	if (view === 'services') {
		await page.waitForSelector('#serviceName', { timeout: 10_000 })
	}
}

async function clickNewSession(sidepanel) {
	const button = sidepanel.getByRole('button', { name: '新建会话' })
	if ((await button.count()) > 0) {
		await button.click().catch(() => undefined)
	}
}

async function runTask(sidepanel, task) {
	const textarea = sidepanel.locator('textarea')
	await textarea.waitFor({ state: 'visible', timeout: 30_000 })
	await textarea.fill(task)
	await sidepanel.getByRole('button', { name: /Send|发送/ }).click()
	await sidepanel.evaluate(async (expectedTask) => {
		for (let i = 0; i < 240; i += 1) {
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
		throw new Error(`Timed out waiting for task: ${expectedTask}`)
	}, task)
}

async function pageMetrics(page) {
	return page.evaluate(() => ({ ...window.__webops }))
}

function diffMetrics(before, after) {
	const result = {}
	for (const key of Object.keys(after)) {
		if (typeof after[key] === 'number') {
			result[key] = after[key] - (before[key] ?? 0)
		}
	}
	return result
}

function summarizeScenarioPromptEvidence(llmRequests) {
	return {
		hasWebOpsMemory: llmRequests.some((item) => item.hasWebOpsMemory),
		hasCurrentPage: llmRequests.some((item) => item.hasCurrentPage),
		hasKnowledgeEvidence: llmRequests.some((item) => item.hasKnowledgeEvidence),
		hasExperienceHints: llmRequests.some((item) => item.hasExperienceHints),
		hasFailureWarnings: llmRequests.some((item) => item.hasFailureWarnings),
	}
}

function collectInjectedMemoryContexts(scenarios) {
	const contexts = []
	for (const scenario of scenarios) {
		scenario.llmRequests.forEach((request, index) => {
			if (!request.memoryContext) return
			contexts.push({
				scenarioId: scenario.id,
				task: scenario.task,
				llmCall: index + 1,
				taskKind: request.taskKind,
				nextGoal: request.nextGoal,
				actionInput: request.actionInput,
				memoryContextLength: request.memoryContextLength,
				memoryContext: request.memoryContext,
			})
		})
	}
	return contexts
}

function summarizeSession(session) {
	if (!session) return null
	return {
		id: session.id,
		task: session.task,
		stepCount: Array.isArray(session.steps) ? session.steps.length : 0,
		memoryUpdates: session.memoryUpdates ?? [],
		steps: (session.steps ?? []).map((step) => ({
			type: step.type,
			pageUrl: step.pageUrl,
			pageTitle: step.pageTitle,
			target: step.target?.name || step.target?.role || step.target?.elementIndex,
			result: step.result,
			note: step.note,
			value: step.value ? '{{value}}' : undefined,
		})),
	}
}

function buildComparison(scenarios) {
	const byID = Object.fromEntries(scenarios.map((scenario) => [scenario.id, scenario]))
	const baseline = byID.baseline_no_memory_service_triage
	const guided = byID.memory_guided_service_triage
	const reuse = byID.memory_experience_reuse_payment_api
	const failureSeed = byID.memory_failure_seed_rollback
	const warning = byID.memory_failure_warning_freeze_check
	return {
		serviceTriage: {
			baselineLLMCalls: baseline.llmCallCount,
			memoryLLMCalls: guided.llmCallCount,
			llmCallReduction: baseline.llmCallCount - guided.llmCallCount,
			baselineWrongIncidentVisits: baseline.pageMetricDelta.wrongIncidentVisits,
			memoryWrongIncidentVisits: guided.pageMetricDelta.wrongIncidentVisits,
			wrongBranchReduction:
				baseline.pageMetricDelta.wrongIncidentVisits - guided.pageMetricDelta.wrongIncidentVisits,
			memoryHadKnowledgeEvidence: guided.promptEvidence.hasKnowledgeEvidence,
		},
		experienceReuse: {
			hasExperienceHints: reuse.promptEvidence.hasExperienceHints,
			stepCount: reuse.lastSession?.stepCount ?? 0,
			memoryUpdates: reuse.lastSession?.memoryUpdates ?? [],
		},
		failureWarning: {
			seedRollbackClicks: failureSeed.pageMetricDelta.rollbackClicks,
			warningTaskRollbackClicks: warning.pageMetricDelta.rollbackClicks,
			warningTaskFreezeChecks: warning.pageMetricDelta.freezeChecks,
			hasFailureWarnings: warning.promptEvidence.hasFailureWarnings,
		},
	}
}

function assertComparison(report) {
	const service = report.comparison.serviceTriage
	assert(!report.scenarios[0].promptEvidence.hasWebOpsMemory, 'baseline should not receive memory')
	assert(
		service.memoryHadKnowledgeEvidence,
		'memory-guided service task should receive knowledge evidence'
	)
	assert(service.llmCallReduction > 0, 'memory-guided service task should reduce LLM calls')
	assert(
		service.wrongBranchReduction > 0,
		'memory-guided service task should avoid wrong incident branch'
	)
	assert(
		report.comparison.experienceReuse.hasExperienceHints,
		'similar task should receive experience hints'
	)
	assert(
		report.comparison.failureWarning.seedRollbackClicks >= 1,
		'seed task should record rollback failure'
	)
	assert(
		report.comparison.failureWarning.hasFailureWarnings,
		'follow-up task should receive failure warning'
	)
	assert(
		report.comparison.failureWarning.warningTaskRollbackClicks === 0,
		'warning task must avoid rollback'
	)
	assert(
		report.comparison.failureWarning.warningTaskFreezeChecks >= 1,
		'warning task should use safe freeze check'
	)
	assert(report.backendCounts.experiences >= 1, 'backend should save experience memories')
	assert(report.backendCounts.failures >= 1, 'backend should save failure memories')
	assert(report.backendCounts.attributionEvents >= 2, 'backend should save attribution events')
	assert(report.backendCounts.evidenceStats >= 2, 'backend should save evidence stats')
	assert(
		report.attributionProbe.correctLabel === 'helpful',
		'attribution probe should label correct evidence helpful'
	)
	assert(
		report.attributionProbe.wrongLabel === 'misleading',
		'attribution probe should label wrong evidence misleading'
	)
	assert(
		report.taskRunUploadSummary.memoryContextId,
		'task-run upload should retain memoryContextId'
	)
	assert(
		report.taskRunUploadSummary.evidenceRefCount > 0,
		'task-run upload should retain memoryEvidenceRefs'
	)
}

function buildHTMLReport({
	report,
	memoryContextBefore,
	memoryContextAfter,
	taskRunUpload,
	backendSavedData,
}) {
	return `<!doctype html>
<html>
<head>
  <meta charset="utf-8" />
  <title>WebOps Memory V2 Complex Effect E2E</title>
  <style>
    body { font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 24px; color: #17202a; background: #f7f9fb; }
    main { max-width: 1200px; }
    h1, h2 { margin: 0 0 12px; }
    section { background: #fff; border: 1px solid #d9e1ec; border-radius: 8px; padding: 16px; margin-bottom: 16px; }
    img { max-width: 100%; border: 1px solid #ccd6e2; border-radius: 6px; }
    pre { white-space: pre-wrap; word-break: break-word; background: #0f1720; color: #e8eef5; padding: 12px; border-radius: 6px; overflow: auto; }
    .muted { color: #5b6b7f; }
  </style>
</head>
<body>
<main>
  <h1>WebOps Memory V2 Complex Effect E2E</h1>
  <p class="muted">Project: ${escapeHTML(report.projectID)}</p>
  <section>
    <h2>Chrome User Flow</h2>
    <img src="./chrome-user-flow.png" alt="Chrome user flow screenshot" />
  </section>
  <section>
    <h2>Summary</h2>
    <pre>${escapeHTML(
			JSON.stringify(
				{
					comparison: report.comparison,
					backendCounts: report.backendCounts,
					attributionCompatibility: report.attributionCompatibility,
				},
				null,
				2
			)
		)}</pre>
  </section>
  <section>
    <h2>Memory Context Before</h2>
    <pre>${escapeHTML(JSON.stringify(memoryContextBefore, null, 2))}</pre>
  </section>
  <section>
    <h2>Task Run Upload</h2>
    <pre>${escapeHTML(JSON.stringify(taskRunUpload, null, 2))}</pre>
  </section>
  <section>
    <h2>Memory Context After</h2>
    <pre>${escapeHTML(JSON.stringify(memoryContextAfter, null, 2))}</pre>
  </section>
  <section>
    <h2>Backend Saved Data</h2>
    <pre>${escapeHTML(JSON.stringify(backendSavedData, null, 2))}</pre>
  </section>
  <section>
    <h2>Full Report</h2>
    <pre>${escapeHTML(JSON.stringify(report, null, 2))}</pre>
  </section>
</main>
</body>
</html>`
}

function formatProcessLog(report) {
	const lines = [
		'# WebOps Memory V2 Complex Effect E2E',
		'',
		`- Project: ${report.projectID}`,
		`- Browser channel: ${report.browserChannel}`,
		`- LLM mode: ${report.llmMode}`,
		`- Backend: ${report.backendURL}`,
		'',
		'## Comparison',
		'',
		`- Service triage LLM calls: baseline ${report.comparison.serviceTriage.baselineLLMCalls}, memory ${report.comparison.serviceTriage.memoryLLMCalls}, reduction ${report.comparison.serviceTriage.llmCallReduction}.`,
		`- Wrong incident branch visits: baseline ${report.comparison.serviceTriage.baselineWrongIncidentVisits}, memory ${report.comparison.serviceTriage.memoryWrongIncidentVisits}.`,
		`- Experience reuse prompt had hints: ${report.comparison.experienceReuse.hasExperienceHints}.`,
		`- Failure warning prompt had warning: ${report.comparison.failureWarning.hasFailureWarnings}.`,
		`- Rollback clicks after warning: ${report.comparison.failureWarning.warningTaskRollbackClicks}; freeze checks: ${report.comparison.failureWarning.warningTaskFreezeChecks}.`,
		'',
		'## Scenario Timeline',
		'',
	]
	for (const scenario of report.scenarios) {
		lines.push(`### ${scenario.id}`)
		lines.push(`- Task: ${scenario.task}`)
		lines.push(`- Memory enabled: ${scenario.memoryEnabled}`)
		lines.push(`- LLM calls: ${scenario.llmCallCount}`)
		lines.push(`- Prompt evidence: ${JSON.stringify(scenario.promptEvidence)}`)
		lines.push(`- Page metric delta: ${JSON.stringify(scenario.pageMetricDelta)}`)
		lines.push(
			`- Session memory updates: ${JSON.stringify(scenario.lastSession?.memoryUpdates ?? [])}`
		)
		lines.push(`- Screenshot: ${scenario.screenshotPath}`)
		lines.push('')
	}
	lines.push('## Raw Events')
	lines.push('')
	for (const event of events) {
		lines.push(`- ${event.at} ${event.event} ${JSON.stringify(omit(event, ['at', 'event']))}`)
	}
	lines.push('')
	return lines.join('\n')
}

function formatInjectedMemoryLog(report) {
	const lines = [
		'# Injected WebOps Memory Contexts',
		'',
		`- Project: ${report.projectID}`,
		`- Backend: ${report.backendURL}`,
		`- LLM mode: ${report.llmMode}`,
		`- Captured contexts: ${report.injectedMemoryContexts.length}`,
		'',
		'This file records the exact `<webops_memory>` blocks that the Page Agent LLM prompt received during the test run.',
		'',
	]
	for (const scenario of report.scenarios) {
		lines.push(`## ${scenario.id}`)
		lines.push('')
		lines.push(`- Task: ${scenario.task}`)
		lines.push(`- Memory enabled: ${scenario.memoryEnabled}`)
		const contexts = report.injectedMemoryContexts.filter(
			(context) => context.scenarioId === scenario.id
		)
		if (contexts.length === 0) {
			lines.push('- Injected memory: none')
			lines.push('')
			continue
		}
		for (const context of contexts) {
			lines.push(`### LLM call ${context.llmCall}`)
			lines.push('')
			lines.push(`- Task kind: ${context.taskKind}`)
			lines.push(`- Next goal selected by mock LLM: ${context.nextGoal}`)
			lines.push(`- Memory context chars: ${context.memoryContextLength}`)
			lines.push('')
			lines.push('```xml')
			lines.push(context.memoryContext)
			lines.push('```')
			lines.push('')
		}
	}
	return lines.join('\n')
}

function omit(value, keys) {
	const result = { ...value }
	for (const key of keys) delete result[key]
	return result
}

async function seedDocuments() {
	const request = {
		projectId: projectID,
		documents: [
			{
				id: `doc_service_triage_${scenarioID}`,
				title: '服务健康巡检手册',
				source: 'runbook',
				url: 'http://127.0.0.1/console#services',
				content:
					'# 服务健康巡检\n从 WebOps 控制台进入服务管理，输入服务名称并点击查询服务。服务画像会展示运行状态、错误率、P95 延迟和负责人。事件中心只适合告警筛选，不适合直接查服务负责人。',
				tags: ['service', 'triage'],
			},
			{
				id: `doc_deploy_safety_${scenarioID}`,
				title: '发布冻结窗口确认手册',
				source: 'runbook',
				url: 'http://127.0.0.1/console#deployments',
				content:
					'# 发布冻结窗口确认\n从发布管理输入服务名称并查询发布。只读任务只允许点击查看冻结窗口，不允许点击立即回滚。立即回滚需要人工审批。',
				tags: ['deployment', 'safety'],
			},
		],
	}
	const response = await post('/api/memory/documents', request)
	return {
		capturedAt: new Date().toISOString(),
		endpoint: '/api/memory/documents',
		request,
		response,
	}
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
			'Use the bundled Chromium default or a browser channel that supports unpacked extension loading.'
	)
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

main().catch((error) => {
	fs.writeFileSync(eventsPath, JSON.stringify(events, null, 2))
	console.error(error)
	process.exit(1)
})
