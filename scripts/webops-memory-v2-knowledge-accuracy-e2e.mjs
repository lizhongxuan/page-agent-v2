import fs from 'node:fs'
import path from 'node:path'

const backendURL = process.env.WEBOPS_MEMORY_V2_BACKEND_URL ?? 'http://127.0.0.1:38435'
const artifactDir = path.resolve(
	process.env.WEBOPS_MEMORY_V2_KNOWLEDGE_ACCURACY_ARTIFACT_DIR ??
		'artifacts/webops-memory-v2-knowledge-accuracy/manual'
)

fs.mkdirSync(artifactDir, { recursive: true })

const scenarioID = `wmv2_kb_accuracy_${Date.now().toString(36)}`
const projectID = `project_${scenarioID}`
const seededDocumentsPath = path.join(artifactDir, 'seeded-documents.json')
const contextsPath = path.join(artifactDir, 'retrieved-contexts.json')
const promptsPath = path.join(artifactDir, 'retrieved-context-prompts.md')
const reportJSONPath = path.join(artifactDir, 'accuracy-report.json')
const reportMDPath = path.join(artifactDir, 'accuracy-report.md')
const inspectorPath = path.join(artifactDir, 'memory-inspector.json')

const modules = [
	{
		key: 'service-health',
		name: '服务健康',
		url: 'https://webops.example.test/console/services',
		title: '服务管理 - WebOps',
		marker: 'KBMARK_SERVICE_HEALTH',
		task: '查询服务健康状态、错误率和负责人',
		terms: ['服务名称', '错误率', 'P95', '负责人', '依赖健康', '查询服务'],
		controls: [
			{ role: 'textbox', name: '服务名称' },
			{ role: 'button', name: '查询服务' },
		],
	},
	{
		key: 'incident-center',
		name: '事件中心',
		url: 'https://webops.example.test/console/incidents',
		title: '事件中心 - WebOps',
		marker: 'KBMARK_INCIDENT_CENTER',
		task: '筛选生产事件、确认告警等级和处理状态',
		terms: ['事件编号', '告警等级', '处理状态', '值班人', '事件时间', '查询事件'],
		controls: [
			{ role: 'textbox', name: '事件编号' },
			{ role: 'button', name: '查询事件' },
		],
	},
	{
		key: 'deployment-freeze',
		name: '发布冻结',
		url: 'https://webops.example.test/console/deployments',
		title: '发布管理 - WebOps',
		marker: 'KBMARK_DEPLOYMENT_FREEZE',
		task: '查看发布批次、冻结窗口和回滚风险',
		terms: ['发布批次', '冻结窗口', '灰度状态', '回滚风险', '审批状态', '查询发布'],
		controls: [
			{ role: 'textbox', name: '发布服务名称' },
			{ role: 'button', name: '查询发布' },
		],
	},
	{
		key: 'access-control',
		name: '权限配置',
		url: 'https://webops.example.test/console/access',
		title: '权限配置 - WebOps',
		marker: 'KBMARK_ACCESS_CONTROL',
		task: '查询角色授权、成员范围和审批状态',
		terms: ['角色名称', '成员范围', '授权策略', '审批状态', '权限边界', '查询权限'],
		controls: [
			{ role: 'textbox', name: '角色名称' },
			{ role: 'button', name: '查询权限' },
		],
	},
	{
		key: 'billing-invoice',
		name: '账单发票',
		url: 'https://webops.example.test/console/billing',
		title: '账单发票 - WebOps',
		marker: 'KBMARK_BILLING_INVOICE',
		task: '核对账单周期、发票状态和成本归属',
		terms: ['账单周期', '发票状态', '成本中心', '付款状态', '费用明细', '查询账单'],
		controls: [
			{ role: 'textbox', name: '账单周期' },
			{ role: 'button', name: '查询账单' },
		],
	},
	{
		key: 'audit-logs',
		name: '审计日志',
		url: 'https://webops.example.test/console/audit',
		title: '审计日志 - WebOps',
		marker: 'KBMARK_AUDIT_LOGS',
		task: '查询操作人、资源对象和审计结果',
		terms: ['操作人', '资源对象', '审计结果', '操作时间', '来源 IP', '查询日志'],
		controls: [
			{ role: 'textbox', name: '操作人' },
			{ role: 'button', name: '查询日志' },
		],
	},
	{
		key: 'feature-flags',
		name: '功能开关',
		url: 'https://webops.example.test/console/flags',
		title: '功能开关 - WebOps',
		marker: 'KBMARK_FEATURE_FLAGS',
		task: '查看开关状态、命中规则和灰度人群',
		terms: ['开关键', '命中规则', '灰度人群', '启用状态', '默认策略', '查询开关'],
		controls: [
			{ role: 'textbox', name: '开关键' },
			{ role: 'button', name: '查询开关' },
		],
	},
	{
		key: 'data-pipeline',
		name: '数据管道',
		url: 'https://webops.example.test/console/pipelines',
		title: '数据管道 - WebOps',
		marker: 'KBMARK_DATA_PIPELINE',
		task: '查看任务 DAG、延迟水位和重跑状态',
		terms: ['任务 DAG', '延迟水位', '重跑状态', '数据分区', '上游依赖', '查询管道'],
		controls: [
			{ role: 'textbox', name: '管道名称' },
			{ role: 'button', name: '查询管道' },
		],
	},
	{
		key: 'notification-rules',
		name: '通知规则',
		url: 'https://webops.example.test/console/notifications',
		title: '通知规则 - WebOps',
		marker: 'KBMARK_NOTIFICATION_RULES',
		task: '检查通知渠道、接收人和静默窗口',
		terms: ['通知渠道', '接收人', '静默窗口', '升级策略', '触发条件', '查询通知'],
		controls: [
			{ role: 'textbox', name: '规则名称' },
			{ role: 'button', name: '查询通知' },
		],
	},
	{
		key: 'customer-profile',
		name: '客户画像',
		url: 'https://webops.example.test/console/customers',
		title: '客户画像 - WebOps',
		marker: 'KBMARK_CUSTOMER_PROFILE',
		task: '查询客户等级、合同状态和服务经理',
		terms: ['客户等级', '合同状态', '服务经理', '续约时间', '账户健康', '查询客户'],
		controls: [
			{ role: 'textbox', name: '客户名称' },
			{ role: 'button', name: '查询客户' },
		],
	},
	{
		key: 'inventory-sync',
		name: '库存同步',
		url: 'https://webops.example.test/console/inventory',
		title: '库存同步 - WebOps',
		marker: 'KBMARK_INVENTORY_SYNC',
		task: '检查库存渠道、同步延迟和失败批次',
		terms: ['库存渠道', '同步延迟', '失败批次', '商品编码', '同步状态', '查询库存'],
		controls: [
			{ role: 'textbox', name: '商品编码' },
			{ role: 'button', name: '查询库存' },
		],
	},
	{
		key: 'report-export',
		name: '报表导出',
		url: 'https://webops.example.test/console/reports',
		title: '报表导出 - WebOps',
		marker: 'KBMARK_REPORT_EXPORT',
		task: '查看报表模板、导出队列和文件状态',
		terms: ['报表模板', '导出队列', '文件状态', '时间范围', '下载链接', '查询报表'],
		controls: [
			{ role: 'textbox', name: '报表模板' },
			{ role: 'button', name: '查询报表' },
		],
	},
]

const decoyTemplates = [
	'通用列表页说明：所有模块都可能有搜索框、状态列、负责人和详情按钮，但必须以当前 URL 所属页面为准。',
	'跨模块误导样例：查询状态、负责人、审批状态、处理状态这些词在很多页面出现，不能仅凭通用词选择知识。',
	'导航规范：不要从事件中心推断服务管理路径，不要从发布管理推断账单页面路径。',
]

function assert(condition, message) {
	if (!condition) throw new Error(message)
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

function buildDocuments() {
	const documents = []
	for (const module of modules) {
		documents.push({
			id: `kb_${module.key}_primary`,
			projectId: projectID,
			title: `${module.name}主路径手册 ${module.marker}`,
			source: `manual:${module.key}`,
			url: module.url,
			tags: [module.key, module.marker, 'primary-path'],
			content: [
				`# ${module.name}`,
				`${module.marker} 只适用于 ${module.url}。`,
				`页面用途：${module.task}。`,
				`最佳路径：打开 ${module.title} -> 使用 ${module.controls[0].name} -> 点击 ${module.controls[1].name} -> 查看结果区域。`,
				`关键字段：${module.terms.join('、')}。`,
				'约束：只记录固定控件和字段名称，不记录变量实例值。',
			].join('\n'),
		})
		for (let index = 1; index <= 7; index += 1) {
			documents.push({
				id: `kb_${module.key}_case_${String(index).padStart(2, '0')}`,
				projectId: projectID,
				title: `${module.name}场景${index} ${module.marker}`,
				source: `scenario:${module.key}:${index}`,
				url: module.url,
				tags: [module.key, module.marker, `case-${index}`],
				content: [
					`# ${module.name}场景${index}`,
					`${module.marker} 页面地址硬规则：${module.url}。`,
					`目标：${module.task}，优先读取 ${module.terms[index % module.terms.length]}。`,
					`操作：确认标题为 ${module.title}，再用 ${module.controls[0].name} 和 ${module.controls[1].name}。`,
					`避免：不要跳转到其他模块，即使其他模块也出现状态、负责人、审批、查询等通用词。`,
				].join('\n'),
			})
		}
	}

	for (let index = 0; index < 36; index += 1) {
		const module = modules[index % modules.length]
		const other = modules[(index + 5) % modules.length]
		documents.push({
			id: `kb_decoy_cross_module_${String(index + 1).padStart(2, '0')}`,
			projectId: projectID,
			title: `跨页面诱饵${index + 1} ${other.marker}`,
			source: `decoy:${other.key}:${index + 1}`,
			url: other.url,
			tags: ['decoy', other.key, other.marker],
			content: [
				`# 跨页面诱饵${index + 1}`,
				`${other.marker} 这个文档属于 ${other.url}，不是 ${module.url}。`,
				`${decoyTemplates[index % decoyTemplates.length]}`,
				`故意包含相似词：${module.task}，${module.terms.slice(0, 4).join('、')}。`,
				`正确行为：当当前 URL 是 ${module.url} 时，这条 ${other.marker} 不应被注入给 agent。`,
			].join('\n'),
		})
	}

	return documents
}

function buildCases() {
	return modules.flatMap((module, index) => {
		const neighbor = modules[(index + 1) % modules.length]
		return [
			{
				id: `${module.key}-primary`,
				expectedModule: module,
				forbiddenModules: modules.filter((item) => item.key !== module.key),
				task: module.task,
				currentUrl: module.url,
				title: module.title,
				visibleText: [module.name, ...module.terms.slice(0, 4)],
				controls: module.controls,
			},
			{
				id: `${module.key}-ambiguous-status`,
				expectedModule: module,
				forbiddenModules: modules.filter((item) => item.key !== module.key),
				task: `在当前页面查询状态、负责人和结果详情，避免进入${neighbor.name}`,
				currentUrl: `${module.url}?from=dashboard&tab=summary`,
				title: module.title,
				visibleText: ['状态', '负责人', '查询', ...module.terms.slice(0, 2)],
				controls: module.controls,
			},
		]
	})
}

function evidenceText(context) {
	return (context.knowledgeEvidence ?? [])
		.map((item) => [item.chunkId, item.title, item.snippet, String(item.score ?? '')].join('\n'))
		.join('\n')
}

function summarizeCase(testCase, context) {
	const allText = evidenceText(context)
	const expectedHit = allText.includes(testCase.expectedModule.marker)
	const expectedTopHit = allText.includes(testCase.expectedModule.marker)
	const wrongMarkers = testCase.forbiddenModules
		.filter((module) => allText.includes(module.marker))
		.map((module) => module.marker)
	const wrongTopMarkers = testCase.forbiddenModules
		.filter((module) => allText.includes(module.marker))
		.map((module) => module.marker)
	const promptWrongMarkers = testCase.forbiddenModules
		.filter((module) => context.contextPrompt.includes(module.marker))
		.map((module) => module.marker)
	return {
		id: testCase.id,
		task: testCase.task,
		currentUrl: testCase.currentUrl,
		expectedMarker: testCase.expectedModule.marker,
		recommendedMode: context.recommendedMode,
		expectedHit,
		expectedTopHit,
		wrongMarkers,
		wrongTopMarkers,
		promptWrongMarkers,
		knowledgeEvidence: (context.knowledgeEvidence ?? []).map((item) => ({
			chunkId: item.chunkId,
			title: item.title,
			score: item.score,
		})),
		contextPrompt: context.contextPrompt,
	}
}

function buildMarkdownReport(summary, caseResults) {
	const rows = caseResults
		.map((item) =>
			[
				item.expectedHit && item.wrongMarkers.length === 0 ? 'PASS' : 'FAIL',
				item.id,
				item.expectedMarker,
				item.wrongMarkers.join(', ') || '-',
				item.knowledgeEvidence.map((evidence) => evidence.title).join('<br>'),
			].join(' | ')
		)
		.join('\n')
	return [
		'# WebOps Memory V2 Knowledge Accuracy E2E',
		'',
		`- Project ID: ${projectID}`,
		`- Seeded documents: ${summary.seededDocuments}`,
		`- Test cases: ${summary.totalCases}`,
		`- Expected hit accuracy: ${(summary.expectedHitAccuracy * 100).toFixed(2)}%`,
		`- Top evidence expected hit accuracy: ${(summary.expectedTopHitAccuracy * 100).toFixed(2)}%`,
		`- Wrong page hit rate: ${(summary.wrongHitRate * 100).toFixed(2)}%`,
		`- Wrong top evidence hit rate: ${(summary.wrongTopHitRate * 100).toFixed(2)}%`,
		'',
		'| Result | Case | Expected | Wrong markers | Evidence titles |',
		'| --- | --- | --- | --- | --- |',
		rows,
		'',
		`Full retrieved contexts: ${contextsPath}`,
		`Full injected prompts: ${promptsPath}`,
	].join('\n')
}

function buildPromptLog(caseResults) {
	return caseResults
		.map((item) =>
			[
				`## ${item.id}`,
				'',
				`- Expected: ${item.expectedMarker}`,
				`- Wrong markers: ${item.wrongMarkers.join(', ') || '-'}`,
				'',
				'```xml',
				item.contextPrompt,
				'```',
			].join('\n')
		)
		.join('\n\n')
}

async function main() {
	const documents = buildDocuments()
	const cases = buildCases()

	const seeded = await post('/api/memory/documents', {
		projectId: projectID,
		documents,
	})
	assert(
		seeded.documentIds?.length === documents.length,
		'all generated documents should be ingested'
	)
	fs.writeFileSync(seededDocumentsPath, JSON.stringify({ projectID, documents, seeded }, null, 2))

	const caseResults = []
	const rawContexts = []
	for (const testCase of cases) {
		const context = await post('/api/memory/context', {
			projectId: projectID,
			task: testCase.task,
			currentUrl: testCase.currentUrl,
			pageObservation: {
				title: testCase.title,
				visibleText: testCase.visibleText,
				controls: testCase.controls,
			},
			riskPolicy: { blocked: ['destructive'] },
			mode: 'knowledge_accuracy_e2e',
		})
		rawContexts.push({ case: testCase.id, context })
		caseResults.push(summarizeCase(testCase, context))
	}

	const inspector = await get(`/api/memory/inspector?projectId=${encodeURIComponent(projectID)}`)
	fs.writeFileSync(inspectorPath, JSON.stringify(inspector, null, 2))
	fs.writeFileSync(contextsPath, JSON.stringify({ projectID, caseResults, rawContexts }, null, 2))
	fs.writeFileSync(promptsPath, buildPromptLog(caseResults))

	const expectedHits = caseResults.filter((item) => item.expectedHit).length
	const expectedTopHits = caseResults.filter((item) => item.expectedTopHit).length
	const wrongHits = caseResults.filter((item) => item.wrongMarkers.length > 0).length
	const wrongTopHits = caseResults.filter((item) => item.wrongTopMarkers.length > 0).length
	const summary = {
		projectID,
		seededDocuments: documents.length,
		totalCases: caseResults.length,
		expectedHits,
		expectedTopHits,
		wrongHits,
		wrongTopHits,
		expectedHitAccuracy: expectedHits / caseResults.length,
		expectedTopHitAccuracy: expectedTopHits / caseResults.length,
		wrongHitRate: wrongHits / caseResults.length,
		wrongTopHitRate: wrongTopHits / caseResults.length,
		artifactDir,
		seededDocumentsPath,
		contextsPath,
		promptsPath,
		reportJSONPath,
		reportMDPath,
		inspectorPath,
		failures: caseResults.filter((item) => !item.expectedHit || item.wrongMarkers.length > 0),
	}

	fs.writeFileSync(reportJSONPath, JSON.stringify({ summary, caseResults }, null, 2))
	fs.writeFileSync(reportMDPath, buildMarkdownReport(summary, caseResults))

	console.log(JSON.stringify(summary, null, 2))

	assert(
		summary.expectedHitAccuracy >= 0.95,
		'expected knowledge should be recalled for at least 95% of cases'
	)
	assert(
		summary.expectedTopHitAccuracy >= 0.9,
		'expected knowledge should appear in top evidence for at least 90% of cases'
	)
	assert(summary.wrongHitRate === 0, 'knowledge context should not include other-page markers')
	assert(
		summary.wrongTopHitRate === 0,
		'top knowledge evidence should not include other-page markers'
	)
}

main().catch((error) => {
	console.error(error)
	process.exitCode = 1
})
