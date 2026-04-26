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
#   bash scripts/subagent-run.sh <agent> <cycle> <workspace_path> [--prompt-file PATH]
#   bash scripts/subagent-run.sh --validate-profile <agent>
#   bash scripts/subagent-run.sh --check-token <artifact_path> <token>
#
# Arguments:
#   <agent>           — one of: scout, builder, auditor, inspirer, evaluator
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

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROFILES_DIR="$REPO_ROOT/.evolve/profiles"
ADAPTERS_DIR="$REPO_ROOT/scripts/cli_adapters"
LEDGER="$REPO_ROOT/.evolve/ledger.jsonl"

log() { echo "[subagent-run] $*" >&2; }
fail() { log "FAIL: $*"; exit 1; }
integrity_fail() { log "INTEGRITY-FAIL: $*"; exit 2; }

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

write_ledger_entry() {
    local cycle="$1" agent="$2" model="$3" exit_code="$4" duration_s="$5"
    local artifact_path="$6" challenge_token="$7"
    local artifact_sha=""
    if [ -f "$artifact_path" ]; then
        if command -v sha256sum >/dev/null 2>&1; then
            artifact_sha=$(sha256sum "$artifact_path" | awk '{print $1}')
        else
            artifact_sha=$(shasum -a 256 "$artifact_path" | awk '{print $1}')
        fi
    fi
    local ts
    ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    jq -nc \
        --arg ts "$ts" \
        --argjson cycle "$cycle" \
        --arg agent "$agent" \
        --arg model "$model" \
        --argjson exit_code "$exit_code" \
        --arg duration_s "$duration_s" \
        --arg artifact_path "$artifact_path" \
        --arg artifact_sha256 "$artifact_sha" \
        --arg challenge_token "$challenge_token" \
        '{ts: $ts, cycle: $cycle, role: $agent, kind: "agent_subprocess",
          model: $model, exit_code: $exit_code, duration_s: $duration_s,
          artifact_path: $artifact_path, artifact_sha256: $artifact_sha256,
          challenge_token: $challenge_token}' \
        >> "$LEDGER"
}

verify_artifact() {
    local artifact_path="$1" challenge_token="$2"
    [ -f "$artifact_path" ] || integrity_fail "artifact missing: $artifact_path"
    [ -s "$artifact_path" ] || integrity_fail "artifact empty: $artifact_path"
    # Freshness: must be < 30s old (subagent just wrote it).
    local age
    if [[ "$OSTYPE" == "darwin"* ]]; then
        age=$(( $(date +%s) - $(stat -f %m "$artifact_path") ))
    else
        age=$(( $(date +%s) - $(stat -c %Y "$artifact_path") ))
    fi
    [ "$age" -lt 30 ] || integrity_fail "artifact stale (${age}s old): $artifact_path"
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

    local cli
    cli=$(jq -r '.cli' "$profile")
    local adapter="$ADAPTERS_DIR/${cli}.sh"
    [ -x "$adapter" ] || fail "adapter not executable: $adapter"

    # Print resolved command via VALIDATE_ONLY=1 invocation.
    local tmp_prompt
    tmp_prompt=$(mktemp)
    echo "VALIDATE-ONLY DRY RUN" > "$tmp_prompt"
    PROFILE_PATH="$profile" \
    RESOLVED_MODEL="$(jq -r '.model_tier_default' "$profile")" \
    PROMPT_FILE="$tmp_prompt" \
    CYCLE="0" \
    WORKSPACE_PATH="$REPO_ROOT/.evolve/runs/cycle-0" \
    WORKTREE_PATH="${WORKTREE_PATH:-$REPO_ROOT}" \
    STDOUT_LOG="/dev/null" \
    STDERR_LOG="/dev/null" \
    ARTIFACT_PATH="$(resolve_artifact_path "$(jq -r '.output_artifact' "$profile")" 0)" \
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

cmd_run() {
    local agent="$1" cycle="$2" workspace="$3"
    [[ "$agent" =~ ^(scout|builder|auditor|inspirer|evaluator)$ ]] || fail "unknown agent: $agent"
    [[ "$cycle" =~ ^[0-9]+$ ]] || fail "cycle must be integer: $cycle"
    [ -d "$workspace" ] || fail "workspace dir does not exist: $workspace"

    local profile="$PROFILES_DIR/${agent}.json"
    [ -f "$profile" ] || fail "profile not found: $profile"

    require_bin jq
    jq empty "$profile" || fail "profile is not valid JSON: $profile"

    local cli adapter
    cli=$(jq -r '.cli' "$profile")
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
    model=$(resolve_model_tier "$profile" "$cycle")
    local artifact_template
    artifact_template=$(jq -r '.output_artifact' "$profile")
    local artifact_path
    artifact_path="$REPO_ROOT/$(resolve_artifact_path "$artifact_template" "$cycle")"

    mkdir -p "$(dirname "$artifact_path")"

    # Generate challenge token; pass to subagent via prompt prefix.
    local challenge_token
    challenge_token=$(generate_challenge_token)

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

    # Inject challenge token + artifact-path mandate into prompt prefix.
    local injected_prompt
    injected_prompt=$(mktemp)
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

    cat >> "$injected_prompt" <<EOF
--- BEGIN TASK PROMPT ---
EOF
    cat "$prompt_file" >> "$injected_prompt"
    echo "" >> "$injected_prompt"
    echo "--- END TASK PROMPT ---" >> "$injected_prompt"

    local stdout_log="$workspace/${agent}-stdout.log"
    local stderr_log="$workspace/${agent}-stderr.log"

    log "starting $agent (cycle $cycle, model $model, cli $cli, token $challenge_token)"
    local start_ts
    start_ts=$(date +%s)

    set +e
    PROFILE_PATH="$profile" \
    RESOLVED_MODEL="$model" \
    PROMPT_FILE="$injected_prompt" \
    CYCLE="$cycle" \
    WORKSPACE_PATH="$workspace" \
    WORKTREE_PATH="${WORKTREE_PATH:-}" \
    STDOUT_LOG="$stdout_log" \
    STDERR_LOG="$stderr_log" \
    ARTIFACT_PATH="$artifact_path" \
    bash "$adapter"
    local cli_exit=$?
    set -e

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
        log "stderr tail: $(tail -5 "$stderr_log" 2>/dev/null || echo '<empty>')"
        write_ledger_entry "$cycle" "$agent" "$model" "$cli_exit" "$duration" "$artifact_path" "$challenge_token"
        exit 1
    fi

    # Post-run integrity verification.
    verify_artifact "$artifact_path" "$challenge_token"

    write_ledger_entry "$cycle" "$agent" "$model" 0 "$duration" "$artifact_path" "$challenge_token"
    log "DONE: $agent cycle $cycle in ${duration}s, artifact at $artifact_path"
    exit 0
}

# --- Main --------------------------------------------------------------------

[ $# -ge 1 ] || { cat >&2 <<'USAGE'
Usage:
  subagent-run.sh <agent> <cycle> <workspace_path>
  subagent-run.sh --validate-profile <agent>
  subagent-run.sh --check-token <artifact_path> <token>

Agents: scout | builder | auditor | inspirer | evaluator
USAGE
    exit 1
}

case "$1" in
    --validate-profile) shift; cmd_validate_profile "$@" ;;
    --check-token) shift; cmd_check_token "$@" ;;
    *) cmd_run "$@" ;;
esac
