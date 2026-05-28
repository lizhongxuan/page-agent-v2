#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

(
	cd "$ROOT_DIR/apps/workflow-backend"
	go test ./...
)

npm test -- \
	packages/extension/src/webops/workflow \
	packages/extension/src/webops/recorder \
	packages/extension/src/agent/MultiPageAgent.workflow.test.ts \
	packages/extension/src/agent/TabsController.background.test.ts
npx playwright test tests/qdrant-workflow-retrieval.spec.ts tests/qdrant-workflow-repair.spec.ts
