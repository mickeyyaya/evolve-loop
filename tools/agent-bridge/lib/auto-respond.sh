#!/usr/bin/env bash
# lib/auto-respond.sh — fallback prompt-detection for interactive REPLs
#
# Design (per user note 2026-05-21):
#   --dangerously-skip-permissions remains the DEFAULT permission strategy.
#   Auto-respond is a FALLBACK safety net for prompts that escape the bypass:
#   auth-recheck, rate-limit, model-deprecation, terminal-resize, etc.
#
# response_keys encoding: comma-separated tmux key names.
#   "y,Enter"     → tmux send-keys y Enter
#   "Enter"       → tmux send-keys Enter
#   "3,Enter"     → tmux send-keys 3 Enter (decline + tell-claude)
#   null/missing  → policy must be "escalate" (no keys to send)
#
# Two layers:
#   auto_respond_decide  — PURE decision function (testable; no tmux).
#                          Takes pane content + manifest path + workspace.
#                          Emits an action token on stdout; rc is action class.
#   auto_respond_tick    — EFFECTFUL wrapper. Calls tmux capture-pane + decide
#                          + send-keys + escalation-report write.
#
# Action tokens / return codes:
#   "noop"                rc=0   — nothing matched; caller continues loop
#   "send:<csv-keys>"     rc=1   — caller should tmux send-keys
#   "escalate:<name>"     rc=85  — known pattern with policy=escalate
#   "loop_guard:<name>"   rc=86  — same pattern matched >5× this session
#
# Bash 3.2-safe: no assoc arrays. Counts kept in workspace/auto-respond-counts.csv.

# ---- PURE DECISION FUNCTION ----
auto_respond_decide() {
  local pane="$1"
  local manifest_path="$2"
  local workspace="$3"

  if [[ ! -f "$manifest_path" ]]; then
    echo "noop"
    return 0
  fi

  mkdir -p "$workspace"
  local counts_file="$workspace/auto-respond-counts.csv"
  [[ ! -f "$counts_file" ]] && : > "$counts_file"

  local matched_name="" matched_keys="" matched_policy=""
  local entry_count
  entry_count=$(jq -r '.interactive_prompts // [] | length' "$manifest_path" 2>/dev/null || echo 0)
  local i=0
  while [[ $i -lt $entry_count ]]; do
    local name regex keys policy
    name=$(jq -r ".interactive_prompts[$i].name" "$manifest_path")
    regex=$(jq -r ".interactive_prompts[$i].regex" "$manifest_path")
    keys=$(jq -r ".interactive_prompts[$i].response_keys // \"\"" "$manifest_path")
    policy=$(jq -r ".interactive_prompts[$i].policy" "$manifest_path")
    i=$((i + 1))

    if [[ -z "$regex" || "$regex" == "null" ]]; then
      continue
    fi
    if echo "$pane" | grep -Eq "$regex"; then
      matched_name="$name"
      matched_keys="$keys"
      matched_policy="$policy"
      break
    fi
  done

  if [[ -z "$matched_name" ]]; then
    echo "noop"
    return 0
  fi

  # Increment match count
  local current_count=0
  if grep -q "^${matched_name}," "$counts_file" 2>/dev/null; then
    current_count=$(awk -F, -v n="$matched_name" '$1==n{print $2}' "$counts_file")
    local tmp="${counts_file}.tmp.$$"
    awk -F, -v n="$matched_name" -v c="$((current_count + 1))" \
      'BEGIN{OFS=","} {if($1==n){$2=c}; print}' "$counts_file" > "$tmp"
    mv "$tmp" "$counts_file"
    current_count=$((current_count + 1))
  else
    echo "${matched_name},1" >> "$counts_file"
    current_count=1
  fi

  if [[ $current_count -gt 5 ]]; then
    echo "loop_guard:$matched_name"
    return 86
  fi

  case "$matched_policy" in
    auto_respond)
      if [[ -z "$matched_keys" || "$matched_keys" == "null" ]]; then
        echo "escalate:$matched_name"
        return 85
      fi
      echo "send:$matched_keys"
      return 1
      ;;
    escalate|*)
      echo "escalate:$matched_name"
      return 85
      ;;
  esac
}

# ---- EFFECTFUL WRAPPER ----
auto_respond_tick() {
  local session="$1"
  local pane
  pane=$(tmux capture-pane -p -t "$session" 2>/dev/null || echo "")

  local action
  action=$(auto_respond_decide "$pane" "$bridge_manifest_path" "$workspace")
  local rc=$?

  case "$rc" in
    0)
      return 0
      ;;
    1)
      local keys_csv="${action#send:}"
      # Reading-pause + key delivery. When human-input mode is opt-in active,
      # use human_send_keys_csv (Gaussian inter-key delays); else fall back
      # to instant bulk send-keys.
      if [[ "$(bridge_human_active 2>/dev/null || echo 0)" == "1" ]]; then
        # F4: scope the reading-pause to the last 5 lines of the pane
        # (the relevant prompt area), not the entire scrollback. Long panes
        # would otherwise produce 30+ second pauses for a quick auto-respond.
        local scoped_pane
        scoped_pane=$(echo "$pane" | tail -5)
        human_reading_pause "$scoped_pane"
        human_send_keys_csv "$session" "$keys_csv"
      else
        local saved_ifs="$IFS"
        IFS=','
        local key_arr=()
        read -ra key_arr <<<"$keys_csv"
        IFS="$saved_ifs"
        if [[ ${#key_arr[@]} -gt 0 ]]; then
          tmux send-keys -t "$session" "${key_arr[@]}"
        fi
      fi
      echo "[auto-respond] sent keys: $keys_csv" >&2
      return 1
      ;;
    85)
      auto_respond_write_escalation_report "$workspace" "$pane" "${action#escalate:}" "$session"
      return 85
      ;;
    86)
      auto_respond_write_escalation_report "$workspace" "$pane" "${action#loop_guard:}" "$session" "loop_guard"
      return 86
      ;;
  esac
}

# ---- ESCALATION REPORT WRITER ----
auto_respond_write_escalation_report() {
  local workspace="$1"
  local pane="$2"
  local pattern_name="$3"
  local session="${4:-}"
  local reason="${5:-escalate}"
  local report="$workspace/escalation-report.json"

  local pane_tail
  pane_tail=$(echo "$pane" | tail -30)

  jq -n \
    --arg captured_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --arg cli "${bridge_manifest_cli:-unknown}" \
    --arg pattern_name "$pattern_name" \
    --arg reason "$reason" \
    --arg pane_tail "$pane_tail" \
    --arg session "$session" \
    '{
      schema_version: 1,
      captured_at: $captured_at,
      cli: $cli,
      pattern_name: $pattern_name,
      reason: $reason,
      session: $session,
      pane_tail: $pane_tail,
      suggested_rule_template: {
        regex: "<<operator: paste the matching substring here>>",
        response_keys: "<<operator: comma-separated tmux key names, e.g. y,Enter>>",
        policy: "auto_respond | escalate",
        note: "<<why this rule is needed>>"
      },
      next_steps: [
        "Read pane_tail above; identify the prompt the agent is stuck on",
        "Run: bridge add-rule --escalation=<this-file> --regex=R --response=KEYS",
        "Re-run the workflow; bridge should now auto-respond to this prompt"
      ]
    }' > "$report"

  echo "[auto-respond] escalation report written: $report (pattern=$pattern_name reason=$reason)" >&2
}

# Standalone debug
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  if [[ $# -ne 2 ]]; then
    echo "usage: $0 MANIFEST_PATH WORKSPACE_DIR  (pane content on stdin)" >&2
    exit 10
  fi
  pane=$(cat)
  action=$(auto_respond_decide "$pane" "$1" "$2")
  rc=$?
  printf 'action=%s rc=%d\n' "$action" "$rc"
  exit "$rc"
fi
