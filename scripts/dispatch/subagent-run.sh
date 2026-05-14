#!/usr/bin/env bash
#
# subagent-run.sh — Single entry point for invoking phase agents as isolated
# subprocesses with least-privilege CLI permission profiles.
#
# This is the only script in the codebase allowed to construct subprocess
# invocations of an AI CLI for evolve-loop phase agents. Phase docs MUST call
# this script; they MUST NOT invoke `claude -p` (or `gemini`, `codex`, etc.)
# directly.
#
# Usage:
#   bash scripts/dispatch/subagent-run.sh <agent> <cycle> <workspace_path> [--prompt-file PATH]
#   bash scripts/dispatch/subagent-run.sh --validate-profile <agent>
#   bash scripts/dispatch/subagent-run.sh --check-token <artifact_path> <token>
#   bash scripts/dispatch/subagent-run.sh --check-ctx-advisory <profile_json> <tokens>
#
# Arguments:
#   <agent>           — one of: intent, scout, tdd-engineer, builder, auditor, inspirer, evaluator, retrospective, orchestrator, plan-reviewer, triage, memo, tester
#   <cycle>           — current cycle number (integer)
#   <workspace_path>  — absolute path to .evolve/runs/cycle-N/ for this cycle
#
# Optional env vars:
#   PROMPT_FILE_OVERRIDE  — read prompt from this file instead of stdin
#   WORKTREE_PATH         — required when agent profile uses {worktree_path}
#   MODEL_TIER_HINT       — override model tier resolution (haiku/sonnet/opus)
#   LEGACY_AGENT_DISPATCH — when set to 1, prints fallback instructions and exits 0
#                            (orchestrator may then use in-process Agent tool;
#                            documented escape hatch for one A/B cycle only)
#
# Exit codes:
#   0  — agent completed, artifact valid, ledger entry written
#   1  — runtime failure (missing binary, profile invalid, CLI non-zero)
#   2  — integrity failure (artifact missing/stale/forged, token mismatch)
#  99  — provider not yet supported (from gemini.sh / codex.sh stubs)
# 127  — required binary not found (claude, jq)

set -euo pipefail

# v8.18.0: dual-root resolution. Profiles & adapters are immutable plugin
# resources; ledger and per-cycle artifacts are writable project state.
__rr_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$__rr_self/../lifecycle/resolve-roots.sh"

PROFILES_DIR="${EVOLVE_PROFILES_DIR_OVERRIDE:-$EVOLVE_PLUGIN_ROOT/.evolve/profiles}"
ADAPTERS_DIR="${EVOLVE_ADAPTERS_DIR_OVERRIDE:-$EVOLVE_PLUGIN_ROOT/scripts/cli_adapters}"
LEDGER="${EVOLVE_LEDGER_OVERRIDE:-$EVOLVE_PROJECT_ROOT/.evolve/ledger.jsonl}"

# REAL_ADAPTERS_DIR always points to the cli_adapters dir alongside this script.
# Derived from BASH_SOURCE[0] so it follows the actual script being executed,
# not EVOLVE_PLUGIN_ROOT (which may point to an older installed copy during dev).
# Used for capability manifest lookups (*.capabilities.json) which must reflect
# the real installed capabilities, not a test-seam sentinel dir.
REAL_ADAPTERS_DIR="$__rr_self/../cli_adapters"
unset __rr_self

# Backwards-compat: many helper functions below still reference $REPO_ROOT.
# Map it to PROJECT_ROOT (the writable side) for those callsites — read-only
# resources (profiles, adapters, sibling scripts) explicitly use PLUGIN_ROOT.
REPO_ROOT="$EVOLVE_PROJECT_ROOT"

# v8.16.2: explicitly export runtime knobs so they reach the adapter through
# any nested bash/sandbox-exec layer. Belt-and-suspenders for env propagation.
[ -n "${EVOLVE_SANDBOX_FALLBACK_ON_EPERM:-}" ] && export EVOLVE_SANDBOX_FALLBACK_ON_EPERM

log() { echo "[subagent-run] $*" >&2; }
fail() { log "FAIL: $*"; exit 1; }
integrity_fail() { log "INTEGRITY-FAIL: $*"; exit 2; }

# abnormal-events.jsonl schema: {event_type, timestamp, source_phase, severity, details, remediation_hint}
# Append a structured event to workspace/abnormal-events.jsonl (best-effort, never fails the pipeline).
_append_abnormal_event() {
    local _ws="$1" _et="$2" _sev="$3" _det="$4" _rem="$5"
    [ -d "$_ws" ] || return 0
    local _ts; _ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    local _det_esc; _det_esc=$(printf '%s' "$_det" | sed 's/"/\\"/g')
    local _rem_esc; _rem_esc=$(printf '%s' "$_rem" | sed 's/"/\\"/g')
    printf '{"event_type":"%s","timestamp":"%s","source_phase":"subagent-run","severity":"%s","details":"%s","remediation_hint":"%s"}\n' \
        "$_et" "$_ts" "$_sev" "$_det_esc" "$_rem_esc" >> "$_ws/abnormal-events.jsonl" 2>/dev/null || true
}

# v9.1.0 Cycle 3: classify the current failure as quota-likely if it matches
# the Claude Code subscription quota-exhaustion signature.
#
# Inputs:
#   $1 — stderr tail (last 5 lines from the failed claude -p invocation)
#   $2 — current cycle number (for cost lookup)
#
# Heuristics (all must hold):
#   - stderr tail is empty/blank or contains only whitespace
#   - cumulative batch cost is ≥ EVOLVE_QUOTA_DANGER_PCT (default 80) of
#     EVOLVE_BATCH_BUDGET_CAP (default 20.00). The reasoning: random rc=1
#     errors happen anywhere in a batch, but quota-exhaustion correlates
#     with substantial cost already consumed.
#
# Returns exit 0 (true) if quota-likely, exit 1 otherwise.
#
# Operator overrides:
#   - EVOLVE_QUOTA_DANGER_PCT=0 forces every empty-stderr rc=1 to classify
#     as quota-likely (useful when running under low budgets where the
#     cost-correlation heuristic doesn't apply).
#   - EVOLVE_QUOTA_DANGER_PCT=100 disables the heuristic (only the explicit
#     `EVOLVE_CHECKPOINT_TRIGGERED=1` operator signal can checkpoint).
_quota_likely() {
    local stderr_tail="$1" cycle="$2"
    local danger_pct="${EVOLVE_QUOTA_DANGER_PCT:-80}"
    local cap="${EVOLVE_BATCH_BUDGET_CAP:-20.00}"

    # Heuristic 1: stderr must be empty/blank. Real errors have content.
    local stripped
    stripped=$(echo "$stderr_tail" | tr -d '[:space:]')
    if [ -n "$stripped" ] && [ "$stderr_tail" != "<empty>" ]; then
        return 1
    fi
    unset stripped

    # Heuristic 2: cost-correlation. Read the cycle's cost-so-far.
    if ! command -v bc >/dev/null 2>&1; then
        # No bc → can't do the correlation check; conservatively return false.
        return 1
    fi
    local scc="$EVOLVE_PLUGIN_ROOT/scripts/observability/show-cycle-cost.sh"
    [ -x "$scc" ] || scc="$EVOLVE_PROJECT_ROOT/scripts/observability/show-cycle-cost.sh"
    if [ ! -x "$scc" ]; then
        return 1
    fi
    local cost_json
    cost_json=$(RUNS_DIR_OVERRIDE="${RUNS_DIR_OVERRIDE:-}" bash "$scc" "$cycle" --json 2>/dev/null || echo "")
    [ -z "$cost_json" ] && return 1
    local cost
    cost=$(echo "$cost_json" | jq -r '.total.cost_usd // 0' 2>/dev/null || echo "0")
    local threshold
    threshold=$(echo "scale=2; $cap * $danger_pct / 100" | bc -l 2>/dev/null || echo "0")
    if [ "$(echo "$cost >= $threshold" | bc -l 2>/dev/null)" = "1" ]; then
        log "  quota-classify: cost=\$$cost >= threshold=\$$threshold (danger_pct=$danger_pct%)"
        return 0
    fi
    return 1
}

# --- Helpers -----------------------------------------------------------------

require_bin() {
    command -v "$1" >/dev/null 2>&1 || { log "missing required binary: $1"; exit 127; }
}

resolve_artifact_path() {
    # Substitute {cycle} placeholder.
    local raw="$1"
    local cycle="$2"
    echo "${raw//\{cycle\}/$cycle}"
}

resolve_model_tier() {
    # Resolve model tier per profile rules. Hint overrides everything.
    local profile_path="$1"
    local cycle="$2"
    if [ -n "${MODEL_TIER_HINT:-}" ]; then
        echo "$MODEL_TIER_HINT"
        return
    fi
    # v8.35.0: adaptive auditor model selection. The auditor profile defaults
    # to opus, but trivial diffs (≤3 files, ≤100 lines, no security paths)
    # are well within Sonnet's reasoning capacity. Auto-downgrading saves
    # ~$1.89/cycle on routine cycles. Operators can force opus by setting
    # MODEL_TIER_HINT=opus or EVOLVE_AUDITOR_TIER_OVERRIDE=opus.
    #
    # Only applies to the auditor agent. All other agents use profile default.
    # When EVOLVE_DIFF_COMPLEXITY_DISABLE=1 is set, fall through to the
    # profile default (kill switch for paranoid runs / CI sweeps).
    local agent_role
    agent_role=$(jq -r '.role // .name // ""' "$profile_path")
    if [ "$agent_role" = "auditor" ] && [ "${EVOLVE_DIFF_COMPLEXITY_DISABLE:-0}" != "1" ]; then
        if [ -n "${EVOLVE_AUDITOR_TIER_OVERRIDE:-}" ]; then
            echo "$EVOLVE_AUDITOR_TIER_OVERRIDE"
            return
        fi
        # Plugin-install dual-root: diff-complexity.sh lives next to this
        # script, not under the user's project. Use script-relative lookup.
        local diff_complexity_script
        diff_complexity_script="$(dirname "${BASH_SOURCE[0]}")/../utility/diff-complexity.sh"
        if [ -x "$diff_complexity_script" ] && command -v jq >/dev/null 2>&1; then
            local tier
            # Use --cached against the worktree if WORKTREE_PATH is set
            # (Builder's per-cycle worktree); otherwise fall back to HEAD diff
            # in the current dir. The diff complexity computation tolerates
            # missing/empty diffs gracefully.
            tier=$(cd "${WORKTREE_PATH:-$REPO_ROOT}" 2>/dev/null && \
                bash "$diff_complexity_script" 2>/dev/null | jq -r '.tier // "complex"' 2>/dev/null \
                || echo "complex")
            case "$tier" in
                trivial)
                    # Cheap, fast model — Sonnet covers ≤3 files / ≤100 lines easily.
                    echo "sonnet"; return ;;
                standard|complex|*)
                    # Fall through to profile default (opus for auditor).
                    : ;;
            esac
        fi
    fi
    # Default tier for now; per-cycle override logic can be added here as the
    # orchestrator passes more state. Profile-encoded overrides are advisory
    # documentation of intent — actual selection lives in the orchestrator
    # which calls this script with MODEL_TIER_HINT set.
    jq -r '.model_tier_default' "$profile_path"
}

generate_challenge_token() {
    # 16 hex chars. Sufficient entropy and short enough to embed in markdown.
    openssl rand -hex 8 2>/dev/null || head -c 16 /dev/urandom | xxd -p | tr -d '\n' | head -c 16
}

capture_git_state() {
    # Returns "<HEAD-sha>:<tree-diff-sha>" describing the repo at this moment.
    # tree-diff-sha = SHA256 of `git diff HEAD` output, capturing all uncommitted
    # (staged + unstaged) modifications. Two repo states with the same HEAD and
    # same tree-diff-sha are byte-for-byte equivalent for the purpose of
    # "what code did the auditor see?". v8.13.0 cycle-binding (H2 from feat
    # branch). Used by ship.sh to refuse shipping when current state diverges
    # from what was audited.
    local head_sha tree_diff_sha
    head_sha=$(git rev-parse HEAD 2>/dev/null || echo "unknown")
    if command -v sha256sum >/dev/null 2>&1; then
        tree_diff_sha=$(git diff HEAD 2>/dev/null | sha256sum | awk '{print $1}')
    else
        tree_diff_sha=$(git diff HEAD 2>/dev/null | shasum -a 256 | awk '{print $1}')
    fi
    echo "${head_sha}:${tree_diff_sha}"
}

# v8.23.0 Task C: build a deterministic cache-prefix file shared across sibling
# fan-out workers in the same batch. The prefix contains: a system framing block,
# the cycle goal (extracted from orchestrator-prompt or scout-report when present),
# and a condensed cycle-state summary. Same cycle+workspace produces byte-identical
# bytes — no timestamps, no randomness — so Anthropic's prompt cache (≥1024 token
# threshold, 5-min TTL) is hit by every sibling after the first.
_write_cache_prefix() {
    local cycle="$1" agent="$2" workspace="$3" out_path="$4"
    local cycle_state_file="$EVOLVE_PROJECT_ROOT/.evolve/cycle-state.json"
    local goal_text="(no goal extracted)"
    local cs_summary="(cycle-state unavailable)"

    # Extract goal from orchestrator-prompt.md if present (the prompt block has
    # a `goal: ...` line written by run-cycle.sh:build_context).
    if [ -f "$workspace/orchestrator-prompt.md" ]; then
        local extracted
        extracted=$(grep -m1 -E '^goal:[[:space:]]*' "$workspace/orchestrator-prompt.md" 2>/dev/null | sed -E 's/^goal:[[:space:]]*//')
        [ -n "$extracted" ] && goal_text="$extracted"
    fi

    # Condense cycle-state into a 2-line summary.
    if [ -f "$cycle_state_file" ] && command -v jq >/dev/null 2>&1; then
        cs_summary=$(jq -r '"phase=" + (.phase // "unknown") + " active_agent=" + (.active_agent // "none") + " completed_phases=[" + ((.completed_phases // []) | join(",")) + "]"' "$cycle_state_file" 2>/dev/null || echo "(cycle-state parse failed)")
    fi

    # Write the prefix. Format: a markdown frame the LLM can ignore if
    # irrelevant. Minimum ~30 lines = ~300+ tokens, well under the 1024 threshold
    # but still substantial. For richer caching, future revs can pad with cycle
    # context. Today's primary win is byte-identical sibling sharing.
    {
        printf '<!-- cache-prefix v8.23.0 — shared across sibling fan-out workers -->\n'
        printf '<!-- agent=%s cycle=%s workspace=%s -->\n\n' "$agent" "$cycle" "$workspace"
        printf '# Shared Context for Cycle %s — %s phase\n\n' "$cycle" "$agent"
        printf '## Cycle Goal\n\n%s\n\n' "$goal_text"
        printf '## Cycle-State Summary\n\n%s\n\n' "$cs_summary"
        printf '## Trust Boundary Reminders\n\n'
        # printf can mistake leading `-` for an option flag; use `%s\n` to bypass.
        printf '%s\n' '- Personas cannot spawn personas (Claude Code structural enforcement)'
        printf '%s\n' '- Builder is excluded from fan-out (single-writer-per-worktree invariant)'
        printf '%s\n' '- Aggregate artifact is the only thing phase-gate validates'
        printf '%s\n' '- Worker artifacts are written under the workspace, not into source tree'
        printf '%s\n\n' '- Each fan-out worker is independent — no cross-worker writes'
        printf '## Output Format\n\n'
        printf 'Write your worker artifact to the path passed in $EVOLVE_FANOUT_WORKER_ARTIFACT\n'
        printf 'or the standard $WORKSPACE/workers/<agent>-<worker>.md location. The aggregator\n'
        printf 'will merge sibling worker outputs into the canonical phase artifact.\n\n'
        printf '<!-- end cache-prefix -->\n'
    } > "$out_path"
}

# v8.37.0: tamper-evident ledger hash-chain helpers.
#
# Each new ledger entry's prev_hash is the SHA256 of the previous entry's
# full JSON line. Modifying any historical entry breaks the chain at the
# next entry, detectable by scripts/observability/verify-ledger-chain.sh. After write,
# the new entry's SHA256 is recorded to .evolve/ledger.tip atomically;
# the tip detects truncation that the chain alone cannot catch.
#
# Pipeline impact: zero. Both fields are additive; existing readers ignore
# them via jq's `// empty` pattern. Pre-v8.37 entries (no prev_hash) are
# tolerated by the verifier as a soft-start boundary.

_ledger_sha256_stdin() {
    if command -v sha256sum >/dev/null 2>&1; then sha256sum | awk '{print $1}';
    else shasum -a 256 | awk '{print $1}'; fi
}

# Compute (prev_hash, entry_seq) for the next ledger entry.
# Echoes "PREV_HASH ENTRY_SEQ" as space-separated tokens.
_ledger_chain_link() {
    local prev_hash="0000000000000000000000000000000000000000000000000000000000000000"
    local entry_seq=0
    if [ -f "$LEDGER" ] && [ -s "$LEDGER" ]; then
        local last_line
        last_line=$(tail -1 "$LEDGER" 2>/dev/null || echo "")
        if [ -n "$last_line" ]; then
            prev_hash=$(printf '%s' "$last_line" | _ledger_sha256_stdin)
        fi
        # entry_seq = current line count (next 0-indexed position).
        # Bash 3.2 portable. Tolerates pre-v8.37 entries (counted unfielded).
        entry_seq=$(wc -l < "$LEDGER" 2>/dev/null | tr -d ' ' || echo 0)
        [ -z "$entry_seq" ] && entry_seq=0
    fi
    printf '%s %s\n' "$prev_hash" "$entry_seq"
}

# Update .evolve/ledger.tip atomically with seq:sha256 of the latest entry.
_ledger_update_tip() {
    local seq="$1" sha="$2"
    local tip_file
    tip_file="$(dirname "$LEDGER")/ledger.tip"
    local tmp="${tip_file}.tmp.$$"
    printf '%s:%s\n' "$seq" "$sha" > "$tmp" 2>/dev/null \
        && mv -f "$tmp" "$tip_file" 2>/dev/null \
        || rm -f "$tmp" 2>/dev/null
}

write_ledger_entry() {
    local cycle="$1" agent="$2" model="$3" exit_code="$4" duration_s="$5"
    local artifact_path="$6" challenge_token="$7" git_state="${8:-unknown:unknown}"
    # v8.51.0: quality_tier (9th arg) is the resolved capability tier of the
    # adapter that produced this entry — `full` / `hybrid` / `degraded` / `none`.
    # Backward-compatible: defaults to "unknown" so pre-v8.51 entries (and tests
    # that call with 8 args) keep working. Existing readers ignore unknown fields.
    local quality_tier="${9:-unknown}"
    # v10.X ADR-1: cli_resolution (10th arg) — JSON object string or empty.
    # Backward-compatible: defaults to "" so existing call sites (8 or 9 args) work.
    local cli_resolution="${10:-}"
    local artifact_sha=""
    if [ -f "$artifact_path" ]; then
        if command -v sha256sum >/dev/null 2>&1; then
            artifact_sha=$(sha256sum "$artifact_path" | awk '{print $1}')
        else
            artifact_sha=$(shasum -a 256 "$artifact_path" | awk '{print $1}')
        fi
    fi
    local git_head="${git_state%%:*}"
    local tree_state_sha="${git_state##*:}"
    local ts
    ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    # v8.37.0: tamper-evident hash chain. Compute prev_hash + entry_seq
    # from the existing ledger BEFORE writing the new entry.
    local prev_hash entry_seq chain_link
    chain_link=$(_ledger_chain_link)
    prev_hash="${chain_link%% *}"
    entry_seq="${chain_link##* }"
    # CYCLE-BINDING (v8.13.0): git_head + tree_state_sha pin the audit to the
    # exact code state at audit time. ship.sh requires the current state to
    # match these — preventing "audit cycle 50, ship cycle 51" exploits.
    local new_line
    new_line=$(jq -nc \
        --arg ts "$ts" \
        --argjson cycle "$cycle" \
        --arg agent "$agent" \
        --arg model "$model" \
        --argjson exit_code "$exit_code" \
        --arg duration_s "$duration_s" \
        --arg artifact_path "$artifact_path" \
        --arg artifact_sha256 "$artifact_sha" \
        --arg challenge_token "$challenge_token" \
        --arg git_head "$git_head" \
        --arg tree_state_sha "$tree_state_sha" \
        --argjson entry_seq "$entry_seq" \
        --arg prev_hash "$prev_hash" \
        --arg quality_tier "$quality_tier" \
        --arg cli_resolution "$cli_resolution" \
        '{ts: $ts, cycle: $cycle, role: $agent, kind: "agent_subprocess",
          model: $model, exit_code: $exit_code, duration_s: $duration_s,
          artifact_path: $artifact_path, artifact_sha256: $artifact_sha256,
          challenge_token: $challenge_token,
          git_head: $git_head, tree_state_sha: $tree_state_sha,
          entry_seq: $entry_seq, prev_hash: $prev_hash,
          quality_tier: $quality_tier,
          cli_resolution: (if ($cli_resolution == null or $cli_resolution == "") then null else ($cli_resolution | fromjson? // null) end)}')
    printf '%s\n' "$new_line" >> "$LEDGER"
    # v8.37.0: update tip with new entry's SHA256.
    local new_sha
    new_sha=$(printf '%s' "$new_line" | _ledger_sha256_stdin)
    _ledger_update_tip "$entry_seq" "$new_sha"
}

verify_artifact() {
    local artifact_path="$1" challenge_token="$2"
    [ -f "$artifact_path" ] || integrity_fail "artifact missing: $artifact_path"
    [ -s "$artifact_path" ] || integrity_fail "artifact empty: $artifact_path"
    # Freshness: must be < 300s old (subagent just wrote it).
    # Token match below is the primary provenance check; age is a sanity guard
    # against truly leftover artifacts from prior failed runs.
    local age
    if [[ "$OSTYPE" == "darwin"* ]]; then
        age=$(( $(date +%s) - $(stat -f %m "$artifact_path") ))
    else
        age=$(( $(date +%s) - $(stat -c %Y "$artifact_path") ))
    fi
    [ "$age" -lt 300 ] || integrity_fail "artifact stale (${age}s old): $artifact_path"
    # Challenge token must appear in artifact.
    if ! grep -q "$challenge_token" "$artifact_path"; then
        integrity_fail "challenge token '$challenge_token' missing from $artifact_path"
    fi
    log "OK: artifact verified ($(wc -l < "$artifact_path") lines, ${age}s old, token present)"
}

# --- Subcommands -------------------------------------------------------------

cmd_validate_profile() {
    local agent="${1:?usage: --validate-profile <agent>}"
    local profile="$PROFILES_DIR/${agent}.json"
    [ -f "$profile" ] || fail "profile not found: $profile"
    require_bin jq
    jq empty "$profile" || fail "profile is not valid JSON: $profile"

    # v10.X ADR-1: LLM router — honor EVOLVE_LLM_CONFIG_PATH in validate path.
    local _vp_cfg="${EVOLVE_LLM_CONFIG_PATH:-${EVOLVE_PROJECT_ROOT}/.evolve/llm_config.json}"
    local _vp_llm_json=""
    local _vp_resolver="$EVOLVE_PLUGIN_ROOT/scripts/dispatch/resolve-llm.sh"
    [ -f "$_vp_resolver" ] || _vp_resolver="$EVOLVE_PROJECT_ROOT/scripts/dispatch/resolve-llm.sh"
    _vp_llm_json=$(bash "$_vp_resolver" "$agent" "$_vp_cfg" 2>/dev/null) || _vp_llm_json=""
    local vp_cli vp_cli_source vp_resolved_model
    if [ -n "$_vp_llm_json" ]; then
        vp_cli=$(printf '%s' "$_vp_llm_json" | jq -r '.cli // empty' 2>/dev/null) || vp_cli=""
        vp_cli_source=$(printf '%s' "$_vp_llm_json" | jq -r '.source // "profile"' 2>/dev/null) || vp_cli_source="profile"
        vp_resolved_model=$(printf '%s' "$_vp_llm_json" | jq -r '.model // empty' 2>/dev/null) || vp_resolved_model=""
        if [ -z "$vp_cli" ] || [ "$vp_cli" = "null" ]; then
            vp_cli=$(jq -r '.cli' "$profile")
            vp_cli_source="profile"
            vp_resolved_model=""
        fi
    else
        vp_cli=$(jq -r '.cli' "$profile")
        vp_cli_source="profile"
        vp_resolved_model=""
    fi
    local adapter="$ADAPTERS_DIR/${vp_cli}.sh"
    [ -x "$adapter" ] || fail "adapter not executable: $adapter"

    local _vp_model
    if [ -n "${vp_resolved_model:-}" ] && [ "$vp_resolved_model" != "null" ]; then
        _vp_model="$vp_resolved_model"
    else
        _vp_model=$(jq -r '.model_tier_default' "$profile")
    fi
    echo "[dispatch-resolve] cli=$vp_cli source=$vp_cli_source model=$_vp_model" >&2
    log "cli_resolution: source=$vp_cli_source target_cli=$vp_cli"

    # v10.X ADR-2: capability WARN emission for validate path.
    local vp_cap_budget_native="true"
    local vp_cap_permission_scoping="true"
    local _vp_cap_manifest="$REAL_ADAPTERS_DIR/${vp_cli}.capabilities.json"
    local _vp_cap_warns=""
    if [ -f "$_vp_cap_manifest" ] && command -v jq >/dev/null 2>&1; then
        vp_cap_budget_native=$(jq -r '.supports.budget_cap_native | if . == null then "true" else tostring end' "$_vp_cap_manifest" 2>/dev/null || echo "true")
        vp_cap_permission_scoping=$(jq -r '.supports.permission_scoping | if . == null then "true" else tostring end' "$_vp_cap_manifest" 2>/dev/null || echo "true")
        if [ "$vp_cap_budget_native" = "false" ]; then
            echo "[adapter-cap] WARN cli=$vp_cli missing=budget_cap_native substitute=wall_clock_timeout" >&2
            _vp_cap_warns="${_vp_cap_warns}cli=$vp_cli missing=budget_cap_native substitute=wall_clock_timeout|"
        fi
        if [ "$vp_cap_permission_scoping" = "false" ]; then
            echo "[adapter-cap] WARN cli=$vp_cli missing=permission_scoping substitute=kernel_role_gate_only" >&2
            _vp_cap_warns="${_vp_cap_warns}cli=$vp_cli missing=permission_scoping substitute=kernel_role_gate_only|"
        fi
    fi

    # v10.X: write dispatch plan log for test seams.
    if [ -n "${EVOLVE_DISPATCH_PLAN_LOG:-}" ]; then
        local _vp_warns_json="[]"
        if [ -n "$_vp_cap_warns" ]; then
            _vp_warns_json="["
            local _vp_first=1
            local _vp_tmpw="$_vp_cap_warns"
            local _vp_we=""
            while [ -n "$_vp_tmpw" ]; do
                _vp_we="${_vp_tmpw%%|*}"
                _vp_tmpw="${_vp_tmpw#*|}"
                if [ -n "$_vp_we" ]; then
                    [ "$_vp_first" = "1" ] && _vp_first=0 || _vp_warns_json="${_vp_warns_json},"
                    _vp_esc=$(printf '%s' "$_vp_we" | sed 's/"/\\"/g')
                    _vp_warns_json="${_vp_warns_json}\"${_vp_esc}\""
                fi
            done
            _vp_warns_json="${_vp_warns_json}]"
        fi
        local _vp_bud="true"; [ "$vp_cap_budget_native" = "false" ] && _vp_bud="false"
        local _vp_perm="true"; [ "$vp_cap_permission_scoping" = "false" ] && _vp_perm="false"
        printf '{"cli":"%s","model":"%s","cli_resolution_source":"%s","cap_budget_native":%s,"cap_permission_scoping":%s,"capability_warns":%s}\n' \
            "$vp_cli" "$_vp_model" "$vp_cli_source" "$_vp_bud" "$_vp_perm" "$_vp_warns_json" \
            > "$EVOLVE_DISPATCH_PLAN_LOG" 2>/dev/null || true
    fi

    # ADR-6: adapter_overrides — read per-adapter tool/flag overrides from profile.
    local vp_ao_tools="" vp_ao_flags=""
    if command -v jq >/dev/null 2>&1; then
        local _vp_ao
        _vp_ao=$(jq -r ".adapter_overrides.\"${vp_cli}\" // empty" "$profile" 2>/dev/null) || _vp_ao=""
        if [ -n "$_vp_ao" ]; then
            vp_ao_tools=$(printf '%s' "$_vp_ao" | jq -r '.tools // empty | if type == "array" then tojson else "" end' 2>/dev/null) || vp_ao_tools=""
            vp_ao_flags=$(printf '%s' "$_vp_ao" | jq -r '.extra_flags // empty | if type == "array" then tojson else "" end' 2>/dev/null) || vp_ao_flags=""
        fi
    fi

    # Print resolved command via VALIDATE_ONLY=1 invocation.
    local tmp_prompt
    tmp_prompt=$(mktemp)
    echo "VALIDATE-ONLY DRY RUN" > "$tmp_prompt"
    PROFILE_PATH="$profile" \
    RESOLVED_MODEL="$_vp_model" \
    PROMPT_FILE="$tmp_prompt" \
    CYCLE="0" \
    WORKSPACE_PATH="$REPO_ROOT/.evolve/runs/cycle-0" \
    WORKTREE_PATH="${WORKTREE_PATH:-$REPO_ROOT}" \
    STDOUT_LOG="/dev/null" \
    STDERR_LOG="/dev/null" \
    ARTIFACT_PATH="$(resolve_artifact_path "$(jq -r '.output_artifact' "$profile")" 0)" \
    RESOLVED_CLI="$vp_cli" \
    CLI_RESOLUTION_SOURCE="$vp_cli_source" \
    CAP_BUDGET_NATIVE="$vp_cap_budget_native" \
    ADAPTER_TOOLS_OVERRIDE="${vp_ao_tools:-}" \
    ADAPTER_EXTRA_FLAGS_OVERRIDE="${vp_ao_flags:-}" \
    VALIDATE_ONLY=1 \
    bash "$adapter"
    local rc=$?
    rm -f "$tmp_prompt"
    [ "$rc" -eq 0 ] || fail "adapter validate-only returned non-zero: $rc"
    log "profile valid: $agent"
}

cmd_check_token() {
    local artifact="${1:?usage: --check-token <artifact> <token>}"
    local token="${2:?usage: --check-token <artifact> <token>}"
    [ -f "$artifact" ] || integrity_fail "artifact missing: $artifact"
    grep -q "$token" "$artifact" || integrity_fail "token absent from $artifact"
    log "OK: token present in $artifact"
}

cmd_check_ctx_advisory() {
    local profile="${1:?usage: --check-ctx-advisory <profile_json> <tokens>}"
    local tokens="${2:?usage: --check-ctx-advisory <profile_json> <tokens>}"
    [ -f "$profile" ] || { echo "[subagent-run] WARN: profile not found: $profile" >&2; exit 0; }
    if command -v jq >/dev/null 2>&1; then
        local _threshold
        _threshold=$(jq -r '.context_clear_trigger_tokens // empty' "$profile" 2>/dev/null || true)
        if [ -n "$_threshold" ] && [ "$tokens" -gt "$_threshold" ]; then
            echo "[subagent-run] INFO: test-agent context at ~${tokens} tokens; profile threshold=${_threshold} (context_clear_trigger_tokens). Agent should apply Tool-Result Hygiene before further tool calls." >&2
        fi
    fi
    exit 0
}

cmd_run() {
    local agent="$1" cycle="$2" workspace="$3"
    # Accept canonical agent names AND fan-out worker names of the form
    # `<role>-worker-<subtask>` (e.g., scout-worker-codebase). For workers,
    # the profile is loaded from <role>.json and the artifact path is
    # overridden to <workspace>/workers/<agent>.md.
    local agent_role="$agent"
    local worker_name=""
    if [[ "$agent" =~ ^([a-z][a-z-]+)-worker-([a-z][a-z0-9-]+)$ ]]; then
        agent_role="${BASH_REMATCH[1]}"
        worker_name="${BASH_REMATCH[2]}"
    fi
    [[ "$agent_role" =~ ^(scout|tdd-engineer|builder|auditor|inspirer|evaluator|retrospective|orchestrator|plan-reviewer|intent|triage|memo|tester)$ ]] || fail "unknown agent: $agent"
    [[ "$cycle" =~ ^[0-9]+$ ]] || fail "cycle must be integer: $cycle"
    [ -d "$workspace" ] || fail "workspace dir does not exist: $workspace"

    # Phase timing instrumentation (bash 3.2 compatible — macOS default shell
    # lacks associative arrays). Each phase boundary records elapsed ms to a
    # temp file as "phase_name <space> ms" lines. Final sidecar writer reads
    # the file. The adapter-invocation phase (which dominates) is the wall-
    # clock window during which `claude -p` runs; subtract from total to get
    # pure runner overhead.
    local timing_log
    timing_log=$(mktemp -t "evolve-timing-XXXXXX") || timing_log=""
    local ts_phase_start_ms
    ts_phase_start_ms=$(($(date +%s%N 2>/dev/null || echo "$(date +%s)000000000")/1000000))
    record_phase() {
        [ -z "$timing_log" ] && return 0
        local phase="$1"
        local ts_now_ms
        ts_now_ms=$(($(date +%s%N 2>/dev/null || echo "$(date +%s)000000000")/1000000))
        local elapsed=$((ts_now_ms - ts_phase_start_ms))
        echo "$phase $elapsed" >> "$timing_log"
        ts_phase_start_ms="$ts_now_ms"
    }

    # Workers borrow the parent role's profile but write to a per-worker artifact.
    local profile="$PROFILES_DIR/${agent_role}.json"
    [ -f "$profile" ] || fail "profile not found: $profile"

    require_bin jq
    jq empty "$profile" || fail "profile is not valid JSON: $profile"
    record_phase profile_load_ms

    # v10.X ADR-1: LLM router — llm_config.json overrides profile cli + model.
    # Resolution: llm_config.phases.<role> > llm_config._fallback > profile.cli
    local _llm_cfg_path="${EVOLVE_LLM_CONFIG_PATH:-${EVOLVE_PROJECT_ROOT}/.evolve/llm_config.json}"
    local _llm_json=""
    local _resolver="$EVOLVE_PLUGIN_ROOT/scripts/dispatch/resolve-llm.sh"
    [ -f "$_resolver" ] || _resolver="$EVOLVE_PROJECT_ROOT/scripts/dispatch/resolve-llm.sh"
    _llm_json=$(bash "$_resolver" "$agent_role" "$_llm_cfg_path" 2>/dev/null) || _llm_json=""
    local cli cli_resolution_source cli_resolved_model
    if [ -n "$_llm_json" ]; then
        cli=$(printf '%s' "$_llm_json" | jq -r '.cli // empty' 2>/dev/null) || cli=""
        cli_resolution_source=$(printf '%s' "$_llm_json" | jq -r '.source // "profile"' 2>/dev/null) || cli_resolution_source="profile"
        cli_resolved_model=$(printf '%s' "$_llm_json" | jq -r '.model // empty' 2>/dev/null) || cli_resolved_model=""
        if [ -z "$cli" ] || [ "$cli" = "null" ]; then
            cli=$(jq -r '.cli' "$profile")
            cli_resolution_source="profile"
            cli_resolved_model=""
        fi
    else
        cli=$(jq -r '.cli' "$profile")
        cli_resolution_source="profile"
        cli_resolved_model=""
    fi
    log "cli_resolution: source=$cli_resolution_source target_cli=$cli"
    local adapter
    adapter="$ADAPTERS_DIR/${cli}.sh"
    [ -x "$adapter" ] || fail "adapter not executable: $adapter"

    # Legacy fallback escape hatch (one A/B cycle only — see CLAUDE.md).
    if [ "${LEGACY_AGENT_DISPATCH:-0}" = "1" ]; then
        log "LEGACY_AGENT_DISPATCH=1 — orchestrator should fall back to in-process Agent tool for $agent"
        log "this fallback is for one A/B cycle only; remove for production"
        echo "LEGACY_DISPATCH"
        exit 1
    fi

    local model
    if [ -n "${cli_resolved_model:-}" ] && [ "$cli_resolved_model" != "null" ]; then
        model="$cli_resolved_model"
        log "model: source=llm_config value=$model (overrides profile model_tier_default)"
    else
        model=$(resolve_model_tier "$profile" "$cycle")
    fi
    local artifact_template artifact_path
    if [ -n "$worker_name" ]; then
        # Workers write to <workspace>/workers/<full-agent>.md regardless of
        # the parent profile's output_artifact template.
        artifact_path="$workspace/workers/${agent}.md"
    else
        artifact_template=$(jq -r '.output_artifact' "$profile")
        artifact_path="$REPO_ROOT/$(resolve_artifact_path "$artifact_template" "$cycle")"
    fi

    mkdir -p "$(dirname "$artifact_path")"

    # Generate challenge token; pass to subagent via prompt prefix.
    local challenge_token
    challenge_token=$(generate_challenge_token)

    # Capture the repo's code state at the moment the agent starts. This is
    # what the agent will see; the ledger pins the audit to this exact state
    # so ship.sh can refuse to ship a different state. v8.13.0 cycle-binding.
    local git_state_at_start
    git_state_at_start=$(capture_git_state)

    # Resolve prompt source: file override > stdin > error.
    local prompt_file
    if [ -n "${PROMPT_FILE_OVERRIDE:-}" ]; then
        [ -f "$PROMPT_FILE_OVERRIDE" ] || fail "PROMPT_FILE_OVERRIDE missing: $PROMPT_FILE_OVERRIDE"
        prompt_file="$PROMPT_FILE_OVERRIDE"
    elif [ ! -t 0 ]; then
        # stdin is piped — capture it
        prompt_file=$(mktemp)
        cat > "$prompt_file"
    else
        fail "no prompt provided: pass via stdin pipe or PROMPT_FILE_OVERRIDE env var"
    fi

    # Build the prompt that the LLM will see. Two ordering modes:
    #
    # v1 (default — EVOLVE_CACHE_PREFIX_V2=0): legacy ordering with dynamic
    # data (challenge_token, artifact_path) injected at the TOP. Backwards
    # compatible; preserved unchanged from v8.60-and-earlier behavior.
    #
    # v2 (opt-in — EVOLVE_CACHE_PREFIX_V2=1): static-first / dynamic-last
    # ordering recommended by Anthropic for prompt-cache reuse. The static
    # bedrock comes from build-invocation-context.sh (byte-identical for
    # same role across runs); dynamic data lives in a small INVOCATION
    # CONTEXT block at the bottom. v8.61.0 Layer 1 (Campaign A — Tier 1).
    local injected_prompt
    injected_prompt=$(mktemp)

    # v9.0.1 SIMP-3: shared task-prompt envelope. Both v1 and v2 paths
    # close the user prompt with the same `--- BEGIN/END TASK PROMPT ---`
    # delimiters around $prompt_file. Defining the envelope as a single
    # helper keeps its format a single source of truth.
    _write_task_envelope() {
        # $1 = injected_prompt path, $2 = prompt_file path
        cat >> "$1" <<EOF
--- BEGIN TASK PROMPT ---
EOF
        cat "$2" >> "$1"
        echo "" >> "$1"
        echo "--- END TASK PROMPT ---" >> "$1"
    }

    if [ "${EVOLVE_CACHE_PREFIX_V2:-1}" = "1" ]; then
        # --- v2 path: bedrock moves to --append-system-prompt at adapter ---
        # Cycle A2 (v8.61.0): the static role bedrock (build-invocation-context.sh)
        # now flows via claude.sh's --append-system-prompt flag. The system
        # prompt slot caches automatically without breakpoint management, and
        # keeping role identity OUT of the user prompt makes the user prompt
        # smaller AND the system prompt deterministic per-role across cycles.
        # Verify the bedrock script exists; the adapter will WARN-only if it
        # actually fails to invoke, but failing fast here surfaces config bugs
        # earlier than runtime.
        local _bic_script="$EVOLVE_PLUGIN_ROOT/scripts/dispatch/build-invocation-context.sh"
        if [ ! -x "$_bic_script" ]; then
            fail "EVOLVE_CACHE_PREFIX_V2=1 but build-invocation-context.sh missing or non-executable at $_bic_script"
        fi
        # The user prompt under v2 is just INVOCATION CONTEXT + task envelope.
        cat > "$injected_prompt" <<EOF
## INVOCATION CONTEXT

- Agent: $agent
- Cycle: $cycle
- Workspace: $workspace
- Artifact path: $artifact_path
- Challenge token: $challenge_token
- Profile: $(basename "$profile")

EOF
        _write_task_envelope "$injected_prompt" "$prompt_file"
    else
        # --- v1 path: legacy ordering (preserved verbatim) ---
        cat > "$injected_prompt" <<EOF
EVOLVE-LOOP SUBAGENT INVOCATION

You are running as the **$agent** subagent for cycle $cycle.

MANDATORY OUTPUT CONTRACT:
- Write your final report to exactly: $artifact_path
- The first line of that file MUST contain the challenge token: $challenge_token
- (Suggested header line: "<!-- challenge-token: $challenge_token -->")
- Reports without the challenge token are rejected as forgeries.

PERMISSION SCOPE:
- Your tool permissions are restricted by profile: $(basename "$profile")
- Attempts to use disallowed tools will fail; do not retry — report the limitation.

WORKSPACE PATH: $workspace
CYCLE: $cycle

EOF

        # Adversarial Auditor framing — default-on for the auditor agent.
        # Counters sycophancy in same-model judges (arxiv 2506.03332). Toggle off
        # with ADVERSARIAL_AUDIT=0 only for deliberately permissive sweeps.
        if [ "$agent" = "auditor" ] && [ "${ADVERSARIAL_AUDIT:-1}" != "0" ]; then
            cat >> "$injected_prompt" <<'ADVEOF'
ADVERSARIAL AUDIT MODE (default-on)

Your role is not to confirm correctness; it is to find a real defect.

Treat the build as guilty until proven innocent. Specifically:
- A "PASS" verdict requires positive evidence that each acceptance criterion is
  met by executable behavior — not by the presence of expected strings in source
  code. Cite the test output, the diff hunk, or the command that demonstrates it.
- Confidence below 0.85 → WARN, not PASS. "I see no problems" is not 0.85
  confidence; it is the absence of evidence, which is the absence of an audit.
- If you have produced ≥5 consecutive PASS verdicts in this loop, the prior is
  now SHIFTED toward latent defects — go deeper than your routine checklist.
- A vague affirmative review is itself a failure. Output `NO_DEFECT_FOUND` with
  explicit per-criterion evidence, OR list at least one concrete defect with
  file:line and a reproduction command.

ADVEOF
        fi

        _write_task_envelope "$injected_prompt" "$prompt_file"
    fi

    # v8.56.0 Layer B: soft prompt-size guard. Kernel-layer check that
    # the orchestrator's prompt assembly didn't blow past the budget.
    # Token estimation uses the 1-token≈4-bytes English upper bound.
    # WARN-only by default; set EVOLVE_PROMPT_BUDGET_ENFORCE=1 to make it
    # a hard exit (operator opt-in). Guard is INFORMATIONAL — it cannot
    # know if the orchestrator legitimately needs a big prompt for this
    # role (Retrospective is the synthesizer; it sees everything).
    #
    # v9.1.0 Cycle 6: extended with autotrim mode (EVOLVE_CONTEXT_AUTOTRIM=1)
    # and per-phase context-monitor.json sidecar.
    local _prompt_max="${EVOLVE_PROMPT_MAX_TOKENS:-30000}"
    local _prompt_bytes
    _prompt_bytes=$(wc -c < "$injected_prompt" | tr -d ' ')
    local _prompt_tokens=$((_prompt_bytes / 4))
    local _cap_pct=0
    if [ "$_prompt_max" -gt 0 ]; then
        _cap_pct=$((_prompt_tokens * 100 / _prompt_max))
    fi

    # v9.1.0 Cycle 6: autotrim mode — when prompt exceeds the cap AND
    # EVOLVE_CONTEXT_AUTOTRIM=1, truncate the prompt aggressively (preserve
    # the head — instructions, role context, intent — and the tail — current
    # task — while dropping the middle, which is typically low-priority
    # ledger entries and instinct summaries). The trim keeps the prompt
    # actionable while bounding token burn.
    if [ "${EVOLVE_CONTEXT_AUTOTRIM:-0}" = "1" ] \
            && [ "$_prompt_tokens" -gt "$_prompt_max" ]; then
        # P-NEW-13: Line-boundary cut — avoids byte-truncation of JSON objects, file paths, function signatures.
        local _total_lines
        _total_lines=$(wc -l < "$injected_prompt" | tr -d ' ')
        local _keep_head_lines=$((_prompt_max * 4 * 60 / 100 / 50))   # 60% from head, ~50 chars/line avg
        local _keep_tail_lines=$((_prompt_max * 4 * 35 / 100 / 50))   # 35% from tail, ~50 chars/line avg
        local _trimmed="${injected_prompt}.trimmed"
        {
            head -n "$_keep_head_lines" "$injected_prompt"
            printf '\n\n[CONTEXT-AUTOTRIM v9.1.0: dropped %d lines of mid-prompt context — original=%d tokens, cap=%d. Set EVOLVE_CONTEXT_AUTOTRIM=0 to disable.]\n\n' \
                "$((_total_lines - _keep_head_lines - _keep_tail_lines))" "$_prompt_tokens" "$_prompt_max"
            tail -n "$_keep_tail_lines" "$injected_prompt"
        } > "$_trimmed"
        mv -f "$_trimmed" "$injected_prompt"
        # Re-measure after trim.
        _prompt_bytes=$(wc -c < "$injected_prompt" | tr -d ' ')
        _prompt_tokens=$((_prompt_bytes / 4))
        _cap_pct=$((_prompt_tokens * 100 / _prompt_max))
        echo "[subagent-run] AUTOTRIM: $agent prompt trimmed to ~$_prompt_tokens tokens (target=$_prompt_max, new cap_pct=${_cap_pct}%)" >&2
        unset _trimmed _keep_head_lines _keep_tail_lines _total_lines
    fi

    if [ "$_prompt_tokens" -gt "$_prompt_max" ]; then
        echo "[subagent-run] WARN: $agent prompt is ~$_prompt_tokens tokens (cap=$_prompt_max). Consider role-context-builder.sh for filtered context, or EVOLVE_CONTEXT_AUTOTRIM=1. Layer-B reference: agents/evolve-orchestrator.md#per-phase-prompt-context" >&2
        if [ "${EVOLVE_PROMPT_BUDGET_ENFORCE:-0}" = "1" ]; then
            fail "EVOLVE_PROMPT_BUDGET_ENFORCE=1 + prompt over cap; aborting"
        fi
    fi

    # v9.1.0 Cycle 6: per-phase context-monitor.json. Records the prompt
    # token estimate + cap percentage so operators can `tail` or `watch`
    # the cumulative cycle context usage during long runs.
    if command -v jq >/dev/null 2>&1; then
        local _monitor="$workspace/context-monitor.json"
        local _now
        _now=$(date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u)
        local _existing='{}'
        [ -f "$_monitor" ] && _existing=$(cat "$_monitor" 2>/dev/null || echo '{}')
        echo "$_existing" | jq -c \
            --arg agent "$agent" \
            --argjson cycle "$cycle" \
            --arg ts "$_now" \
            --argjson tokens "$_prompt_tokens" \
            --argjson cap "$_prompt_max" \
            --argjson cap_pct "$_cap_pct" \
            '. as $existing
             | (.cycle // $cycle) as $c
             | (.phases // {}) as $phases
             | {
                 cycle: $c,
                 lastUpdated: $ts,
                 phases: ($phases + {($agent): {
                     input_tokens: $tokens,
                     cap_tokens: $cap,
                     cap_pct: $cap_pct,
                     measuredAt: $ts
                 }}),
                 cumulative_input_tokens: (
                     ($phases + {($agent): {input_tokens: $tokens}})
                     | to_entries | map(.value.input_tokens // 0) | add // 0
                 ),
                 cumulative_cap: ($cap * 4)
             }
             | . + {
                 cumulative_pct: (
                     if .cumulative_cap > 0
                     then ((.cumulative_input_tokens * 100 / .cumulative_cap) | floor)
                     else 0 end
                 )
             }' > "${_monitor}.tmp.$$" 2>/dev/null \
             && mv -f "${_monitor}.tmp.$$" "$_monitor" \
             || rm -f "${_monitor}.tmp.$$" 2>/dev/null
        unset _monitor _existing _now
    fi
    unset _cap_pct

    local stdout_log="$workspace/${agent}-stdout.log"
    local stderr_log="$workspace/${agent}-stderr.log"

    # NOTE: cycle 8127 RC4 audit empirically demonstrated that an earlier
    # auto-worktree-in-subagent-run.sh attempt had three failures:
    # (1) writes still denied because the profile's relative-path patterns
    #     don't match the auditor's absolute-path resolution under cwd=worktree;
    # (2) `git worktree add --detach HEAD` shows the COMMITTED state at HEAD,
    #     not the orchestrator's uncommitted working tree, so the subagent
    #     audits the wrong code;
    # (3) EPERM still occurred — Claude Code's cleanup is NOT keyed on
    #     project-path-hash alone.
    # That code was reverted. v8.21.0 fixed the issue at the correct
    # architectural layer: run-cycle.sh (privileged shell) provisions the
    # worktree before the orchestrator subprocess starts and records it in
    # cycle-state.json. subagent-run.sh now consumes that path, never creates.

    # v8.21.0: Single source of truth — cycle-state.json. If the env var is
    # already set (manual test, legacy caller), respect it. Otherwise read the
    # canonical path that run-cycle.sh wrote via `cycle-state.sh set-worktree`.
    if [ -z "${WORKTREE_PATH:-}" ]; then
        WORKTREE_PATH=$("$EVOLVE_PLUGIN_ROOT/scripts/lifecycle/cycle-state.sh" get active_worktree 2>/dev/null || true)
        if [ -n "$WORKTREE_PATH" ]; then
            log "WORKTREE_PATH derived from cycle-state.json: $WORKTREE_PATH"
        fi
    fi

    # v8.23.2 BUG-007 fix: pre-expand {worktree_path} placeholders in EVERY string
    # field of the profile before the adapter sees it. Pre-v8.23.2, only claude.sh
    # substituted at two specific sites (add_dir, sandbox.write_subpaths). Other
    # adapters (gemini.sh, codex.sh) and any future tool-permission engine that
    # reads the profile would see the literal "{worktree_path}" token. Cycle 53
    # of a downstream user's project surfaced this when builder Edits returned
    # EPERM despite WORKTREE_PATH being set — Claude Code's tool-permission layer
    # was evidently consulting an unexpanded profile path.
    #
    # The fix: jq's `walk` recurses through every value in the profile JSON; for
    # string values, substitute "{worktree_path}" → "$WORKTREE_PATH". The expanded
    # profile is written to the workspace (per-cycle, traceable artifact), and
    # PROFILE_PATH is updated to point at it. The original profile in
    # .evolve/profiles/ is never modified.
    local effective_profile="$profile"
    local profile_worktree_aware="0"
    if grep -q '{worktree_path}' "$profile" 2>/dev/null; then
        # v8.23.3: ALWAYS mark the profile as worktree-aware (even if expansion
        # fails or WORKTREE_PATH is unset). claude.sh uses this env hint to set
        # WORKING_DIR=WORKTREE_PATH instead of relying on the literal token
        # being present in the profile (v8.23.2 regression: pre-expansion swept
        # the literal out, causing claude.sh's WORKING_DIR detection at line ~165
        # to fall through to PWD, which made the builder cd into the main repo
        # instead of the worktree → builder Edits hit --add-dir EPERM).
        profile_worktree_aware="1"
        if [ -z "${WORKTREE_PATH:-}" ]; then
            log "WARN: profile $profile contains {worktree_path} but WORKTREE_PATH is unset"
            # fall through — adapter will fail loudly with its own check
        else
            local expanded_profile="$workspace/${agent}-profile-expanded.json"
            if jq --arg wp "$WORKTREE_PATH" \
                'walk(if type == "string" then gsub("\\{worktree_path\\}"; $wp) else . end)' \
                "$profile" > "$expanded_profile" 2>/dev/null; then
                effective_profile="$expanded_profile"
                log "profile pre-expanded: $expanded_profile (WORKTREE_PATH=$WORKTREE_PATH)"
            else
                log "WARN: jq walk failed on $profile — falling back to original; adapter must substitute"
            fi
        fi
    fi

    record_phase prep_total_ms

    # c45-P-NEW-6 (cycle 36, corrected cycle 37): per-profile context_clear_trigger_tokens.
    # Must appear AFTER profile expansion (effective_profile is finalized above).
    if command -v jq >/dev/null 2>&1 && [ -f "${effective_profile:-}" ]; then
        local _ctx_clear_threshold
        _ctx_clear_threshold=$(jq -r '.context_clear_trigger_tokens // empty' "$effective_profile" 2>/dev/null || true)
        if [ -n "$_ctx_clear_threshold" ] && [ "$_prompt_tokens" -gt "$_ctx_clear_threshold" ]; then
            echo "[subagent-run] INFO: $agent context at ~${_prompt_tokens} tokens; profile threshold=${_ctx_clear_threshold} (context_clear_trigger_tokens). Agent should apply Tool-Result Hygiene before further tool calls." >&2
        fi
        unset _ctx_clear_threshold
    fi

    # v8.51.0: resolve adapter capability tier. Profile.cli is authoritative
    # (allows multi-LLM-per-phase via profile config). Fall back to "unknown"
    # if the manifest is missing or capability-check fails — pipeline must not
    # block on capability resolution failure.
    local quality_tier="unknown"
    local cap_check="$ADAPTERS_DIR/_capability-check.sh"
    if [ -x "$cap_check" ]; then
        quality_tier=$(bash "$cap_check" "$cli" 2>/dev/null | jq -r '.quality_tier // "unknown"' 2>/dev/null || echo "unknown")
        if [ -z "$quality_tier" ] || [ "$quality_tier" = "null" ]; then
            quality_tier="unknown"
        fi
    fi
    if [ "$quality_tier" = "degraded" ] || [ "$quality_tier" = "none" ]; then
        log "WARN: adapter $cli resolved to quality_tier=$quality_tier — pipeline runs with reduced isolation"
        if [ -x "$cap_check" ]; then
            # Surface specific warnings to the operator
            bash "$cap_check" "$cli" 2>/dev/null | jq -r '.warnings[] // empty' 2>/dev/null | while read -r w; do
                [ -n "$w" ] && log "  $w"
            done
        fi
    fi

    # v10.X ADR-2: structured WARN for degraded capabilities (supports.* booleans).
    # Emits one parseable [adapter-cap] WARN line per missing capability and exports
    # CAP_BUDGET_NATIVE so adapters can gate --max-budget-usd flag on it.
    local cap_budget_native="true"
    local cap_permission_scoping="true"
    local _cap_manifest="$REAL_ADAPTERS_DIR/${cli}.capabilities.json"
    local _cap_warns=""
    if [ -f "$_cap_manifest" ] && command -v jq >/dev/null 2>&1; then
        cap_budget_native=$(jq -r '.supports.budget_cap_native | if . == null then "true" else tostring end' "$_cap_manifest" 2>/dev/null || echo "true")
        cap_permission_scoping=$(jq -r '.supports.permission_scoping | if . == null then "true" else tostring end' "$_cap_manifest" 2>/dev/null || echo "true")
        if [ "$cap_budget_native" = "false" ]; then
            echo "[adapter-cap] WARN cli=$cli missing=budget_cap_native substitute=wall_clock_timeout" >&2
            _cap_warns="${_cap_warns}cli=$cli missing=budget_cap_native substitute=wall_clock_timeout|"
        fi
        if [ "$cap_permission_scoping" = "false" ]; then
            echo "[adapter-cap] WARN cli=$cli missing=permission_scoping substitute=kernel_role_gate_only" >&2
            _cap_warns="${_cap_warns}cli=$cli missing=permission_scoping substitute=kernel_role_gate_only|"
        fi
    fi
    export CAP_BUDGET_NATIVE="$cap_budget_native"

    # ADR-6: adapter_overrides — read per-adapter tool/flag overrides from profile.
    local ao_tools="" ao_flags=""
    if command -v jq >/dev/null 2>&1; then
        local _ao_block
        _ao_block=$(jq -r ".adapter_overrides.\"${cli}\" // empty" "$profile" 2>/dev/null) || _ao_block=""
        if [ -n "$_ao_block" ]; then
            ao_tools=$(printf '%s' "$_ao_block" | jq -r '.tools // empty | if type == "array" then tojson else "" end' 2>/dev/null) || ao_tools=""
            ao_flags=$(printf '%s' "$_ao_block" | jq -r '.extra_flags // empty | if type == "array" then tojson else "" end' 2>/dev/null) || ao_flags=""
        fi
    fi

    # v10.X: write dispatch plan JSON for test seams (EVOLVE_DISPATCH_PLAN_LOG).
    # Format: {cli, model, cli_resolution_source, cap_budget_native, cap_permission_scoping, capability_warns[]}
    if [ -n "${EVOLVE_DISPATCH_PLAN_LOG:-}" ]; then
        local _plan_warns_json="[]"
        if [ -n "$_cap_warns" ]; then
            _plan_warns_json="["
            local _pw_first=1
            local _pw_tmp="$_cap_warns"
            local _pw_entry=""
            while [ -n "$_pw_tmp" ]; do
                _pw_entry="${_pw_tmp%%|*}"
                _pw_tmp="${_pw_tmp#*|}"
                if [ -n "$_pw_entry" ]; then
                    [ "$_pw_first" = "1" ] && _pw_first=0 || _plan_warns_json="${_plan_warns_json},"
                    _pw_esc=$(printf '%s' "$_pw_entry" | sed 's/"/\\"/g')
                    _plan_warns_json="${_plan_warns_json}\"${_pw_esc}\""
                fi
            done
            _plan_warns_json="${_plan_warns_json}]"
        fi
        local _dp_bud="true"; [ "$cap_budget_native" = "false" ] && _dp_bud="false"
        local _dp_perm="true"; [ "$cap_permission_scoping" = "false" ] && _dp_perm="false"
        printf '{"cli":"%s","model":"%s","cli_resolution_source":"%s","cap_budget_native":%s,"cap_permission_scoping":%s,"capability_warns":%s}\n' \
            "$cli" "$model" "$cli_resolution_source" "$_dp_bud" "$_dp_perm" "$_plan_warns_json" \
            > "$EVOLVE_DISPATCH_PLAN_LOG" 2>/dev/null || true
    fi

    # Build cli_resolution JSON once (used at both write_ledger_entry call sites).
    local cli_resolution_json=""
    if command -v jq >/dev/null 2>&1; then
        cli_resolution_json=$(jq -nc \
            --arg source "$cli_resolution_source" \
            --arg target_cli "$cli" \
            --arg model "$model" \
            --arg mode "$quality_tier" \
            '{source: $source, target_cli: $target_cli, model: $model, mode: $mode}') || cli_resolution_json=""
    fi

    log "starting $agent (cycle $cycle, model $model, cli $cli, tier $quality_tier, token $challenge_token)"
    local start_ts
    start_ts=$(date +%s)

    set +e
    # v9.5.0: spawn phase-observer alongside the adapter (per-phase OODA loop).
    # Observer tails $stdout_log, emits structured INFO/WARN/INCIDENT envelopes
    # to {agent}-observer-events.ndjson, and writes {agent}-observer-report.json
    # at phase exit. Gated on EVOLVE_OBSERVER_ENABLED=1 (default OFF in v1).
    # Sibling to phase-watchdog; coexists in v1 (observer informs, watchdog
    # still kills). See docs/architecture/phase-observer.md.
    local OBSERVER_PID=""
    if [ "${EVOLVE_OBSERVER_ENABLED:-0}" = "1" ]; then
        local _observer_cycle_state="${EVOLVE_PROJECT_ROOT:-$PWD}/.evolve/cycle-state.json"
        # v9.5.0: pass --enforce when EVOLVE_OBSERVER_ENFORCE=1; observer then
        # has kill authority on INCIDENT(stuck) or INCIDENT(infinite_loop).
        # Default OFF preserves v9.4.0 advisory-only behavior.
        local _observer_args=()
        [ "${EVOLVE_OBSERVER_ENFORCE:-0}" = "1" ] && _observer_args+=("--enforce")
        # v10.0.0: bash 3.2 + set -u fix — empty array expansion needs the :+ guard.
        bash "$EVOLVE_PLUGIN_ROOT/scripts/dispatch/phase-observer.sh" \
            ${_observer_args[@]:+"${_observer_args[@]}"} \
            "$workspace" "$$" "$cycle" "$agent" "$agent" "$_observer_cycle_state" \
            >/dev/null 2>&1 &
        OBSERVER_PID=$!
        log "phase-observer spawned (pid=$OBSERVER_PID agent=$agent enforce=${EVOLVE_OBSERVER_ENFORCE:-0})"
    fi

    # v8.61.0 Cycle A2: pass AGENT to adapter so claude.sh can emit the
    # role-specific bedrock as --append-system-prompt under v2.
    # v10.X ADR-1/ADR-2: pass cli_resolution vars and capability flags to adapter.
    # ADR-6: pass adapter_overrides tool/flag overrides to adapter.
    PROFILE_PATH="$effective_profile" \
    RESOLVED_MODEL="$model" \
    PROMPT_FILE="$injected_prompt" \
    CYCLE="$cycle" \
    WORKSPACE_PATH="$workspace" \
    WORKTREE_PATH="${WORKTREE_PATH:-}" \
    EVOLVE_PROFILE_WORKTREE_AWARE="$profile_worktree_aware" \
    STDOUT_LOG="$stdout_log" \
    STDERR_LOG="$stderr_log" \
    ARTIFACT_PATH="$artifact_path" \
    AGENT="$agent" \
    RESOLVED_CLI="$cli" \
    CLI_RESOLUTION_SOURCE="$cli_resolution_source" \
    CAP_BUDGET_NATIVE="$cap_budget_native" \
    ADAPTER_TOOLS_OVERRIDE="${ao_tools:-}" \
    ADAPTER_EXTRA_FLAGS_OVERRIDE="${ao_flags:-}" \
    bash "$adapter"
    local cli_exit=$?
    set -e
    record_phase adapter_invoke_ms

    # v9.5.0: signal phase-observer that subagent exited; observer flushes
    # final report and exits gracefully.
    if [ -n "$OBSERVER_PID" ]; then
        kill -USR1 "$OBSERVER_PID" 2>/dev/null || true
        wait "$OBSERVER_PID" 2>/dev/null || true
        log "phase-observer signaled + reaped (pid=$OBSERVER_PID)"
    fi

    local end_ts duration
    end_ts=$(date +%s)
    duration=$((end_ts - start_ts))

    rm -f "$injected_prompt"
    [ -z "${PROMPT_FILE_OVERRIDE:-}" ] && rm -f "$prompt_file"

    if [ "$cli_exit" -eq 99 ]; then
        log "provider $cli returned 99 (not yet supported)"
        exit 99
    fi

    if [ "$cli_exit" -ne 0 ]; then
        log "CLI exited non-zero: $cli_exit"
        local _stderr_tail
        _stderr_tail=$(tail -5 "$stderr_log" 2>/dev/null || echo '<empty>')
        log "stderr tail: $_stderr_tail"
        write_ledger_entry "$cycle" "$agent" "$model" "$cli_exit" "$duration" "$artifact_path" "$challenge_token" "$git_state_at_start" "$quality_tier" "${cli_resolution_json:-}"

        # v9.1.0 Cycle 3: reactive quota-likely classification.
        # Signature for Claude Code subscription quota exhaustion
        # (GitHub issue #29579): rc=1, empty/blank stderr, and the cycle
        # has already done substantial work (cost ≥ 80% of cap).
        #
        # The outer Claude Code process consumes the rate-limit response;
        # the nested `claude -p` subprocess dies silently. We can't query
        # Anthropic's quota directly, but we CAN detect the signature.
        # When it fires we write a checkpoint and signal the EXIT trap to
        # preserve worktree + state for `--resume`.
        if [ "${EVOLVE_CHECKPOINT_DISABLE:-0}" != "1" ] \
                && [ "$cli_exit" = "1" ] \
                && _quota_likely "$_stderr_tail" "$cycle"; then
            log "QUOTA-LIKELY: rc=1 + empty stderr + cost in danger zone — classifying as quota-likely"
            log "  writing checkpoint and signaling EXIT trap to preserve state"
            local _csh="$EVOLVE_PLUGIN_ROOT/scripts/lifecycle/cycle-state.sh"
            [ -f "$_csh" ] || _csh="$EVOLVE_PROJECT_ROOT/scripts/lifecycle/cycle-state.sh"
            # v10.6.0 auto-resume Layer 1+2 glue: compute the estimated quota
            # reset time and export it for cycle-state.sh checkpoint to persist
            # in the checkpoint block. estimate-quota-reset.sh prefers
            # EVOLVE_QUOTA_RESET_AT operator override > parsed hint file in the
            # workspace > now + EVOLVE_QUOTA_RESET_HOURS fallback. Failure is
            # non-fatal — we still write the checkpoint, just without timing.
            local _eqr="$EVOLVE_PLUGIN_ROOT/scripts/dispatch/estimate-quota-reset.sh"
            [ -f "$_eqr" ] || _eqr="$EVOLVE_PROJECT_ROOT/scripts/dispatch/estimate-quota-reset.sh"
            if [ -f "$_eqr" ]; then
                local _eqr_out
                _eqr_out=$(bash "$_eqr" "$workspace" 2>/dev/null) || _eqr_out=""
                if [ -n "$_eqr_out" ]; then
                    EVOLVE_CHECKPOINT_QUOTA_RESET_AT=$(printf '%s\n' "$_eqr_out" | sed -n '1p')
                    EVOLVE_CHECKPOINT_QUOTA_RESET_SOURCE=$(printf '%s\n' "$_eqr_out" | sed -n '2p' | sed 's/^source=//')
                    export EVOLVE_CHECKPOINT_QUOTA_RESET_AT EVOLVE_CHECKPOINT_QUOTA_RESET_SOURCE
                    log "  quota-reset-estimate: at=$EVOLVE_CHECKPOINT_QUOTA_RESET_AT source=$EVOLVE_CHECKPOINT_QUOTA_RESET_SOURCE"
                fi
            fi
            if bash "$_csh" checkpoint quota-likely 2>/dev/null; then
                log "  checkpoint written; resume with: bash scripts/dispatch/evolve-loop-dispatch.sh --resume"
            else
                log "  WARN: checkpoint write failed; setting EVOLVE_CHECKPOINT_TRIGGERED=1 for EXIT trap"
            fi
            # Signal run-cycle.sh's EXIT trap that a checkpoint should be
            # honored even if the explicit write failed (e.g., cycle-state
            # was already cleared by a parallel cleanup).
            export EVOLVE_CHECKPOINT_TRIGGERED=1
        fi

        _append_abnormal_event "$workspace" "dispatch-error" "HIGH" \
            "agent=$agent cycle=$cycle cli_exit=$cli_exit" \
            "Check ${workspace}/${agent}-stderr.log; if quota-likely, resume with --resume flag"
        unset _stderr_tail
        exit 1
    fi

    # Post-run integrity verification.
    verify_artifact "$artifact_path" "$challenge_token"

    # Telemetry sidecar: extract `usage` block from the final JSON line of
    # stdout (claude -p --output-format json emits one JSON object per session)
    # so future Auditor invocations can verify empirical token reduction
    # without parsing their own stdout. Suggested by cycle 8124 audit
    # (LOW-finding: empirical-test-not-instrumentable). Best-effort: jq failure
    # leaves the sidecar absent rather than failing the run.
    local usage_sidecar="$workspace/${agent}-usage.json"
    if command -v jq >/dev/null 2>&1 && [ -s "$stdout_log" ]; then
        # Take the last line containing a "usage" key. claude -p produces a
        # single-line JSON for --output-format json, but be defensive.
        local last_json
        last_json=$(grep -F '"usage"' "$stdout_log" 2>/dev/null | tail -1)
        if [ -n "$last_json" ]; then
            echo "$last_json" | jq -c '{
                duration_ms: .duration_ms,
                num_turns: .num_turns,
                total_cost_usd: .total_cost_usd,
                usage: .usage,
                modelUsage: .modelUsage
            }' > "$usage_sidecar" 2>/dev/null || rm -f "$usage_sidecar"
        fi
    fi

    # T2 / P-NEW-22 observability: turn-overrun detection.
    # Compare num_turns from the usage sidecar against the profile's max_turns.
    # Overruns are appended to abnormal-events.jsonl for dashboard visibility and
    # carryover todo generation — they do NOT fail the cycle (best-effort).
    if command -v jq >/dev/null 2>&1 && [ -s "$usage_sidecar" ]; then
        local _actual_turns _max_turns_profile
        _actual_turns=$(jq -r '.num_turns // 0' "$usage_sidecar" 2>/dev/null || echo "0")
        _max_turns_profile=$(jq -r '.max_turns // 0' "$effective_profile" 2>/dev/null || echo "0")
        if [ "$_max_turns_profile" -gt 0 ] && [ "$_actual_turns" -gt "$_max_turns_profile" ] 2>/dev/null; then
            _append_abnormal_event "$workspace" "turn-overrun" "WARN" \
                "agent=$agent actual_turns=$_actual_turns max_turns=$_max_turns_profile (${_actual_turns}x ceiling)" \
                "Review ${agent}-report.md for scope; split task or tighten STOP CRITERION"
            log "WARN: turn-overrun agent=$agent turns=$_actual_turns max=$_max_turns_profile"
        fi
        unset _actual_turns _max_turns_profile
    fi

    write_ledger_entry "$cycle" "$agent" "$model" 0 "$duration" "$artifact_path" "$challenge_token" "$git_state_at_start" "$quality_tier" "${cli_resolution_json:-}"
    record_phase finalize_ms

    # Phase-timing sidecar: a per-phase ms breakdown that lets us identify
    # which parts of the runner are slow vs the dominant adapter_invoke_ms
    # (the actual claude -p call). Reads the temp file populated by
    # record_phase calls. Total = sum of phase values.
    local timing_sidecar="$workspace/${agent}-timing.json"
    if command -v jq >/dev/null 2>&1 && [ -n "$timing_log" ] && [ -f "$timing_log" ]; then
        local total_ms=0
        local timing_json="{"
        local first=1
        while IFS=' ' read -r phase ms; do
            [ -z "$phase" ] && continue
            [ "$first" = "1" ] && first=0 || timing_json+=","
            timing_json+="\"$phase\":$ms"
            total_ms=$((total_ms + ms))
        done < "$timing_log"
        timing_json+="}"
        echo "$timing_json" | jq --argjson total "$total_ms" \
            --arg agent "$agent" --argjson cycle "$cycle" \
            '. + {agent: $agent, cycle: $cycle, total_ms: $total}' \
            > "$timing_sidecar" 2>/dev/null || rm -f "$timing_sidecar"
        rm -f "$timing_log"
    fi

    # v10.5.0: Phase-B observability — replay NDJSON stdout through
    # tracker-writer and append "Performance & Cost" to the phase report.
    # Gated by EVOLVE_TRACKER_ENABLED (default OFF, opt-in per the
    # verify→default-on ladder documented in CLAUDE.md). Best-effort: any
    # failure is logged but does not fail the cycle. The replay runs AFTER
    # the usage + timing sidecars and the ledger entry are committed, so a
    # tracker fault cannot corrupt the audit-binding chain.
    if [ "${EVOLVE_TRACKER_ENABLED:-0}" = "1" ] && [ -s "$stdout_log" ]; then
        local _inv_id="${agent}-c${cycle}-${challenge_token:0:12}"
        local _tracker_sh="$EVOLVE_PLUGIN_ROOT/scripts/observability/tracker-writer.sh"
        local _perf_sh="$EVOLVE_PLUGIN_ROOT/scripts/observability/append-phase-perf.sh"
        if [ -x "$_tracker_sh" ]; then
            cat "$stdout_log" | bash "$_tracker_sh" \
                --cycle="$cycle" --phase="$agent" --invocation-id="$_inv_id" \
                >/dev/null 2>"$workspace/${agent}-tracker.stderr.log" \
                || log "WARN: tracker-writer replay rc=$? (non-fatal)"
        fi
        if [ -x "$_perf_sh" ] && [ -f "$artifact_path" ]; then
            bash "$_perf_sh" "$cycle" "$agent" \
                >/dev/null 2>"$workspace/${agent}-perf.stderr.log" \
                || log "WARN: append-phase-perf rc=$? (non-fatal)"
        fi
    fi

    log "DONE: $agent cycle $cycle in ${duration}s, artifact at $artifact_path"
    exit 0
}

# Sprint 1 — parallel fan-out dispatcher. Reads the parent agent's profile
# `parallel_subtasks` array, generates worker commands, runs them via
# fanout-dispatch.sh, then merges their artifacts via aggregator.sh into the
# canonical phase artifact. Writes one parent ledger entry of kind
# `agent_fanout` referencing all worker child entries.
#
# Worker command resolution:
#   - If EVOLVE_FANOUT_TEST_EXECUTOR is set, the worker command runs that
#     script with EVOLVE_FANOUT_WORKER_{NAME,ARTIFACT,TOKEN,CYCLE,WORKSPACE}
#     env vars populated. Used by dispatch-parallel-test.sh.
#   - Otherwise, the worker command recurses into this script as
#     `<agent>-worker-<name>`, with PROMPT_FILE_OVERRIDE pointing to the
#     rendered subtask prompt. cmd_run handles the worker pattern (above).
cmd_dispatch_parallel() {
    local agent="${1:?usage: dispatch-parallel <agent> <cycle> <workspace>}"
    local cycle="${2:?usage: dispatch-parallel <agent> <cycle> <workspace>}"
    local workspace="${3:?usage: dispatch-parallel <agent> <cycle> <workspace>}"

    [[ "$agent" =~ ^(scout|tdd-engineer|builder|auditor|inspirer|evaluator|retrospective|orchestrator|plan-reviewer|intent|triage|memo|tester)$ ]] \
        || fail "dispatch-parallel: unknown agent: $agent"
    [[ "$cycle" =~ ^[0-9]+$ ]] || fail "dispatch-parallel: cycle must be integer: $cycle"
    [ -d "$workspace" ] || fail "dispatch-parallel: workspace dir missing: $workspace"

    local profile="$PROFILES_DIR/${agent}.json"
    [ -f "$profile" ] || fail "dispatch-parallel: profile not found: $profile"
    require_bin jq

    # v8.55.0: structural enforcement of the parallelization-discipline
    # principle. A role can only be dispatched in parallel if its profile
    # explicitly declares parallel_eligible=true. The default is false (safe).
    # Profiles that mutate state (Builder, Intent, Orchestrator, tdd-engineer,
    # ship.sh) must declare false. Profiles that are READ-ONLY summarizers
    # (Scout, Auditor, Retrospective, plan-reviewer, evaluator, inspirer) may
    # declare true. See docs/architecture/sequential-write-discipline.md.
    local parallel_eligible
    parallel_eligible=$(jq -r '.parallel_eligible // false' "$profile" 2>/dev/null)
    if [ "$parallel_eligible" != "true" ]; then
        log "PROFILE-ERROR: agent '$agent' is not parallel-eligible (parallel_eligible=$parallel_eligible)"
        log "  Roles that WRITE state (Builder, Intent, Orchestrator, tdd-engineer) must run sequentially."
        log "  Only READ-ONLY summarizing roles (Scout, Auditor, Retrospective, plan-reviewer, evaluator, inspirer)"
        log "  may opt-in by declaring parallel_eligible=true in their profile JSON."
        log "  See: docs/architecture/sequential-write-discipline.md"
        log "  Single-writer invariant preserved: refusing dispatch-parallel for $agent."
        exit 2
    fi

    # Extract parallel_subtasks; require at least one entry.
    local subtask_count
    subtask_count=$(jq -r '.parallel_subtasks // [] | length' "$profile")
    if [ "$subtask_count" = "0" ] || [ "$subtask_count" = "null" ]; then
        fail "dispatch-parallel: profile $profile has no parallel_subtasks array"
    fi

    log "dispatch-parallel: agent=$agent cycle=$cycle workers=$subtask_count"

    # v8.52.0: resolve quality_tier per profile.cli for the fan-out parent
    # ledger entry. All fan-out workers share the same profile (and cli), so
    # one tier represents the whole fan-out.
    local fanout_quality_tier="unknown"
    local _cap_check="$EVOLVE_PLUGIN_ROOT/scripts/cli_adapters/_capability-check.sh"
    local _fanout_cli
    _fanout_cli=$(jq -r '.cli // "claude"' "$profile" 2>/dev/null)
    if [ -x "$_cap_check" ] && [ -n "$_fanout_cli" ]; then
        fanout_quality_tier=$(bash "$_cap_check" "$_fanout_cli" 2>/dev/null | jq -r '.quality_tier // "unknown"' 2>/dev/null || echo "unknown")
        [ -z "$fanout_quality_tier" ] && fanout_quality_tier="unknown"
        [ "$fanout_quality_tier" = "null" ] && fanout_quality_tier="unknown"
    fi
    log "dispatch-parallel: fanout_quality_tier=$fanout_quality_tier (cli=$_fanout_cli)"

    local workers_dir="$workspace/workers"
    mkdir -p "$workers_dir"

    # v8.23.0 Task D: collect worker names for cycle-state init-workers call.
    # We populate this in the loop below alongside building commands.tsv.
    local task_d_worker_names=""

    # v8.23.0 Task C: generate a shared cache-prefix file. Sibling workers in
    # the same fan-out batch reading byte-identical prefix bytes hit the same
    # Anthropic prompt-cache entry (≥1024 token, 5-min TTL) → ~47% token cost
    # reduction on 3-worker fan-out. Idempotent: same cycle+workspace produces
    # same bytes (deterministic — no timestamps, no random salts).
    local cache_prefix_file=""
    if [ "${EVOLVE_FANOUT_CACHE_PREFIX:-1}" = "1" ]; then
        cache_prefix_file="$workers_dir/cache-prefix.md"
        _write_cache_prefix "$cycle" "$agent" "$workspace" "$cache_prefix_file"
        log "dispatch-parallel: cache-prefix written ($(wc -l < "$cache_prefix_file" | tr -d ' ') lines, $(wc -c < "$cache_prefix_file" | tr -d ' ') bytes)"
    fi

    # Map phase name to aggregator merge mode (Scout-class agents → concat,
    # auditor → verdict, retrospective → lessons).
    local merge_phase
    case "$agent" in
        scout)         merge_phase="scout" ;;
        auditor)       merge_phase="audit" ;;
        retrospective) merge_phase="learn" ;;
        *)             merge_phase="$agent" ;; # passthrough; aggregator validates
    esac

    # Determine canonical aggregate artifact path: profile.output_artifact if
    # present, otherwise <workspace>/<agent>-report.md.
    local agg_template agg_path
    agg_template=$(jq -r '.output_artifact // empty' "$profile")
    if [ -n "$agg_template" ]; then
        agg_path="$REPO_ROOT/$(resolve_artifact_path "$agg_template" "$cycle")"
    else
        agg_path="$workspace/${agent}-report.md"
    fi
    mkdir -p "$(dirname "$agg_path")"

    local parent_token
    parent_token=$(generate_challenge_token)
    local git_state_at_start
    git_state_at_start=$(capture_git_state)

    # Build commands.tsv: <worker_name>\t<command>
    local commands_tsv="$workers_dir/.fanout-commands.tsv"
    : > "$commands_tsv"

    # Track worker names + artifact paths for the aggregator pass.
    local worker_names="" worker_artifacts=""

    local i=0
    while [ "$i" -lt "$subtask_count" ]; do
        local sname stemplate sprompt_file worker_artifact worker_token cmd
        sname=$(jq -r ".parallel_subtasks[$i].name" "$profile")
        stemplate=$(jq -r ".parallel_subtasks[$i].prompt_template // \"\"" "$profile")
        worker_artifact="$workers_dir/${agent}-${sname}.md"
        worker_token=$(generate_challenge_token)

        # Render prompt template (substitute {cycle}, {agent}, {worker}, {workspace}).
        sprompt_file="$workers_dir/.prompt-${sname}.txt"
        printf '%s' "$stemplate" \
            | sed "s|{cycle}|$cycle|g; s|{agent}|$agent|g; s|{worker}|$sname|g; s|{workspace}|$workspace|g" \
            > "$sprompt_file"

        if [ -n "${EVOLVE_FANOUT_TEST_EXECUTOR:-}" ]; then
            # Test mode: run the test executor with env vars; no LLM call.
            cmd="EVOLVE_FANOUT_PARENT_AGENT=$agent EVOLVE_FANOUT_WORKER_NAME=$sname EVOLVE_FANOUT_WORKER_ARTIFACT=$worker_artifact EVOLVE_FANOUT_WORKER_TOKEN=$worker_token EVOLVE_FANOUT_CYCLE=$cycle EVOLVE_FANOUT_WORKSPACE=$workspace bash $EVOLVE_FANOUT_TEST_EXECUTOR"
        else
            # Production mode: recurse through subagent-run.sh as <agent>-worker-<name>.
            # The worker borrows the parent profile but writes its own artifact.
            cmd="PROMPT_FILE_OVERRIDE=$sprompt_file bash ${BASH_SOURCE[0]} ${agent}-worker-${sname} $cycle $workspace"
        fi
        printf '%s\t%s\n' "${agent}-${sname}" "$cmd" >> "$commands_tsv"

        worker_names="$worker_names ${agent}-${sname}"
        worker_artifacts="$worker_artifacts $worker_artifact"
        task_d_worker_names="$task_d_worker_names ${agent}-${sname}"
        i=$((i + 1))
    done

    # v8.23.0 Task D: seed parallel_workers.workers[] with all workers in pending
    # status BEFORE dispatch. fanout-dispatch.sh's _run_worker will transition
    # each to running → done/failed as they execute.
    if [ "${EVOLVE_FANOUT_TRACK_WORKERS:-1}" = "1" ] && [ -f "$EVOLVE_PROJECT_ROOT/.evolve/cycle-state.json" ]; then
        # shellcheck disable=SC2086
        bash "$EVOLVE_PLUGIN_ROOT/scripts/lifecycle/cycle-state.sh" init-workers "$agent" $task_d_worker_names 2>/dev/null || true
    fi

    # Run workers in parallel.
    local results_tsv="$workers_dir/.fanout-results.tsv"
    local fanout_rc=0
    local fanout_args=()
    [ -n "$cache_prefix_file" ] && fanout_args+=(--cache-prefix-file "$cache_prefix_file")
    fanout_args+=("$commands_tsv" "$results_tsv")
    bash "$EVOLVE_PLUGIN_ROOT/scripts/dispatch/fanout-dispatch.sh" "${fanout_args[@]}" || fanout_rc=$?

    if [ "$fanout_rc" -ne 0 ]; then
        log "dispatch-parallel: fanout-dispatch returned non-zero (rc=$fanout_rc); per-worker exit codes:"
        if [ -f "$results_tsv" ]; then
            sed 's/^/  /' "$results_tsv" >&2
        fi
        # Continue to write a parent ledger entry recording the failure, then exit.
        local agg_path_or_empty=""
        [ -f "$agg_path" ] && agg_path_or_empty="$agg_path"
        _write_fanout_ledger_entry "$cycle" "$agent" "$parent_token" "$git_state_at_start" \
            "$worker_names" "$subtask_count" "$fanout_rc" "$agg_path_or_empty" "$results_tsv" "$fanout_quality_tier"
        exit 1
    fi

    # Aggregate worker artifacts into canonical phase artifact.
    local agg_rc=0
    # shellcheck disable=SC2086
    bash "$EVOLVE_PLUGIN_ROOT/scripts/dispatch/aggregator.sh" "$merge_phase" "$agg_path" $worker_artifacts || agg_rc=$?

    # Write parent ledger entry regardless of agg_rc (record outcome).
    _write_fanout_ledger_entry "$cycle" "$agent" "$parent_token" "$git_state_at_start" \
        "$worker_names" "$subtask_count" "$agg_rc" "$agg_path" "$results_tsv" "$fanout_quality_tier"

    if [ "$agg_rc" -ne 0 ]; then
        log "dispatch-parallel: aggregator returned non-zero (rc=$agg_rc) for phase=$merge_phase"
        exit 1
    fi
    log "dispatch-parallel: DONE agent=$agent cycle=$cycle aggregate=$agg_path workers=$subtask_count"
    exit 0
}

# Helper: write a single ledger entry for a fan-out parent invocation.
# Args: cycle agent token git_state worker_names_space_separated worker_count exit_code agg_path results_tsv
_write_fanout_ledger_entry() {
    local cycle="$1" agent="$2" token="$3" git_state="$4"
    local worker_names="$5" worker_count="$6" exit_code="$7"
    local agg_path="$8" results_tsv="$9"
    # v8.52.0: quality_tier (10th arg) — resolved per profile.cli at fan-out
    # dispatch time. All workers in a fan-out share the same agent profile.
    local quality_tier="${10:-unknown}"
    local artifact_sha=""
    if [ -n "$agg_path" ] && [ -f "$agg_path" ]; then
        if command -v sha256sum >/dev/null 2>&1; then
            artifact_sha=$(sha256sum "$agg_path" | awk '{print $1}')
        else
            artifact_sha=$(shasum -a 256 "$agg_path" | awk '{print $1}')
        fi
    fi
    local ts; ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    local git_head="${git_state%%:*}"
    local tree_state_sha="${git_state##*:}"

    # Build workers JSON array via jq.
    local workers_json="[]"
    if [ -n "$worker_names" ]; then
        # shellcheck disable=SC2086
        workers_json=$(printf '%s\n' $worker_names | jq -R . | jq -s .)
    fi

    mkdir -p "$(dirname "$LEDGER")"
    # v8.37.0: tamper-evident hash chain.
    local prev_hash entry_seq chain_link
    chain_link=$(_ledger_chain_link)
    prev_hash="${chain_link%% *}"
    entry_seq="${chain_link##* }"
    local new_line
    new_line=$(jq -nc \
        --arg ts "$ts" \
        --argjson cycle "$cycle" \
        --arg agent "$agent" \
        --argjson exit_code "$exit_code" \
        --arg artifact_path "$agg_path" \
        --arg artifact_sha256 "$artifact_sha" \
        --arg challenge_token "$token" \
        --arg git_head "$git_head" \
        --arg tree_state_sha "$tree_state_sha" \
        --argjson worker_count "$worker_count" \
        --argjson workers "$workers_json" \
        --argjson entry_seq "$entry_seq" \
        --arg prev_hash "$prev_hash" \
        --arg quality_tier "$quality_tier" \
        '{ts: $ts, cycle: $cycle, role: $agent, kind: "agent_fanout",
          exit_code: $exit_code,
          artifact_path: $artifact_path, artifact_sha256: $artifact_sha256,
          challenge_token: $challenge_token,
          git_head: $git_head, tree_state_sha: $tree_state_sha,
          worker_count: $worker_count, workers: $workers,
          entry_seq: $entry_seq, prev_hash: $prev_hash,
          quality_tier: $quality_tier}')
    printf '%s\n' "$new_line" >> "$LEDGER"
    local new_sha
    new_sha=$(printf '%s' "$new_line" | _ledger_sha256_stdin)
    _ledger_update_tip "$entry_seq" "$new_sha"
}

# --- Main --------------------------------------------------------------------

[ $# -ge 1 ] || { cat >&2 <<'USAGE'
Usage:
  subagent-run.sh <agent> <cycle> <workspace_path>
  subagent-run.sh dispatch-parallel <agent> <cycle> <workspace_path>
  subagent-run.sh --validate-profile <agent>
  subagent-run.sh --check-token <artifact_path> <token>
  subagent-run.sh --check-ctx-advisory <profile_json> <tokens>

Agents: intent | scout | tdd-engineer | builder | auditor | inspirer | evaluator | retrospective | orchestrator | plan-reviewer
USAGE
    exit 1
}

case "$1" in
    --validate-profile) shift; cmd_validate_profile "$@" ;;
    --check-token) shift; cmd_check_token "$@" ;;
    --check-ctx-advisory) shift; cmd_check_ctx_advisory "$@" ;;
    # v8.35.0: testability hook for adaptive auditor model selection.
    # Usage: --resolve-tier <agent> — prints the tier resolved for that agent
    # given current env (MODEL_TIER_HINT, EVOLVE_AUDITOR_TIER_OVERRIDE, etc.).
    --resolve-tier)
        shift
        agent="${1:?usage: --resolve-tier <agent>}"
        profile="$PROFILES_DIR/${agent}.json"
        [ -f "$profile" ] || { echo "[subagent-run] no profile: $profile" >&2; exit 1; }
        resolve_model_tier "$profile" "0"
        exit 0
        ;;
    dispatch-parallel) shift; cmd_dispatch_parallel "$@" ;;
    *) cmd_run "$@" ;;
esac
