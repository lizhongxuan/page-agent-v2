#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_ID="$(date +%Y%m%d-%H%M%S)"
ARTIFACT_ROOT="${SITE_TASK_GUIDE_BENCHMARK_ARTIFACT_DIR:-$ROOT_DIR/artifacts/site-task-guide-real-user-benchmark/$RUN_ID}"
LATEST_ARTIFACT_DIR="$ROOT_DIR/artifacts/site-task-guide-real-user-benchmark/latest"

mkdir -p "$ARTIFACT_ROOT"
rm -rf "$LATEST_ARTIFACT_DIR"
ln -s "$ARTIFACT_ROOT" "$LATEST_ARTIFACT_DIR"

cd "$ROOT_DIR"
SITE_TASK_GUIDE_BENCHMARK_ARTIFACT_DIR="$ARTIFACT_ROOT" \
	node scripts/site-task-guide-real-user-benchmark.mjs

echo "Benchmark artifacts: $ARTIFACT_ROOT"
