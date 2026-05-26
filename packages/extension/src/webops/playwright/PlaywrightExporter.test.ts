import { describe, expect, it } from 'vitest'

import type { RecordedSession } from '../recorder/actionEvents'
import { exportPlaywrightProject } from './PlaywrightExporter'

describe('exportPlaywrightProject', () => {
	it('exports the expected Playwright project files', () => {
		const session: RecordedSession = {
			id: 's1',
			task: '查看 kme 服务状态',
			startUrl: 'https://example.test/service',
			startedAt: 1,
			steps: [],
			knowledgeHits: [],
			redactionReport: [],
		}

		const files = exportPlaywrightProject(session)
		expect(Object.keys(files).sort()).toEqual([
			'README.md',
			'fixtures/session.json',
			'package.json',
			'playwright.config.ts',
			'tests/replay.spec.ts',
		])
		expect(files['README.md']).toContain('查看 kme 服务状态')
		expect(JSON.parse(files['fixtures/session.json'])).toMatchObject({ id: 's1' })
	})

	it('generates replay code and comments handover steps without automating them', () => {
		const session: RecordedSession = {
			id: 's1',
			task: '查看 kme 服务状态',
			startUrl: 'https://example.test/service',
			startedAt: 1,
			steps: [
				{
					id: 'a1',
					type: 'input',
					timestamp: 2,
					pageUrl: 'https://example.test/service',
					pageTitle: '服务管理',
					target: { name: '按照服务名称或容器IP搜索' },
					value: 'kme',
					result: 'success',
				},
				{
					id: 'a2',
					type: 'click',
					timestamp: 3,
					pageUrl: 'https://example.test/service',
					pageTitle: '服务管理',
					target: { role: 'button', name: '搜索' },
					result: 'success',
				},
				{
					id: 'a3',
					type: 'select',
					timestamp: 4,
					pageUrl: 'https://example.test/service',
					pageTitle: '服务管理',
					target: { role: 'combobox', name: '命名空间' },
					value: 'kce-system',
					result: 'success',
				},
				{
					id: 'a4',
					type: 'wait',
					timestamp: 5,
					pageUrl: 'https://example.test/service',
					pageTitle: '服务管理',
					result: 'success',
				},
				{
					id: 'a5',
					type: 'navigate',
					timestamp: 6,
					pageUrl: 'https://example.test/service',
					pageTitle: '服务管理',
					value: 'https://example.test/service/detail',
					result: 'success',
				},
				{
					id: 'a6',
					type: 'extract',
					timestamp: 7,
					pageUrl: 'https://example.test/service',
					pageTitle: '服务管理',
					target: { text: '运行中' },
					result: 'success',
				},
				{
					id: 'a7',
					type: 'observe',
					timestamp: 8,
					pageUrl: 'https://example.test/service',
					pageTitle: '服务管理',
					result: 'success',
					note: '读取服务表格',
				},
				{
					id: 'a8',
					type: 'handover',
					timestamp: 9,
					pageUrl: 'https://example.test/service',
					pageTitle: '服务管理',
					result: 'skipped',
					note: '请人工完成 MFA',
				},
			],
			knowledgeHits: [],
			redactionReport: [],
		}

		const files = exportPlaywrightProject(session)
		expect(files['tests/replay.spec.ts']).toContain(
			"await page.getByPlaceholder('按照服务名称或容器IP搜索').fill('kme')"
		)
		expect(files['tests/replay.spec.ts']).toContain(
			"await page.getByRole('button', { name: '搜索' }).click()"
		)
		expect(files['tests/replay.spec.ts']).toContain(
			"await page.getByRole('combobox', { name: '命名空间' }).selectOption('kce-system')"
		)
		expect(files['tests/replay.spec.ts']).toContain(
			"await page.goto('https://example.test/service/detail')"
		)
		expect(files['tests/replay.spec.ts']).toContain(
			"await expect(page.getByText('运行中')).toBeVisible()"
		)
		expect(files['tests/replay.spec.ts']).toContain('await page.waitForLoadState')
		expect(files['tests/replay.spec.ts']).toContain('// Observed page state: 读取服务表格')
		expect(files['tests/replay.spec.ts']).toContain('// Manual handover: 请人工完成 MFA')
		expect(files['tests/replay.spec.ts']).not.toContain("fill('请人工完成 MFA')")
	})

	it('explains how to repair selectors in the README', () => {
		const session: RecordedSession = {
			id: 's1',
			task: '查看 kme 服务状态',
			startUrl: 'https://example.test/service',
			startedAt: 1,
			steps: [],
			knowledgeHits: [],
			redactionReport: [],
		}

		const files = exportPlaywrightProject(session)
		expect(files['README.md']).toContain('Selector repair')
		expect(files['README.md']).toContain('getByRole')
	})
})
