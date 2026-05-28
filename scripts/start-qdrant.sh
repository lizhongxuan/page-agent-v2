#!/usr/bin/env bash
set -euo pipefail

QDRANT_STORAGE_DIR="${QDRANT_STORAGE_DIR:-$HOME/.page-agent/qdrant}"
QDRANT_HTTP_PORT="${QDRANT_HTTP_PORT:-6333}"
QDRANT_GRPC_PORT="${QDRANT_GRPC_PORT:-6334}"

mkdir -p "$QDRANT_STORAGE_DIR"

if command -v docker >/dev/null 2>&1; then
	exec docker run --rm \
		-p "127.0.0.1:${QDRANT_HTTP_PORT}:6333" \
		-p "127.0.0.1:${QDRANT_GRPC_PORT}:6334" \
		-v "${QDRANT_STORAGE_DIR}:/qdrant/storage" \
		qdrant/qdrant:latest
fi

echo "Docker is required to start local Qdrant with this script." >&2
echo "Install Docker Desktop, then run: npm run workflow:qdrant" >&2
exit 1
