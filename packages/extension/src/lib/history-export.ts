import type { HistoricalEvent } from '@page-agent/core'

import { exportPlaywrightProject } from '../webops/playwright/PlaywrightExporter'
import type { RecordedSession } from '../webops/recorder/actionEvents'
import { createZipBlob } from './zip'

const EXPORT_FILE_PREFIX = 'page-agent-history'
const MAX_TASK_SLUG_LENGTH = 40

export function serializeHistoryExport(history: HistoricalEvent[]): string {
	return `${JSON.stringify(history, null, 2)}\n`
}

export function buildHistoryExportFilename(task: string, createdAt: number): string {
	const taskSlug = sanitizeTaskForFilename(task)
	const timestamp = formatTimestampForFilename(createdAt)

	return taskSlug
		? `${EXPORT_FILE_PREFIX}-${taskSlug}-${timestamp}.json`
		: `${EXPORT_FILE_PREFIX}-${timestamp}.json`
}

export function downloadHistoryExport(
	task: string,
	createdAt: number,
	history: HistoricalEvent[]
): void {
	const filename = buildHistoryExportFilename(task, createdAt)
	const content = serializeHistoryExport(history)
	const blob = new Blob([content], { type: 'application/json;charset=utf-8' })
	const url = URL.createObjectURL(blob)
	const link = document.createElement('a')

	link.href = url
	link.download = filename
	link.click()

	URL.revokeObjectURL(url)
}

export function downloadPlaywrightExport(
	task: string,
	createdAt: number,
	webOpsSession: RecordedSession
): void {
	const { filename, blob } = buildPlaywrightExport(task, createdAt, webOpsSession)
	const url = URL.createObjectURL(blob)
	const link = document.createElement('a')

	link.href = url
	link.download = filename
	link.click()

	URL.revokeObjectURL(url)
}

export function buildPlaywrightExport(
	task: string,
	createdAt: number,
	webOpsSession: RecordedSession
): { filename: string; blob: Blob } {
	const filename = buildPlaywrightExportFilename(task, createdAt)
	const files = exportPlaywrightProject(webOpsSession)
	const blob = createZipBlob(files)

	return { filename, blob }
}

function buildPlaywrightExportFilename(task: string, createdAt: number): string {
	const taskSlug = sanitizeTaskForFilename(task)
	const timestamp = formatTimestampForFilename(createdAt)

	return taskSlug
		? `page-agent-v2-replay-${taskSlug}-${timestamp}.zip`
		: `page-agent-v2-replay-${timestamp}.zip`
}

function sanitizeTaskForFilename(task: string): string {
	return task
		.trim()
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/^-+|-+$/g, '')
		.slice(0, MAX_TASK_SLUG_LENGTH)
}

function formatTimestampForFilename(createdAt: number): string {
	const date = new Date(createdAt)
	const year = date.getFullYear()
	const month = pad(date.getMonth() + 1)
	const day = pad(date.getDate())
	const hours = pad(date.getHours())
	const minutes = pad(date.getMinutes())
	const seconds = pad(date.getSeconds())

	return `${year}-${month}-${day}_${hours}-${minutes}-${seconds}`
}

function pad(value: number): string {
	return value.toString().padStart(2, '0')
}
