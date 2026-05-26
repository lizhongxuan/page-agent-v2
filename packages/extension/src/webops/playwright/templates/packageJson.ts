export function packageJsonTemplate() {
	return JSON.stringify(
		{
			name: 'page-agent-v2-replay',
			private: true,
			type: 'module',
			scripts: { test: 'playwright test' },
			devDependencies: { '@playwright/test': '^1.57.0' },
		},
		null,
		2
	)
}
