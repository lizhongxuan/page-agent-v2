package replay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/page-agent/workflow-backend/internal/registry"
)

type PlaywrightRunnerConfig struct {
	WorkingDir string
	NodeBin    string
	Timeout    time.Duration
	Headless   bool
}

type PlaywrightRunner struct {
	workingDir string
	nodeBin    string
	timeout    time.Duration
	headless   bool
}

func NewPlaywrightRunner(config PlaywrightRunnerConfig) *PlaywrightRunner {
	nodeBin := config.NodeBin
	if nodeBin == "" {
		nodeBin = "node"
	}
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	workingDir := config.WorkingDir
	if workingDir == "" {
		workingDir = findPackageRoot()
	}
	return &PlaywrightRunner{
		workingDir: workingDir,
		nodeBin:    nodeBin,
		timeout:    timeout,
		headless:   config.Headless,
	}
}

func (runner *PlaywrightRunner) RunChunk(ctx context.Context, request ChunkRunRequest) error {
	if request.URL == "" {
		return errors.New("playwright runner requires a current URL")
	}
	return runner.run(ctx, playwrightChunkPayload{
		URL:      request.URL,
		Headless: runner.headless,
		Chunk:    request.Chunk,
		Chunks:   []registry.WorkflowChunk{request.Chunk},
		Bindings: request.Bindings,
	}, request.Chunk.ID)
}

func (runner *PlaywrightRunner) RunWorkflow(ctx context.Context, request WorkflowRunRequest) error {
	if request.URL == "" {
		return errors.New("playwright runner requires a current URL")
	}
	_, err := runner.runDetailed(ctx, playwrightChunkPayload{
		URL:                request.URL,
		Headless:           runner.headless,
		Chunks:             request.Workflow.Chunks,
		Bindings:           request.Bindings,
		RepairPatches:      request.RepairPatches,
		InterruptHandlers:  request.InterruptHandlers,
		InterruptWorkflows: request.InterruptWorkflows,
	}, request.Workflow.ID)
	return err
}

func (runner *PlaywrightRunner) RunWorkflowDetailed(ctx context.Context, request WorkflowRunRequest) (RunExecution, error) {
	if request.URL == "" {
		return RunExecution{}, errors.New("playwright runner requires a current URL")
	}
	return runner.runDetailed(ctx, playwrightChunkPayload{
		URL:                request.URL,
		Headless:           runner.headless,
		Chunks:             request.Workflow.Chunks,
		Bindings:           request.Bindings,
		RepairPatches:      request.RepairPatches,
		InterruptHandlers:  request.InterruptHandlers,
		InterruptWorkflows: request.InterruptWorkflows,
	}, request.Workflow.ID)
}

func (runner *PlaywrightRunner) run(ctx context.Context, payload playwrightChunkPayload, label string) error {
	_, err := runner.runDetailed(ctx, payload, label)
	return err
}

func (runner *PlaywrightRunner) runDetailed(ctx context.Context, payload playwrightChunkPayload, label string) (RunExecution, error) {
	runCtx, cancel := context.WithTimeout(ctx, runner.timeout)
	defer cancel()

	runDir := filepath.Join(runner.workingDir, ".page-agent-playwright-runner")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return RunExecution{}, err
	}
	payloadPath := filepath.Join(runDir, "chunk.json")
	resultPath := filepath.Join(runDir, "result-"+strings.NewReplacer("/", "_", ":", "_").Replace(label)+".json")
	scriptPath := filepath.Join(runDir, "run-chunk.cjs")
	payload.ResultPath = resultPath
	payloadJSON, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return RunExecution{}, err
	}
	if err := os.WriteFile(payloadPath, payloadJSON, 0o600); err != nil {
		return RunExecution{}, err
	}
	if err := os.WriteFile(scriptPath, []byte(playwrightRunnerScript), 0o700); err != nil {
		return RunExecution{}, err
	}

	command := exec.CommandContext(runCtx, runner.nodeBin, scriptPath, payloadPath)
	command.Dir = runner.workingDir
	output, err := command.CombinedOutput()
	execution := readRunExecution(resultPath)
	if err != nil {
		if execution.FailureReason == "" {
			execution.FailureReason = strings.TrimSpace(string(output))
		}
		return execution, fmt.Errorf("playwright run %s failed: %w: %s", label, err, strings.TrimSpace(string(output)))
	}
	return execution, nil
}

type playwrightChunkPayload struct {
	URL                string                             `json:"url"`
	Headless           bool                               `json:"headless"`
	Chunk              registry.WorkflowChunk             `json:"chunk"`
	Chunks             []registry.WorkflowChunk           `json:"chunks,omitempty"`
	Bindings           map[string]string                  `json:"bindings"`
	RepairPatches      []registry.RepairPatch             `json:"repairPatches,omitempty"`
	InterruptHandlers  []registry.InterruptHandler        `json:"interruptHandlers,omitempty"`
	InterruptWorkflows map[string]registry.WorkflowRecipe `json:"interruptWorkflows,omitempty"`
	ResultPath         string                             `json:"resultPath,omitempty"`
}

func readRunExecution(path string) RunExecution {
	content, err := os.ReadFile(path)
	if err != nil {
		return RunExecution{}
	}
	var execution RunExecution
	if err := json.Unmarshal(content, &execution); err != nil {
		return RunExecution{}
	}
	return execution
}

func findPackageRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}

func renderTemplate(value string, bindings map[string]string) string {
	result := value
	for key, binding := range bindings {
		result = strings.ReplaceAll(result, "{{"+key+"}}", binding)
		result = strings.ReplaceAll(result, "{"+key+"}", binding)
	}
	return result
}

func waitDurationMillis(value string) int {
	if value == "" {
		return 500
	}
	if number, err := strconv.Atoi(value); err == nil && number > 0 {
		return number
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 500
	}
	return int(parsed / time.Millisecond)
}

const playwrightRunnerScript = `
const fs = require('node:fs')
const { chromium } = require('playwright')

const payload = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'))
const result = {
	logs: [],
	selectorStats: [],
	artifactRefs: [],
}

function render(value) {
	if (!value) return ''
	let result = String(value)
	for (const [key, binding] of Object.entries(payload.bindings || {})) {
		result = result.replaceAll('{{' + key + '}}', String(binding))
		result = result.replaceAll('{' + key + '}', String(binding))
	}
	return result
}

function writeResult() {
	if (!payload.resultPath) return
	fs.writeFileSync(payload.resultPath, JSON.stringify(result, null, 2))
}

function log(event, detail = {}) {
	result.logs.push({
		timestamp: new Date().toISOString(),
		event,
		...detail,
	})
}

function targetSelectorText(target) {
	if (!target) return ''
	return [target.strategy, target.value, target.role, target.name].filter(Boolean).map(render).join(' ')
}

function recordSelectorAttempt(step, target, success, failureReason = '') {
	const selector = targetSelectorText(target) || String(step.type || '')
	const attempt = {
		timestamp: new Date().toISOString(),
		stepId: step.id || '',
		strategy: target?.strategy || '',
		selector,
		result: success ? 'success' : 'failed',
		failureReason,
	}
	log('selector_attempted', {
		stepId: step.id || '',
		selectorAttempts: [attempt],
	})
	result.selectorStats.push({
		stepId: step.id || '',
		strategy: target?.strategy || '',
		selector,
		successCount: success ? 1 : 0,
		failCount: success ? 0 : 1,
		lastFailureReason: failureReason,
	})
}

function locatorFor(page, target) {
	const renderedValue = render(target.value || '')
	const renderedName = render(target.name || '')
	switch (target.strategy) {
		case 'role':
			return page.getByRole(target.role, renderedName ? { name: renderedName } : undefined)
		case 'label':
			return page.getByLabel(renderedValue || renderedName)
		case 'placeholder':
			return page.getByPlaceholder(renderedValue || renderedName)
		case 'test_id':
			return page.getByTestId(renderedValue || renderedName)
		case 'text':
			return page.getByText(renderedValue || renderedName)
		case 'css':
			return page.locator(renderedValue)
		case 'xpath':
			return page.locator('xpath=' + renderedValue)
		default:
			throw new Error('Unsupported target strategy: ' + target.strategy)
	}
}

async function resolveLocator(page, step, chunk) {
	const candidates = [step.target?.primary, ...(step.target?.fallbacks || [])].filter(Boolean)
	if (candidates.length === 0) throw new Error('Step has no target: ' + step.id)
	let lastError
	for (let pass = 0; pass < 2; pass += 1) {
		for (const target of candidates) {
			try {
				const locator = locatorFor(page, target).first()
				await locator.waitFor({ state: 'visible', timeout: 5000 })
				recordSelectorAttempt(step, target, true)
				return locator
			} catch (error) {
				lastError = error
				if (pass === 1) recordSelectorAttempt(step, target, false, error?.message || String(error))
			}
		}
		if (pass === 0) {
			const dismissed = await safeDismissInterrupts(page, chunk)
			if (!dismissed) await tryInterruptHandler(page, chunk, 'locator_failed')
		}
	}
	const repaired = await tryRepairPatchTarget(page, step, chunk)
	if (repaired) return repaired
	throw lastError || new Error('No target matched: ' + step.id)
}

function repairPatchForStep(step, chunk) {
	const patches = Array.isArray(payload.repairPatches) ? payload.repairPatches : []
	return patches.find((patch) => {
		if (patch.status && patch.status !== 'active') return false
		if (patch.chunkId && patch.chunkId !== chunk?.id) return false
		if (patch.stepId && patch.stepId !== step?.id) return false
		const risk = patch.riskLevel || ''
		if (risk === 'destructive' || risk === 'external_send') return false
		return targetCandidatesFromPatch(patch).length > 0
	})
}

function targetCandidatesFromPatch(patch) {
	return [patch?.newTarget?.primary, ...(patch?.newTarget?.fallbacks || [])].filter(Boolean)
}

async function tryRepairPatchTarget(page, step, chunk) {
	const patch = repairPatchForStep(step, chunk)
	if (!patch) return undefined
	log('repair_patch_applied', {
		chunkId: chunk?.id || '',
		stepId: step?.id || '',
		appliedRepairPatchId: patch.id || patch.patchId || '',
		message: patch.newTargetSummary || 'Applied repair patch target',
	})
	let lastError
	for (const target of targetCandidatesFromPatch(patch)) {
		try {
			const locator = locatorFor(page, target).first()
			await locator.waitFor({ state: 'visible', timeout: 5000 })
			recordSelectorAttempt(step, target, true)
			return locator
		} catch (error) {
			lastError = error
			recordSelectorAttempt(step, target, false, error?.message || String(error))
		}
	}
	throw lastError || new Error('Repair patch target did not match: ' + step.id)
}

async function safeDismissInterrupts(page, chunk) {
	const labels = ['Got it', 'Close', 'Dismiss', 'OK', 'Ok', 'Cancel', 'No thanks', 'Maybe later', 'Not now', 'Skip']
	for (const label of labels) {
		try {
			const button = page.getByRole('button', { name: label }).first()
			if (await button.isVisible({ timeout: 250 }).catch(() => false)) {
				log('interrupt_detected', {
					chunkId: chunk?.id || '',
					message: 'Common dialog action is visible: ' + label,
				})
				await button.click()
				log('interrupt_handler_applied', {
					chunkId: chunk?.id || '',
					appliedInterruptHandlerId: 'builtin_common_dialog_dismiss',
					message: 'Clicked ' + label,
				})
				await page.waitForTimeout(150)
				return true
			}
		} catch (error) {
			// Continue checking other low-risk dismiss controls.
		}
	}
	return false
}

async function controlVisible(page, control) {
	if (!control) return true
	try {
		if (control.role && control.name) {
			return await page.getByRole(control.role, { name: control.name }).first().isVisible({ timeout: 250 }).catch(() => false)
		}
		if (control.name) {
			return await page.getByText(control.name).first().isVisible({ timeout: 250 }).catch(() => false)
		}
		return true
	} catch {
		return false
	}
}

function handlerRiskAllowed(handler) {
	const risk = handler?.riskLevel || ''
	return risk !== 'destructive' && risk !== 'external_send'
}

function handlerAppliesToChunk(handler, chunk) {
	const states = Array.isArray(handler?.appliesToPageStates) ? handler.appliesToPageStates : []
	if (states.length === 0) return true
	return states.includes(chunk?.fromPageState || '') || states.includes(chunk?.toPageState || '')
}

async function handlerMatchesPage(page, handler, chunk, observation) {
	if (!handlerRiskAllowed(handler)) return false
	if (!handlerAppliesToChunk(handler, chunk)) return false
	const requiredText = Array.isArray(handler.requiredText) ? handler.requiredText : []
	for (const text of requiredText) {
		if (!observation.bodyText.includes(render(text))) return false
	}
	const controls = Array.isArray(handler.targetControls) ? handler.targetControls : []
	for (const control of controls) {
		if (!(await controlVisible(page, control))) return false
	}
	return true
}

async function tryInterruptHandler(page, chunk, reason, options = {}) {
	if (options.skipInterrupts) return false
	const handlers = Array.isArray(payload.interruptHandlers) ? payload.interruptHandlers : []
	if (handlers.length === 0) {
		log('interrupt_handler_not_found', {
			chunkId: chunk?.id || '',
			reason,
			message: 'No active interrupt handlers are available.',
		})
		return false
	}
	const observation = await currentPageObservation(page)
	for (const handler of handlers) {
		if (!(await handlerMatchesPage(page, handler, chunk, observation))) continue
		const handlerId = handler.id || handler.handlerId || ''
		log('interrupt_detected', {
			chunkId: chunk?.id || '',
			appliedInterruptHandlerId: handlerId,
			reason,
			message: handler.interruptType || 'interrupt handler matched',
		})
		const workflow = payload.interruptWorkflows?.[handler.workflowId]
		if (!workflow || !Array.isArray(workflow.chunks)) {
			log('interrupt_handler_not_found', {
				chunkId: chunk?.id || '',
				appliedInterruptHandlerId: handlerId,
				reason,
				message: 'Matched interrupt handler has no executable workflow.',
			})
			continue
		}
		await runWorkflowChunks(page, workflow.chunks, {
			skipInterrupts: true,
			interruptHandlerId: handlerId,
		})
		log('interrupt_handler_applied', {
			chunkId: chunk?.id || '',
			appliedInterruptHandlerId: handlerId,
			message: 'Executed interrupt handler workflow.',
		})
		log('interrupt_retrying_original_chunk', {
			chunkId: chunk?.id || '',
			appliedInterruptHandlerId: handlerId,
			reason,
		})
		await page.waitForTimeout(150)
		return true
	}
	log('interrupt_handler_not_found', {
		chunkId: chunk?.id || '',
		reason,
		message: 'No interrupt handler matched the current page.',
	})
	return false
}

async function bodyContains(page, text) {
	const expected = render(text)
	if (!expected) return true
	const body = await page.locator('body').innerText({ timeout: 2000 }).catch(() => '')
	return body.includes(expected)
}

async function currentPageObservation(page) {
	return {
		url: page.url(),
		title: await page.title().catch(() => ''),
		bodyText: await page.locator('body').innerText({ timeout: 2000 }).catch(() => ''),
	}
}

function slug(value) {
	return String(value || '')
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, '_')
		.replace(/^_+|_+$/g, '')
}

function urlInfo(url) {
	try {
		return new URL(url)
	} catch {
		return undefined
	}
}

function pageStateMatches(expectedPageState, observation, chunk) {
	const expected = render(expectedPageState).toLowerCase().trim()
	if (!expected) return true
	const haystack = [observation.url, observation.title, observation.bodyText]
		.join(' ')
		.toLowerCase()
	if (haystack.includes(expected)) return true
	const parsed = urlInfo(observation.url)
	const path = parsed?.pathname || ''
	const hostSlug = slug(parsed?.hostname || '')
	if (expected === 'github_repo_home') {
		const segments = path.split('/').filter(Boolean)
		if (parsed?.hostname === 'github.com' && segments.length >= 2 && !path.includes('/issues')) return true
	}
	if (expected.includes('github_issues') && (path.includes('/issues') || haystack.includes('issues'))) {
		return true
	}
	if (hostSlug && expected.startsWith(hostSlug)) return true
	if (chunk?.preconditionText && observation.bodyText.includes(render(chunk.preconditionText))) {
		return true
	}
	const ignored = new Set(['page', 'home', 'list', 'detail', 'state', 'web'])
	const tokens = expected
		.split(/[^a-z0-9]+/g)
		.filter((token) => token.length > 2 && !ignored.has(token))
	if (tokens.length === 0) return true
	return tokens.every((token) => haystack.includes(token))
}

async function validateFromPageState(page, chunk) {
	if (!chunk?.fromPageState) return
	const observation = await currentPageObservation(page)
	if (pageStateMatches(chunk.fromPageState, observation, chunk)) {
		log('from_page_state_validated', {
			chunkId: chunk.id || '',
			pageState: chunk.fromPageState || '',
		})
		return
	}
	log('from_page_state_mismatch', {
		chunkId: chunk.id || '',
		expectedPageState: chunk.fromPageState || '',
		message: 'Current page does not satisfy the expected workflow start page state.',
	})
	throw chunkFailure(
		chunk,
		'from_page_state_mismatch',
		'from_page_state_mismatch: expected ' + chunk.fromPageState
	)
}

function chunkFailure(chunk, reason, message) {
	const error = new Error(message)
	error.chunkId = chunk?.id || ''
	error.reason = reason
	return error
}

async function runStep(page, step, chunk) {
	if (step.type === 'wait') {
		const value = render(step.value)
		const millis = Number.parseInt(value || '500', 10)
		await page.waitForTimeout(Number.isFinite(millis) && millis > 0 ? millis : 500)
		return
	}
	const locator = await resolveLocator(page, step, chunk)
	if (step.type === 'click') {
		await locator.click()
		return
	}
	if (step.type === 'fill') {
		await locator.fill(render(step.value))
		return
	}
	if (step.type === 'press') {
		await locator.press(render(step.key || step.value))
		return
	}
	if (step.type === 'select') {
		await locator.selectOption(render(step.value))
		return
	}
	throw new Error('Unsupported step type: ' + step.type)
}

async function runWorkflowChunk(page, chunk, options = {}) {
	log('chunk_started', {
		chunkId: chunk.id || '',
		pageState: chunk.fromPageState || '',
		appliedInterruptHandlerId: options.interruptHandlerId || '',
	})
	if (!options.skipInterrupts) {
		const dismissed = await safeDismissInterrupts(page, chunk)
		if (!dismissed) await tryInterruptHandler(page, chunk, 'before_chunk', options)
	}
	await validateFromPageState(page, chunk)
	if (chunk.preconditionText && !(await bodyContains(page, chunk.preconditionText))) {
		const repaired = await tryInterruptHandler(page, chunk, 'precondition_text_missing', options)
		if (!repaired || !(await bodyContains(page, chunk.preconditionText))) {
			throw chunkFailure(
				chunk,
				'precondition_text_missing',
				'precondition_text_missing: ' + render(chunk.preconditionText)
			)
		}
	}
	for (const step of chunk.steps || []) {
		try {
			await runStep(page, step, chunk)
		} catch (error) {
			error.chunkId = error.chunkId || chunk.id || ''
			error.stepId = error.stepId || step.id || ''
			error.reason = error.reason || 'step_failed'
			throw error
		}
	}
	if (chunk.postconditionText && !(await bodyContains(page, chunk.postconditionText))) {
		log('postcondition_failed', {
			chunkId: chunk.id || '',
			reason: 'postcondition_text_missing',
			message: render(chunk.postconditionText),
		})
		throw chunkFailure(
			chunk,
			'postcondition_text_missing',
			'postcondition_text_missing: ' + render(chunk.postconditionText)
		)
	}
	log('chunk_finished', {
		chunkId: chunk.id || '',
		pageState: chunk.toPageState || '',
		appliedInterruptHandlerId: options.interruptHandlerId || '',
	})
}

async function runWorkflowChunks(page, chunks, options = {}) {
	for (const chunk of (chunks || []).filter(Boolean)) {
		await runWorkflowChunk(page, chunk, options)
	}
}

(async () => {
	const browser = await chromium.launch({ headless: payload.headless !== false })
	const page = await browser.newPage()
	try {
		await page.goto(payload.url, { waitUntil: 'domcontentloaded' })
		const chunks = payload.chunks && payload.chunks.length > 0 ? payload.chunks : [payload.chunk]
		await runWorkflowChunks(page, chunks)
		await browser.close()
	} catch (error) {
		result.failedChunkId = error?.chunkId || 'workflow'
		result.failedStepId = error?.stepId || ''
		result.failureReason = error?.message || String(error)
		log('chunk_failed', {
			chunkId: result.failedChunkId,
			stepId: result.failedStepId,
			reason: error?.reason || 'playwright_error',
			message: result.failureReason,
		})
		if (payload.resultPath) {
			const screenshotPath = payload.resultPath.replace(/\.json$/, '-failure.png')
			await page.screenshot({ path: screenshotPath, fullPage: true }).then(() => {
				result.artifactRefs.push({
					id: 'failure_screenshot',
					kind: 'screenshot',
					contentType: 'image/png',
					path: screenshotPath,
				})
			}).catch(() => {})
		}
		await browser.close()
		writeResult()
		throw error
	} finally {
		writeResult()
	}
})().catch((error) => {
	console.error(error && error.stack ? error.stack : error)
	process.exit(1)
})
`
