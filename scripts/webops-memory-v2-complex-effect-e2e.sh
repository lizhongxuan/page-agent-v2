#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_ID="${WEBOPS_MEMORY_V2_COMPLEX_E2E_RUN_ID:-latest}"
ARTIFACT_ROOT="${WEBOPS_MEMORY_V2_COMPLEX_E2E_ARTIFACT_DIR:-$ROOT_DIR/artifacts/webops-memory-v2-complex/$RUN_ID}"
LATEST_ARTIFACT_DIR="$ROOT_DIR/artifacts/webops-memory-v2-complex/latest"

POSTGRES_CONTAINER_NAME="${WEBOPS_MEMORY_V2_COMPLEX_POSTGRES_CONTAINER:-page-agent-webops-memory-v2-complex-postgres-e2e}"
POSTGRES_PORT="${WEBOPS_MEMORY_V2_COMPLEX_POSTGRES_PORT:-54334}"
POSTGRES_DB="${WEBOPS_MEMORY_V2_COMPLEX_POSTGRES_DB:-page_agent_webops_memory_v2_complex_e2e}"
POSTGRES_USER="${WEBOPS_MEMORY_V2_COMPLEX_POSTGRES_USER:-page_agent}"
POSTGRES_PASSWORD="${WEBOPS_MEMORY_V2_COMPLEX_POSTGRES_PASSWORD:-page_agent_dev}"
WORKFLOW_BACKEND_ADDR="${WEBOPS_MEMORY_V2_COMPLEX_BACKEND_ADDR:-127.0.0.1:38434}"

BACKEND_PID=""

if [[ "${KEEP_WEBOPS_MEMORY_V2_COMPLEX_E2E_ARTIFACTS:-0}" != "1" ]]; then
	rm -rf "$ARTIFACT_ROOT"
fi
mkdir -p "$ARTIFACT_ROOT"

cleanup() {
	if [[ -n "$BACKEND_PID" ]] && kill -0 "$BACKEND_PID" >/dev/null 2>&1; then
		kill "$BACKEND_PID" >/dev/null 2>&1 || true
		wait "$BACKEND_PID" >/dev/null 2>&1 || true
	fi
	docker logs "$POSTGRES_CONTAINER_NAME" >"$ARTIFACT_ROOT/postgres.log" 2>&1 || true
	if [[ "${KEEP_WEBOPS_MEMORY_V2_COMPLEX_E2E:-0}" != "1" ]]; then
		docker rm -f "$POSTGRES_CONTAINER_NAME" >/dev/null 2>&1 || true
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

stop_listener_on_port() {
	local port="$1"
	if ! command -v lsof >/dev/null 2>&1; then
		return 0
	fi
	local pids
	pids="$(lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null || true)"
	if [[ -z "$pids" ]]; then
		return 0
	fi
	echo "Stopping stale listener(s) on port $port: $pids"
	kill $pids >/dev/null 2>&1 || true
	for _ in $(seq 1 30); do
		if ! lsof -tiTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
			return 0
		fi
		sleep 0.2
	done
	kill -9 $pids >/dev/null 2>&1 || true
}

require_command docker
require_command go
require_command node
require_command curl

stop_listener_on_port "$POSTGRES_PORT"
stop_listener_on_port "${WORKFLOW_BACKEND_ADDR##*:}"

docker rm -f "$POSTGRES_CONTAINER_NAME" >/dev/null 2>&1 || true
docker run -d \
	--name "$POSTGRES_CONTAINER_NAME" \
	-e POSTGRES_DB="$POSTGRES_DB" \
	-e POSTGRES_USER="$POSTGRES_USER" \
	-e POSTGRES_PASSWORD="$POSTGRES_PASSWORD" \
	-p "127.0.0.1:$POSTGRES_PORT:5432" \
	pgvector/pgvector:pg17 >"$ARTIFACT_ROOT/postgres.container"

for _ in $(seq 1 90); do
	if docker exec "$POSTGRES_CONTAINER_NAME" pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB" >/dev/null 2>&1; then
		echo "Postgres ready on 127.0.0.1:$POSTGRES_PORT"
		break
	fi
	sleep 1
done

WORKFLOW_POSTGRES_URL="postgres://$POSTGRES_USER:$POSTGRES_PASSWORD@127.0.0.1:$POSTGRES_PORT/$POSTGRES_DB?sslmode=disable"

(
	cd "$ROOT_DIR/apps/workflow-backend"
	WORKFLOW_BACKEND_ADDR="$WORKFLOW_BACKEND_ADDR" \
		WORKFLOW_STORAGE_BACKEND=postgres \
		WORKFLOW_POSTGRES_URL="$WORKFLOW_POSTGRES_URL" \
		WORKFLOW_DISABLE_QDRANT=true \
		NO_PROXY="127.0.0.1,localhost" \
		no_proxy="127.0.0.1,localhost" \
		go run ./cmd/server
) >"$ARTIFACT_ROOT/backend.log" 2>&1 &
BACKEND_PID="$!"

wait_for_url "http://$WORKFLOW_BACKEND_ADDR/ready" "WebOps memory backend"

(
	cd "$ROOT_DIR"
	NO_PROXY="127.0.0.1,localhost" \
		no_proxy="127.0.0.1,localhost" \
		WEBOPS_MEMORY_V2_BACKEND_URL="http://$WORKFLOW_BACKEND_ADDR" \
		WEBOPS_MEMORY_V2_COMPLEX_ARTIFACT_DIR="$ARTIFACT_ROOT" \
		node scripts/webops-memory-v2-complex-effect-e2e.mjs
)

if [[ "$ARTIFACT_ROOT" != "$LATEST_ARTIFACT_DIR" ]]; then
	rm -rf "$LATEST_ARTIFACT_DIR"
	mkdir -p "$(dirname "$LATEST_ARTIFACT_DIR")"
	ln -s "$ARTIFACT_ROOT" "$LATEST_ARTIFACT_DIR"
fi

echo "WebOps memory V2 complex effect E2E artifacts: $ARTIFACT_ROOT"
echo "Stable latest artifacts: $LATEST_ARTIFACT_DIR"
