#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ACTION="${1:-start}"

RUNTIME_DIR="${PAGE_AGENT_RUNTIME_DIR:-$HOME/.page-agent/runtime}"
QDRANT_CONTAINER_NAME="${QDRANT_CONTAINER_NAME:-page-agent-qdrant}"
QDRANT_STORAGE_DIR="${QDRANT_STORAGE_DIR:-$HOME/.page-agent/qdrant}"
QDRANT_HTTP_PORT="${QDRANT_HTTP_PORT:-6333}"
QDRANT_GRPC_PORT="${QDRANT_GRPC_PORT:-6334}"
WORKFLOW_BACKEND_ADDR="${WORKFLOW_BACKEND_ADDR:-127.0.0.1:38402}"
WORKFLOW_DATA_DIR="${WORKFLOW_DATA_DIR:-$HOME/.page-agent/workflow-backend}"
QDRANT_COLLECTION_PREFIX="${QDRANT_COLLECTION_PREFIX:-pa}"
BACKEND_BIN="$RUNTIME_DIR/workflow-backend"
BACKEND_PID_FILE="$RUNTIME_DIR/workflow-backend.pid"
BACKEND_LOG="$RUNTIME_DIR/workflow-backend.log"

mkdir -p "$RUNTIME_DIR" "$QDRANT_STORAGE_DIR" "$WORKFLOW_DATA_DIR"

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

pid_is_running() {
	if [[ -f "$BACKEND_PID_FILE" ]] && kill -0 "$(cat "$BACKEND_PID_FILE")" >/dev/null 2>&1; then
		return 0
	fi
	backend_listener_pid >/dev/null
}

backend_listener_pid() {
	local port="${WORKFLOW_BACKEND_ADDR##*:}"
	if command -v lsof >/dev/null 2>&1; then
		lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null | head -1
	fi
}

start_qdrant() {
	require_command docker
	if docker ps --format '{{.Names}}' | grep -Fx "$QDRANT_CONTAINER_NAME" >/dev/null; then
		echo "Qdrant already running: $QDRANT_CONTAINER_NAME"
	else
		if docker ps -a --format '{{.Names}}' | grep -Fx "$QDRANT_CONTAINER_NAME" >/dev/null; then
			docker start "$QDRANT_CONTAINER_NAME" >/dev/null
		else
			docker run -d \
				--name "$QDRANT_CONTAINER_NAME" \
				--restart unless-stopped \
				-p "127.0.0.1:$QDRANT_HTTP_PORT:6333" \
				-p "127.0.0.1:$QDRANT_GRPC_PORT:6334" \
				-v "$QDRANT_STORAGE_DIR:/qdrant/storage" \
				qdrant/qdrant:latest >/dev/null
		fi
	fi
	wait_for_url "http://127.0.0.1:$QDRANT_HTTP_PORT/" "Qdrant"
}

build_backend() {
	require_command go
	(
		cd "$ROOT_DIR/apps/workflow-backend"
		go build -o "$BACKEND_BIN" ./cmd/server
	)
	echo "Backend binary: $BACKEND_BIN"
}

start_backend() {
	if pid_is_running; then
		echo "Workflow backend already running with PID $(backend_listener_pid || cat "$BACKEND_PID_FILE")"
		return
	fi
	WORKFLOW_BACKEND_ADDR="$WORKFLOW_BACKEND_ADDR" \
		WORKFLOW_DATA_DIR="$WORKFLOW_DATA_DIR" \
		QDRANT_URL="http://127.0.0.1:$QDRANT_HTTP_PORT" \
		QDRANT_GRPC_URL="127.0.0.1:$QDRANT_GRPC_PORT" \
		QDRANT_COLLECTION_PREFIX="$QDRANT_COLLECTION_PREFIX" \
		LLM_BASE_URL="${LLM_BASE_URL:-}" \
		LLM_API_KEY="${LLM_API_KEY:-}" \
		LLM_MODEL="${LLM_MODEL:-gpt-5.4}" \
		EMBEDDING_BASE_URL="${EMBEDDING_BASE_URL:-}" \
		EMBEDDING_API_KEY="${EMBEDDING_API_KEY:-}" \
		EMBEDDING_MODEL="${EMBEDDING_MODEL:-bge-m3}" \
		NO_PROXY="127.0.0.1,localhost,${NO_PROXY:-}" \
		no_proxy="127.0.0.1,localhost,${no_proxy:-}" \
		nohup "$BACKEND_BIN" >"$BACKEND_LOG" 2>&1 &
	local backend_pid="$!"
	echo "$backend_pid" >"$BACKEND_PID_FILE"
	disown "$backend_pid" >/dev/null 2>&1 || true
	wait_for_url "http://$WORKFLOW_BACKEND_ADDR/ready" "Workflow backend"
}

build_extension() {
	if [[ "${BUILD_EXTENSION:-1}" == "0" ]]; then
		return
	fi
	require_command npm
	(
		cd "$ROOT_DIR"
		npm run build:ext:desktop
	)
}

start_stack() {
	require_command curl
	start_qdrant
	build_backend
	start_backend
	build_extension
	echo "Workflow backend URL: http://$WORKFLOW_BACKEND_ADDR"
	echo "Qdrant URL: http://127.0.0.1:$QDRANT_HTTP_PORT"
	echo "Backend log: $BACKEND_LOG"
}

stop_stack() {
	if pid_is_running; then
		kill "$(cat "$BACKEND_PID_FILE")" >/dev/null 2>&1 || true
		rm -f "$BACKEND_PID_FILE"
		echo "Stopped workflow backend."
	else
		echo "Workflow backend is not running."
	fi
	if [[ "${STOP_QDRANT:-1}" == "1" ]]; then
		docker stop "$QDRANT_CONTAINER_NAME" >/dev/null 2>&1 || true
		echo "Stopped Qdrant container: $QDRANT_CONTAINER_NAME"
	fi
}

status_stack() {
	if pid_is_running; then
		echo "Workflow backend: running PID $(backend_listener_pid || cat "$BACKEND_PID_FILE")"
	else
		echo "Workflow backend: stopped"
	fi
	docker ps --format '{{.Names}}' | grep -Fx "$QDRANT_CONTAINER_NAME" >/dev/null 2>&1 \
		&& echo "Qdrant: running ($QDRANT_CONTAINER_NAME)" \
		|| echo "Qdrant: stopped ($QDRANT_CONTAINER_NAME)"
	curl --noproxy '*' -fsS "http://$WORKFLOW_BACKEND_ADDR/health" >/dev/null 2>&1 \
		&& echo "Backend health: ok" \
		|| echo "Backend health: unavailable"
}

case "$ACTION" in
	start)
		start_stack
		;;
	stop)
		stop_stack
		;;
	restart)
		stop_stack
		start_stack
		;;
	status)
		status_stack
		;;
	*)
		echo "Usage: $0 {start|stop|restart|status}" >&2
		exit 1
		;;
esac
