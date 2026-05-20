#!/bin/bash
set -e

PALACE_PATH="${MEMPALACE_PALACE_PATH:-/data/palace}"
export MEMPALACE_PALACE_PATH="$PALACE_PATH"

if [ ! -d "$PALACE_PATH" ]; then
    echo "Initializing palace at $PALACE_PATH"
    mkdir -p "$PALACE_PATH"
    mempalace init --yes --no-llm "$PALACE_PATH"
fi

exec supergateway \
    --stdio "python3 -m mempalace.mcp_server --palace $PALACE_PATH" \
    --port 8080 \
    --outputTransport streamableHttp \
    --healthEndpoint /healthz
