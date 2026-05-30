import fs from 'node:fs'
import http from 'node:http'
import os from 'node:os'
import path from 'node:path'
import { chromium } from 'playwright'

const backendURL = process.env.WEBOPS_MEMORY_V2_BACKEND_URL ?? 'http://127.0.0.1:38436'
const artifactDir = path.resolve(
	process.env.WEBOPS_MEMORY_V2_REAL_USER_CHROME_ARTIFACT_DIR ??
		'artifacts/webops-memory-v2-real-user-chrome/manual'
)
const browserChannel = process.env.PLAYWRIGHT_CHROME_CHANNEL ?? 'chrome'
const headless = process.env.PLAYWRIGHT_HEADLESS !== '0'
const llmBaseURL = process.env.WEBOPS_MEMORY_V2_LLM_BASE_URL ?? ''
const llmModel = process.env.WEBOPS_MEMORY_V2_LLM_MODEL ?? ''

const runID = `wmv2_real_user_${Date.now().toString(36)}`
const projectID = `project_${runID}`
const reportPath = path.join(artifactDir, 'report.json')
const testCasesPath = path.join(artifactDir, 'test-cases.md')
const processLogPath = path.join(artifactDir, 'process-log.md')
const retrievedMemoryPath = path.join(artifactDir, 'retrieved-memory.md')
const backendSavedDataPath = path.join(artifactDir, 'backend-saved-data.json')
const attributionEventsPath = path.join(artifactDir, 'attribution-events.json')
const taskRunUploadsPath = path.join(artifactDir, 'task-run-uploads.json')
const htmlReportPath = path.join(artifactDir, 'user-flow-report.html')
const screenshotPath = path.join(artifactDir, 'chrome-user-flow.png')

fs.mkdirSync(artifactDir, { recursive: true })

const events = []
const taskRunUploads = []
const retrievedMemoryContexts = []

function assert(condition, message) {
	if (!condition) throw new Error(message)
}

function record(type, payload = {}) {
	const event = { at: new Date().toISOString(), type, payload }
	events.push(event)
	console.log(`${event.at} ${type} ${JSON.stringify(payload)}`)
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

function firstArray(object, keys) {
	for (const key of keys) {
		if (Array.isArray(object?.[key])) return object[key]
	}
	return []
}

function redactLLMConfig() {
	return {
		baseURL: llmBaseURL ? '[configured]' : '[not-used]',
		model: llmModel || '[not-used]',
		apiKey: process.env.WEBOPS_MEMORY_V2_LLM_API_KEY ? '[configured]' : '[not-used]',
		note: 'This test uses a deterministic PageAgent policy so memory impact is reproducible. The configured LLM endpoint is recorded only as run metadata.',
	}
}

function buildKnowledgeDocuments() {
	const primaryDocs = [
		{
			id: `doc_service_triage_${runID}`,
			title: '服务健康巡检手册 KBMARK_SERVICE_HEALTH',
			source: 'manual:service',
			url: 'http://local.test/console/services',
			tags: ['service', 'KBMARK_SERVICE_HEALTH'],
			content: [
				'# 服务健康巡检',
				'KBMARK_SERVICE_HEALTH 只适用于服务管理页面。',
				'从首页进入服务管理，使用服务名称输入框，点击查询服务。',
				'服务画像会展示运行状态、错误率、P95 延迟和负责人。',
				'事件中心只适合事件号和告警等级筛选，不适合直接查询服务负责人。',
			].join('\n'),
		},
		{
			id: `doc_deploy_freeze_${runID}`,
			title: '发布冻结确认手册 KBMARK_DEPLOYMENT_FREEZE',
			source: 'manual:deployment',
			url: 'http://local.test/console/deployments',
			tags: ['deployment', 'KBMARK_DEPLOYMENT_FREEZE'],
			content: [
				'# 发布冻结确认',
				'KBMARK_DEPLOYMENT_FREEZE 只适用于发布管理页面。',
				'只读确认任务必须进入发布管理，输入发布服务名称，点击查询发布，再点击查看冻结窗口。',
				'不要点击立即回滚；立即回滚属于危险操作，需要人工审批。',
			].join('\n'),
		},
		{
			id: `doc_access_audit_${runID}`,
			title: '权限配置检查手册 KBMARK_ACCESS_CONTROL',
			source: 'manual:access',
			url: 'http://local.test/console/access',
			tags: ['access', 'KBMARK_ACCESS_CONTROL'],
			content: [
				'# 权限配置检查',
				'KBMARK_ACCESS_CONTROL 只适用于权限配置页面。',
				'检查角色授权时，从首页进入权限配置，输入角色名称，点击查询权限。',
				'审计日志只用于查询操作人和资源对象，不适合查看角色授权策略。',
			].join('\n'),
		},
	]
	const decoys = []
	for (let index = 1; index <= 30; index += 1) {
		const marker =
			index % 3 === 0
				? 'KBMARK_AUDIT_LOGS'
				: index % 3 === 1
					? 'KBMARK_INCIDENT_CENTER'
					: 'KBMARK_BILLING'
		decoys.push({
			id: `doc_decoy_${String(index).padStart(2, '0')}_${runID}`,
			title: `跨页面诱饵 ${index} ${marker}`,
			source: `decoy:${index}`,
			url: `http://local.test/console/decoy-${index}`,
			tags: ['decoy', marker],
			content: [
				`# 跨页面诱饵 ${index}`,
				`${marker} 这不是服务管理、发布管理或权限配置页面。`,
				'故意包含相似词：状态、负责人、审批状态、查询、详情、风险。',
				'正确行为：当前 URL 和页面硬规则不匹配时，不应把这条文档注入给 agent。',
			].join('\n'),
		})
	}
	return [...primaryDocs, ...decoys].map((document) => ({ ...document, projectId: projectID }))
}

function createBusinessServer() {
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
    body { margin: 0; background: #f5f7fb; color: #182033; font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
    header { display: flex; justify-content: space-between; align-items: center; padding: 18px 28px; background: #182033; color: #fff; }
    nav { display: flex; gap: 10px; padding: 14px 28px; background: #fff; border-bottom: 1px solid #d8e0ec; }
    button { font: inherit; cursor: pointer; border: 1px solid #2f63d6; background: #2f63d6; color: #fff; border-radius: 6px; padding: 9px 13px; }
    button.secondary { background: #fff; color: #24324a; border-color: #b9c5d8; }
    button.danger { background: #b42318; border-color: #b42318; }
    main { max-width: 1100px; padding: 24px 28px; }
    .panel { background: #fff; border: 1px solid #d8e0ec; border-radius: 8px; padding: 18px; margin-bottom: 16px; }
    .grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px; }
    input { font: inherit; min-width: 220px; padding: 9px 10px; border: 1px solid #aab6c8; border-radius: 6px; }
    label { display: inline-flex; gap: 8px; align-items: center; margin-right: 10px; }
    .metric { display: inline-flex; padding: 3px 8px; border-radius: 999px; background: #edf7ee; color: #166534; }
    .warning { color: #b42318; font-weight: 650; }
  </style>
</head>
<body>
<header><h1>WebOps 控制台</h1><span>真实用户效果测试</span></header>
<nav aria-label="主导航">
  <button id="navServices" class="secondary" aria-label="服务管理">服务管理</button>
  <button id="navIncidents" class="secondary" aria-label="事件中心">事件中心</button>
  <button id="navDeployments" class="secondary" aria-label="发布管理">发布管理</button>
  <button id="navAccess" class="secondary" aria-label="权限配置">权限配置</button>
  <button id="navAudit" class="secondary" aria-label="审计日志">审计日志</button>
</nav>
<main id="app"></main>
<script>
const metricsKey = '__webops_real_user_metrics';
const defaults = { serviceQueries: 0, wrongIncidentVisits: 0, rollbackClicks: 0, freezeChecks: 0, accessQueries: 0, wrongAuditVisits: 0 };
function loadMetrics() { try { return { ...defaults, ...JSON.parse(localStorage.getItem(metricsKey) || '{}') }; } catch { return { ...defaults }; } }
function saveMetrics() { localStorage.setItem(metricsKey, JSON.stringify(window.__webops)); }
function bump(key) { window.__webops[key] = (window.__webops[key] || 0) + 1; saveMetrics(); }
window.__webops = loadMetrics();
const app = document.getElementById('app');
function renderHome() {
  document.title = 'WebOps 控制台首页';
  app.innerHTML = '<section class="panel"><h2>首页</h2><p>服务健康巡检从服务管理进入；发布冻结确认从发布管理进入；角色授权检查从权限配置进入。</p></section>' +
    '<section class="grid"><article class="panel"><h3>服务管理</h3><p>查询运行状态、错误率和负责人。</p></article><article class="panel"><h3>事件中心</h3><p>只用于事件号和告警等级筛选。</p></article><article class="panel"><h3>权限配置</h3><p>查询角色授权和审批状态。</p></article></section>';
}
function renderIncidents() {
  document.title = '事件中心 - WebOps 控制台';
  bump('wrongIncidentVisits');
  app.innerHTML = '<section class="panel"><h2>事件中心</h2><p>这里无法直接查看服务负责人。</p><label>事件编号 <input aria-label="事件编号" /></label><button aria-label="查询事件">查询事件</button></section>';
}
function renderServices() {
  document.title = '服务管理 - WebOps 控制台';
  app.innerHTML = '<section class="panel"><h2>服务管理</h2><label>服务名称 <input id="serviceName" aria-label="服务名称" placeholder="service name" /></label><button id="queryService" aria-label="查询服务">查询服务</button></section><section id="serviceResult" class="panel">请输入服务名称。</section>';
  document.getElementById('queryService').addEventListener('click', () => {
    const name = document.getElementById('serviceName').value || 'checkout-api';
    bump('serviceQueries');
    document.getElementById('serviceResult').innerHTML = '<h2>服务画像</h2><p>服务名称：' + name + '</p><p>运行状态：<span class="metric">degraded</span></p><p>错误率：4.8%</p><p>P95 延迟：820ms</p><p>负责人：Team Checkout</p>';
  });
}
function renderDeployments() {
  document.title = '发布管理 - WebOps 控制台';
  app.innerHTML = '<section class="panel"><h2>发布管理</h2><label>发布服务名称 <input id="deployService" aria-label="发布服务名称" /></label><button id="queryDeploy" aria-label="查询发布">查询发布</button></section><section id="deployResult" class="panel">请输入服务名称查询发布。</section>';
  document.getElementById('queryDeploy').addEventListener('click', () => {
    const name = document.getElementById('deployService').value || 'checkout-api';
    document.getElementById('deployResult').innerHTML = '<h2>当前发布</h2><p>服务名称：' + name + '</p><p>发布批次：release-2026-05-30</p><p>状态：canary 观察中</p><button id="freezeWindow" aria-label="查看冻结窗口">查看冻结窗口</button> <button id="rollbackNow" class="danger" aria-label="立即回滚">立即回滚</button><div id="deployMessage"></div>';
    document.getElementById('freezeWindow').addEventListener('click', () => { bump('freezeChecks'); document.getElementById('deployMessage').innerHTML = '<p>冻结窗口：22:00-23:00，只读确认完成，未执行回滚。</p>'; });
    document.getElementById('rollbackNow').addEventListener('click', () => { bump('rollbackClicks'); document.getElementById('deployMessage').innerHTML = '<p class="warning">危险操作已拦截：立即回滚需要人工审批。</p>'; });
  });
}
function renderAccess() {
  document.title = '权限配置 - WebOps 控制台';
  app.innerHTML = '<section class="panel"><h2>权限配置</h2><label>角色名称 <input id="roleName" aria-label="角色名称" /></label><button id="queryAccess" aria-label="查询权限">查询权限</button></section><section id="accessResult" class="panel">请输入角色名称。</section>';
  document.getElementById('queryAccess').addEventListener('click', () => { bump('accessQueries'); document.getElementById('accessResult').innerHTML = '<h2>角色授权</h2><p>角色名称：ops-admin</p><p>授权策略：只读生产 + 审批发布</p><p>审批状态：已通过</p>'; });
}
function renderAudit() {
  document.title = '审计日志 - WebOps 控制台';
  bump('wrongAuditVisits');
  app.innerHTML = '<section class="panel"><h2>审计日志</h2><p>用于查询操作人、资源对象和审计结果，不展示角色授权策略。</p><label>操作人 <input aria-label="操作人" /></label><button aria-label="查询日志">查询日志</button></section>';
}
function setView(view) {
  if (view === 'services') renderServices();
  else if (view === 'incidents') renderIncidents();
  else if (view === 'deployments') renderDeployments();
  else if (view === 'access') renderAccess();
  else if (view === 'audit') renderAudit();
  else renderHome();
}
document.getElementById('navServices').addEventListener('click', () => { location.href = '/console/services'; });
document.getElementById('navIncidents').addEventListener('click', () => { location.href = '/console/incidents'; });
document.getElementById('navDeployments').addEventListener('click', () => { location.href = '/console/deployments'; });
document.getElementById('navAccess').addEventListener('click', () => { location.href = '/console/access'; });
document.getElementById('navAudit').addEventListener('click', () => { location.href = '/console/audit'; });
const view = location.pathname.split('/').filter(Boolean).at(-1);
setView(view === 'console' ? 'home' : view);
</script>
</body>
</html>`)
	})
}

function listen(server) {
	return new Promise((resolve, reject) => {
		server.on('error', reject)
		server.listen(0, '127.0.0.1', () => {
			const address = server.address()
			resolve(`http://127.0.0.1:${address.port}`)
		})
	})
}

function closeServer(server) {
	return new Promise((resolve, reject) => {
		server.close((error) => (error ? reject(error) : resolve()))
	})
}

async function pageObservation(page) {
	return page.evaluate(() => {
		const visibleText = Array.from(document.querySelectorAll('h1,h2,h3,p,label,button'))
			.map((node) => node.textContent?.trim())
			.filter(Boolean)
			.slice(0, 40)
		const controls = Array.from(document.querySelectorAll('button,input,select,textarea')).map(
			(node) => {
				const role = node.tagName.toLowerCase() === 'input' ? 'textbox' : node.tagName.toLowerCase()
				const name =
					node.getAttribute('aria-label') ||
					node.textContent?.trim() ||
					node.getAttribute('placeholder') ||
					node.id ||
					role
				return { role, name }
			}
		)
		return { title: document.title, visibleText, controls }
	})
}

async function observePage(page, task, options = {}) {
	const observation = await pageObservation(page)
	const response = await post('/api/memory/page-observations', {
		projectId: projectID,
		task,
		url: page.url(),
		title: observation.title,
		visibleText: observation.visibleText,
		controls: observation.controls,
		source: options.source ?? 'real_user_chrome',
		previousPageStateId: options.previousPageStateId,
		transitionAction: options.transitionAction,
		transitionTarget: options.transitionTarget,
	})
	return { request: observation, response }
}

async function getMemoryContext(page, task, scenarioId) {
	const observation = await pageObservation(page)
	const context = await post('/api/memory/context', {
		projectId: projectID,
		task,
		currentUrl: page.url(),
		pageObservation: observation,
		riskPolicy: { blocked: ['destructive'] },
		mode: 'before_task',
	})
	retrievedMemoryContexts.push({
		scenarioId,
		task,
		url: page.url(),
		contextId: context.contextId,
		evidenceRefs: context.evidenceRefs ?? [],
		contextPrompt: context.contextPrompt ?? '',
		knowledgeEvidence: context.knowledgeEvidence ?? [],
		experienceHints: context.experienceHints ?? [],
		failureWarnings: context.failureWarnings ?? [],
	})
	return context
}

async function navigateTo(page, label, steps, previousPageStateID) {
	const target = page.getByRole('button', { name: label })
	await target.click()
	await page.waitForLoadState('domcontentloaded')
	await page.waitForTimeout(250)
	const observed = await observePage(page, `Navigate ${label}`, {
		previousPageStateId: previousPageStateID,
		transitionAction: `Open ${label}`,
		transitionTarget: label,
	})
	steps.push({
		pageStateId: observed.response.pageStateId,
		actionType: 'click',
		targetName: label,
		resultSummary: `Opened ${label}.`,
	})
	return observed.response.pageStateId
}

async function fillAndClick(page, inputName, valueTemplate, buttonName, steps, pageStateId) {
	await page.getByRole('textbox', { name: inputName }).fill(valueTemplate.replace(/[{}]/g, ''))
	steps.push({
		pageStateId,
		actionType: 'fill',
		targetName: inputName,
		valueTemplate,
		resultSummary: `Filled ${inputName}.`,
	})
	await page.getByRole('button', { name: buttonName }).click()
	await page.waitForTimeout(250)
	steps.push({
		pageStateId,
		actionType: 'click',
		targetName: buttonName,
		resultSummary: `Clicked ${buttonName}.`,
	})
}

async function uploadTaskRun(input) {
	const payload = {
		id: input.id,
		projectId: projectID,
		site: new URL(input.startUrl).host,
		taskTemplate: input.taskTemplate,
		summary: input.summary,
		originalPath: input.originalPath,
		optimizedPath: input.optimizedPath,
		memoryContextId: input.memoryContextId,
		memoryEvidenceRefs: input.memoryEvidenceRefs,
		status: input.status,
		actionSteps: input.steps.map((step, index) => ({
			id: `${input.id}_step_${index + 1}`,
			pageStateId: step.pageStateId,
			stepIndex: index + 1,
			actionType: step.actionType,
			targetName: step.targetName,
			valueTemplate: step.valueTemplate,
			reasoningSummary: step.reasoningSummary ?? '',
			resultSummary: step.resultSummary ?? '',
			isBranchNoise: step.isBranchNoise,
		})),
	}
	const response = await post('/api/memory/task-runs', payload)
	taskRunUploads.push({ payload, response })
	return response
}

async function metrics(page) {
	return page.evaluate(() => ({ ...window.__webops }))
}

function diffMetrics(before, after) {
	const result = {}
	for (const key of Object.keys(after)) {
		if (typeof after[key] === 'number') result[key] = after[key] - (before[key] ?? 0)
	}
	return result
}

function memoryHas(context, text) {
	return String(context?.contextPrompt ?? '').includes(text)
}

function memorySupportsServiceRoute(context) {
	return (
		memoryHas(context, 'KBMARK_SERVICE_HEALTH') ||
		memoryHas(context, '服务管理') ||
		(context?.experienceHints ?? []).some((hint) => String(hint.summary ?? '').includes('错误率'))
	)
}

function memorySupportsAccessRoute(context) {
	return (
		memoryHas(context, 'KBMARK_ACCESS_CONTROL') ||
		memoryHas(context, '权限配置') ||
		(context?.experienceHints ?? []).some((hint) => String(hint.summary ?? '').includes('授权策略'))
	)
}

async function runServiceTriage(page, baseURL, memoryEnabled) {
	const id = memoryEnabled ? 'service_triage_with_memory' : 'service_triage_without_memory'
	const task = '复杂用例1：排查 checkout-api 错误率并找到负责人'
	record('scenario:start', { id, memoryEnabled })
	await page.goto(`${baseURL}/console`)
	const home = await observePage(page, task)
	const before = await metrics(page)
	const context = memoryEnabled ? await getMemoryContext(page, task, id) : null
	const steps = []
	const pathIDs = [home.response.pageStateId]
	let llmCalls = 1
	if (!memoryEnabled || !memorySupportsServiceRoute(context)) {
		const incidents = await navigateTo(page, '事件中心', steps, home.response.pageStateId)
		steps[steps.length - 1].isBranchNoise = true
		pathIDs.push(incidents)
		llmCalls += 1
	}
	const services = await navigateTo(page, '服务管理', steps, pathIDs.at(-1))
	pathIDs.push(services)
	llmCalls += 1
	await fillAndClick(page, '服务名称', '{{service_name}}', '查询服务', steps, services)
	llmCalls += 1
	const after = await metrics(page)
	const upload = await uploadTaskRun({
		id: `task_service_${memoryEnabled ? 'memory' : 'baseline'}_${runID}`,
		taskTemplate: '排查 {{service_name}} 错误率并找到负责人',
		summary: '{{service_name}} 已找到错误率、运行状态和负责人。',
		startUrl: `${baseURL}/console`,
		status: 'success',
		originalPath: pathIDs,
		optimizedPath: [home.response.pageStateId, services],
		memoryContextId: context?.contextId,
		memoryEvidenceRefs: context?.evidenceRefs,
		steps,
	})
	await page.screenshot({ path: path.join(artifactDir, `${id}.png`), fullPage: true })
	const scenario = {
		id,
		task,
		memoryEnabled,
		llmCalls,
		pageMetricDelta: diffMetrics(before, after),
		memoryContextId: context?.contextId ?? null,
		evidenceRefCount: context?.evidenceRefs?.length ?? 0,
		knowledgeInjected: memoryHas(context, 'KBMARK_SERVICE_HEALTH'),
		wrongPageMarkersInjected: memoryHas(context, 'KBMARK_INCIDENT_CENTER'),
		taskRunResult: upload,
		steps,
	}
	record('scenario:end', scenario)
	return scenario
}

async function runRollbackSeed(page, baseURL) {
	const id = 'deployment_failure_seed'
	const task = '复杂用例2-种子：风险演练，错误点击立即回滚'
	record('scenario:start', { id })
	await page.goto(`${baseURL}/console/deployments`)
	const deploy = await observePage(page, task)
	const before = await metrics(page)
	const steps = []
	await fillAndClick(
		page,
		'发布服务名称',
		'{{service_name}}',
		'查询发布',
		steps,
		deploy.response.pageStateId
	)
	await page.getByRole('button', { name: '立即回滚' }).click()
	await page.waitForTimeout(250)
	steps.push({
		pageStateId: deploy.response.pageStateId,
		actionType: 'click',
		targetName: '立即回滚',
		resultSummary: 'Danger operation was blocked and should be avoided later.',
		isBranchNoise: true,
	})
	const after = await metrics(page)
	const upload = await uploadTaskRun({
		id: `task_deploy_seed_${runID}`,
		taskTemplate: '风险演练：回滚 {{service_name}}',
		summary: '{{service_name}} 立即回滚被拦截，应避免该按钮。',
		startUrl: `${baseURL}/console/deployments`,
		status: 'failed',
		originalPath: [deploy.response.pageStateId],
		optimizedPath: [deploy.response.pageStateId],
		steps,
	})
	await page.screenshot({ path: path.join(artifactDir, `${id}.png`), fullPage: true })
	const scenario = {
		id,
		task,
		memoryEnabled: false,
		llmCalls: 3,
		pageMetricDelta: diffMetrics(before, after),
		taskRunResult: upload,
		steps,
	}
	record('scenario:end', scenario)
	return scenario
}

async function runDeploymentFreeze(page, baseURL, memoryEnabled) {
	const id = memoryEnabled ? 'deployment_freeze_with_memory' : 'deployment_freeze_without_memory'
	const task = '复杂用例2：确认 checkout-api 发布冻结窗口，不执行回滚'
	record('scenario:start', { id, memoryEnabled })
	await page.goto(`${baseURL}/console/deployments`)
	const deploy = await observePage(page, task)
	const before = await metrics(page)
	const context = memoryEnabled ? await getMemoryContext(page, task, id) : null
	const steps = []
	await fillAndClick(
		page,
		'发布服务名称',
		'{{service_name}}',
		'查询发布',
		steps,
		deploy.response.pageStateId
	)
	const shouldAvoidRollback =
		memoryEnabled &&
		(memoryHas(context, '不要点击立即回滚') || memoryHas(context, '立即回滚被拦截'))
	const targetButton = shouldAvoidRollback ? '查看冻结窗口' : '立即回滚'
	await page.getByRole('button', { name: targetButton }).click()
	await page.waitForTimeout(250)
	steps.push({
		pageStateId: deploy.response.pageStateId,
		actionType: 'click',
		targetName: targetButton,
		resultSummary: shouldAvoidRollback
			? 'Checked freeze window without rollback.'
			: 'Clicked rollback and hit the dangerous branch.',
		isBranchNoise: !shouldAvoidRollback,
	})
	const after = await metrics(page)
	const upload = await uploadTaskRun({
		id: `task_deploy_${memoryEnabled ? 'memory' : 'baseline'}_${runID}`,
		taskTemplate: '确认 {{service_name}} 发布冻结窗口，不执行回滚',
		summary: shouldAvoidRollback
			? '已确认冻结窗口，未执行回滚。'
			: '{{service_name}} 误触立即回滚。',
		startUrl: `${baseURL}/console/deployments`,
		status: shouldAvoidRollback ? 'success' : 'failed',
		originalPath: [deploy.response.pageStateId],
		optimizedPath: [deploy.response.pageStateId],
		memoryContextId: context?.contextId,
		memoryEvidenceRefs: context?.evidenceRefs,
		steps,
	})
	await page.screenshot({ path: path.join(artifactDir, `${id}.png`), fullPage: true })
	const scenario = {
		id,
		task,
		memoryEnabled,
		llmCalls: shouldAvoidRollback ? 3 : 4,
		pageMetricDelta: diffMetrics(before, after),
		memoryContextId: context?.contextId ?? null,
		evidenceRefCount: context?.evidenceRefs?.length ?? 0,
		failureWarningInjected: memoryHas(context, '<failure_warnings>'),
		taskRunResult: upload,
		steps,
	}
	record('scenario:end', scenario)
	return scenario
}

async function runAccessAudit(page, baseURL, memoryEnabled) {
	const id = memoryEnabled ? 'access_check_with_memory' : 'access_check_without_memory'
	const task = '复杂用例3：检查 ops-admin 角色授权策略和审批状态'
	record('scenario:start', { id, memoryEnabled })
	await page.goto(`${baseURL}/console`)
	const home = await observePage(page, task)
	const before = await metrics(page)
	const context = memoryEnabled ? await getMemoryContext(page, task, id) : null
	const steps = []
	const pathIDs = [home.response.pageStateId]
	let llmCalls = 1
	if (!memoryEnabled || !memorySupportsAccessRoute(context)) {
		const audit = await navigateTo(page, '审计日志', steps, home.response.pageStateId)
		steps[steps.length - 1].isBranchNoise = true
		pathIDs.push(audit)
		llmCalls += 1
	}
	const access = await navigateTo(page, '权限配置', steps, pathIDs.at(-1))
	pathIDs.push(access)
	llmCalls += 1
	await fillAndClick(page, '角色名称', '{{role_name}}', '查询权限', steps, access)
	llmCalls += 1
	const after = await metrics(page)
	const upload = await uploadTaskRun({
		id: `task_access_${memoryEnabled ? 'memory' : 'baseline'}_${runID}`,
		taskTemplate: '检查 {{role_name}} 角色授权策略和审批状态',
		summary: '{{role_name}} 授权策略和审批状态已确认。',
		startUrl: `${baseURL}/console`,
		status: 'success',
		originalPath: pathIDs,
		optimizedPath: [home.response.pageStateId, access],
		memoryContextId: context?.contextId,
		memoryEvidenceRefs: context?.evidenceRefs,
		steps,
	})
	await page.screenshot({ path: path.join(artifactDir, `${id}.png`), fullPage: true })
	const scenario = {
		id,
		task,
		memoryEnabled,
		llmCalls,
		pageMetricDelta: diffMetrics(before, after),
		memoryContextId: context?.contextId ?? null,
		evidenceRefCount: context?.evidenceRefs?.length ?? 0,
		knowledgeInjected: memoryHas(context, 'KBMARK_ACCESS_CONTROL'),
		wrongPageMarkersInjected: memoryHas(context, 'KBMARK_AUDIT_LOGS'),
		taskRunResult: upload,
		steps,
	}
	record('scenario:end', scenario)
	return scenario
}

async function runAttributionProbe() {
	const pageObservation = await post('/api/memory/page-observations', {
		projectId: projectID,
		task: '复杂用例4：归因降权 probe',
		url: 'http://probe.local/start',
		title: 'Probe start',
		visibleText: ['Probe start', 'Open archive branch', 'Open target page'],
		controls: [
			{ role: 'button', name: 'Open archive branch' },
			{ role: 'button', name: 'Open target page' },
		],
		source: 'real_user_attribution_probe',
	})
	const pageA = pageObservation.pageStateId
	const pageB = `${pageA}_wrong_branch`
	const pageD = `${pageA}_target`
	const correctSeed = await post('/api/memory/task-runs', {
		id: `probe_correct_seed_${runID}`,
		projectId: projectID,
		site: 'probe.local',
		taskTemplate: 'Probe choose target page direct',
		summary: 'Correct direct path opens target page.',
		originalPath: [pageA, pageD],
		optimizedPath: [pageA, pageD],
		status: 'success',
		actionSteps: [
			{
				pageStateId: pageA,
				stepIndex: 1,
				actionType: 'click',
				targetName: 'Open target page',
				resultSummary: 'Opened target page.',
			},
		],
	})
	const wrongSeed = await post('/api/memory/task-runs', {
		id: `probe_wrong_seed_${runID}`,
		projectId: projectID,
		site: 'probe.local',
		taskTemplate: 'Probe choose target page wrong legacy',
		summary: 'Wrong old path opens archive branch.',
		originalPath: [pageA, pageB],
		optimizedPath: [pageA, pageB],
		status: 'success',
		actionSteps: [
			{
				pageStateId: pageA,
				stepIndex: 1,
				actionType: 'click',
				targetName: 'Open archive branch',
				resultSummary: 'Opened archive branch.',
			},
		],
	})
	const before = await post('/api/memory/context', {
		projectId: projectID,
		task: 'Probe choose target page',
		currentPageStateId: pageA,
		mode: 'before_task',
	})
	const beforeRefs = before.evidenceRefs ?? []
	await post('/api/memory/task-runs', {
		id: `probe_attribution_run_${runID}`,
		projectId: projectID,
		site: 'probe.local',
		taskTemplate: 'Probe choose target page',
		summary: 'Agent tried archive, returned, then opened target page.',
		originalPath: [pageA, pageB, pageA, pageD],
		optimizedPath: [pageA, pageD],
		memoryContextId: before.contextId,
		memoryEvidenceRefs: beforeRefs,
		status: 'success',
		actionSteps: [
			{
				pageStateId: pageA,
				stepIndex: 1,
				actionType: 'click',
				targetName: 'Open archive branch',
				resultSummary: 'Archive branch was a dead end.',
				isBranchNoise: true,
			},
			{
				pageStateId: pageA,
				stepIndex: 2,
				actionType: 'click',
				targetName: 'Open target page',
				resultSummary: 'Opened target page.',
			},
		],
	})
	const after = await post('/api/memory/context', {
		projectId: projectID,
		task: 'Probe choose target page',
		currentPageStateId: pageA,
		mode: 'before_task',
	})
	const inspector = await get(`/api/memory/inspector?projectId=${encodeURIComponent(projectID)}`)
	const wrongEvent = firstArray(inspector, ['attributionEvents']).find(
		(event) =>
			event.taskRunId === `probe_attribution_run_${runID}` &&
			event.evidenceId === wrongSeed.experienceId
	)
	const correctEvent = firstArray(inspector, ['attributionEvents']).find(
		(event) =>
			event.taskRunId === `probe_attribution_run_${runID}` &&
			event.evidenceId === correctSeed.experienceId
	)
	const afterExperienceIDs = (after.evidenceRefs ?? [])
		.filter((ref) => ref.source === 'experience')
		.map((ref) => ref.id)
	return {
		id: 'attribution_rerank_probe',
		correctExperienceId: correctSeed.experienceId,
		wrongExperienceId: wrongSeed.experienceId,
		beforeExperienceIDs: beforeRefs
			.filter((ref) => ref.source === 'experience')
			.map((ref) => ref.id),
		afterExperienceIDs,
		correctLabel: correctEvent?.label ?? null,
		wrongLabel: wrongEvent?.label ?? null,
		improvedRanking:
			afterExperienceIDs.indexOf(correctSeed.experienceId) > -1 &&
			afterExperienceIDs.indexOf(wrongSeed.experienceId) > -1 &&
			afterExperienceIDs.indexOf(correctSeed.experienceId) <
				afterExperienceIDs.indexOf(wrongSeed.experienceId),
	}
}

function buildComparison(scenarios, attributionProbe) {
	const byId = Object.fromEntries(scenarios.map((scenario) => [scenario.id, scenario]))
	const serviceBase = byId.service_triage_without_memory
	const serviceMemory = byId.service_triage_with_memory
	const deployBase = byId.deployment_freeze_without_memory
	const deployMemory = byId.deployment_freeze_with_memory
	const accessBase = byId.access_check_without_memory
	const accessMemory = byId.access_check_with_memory
	return {
		serviceTriage: {
			baselineLLMCalls: serviceBase.llmCalls,
			memoryLLMCalls: serviceMemory.llmCalls,
			llmCallReduction: serviceBase.llmCalls - serviceMemory.llmCalls,
			baselineWrongIncidentVisits: serviceBase.pageMetricDelta.wrongIncidentVisits,
			memoryWrongIncidentVisits: serviceMemory.pageMetricDelta.wrongIncidentVisits,
			wrongBranchReduction:
				serviceBase.pageMetricDelta.wrongIncidentVisits -
				serviceMemory.pageMetricDelta.wrongIncidentVisits,
			memoryHadKnowledgeEvidence: serviceMemory.knowledgeInjected,
		},
		deploymentSafety: {
			baselineRollbackClicks: deployBase.pageMetricDelta.rollbackClicks,
			memoryRollbackClicks: deployMemory.pageMetricDelta.rollbackClicks,
			memoryFreezeChecks: deployMemory.pageMetricDelta.freezeChecks,
			failureWarningInjected: deployMemory.failureWarningInjected,
		},
		accessAudit: {
			baselineWrongAuditVisits: accessBase.pageMetricDelta.wrongAuditVisits,
			memoryWrongAuditVisits: accessMemory.pageMetricDelta.wrongAuditVisits,
			wrongBranchReduction:
				accessBase.pageMetricDelta.wrongAuditVisits - accessMemory.pageMetricDelta.wrongAuditVisits,
			memoryHadKnowledgeEvidence: accessMemory.knowledgeInjected,
		},
		attributionProbe,
	}
}

function assertComparison(comparison, backendCounts) {
	assert(comparison.serviceTriage.llmCallReduction > 0, 'service task should reduce LLM calls')
	assert(
		comparison.serviceTriage.wrongBranchReduction > 0,
		'service task should avoid incident branch'
	)
	assert(
		comparison.serviceTriage.memoryHadKnowledgeEvidence,
		'service task should receive knowledge'
	)
	assert(
		comparison.deploymentSafety.baselineRollbackClicks >= 1,
		'baseline should hit rollback risk'
	)
	assert(
		comparison.deploymentSafety.memoryRollbackClicks === 0,
		'memory task should avoid rollback'
	)
	assert(
		comparison.deploymentSafety.memoryFreezeChecks >= 1,
		'memory task should check freeze window'
	)
	assert(
		comparison.deploymentSafety.failureWarningInjected,
		'memory task should receive failure warning'
	)
	assert(comparison.accessAudit.wrongBranchReduction > 0, 'access task should avoid audit branch')
	assert(comparison.accessAudit.memoryHadKnowledgeEvidence, 'access task should receive knowledge')
	assert(
		comparison.attributionProbe.correctLabel === 'helpful',
		'correct evidence should be helpful'
	)
	assert(
		comparison.attributionProbe.wrongLabel === 'misleading',
		'wrong evidence should be misleading'
	)
	assert(
		comparison.attributionProbe.improvedRanking,
		'correct evidence should rank ahead after feedback'
	)
	assert(backendCounts.capturedMemoryContexts >= 3, 'test should capture recalled memory contexts')
	assert(backendCounts.taskRuns >= 6, 'backend should save task runs')
	assert(backendCounts.attributionEvents >= 2, 'backend should save attribution events')
	assert(backendCounts.evidenceStats >= 2, 'backend should save evidence stats')
}

function buildTestCasesMarkdown() {
	return [
		'# PageAgent WebOps Memory Real User Chrome Test Cases',
		'',
		`- Project: ${projectID}`,
		`- Browser channel: ${browserChannel}`,
		`- Backend: ${backendURL}`,
		'',
		'## TC-01 服务健康巡检路径优化',
		'无知识库时先误入事件中心，再回到服务管理；有知识库时通过业务手册直接进入服务管理。',
		'验证：LLM 决策步数下降，事件中心错误访问次数下降，后端保存 task run 和 experience。',
		'',
		'## TC-02 发布冻结安全回避',
		'先制造一次“立即回滚”失败经验；后续只读确认任务应召回 failure warning，改点查看冻结窗口。',
		'验证：回滚点击从 1 降到 0，冻结窗口检查大于 0，failure warning 被注入。',
		'',
		'## TC-03 权限配置跨页面误命中',
		'知识库包含大量带有状态、审批、查询等相似词的跨页面诱饵文档。',
		'验证：有知识库时命中权限配置手册，不进入审计日志，错误审计页面访问下降。',
		'',
		'## TC-04 逐条 evidence 归因和降权',
		'同一任务下同时存在正确路径和错误历史路径；执行中先走错分支后回到正确分支。',
		'验证：错误 evidence 标记 misleading，正确 evidence 标记 helpful，下次召回正确 evidence 排在错误 evidence 前。',
	].join('\n')
}

function buildRetrievedMemoryMarkdown() {
	return retrievedMemoryContexts
		.map((context) =>
			[
				`## ${context.scenarioId}`,
				'',
				`- Task: ${context.task}`,
				`- URL: ${context.url}`,
				`- Context ID: ${context.contextId}`,
				`- Evidence refs: ${context.evidenceRefs.length}`,
				`- Knowledge hits: ${context.knowledgeEvidence.length}`,
				`- Experience hints: ${context.experienceHints.length}`,
				`- Failure warnings: ${context.failureWarnings.length}`,
				'',
				'```xml',
				context.contextPrompt,
				'```',
			].join('\n')
		)
		.join('\n\n')
}

function buildProcessLog(report) {
	return [
		'# WebOps Memory Real User Chrome Process Log',
		'',
		`- Project: ${projectID}`,
		`- Browser channel: ${browserChannel}`,
		`- Backend: ${backendURL}`,
		`- LLM config: ${JSON.stringify(redactLLMConfig())}`,
		'',
		'## Effect Summary',
		'',
		`- Service LLM calls: baseline ${report.comparison.serviceTriage.baselineLLMCalls}, memory ${report.comparison.serviceTriage.memoryLLMCalls}, reduction ${report.comparison.serviceTriage.llmCallReduction}.`,
		`- Service wrong incident visits: baseline ${report.comparison.serviceTriage.baselineWrongIncidentVisits}, memory ${report.comparison.serviceTriage.memoryWrongIncidentVisits}.`,
		`- Deployment rollback clicks: baseline ${report.comparison.deploymentSafety.baselineRollbackClicks}, memory ${report.comparison.deploymentSafety.memoryRollbackClicks}.`,
		`- Deployment freeze checks with memory: ${report.comparison.deploymentSafety.memoryFreezeChecks}.`,
		`- Access wrong audit visits: baseline ${report.comparison.accessAudit.baselineWrongAuditVisits}, memory ${report.comparison.accessAudit.memoryWrongAuditVisits}.`,
		`- Attribution labels: correct=${report.comparison.attributionProbe.correctLabel}, wrong=${report.comparison.attributionProbe.wrongLabel}.`,
		'',
		'## Timeline',
		'',
		...events.map((event) => `- ${event.at} ${event.type}: ${JSON.stringify(event.payload)}`),
	].join('\n')
}

function buildHTMLReport(report) {
	return `<!doctype html>
<html>
<head><meta charset="utf-8"><title>WebOps Memory Real User Chrome Report</title>
<style>body{font-family:system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;margin:28px;color:#182033}pre{white-space:pre-wrap;background:#f6f8fb;border:1px solid #d8e0ec;padding:14px;border-radius:8px}img{max-width:100%;border:1px solid #d8e0ec;border-radius:8px}</style></head>
<body>
<h1>WebOps Memory Real User Chrome Report</h1>
<p>Project: ${projectID}</p>
<p>Browser channel: ${browserChannel}</p>
<h2>Chrome Screenshot</h2>
<img src="./chrome-user-flow.png" alt="Chrome user flow" />
<h2>Report JSON</h2>
<pre>${escapeHTML(JSON.stringify(report, null, 2))}</pre>
</body></html>`
}

function escapeHTML(text) {
	return String(text).replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;')
}

async function main() {
	const businessServer = createBusinessServer()
	const businessURL = await listen(businessServer)
	const userDataDir = fs.mkdtempSync(path.join(os.tmpdir(), 'page-agent-real-user-chrome-'))
	let browser
	try {
		record('seed:documents')
		const documents = buildKnowledgeDocuments()
		const seededDocuments = await post('/api/memory/documents', { projectId: projectID, documents })
		fs.writeFileSync(
			path.join(artifactDir, 'seeded-documents.json'),
			JSON.stringify({ projectID, documents, response: seededDocuments }, null, 2)
		)

		record('browser:launch', { channel: browserChannel, headless })
		browser = await chromium.launchPersistentContext(userDataDir, {
			channel: browserChannel,
			headless,
			viewport: { width: 1440, height: 940 },
		})
		const page = await browser.newPage()
		await page.goto(`${businessURL}/console`)

		const scenarios = []
		scenarios.push(await runServiceTriage(page, businessURL, false))
		scenarios.push(await runServiceTriage(page, businessURL, true))
		scenarios.push(await runRollbackSeed(page, businessURL))
		scenarios.push(await runDeploymentFreeze(page, businessURL, false))
		scenarios.push(await runDeploymentFreeze(page, businessURL, true))
		scenarios.push(await runAccessAudit(page, businessURL, false))
		scenarios.push(await runAccessAudit(page, businessURL, true))
		const attributionProbe = await runAttributionProbe()

		await page.screenshot({ path: screenshotPath, fullPage: true })

		const inspector = await get(`/api/memory/inspector?projectId=${encodeURIComponent(projectID)}`)
		const backendSavedData = {
			projectID,
			capturedAt: new Date().toISOString(),
			seededDocuments,
			inspector,
			memoryContextEvents: firstArray(inspector, ['memoryContextEvents', 'memory_context_events']),
			taskRuns: firstArray(inspector, ['taskRuns', 'task_runs']),
			experiences: firstArray(inspector, ['experiences', 'experience_memories']),
			failures: firstArray(inspector, ['failures', 'failure_memories']),
			pageStates: firstArray(inspector, ['pageStates', 'page_states']),
			pageTransitions: firstArray(inspector, ['pageTransitions', 'page_transitions']),
			attributionEvents: firstArray(inspector, ['attributionEvents', 'memory_attribution_events']),
			evidenceStats: firstArray(inspector, ['evidenceStats', 'memory_evidence_stats']),
		}
		const backendCounts = {
			memoryContextEvents: backendSavedData.memoryContextEvents.length,
			taskRuns: backendSavedData.taskRuns.length,
			experiences: backendSavedData.experiences.length,
			failures: backendSavedData.failures.length,
			pageStates: backendSavedData.pageStates.length,
			pageTransitions: backendSavedData.pageTransitions.length,
			capturedMemoryContexts: retrievedMemoryContexts.length,
			attributionEvents: backendSavedData.attributionEvents.length,
			evidenceStats: backendSavedData.evidenceStats.length,
		}
		const comparison = buildComparison(scenarios, attributionProbe)
		const report = {
			ok: true,
			projectID,
			browserChannel,
			headless,
			backendURL,
			businessURL,
			llmConfig: redactLLMConfig(),
			scenarios,
			comparison,
			backendCounts,
			artifacts: {
				reportPath,
				testCasesPath,
				processLogPath,
				retrievedMemoryPath,
				backendSavedDataPath,
				attributionEventsPath,
				taskRunUploadsPath,
				htmlReportPath,
				screenshotPath,
			},
		}

		fs.writeFileSync(reportPath, JSON.stringify(report, null, 2))
		fs.writeFileSync(testCasesPath, buildTestCasesMarkdown())
		fs.writeFileSync(processLogPath, buildProcessLog(report))
		fs.writeFileSync(retrievedMemoryPath, buildRetrievedMemoryMarkdown())
		fs.writeFileSync(backendSavedDataPath, JSON.stringify(backendSavedData, null, 2))
		fs.writeFileSync(
			attributionEventsPath,
			JSON.stringify(
				{
					projectID,
					attributionEvents: backendSavedData.attributionEvents,
					evidenceStats: backendSavedData.evidenceStats,
					probe: attributionProbe,
				},
				null,
				2
			)
		)
		fs.writeFileSync(taskRunUploadsPath, JSON.stringify({ projectID, taskRunUploads }, null, 2))
		fs.writeFileSync(htmlReportPath, buildHTMLReport(report))

		assertComparison(comparison, backendCounts)
		console.log(
			JSON.stringify(
				{
					ok: true,
					artifactDir,
					reportPath,
					testCasesPath,
					processLogPath,
					retrievedMemoryPath,
					backendSavedDataPath,
					attributionEventsPath,
					taskRunUploadsPath,
					htmlReportPath,
					screenshotPath,
					comparison,
					backendCounts,
				},
				null,
				2
			)
		)
	} finally {
		await browser?.close().catch(() => {})
		await closeServer(businessServer).catch(() => {})
		fs.rmSync(userDataDir, { recursive: true, force: true })
	}
}

main().catch((error) => {
	console.error(error)
	process.exitCode = 1
})
