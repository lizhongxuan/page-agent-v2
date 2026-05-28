#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ARTIFACT_ROOT="${WORKFLOW_TEST_ARTIFACT_DIR:-$ROOT_DIR/artifacts/workflow-user-guide}"
RUN_ID="$(date +%Y%m%d%H%M%S)"
LOG_DIR="$ARTIFACT_ROOT/_stack-$RUN_ID"

QDRANT_CONTAINER_NAME="${QDRANT_CONTAINER_NAME:-page-agent-qdrant-e2e}"
QDRANT_HTTP_PORT="${QDRANT_HTTP_PORT:-6339}"
QDRANT_GRPC_PORT="${QDRANT_GRPC_PORT:-6340}"
WORKFLOW_BACKEND_ADDR="${WORKFLOW_BACKEND_ADDR:-127.0.0.1:38422}"
WORKFLOW_DATA_DIR="${WORKFLOW_DATA_DIR:-$LOG_DIR/workflow-data}"
QDRANT_COLLECTION_PREFIX="${QDRANT_COLLECTION_PREFIX:-pa_e2e_$RUN_ID}"

BACKEND_PID=""

mkdir -p "$LOG_DIR"

cleanup() {
	if [[ -n "$BACKEND_PID" ]] && kill -0 "$BACKEND_PID" >/dev/null 2>&1; then
		kill "$BACKEND_PID" >/dev/null 2>&1 || true
		wait "$BACKEND_PID" >/dev/null 2>&1 || true
	fi
	docker logs "$QDRANT_CONTAINER_NAME" >"$LOG_DIR/qdrant.log" 2>&1 || true
	if [[ "${KEEP_WORKFLOW_E2E:-0}" != "1" ]]; then
		docker rm -f "$QDRANT_CONTAINER_NAME" >/dev/null 2>&1 || true
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
	local label="$2"
	for _ in $(seq 1 90); do
		if curl --noproxy '*' -fsS "$url" >/dev/null 2>&1; then
			echo "$label ready: $url"
			return 0
		fi
		sleep 1
	done
	echo "$label did not become ready: $url" >&2
	return 1
}

stop_existing_backend_on_port() {
	local port="${WORKFLOW_BACKEND_ADDR##*:}"
	if ! command -v lsof >/dev/null 2>&1; then
		return 0
	fi
	local pids
	pids="$(lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null || true)"
	if [[ -z "$pids" ]]; then
		return 0
	fi
	echo "Stopping stale workflow backend listener(s) on $WORKFLOW_BACKEND_ADDR: $pids"
	kill $pids >/dev/null 2>&1 || true
	for _ in $(seq 1 20); do
		if ! lsof -tiTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
			return 0
		fi
		sleep 0.2
	done
	kill -9 $pids >/dev/null 2>&1 || true
}

require_command docker
require_command npm
require_command go
require_command curl

stop_existing_backend_on_port

docker rm -f "$QDRANT_CONTAINER_NAME" >/dev/null 2>&1 || true
docker run -d \
	--name "$QDRANT_CONTAINER_NAME" \
	-p "127.0.0.1:$QDRANT_HTTP_PORT:6333" \
	-p "127.0.0.1:$QDRANT_GRPC_PORT:6334" \
	qdrant/qdrant:latest >"$LOG_DIR/qdrant.container"

wait_for_url "http://127.0.0.1:$QDRANT_HTTP_PORT/" "Qdrant"

(
	cd "$ROOT_DIR/apps/workflow-backend"
	WORKFLOW_DATA_DIR="$WORKFLOW_DATA_DIR" \
		WORKFLOW_BACKEND_ADDR="$WORKFLOW_BACKEND_ADDR" \
		QDRANT_URL="http://127.0.0.1:$QDRANT_HTTP_PORT" \
		QDRANT_GRPC_URL="127.0.0.1:$QDRANT_GRPC_PORT" \
		QDRANT_COLLECTION_PREFIX="$QDRANT_COLLECTION_PREFIX" \
		NO_PROXY="127.0.0.1,localhost" \
		no_proxy="127.0.0.1,localhost" \
		go run ./cmd/server
) >"$LOG_DIR/backend.log" 2>&1 &
BACKEND_PID="$!"

wait_for_url "http://$WORKFLOW_BACKEND_ADDR/ready" "Workflow backend"

(
	cd "$ROOT_DIR"
	npm run build:ext
)

cat >"$LOG_DIR/e2e-env.json" <<EOF
{
  "workflowBackendUrl": "http://$WORKFLOW_BACKEND_ADDR",
  "qdrantUrl": "http://127.0.0.1:$QDRANT_HTTP_PORT",
  "qdrantCollectionPrefix": "$QDRANT_COLLECTION_PREFIX",
  "workflowDataDir": "$WORKFLOW_DATA_DIR",
  "artifactRoot": "$ARTIFACT_ROOT"
}
EOF

if [[ "$#" -gt 0 ]]; then
	TESTS=("$@")
else
	TESTS=(
		"tests/qdrant-workflow-retrieval.spec.ts"
		"tests/qdrant-workflow-repair.spec.ts"
		"tests/page-agent-workflow-user-guide.spec.ts"
		"tests/page-agent-workflow-real-user.spec.ts"
	)
fi

(
	cd "$ROOT_DIR"
	NO_PROXY="127.0.0.1,localhost" \
		no_proxy="127.0.0.1,localhost" \
		WORKFLOW_BACKEND_URL="http://$WORKFLOW_BACKEND_ADDR" \
		PAGE_AGENT_EXTENSION_PATH="$ROOT_DIR/packages/extension/.output/chrome-mv3" \
		WORKFLOW_TEST_ARTIFACT_DIR="$ARTIFACT_ROOT" \
		npx playwright test "${TESTS[@]}"
)

echo "Workflow real-user artifacts: $ARTIFACT_ROOT"
echo "Stack logs: $LOG_DIR"
