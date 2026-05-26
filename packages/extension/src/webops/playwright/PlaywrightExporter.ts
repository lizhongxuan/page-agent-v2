import type { RecordedSession } from '../recorder/actionEvents'
import { packageJsonTemplate } from './templates/packageJson'
import { playwrightConfigTemplate } from './templates/playwrightConfig'
import { readmeTemplate } from './templates/readme'
import { testFileTemplate } from './templates/testFile'

export type ExportedPlaywrightProject = Record<string, string>

export function exportPlaywrightProject(session: RecordedSession): ExportedPlaywrightProject {
	return {
		'package.json': packageJsonTemplate(),
		'playwright.config.ts': playwrightConfigTemplate(),
		'README.md': readmeTemplate(session),
		'tests/replay.spec.ts': testFileTemplate(session),
		'fixtures/session.json': JSON.stringify(session, null, 2),
	}
}
