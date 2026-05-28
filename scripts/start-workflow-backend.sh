#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

export WORKFLOW_BACKEND_ADDR="${WORKFLOW_BACKEND_ADDR:-127.0.0.1:38402}"
export WORKFLOW_DATA_DIR="${WORKFLOW_DATA_DIR:-$HOME/.page-agent/workflow-backend}"
export QDRANT_URL="${QDRANT_URL:-http://127.0.0.1:6333}"
export QDRANT_GRPC_URL="${QDRANT_GRPC_URL:-127.0.0.1:6334}"
export QDRANT_COLLECTION_PREFIX="${QDRANT_COLLECTION_PREFIX:-pa}"
export LLM_MODEL="${LLM_MODEL:-gpt-5.4}"

cd "$ROOT_DIR/apps/workflow-backend"
exec go run ./cmd/server
