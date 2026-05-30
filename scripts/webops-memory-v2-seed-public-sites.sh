#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_ID="${WEBOPS_PUBLIC_SITE_SEED_RUN_ID:-latest}"
ARTIFACT_ROOT="${WEBOPS_PUBLIC_SITE_SEED_ARTIFACT_DIR:-$ROOT_DIR/artifacts/webops-memory-v2-public-sites/$RUN_ID}"
WORKFLOW_BACKEND_ADDR="${WEBOPS_PUBLIC_SITE_BACKEND_ADDR:-127.0.0.1:38402}"
BACKEND_URL="http://$WORKFLOW_BACKEND_ADDR"
BACKEND_PID=""
STARTED_BACKEND=0

if [[ "${KEEP_WEBOPS_PUBLIC_SITE_SEED_ARTIFACTS:-0}" != "1" ]]; then
	rm -rf "$ARTIFACT_ROOT"
fi
mkdir -p "$ARTIFACT_ROOT"

cleanup() {
	if [[ "$STARTED_BACKEND" == "1" && -n "$BACKEND_PID" ]] && kill -0 "$BACKEND_PID" >/dev/null 2>&1; then
		kill "$BACKEND_PID" >/dev/null 2>&1 || true
		wait "$BACKEND_PID" >/dev/null 2>&1 || true
	fi
}
trap cleanup EXIT

require_command() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "Missing required command: $1" >&2
		exit 1
	fi
}

wait_for_url() {
	local url="$1"
	for _ in $(seq 1 90); do
		if curl --noproxy '*' -fsS "$url" >/dev/null 2>&1; then
			return 0
		fi
		sleep 1
	done
	echo "Backend did not become ready: $url" >&2
	return 1
}

require_command go
require_command node
require_command curl

if ! curl --noproxy '*' -fsS "$BACKEND_URL/ready" >/dev/null 2>&1; then
	(
		cd "$ROOT_DIR/apps/workflow-backend"
		WORKFLOW_BACKEND_ADDR="$WORKFLOW_BACKEND_ADDR" \
			WORKFLOW_STORAGE_BACKEND=file \
			WORKFLOW_DATA_DIR="${WORKFLOW_DATA_DIR:-$HOME/.page-agent/workflow-backend}" \
			NO_PROXY="127.0.0.1,localhost" \
			no_proxy="127.0.0.1,localhost" \
			go run ./cmd/server
	) >"$ARTIFACT_ROOT/backend.log" 2>&1 &
	BACKEND_PID="$!"
	STARTED_BACKEND=1
	wait_for_url "$BACKEND_URL/ready"
fi

(
	cd "$ROOT_DIR"
	NO_PROXY="127.0.0.1,localhost" \
		no_proxy="127.0.0.1,localhost" \
		WEBOPS_MEMORY_V2_BACKEND_URL="$BACKEND_URL" \
		WEBOPS_PUBLIC_SITE_SEED_ARTIFACT_DIR="$ARTIFACT_ROOT" \
		node scripts/webops-memory-v2-seed-public-sites.mjs
)

echo "Public site memory seed artifacts: $ARTIFACT_ROOT"
