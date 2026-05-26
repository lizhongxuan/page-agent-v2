# Upstream Tracking

Upstream repository: https://github.com/alibaba/page-agent
Fork repository: https://github.com/lizhongxuan/page-agent-v2
Base commit: ebc785d
Base version: 1.8.2

## Local Product Direction

page-agent-v2 keeps Page Agent behavior and adds:

1. Page interaction components.
2. Playwright replay export.
3. Project knowledge base assistance.

## Sync Rules

- Sync upstream bug fixes and Chrome compatibility changes.
- Keep local WebOps modules under packages/extension/src/webops.
- Do not rewrite Page Agent core packages for first release.
