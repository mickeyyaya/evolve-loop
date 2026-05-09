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
#
# Arguments:
#   <agent>           — one of: intent, scout, tdd-engineer, builder, auditor, inspirer, evaluator, retrospective, orchestrator, plan-reviewer
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
unset __rr_self

# Backwards-compat: many helper functions below still reference $REPO_ROOT.
# Map it to PROJECT_ROOT (the writable side) for those callsites — read-only
# resources (profiles, adapters, sibling scripts) explicitly use PLUGIN_ROOT.
REPO_ROOT="$EVOLVE_PROJECT_ROOT"

PROFILES_DIR="${EVOLVE_PROFILES_DIR_OVERRIDE:-$EVOLVE_PLUGIN_ROOT/.evolve/profiles}"
ADAPTERS_DIR="$EVOLVE_PLUGIN_ROOT/scripts/cli_adapters"
LEDGER="${EVOLVE_LEDGER_OVERRIDE:-$EVOLVE_PROJECT_ROOT/.evolve/ledger.jsonl}"

# v8.16.2: explicitly export runtime knobs so they reach the adapter through
# any nested bash/sandbox-exec layer. Belt-and-suspenders for env propagation.
[ -n "${EVOLVE_SANDBOX_FALLBACK_ON_EPERM:-}" ] && export EVOLVE_SANDBOX_FALLBACK_ON_EPERM

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
        '{ts: $ts, cycle: $cycle, role: $agent, kind: "agent_subprocess",
          model: $model, exit_code: $exit_code, duration_s: $duration_s,
          artifact_path: $artifact_path, artifact_sha256: $artifact_sha256,
          challenge_token: $challenge_token,
          git_head: $git_head, tree_state_sha: $tree_state_sha,
          entry_seq: $entry_seq, prev_hash: $prev_hash,
          quality_tier: $quality_tier}')
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
    [[ "$agent_role" =~ ^(scout|tdd-engineer|builder|auditor|inspirer|evaluator|retrospective|orchestrator|plan-reviewer|intent)$ ]] || fail "unknown agent: $agent"
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

    # v8.56.0 Layer B: soft prompt-size guard. Kernel-layer check that
    # the orchestrator's prompt assembly didn't blow past the budget.
    # Token estimation uses the 1-token≈4-bytes English upper bound.
    # WARN-only by default; set EVOLVE_PROMPT_BUDGET_ENFORCE=1 to make it
    # a hard exit (operator opt-in). Guard is INFORMATIONAL — it cannot
    # know if the orchestrator legitimately needs a big prompt for this
    # role (Retrospective is the synthesizer; it sees everything).
    local _prompt_max="${EVOLVE_PROMPT_MAX_TOKENS:-30000}"
    local _prompt_bytes
    _prompt_bytes=$(wc -c < "$injected_prompt" | tr -d ' ')
    local _prompt_tokens=$((_prompt_bytes / 4))
    if [ "$_prompt_tokens" -gt "$_prompt_max" ]; then
        echo "[subagent-run] WARN: $agent prompt is ~$_prompt_tokens tokens (cap=$_prompt_max). Consider role-context-builder.sh for filtered context. Layer-B reference: agents/evolve-orchestrator.md#per-phase-prompt-context" >&2
        if [ "${EVOLVE_PROMPT_BUDGET_ENFORCE:-0}" = "1" ]; then
            fail "EVOLVE_PROMPT_BUDGET_ENFORCE=1 + prompt over cap; aborting"
        fi
    fi

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

    log "starting $agent (cycle $cycle, model $model, cli $cli, tier $quality_tier, token $challenge_token)"
    local start_ts
    start_ts=$(date +%s)

    set +e
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
        write_ledger_entry "$cycle" "$agent" "$model" "$cli_exit" "$duration" "$artifact_path" "$challenge_token" "$git_state_at_start" "$quality_tier"
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

    write_ledger_entry "$cycle" "$agent" "$model" 0 "$duration" "$artifact_path" "$challenge_token" "$git_state_at_start" "$quality_tier"
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

    [[ "$agent" =~ ^(scout|tdd-engineer|builder|auditor|inspirer|evaluator|retrospective|orchestrator|plan-reviewer|intent)$ ]] \
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

Agents: intent | scout | tdd-engineer | builder | auditor | inspirer | evaluator | retrospective | orchestrator | plan-reviewer
USAGE
    exit 1
}

case "$1" in
    --validate-profile) shift; cmd_validate_profile "$@" ;;
    --check-token) shift; cmd_check_token "$@" ;;
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
