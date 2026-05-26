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
