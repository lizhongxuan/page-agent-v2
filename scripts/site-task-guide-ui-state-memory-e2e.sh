#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

export LLM_BASE_URL="${LLM_BASE_URL:-https://www.aicodexcn.com/v1}"
export LLM_MODEL="${LLM_MODEL:-gpt-5.4}"

node scripts/site-task-guide-ui-state-memory-e2e.mjs
