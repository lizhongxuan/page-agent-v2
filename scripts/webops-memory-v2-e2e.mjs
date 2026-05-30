import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import { chromium } from 'playwright'

const backendURL = process.env.WEBOPS_MEMORY_V2_BACKEND_URL ?? 'http://127.0.0.1:38432'
const artifactDir = path.resolve(
	process.env.WEBOPS_MEMORY_V2_ARTIFACT_DIR ?? 'artifacts/webops-memory-v2/manual'
)
const postgresContainer =
	process.env.WEBOPS_MEMORY_V2_POSTGRES_CONTAINER ?? 'page-agent-webops-memory-v2-postgres-e2e'
const postgresDB = process.env.WEBOPS_MEMORY_V2_POSTGRES_DB ?? 'page_agent_webops_memory_v2_e2e'
const postgresUser = process.env.WEBOPS_MEMORY_V2_POSTGRES_USER ?? 'page_agent'

fs.mkdirSync(artifactDir, { recursive: true })

const scenarioID = `wmv2_${Date.now().toString(36)}`
const projectID = `project_${scenarioID}`
const screenshotPath = path.join(artifactDir, 'chrome-user-flow.png')
const reportPath = path.join(artifactDir, 'report.json')
const dbSnapshotPath = path.join(artifactDir, 'backend-saved-data.json')
const htmlReportPath = path.join(artifactDir, 'user-flow-report.html')

function assert(condition, message) {
	if (!condition) throw new Error(message)
}

function psqlJSON(sql) {
	const stdout = execFileSync(
		'docker',
		[
			'exec',
			postgresContainer,
			'psql',
			'-U',
			postgresUser,
			'-d',
			postgresDB,
			'-t',
			'-A',
			'-c',
			sql,
		],
		{ encoding: 'utf8', maxBuffer: 20 * 1024 * 1024 }
	)
	return JSON.parse(stdout.trim())
}

function dumpBackendData() {
	const sql = `
select jsonb_pretty(jsonb_build_object(
  'projectId', '${projectID}',
  'business_system_profiles', coalesce((select jsonb_agg(to_jsonb(business_system_profiles) order by project_id) from business_system_profiles where project_id = '${projectID}'), '[]'::jsonb),
  'page_observation_events', coalesce((select jsonb_agg(to_jsonb(page_observation_events) order by created_at, id) from page_observation_events where project_id = '${projectID}'), '[]'::jsonb),
  'page_states', coalesce((select jsonb_agg(to_jsonb(page_states) order by id) from page_states where project_id = '${projectID}'), '[]'::jsonb),
  'page_transitions', coalesce((select jsonb_agg(to_jsonb(page_transitions) order by id) from page_transitions where project_id = '${projectID}'), '[]'::jsonb),
  'experience_memories', coalesce((select jsonb_agg(to_jsonb(experience_memories) order by updated_at, id) from experience_memories where project_id = '${projectID}'), '[]'::jsonb),
  'failure_memories', coalesce((select jsonb_agg(to_jsonb(failure_memories) order by created_at, id) from failure_memories where project_id = '${projectID}'), '[]'::jsonb),
  'memory_context_events', coalesce((select jsonb_agg(to_jsonb(memory_context_events) order by created_at, id) from memory_context_events where project_id = '${projectID}'), '[]'::jsonb),
  'knowledge_documents', coalesce((select jsonb_agg(to_jsonb(knowledge_documents) order by id) from knowledge_documents where project_id = '${projectID}'), '[]'::jsonb),
  'knowledge_chunks', coalesce((select jsonb_agg(to_jsonb(knowledge_chunks) order by id) from knowledge_chunks where project_id = '${projectID}'), '[]'::jsonb),
  'task_runs', coalesce((select jsonb_agg(to_jsonb(task_runs) order by created_at, id) from task_runs where project_id = '${projectID}'), '[]'::jsonb)
));`
	return psqlJSON(sql)
}

async function main() {
	const browser = await chromium.launch({
		channel: process.env.PLAYWRIGHT_CHROME_CHANNEL ?? 'chrome',
		headless: process.env.PLAYWRIGHT_HEADLESS !== '0',
		args: ['--disable-web-security'],
	})
	const page = await browser.newPage({ viewport: { width: 1360, height: 920 } })
	const browserLogs = []
	page.on('console', (message) => {
		const line = `[browser:${message.type()}] ${message.text()}`
		browserLogs.push(line)
		console.log(line)
	})
	page.on('pageerror', (error) => browserLogs.push(`[pageerror] ${error.message}`))

	await page.setContent(`<!doctype html>
<html>
<head>
  <meta charset="utf-8" />
  <title>WebOps Memory V2 E2E</title>
  <style>
    body { font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 32px; color: #17202a; background: #f7f9fb; }
    main { max-width: 1100px; }
    h1 { font-size: 28px; margin: 0 0 16px; }
    .toolbar { display: flex; gap: 12px; align-items: center; margin-bottom: 18px; }
    input { font: inherit; padding: 10px 12px; width: 260px; border: 1px solid #b7c2d0; border-radius: 6px; }
    button { font: inherit; padding: 10px 14px; border: 1px solid #275efe; background: #275efe; color: white; border-radius: 6px; cursor: pointer; }
    #status { padding: 16px; min-height: 500px; white-space: pre-wrap; background: white; border: 1px solid #d9e1ec; border-radius: 8px; line-height: 1.55; }
  </style>
</head>
<body>
<main>
  <h1>WebOps Memory V2 E2E</h1>
  <div class="toolbar">
    <input id="serviceName" aria-label="服务名称搜索框" value="kme-prod-001" />
    <button id="run">查询服务状态</button>
  </div>
  <div id="status">idle</div>
</main>
<script>
const API = ${JSON.stringify(backendURL)};
const projectID = ${JSON.stringify(projectID)};
const status = document.getElementById('status');
function line(text) { status.textContent += '\\n' + text; console.log(text); }
async function post(path, body) {
  const response = await fetch(API + path, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(body),
  });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(path + ' failed ' + response.status + ': ' + JSON.stringify(data));
  return data;
}
window.e2eResult = null;
document.getElementById('run').addEventListener('click', async () => {
  try {
    status.textContent = 'running';
    const serviceName = document.getElementById('serviceName').value;
    line('user: query service status for ' + serviceName);
    const documents = await post('/api/memory/documents', {
      projectId: projectID,
      documents: [{
        id: 'doc_service_' + ${JSON.stringify(scenarioID)},
        title: '服务管理手册',
        source: 'manual',
        url: 'https://ops.example.com/services',
        content: '# 服务管理\\n服务管理页可通过服务名称搜索框定位服务，状态列表示当前运行状态。详情页展示运行状态和负责人。',
        tags: ['服务管理', 'status'],
      }],
    });
    line('memory: documents imported -> ' + documents.documentIds.join(','));
    const list = await post('/api/memory/page-observations', {
      projectId: projectID,
      task: '查询服务状态',
      url: 'https://ops.example.com/services?k=' + encodeURIComponent(serviceName),
      title: '服务管理',
      visibleText: ['服务管理', '服务名称', '状态', '负责人'],
      controls: [{ role: 'textbox', name: '服务名称' }, { role: 'button', name: '搜索' }],
      links: [{ text: '服务详情', hrefPattern: '/services/:service_id' }],
      source: 'before_task',
    });
    line('memory: observed list page -> ' + list.pageStateId);
    const detail = await post('/api/memory/page-observations', {
      projectId: projectID,
      task: '查询服务状态',
      url: 'https://ops.example.com/services/' + encodeURIComponent(serviceName),
      title: '服务详情',
      visibleText: ['服务详情', '运行状态', '负责人'],
      controls: [{ role: 'button', name: '部署记录' }],
      previousPageStateId: list.pageStateId,
      transitionAction: '搜索服务名后打开详情',
      transitionTarget: '服务名称链接',
      source: 'after_navigation',
    });
    line('memory: observed detail page -> ' + detail.pageStateId);
    const beforeContext = await post('/api/memory/context', {
      projectId: projectID,
      task: '查询 ' + serviceName + ' 服务是否正常',
      currentUrl: 'https://ops.example.com/services',
      pageObservation: {
        title: '服务管理',
        visibleText: ['服务管理', '服务名称', '状态'],
        controls: [{ role: 'textbox', name: '服务名称' }, { role: 'button', name: '搜索' }],
      },
      riskPolicy: { blocked: ['destructive'] },
      mode: 'before_task',
    });
    line('memory: before context mode -> ' + beforeContext.recommendedMode);
    const success = await post('/api/memory/task-runs', {
      id: 'task_run_success_' + ${JSON.stringify(scenarioID)},
      projectId: projectID,
      site: 'ops.example.com',
      taskTemplate: '查询 {{service_name}} 服务是否正常',
      summary: '已查询服务运行状态。',
      originalPath: [list.pageStateId, detail.pageStateId],
      status: 'success',
      actionSteps: [{
        id: 'step_search',
        pageStateId: list.pageStateId,
        stepIndex: 1,
        actionType: 'fill',
        targetName: '服务名称搜索框',
        valueTemplate: '{{service_name}}',
        reasoningSummary: '需要按服务名定位目标服务。',
        resultSummary: '服务列表过滤到目标服务。',
      }],
    });
    line('memory: success task consolidated -> ' + success.experienceId);
    const afterSuccessContext = await post('/api/memory/context', {
      projectId: projectID,
      task: '查询 another-service 服务是否正常',
      currentPageStateId: list.pageStateId,
      riskPolicy: { blocked: ['destructive'] },
      mode: 'before_task',
    });
    line('memory: after success hints -> ' + afterSuccessContext.experienceHints.length);
    const failure = await post('/api/memory/task-runs', {
      id: 'task_run_failure_' + ${JSON.stringify(scenarioID)},
      projectId: projectID,
      site: 'ops.example.com',
      taskTemplate: '查询 {{service_name}} 服务是否正常',
      summary: '服务详情打开失败。',
      originalPath: [list.pageStateId],
      optimizedPath: [list.pageStateId],
      status: 'failed',
      actionSteps: [{
        id: 'step_failed',
        pageStateId: list.pageStateId,
        stepIndex: 1,
        actionType: 'fill',
        targetName: '服务名称搜索框',
        valueTemplate: '{{service_name}}',
        reasoningSummary: '尝试搜索服务。',
        resultSummary: '服务详情打开失败。',
      }],
    });
    line('memory: failure task recorded -> ' + failure.failureMemoryId);
    const afterFailureContext = await post('/api/memory/context', {
      projectId: projectID,
      task: '查询服务状态',
      currentPageStateId: list.pageStateId,
      riskPolicy: { blocked: ['destructive'] },
      mode: 'before_task',
    });
    line('memory: failure warnings -> ' + afterFailureContext.failureWarnings.length);
    window.e2eResult = {
      projectID, documents, list, detail, beforeContext, success,
      afterSuccessContext, failure, afterFailureContext,
    };
    status.textContent += '\\ncompleted';
  } catch (error) {
    window.e2eError = String(error && error.stack ? error.stack : error);
    status.textContent += '\\nERROR: ' + window.e2eError;
    console.error(window.e2eError);
  }
});
</script>
</body>
</html>`)

	await page.click('#run')
	await page.waitForFunction(() => window.e2eResult || window.e2eError, null, { timeout: 60_000 })
	const result = await page.evaluate(() => window.e2eResult)
	const error = await page.evaluate(() => window.e2eError)
	await page.screenshot({ path: screenshotPath, fullPage: true })
	await browser.close()
	if (error) throw new Error(error)

	assert(result.documents.updatedBusinessProfile === true, 'document import should update profile')
	assert(result.list.pageStateId, 'page observation should return list page state')
	assert(result.detail.pageStateId, 'page observation should return detail page state')
	assert(
		result.beforeContext.contextPrompt.includes('<webops_memory>'),
		'context prompt should use webops memory wrapper'
	)
	assert(result.beforeContext.businessContext?.summary, 'context should include business summary')
	assert(result.beforeContext.currentPageState?.id, 'context should include current page')
	assert(
		result.beforeContext.knowledgeEvidence.length >= 1,
		'context should include knowledge evidence'
	)
	assert(result.success.experienceId, 'successful task should create experience memory')
	assert(result.success.optimizedPath.length >= 2, 'successful task should return optimized path')
	assert(
		result.afterSuccessContext.experienceHints.length >= 1,
		'second context should recall reusable experience'
	)
	assert(result.failure.failureMemoryId, 'failed task should create failure memory')
	assert(
		result.afterFailureContext.failureWarnings.length >= 1,
		'context should recall failure warning'
	)

	const backendSavedData = dumpBackendData()
	fs.writeFileSync(dbSnapshotPath, JSON.stringify(backendSavedData, null, 2))
	for (const key of [
		'business_system_profiles',
		'page_observation_events',
		'page_states',
		'page_transitions',
		'experience_memories',
		'failure_memories',
		'memory_context_events',
	]) {
		assert(
			Array.isArray(backendSavedData[key]) && backendSavedData[key].length > 0,
			`${key} should be saved`
		)
	}

	const report = {
		projectID,
		backendURL,
		artifactDir,
		browserLogs,
		result,
		savedDataCounts: Object.fromEntries(
			Object.entries(backendSavedData).map(([key, value]) => [
				key,
				Array.isArray(value) ? value.length : value,
			])
		),
	}
	fs.writeFileSync(reportPath, JSON.stringify(report, null, 2))
	fs.writeFileSync(
		htmlReportPath,
		`<!doctype html><meta charset="utf-8"><title>WebOps Memory V2 E2E</title><body><h1>WebOps Memory V2 E2E</h1><p>Project: ${projectID}</p><img src="./chrome-user-flow.png" style="max-width:100%;border:1px solid #ccc"><pre>${escapeHTML(JSON.stringify(report, null, 2))}</pre></body>`
	)
	console.log(
		JSON.stringify({ ok: true, artifactDir, reportPath, dbSnapshotPath, screenshotPath }, null, 2)
	)
}

function escapeHTML(value) {
	return value
		.replaceAll('&', '&amp;')
		.replaceAll('<', '&lt;')
		.replaceAll('>', '&gt;')
		.replaceAll('"', '&quot;')
}

main().catch((error) => {
	console.error(error)
	process.exit(1)
})
