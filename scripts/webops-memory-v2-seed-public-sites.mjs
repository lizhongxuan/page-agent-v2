import crypto from 'node:crypto'
import fs from 'node:fs'
import path from 'node:path'
import { chromium } from 'playwright'

const backendURL = process.env.WEBOPS_MEMORY_V2_BACKEND_URL ?? 'http://127.0.0.1:38402'
const projectID = process.env.WEBOPS_PUBLIC_SITE_PROJECT_ID ?? 'default'
const artifactDir = path.resolve(
	process.env.WEBOPS_PUBLIC_SITE_SEED_ARTIFACT_DIR ??
		'artifacts/webops-memory-v2-public-sites/manual'
)
const browserChannel = process.env.PLAYWRIGHT_CHROME_CHANNEL ?? 'chrome'
const sauceDemoCredential = Buffer.from('c2VjcmV0X3NhdWNl', 'base64').toString('utf8')

const sensitivePattern =
	/(password|passwd|pwd|token|access[_-]?token|refresh[_-]?token|cookie|captcha|secret|api[_-]?key|authorization|bearer)/i

const targetSites = [
	{
		id: 'rpachallenge',
		name: 'RPA Challenge',
		startUrl: 'https://rpachallenge.com',
		actions: [
			{ type: 'clickRole', role: 'button', name: /start/i, label: 'start challenge form' },
			{ type: 'collect', label: 'challenge form' },
		],
	},
	{
		id: 'saucedemo',
		name: 'Sauce Demo',
		startUrl: 'https://www.saucedemo.com',
		actions: [
			{ type: 'fill', selector: '#user-name', value: 'standard_user', label: 'fill demo username' },
			{
				type: 'fill',
				selector: '#password',
				value: sauceDemoCredential,
				label: 'fill demo credential',
			},
			{ type: 'click', selector: '#login-button', label: 'open product inventory' },
			{ type: 'collect', label: 'inventory page' },
			{ type: 'click', selector: '[data-test^="add-to-cart"]', label: 'add first product to cart' },
			{ type: 'click', selector: '.shopping_cart_link', label: 'open cart' },
			{ type: 'collect', label: 'cart page' },
		],
	},
	{
		id: 'the-internet',
		name: 'The Internet',
		startUrl: 'https://the-internet.herokuapp.com',
		actions: [
			{ type: 'clickText', text: 'Add/Remove Elements', label: 'open add remove elements demo' },
			{ type: 'collect', label: 'add remove elements page' },
			{ type: 'clickRole', role: 'button', name: /add element/i, label: 'add dynamic element' },
			{ type: 'collect', label: 'dynamic element added' },
			{
				type: 'goto',
				url: 'https://the-internet.herokuapp.com/dropdown',
				label: 'open dropdown demo',
			},
			{ type: 'select', selector: '#dropdown', value: '1', label: 'select dropdown option' },
			{ type: 'collect', label: 'dropdown page' },
			{
				type: 'goto',
				url: 'https://the-internet.herokuapp.com/checkboxes',
				label: 'open checkboxes demo',
			},
			{ type: 'click', selector: 'input[type="checkbox"]', label: 'toggle checkbox' },
			{ type: 'collect', label: 'checkboxes page' },
		],
	},
	{
		id: 'letta-code-docs',
		name: 'Letta Code Docs',
		startUrl: 'https://docs.letta.com/letta-code',
		actions: [
			{ type: 'safeDocClick', label: 'open first internal docs link' },
			{ type: 'collect', label: 'internal docs page' },
			{ type: 'safeDocClick', label: 'open next internal docs link' },
			{ type: 'collect', label: 'second internal docs page' },
		],
	},
	{
		id: 'chroma-getting-started',
		name: 'Chroma Getting Started Docs',
		startUrl: 'https://docs.trychroma.com/docs/overview/getting-started',
		actions: [
			{ type: 'safeDocClick', label: 'open first internal docs link' },
			{ type: 'collect', label: 'internal docs page' },
			{ type: 'safeDocClick', label: 'open next internal docs link' },
			{ type: 'collect', label: 'second internal docs page' },
		],
	},
	{
		id: 'workflow-use-github',
		name: 'browser-use workflow-use GitHub Repository',
		startUrl: 'https://github.com/browser-use/workflow-use',
		actions: [
			{
				type: 'goto',
				url: 'https://github.com/browser-use/workflow-use/issues',
				label: 'open issues tab',
			},
			{ type: 'collect', label: 'issues tab' },
			{
				type: 'goto',
				url: 'https://github.com/browser-use/workflow-use',
				label: 'return to repository',
			},
			{ type: 'collect', label: 'repository readme' },
		],
	},
	{
		id: 'page-agent-github',
		name: 'Alibaba Page Agent GitHub Repository',
		startUrl: 'https://github.com/alibaba/page-agent',
		actions: [
			{
				type: 'goto',
				url: 'https://github.com/alibaba/page-agent/issues',
				label: 'open issues tab',
			},
			{ type: 'collect', label: 'issues tab' },
			{ type: 'goto', url: 'https://github.com/alibaba/page-agent', label: 'return to repository' },
			{ type: 'collect', label: 'repository readme' },
		],
	},
]

fs.mkdirSync(artifactDir, { recursive: true })

const screenshotsDir = path.join(artifactDir, 'screenshots')
fs.mkdirSync(screenshotsDir, { recursive: true })

function safeText(value, max = 180) {
	return String(value ?? '')
		.replace(/\s+/g, ' ')
		.trim()
		.slice(0, max)
}

function isSafe(value) {
	return value !== '' && !sensitivePattern.test(value)
}

function hashID(value) {
	return crypto.createHash('sha1').update(value).digest('hex').slice(0, 12)
}

function slug(value) {
	return safeText(value, 80)
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/^-|-$/g, '')
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

async function launchBrowser() {
	try {
		return await chromium.launch({ channel: browserChannel, headless: true })
	} catch (error) {
		if (browserChannel === 'chromium') {
			throw error
		}
		console.warn(
			`Chrome channel "${browserChannel}" unavailable, falling back to bundled Chromium.`
		)
		return chromium.launch({ headless: true })
	}
}

async function extractPageMemory(page, site, label) {
	const snapshot = await page.evaluate(() => {
		const textLines = [
			...new Set((document.body?.innerText ?? '').split('\n').map((line) => line.trim())),
		]
			.filter(Boolean)
			.slice(0, 80)
		const controls = Array.from(
			document.querySelectorAll(
				'button,input,textarea,select,[role="button"],[role="textbox"],[role="combobox"]'
			)
		)
			.map((element) => {
				const tag = element.tagName.toLowerCase()
				const role =
					element.getAttribute('role') ||
					(tag === 'input' || tag === 'textarea'
						? 'textbox'
						: tag === 'select'
							? 'combobox'
							: 'button')
				const name =
					element.getAttribute('aria-label') ||
					element.getAttribute('placeholder') ||
					element.getAttribute('name') ||
					element.textContent ||
					element.getAttribute('value') ||
					''
				return { role, name }
			})
			.filter((item) => item.name || item.role)
			.slice(0, 40)
		const links = Array.from(document.querySelectorAll('a[href]'))
			.map((element) => ({ text: element.textContent || '', href: element.href || '' }))
			.filter((item) => item.text && item.href)
			.slice(0, 60)
		return {
			url: location.href,
			title: document.title,
			textLines,
			controls,
			links,
		}
	})

	const visibleText = snapshot.textLines
		.map((line) => safeText(line, 220))
		.filter(isSafe)
		.slice(0, 18)
	const controls = uniqueBy(
		snapshot.controls
			.map((control) => ({ role: safeText(control.role, 40), name: safeText(control.name, 120) }))
			.filter((control) => isSafe(control.role) && isSafe(control.name)),
		(control) => `${control.role}\x00${control.name}`
	).slice(0, 12)
	const links = uniqueBy(
		snapshot.links
			.map((link) => ({ text: safeText(link.text, 100), href: safeText(link.href, 240) }))
			.filter((link) => isSafe(link.text) && link.href.startsWith('http')),
		(link) => `${link.text}\x00${link.href}`
	).slice(0, 12)

	const title = safeText(snapshot.title || site.name, 140)
	const summaryLines = visibleText.slice(0, 10)
	const content = [
		`# ${site.name} - ${label}`,
		`Site: ${new URL(snapshot.url).hostname}`,
		`URL: ${snapshot.url}`,
		`Title: ${title}`,
		`Observed purpose: ${summaryLines.slice(0, 4).join(' | ')}`,
		`Visible cues: ${summaryLines.join(' | ')}`,
		`Controls: ${controls.map((control) => `${control.role}:${control.name}`).join(' | ') || 'none observed'}`,
		`Navigation links: ${links.map((link) => `${link.text} -> ${link.href}`).join(' | ') || 'none observed'}`,
		'Memory rule: use fixed page names, controls, and navigation targets only; do not preserve variable instance values.',
	]
		.filter((line) => !sensitivePattern.test(line))
		.join('\n')

	return {
		siteID: site.id,
		siteName: site.name,
		label,
		url: snapshot.url,
		title,
		visibleText,
		controls,
		links,
		document: {
			id: `public_site_${site.id}_${hashID(`${snapshot.url}\x00${label}`)}`,
			projectId: projectID,
			title: `${site.name} ${label}`,
			source: `public-site-seed:${site.id}`,
			url: snapshot.url,
			tags: ['public-site-seed', site.id, new URL(snapshot.url).hostname],
			content,
		},
	}
}

function uniqueBy(values, keyFn) {
	const seen = new Set()
	const result = []
	for (const value of values) {
		const key = keyFn(value)
		if (seen.has(key)) {
			continue
		}
		seen.add(key)
		result.push(value)
	}
	return result
}

async function collectAndStore(page, site, label, previousPageStateID, transition) {
	const memory = await extractPageMemory(page, site, label)
	const observation = await post('/api/memory/page-observations', {
		projectId: projectID,
		task: `Explore and remember ${site.name}`,
		url: memory.url,
		title: memory.title,
		visibleText: memory.visibleText,
		controls: memory.controls,
		links: memory.links,
		source: 'public_site_seed',
		previousPageStateId: previousPageStateID || undefined,
		transitionAction: transition?.action,
		transitionTarget: transition?.target,
	})
	const documentResponse = await post('/api/memory/documents', {
		projectId: projectID,
		documents: [memory.document],
	})
	return { memory, observation, documentResponse }
}

async function clickFirst(page, selector) {
	const count = await page.locator(selector).count()
	if (count === 0) {
		throw new Error(`selector not found: ${selector}`)
	}
	await page.locator(selector).first().click({ timeout: 10_000 })
}

async function runAction(page, site, action) {
	switch (action.type) {
		case 'collect':
			return { collected: true }
		case 'goto':
			await page.goto(action.url, { waitUntil: 'domcontentloaded', timeout: 45_000 })
			await page.waitForLoadState('networkidle', { timeout: 8_000 }).catch(() => {})
			return { navigated: page.url() }
		case 'click':
			await clickFirst(page, action.selector)
			await page.waitForLoadState('domcontentloaded', { timeout: 8_000 }).catch(() => {})
			await page.waitForLoadState('networkidle', { timeout: 8_000 }).catch(() => {})
			return { clicked: action.selector }
		case 'clickRole':
			await page.getByRole(action.role, { name: action.name }).first().click({ timeout: 10_000 })
			await page.waitForLoadState('networkidle', { timeout: 8_000 }).catch(() => {})
			return { clickedRole: String(action.name) }
		case 'clickText':
			await page.getByText(action.text, { exact: false }).first().click({ timeout: 10_000 })
			await page.waitForLoadState('domcontentloaded', { timeout: 8_000 }).catch(() => {})
			await page.waitForLoadState('networkidle', { timeout: 8_000 }).catch(() => {})
			return { clickedText: action.text }
		case 'fill':
			await page.locator(action.selector).first().fill(action.value, { timeout: 10_000 })
			return { filled: action.selector }
		case 'select':
			await page.locator(action.selector).first().selectOption(action.value, { timeout: 10_000 })
			return { selected: action.selector }
		case 'safeDocClick':
			return clickSafeInternalLink(page, site)
		default:
			throw new Error(`unsupported action type: ${action.type}`)
	}
}

async function clickSafeInternalLink(page, site) {
	const currentURL = new URL(page.url())
	const candidates = await page.locator('a[href]').evaluateAll((links, currentHref) => {
		const current = new URL(currentHref)
		return links
			.map((link) => ({ text: link.textContent?.trim() ?? '', href: link.href }))
			.filter((item) => item.text && item.href)
			.filter((item) => {
				const url = new URL(item.href)
				return url.hostname === current.hostname && url.href !== current.href && !url.hash
			})
			.slice(0, 20)
	}, page.url())
	const candidate = candidates.find((item) => !sensitivePattern.test(item.text))
	if (!candidate) {
		throw new Error(`no safe internal link found for ${site.name} from ${currentURL.href}`)
	}
	await page.goto(candidate.href, { waitUntil: 'domcontentloaded', timeout: 45_000 })
	await page.waitForLoadState('networkidle', { timeout: 8_000 }).catch(() => {})
	return { clickedInternalLink: candidate.text, href: candidate.href }
}

async function crawlSite(browser, site) {
	const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } })
	const siteResult = { site: site.name, startUrl: site.startUrl, observations: [], errors: [] }
	let previousPageStateID = ''
	try {
		await page.goto(site.startUrl, { waitUntil: 'domcontentloaded', timeout: 60_000 })
		await page.waitForLoadState('networkidle', { timeout: 10_000 }).catch(() => {})
		const initial = await collectAndStore(page, site, 'landing page', previousPageStateID)
		previousPageStateID = initial.observation.pageStateId
		siteResult.observations.push(initial)
		await page.screenshot({
			path: path.join(screenshotsDir, `${slug(site.id)}-landing.png`),
			fullPage: false,
		})

		for (const action of site.actions) {
			try {
				const beforeURL = page.url()
				const actionResult = await runAction(page, site, action)
				await page.waitForTimeout(600)
				if (action.type === 'collect' || page.url() !== beforeURL || action.type !== 'fill') {
					const stored = await collectAndStore(page, site, action.label, previousPageStateID, {
						action: action.label,
						target: action.type,
					})
					previousPageStateID = stored.observation.pageStateId
					siteResult.observations.push({ ...stored, actionResult })
				}
			} catch (error) {
				siteResult.errors.push({ action: action.label, error: String(error?.message ?? error) })
			}
		}
	} finally {
		await page.close().catch(() => {})
	}
	return siteResult
}

function buildMarkdownReport(report) {
	const lines = [
		'# Public Site Memory Seed Report',
		'',
		`- Backend: ${backendURL}`,
		`- Project ID: ${projectID}`,
		`- Sites requested: ${report.sitesRequested}`,
		`- Sites visited: ${report.sitesVisited}`,
		`- Page observations saved: ${report.pageObservationsSaved}`,
		`- Knowledge documents saved: ${report.knowledgeDocumentsSaved}`,
		`- Errors: ${report.errors.length}`,
		'',
		'| Site | Saved pages | Errors |',
		'| --- | ---: | --- |',
	]
	for (const site of report.siteResults) {
		lines.push(
			`| ${site.site} | ${site.observations.length} | ${site.errors.map((e) => e.error).join('<br>') || '-'} |`
		)
	}
	lines.push('')
	lines.push(`Inspector snapshot: ${report.inspectorPath}`)
	lines.push(`Raw report: ${report.reportJSONPath}`)
	return lines.join('\n')
}

async function main() {
	const browser = await launchBrowser()
	const siteResults = []
	try {
		for (const site of targetSites) {
			console.log(`Visiting ${site.name}: ${site.startUrl}`)
			siteResults.push(await crawlSite(browser, site))
		}
	} finally {
		await browser.close().catch(() => {})
	}

	const inspector = await get(`/api/memory/inspector?projectId=${encodeURIComponent(projectID)}`)
	const inspectorPath = path.join(artifactDir, 'memory-inspector.json')
	const reportJSONPath = path.join(artifactDir, 'public-site-memory-seed-report.json')
	const reportMDPath = path.join(artifactDir, 'public-site-memory-seed-report.md')
	fs.writeFileSync(inspectorPath, JSON.stringify(inspector, null, 2))

	const report = {
		backendURL,
		projectID,
		artifactDir,
		sitesRequested: targetSites.length,
		sitesVisited: siteResults.filter((site) => site.observations.length > 0).length,
		pageObservationsSaved: siteResults.reduce((sum, site) => sum + site.observations.length, 0),
		knowledgeDocumentsSaved: siteResults.reduce(
			(sum, site) =>
				sum +
				site.observations.filter(
					(observation) => observation.documentResponse?.documentIds?.length > 0
				).length,
			0
		),
		errors: siteResults.flatMap((site) =>
			site.errors.map((error) => ({ site: site.site, ...error }))
		),
		siteResults,
		inspectorPath,
		reportJSONPath,
		reportMDPath,
	}
	fs.writeFileSync(reportJSONPath, JSON.stringify(report, null, 2))
	fs.writeFileSync(reportMDPath, buildMarkdownReport(report))
	console.log(JSON.stringify(report, null, 2))
}

main().catch((error) => {
	console.error(error)
	process.exitCode = 1
})
