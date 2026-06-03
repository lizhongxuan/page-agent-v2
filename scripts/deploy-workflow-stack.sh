#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${PAGE_AGENT_ENV_FILE:-$ROOT_DIR/.env.local}"

if [[ -f "$ENV_FILE" ]]; then
	set -a
	# shellcheck disable=SC1090
	source "$ENV_FILE"
	set +a
fi

RUNTIME_DIR="${PAGE_AGENT_RUNTIME_DIR:-$HOME/.page-agent/runtime}"
WORKFLOW_BACKEND_ADDR="${WORKFLOW_BACKEND_ADDR:-127.0.0.1:38402}"
WORKFLOW_BACKEND_PORT="${WORKFLOW_BACKEND_ADDR##*:}"
WORKFLOW_DATA_DIR="${WORKFLOW_DATA_DIR:-$HOME/.page-agent/workflow-backend}"

POSTGRES_CONTAINER_NAME="${POSTGRES_CONTAINER_NAME:-page-agent-workflow-postgres}"
POSTGRES_DATA_DIR="${POSTGRES_DATA_DIR:-$HOME/.page-agent/postgres}"
POSTGRES_PORT="${POSTGRES_PORT:-54329}"
POSTGRES_DB="${POSTGRES_DB:-page_agent_workflow}"
POSTGRES_USER="${POSTGRES_USER:-page_agent}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-page_agent_dev}"
WORKFLOW_POSTGRES_URL="postgres://$POSTGRES_USER:$POSTGRES_PASSWORD@127.0.0.1:$POSTGRES_PORT/$POSTGRES_DB?sslmode=disable"
LEGACY_LAUNCH_AGENT_LABEL="com.page-agent.workflow-backend"
LEGACY_LAUNCH_AGENT_PLIST="$HOME/Library/LaunchAgents/$LEGACY_LAUNCH_AGENT_LABEL.plist"

BACKEND_BIN="$RUNTIME_DIR/workflow-backend"
BACKEND_LOG="$RUNTIME_DIR/workflow-backend.log"

mkdir -p "$RUNTIME_DIR" "$WORKFLOW_DATA_DIR" "$POSTGRES_DATA_DIR"

require_command() {
	local name="$1"
	if ! command -v "$name" >/dev/null 2>&1; then
		echo "Missing required command: $name" >&2
		exit 1
	fi
}

stop_docker_container_by_name() {
	local name="$1"
	if docker ps -a --format '{{.Names}}' | grep -Fx "$name" >/dev/null 2>&1; then
		echo "Removing existing Docker container: $name"
		docker rm -f "$name" >/dev/null 2>&1 || true
	fi
}

stop_docker_containers_on_port() {
	local port="$1"
	local containers
	containers="$(
		docker ps --format '{{.ID}} {{.Ports}}' |
			awk -v port=":$port->" 'index($0, port) > 0 {print $1}'
	)"
	if [[ -z "$containers" ]]; then
		return
	fi
	echo "Removing Docker container(s) using port $port: $containers"
	docker rm -f $containers >/dev/null 2>&1 || true
}

close_port() {
	local port="$1"
	local label="$2"
	local pids
	pids="$(lsof -nP -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null | sort -u || true)"
	if [[ -z "$pids" ]]; then
		return
	fi

	echo "Closing $label listener(s) on port $port: $pids"
	kill $pids >/dev/null 2>&1 || true
	for _ in $(seq 1 30); do
		if ! lsof -nP -tiTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
			return
		fi
		sleep 0.2
	done

	echo "Force closing $label listener(s) on port $port: $pids"
	kill -9 $pids >/dev/null 2>&1 || true
	for _ in $(seq 1 15); do
		if ! lsof -nP -tiTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
			return
		fi
		sleep 0.2
	done

	echo "Port $port is still occupied; cannot start $label." >&2
	exit 1
}

wait_for_backend() {
	local port="$1"
	local url="http://$WORKFLOW_BACKEND_ADDR/ready"
	local listener_pids
	for _ in $(seq 1 90); do
		listener_pids="$(lsof -nP -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null | sort -u || true)"
		if [[ -n "$listener_pids" ]] && curl --noproxy '*' -fsS "$url" >/dev/null 2>&1; then
			echo "Workflow backend ready: $url"
			return
		fi
		sleep 1
	done
	echo "Workflow backend did not become ready on port $port." >&2
	echo "Backend log: $BACKEND_LOG" >&2
	sed -n '1,160p' "$BACKEND_LOG" >&2 || true
	exit 1
}

extension_version() {
	node -e "const fs=require('fs'); const pkg=JSON.parse(fs.readFileSync(process.argv[1], 'utf8')); process.stdout.write(pkg.version)" "$ROOT_DIR/packages/extension/package.json"
}

extension_archive_name() {
	echo "page-agent-ext-$(extension_version)-chrome"
}

extension_unpacked_path() {
	echo "$HOME/Desktop/$(extension_archive_name)"
}

extension_zip_path() {
	echo "$HOME/Desktop/$(extension_archive_name).zip"
}

prepare_ports() {
	stop_legacy_launch_agent
	stop_docker_container_by_name "$POSTGRES_CONTAINER_NAME"
	stop_docker_containers_on_port "$POSTGRES_PORT"
	stop_docker_containers_on_port "$WORKFLOW_BACKEND_PORT"
	close_port "$POSTGRES_PORT" "PostgreSQL"
	close_port "$WORKFLOW_BACKEND_PORT" "workflow backend"
	rm -f "$RUNTIME_DIR/workflow-backend.pid"
}

stop_legacy_launch_agent() {
	local domain="gui/$(id -u)"
	if launchctl print "$domain/$LEGACY_LAUNCH_AGENT_LABEL" >/dev/null 2>&1; then
		echo "Stopping legacy LaunchAgent: $LEGACY_LAUNCH_AGENT_LABEL"
		launchctl bootout "$domain/$LEGACY_LAUNCH_AGENT_LABEL" >/dev/null 2>&1 || true
	fi
	launchctl disable "$domain/$LEGACY_LAUNCH_AGENT_LABEL" >/dev/null 2>&1 || true
	if [[ -f "$LEGACY_LAUNCH_AGENT_PLIST" ]]; then
		echo "Removing legacy LaunchAgent plist: $LEGACY_LAUNCH_AGENT_PLIST"
		rm -f "$LEGACY_LAUNCH_AGENT_PLIST"
	fi
}

start_postgres() {
	docker run -d \
		--name "$POSTGRES_CONTAINER_NAME" \
		--restart unless-stopped \
		-e POSTGRES_DB="$POSTGRES_DB" \
		-e POSTGRES_USER="$POSTGRES_USER" \
		-e POSTGRES_PASSWORD="$POSTGRES_PASSWORD" \
		-p "127.0.0.1:$POSTGRES_PORT:5432" \
		-v "$POSTGRES_DATA_DIR:/var/lib/postgresql/data" \
		pgvector/pgvector:pg17 >/dev/null

	for _ in $(seq 1 90); do
		if docker exec "$POSTGRES_CONTAINER_NAME" pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB" >/dev/null 2>&1; then
			echo "PostgreSQL ready: 127.0.0.1:$POSTGRES_PORT"
			return
		fi
		sleep 1
	done

	echo "PostgreSQL did not become ready on 127.0.0.1:$POSTGRES_PORT" >&2
	exit 1
}

build_backend() {
	(
		cd "$ROOT_DIR/apps/workflow-backend"
		go build -o "$BACKEND_BIN" ./cmd/server
	)
	echo "Backend binary: $BACKEND_BIN"
}

start_backend() {
	close_port "$WORKFLOW_BACKEND_PORT" "workflow backend"
	write_backend_launch_agent
	launchctl enable "gui/$(id -u)/$LEGACY_LAUNCH_AGENT_LABEL" >/dev/null 2>&1 || true
	launchctl bootstrap "gui/$(id -u)" "$LEGACY_LAUNCH_AGENT_PLIST" >/dev/null 2>&1 || true
	launchctl kickstart -k "gui/$(id -u)/$LEGACY_LAUNCH_AGENT_LABEL" >/dev/null 2>&1 || true
	wait_for_backend "$WORKFLOW_BACKEND_PORT"
}

write_backend_launch_agent() {
	mkdir -p "$(dirname "$LEGACY_LAUNCH_AGENT_PLIST")"
	cat >"$LEGACY_LAUNCH_AGENT_PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>$LEGACY_LAUNCH_AGENT_LABEL</string>
  <key>ProgramArguments</key>
  <array>
    <string>/usr/bin/env</string>
    <string>WORKFLOW_BACKEND_ADDR=$WORKFLOW_BACKEND_ADDR</string>
    <string>WORKFLOW_DATA_DIR=$WORKFLOW_DATA_DIR</string>
    <string>WORKFLOW_STORAGE_BACKEND=postgres</string>
    <string>WORKFLOW_POSTGRES_URL=$WORKFLOW_POSTGRES_URL</string>
    <string>LLM_BASE_URL=${LLM_BASE_URL:-}</string>
    <string>LLM_API_KEY=${LLM_API_KEY:-}</string>
    <string>LLM_MODEL=${LLM_MODEL:-gpt-5.4}</string>
    <string>EMBEDDING_BASE_URL=${EMBEDDING_BASE_URL:-}</string>
    <string>EMBEDDING_API_KEY=${EMBEDDING_API_KEY:-}</string>
    <string>EMBEDDING_MODEL=${EMBEDDING_MODEL:-bge-m3}</string>
    <string>NO_PROXY=127.0.0.1,localhost,${NO_PROXY:-}</string>
    <string>no_proxy=127.0.0.1,localhost,${no_proxy:-}</string>
    <string>$BACKEND_BIN</string>
  </array>
  <key>KeepAlive</key>
  <true/>
  <key>RunAtLoad</key>
  <true/>
  <key>WorkingDirectory</key>
  <string>$RUNTIME_DIR</string>
  <key>StandardOutPath</key>
  <string>$BACKEND_LOG</string>
  <key>StandardErrorPath</key>
  <string>$BACKEND_LOG</string>
</dict>
</plist>
EOF
}

build_extension() {
	(
		cd "$ROOT_DIR"
		npm run build:ext:desktop
	)
}

print_summary() {
	echo
	echo "Local Page Agent workflow memory is running."
	echo
	echo "Workflow backend: http://$WORKFLOW_BACKEND_ADDR"
	echo "Set workflowBackend to: http://$WORKFLOW_BACKEND_ADDR"
	echo "Extension package: $(extension_zip_path)"
	echo
	echo "Backend:"
	echo "- URL: http://$WORKFLOW_BACKEND_ADDR"
	echo "- Port: $WORKFLOW_BACKEND_PORT"
	echo "- Health: http://$WORKFLOW_BACKEND_ADDR/health"
	echo "- Ready: http://$WORKFLOW_BACKEND_ADDR/ready"
	echo "- Inspector: http://$WORKFLOW_BACKEND_ADDR/api/memory/inspector?projectId=default"
	echo "- Log: $BACKEND_LOG"
	echo
	echo "Database:"
	echo "- PostgreSQL: 127.0.0.1:$POSTGRES_PORT"
	echo "- Container: $POSTGRES_CONTAINER_NAME"
	echo "- Data: $POSTGRES_DATA_DIR"
	echo
	echo "Chrome extension:"
	echo "- Load unpacked path: $(extension_unpacked_path)"
	echo "- Zip package: $(extension_zip_path)"
	echo
	echo "Extension settings:"
	echo "- Workflow Memory Backend: http://$WORKFLOW_BACKEND_ADDR"
	echo "- Project ID: default"
	echo "- API Key: leave empty for local backend"
	echo
	if [[ -z "${LLM_API_KEY:-}" ]]; then
		echo "LLM_API_KEY is not set. To enable LLM-backed summaries, put it in $ENV_FILE or export it before running this script."
	fi
}

require_command docker
require_command go
require_command npm
require_command node
require_command curl
require_command lsof

prepare_ports
start_postgres
build_backend
start_backend
build_extension
print_summary
