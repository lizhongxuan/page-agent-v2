import type { RecordedSession } from '../../recorder/actionEvents'

export function readmeTemplate(session: RecordedSession) {
	return `# Page Agent V2 Replay

Task: ${session.task}

Start URL: ${session.startUrl}

## Usage

\`\`\`bash
npm install
npm test
\`\`\`

Sensitive values such as passwords, tokens, API keys, CAPTCHA, and MFA codes are redacted before export. Manual handover steps are emitted as comments and must be completed by a human.

## Selector repair

If a replay step fails because the page changed, edit \`tests/replay.spec.ts\` and replace the failing locator with a more stable one. Prefer \`getByTestId\`, then \`getByRole(..., { name })\`, then label, placeholder, visible text, CSS, or XPath. Keep manual login, MFA, CAPTCHA, and password steps as human steps instead of automating them.
`
}
