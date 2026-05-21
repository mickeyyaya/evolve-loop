#!/usr/bin/env bash
# lib/report.sh — re-derive a structured JSON summary from a past workspace
#
# Reads the conventional files bridge writes to a workspace:
#   artifact (*.md, default name "artifact.md" but configurable)
#   challenge-token.txt
#   stdout.log, stderr.log
#   resolved-prompt.txt    (tmux variants)
#   tmux-final-scrollback.txt
#   escalation-report.json (auto-respond escalations)
#   auto-respond-counts.csv
#   snap-*.json            (billing snapshots, if any)
#
# Emits a single JSON object to stdout describing what's present.
#
# Verdict is derived from file state, not from a status file:
#   complete   — artifact present + (token matches if token file exists)
#   escalated  — escalation-report.json present
#   incomplete — no artifact, no escalation report
#   unknown    — workspace empty or unreadable

bridge_report_emit() {
  local workspace="$1"
  local artifact_name="${2:-artifact.md}"

  if [[ ! -d "$workspace" ]]; then
    echo "[bridge:report] workspace not a directory: $workspace" >&2
    return 1
  fi

  local artifact_path="$workspace/$artifact_name"
  local artifact_exists=false
  local artifact_size=0
  local artifact_has_token=false
  if [[ -f "$artifact_path" ]]; then
    artifact_exists=true
    artifact_size=$(wc -c < "$artifact_path" | tr -d ' ')
  fi

  local token_path="$workspace/challenge-token.txt"
  local token_value=""
  if [[ -f "$token_path" ]]; then
    token_value="$(cat "$token_path" | tr -d '\n')"
    if [[ "$artifact_exists" == "true" ]] && grep -q "$token_value" "$artifact_path" 2>/dev/null; then
      artifact_has_token=true
    fi
  fi

  local stdout_log="$workspace/stdout.log"
  local stderr_log="$workspace/stderr.log"
  local scrollback="$workspace/tmux-final-scrollback.txt"
  local resolved_prompt="$workspace/resolved-prompt.txt"
  local escalation="$workspace/escalation-report.json"
  local counts="$workspace/auto-respond-counts.csv"

  # Collect billing snapshots if any (snap-*.json)
  local snapshots_json="[]"
  if compgen -G "$workspace/snaps/snap-*.json" >/dev/null 2>&1; then
    snapshots_json=$(find "$workspace/snaps" -maxdepth 1 -name 'snap-*.json' -type f \
      | while read -r f; do
          jq -n --arg path "$f" --arg label "$(jq -r '.label // ""' "$f" 2>/dev/null)" \
            --arg ts "$(jq -r '.ts // ""' "$f" 2>/dev/null)" \
            '{path: $path, label: $label, ts: $ts}'
        done | jq -s '.')
  fi

  # Derive verdict
  local verdict="unknown"
  if [[ -f "$escalation" ]]; then
    verdict="escalated"
  elif [[ "$artifact_exists" == "true" ]]; then
    if [[ -z "$token_value" ]] || [[ "$artifact_has_token" == "true" ]]; then
      verdict="complete"
    else
      verdict="incomplete-token-mismatch"
    fi
  else
    verdict="incomplete"
  fi

  jq -n \
    --arg workspace "$workspace" \
    --arg scanned_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --arg verdict "$verdict" \
    --arg artifact_path "$artifact_path" \
    --argjson artifact_exists "$artifact_exists" \
    --argjson artifact_size "$artifact_size" \
    --argjson artifact_has_token "$artifact_has_token" \
    --arg token_value "$token_value" \
    --arg stdout_log "$stdout_log" \
    --arg stderr_log "$stderr_log" \
    --arg scrollback "$scrollback" \
    --arg resolved_prompt "$resolved_prompt" \
    --arg escalation_path "$escalation" \
    --arg counts_path "$counts" \
    --argjson snapshots "$snapshots_json" \
    '{
      workspace: $workspace,
      scanned_at: $scanned_at,
      verdict: $verdict,
      artifact: {
        path: $artifact_path,
        exists: $artifact_exists,
        size_bytes: $artifact_size,
        has_challenge_token: $artifact_has_token
      },
      challenge_token: (if $token_value == "" then null else $token_value end),
      logs: {
        stdout_log: { path: $stdout_log, exists: ($stdout_log | . != "" and (test("/"))) },
        stderr_log: { path: $stderr_log, exists: ($stderr_log | . != "" and (test("/"))) },
        tmux_scrollback: { path: $scrollback },
        resolved_prompt: { path: $resolved_prompt }
      },
      escalation_report: { path: $escalation_path },
      auto_respond_counts: { path: $counts_path },
      billing_snapshots: $snapshots
    }
    | .logs.stdout_log.exists      = (.logs.stdout_log.path | test("/"))
    | .logs.stderr_log.exists      = (.logs.stderr_log.path | test("/"))
    '

  return 0
}

# Lightweight existence-checker (used by the JSON above)
# (the embedded jq filter uses test("/") as a placeholder; we patch
# the actual file-exists flags here via shell test)
bridge_report_with_exists() {
  local workspace="$1"
  local artifact_name="${2:-artifact.md}"
  local raw
  raw=$(bridge_report_emit "$workspace" "$artifact_name")
  # Patch file-exists flags by re-checking on disk (jq can't stat)
  local stdout_log="$workspace/stdout.log"
  local stderr_log="$workspace/stderr.log"
  local scrollback="$workspace/tmux-final-scrollback.txt"
  local resolved_prompt="$workspace/resolved-prompt.txt"
  local escalation="$workspace/escalation-report.json"
  local counts="$workspace/auto-respond-counts.csv"

  echo "$raw" | jq \
    --argjson stdout_exists "$([[ -f "$stdout_log" ]] && echo true || echo false)" \
    --argjson stderr_exists "$([[ -f "$stderr_log" ]] && echo true || echo false)" \
    --argjson scrollback_exists "$([[ -f "$scrollback" ]] && echo true || echo false)" \
    --argjson resolved_prompt_exists "$([[ -f "$resolved_prompt" ]] && echo true || echo false)" \
    --argjson escalation_exists "$([[ -f "$escalation" ]] && echo true || echo false)" \
    --argjson counts_exists "$([[ -f "$counts" ]] && echo true || echo false)" \
    --argjson stdout_size "$([[ -f "$stdout_log" ]] && wc -c < "$stdout_log" | tr -d ' ' || echo 0)" \
    --argjson stderr_size "$([[ -f "$stderr_log" ]] && wc -c < "$stderr_log" | tr -d ' ' || echo 0)" \
    --argjson scrollback_size "$([[ -f "$scrollback" ]] && wc -c < "$scrollback" | tr -d ' ' || echo 0)" \
    '.logs.stdout_log.exists = $stdout_exists
     | .logs.stdout_log.size_bytes = $stdout_size
     | .logs.stderr_log.exists = $stderr_exists
     | .logs.stderr_log.size_bytes = $stderr_size
     | .logs.tmux_scrollback.exists = $scrollback_exists
     | .logs.tmux_scrollback.size_bytes = $scrollback_size
     | .logs.resolved_prompt.exists = $resolved_prompt_exists
     | .escalation_report.exists = $escalation_exists
     | .auto_respond_counts.exists = $counts_exists'
}
