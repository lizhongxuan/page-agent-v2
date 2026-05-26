# Page Agent Long-Running Context Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement bounded long-running context management for Page Agent while preserving browser-operation controllability, performance, and correctness.

**Architecture:** Add a focused context layer under `packages/core/src/context/` that builds prompt context from structured sections instead of serializing full history forever. Keep current `history` as the complete UI/debug ledger, but feed the LLM a bounded `ContextPack` with current page state, recent steps, protected facts, and compacted older history. Add a deterministic `ContinuationResolver` so follow-up user input inherits only the context that is valid for the current task, business object, and page.

**Tech Stack:** TypeScript, Vitest, npm workspaces, existing `@page-agent/core` and `@page-agent/ext` packages. No filesystem dependency in runtime code.

---

## Source Design

Primary design: `docs/page-agent-long-running-context-management-design.zh.md`

Reference design: `docs/page-agent-context-manager-claude-code-porting-design.zh.md`

Non-negotiable constraints:

- Do not change `PageController` action semantics.
- Do not use historical DOM indexes for future actions.
- Do not summarize or restore `browser_state` as an action source.
- Keep current page observation as the highest-priority fact.
- Keep old behavior available behind `context.enabled=false`.

## File Map

Create:

- `packages/core/src/context/types.ts`  
  Shared context types: `ContextConfig`, `ContextSection`, `ContextPack`, `ContextBudgetReport`, `BrowserSessionSummary`, `BusinessObjectRef`, `ContinuationDecision`.

- `packages/core/src/context/tokenBudget.ts`  
  Character-based token estimation, section budgeting, and trimming helpers.

- `packages/core/src/context/PageAgentPromptBuilder.ts`  
  Renders both legacy-equivalent prompts and sectioned `ContextPack` prompts.

- `packages/core/src/context/ContextRuntime.ts`  
  Per-task context runtime that builds packs, records steps, stores protected facts, and triggers compact.

- `packages/core/src/context/BrowserCompactManager.ts`  
  Deterministic compact summary generation and compact-boundary helpers.

- `packages/core/src/context/ContinuationResolver.ts`  
  Determines whether new user input should continue the old task, start a new task on the same page, inherit only business facts, or start fresh.

- `packages/core/src/context/ContextStore.ts`  
  Optional persistence interface plus in-memory store.

- `packages/core/src/context/index.ts`  
  Barrel exports for context modules.

- `packages/core/src/context/*.test.ts`  
  Focused unit tests for budget, prompt rendering, compacting, runtime, continuation, and store behavior.

Modify:

- `packages/core/src/types.ts`  
  Add context config types to `AgentConfig`, add compact boundary event type, and expose context budget metadata.

- `packages/core/src/PageAgentCore.ts`  
  Replace direct prompt assembly with `PageAgentPromptBuilder` and optional `ContextRuntime`. Keep legacy path when context is disabled.

- `packages/core/src/prompts/system_prompt.md`  
  Add concise context rules: current browser state is source of truth; history indexes are stale.

- `packages/extension/src/agent/sessionContinuation.ts`  
  Keep current compatibility helpers and add resolver-facing input/output helpers in Task 13.

- `packages/extension/src/agent/useAgent.ts`  
  Phase 5 integration point: route follow-up user input through `ContinuationResolver` before execution.

Verification commands:

```bash
npm test -- packages/core/src/context
npm test -- packages/core/src/searchExplorationGuard.test.ts packages/extension/src/agent/sessionContinuation.test.ts
npm run typecheck
```

---

## Phase 1: Prompt Builder Extraction With Legacy Parity

### Task 1: Add Context Type Foundations

**Files:**

- Create: `packages/core/src/context/types.ts`
- Create: `packages/core/src/context/index.ts`
- Modify: `packages/core/src/types.ts`

- [x] **Step 1: Create `packages/core/src/context/types.ts` with shared types**

Add:

```ts
import type { BrowserState } from '@page-agent/page-controller'

import type { HistoricalEvent } from '../types'

export interface ContextConfig {
	enabled?: boolean
	maxPromptTokens?: number
	recentStepCount?: number
	compactAfterSteps?: number
	compactAtBudgetRatio?: number
	preserveFailedSteps?: number
	persistence?: 'none' | 'adapter'
	debug?: boolean
}

export interface ContextSection {
	key: ContextSectionKey
	title: string
	content: string
	priority: number
	required?: boolean
}

export type ContextSectionKey =
	| 'instructions'
	| 'task'
	| 'sessionSummary'
	| 'protectedFacts'
	| 'recentSteps'
	| 'businessObjects'
	| 'currentPage'
	| 'safetyRules'
	| 'debug'

export interface ContextBudgetItem {
	key: ContextSectionKey
	estimatedTokens: number
	characters: number
	status: 'kept' | 'trimmed' | 'dropped'
}

export interface ContextBudgetReport {
	maxPromptTokens: number
	estimatedTokens: number
	items: ContextBudgetItem[]
	triggeredCompact: boolean
}

export interface BusinessObjectRef {
	type: string
	name?: string
	id?: string
	aliases?: string[]
	evidence: string[]
}

export interface BrowserSessionSummary {
	userGoal: string
	currentSubGoal: string
	completed: string[]
	failedAttempts: string[]
	stableFacts: string[]
	businessObjects: BusinessObjectRef[]
	userDecisions: string[]
	riskDecisions: string[]
	currentPageSemanticState: {
		url: string
		title: string
		semanticLocation: string
		visibleEvidence: string[]
	}
	doNotRepeat: string[]
	nextRecommendedActions: string[]
}

export interface ContextPack {
	instructions: ContextSection[]
	agentState: ContextSection
	sessionSummary?: ContextSection
	protectedFacts: ContextSection
	recentSteps: ContextSection
	currentPage: ContextSection
	safetyRules: ContextSection
	budgetReport: ContextBudgetReport
}

export interface BuildContextPackInput {
	task: string
	stepCount: number
	maxSteps: number
	currentTime: string
	instructions: string
	history: HistoricalEvent[]
	browserState: BrowserState
}

export const DEFAULT_CONTEXT_CONFIG: Required<ContextConfig> = {
	enabled: false,
	maxPromptTokens: 32_000,
	recentStepCount: 6,
	compactAfterSteps: 12,
	compactAtBudgetRatio: 0.7,
	preserveFailedSteps: 3,
	persistence: 'none',
	debug: false,
}
```

- [x] **Step 2: Create `packages/core/src/context/index.ts` exports**

Add:

```ts
export * from './types'
```

- [x] **Step 3: Extend `AgentConfig` in `packages/core/src/types.ts`**

Import and add optional config:

```ts
import type { ContextConfig, ContextStore } from './context'
```

Add to `AgentConfig`:

```ts
	/**
	 * Long-running context management.
	 * Disabled by default to preserve existing behavior.
	 */
	context?: ContextConfig

	/**
	 * Optional context persistence adapter.
	 * Core code must work without this adapter.
	 */
	contextStore?: ContextStore
```

- [x] **Step 4: Run typecheck**

Run:

```bash
npm run typecheck
```

Expected: typecheck fails only if `ContextStore` is not exported yet. If so, continue to Task 2 before re-running full typecheck.

### Task 2: Add ContextStore Interface

**Files:**

- Create: `packages/core/src/context/ContextStore.ts`
- Modify: `packages/core/src/context/index.ts`
- Test: `packages/core/src/context/ContextStore.test.ts`

- [x] **Step 1: Write failing store test**

Create `packages/core/src/context/ContextStore.test.ts`:

```ts
import { describe, expect, it } from 'vitest'

import { InMemoryContextStore } from './ContextStore'

describe('InMemoryContextStore', () => {
	it('saves, loads, and clears context by task id', async () => {
		const store = new InMemoryContextStore()
		await store.save('task-1', {
			taskId: 'task-1',
			updatedAt: 123,
			summary: undefined,
			recentEvents: [],
			budgetReport: undefined,
		})

		await expect(store.load('task-1')).resolves.toMatchObject({
			taskId: 'task-1',
			updatedAt: 123,
		})

		await store.clear('task-1')
		await expect(store.load('task-1')).resolves.toBeUndefined()
	})
})
```

- [x] **Step 2: Run failing test**

Run:

```bash
npm test -- packages/core/src/context/ContextStore.test.ts
```

Expected: FAIL because `ContextStore` does not exist.

- [x] **Step 3: Implement `ContextStore.ts`**

Add:

```ts
import type { HistoricalEvent } from '../types'

import type { BrowserSessionSummary, ContextBudgetReport } from './types'

export interface StoredContext {
	taskId: string
	updatedAt: number
	summary?: BrowserSessionSummary
	recentEvents: HistoricalEvent[]
	budgetReport?: ContextBudgetReport
}

export interface ContextStore {
	load(taskId: string): Promise<StoredContext | undefined>
	save(taskId: string, context: StoredContext): Promise<void>
	clear(taskId: string): Promise<void>
}

export class InMemoryContextStore implements ContextStore {
	private readonly contexts = new Map<string, StoredContext>()

	async load(taskId: string): Promise<StoredContext | undefined> {
		return this.contexts.get(taskId)
	}

	async save(taskId: string, context: StoredContext): Promise<void> {
		this.contexts.set(taskId, context)
	}

	async clear(taskId: string): Promise<void> {
		this.contexts.delete(taskId)
	}
}
```

- [x] **Step 4: Export store**

Modify `packages/core/src/context/index.ts`:

```ts
export * from './ContextStore'
export * from './types'
```

- [x] **Step 5: Run test and typecheck**

Run:

```bash
npm test -- packages/core/src/context/ContextStore.test.ts
npm run typecheck
```

Expected: PASS.

### Task 3: Extract PageAgentPromptBuilder With Legacy Output

**Files:**

- Create: `packages/core/src/context/PageAgentPromptBuilder.ts`
- Test: `packages/core/src/context/PageAgentPromptBuilder.test.ts`
- Modify in Task 10: `packages/core/src/PageAgentCore.ts`

- [x] **Step 1: Write failing prompt builder test**

Create `packages/core/src/context/PageAgentPromptBuilder.test.ts`:

```ts
import { describe, expect, it } from 'vitest'

import { PageAgentPromptBuilder } from './PageAgentPromptBuilder'

describe('PageAgentPromptBuilder', () => {
	it('renders a legacy-compatible prompt with agent history and browser state', () => {
		const prompt = new PageAgentPromptBuilder().buildLegacyPrompt({
			instructions: '<instructions>Use the app carefully.</instructions>\n\n',
			task: 'Find the latest invoice',
			stepCount: 1,
			maxSteps: 40,
			currentTime: '2026/5/26 20:00:00',
			history: [
				{
					type: 'step',
					stepIndex: 0,
					reflection: {
						evaluation_previous_goal: 'Clicked search. Verdict: Success',
						memory: 'Search panel is open.',
						next_goal: 'Type invoice id.',
					},
					action: {
						name: 'click_element_by_index',
						input: { index: 3 },
						output: '✅ Clicked element ([3]<button>Search</button>).',
					},
					usage: {
						promptTokens: 1,
						completionTokens: 1,
						totalTokens: 2,
					},
				},
				{ type: 'observation', content: 'Page navigated to → https://example.com' },
			],
			browserState: {
				url: 'https://example.com',
				title: 'Example',
				header: 'Current Page: [Example](https://example.com)',
				content: '[0]<input placeholder=Search />',
				footer: '[End of page]',
			},
			pageContent: '[0]<input placeholder=Search />',
		})

		expect(prompt).toContain('<instructions>Use the app carefully.</instructions>')
		expect(prompt).toContain('<agent_state>')
		expect(prompt).toContain('<agent_history>')
		expect(prompt).toContain('<step_1>')
		expect(prompt).toContain('Action Results: ✅ Clicked element')
		expect(prompt).toContain('<sys>Page navigated to → https://example.com</sys>')
		expect(prompt).toContain('<browser_state>')
		expect(prompt).toContain('[0]<input placeholder=Search />')
	})
})
```

- [x] **Step 2: Run failing test**

Run:

```bash
npm test -- packages/core/src/context/PageAgentPromptBuilder.test.ts
```

Expected: FAIL because builder does not exist.

- [x] **Step 3: Implement legacy builder**

Create `packages/core/src/context/PageAgentPromptBuilder.ts`:

```ts
import type { BrowserState } from '@page-agent/page-controller'

import type { HistoricalEvent } from '../types'

export interface BuildLegacyPromptInput {
	instructions: string
	task: string
	stepCount: number
	maxSteps: number
	currentTime: string
	history: HistoricalEvent[]
	browserState: BrowserState
	pageContent: string
}

export class PageAgentPromptBuilder {
	buildLegacyPrompt(input: BuildLegacyPromptInput): string {
		let prompt = ''
		prompt += input.instructions
		prompt += '<agent_state>\n'
		prompt += '<user_request>\n'
		prompt += `${input.task}\n`
		prompt += '</user_request>\n'
		prompt += '<step_info>\n'
		prompt += `Step ${input.stepCount + 1} of ${input.maxSteps} max possible steps\n`
		prompt += `Current time: ${input.currentTime}\n`
		prompt += '</step_info>\n'
		prompt += '</agent_state>\n\n'

		prompt += '<agent_history>\n'
		let renderedStepIndex = 0
		for (const event of input.history) {
			if (event.type === 'step') {
				renderedStepIndex++
				prompt += `<step_${renderedStepIndex}>\n`
				prompt += `Evaluation of Previous Step: ${event.reflection.evaluation_previous_goal}\n`
				prompt += `Memory: ${event.reflection.memory}\n`
				prompt += `Next Goal: ${event.reflection.next_goal}\n`
				prompt += `Action Results: ${event.action.output}\n`
				prompt += `</step_${renderedStepIndex}>\n`
			} else if (event.type === 'observation') {
				prompt += `<sys>${event.content}</sys>\n`
			} else if (event.type === 'user_takeover') {
				prompt += `<sys>User took over control and made changes to the page</sys>\n`
			}
		}
		prompt += '</agent_history>\n\n'

		prompt += '<browser_state>\n'
		prompt += input.browserState.header + '\n'
		prompt += input.pageContent + '\n'
		prompt += input.browserState.footer + '\n\n'
		prompt += '</browser_state>\n\n'

		return prompt
	}
}
```

- [x] **Step 4: Export builder and run tests**

Modify `packages/core/src/context/index.ts`:

```ts
export * from './ContextStore'
export * from './PageAgentPromptBuilder'
export * from './types'
```

Run:

```bash
npm test -- packages/core/src/context/PageAgentPromptBuilder.test.ts
npm run typecheck
```

Expected: PASS.

### Task 4: Wire PromptBuilder Into PageAgentCore Without Behavior Change

**Files:**

- Modify: `packages/core/src/PageAgentCore.ts`
- Test: existing core tests plus typecheck

- [x] **Step 1: Import builder**

Add near existing imports:

```ts
import { PageAgentPromptBuilder } from './context'
```

- [x] **Step 2: Add private builder field**

Inside `PageAgentCore`:

```ts
	#promptBuilder = new PageAgentPromptBuilder()
```

- [x] **Step 3: Replace body of `#assembleUserPrompt()` with legacy builder call**

Keep `#getInstructions()` and `transformPageContent` behavior. The body should become:

```ts
	async #assembleUserPrompt(): Promise<string> {
		const browserState = this.#states.browserState!
		let pageContent = browserState.content
		if (this.config.transformPageContent) {
			pageContent = await this.config.transformPageContent(pageContent)
		}

		const stepCount = this.history.filter((e) => e.type === 'step').length

		return this.#promptBuilder.buildLegacyPrompt({
			instructions: await this.#getInstructions(),
			task: this.task,
			stepCount,
			maxSteps: this.config.maxSteps,
			currentTime: new Date().toLocaleString(),
			history: this.history,
			browserState,
			pageContent,
		})
	}
```

- [x] **Step 4: Run regression tests**

Run:

```bash
npm test -- packages/core/src/context/PageAgentPromptBuilder.test.ts packages/core/src/searchExplorationGuard.test.ts
npm run typecheck
```

Expected: PASS. No context-management behavior has changed yet.

---

## Phase 2: Budget Reporting and Context Pack

### Task 5: Add Token Budget Estimator

**Files:**

- Create: `packages/core/src/context/tokenBudget.ts`
- Test: `packages/core/src/context/tokenBudget.test.ts`

- [x] **Step 1: Write failing budget tests**

Create `packages/core/src/context/tokenBudget.test.ts`:

```ts
import { describe, expect, it } from 'vitest'

import { estimateTokens, makeBudgetReport } from './tokenBudget'

describe('tokenBudget', () => {
	it('estimates tokens conservatively from characters', () => {
		expect(estimateTokens('abcdef')).toBe(2)
		expect(estimateTokens('')).toBe(0)
	})

	it('reports section sizes and total estimated tokens', () => {
		const report = makeBudgetReport(
			[
				{ key: 'task', title: 'Task', content: 'abc', priority: 100, required: true },
				{ key: 'currentPage', title: 'Page', content: 'abcdef', priority: 90, required: true },
			],
			32_000,
			false
		)

		expect(report.estimatedTokens).toBe(3)
		expect(report.items).toHaveLength(2)
		expect(report.items[0]).toMatchObject({ key: 'task', status: 'kept' })
	})
})
```

- [x] **Step 2: Run failing test**

Run:

```bash
npm test -- packages/core/src/context/tokenBudget.test.ts
```

Expected: FAIL because module does not exist.

- [x] **Step 3: Implement estimator**

Add `packages/core/src/context/tokenBudget.ts`:

```ts
import type { ContextBudgetReport, ContextSection } from './types'

export function estimateTokens(text: string): number {
	if (!text) return 0
	return Math.ceil(text.length / 3)
}

export function makeBudgetReport(
	sections: ContextSection[],
	maxPromptTokens: number,
	triggeredCompact: boolean
): ContextBudgetReport {
	const items = sections.map((section) => ({
		key: section.key,
		characters: section.content.length,
		estimatedTokens: estimateTokens(section.content),
		status: 'kept' as const,
	}))

	return {
		maxPromptTokens,
		estimatedTokens: items.reduce((sum, item) => sum + item.estimatedTokens, 0),
		items,
		triggeredCompact,
	}
}
```

- [x] **Step 4: Export and verify**

Modify `packages/core/src/context/index.ts`:

```ts
export * from './ContextStore'
export * from './PageAgentPromptBuilder'
export * from './tokenBudget'
export * from './types'
```

Run:

```bash
npm test -- packages/core/src/context/tokenBudget.test.ts
npm run typecheck
```

Expected: PASS.

### Task 6: Add Context Pack Rendering

**Files:**

- Modify: `packages/core/src/context/PageAgentPromptBuilder.ts`
- Test: `packages/core/src/context/PageAgentPromptBuilder.test.ts`

- [x] **Step 1: Add failing test for sectioned prompt**

Append:

```ts
	it('renders sectioned context with stale-index safety rules', () => {
		const prompt = new PageAgentPromptBuilder().buildContextPrompt({
			instructions: [],
			agentState: {
				key: 'task',
				title: 'Agent State',
				content: '<user_request>Find invoice</user_request>',
				priority: 100,
				required: true,
			},
			protectedFacts: {
				key: 'protectedFacts',
				title: 'Protected Facts',
				content: '- User confirmed read-only mode.',
				priority: 95,
				required: true,
			},
			recentSteps: {
				key: 'recentSteps',
				title: 'Recent Steps',
				content: '<step_1>...</step_1>',
				priority: 80,
			},
			currentPage: {
				key: 'currentPage',
				title: 'Current Page',
				content: '[0]<button>Search</button>',
				priority: 100,
				required: true,
			},
			safetyRules: {
				key: 'safetyRules',
				title: 'Context Rules',
				content: 'Only use indexes in current browser_state.',
				priority: 100,
				required: true,
			},
			budgetReport: {
				maxPromptTokens: 32000,
				estimatedTokens: 100,
				items: [],
				triggeredCompact: false,
			},
		})

		expect(prompt).toContain('<context_rules>')
		expect(prompt).toContain('Only use indexes in current browser_state.')
		expect(prompt).toContain('<protected_facts>')
		expect(prompt).toContain('<recent_steps>')
		expect(prompt).toContain('<browser_state>')
	})
```

- [x] **Step 2: Run failing test**

Run:

```bash
npm test -- packages/core/src/context/PageAgentPromptBuilder.test.ts
```

Expected: FAIL because `buildContextPrompt` is missing.

- [x] **Step 3: Implement `buildContextPrompt`**

Add imports and method:

```ts
import type { ContextPack } from './types'
```

```ts
	buildContextPrompt(pack: ContextPack): string {
		let prompt = ''
		for (const section of pack.instructions) {
			if (section.content.trim()) prompt += section.content + '\n\n'
		}

		prompt += '<agent_state>\n'
		prompt += pack.agentState.content + '\n'
		prompt += '<context_rules>\n'
		prompt += pack.safetyRules.content + '\n'
		prompt += '</context_rules>\n'
		prompt += '</agent_state>\n\n'

		if (pack.sessionSummary?.content.trim()) {
			prompt += '<session_summary>\n'
			prompt += pack.sessionSummary.content + '\n'
			prompt += '</session_summary>\n\n'
		}

		if (pack.protectedFacts.content.trim()) {
			prompt += '<protected_facts>\n'
			prompt += pack.protectedFacts.content + '\n'
			prompt += '</protected_facts>\n\n'
		}

		prompt += '<recent_steps>\n'
		prompt += pack.recentSteps.content + '\n'
		prompt += '</recent_steps>\n\n'

		prompt += '<browser_state>\n'
		prompt += pack.currentPage.content + '\n'
		prompt += '</browser_state>\n\n'

		if (pack.budgetReport.items.length > 0) {
			prompt += '<context_budget>\n'
			prompt += `Estimated tokens: ${pack.budgetReport.estimatedTokens}/${pack.budgetReport.maxPromptTokens}\n`
			prompt += '</context_budget>\n\n'
		}

		return prompt
	}
```

- [x] **Step 4: Verify**

Run:

```bash
npm test -- packages/core/src/context/PageAgentPromptBuilder.test.ts
npm run typecheck
```

Expected: PASS.

### Task 7: Add ContextRuntime in Disabled-Compatible Mode

**Files:**

- Create: `packages/core/src/context/ContextRuntime.ts`
- Test: `packages/core/src/context/ContextRuntime.test.ts`

- [x] **Step 1: Write failing runtime test**

Create `packages/core/src/context/ContextRuntime.test.ts`:

```ts
import { describe, expect, it } from 'vitest'

import { ContextRuntime } from './ContextRuntime'

describe('ContextRuntime', () => {
	it('builds a context pack with current page and budget report', () => {
		const runtime = new ContextRuntime({
			config: { enabled: true },
			taskId: 'task-1',
		})

		const pack = runtime.buildPack({
			task: 'Find invoice',
			stepCount: 0,
			maxSteps: 40,
			currentTime: '2026/5/26 20:00:00',
			instructions: '',
			history: [],
			browserState: {
				url: 'https://example.com',
				title: 'Example',
				header: 'Current Page: [Example](https://example.com)',
				content: '[0]<button>Search</button>',
				footer: '[End of page]',
			},
		})

		expect(pack.agentState.content).toContain('Find invoice')
		expect(pack.currentPage.content).toContain('[0]<button>Search</button>')
		expect(pack.safetyRules.content).toContain('current browser_state')
		expect(pack.budgetReport.estimatedTokens).toBeGreaterThan(0)
	})
})
```

- [x] **Step 2: Run failing test**

Run:

```bash
npm test -- packages/core/src/context/ContextRuntime.test.ts
```

Expected: FAIL because runtime does not exist.

- [x] **Step 3: Implement minimal runtime**

Create `packages/core/src/context/ContextRuntime.ts`:

```ts
import type { AgentStepEvent } from '../types'

import { makeBudgetReport } from './tokenBudget'
import {
	DEFAULT_CONTEXT_CONFIG,
	type BuildContextPackInput,
	type ContextConfig,
	type ContextPack,
	type ContextSection,
} from './types'

export interface ContextRuntimeOptions {
	config?: ContextConfig
	taskId: string
}

export class ContextRuntime {
	readonly config: Required<ContextConfig>
	private readonly taskId: string
	private readonly recordedSteps: AgentStepEvent[] = []

	constructor(options: ContextRuntimeOptions) {
		this.config = { ...DEFAULT_CONTEXT_CONFIG, ...(options.config ?? {}) }
		this.taskId = options.taskId
	}

	buildPack(input: BuildContextPackInput): ContextPack {
		const agentState: ContextSection = {
			key: 'task',
			title: 'Agent State',
			required: true,
			priority: 100,
			content: [
				'<user_request>',
				input.task,
				'</user_request>',
				'<step_info>',
				`Step ${input.stepCount + 1} of ${input.maxSteps} max possible steps`,
				`Current time: ${input.currentTime}`,
				'</step_info>',
			].join('\n'),
		}

		const recentSteps: ContextSection = {
			key: 'recentSteps',
			title: 'Recent Steps',
			priority: 80,
			content: this.renderRecentSteps(input.history),
		}

		const currentPage: ContextSection = {
			key: 'currentPage',
			title: 'Current Page',
			required: true,
			priority: 100,
			content: [input.browserState.header, input.browserState.content, input.browserState.footer].join(
				'\n'
			),
		}

		const protectedFacts: ContextSection = {
			key: 'protectedFacts',
			title: 'Protected Facts',
			required: true,
			priority: 95,
			content: '',
		}

		const safetyRules: ContextSection = {
			key: 'safetyRules',
			title: 'Context Rules',
			required: true,
			priority: 100,
			content: [
				'Current browser_state is the source of truth for page operations.',
				'Element indexes from history, summaries, or memory are stale and must not be reused.',
				'If summary conflicts with current browser_state, trust current browser_state.',
			].join('\n'),
		}

		const sections = [agentState, recentSteps, currentPage, protectedFacts, safetyRules]
		const budgetReport = makeBudgetReport(sections, this.config.maxPromptTokens, false)

		return {
			instructions: input.instructions
				? [
						{
							key: 'instructions',
							title: 'Instructions',
							content: input.instructions,
							priority: 90,
						},
					]
				: [],
			agentState,
			protectedFacts,
			recentSteps,
			currentPage,
			safetyRules,
			budgetReport,
		}
	}

	recordStep(step: AgentStepEvent): void {
		this.recordedSteps.push(step)
	}

	private renderRecentSteps(history: BuildContextPackInput['history']): string {
		const stepEvents = history.filter((event): event is AgentStepEvent => event.type === 'step')
		const recent = stepEvents.slice(-this.config.recentStepCount)
		return recent
			.map((event, index) =>
				[
					`<step_${index + 1}>`,
					`Evaluation of Previous Step: ${event.reflection.evaluation_previous_goal ?? ''}`,
					`Memory: ${event.reflection.memory ?? ''}`,
					`Next Goal: ${event.reflection.next_goal ?? ''}`,
					`Action Results: ${event.action.output}`,
					`</step_${index + 1}>`,
				].join('\n')
			)
			.join('\n')
	}
}
```

- [x] **Step 4: Export runtime and verify**

Modify `packages/core/src/context/index.ts`:

```ts
export * from './ContextRuntime'
export * from './ContextStore'
export * from './PageAgentPromptBuilder'
export * from './tokenBudget'
export * from './types'
```

Run:

```bash
npm test -- packages/core/src/context/ContextRuntime.test.ts
npm run typecheck
```

Expected: PASS.

---

## Phase 3: Recent Steps, Protected Facts, and Deterministic Compact

### Task 8: Add BrowserCompactManager

**Files:**

- Create: `packages/core/src/context/BrowserCompactManager.ts`
- Test: `packages/core/src/context/BrowserCompactManager.test.ts`

- [x] **Step 1: Write failing compact tests**

Create `packages/core/src/context/BrowserCompactManager.test.ts`:

```ts
import { describe, expect, it } from 'vitest'

import { BrowserCompactManager } from './BrowserCompactManager'

describe('BrowserCompactManager', () => {
	it('summarizes older history without preserving executable DOM indexes', () => {
		const summary = new BrowserCompactManager().compact({
			task: 'Check order 123',
			history: [
				{
					type: 'step',
					stepIndex: 0,
					reflection: {
						evaluation_previous_goal: 'Opened search. Verdict: Success',
						memory: 'Search page is open.',
						next_goal: 'Search order.',
					},
					action: {
						name: 'click_element_by_index',
						input: { index: 12 },
						output: '✅ Clicked element ([12]<button>Search</button>).',
					},
					usage: { promptTokens: 1, completionTokens: 1, totalTokens: 2 },
				},
				{ type: 'observation', content: 'Page navigated to → https://example.com/orders' },
			],
			browserState: {
				url: 'https://example.com/orders',
				title: 'Orders',
				header: 'Current Page: [Orders](https://example.com/orders)',
				content: '[0]<input placeholder=Order ID />',
				footer: '[End of page]',
			},
		})

		expect(summary.userGoal).toBe('Check order 123')
		expect(summary.currentPageSemanticState.url).toBe('https://example.com/orders')
		expect(JSON.stringify(summary)).not.toContain('[12]')
		expect(summary.completed[0]).toContain('click_element_by_index')
	})
})
```

- [x] **Step 2: Run failing test**

Run:

```bash
npm test -- packages/core/src/context/BrowserCompactManager.test.ts
```

Expected: FAIL because manager does not exist.

- [x] **Step 3: Implement deterministic compact**

Create `packages/core/src/context/BrowserCompactManager.ts`:

```ts
import type { BrowserState } from '@page-agent/page-controller'

import type { AgentStepEvent, HistoricalEvent } from '../types'

import type { BrowserSessionSummary } from './types'

export interface CompactInput {
	task: string
	history: HistoricalEvent[]
	browserState: BrowserState
}

export class BrowserCompactManager {
	compact(input: CompactInput): BrowserSessionSummary {
		const stepEvents = input.history.filter((event): event is AgentStepEvent => event.type === 'step')
		const failedAttempts = stepEvents
			.filter((event) => event.action.output.startsWith('❌') || event.action.output.includes('failed'))
			.map((event) => this.sanitizeActionLine(event))
		const completed = stepEvents
			.filter((event) => !event.action.output.startsWith('❌'))
			.map((event) => this.sanitizeActionLine(event))
		const navigationTrail = input.history
			.filter((event) => event.type === 'observation' && event.content.includes('Page navigated to'))
			.map((event) => (event.type === 'observation' ? event.content.replace(/\[[0-9]+]/g, '[stale-index]') : ''))
			.filter(Boolean)

		return {
			userGoal: input.task,
			currentSubGoal: stepEvents.at(-1)?.reflection.next_goal ?? '',
			completed,
			failedAttempts,
			stableFacts: navigationTrail,
			businessObjects: [],
			userDecisions: [],
			riskDecisions: [],
			currentPageSemanticState: {
				url: input.browserState.url,
				title: input.browserState.title,
				semanticLocation: input.browserState.header.split('\n')[0] ?? '',
				visibleEvidence: input.browserState.content
					.split('\n')
					.slice(0, 5)
					.map((line) => line.replace(/\[[0-9]+]/g, '[current-index-redacted]')),
			},
			doNotRepeat: failedAttempts,
			nextRecommendedActions: stepEvents.at(-1)?.reflection.next_goal
				? [stepEvents.at(-1)!.reflection.next_goal!]
				: [],
		}
	}

	private sanitizeActionLine(event: AgentStepEvent): string {
		return `${event.action.name}: ${event.action.output}`.replace(/\[[0-9]+]/g, '[stale-index]')
	}
}
```

- [x] **Step 4: Export and verify**

Modify `packages/core/src/context/index.ts`:

```ts
export * from './BrowserCompactManager'
export * from './ContextRuntime'
export * from './ContextStore'
export * from './PageAgentPromptBuilder'
export * from './tokenBudget'
export * from './types'
```

Run:

```bash
npm test -- packages/core/src/context/BrowserCompactManager.test.ts
npm run typecheck
```

Expected: PASS.

### Task 9: Teach ContextRuntime To Build Summary and Keep Recent Steps Bounded

**Files:**

- Modify: `packages/core/src/context/ContextRuntime.ts`
- Test: `packages/core/src/context/ContextRuntime.test.ts`

- [x] **Step 1: Add failing bounded-history test**

Append to `ContextRuntime.test.ts`:

```ts
	it('keeps only recent steps in recentSteps section after compact threshold', () => {
		const runtime = new ContextRuntime({
			config: { enabled: true, recentStepCount: 2, compactAfterSteps: 3 },
			taskId: 'task-1',
		})

		const history = Array.from({ length: 5 }, (_, stepIndex) => ({
			type: 'step' as const,
			stepIndex,
			reflection: {
				evaluation_previous_goal: `step ${stepIndex} done`,
				memory: `memory ${stepIndex}`,
				next_goal: `next ${stepIndex}`,
			},
			action: {
				name: 'wait',
				input: { seconds: 1 },
				output: `✅ Waited ${stepIndex}`,
			},
			usage: { promptTokens: 1, completionTokens: 1, totalTokens: 2 },
		}))

		const pack = runtime.buildPack({
			task: 'Long task',
			stepCount: 5,
			maxSteps: 40,
			currentTime: '2026/5/26 20:00:00',
			instructions: '',
			history,
			browserState: {
				url: 'https://example.com',
				title: 'Example',
				header: 'Current Page: [Example](https://example.com)',
				content: '[0]<button>Search</button>',
				footer: '[End of page]',
			},
		})

		expect(pack.recentSteps.content).toContain('Waited 3')
		expect(pack.recentSteps.content).toContain('Waited 4')
		expect(pack.recentSteps.content).not.toContain('Waited 0')
		expect(pack.sessionSummary?.content).toContain('Long task')
	})
```

- [x] **Step 2: Run failing runtime test**

Run:

```bash
npm test -- packages/core/src/context/ContextRuntime.test.ts
```

Expected: FAIL until summary is included.

- [x] **Step 3: Update runtime to use `BrowserCompactManager`**

Add import:

```ts
import { BrowserCompactManager } from './BrowserCompactManager'
```

Add field:

```ts
	private readonly compactManager = new BrowserCompactManager()
```

Inside `buildPack`, compute `sessionSummary` when `stepCount >= this.config.compactAfterSteps`:

```ts
		const shouldCompact = input.stepCount >= this.config.compactAfterSteps
		const summary = shouldCompact
			? this.compactManager.compact({
					task: input.task,
					history: input.history.slice(0, Math.max(0, input.history.length - this.config.recentStepCount)),
					browserState: input.browserState,
				})
			: undefined

		const sessionSummary = summary
			? {
					key: 'sessionSummary' as const,
					title: 'Session Summary',
					priority: 70,
					content: JSON.stringify(summary, null, 2),
				}
			: undefined
```

Pass `sessionSummary` into returned pack and include it in `sections` before budget reporting.

- [x] **Step 4: Verify bounded context**

Run:

```bash
npm test -- packages/core/src/context/ContextRuntime.test.ts packages/core/src/context/BrowserCompactManager.test.ts
npm run typecheck
```

Expected: PASS.

---

## Phase 4: Optional ContextRuntime Integration in PageAgentCore

### Task 10: Enable ContextRuntime Behind Feature Flag

**Files:**

- Modify: `packages/core/src/PageAgentCore.ts`
- Test: typecheck plus context tests

- [x] **Step 1: Import runtime**

Add:

```ts
import { ContextRuntime, PageAgentPromptBuilder } from './context'
```

If `PageAgentPromptBuilder` import already exists, merge imports.

- [x] **Step 2: Add private runtime field**

Inside `PageAgentCore`:

```ts
	#contextRuntime: ContextRuntime | null = null
```

- [x] **Step 3: Initialize runtime at task start**

After `this.taskId = uid()`:

```ts
		this.#contextRuntime = this.config.context?.enabled
			? new ContextRuntime({
					config: this.config.context,
					taskId: this.taskId,
				})
			: null
```

- [x] **Step 4: Build sectioned prompt only when enabled**

In `#assembleUserPrompt()`, after `pageContent` transformation and `stepCount`, branch:

```ts
		if (this.#contextRuntime) {
			const pack = this.#contextRuntime.buildPack({
				task: this.task,
				stepCount,
				maxSteps: this.config.maxSteps,
				currentTime: new Date().toLocaleString(),
				instructions: await this.#getInstructions(),
				history: this.history,
				browserState: {
					...browserState,
					content: pageContent,
				},
			})

			return this.#promptBuilder.buildContextPrompt(pack)
		}
```

Keep legacy builder return as fallback.

- [x] **Step 5: Record steps into runtime after history push**

After `this.history.push({ ... } as AgentStepEvent)`, store the event in a variable and record it:

```ts
				const stepEvent = {
					type: 'step',
					stepIndex: step,
					reflection,
					action,
					usage: result.usage,
					rawResponse: result.rawResponse,
					rawRequest: result.rawRequest,
				} as AgentStepEvent
				this.history.push(stepEvent)
				this.#contextRuntime?.recordStep(stepEvent)
```

- [x] **Step 6: Run core verification**

Run:

```bash
npm test -- packages/core/src/context packages/core/src/searchExplorationGuard.test.ts
npm run typecheck
```

Expected: PASS. Default config still uses legacy prompt.

### Task 11: Add System Prompt Context Rules

**Files:**

- Modify: `packages/core/src/prompts/system_prompt.md`

- [x] **Step 1: Add short context rules to browser rules**

Add under `<browser_rules>`:

```md
- Current <browser_state> is the source of truth for all page operations.
- Element indexes from <agent_history>, summaries, memory, or previous steps are stale. Only use indexes visible in the current <browser_state>.
- If a summary or memory conflicts with the current page, trust the current page and explain the mismatch in `evaluation_previous_goal`.
```

- [x] **Step 2: Run typecheck**

Run:

```bash
npm run typecheck
```

Expected: PASS.

---

## Phase 5: Continuation Decision

### Task 12: Implement ContinuationResolver Core

**Files:**

- Create: `packages/core/src/context/ContinuationResolver.ts`
- Test: `packages/core/src/context/ContinuationResolver.test.ts`
- Modify: `packages/core/src/context/index.ts`

- [x] **Step 1: Write failing continuation tests**

Create `packages/core/src/context/ContinuationResolver.test.ts`:

```ts
import { describe, expect, it } from 'vitest'

import { ContinuationResolver } from './ContinuationResolver'

describe('ContinuationResolver', () => {
	const resolver = new ContinuationResolver()

	it('continues original task when user answers pending question', () => {
		const decision = resolver.resolve({
			previousTask: 'Find failing PG instance',
			userMessage: 'yes, continue',
			pendingQuestion: 'Should I continue checking this instance?',
			currentUrl: 'https://example.com/pg/pg-1',
			currentTitle: 'PG pg-1',
			previousBusinessObjects: [{ type: 'pg', id: 'pg-1', evidence: ['PG pg-1'] }],
		})

		expect(decision.mode).toBe('continue_original_task')
		expect(decision.inheritedSections).toContain('recentSteps')
	})

	it('starts a new task on the same page when intent changes', () => {
		const decision = resolver.resolve({
			previousTask: 'Find failing PG instance',
			userMessage: 'summarize what buttons are on this page',
			currentUrl: 'https://example.com/pg/pg-1',
			currentTitle: 'PG pg-1',
			previousUrl: 'https://example.com/pg/pg-1',
			previousTitle: 'PG pg-1',
			previousBusinessObjects: [{ type: 'pg', id: 'pg-1', evidence: ['PG pg-1'] }],
		})

		expect(decision.mode).toBe('new_task_same_page')
		expect(decision.inheritedSections).toEqual(['currentPage', 'instructions'])
	})

	it('inherits business facts when page changes but object is the same', () => {
		const decision = resolver.resolve({
			previousTask: 'Diagnose PG pg-1 error',
			userMessage: 'continue checking pg-1 logs here',
			currentUrl: 'https://example.com/logs',
			currentTitle: 'Logs',
			previousUrl: 'https://example.com/pg/pg-1',
			previousTitle: 'PG pg-1',
			previousBusinessObjects: [{ type: 'pg', id: 'pg-1', aliases: ['pg-1'], evidence: ['PG pg-1'] }],
		})

		expect(decision.mode).toBe('same_business_new_page')
		expect(decision.inheritedSections).toContain('businessObjects')
		expect(decision.discardedSections).toContain('recentSteps')
	})

	it('starts fresh when page and problem both change', () => {
		const decision = resolver.resolve({
			previousTask: 'Diagnose PG pg-1 error',
			userMessage: 'find invoice abc',
			currentUrl: 'https://example.com/billing',
			currentTitle: 'Billing',
			previousUrl: 'https://example.com/pg/pg-1',
			previousTitle: 'PG pg-1',
			previousBusinessObjects: [{ type: 'pg', id: 'pg-1', evidence: ['PG pg-1'] }],
		})

		expect(decision.mode).toBe('fresh_task')
		expect(decision.inheritedSections).toEqual(['currentPage', 'instructions'])
	})
})
```

- [x] **Step 2: Run failing tests**

Run:

```bash
npm test -- packages/core/src/context/ContinuationResolver.test.ts
```

Expected: FAIL because resolver does not exist.

- [x] **Step 3: Implement deterministic resolver**

Create `packages/core/src/context/ContinuationResolver.ts`:

```ts
import type { BusinessObjectRef, ContextSectionKey } from './types'

export type ContinuationMode =
	| 'continue_original_task'
	| 'new_task_same_page'
	| 'same_business_new_page'
	| 'fresh_task'
	| 'ask_user_to_confirm'

export interface ContinuationDecision {
	mode: ContinuationMode
	reason: string
	inheritedSections: ContextSectionKey[]
	discardedSections: ContextSectionKey[]
	requiresFreshObserve: boolean
	requiresUserConfirmation: boolean
}

export interface ResolveContinuationInput {
	previousTask: string
	userMessage: string
	pendingQuestion?: string
	currentUrl: string
	currentTitle: string
	previousUrl?: string
	previousTitle?: string
	previousBusinessObjects?: BusinessObjectRef[]
	hasUnconfirmedRisk?: boolean
}

export class ContinuationResolver {
	resolve(input: ResolveContinuationInput): ContinuationDecision {
		if (input.hasUnconfirmedRisk) {
			return this.confirm('Unconfirmed high-risk context must not be resumed automatically.')
		}

		if (input.pendingQuestion && this.looksLikeAnswer(input.userMessage)) {
			return {
				mode: 'continue_original_task',
				reason: 'User message answers the pending agent question.',
				inheritedSections: ['task', 'sessionSummary', 'recentSteps', 'protectedFacts', 'businessObjects', 'currentPage', 'instructions'],
				discardedSections: [],
				requiresFreshObserve: true,
				requiresUserConfirmation: false,
			}
		}

		const samePage =
			!!input.previousUrl &&
			input.previousUrl === input.currentUrl &&
			(!input.previousTitle || input.previousTitle === input.currentTitle)
		const sameBusinessObject = this.referencesBusinessObject(
			input.userMessage,
			input.previousBusinessObjects ?? []
		)

		if (samePage && !this.looksLikeContinuation(input.previousTask, input.userMessage)) {
			return {
				mode: 'new_task_same_page',
				reason: 'Same page but user intent changed.',
				inheritedSections: ['currentPage', 'instructions'],
				discardedSections: ['task', 'sessionSummary', 'recentSteps', 'businessObjects', 'protectedFacts'],
				requiresFreshObserve: true,
				requiresUserConfirmation: false,
			}
		}

		if (!samePage && sameBusinessObject) {
			return {
				mode: 'same_business_new_page',
				reason: 'Page changed but user referenced the same business object.',
				inheritedSections: ['businessObjects', 'protectedFacts', 'currentPage', 'instructions'],
				discardedSections: ['recentSteps'],
				requiresFreshObserve: true,
				requiresUserConfirmation: false,
			}
		}

		if (samePage && this.looksLikeContinuation(input.previousTask, input.userMessage)) {
			return {
				mode: 'continue_original_task',
				reason: 'Same page and message appears to continue the original task.',
				inheritedSections: ['task', 'sessionSummary', 'recentSteps', 'protectedFacts', 'businessObjects', 'currentPage', 'instructions'],
				discardedSections: [],
				requiresFreshObserve: true,
				requiresUserConfirmation: false,
			}
		}

		return {
			mode: 'fresh_task',
			reason: 'Page and task signals do not match previous context.',
			inheritedSections: ['currentPage', 'instructions'],
			discardedSections: ['task', 'sessionSummary', 'recentSteps', 'businessObjects', 'protectedFacts'],
			requiresFreshObserve: true,
			requiresUserConfirmation: false,
		}
	}

	private confirm(reason: string): ContinuationDecision {
		return {
			mode: 'ask_user_to_confirm',
			reason,
			inheritedSections: [],
			discardedSections: [],
			requiresFreshObserve: true,
			requiresUserConfirmation: true,
		}
	}

	private looksLikeAnswer(message: string): boolean {
		return /^(yes|no|ok|继续|可以|确认|不用|不要|是|否)\b/i.test(message.trim())
	}

	private looksLikeContinuation(previousTask: string, message: string): boolean {
		const continuationWords = ['continue', '继续', '接着', '再看', '多看', 'same', '这个', '它']
		const lower = `${previousTask}\n${message}`.toLowerCase()
		return continuationWords.some((word) => lower.includes(word.toLowerCase()))
	}

	private referencesBusinessObject(message: string, objects: BusinessObjectRef[]): boolean {
		const normalized = message.toLowerCase()
		return objects.some((object) => {
			const candidates = [object.id, object.name, ...(object.aliases ?? [])].filter(Boolean)
			return candidates.some((candidate) => normalized.includes(String(candidate).toLowerCase()))
		})
	}
}
```

- [x] **Step 4: Export and verify**

Modify `packages/core/src/context/index.ts`:

```ts
export * from './BrowserCompactManager'
export * from './ContinuationResolver'
export * from './ContextRuntime'
export * from './ContextStore'
export * from './PageAgentPromptBuilder'
export * from './tokenBudget'
export * from './types'
```

Run:

```bash
npm test -- packages/core/src/context/ContinuationResolver.test.ts
npm run typecheck
```

Expected: PASS.

### Task 13: Connect ContinuationResolver to Extension Follow-Up Flow

**Files:**

- Modify: `packages/extension/src/agent/sessionContinuation.ts`
- Modify: `packages/extension/src/agent/sessionContinuation.test.ts`
- Modify: `packages/extension/src/agent/useAgent.ts`

- [x] **Step 1: Add extension-facing continuation mode helper tests**

Append to `sessionContinuation.test.ts`:

```ts
import { ContinuationResolver } from '@page-agent/core'

it('can classify same-page new questions without forcing old history continuation', () => {
	const resolver = new ContinuationResolver()
	const decision = resolver.resolve({
		previousTask: '检查 PG pg-1 的故障',
		userMessage: '总结这个页面有哪些按钮',
		currentUrl: 'https://example.com/pg/pg-1',
		currentTitle: 'PG pg-1',
		previousUrl: 'https://example.com/pg/pg-1',
		previousTitle: 'PG pg-1',
		previousBusinessObjects: [{ type: 'pg', id: 'pg-1', evidence: ['PG pg-1'] }],
	})

	expect(decision.mode).toBe('new_task_same_page')
})
```

- [x] **Step 2: Run targeted test**

Run:

```bash
npm test -- packages/extension/src/agent/sessionContinuation.test.ts
```

Expected: PASS after Task 12 exports resolver.

- [x] **Step 3: Add `useAgent.ts` continuation decision hook**

Add a small helper around the `execute()` call site that computes `ContinuationDecision` before passing `carryHistory`. Keep behavior unchanged unless a caller supplies previous URL/title/business objects.

- [x] **Step 4: Verify extension typecheck**

Run:

```bash
npm run typecheck
```

Expected: PASS.

---

## Phase 6: ContextStore Resume Hook Points

### Task 14: Save Lightweight Context Checkpoints

**Files:**

- Modify: `packages/core/src/context/ContextRuntime.ts`
- Modify: `packages/core/src/context/ContextRuntime.test.ts`
- Modify: `packages/core/src/PageAgentCore.ts`

- [x] **Step 1: Add runtime snapshot test**

Append to `ContextRuntime.test.ts`:

```ts
	it('exports a lightweight stored context snapshot', () => {
		const runtime = new ContextRuntime({ config: { enabled: true }, taskId: 'task-1' })
		const snapshot = runtime.toStoredContext()

		expect(snapshot.taskId).toBe('task-1')
		expect(snapshot.recentEvents).toEqual([])
		expect(snapshot.updatedAt).toEqual(expect.any(Number))
	})
```

- [x] **Step 2: Implement `toStoredContext()`**

In `ContextRuntime.ts`, import `StoredContext`:

```ts
import type { StoredContext } from './ContextStore'
```

Add:

```ts
	toStoredContext(): StoredContext {
		return {
			taskId: this.taskId,
			updatedAt: Date.now(),
			summary: undefined,
			recentEvents: this.recordedSteps.slice(-this.config.recentStepCount),
			budgetReport: undefined,
		}
	}
```

If `taskId` is private readonly, this method can access it directly.

- [x] **Step 3: Save snapshot in PageAgentCore after each step when store is provided**

After `this.#contextRuntime?.recordStep(stepEvent)`:

```ts
				if (this.#contextRuntime && this.config.contextStore) {
					await this.config.contextStore.save(
						this.taskId,
						this.#contextRuntime.toStoredContext()
					)
				}
```

- [x] **Step 4: Verify**

Run:

```bash
npm test -- packages/core/src/context/ContextRuntime.test.ts
npm run typecheck
```

Expected: PASS.

---

## Phase 7: Final Verification and Documentation

### Task 15: Run Full Regression Suite

**Files:**

- No source changes.

- [x] **Step 1: Run context tests**

Run:

```bash
npm test -- packages/core/src/context
```

Expected: PASS.

- [x] **Step 2: Run related regression tests**

Run:

```bash
npm test -- packages/core/src/searchExplorationGuard.test.ts packages/extension/src/agent/sessionContinuation.test.ts
```

Expected: PASS.

- [x] **Step 3: Run repository typecheck**

Run:

```bash
npm run typecheck
```

Expected: PASS.

- [x] **Step 4: Run lint if implementation touched exported APIs or prompt files**

Run:

```bash
npm run lint
```

Expected: PASS or pre-existing lint findings clearly documented.

### Task 16: Update Design Docs With Implementation Notes

**Files:**

- Modify: `docs/page-agent-long-running-context-management-design.zh.md`

- [x] **Step 1: Add an implementation status section**

Append a short section:

```md
## 实施状态

- PromptBuilder: implemented in `packages/core/src/context/PageAgentPromptBuilder.ts`.
- ContextPack and budget report: implemented in `packages/core/src/context/ContextRuntime.ts` and `tokenBudget.ts`.
- Deterministic compact: implemented in `packages/core/src/context/BrowserCompactManager.ts`.
- ContinuationResolver: implemented in `packages/core/src/context/ContinuationResolver.ts`.
- Runtime persistence: optional through `ContextStore`; Core works without browser storage or filesystem access.
```

- [x] **Step 2: Verify no stale claims**

Run:

```bash
rg -n "未实[现]" docs/page-agent-long-running-context-management-design.zh.md docs/superpowers/plans/2026-05-26-page-agent-long-running-context-management.md
```

Expected: no placeholder findings.

### Task 17: Commit in Small Batches

**Files:**

- All files changed in implementation.

- [x] **Step 1: Commit type foundations and builder extraction**

Run:

```bash
git add packages/core/src/context packages/core/src/types.ts packages/core/src/PageAgentCore.ts
git commit -m "feat(core): add context prompt builder"
```

- [x] **Step 2: Commit compact and runtime context**

Run:

```bash
git add packages/core/src/context packages/core/src/PageAgentCore.ts packages/core/src/prompts/system_prompt.md
git commit -m "feat(core): add bounded context runtime"
```

- [x] **Step 3: Commit continuation resolver**

Run:

```bash
git add packages/core/src/context packages/extension/src/agent/sessionContinuation.test.ts packages/extension/src/agent/sessionContinuation.ts
git commit -m "feat(core): classify browser task continuation"
```

- [x] **Step 4: Commit docs**

Because `docs/` is ignored by the global gitignore in this workspace, force-add the docs:

```bash
git add -f docs/page-agent-long-running-context-management-design.zh.md docs/superpowers/plans/2026-05-26-page-agent-long-running-context-management.md
git commit -m "docs: plan long-running context management"
```

---

## Completion Checklist

- [x] Default behavior is unchanged when `context.enabled` is absent or `false`.
- [x] Context-enabled prompts include current page, recent steps, protected facts, and context rules.
- [x] Old history does not grow linearly inside the prompt after compact threshold.
- [x] Deterministic summary does not preserve executable DOM indexes.
- [x] ContinuationResolver handles:
  - [x] user answers pending question
  - [x] same page, new task
  - [x] new page, same business object
  - [x] new page, new task
  - [x] high-risk resume requires confirmation
- [x] No runtime code depends on filesystem writes.
- [x] `npm test -- packages/core/src/context` passes.
- [x] `npm run typecheck` passes.
- [x] Any `npm run lint` findings are fixed or documented as pre-existing.
