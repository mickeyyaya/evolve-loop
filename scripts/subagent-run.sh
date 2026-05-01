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
#   <agent>           — one of: scout, tdd-engineer, builder, auditor, inspirer, evaluator, retrospective, orchestrator
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
PROFILES_DIR="${EVOLVE_PROFILES_DIR_OVERRIDE:-$REPO_ROOT/.evolve/profiles}"
ADAPTERS_DIR="$REPO_ROOT/scripts/cli_adapters"
LEDGER="${EVOLVE_LEDGER_OVERRIDE:-$REPO_ROOT/.evolve/ledger.jsonl}"

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

write_ledger_entry() {
    local cycle="$1" agent="$2" model="$3" exit_code="$4" duration_s="$5"
    local artifact_path="$6" challenge_token="$7" git_state="${8:-unknown:unknown}"
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
    # CYCLE-BINDING (v8.13.0): git_head + tree_state_sha pin the audit to the
    # exact code state at audit time. ship.sh requires the current state to
    # match these — preventing "audit cycle 50, ship cycle 51" exploits.
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
        --arg git_head "$git_head" \
        --arg tree_state_sha "$tree_state_sha" \
        '{ts: $ts, cycle: $cycle, role: $agent, kind: "agent_subprocess",
          model: $model, exit_code: $exit_code, duration_s: $duration_s,
          artifact_path: $artifact_path, artifact_sha256: $artifact_sha256,
          challenge_token: $challenge_token,
          git_head: $git_head, tree_state_sha: $tree_state_sha}' \
        >> "$LEDGER"
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
    [[ "$agent_role" =~ ^(scout|tdd-engineer|builder|auditor|inspirer|evaluator|retrospective|orchestrator|plan-reviewer)$ ]] || fail "unknown agent: $agent"
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

    # NOTE: cycle 8127 RC4 audit empirically demonstrated that the auto-
    # worktree EPERM fix had three failures:
    # (1) writes still denied because the profile's relative-path patterns
    #     don't match the auditor's absolute-path resolution under cwd=worktree;
    # (2) `git worktree add --detach HEAD` shows the COMMITTED state at HEAD,
    #     not the orchestrator's uncommitted working tree, so the subagent
    #     audits the wrong code;
    # (3) EPERM still occurred — Claude Code's cleanup is NOT keyed on
    #     project-path-hash alone.
    # The auto-worktree code has been reverted. EPERM remains an open issue
    # tracked for v8.12.4 with a different research path (likely targeting
    # the actual cleanup trigger — possibly `.claude/` directory location
    # or parent PID, not cwd).

    record_phase prep_total_ms

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
    record_phase adapter_invoke_ms

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
        write_ledger_entry "$cycle" "$agent" "$model" "$cli_exit" "$duration" "$artifact_path" "$challenge_token" "$git_state_at_start"
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

    write_ledger_entry "$cycle" "$agent" "$model" 0 "$duration" "$artifact_path" "$challenge_token" "$git_state_at_start"
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

    [[ "$agent" =~ ^(scout|tdd-engineer|builder|auditor|inspirer|evaluator|retrospective|orchestrator|plan-reviewer)$ ]] \
        || fail "dispatch-parallel: unknown agent: $agent"
    [[ "$cycle" =~ ^[0-9]+$ ]] || fail "dispatch-parallel: cycle must be integer: $cycle"
    [ -d "$workspace" ] || fail "dispatch-parallel: workspace dir missing: $workspace"

    local profile="$PROFILES_DIR/${agent}.json"
    [ -f "$profile" ] || fail "dispatch-parallel: profile not found: $profile"
    require_bin jq

    # Extract parallel_subtasks; require at least one entry.
    local subtask_count
    subtask_count=$(jq -r '.parallel_subtasks // [] | length' "$profile")
    if [ "$subtask_count" = "0" ] || [ "$subtask_count" = "null" ]; then
        fail "dispatch-parallel: profile $profile has no parallel_subtasks array"
    fi

    log "dispatch-parallel: agent=$agent cycle=$cycle workers=$subtask_count"

    local workers_dir="$workspace/workers"
    mkdir -p "$workers_dir"

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
        i=$((i + 1))
    done

    # Run workers in parallel.
    local results_tsv="$workers_dir/.fanout-results.tsv"
    local fanout_rc=0
    bash "$REPO_ROOT/scripts/fanout-dispatch.sh" "$commands_tsv" "$results_tsv" || fanout_rc=$?

    if [ "$fanout_rc" -ne 0 ]; then
        log "dispatch-parallel: fanout-dispatch returned non-zero (rc=$fanout_rc); per-worker exit codes:"
        if [ -f "$results_tsv" ]; then
            sed 's/^/  /' "$results_tsv" >&2
        fi
        # Continue to write a parent ledger entry recording the failure, then exit.
        local agg_path_or_empty=""
        [ -f "$agg_path" ] && agg_path_or_empty="$agg_path"
        _write_fanout_ledger_entry "$cycle" "$agent" "$parent_token" "$git_state_at_start" \
            "$worker_names" "$subtask_count" "$fanout_rc" "$agg_path_or_empty" "$results_tsv"
        exit 1
    fi

    # Aggregate worker artifacts into canonical phase artifact.
    local agg_rc=0
    # shellcheck disable=SC2086
    bash "$REPO_ROOT/scripts/aggregator.sh" "$merge_phase" "$agg_path" $worker_artifacts || agg_rc=$?

    # Write parent ledger entry regardless of agg_rc (record outcome).
    _write_fanout_ledger_entry "$cycle" "$agent" "$parent_token" "$git_state_at_start" \
        "$worker_names" "$subtask_count" "$agg_rc" "$agg_path" "$results_tsv"

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
    jq -nc \
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
        '{ts: $ts, cycle: $cycle, role: $agent, kind: "agent_fanout",
          exit_code: $exit_code,
          artifact_path: $artifact_path, artifact_sha256: $artifact_sha256,
          challenge_token: $challenge_token,
          git_head: $git_head, tree_state_sha: $tree_state_sha,
          worker_count: $worker_count, workers: $workers}' \
        >> "$LEDGER"
}

# --- Main --------------------------------------------------------------------

[ $# -ge 1 ] || { cat >&2 <<'USAGE'
Usage:
  subagent-run.sh <agent> <cycle> <workspace_path>
  subagent-run.sh dispatch-parallel <agent> <cycle> <workspace_path>
  subagent-run.sh --validate-profile <agent>
  subagent-run.sh --check-token <artifact_path> <token>

Agents: scout | tdd-engineer | builder | auditor | inspirer | evaluator | retrospective | orchestrator
USAGE
    exit 1
}

case "$1" in
    --validate-profile) shift; cmd_validate_profile "$@" ;;
    --check-token) shift; cmd_check_token "$@" ;;
    dispatch-parallel) shift; cmd_dispatch_parallel "$@" ;;
    *) cmd_run "$@" ;;
esac
