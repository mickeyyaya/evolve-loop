#!/usr/bin/env bash
# fake-claude-streaming.sh — simulates claude -p --output-format=stream-json
# emitting JSONL events incrementally over time. Used by T-stream-drv.8 to
# verify that bridge captures progressive writes (vs. the default text-mode
# behavior where stdout is buffered until session end).
#
# Behavior:
#   - When --output-format=stream-json is in argv, emit N JSONL events
#     spread over $delay seconds (default 1 event every 0.5s, total ~3s).
#   - Each event is a single printf, so the parent's stdout_log grows
#     incrementally rather than in a single write at exit.
#   - When --output-format=stream-json is NOT present, behave like fake-claude.sh
#     (single buffered write at end), letting tests verify the contrast.
#   - Always produce the expected artifact at the path embedded in the prompt
#     so bridge artifact-verification succeeds either way.
#
# Tuning via env:
#   BRIDGE_FAKE_STREAM_EVENTS   — number of JSONL events to emit (default 5)
#   BRIDGE_FAKE_STREAM_DELAY_S  — seconds between events (default 0.5)

set -uo pipefail

if [[ "${1:-}" == "--version" ]]; then
  echo "fake-claude-streaming 0.0.1 (Mock)"
  exit 0
fi

streaming=0
prompt=""
prev=""
for a in "$@"; do
  if [[ "$prev" == "--output-format" && "$a" == "stream-json" ]]; then
    streaming=1
  fi
  if [[ "$prev" == "-p" ]]; then
    prompt="$a"
  fi
  prev="$a"
done

events="${BRIDGE_FAKE_STREAM_EVENTS:-5}"
delay="${BRIDGE_FAKE_STREAM_DELAY_S:-0.5}"

artifact_path=$(echo "$prompt" | grep -oE '/[^ "]*\.md' | head -1)
token=$(echo "$prompt" | grep -oE 'challenge-token: [a-f0-9]+' | awk '{print $2}' | head -1)

if [[ "$streaming" == "1" ]]; then
  i=1
  while [ "$i" -le "$events" ]; do
    printf '{"type":"message_delta","seq":%d,"content":"chunk %d/%d at t=%s"}\n' "$i" "$i" "$events" "$(date +%s)"
    sleep "$delay"
    i=$((i + 1))
  done
  printf '{"type":"message_stop","seq":%d,"total_events":%d}\n' "$((events + 1))" "$events"
else
  sleep "$(awk "BEGIN { print $events * $delay }")"
  printf 'Final response (text mode).\n'
fi

if [[ -n "$artifact_path" ]]; then
  mkdir -p "$(dirname "$artifact_path")"
  cat > "$artifact_path" <<EOF
<!-- challenge-token: ${token:-unknown} -->
PROTOTYPE OK (FAKE-CLAUDE-STREAMING streaming=$streaming events=$events)
EOF
fi

exit 0
