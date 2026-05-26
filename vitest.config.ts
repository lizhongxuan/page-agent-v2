import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vitest/config'

export default defineConfig({
	resolve: {
		alias: {
			'@': fileURLToPath(new URL('./packages/extension/src', import.meta.url)),
		},
	},
	test: {
		exclude: [
			'**/node_modules/**',
			'**/dist/**',
			'**/.output/**',
			'test-results/**',
			'tests/**/*.spec.ts',
		],
	},
})
