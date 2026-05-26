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
