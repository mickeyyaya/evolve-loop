#!/usr/bin/env bash
# probe-proxy.sh — verify EVOLVE_ANTHROPIC_BASE_URL endpoint is reachable.
# Usage: bash legacy/scripts/utility/probe-proxy.sh [URL]
# If URL omitted, reads EVOLVE_ANTHROPIC_BASE_URL from environment.
# Exit 0 = reachable, exit 1 = not configured, exit 2 = unreachable.

set -uo pipefail

TIMEOUT=5

TARGET="${1:-${EVOLVE_ANTHROPIC_BASE_URL:-}}"

if [ -z "$TARGET" ]; then
    echo "[probe-proxy] EVOLVE_ANTHROPIC_BASE_URL is not set — proxy mode inactive (subscription auth via ~/.claude.json is the default)" >&2
    exit 1
fi

# Strip trailing /v1 or /v1/ to get the base host for a health check.
BASE="${TARGET%/v1}"
BASE="${BASE%/v1/}"

# Prefer curl; fall back to nc for a TCP-only check.
if command -v curl >/dev/null 2>&1; then
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
        --max-time "$TIMEOUT" --connect-timeout "$TIMEOUT" \
        -X POST "${BASE}/v1/messages" \
        -H "content-type: application/json" \
        -d '{}' 2>/dev/null) || HTTP_CODE="000"
    # Any HTTP response (even 4xx/5xx) means the endpoint is up.
    if [ "$HTTP_CODE" != "000" ]; then
        echo "[probe-proxy] OK: $TARGET responded HTTP $HTTP_CODE" >&2
        exit 0
    fi
    echo "[probe-proxy] UNREACHABLE: $TARGET timed out or connection refused (timeout=${TIMEOUT}s)" >&2
    exit 2
elif command -v nc >/dev/null 2>&1; then
    # Extract host and port from URL for nc check.
    HOST=$(echo "$BASE" | sed 's|.*://||' | cut -d: -f1)
    PORT=$(echo "$BASE" | sed 's|.*://||' | cut -d: -f2 | cut -d/ -f1)
    PORT="${PORT:-80}"
    if nc -z -w "$TIMEOUT" "$HOST" "$PORT" 2>/dev/null; then
        echo "[probe-proxy] OK: $HOST:$PORT is reachable (TCP)" >&2
        exit 0
    fi
    echo "[probe-proxy] UNREACHABLE: $HOST:$PORT refused connection (timeout=${TIMEOUT}s)" >&2
    exit 2
else
    echo "[probe-proxy] WARN: neither curl nor nc available — cannot probe $TARGET" >&2
    exit 2
fi
