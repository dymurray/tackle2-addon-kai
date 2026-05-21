#!/bin/bash
set -e

PALACE_PATH="${MEMPALACE_PALACE_PATH:-/data/palace}"
export MEMPALACE_PALACE_PATH="$PALACE_PATH"

if [ ! -d "$PALACE_PATH" ]; then
    echo "Initializing palace at $PALACE_PATH"
    mkdir -p "$PALACE_PATH"
    mempalace init --yes --no-llm "$PALACE_PATH"
fi

echo "Warming up embedding model..."
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"mempalace_search","arguments":{"query":"warmup"}}}' \
    | timeout 120 python3 -m mempalace.mcp_server --palace "$PALACE_PATH" > /dev/null 2>&1
echo "Embedding model ready."

exec supergateway \
    --stdio "python3 -m mempalace.mcp_server --palace $PALACE_PATH" \
    --port 8080 \
    --outputTransport streamableHttp \
    --healthEndpoint /healthz
