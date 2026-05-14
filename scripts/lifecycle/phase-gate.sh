#!/usr/bin/env bash
#
# phase-gate.sh — Deterministic phase transition verification
#
# Runs between every phase transition in the evolve-loop. Verifies that
# required artifacts exist, agents actually ran, and integrity checks pass.
# The orchestrator (LLM) cannot skip this script because it's invoked by
# the host environment, not by the LLM itself.
#
# Usage: bash scripts/lifecycle/phase-gate.sh <gate> <cycle> <workspace_path>
#
# Gates:
#   research-to-discover — Verify Phase 1 ran, research-brief exists
#   discover-to-build   — Verify Scout ran, eval definitions exist
#   build-to-audit      — Verify Builder ran, build-report exists
#   audit-to-ship       — Verify Auditor ran, eval graders pass independently
#   ship-to-learn       — Verify commit succeeded, update state.json
#   cycle-complete       — Full cycle health check, mastery update
#
# Exit codes:
#   0 = PASS (proceed to next phase)
#   1 = FAIL (block phase transition, retry or halt)
#   2 = ANOMALY (halt immediately, present to human)
#
# IMPORTANT: This script is the trust boundary. The LLM orchestrator
# should NOT be able to modify this script during a cycle. If the Builder
# modifies files in scripts/, the Auditor flags it as CRITICAL.

set -euo pipefail

# v8.18.0: dual-root resolution. phase-gate.sh reads ledger and state under the
# user's project (writable side). Previously used relative ".evolve/..." paths
# which depended on cwd; now resolves explicitly via EVOLVE_PROJECT_ROOT.
__rr_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$__rr_self/resolve-roots.sh"
unset __rr_self

GATE="${1:?Usage: phase-gate.sh <gate> <cycle> <workspace_path>}"
CYCLE="${2:?Missing cycle number}"
WORKSPACE="${3:?Missing workspace path}"
EVOLVE_DIR="$EVOLVE_PROJECT_ROOT/.evolve"
LEDGER="$EVOLVE_DIR/ledger.jsonl"
STATE="$EVOLVE_DIR/state.json"

# Timestamp for freshness check (files must be < 10 minutes old)
FRESHNESS_THRESHOLD=600

log() { echo "[phase-gate:$GATE] $1"; }
fail() { log "FAIL: $1"; exit 1; }
anomaly() { log "ANOMALY: $1"; exit 2; }

check_file_exists() {
    local file="$1"
    local desc="$2"
    [ -f "$file" ] || fail "$desc missing: $file"
    [ -s "$file" ] || fail "$desc is empty: $file"
    log "OK: $desc exists ($(wc -l < "$file") lines)"
}

check_file_fresh() {
    local file="$1"
    local desc="$2"
    if [[ "$OSTYPE" == "darwin"* ]]; then
        local mtime
        mtime=$(stat -f %m "$file")
        local now
        now=$(date +%s)
        local age=$((now - mtime))
    else
        local age
        age=$(( $(date +%s) - $(stat -c %Y "$file") ))
    fi
    [ "$age" -lt "$FRESHNESS_THRESHOLD" ] || fail "$desc is stale (${age}s old, threshold ${FRESHNESS_THRESHOLD}s): $file"
    log "OK: $desc is fresh (${age}s old)"
}

check_ledger_role() {
    local role="$1"
    if [ -f "$LEDGER" ]; then
        local count
        count=$(grep -c "\"cycle\":$CYCLE.*\"role\":\"$role\"\|\"role\":\"$role\".*\"cycle\":$CYCLE" "$LEDGER" 2>/dev/null || echo "0")
        [ "$count" -gt 0 ] || fail "No $role ledger entry for cycle $CYCLE"
        log "OK: $role ledger entry found for cycle $CYCLE"
    else
        log "WARN: No ledger file found (first cycle?)"
    fi
}

# Verify subagent-run.sh ledger entries match on-disk artifacts.
# When subagents are invoked via scripts/dispatch/subagent-run.sh, each invocation appends
# an "agent_subprocess" ledger entry containing the SHA256 of the artifact at
# write time. This check verifies the artifact has not been mutated between the
# subagent's exit and the phase gate. Catches the "wrote artifact then mutated"
# forgery class.
check_subagent_ledger_match() {
    local role="$1"
    [ -f "$LEDGER" ] || { log "WARN: ledger missing, skipping subagent match check"; return 0; }
    if ! command -v jq >/dev/null 2>&1; then
        log "WARN: jq not available, skipping subagent_ledger_match"
        return 0
    fi
    # Find the most recent agent_subprocess entry for this cycle+role.
    local entry
    entry=$(grep "\"kind\":\"agent_subprocess\"" "$LEDGER" 2>/dev/null \
        | jq -c --argjson cycle "$CYCLE" --arg role "$role" \
            'select(.cycle == $cycle and .role == $role)' 2>/dev/null \
        | tail -1)
    if [ -z "$entry" ]; then
        # Subagent runner was not used for this role/cycle (legacy path or no run yet).
        # Don't fail — this check is additive for the subprocess-isolation rollout.
        log "INFO: no agent_subprocess ledger entry for $role cycle $CYCLE (legacy dispatch?)"
        return 0
    fi
    local exit_code recorded_sha artifact_path
    exit_code=$(echo "$entry" | jq -r '.exit_code')
    recorded_sha=$(echo "$entry" | jq -r '.artifact_sha256')
    artifact_path=$(echo "$entry" | jq -r '.artifact_path')
    [ "$exit_code" = "0" ] || fail "subagent $role cycle $CYCLE exit_code=$exit_code in ledger"
    [ -f "$artifact_path" ] || fail "subagent $role artifact missing on disk: $artifact_path"
    local actual_sha
    if command -v sha256sum >/dev/null 2>&1; then
        actual_sha=$(sha256sum "$artifact_path" | awk '{print $1}')
    else
        actual_sha=$(shasum -a 256 "$artifact_path" | awk '{print $1}')
    fi
    if [ "$recorded_sha" != "$actual_sha" ]; then
        anomaly "subagent $role artifact mutated post-run: ledger=$recorded_sha actual=$actual_sha"
    fi
    log "OK: subagent $role artifact SHA256 matches ledger ($recorded_sha)"
}

# ─── Anti-Forgery Checks (added after Gemini forgery incident) ───

# Verify artifact contains substantive content, not templated forgery.
# Forgery scripts generate generic text like "Improve color and texture for UI element $i"
# Real artifacts reference specific files, line numbers, and eval commands.
check_artifact_substance() {
    local file="$1"
    local desc="$2"
    local min_unique_words=20

    # Check 1: Minimum content complexity (forgeries are short templates)
    local word_count
    word_count=$(wc -w < "$file" | tr -d ' ')
    [ "$word_count" -ge 50 ] || fail "$desc has only $word_count words — likely templated forgery (minimum 50)"

    # Check 2: Must reference at least one real project file path
    # Real reports mention files like "src/game.swift" or "agents/evolve-scout.md"
    local file_refs
    file_refs=$(grep -cE '\.(swift|ts|js|py|go|rs|md|json|yaml|sh|css|html)' "$file" 2>/dev/null | tail -1 || echo "0")
    [ "$file_refs" -gt 0 ] || fail "$desc contains no file path references — likely forgery (real reports reference specific files)"

    log "OK: $desc has substantive content ($word_count words, $file_refs file references)"
}

# Verify git diff is non-empty (catches `git commit --allow-empty` forgery)
check_git_diff_substance() {
    local files_changed
    files_changed=$(git diff HEAD~1 --stat 2>/dev/null | tail -1 | grep -oE '[0-9]+ file' | grep -oE '[0-9]+' || echo "0")
    [ "$files_changed" -gt 0 ] || anomaly "Most recent commit has 0 file changes — possible empty-commit forgery (git commit --allow-empty)"
    log "OK: Most recent commit changed $files_changed file(s)"
}

# Verify state.json wasn't modified outside the phase gate
# Call at cycle start to capture checksum, call at cycle end to verify
capture_state_checksum() {
    if command -v sha256sum &>/dev/null; then
        sha256sum "$STATE" > "$WORKSPACE/.state-checksum"
    elif command -v shasum &>/dev/null; then
        shasum -a 256 "$STATE" > "$WORKSPACE/.state-checksum"
    fi
    log "OK: state.json checksum captured"
}

verify_state_checksum() {
    if [ -f "$WORKSPACE/.state-checksum" ]; then
        local valid
        if command -v sha256sum &>/dev/null; then
            valid=$(sha256sum -c "$WORKSPACE/.state-checksum" --quiet 2>&1 && echo "yes" || echo "no")
        elif command -v shasum &>/dev/null; then
            valid=$(shasum -a 256 -c "$WORKSPACE/.state-checksum" --quiet 2>&1 && echo "yes" || echo "no")
        else
            valid="skip"
        fi
        if [ "$valid" = "no" ]; then
            anomaly "state.json was modified outside phase-gate.sh — possible direct manipulation (jq, python, etc.)"
        fi
        [ "$valid" != "skip" ] && log "OK: state.json checksum matches (no external modification)"
    fi
}

# Check for forgery script artifacts in the workspace
check_no_forgery_scripts() {
    local forgery_scripts
    forgery_scripts=$(find . -maxdepth 2 -name '*forgery*' -o -name '*forge*' -o -name '*fake*' -o -name '*simulate*cycles*' 2>/dev/null | head -5)
    if [ -n "$forgery_scripts" ]; then
        anomaly "Forgery-related files detected: $forgery_scripts"
    fi

    # Check for new .sh files created during this cycle (LLM should not write scripts)
    local new_scripts
    new_scripts=$(find . -maxdepth 3 -name '*.sh' -newer "$WORKSPACE/.state-checksum" 2>/dev/null \
        | grep -v 'scripts/lifecycle/phase-gate.sh' \
        | grep -v 'scripts/observability/cycle-health-check.sh' \
        | grep -v 'scripts/verification/verify-eval.sh' \
        | grep -v 'scripts/verification/eval-quality-check.sh' \
        | grep -v 'scripts/utility/setup-skill-inventory.sh' \
        | head -5 || true)
    if [ -n "$new_scripts" ]; then
        log "WARN: New shell scripts created during cycle: $new_scripts — review for forgery"
    fi
}

# ─── Gate: RESEARCH → DISCOVER ───
gate_research_to_discover() {
    log "Checking RESEARCH → DISCOVER gate for cycle $CYCLE"

    # 1. Research brief must exist and be fresh
    check_file_exists "$WORKSPACE/research-brief.md" "Research brief"
    check_file_fresh "$WORKSPACE/research-brief.md" "Research brief"

    # 2. Research brief must have substantive content (not just headers)
    local brief_words
    brief_words=$(wc -w < "$WORKSPACE/research-brief.md" | tr -d ' ')
    if [ "$brief_words" -lt 30 ]; then
        fail "Research brief has only $brief_words words (min 30)"
    fi

    # 3. Research agenda must have been updated (check state.json)
    if [ -f "$STATE" ]; then
        local has_agenda
        has_agenda=$(grep -c '"researchAgenda"' "$STATE" 2>/dev/null || echo "0")
        if [ "$has_agenda" -eq 0 ]; then
            log "WARN: No researchAgenda in state.json — may be first cycle"
        fi
    fi

    log "PASS: RESEARCH → DISCOVER gate"
}

# ─── Gate: DISCOVER → BUILD ───
gate_discover_to_build() {
    log "Checking DISCOVER → BUILD gate for cycle $CYCLE"

    # 0. Capture state checksum (for tamper detection at cycle end)
    capture_state_checksum

    # 1. Scout report must exist, be fresh, and have substantive content
    check_file_exists "$WORKSPACE/scout-report.md" "Scout report"
    check_file_fresh "$WORKSPACE/scout-report.md" "Scout report"
    check_artifact_substance "$WORKSPACE/scout-report.md" "Scout report"
    check_subagent_ledger_match "scout"

    # 1b. v8.57.0 Layer S: when state.json:carryoverTodos[] is non-empty, the
    # scout-report MUST contain a '## Carryover Decisions' section so Layer-D
    # reconcile can update cycles_unpicked correctly. Empty carryoverTodos[]
    # means no requirement — backward-compatible with v8.56.0 cycles.
    if [ -f "$STATE" ]; then
        local _carryover_count
        _carryover_count=$(jq -r '(.carryoverTodos // []) | length' "$STATE" 2>/dev/null || echo 0)
        if [ "${_carryover_count:-0}" -gt 0 ]; then
            if ! grep -qE '^##[[:space:]]+Carryover[[:space:]]+Decisions' "$WORKSPACE/scout-report.md"; then
                fail "carryoverTodos[] has $_carryover_count entries but scout-report.md is missing required '## Carryover Decisions' section (v8.57.0 Layer S — see agents/evolve-scout.md Task Selection)"
            fi
            log "OK: scout-report contains '## Carryover Decisions' section ($_carryover_count carryover(s) to reconcile)"
        fi
    fi

    # 1c. v8.59.0 Layer T: Triage default-on (opt-out via EVOLVE_TRIAGE_DISABLE=1).
    # Soft WARN if cycle skipped Triage without explicit opt-out. First-rollout
    # is WARN-only so v8.58 cycles aren't retroactively blocked. Promote to fail
    # in v8.60+ after one verification cycle confirms orchestrator follows the
    # default-on instruction. Mirrors the v8.55 default-off→verify→default-on→
    # enforce ladder used for fan-out + budget-cap.
    if [ "${EVOLVE_TRIAGE_DISABLE:-0}" != "1" ]; then
        if [ ! -f "$WORKSPACE/triage-decision.md" ]; then
            log "WARN: Triage default-on (v8.59.0+) but triage-decision.md missing in workspace — orchestrator skipped Layer C"
            log "  → set EVOLVE_TRIAGE_DISABLE=1 if intentional; otherwise check agents/evolve-orchestrator.md PASS branch"
        else
            log "OK: triage-decision.md present (Triage ran)"
        fi
    fi

    # 2. At least one eval definition must exist
    local eval_count
    eval_count=$(ls "$EVOLVE_DIR/evals/"*.md 2>/dev/null | wc -l | tr -d ' ')
    [ "$eval_count" -gt 0 ] || fail "No eval definitions found in $EVOLVE_DIR/evals/"
    log "OK: $eval_count eval definition(s) found"

    # 3. Run eval quality check on new evals
    if [ -f "scripts/verification/eval-quality-check.sh" ]; then
        for eval_file in "$EVOLVE_DIR/evals/"*.md; do
            local result
            result=$(bash scripts/verification/eval-quality-check.sh "$eval_file" 2>&1) || {
                local exit_code=$?
                if [ "$exit_code" -eq 2 ]; then
                    anomaly "Level 0 eval commands in $eval_file — possible specification gaming"
                fi
                fail "eval-quality-check flagged $eval_file (exit $exit_code)"
            }
        done
        log "OK: Eval quality checks passed"
    fi

    # 4. Capture eval checksums for tamper detection
    if command -v sha256sum &>/dev/null; then
        sha256sum "$EVOLVE_DIR/evals/"*.md > "$WORKSPACE/eval-checksums.json"
    elif command -v shasum &>/dev/null; then
        shasum -a 256 "$EVOLVE_DIR/evals/"*.md > "$WORKSPACE/eval-checksums.json"
    fi
    log "OK: Eval checksums captured"

    # 5. Mutation testing — verify NEW eval files (created during this cycle) are rigorous.
    # Scopes to files newer than $WORKSPACE/.cycle-start-marker (set by run-cycle.sh
    # at cycle init). Pre-cycle-21 used `git ls-files --others --exclude-standard`,
    # but .evolve/* is gitignored — that filter always returned empty (cycle-19 WARN).
    # Removing --exclude-standard returns all 277 existing evals — also wrong.
    # find -newer is precise: only files mtime > marker, i.e. created this cycle.
    # Threshold: 0.7 (WARN-only this rollout; escalate to FAIL after one verification cycle).
    # Opt-out: EVOLVE_MUTATION_CHECK_DISABLE=1.
    if [ -x "scripts/verification/mutate-eval.sh" ] && [ "${EVOLVE_MUTATION_CHECK_DISABLE:-0}" != "1" ]; then
        local _new_evals _mutation_warnings _marker
        _mutation_warnings=0
        _marker="$WORKSPACE/.cycle-start-marker"
        if [ ! -f "$_marker" ]; then
            log "WARN: cycle-start marker missing ($_marker) — mutation gate cannot scope to new evals; skipping"
            _new_evals=""
        else
            _new_evals=$(find "${EVOLVE_DIR}/evals" -name '*.md' -newer "$_marker" -type f 2>/dev/null || true)
        fi
        if [ -z "$_new_evals" ]; then
            log "OK: No new eval files (newer than cycle-start) — mutation pre-flight skipped"
        else
            while IFS= read -r eval_file; do
                [ -f "$eval_file" ] || continue
                local mut_out mut_rc
                mut_out=$(bash scripts/verification/mutate-eval.sh "$eval_file" --threshold 0.7 2>&1)
                mut_rc=$?
                case "$mut_rc" in
                    0) log "OK: $eval_file kill rate ≥ 0.7 — eval is rigorous" ;;
                    1)
                        log "WARN: $eval_file kill rate below 0.7 — Auditor must verify behavioral coverage (rollout: WARN-only)"
                        _mutation_warnings=$((_mutation_warnings + 1)) ;;
                    2)
                        log "WARN: $eval_file mutation testing inconclusive (no inferable source files)" ;;
                    127)
                        log "WARN: mutate-eval.sh missing required binary; skipping mutation pass" ;;
                esac
            done <<EOF
$_new_evals
EOF
            if [ "$_mutation_warnings" -gt 0 ]; then
                # v10.2.0: EVOLVE_MUTATION_GATE_STRICT=1 promotes WARN to FAIL.
                if [ "${EVOLVE_MUTATION_GATE_STRICT:-0}" = "1" ]; then
                    log "MUTATION-FAIL: $_mutation_warnings new eval(s) failed mutation testing — gate FAIL (EVOLVE_MUTATION_GATE_STRICT=1)"
                    return 1
                else
                    log "MUTATION-WARN: $_mutation_warnings new eval(s) failed mutation testing (rollout: WARN-only; set EVOLVE_MUTATION_GATE_STRICT=1 to FAIL)"
                fi
            else
                log "OK: All new evals passed mutation testing (kill rate ≥ 0.7)"
            fi
        fi
    fi

    # C2-handoff-schemas: scout-report schema violations fail the gate (C4 enforced)
    if [ -x "scripts/tests/validate-handoff-artifact.sh" ]; then
        local _schema_out
        _schema_out=$(bash scripts/tests/validate-handoff-artifact.sh \
            --artifact "$WORKSPACE/scout-report.md" --type scout \
            --state "${STATE:-}" 2>&1) || true
        if [ -z "$_schema_out" ]; then
            log "OK: scout-report.md passes handoff schema (C2)"
        else
            fail "scout-report.md schema violations (C4 enforcement): $_schema_out"
        fi
    fi

    log "PASS: DISCOVER → BUILD gate"
}

# ─── Gate: DISCOVER → TRIAGE (v8.56.0 Layer C, opt-in) ───
# Triage is a single-writer phase that picks the top-N items from scout-report
# backlog + carryoverTodos, defers the rest, and surfaces large items as
# requiring split. It runs between Scout and Plan-review when
# default-on as of v8.59.0 (was opt-in EVOLVE_TRIAGE_ENABLED=1 in v8.56-v8.58); legacy
# discover→build paths still work).
gate_discover_to_triage() {
    log "Checking DISCOVER → TRIAGE gate for cycle $CYCLE"

    check_file_exists "$WORKSPACE/scout-report.md" "Scout report"
    check_file_fresh "$WORKSPACE/scout-report.md" "Scout report"
    check_artifact_substance "$WORKSPACE/scout-report.md" "Scout report"

    # Don't require any extra ledger entry — Scout's ledger entry already
    # exists from the discover→build path. We just need the scout-report
    # to be readable input for Triage.
    log "PASS: DISCOVER → TRIAGE gate"
}

# ─── Gate: TRIAGE → PLAN-REVIEW (v8.56.0 Layer C) ───
# Requires triage-decision.md fresh + substantive + a top_n list (even if
# 0 — that's a valid signal that Triage decided not to ship anything this
# cycle, in which case the cycle should be closed without entering plan-review).
gate_triage_to_plan_review() {
    log "Checking TRIAGE → PLAN-REVIEW gate for cycle $CYCLE"

    check_file_exists "$WORKSPACE/triage-decision.md" "Triage decision"
    check_file_fresh "$WORKSPACE/triage-decision.md" "Triage decision"
    check_artifact_substance "$WORKSPACE/triage-decision.md" "Triage decision"

    if ! grep -q '"role":"triage"' "$LEDGER" 2>/dev/null; then
        fail "No triage ledger entry for cycle $CYCLE"
    fi

    # cycle_size_estimate=large means Triage is asking for a split. Block.
    local size
    size=$(awk '/^cycle_size_estimate:/ { gsub(/^[^:]*:[[:space:]]*/, ""); gsub(/[[:space:]].*/, ""); print tolower($0); exit }' "$WORKSPACE/triage-decision.md")
    case "$size" in
        small|medium)
            log "OK: cycle_size_estimate=$size"
            ;;
        large)
            fail "Triage flagged cycle_size_estimate=large — split required (do not advance to plan-review with this scope). Defer items until top_n is small/medium."
            ;;
        "")
            fail "triage-decision.md missing 'cycle_size_estimate:' line"
            ;;
        *)
            fail "Unrecognized cycle_size_estimate: '$size' (expected small|medium|large)"
            ;;
    esac

    log "PASS: TRIAGE → PLAN-REVIEW gate"
}

# ─── Gate: PLAN-REVIEW → TDD (Sprint 2) ───
# Requires plan-review.md fresh + substantive + verdict != ABORT.
# Verdict semantics:
#   PROCEED — orchestrator routes cycle to TDD phase
#   REVISE  — orchestrator returns to Scout (max 2 retries; tracked in cycle-state)
#   ABORT   — gate fails the cycle; ship.sh would refuse anyway
gate_plan_review_to_tdd() {
    log "Checking PLAN-REVIEW → TDD gate for cycle $CYCLE"

    check_file_exists "$WORKSPACE/plan-review.md" "Plan-review report"
    check_file_fresh "$WORKSPACE/plan-review.md" "Plan-review report"
    check_artifact_substance "$WORKSPACE/plan-review.md" "Plan-review report"

    # Look for ledger entry from plan-reviewer (either as agent_subprocess or
    # agent_fanout from dispatch-parallel).
    if ! grep -q '"role":"plan-reviewer"' "$LEDGER" 2>/dev/null; then
        fail "No plan-reviewer ledger entry for cycle $CYCLE"
    fi

    # First content line must be 'Verdict: <X>'.
    local verdict
    verdict=$(awk 'tolower($0) ~ /^verdict:/ { gsub(/^[^:]*:[[:space:]]*/, ""); gsub(/[[:space:]].*/, ""); print toupper($0); exit }' "$WORKSPACE/plan-review.md")
    case "$verdict" in
        PROCEED)
            log "OK: Plan-review verdict PROCEED"
            ;;
        REVISE)
            fail "Plan-review verdict REVISE — orchestrator should re-run Scout (not advance to TDD)"
            ;;
        ABORT)
            fail "Plan-review verdict ABORT — cycle should end (do not advance to TDD)"
            ;;
        *)
            fail "Plan-review verdict missing or unrecognized: '$verdict'"
            ;;
    esac

    log "PASS: PLAN-REVIEW → TDD gate"
}

# ─── Abnormal event appender (v11.0) ───
# Writes a structured event to $WORKSPACE/abnormal-events.jsonl.
# Args: event_type, details, remediation_hint
# Source phase is hard-coded as "phase-gate".
_append_abnormal_event() {
    local _et="$1" _det="$2" _rem="$3"
    [ -d "${WORKSPACE:-}" ] || return 0
    local _ts; _ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    local _det_esc; _det_esc=$(printf '%s' "$_det" | sed 's/"/\\"/g')
    local _rem_esc; _rem_esc=$(printf '%s' "$_rem" | sed 's/"/\\"/g')
    printf '{"event_type":"%s","timestamp":"%s","source_phase":"phase-gate","severity":"WARN","details":"%s","remediation_hint":"%s"}\n' \
        "$_et" "$_ts" "$_det_esc" "$_rem_esc" >> "$WORKSPACE/abnormal-events.jsonl" 2>/dev/null || true
}

# ─── Builder isolation breach detector (v8.N) ───
# Scans PROJECT_ROOT sensitive directories for files newer than the scout-report
# (written before the build phase). Files newer than scout-report were written
# during the build phase; if they land in sensitive PROJECT_ROOT dirs instead of
# the worktree, it is a builder isolation breach.
#
# EVOLVE_BUILDER_ISOLATION_CHECK=0  — set to 0 to disable detection (default ON)
# EVOLVE_BUILDER_ISOLATION_STRICT=0 — set to 0 for warn-only mode (default ON: fail the gate)
#
# Uses `git diff --quiet HEAD` against EVOLVE_PROJECT_ROOT to detect any
# uncommitted tracked-file modifications during the build phase — covers ALL
# paths, not just evals/instincts. Operator can set CHECK=0 to disable or
# STRICT=0 for warn-only mode.
_check_builder_isolation_breach() {
    [ "${EVOLVE_BUILDER_ISOLATION_CHECK:-1}" = "1" ] || return 0

    local breach_output
    breach_output=$(git -C "${EVOLVE_PROJECT_ROOT:-.}" diff HEAD --name-only 2>/dev/null || true)

    if [ -n "$breach_output" ]; then
        log "WARN[builder-isolation-breach]: Builder wrote to PROJECT_ROOT tracked files during build phase:"
        log "  Modified: $breach_output"
        if command -v jq >/dev/null 2>&1 && [ -f "${LEDGER:-}" ]; then
            local ts
            ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
            local entry
            entry=$(jq -nc \
                --arg ts "$ts" \
                --argjson cycle "${CYCLE:-0}" \
                --arg stray "$breach_output" \
                '{ts:$ts,cycle:$cycle,kind:"gate-observation",classification:"builder-isolation-breach",stray_files:$stray}')
            printf '%s\n' "$entry" >> "$LEDGER" 2>/dev/null || true
        fi
        # v11.0 T3: Wire to abnormal-events.jsonl so retro pipeline sees breach events.
        _append_abnormal_event "builder-isolation-breach" \
            "Builder wrote to PROJECT_ROOT tracked files: $breach_output" \
            "Ensure Builder edits only the per-cycle worktree, not PROJECT_ROOT directly"
        if [ "${EVOLVE_BUILDER_ISOLATION_STRICT:-1}" = "1" ]; then
            fail "Builder isolation breach: uncommitted tracked-file modifications on PROJECT_ROOT main branch during build phase (set EVOLVE_BUILDER_ISOLATION_STRICT=0 for warn-only)"
        fi
    else
        log "OK: No builder isolation breach detected (git diff HEAD is clean)"
    fi
}

# ─── Builder cost-overrun guard (v8.60+) ───
# Reads builder-usage.json total_cost_usd against a threshold and emits an
# audit-visible WARN when exceeded. Default mode: non-blocking (WARN only).
# Set EVOLVE_BUILDER_COST_GUARD_STRICT=1 to fail-fast on overrun.
_check_builder_cost_overrun() {
    local usage_file="$WORKSPACE/builder-usage.json"
    if [ ! -f "$usage_file" ]; then
        log "SKIP: builder-usage.json not found — cost-overrun guard inactive"
        return 0
    fi
    local actual_cost threshold
    actual_cost=$(jq -r '.total_cost_usd // 0' "$usage_file" 2>/dev/null || echo 0)
    if [ -n "${EVOLVE_MAX_BUDGET_USD:-}" ]; then
        threshold="$EVOLVE_MAX_BUDGET_USD"
    elif [ -n "${EVOLVE_BUILDER_COST_THRESHOLD:-}" ]; then
        threshold="$EVOLVE_BUILDER_COST_THRESHOLD"
    else
        threshold="2.00"
    fi
    local overrun
    overrun=$(echo "$actual_cost > $threshold" | bc -l 2>/dev/null || echo 0)
    if [ "${overrun}" = "1" ]; then
        if [ "${EVOLVE_BUILDER_COST_GUARD_STRICT:-0}" = "1" ]; then
            fail "Builder cost overrun: \$$actual_cost > threshold \$$threshold (EVOLVE_BUILDER_COST_GUARD_STRICT=1)"
        else
            echo "[phase-gate] WARN: Builder cost overrun: \$$actual_cost > threshold \$$threshold. Set EVOLVE_BUILDER_COST_GUARD_STRICT=1 to fail-fast." >&2
            printf '\n## Cost Guard Warning\nBuilder spent $%s vs threshold $%s. Review for scope creep.\n' "$actual_cost" "$threshold" >> "$WORKSPACE/build-report.md"
        fi
    else
        log "OK: Builder cost within threshold (\$$actual_cost <= \$$threshold)"
    fi
}

# ─── Gate: BUILD → TESTER (v10.3.0+) ───
# Verifies build-report.md exists before the Tester phase can run.
gate_build_to_tester() {
    log "Checking BUILD → TESTER gate for cycle $CYCLE"
    check_file_exists "$WORKSPACE/build-report.md" "Build report"
    if grep -qi "Status:.*FAIL\|## Status.*FAIL" "$WORKSPACE/build-report.md"; then
        fail "Build report indicates FAIL — cannot proceed to tester phase"
    fi
    log "PASS: BUILD → TESTER gate"
}

# ─── Gate: TESTER → AUDIT (v10.3.0+) ───
# Verifies tester-report.md exists before the Audit phase can run.
gate_tester_to_audit() {
    log "Checking TESTER → AUDIT gate for cycle $CYCLE"
    check_file_exists "$WORKSPACE/tester-report.md" "Tester report"
    check_artifact_substance "$WORKSPACE/tester-report.md" "Tester report"
    log "PASS: TESTER → AUDIT gate"
}

# ─── Dynamic gate dispatcher (v10.3.0+, cycle 57) ───
# Dispatches a gate function by name. Uses 'type' (bash 3.2 compat) to verify
# the function exists before calling it. Called from list-phase-order.sh driven
# dispatch in the orchestrator (EVOLVE_USE_PHASE_REGISTRY=1 path).
#
# Usage: gate_run_by_name <gate_name> [cycle] [workspace]
# Exit: 0 on PASS, 1 on FAIL, 2 on function-not-found.
gate_run_by_name() {
    local gate_name="${1:-}"
    # Optional overrides (pass extra cycle/workspace context when called standalone)
    local _override_cycle="${2:-}"
    local _override_ws="${3:-}"
    if [ -z "$gate_name" ]; then
        echo "[gate_run_by_name] ERROR: gate_name required" >&2
        return 2
    fi
    # bash 3.2 compat: 'type NAME | grep -q function' (not 'declare -f NAME')
    if ! type "$gate_name" 2>/dev/null | grep -q "function"; then
        echo "[gate_run_by_name] WARN: gate '$gate_name' not declared — treating as no-op PASS" >&2
        return 0
    fi
    if [ -n "$_override_cycle" ]; then
        CYCLE="$_override_cycle"
    fi
    if [ -n "$_override_ws" ]; then
        WORKSPACE="$_override_ws"
    fi
    "$gate_name"
}

# ─── Gate: BUILD → AUDIT ───
gate_build_to_audit() {
    log "Checking BUILD → AUDIT gate for cycle $CYCLE"

    # 1. Build report must exist, be fresh, and have substantive content
    check_file_exists "$WORKSPACE/build-report.md" "Build report"
    check_file_fresh "$WORKSPACE/build-report.md" "Build report"
    check_artifact_substance "$WORKSPACE/build-report.md" "Build report"
    check_subagent_ledger_match "builder"

    # 2. Build report must say PASS (not FAIL)
    if grep -qi "Status:.*FAIL\|## Status.*FAIL" "$WORKSPACE/build-report.md"; then
        fail "Build report indicates FAIL — cannot proceed to audit"
    fi
    log "OK: Build report status is not FAIL"

    # 3. Challenge token must appear in build report
    if [ -n "${CHALLENGE_TOKEN:-}" ]; then
        if ! grep -q "$CHALLENGE_TOKEN" "$WORKSPACE/build-report.md"; then
            anomaly "Challenge token missing from build-report.md — possible report forgery"
        fi
        log "OK: Challenge token present in build report"
    fi

    # 4. Builder cost-overrun guard (v8.60+)
    _check_builder_cost_overrun

    # 4b. Builder isolation breach detector (v10.6+, default ON; set EVOLVE_BUILDER_ISOLATION_CHECK=0 to disable)
    _check_builder_isolation_breach

    # 5. Mutation testing on Builder-created eval files (cycle-19 + cycle-21 fix).
    # Same approach as gate_discover_to_build step 5: find -newer than the
    # .cycle-start-marker that run-cycle.sh creates at cycle init. This catches
    # both Scout-created and Builder-created evals; the redundancy with the
    # discover-to-build pass is acceptable (typical cycle adds 0-2 evals).
    # Threshold: 0.7 WARN-only. Opt-out: EVOLVE_MUTATION_CHECK_DISABLE=1.
    if [ -x "scripts/verification/mutate-eval.sh" ] && [ "${EVOLVE_MUTATION_CHECK_DISABLE:-0}" != "1" ]; then
        local _new_build_evals _build_mut_warnings _b_marker
        _build_mut_warnings=0
        _b_marker="$WORKSPACE/.cycle-start-marker"
        if [ ! -f "$_b_marker" ]; then
            log "WARN: cycle-start marker missing ($_b_marker) — build-to-audit mutation gate cannot scope to new evals; skipping"
            _new_build_evals=""
        else
            _new_build_evals=$(find "${EVOLVE_DIR}/evals" -name '*.md' -newer "$_b_marker" -type f 2>/dev/null || true)
        fi
        if [ -z "$_new_build_evals" ]; then
            log "OK: No new eval files from build — mutation check skipped"
        else
            while IFS= read -r eval_file; do
                [ -f "$eval_file" ] || continue
                local b_mut_out b_mut_rc
                b_mut_out=$(bash scripts/verification/mutate-eval.sh "$eval_file" --threshold 0.7 2>&1)
                b_mut_rc=$?
                case "$b_mut_rc" in
                    0) log "OK: $eval_file kill rate ≥ 0.7" ;;
                    1)
                        log "WARN: $eval_file kill rate below 0.7 — tautological grader risk (Auditor must flag)"
                        _build_mut_warnings=$((_build_mut_warnings + 1)) ;;
                    2)
                        log "WARN: $eval_file mutation inconclusive (no inferable source files)" ;;
                    127)
                        log "WARN: mutate-eval.sh binary missing; build-to-audit mutation check skipped" ;;
                esac
            done <<EOF
$_new_build_evals
EOF
            if [ "$_build_mut_warnings" -gt 0 ]; then
                # v10.2.0: EVOLVE_MUTATION_GATE_STRICT=1 promotes build-to-audit
                # mutation WARN to FAIL. Default OFF preserves WARN-only.
                if [ "${EVOLVE_MUTATION_GATE_STRICT:-0}" = "1" ]; then
                    log "MUTATION-FAIL: $_build_mut_warnings Builder eval(s) below 0.7 kill rate — gate FAIL (EVOLVE_MUTATION_GATE_STRICT=1)"
                    return 1
                else
                    log "MUTATION-WARN: $_build_mut_warnings Builder eval(s) below 0.7 kill rate — Auditor see above (set EVOLVE_MUTATION_GATE_STRICT=1 to FAIL)"
                fi
            else
                log "OK: All Builder evals passed mutation testing (kill rate ≥ 0.7)"
            fi
        fi
    fi

    # C2-handoff-schemas: build-report schema violations fail the gate (C5 enforced)
    if [ -x "scripts/tests/validate-handoff-artifact.sh" ]; then
        local _schema_out
        _schema_out=$(bash scripts/tests/validate-handoff-artifact.sh \
            --artifact "$WORKSPACE/build-report.md" --type build 2>&1) || true
        if [ -z "$_schema_out" ]; then
            log "OK: build-report.md passes handoff schema (C2)"
        else
            fail "build-report.md schema violations (C5 enforcement): $_schema_out"
        fi
    fi

    # 6. Validate-predicate sweep: WARN on NOT_EXECUTABLE predicates in acs/cycle-N/
    # Non-blocking WARN (logs to stderr + visible to Auditor). Closure for cycle-55 audit H-3.
    if [ -x "scripts/verification/validate-predicate.sh" ]; then
        local _pred_file _pred_validate_out _pred_validate_rc
        while IFS= read -r _pred_file; do
            [ -f "$_pred_file" ] || continue
            _pred_validate_out=$(bash scripts/verification/validate-predicate.sh "$_pred_file" 2>&1)
            _pred_validate_rc=$?
            if [ "$_pred_validate_rc" -ne 0 ]; then
                echo "[phase-gate] WARN: predicate validation failed for $_pred_file: $_pred_validate_out" >&2
            fi
        done < <(find "${EVOLVE_PROJECT_ROOT:-.}/acs/cycle-${CYCLE}" -maxdepth 1 -name "*.sh" -type f 2>/dev/null | sort)
    fi

    log "PASS: BUILD → AUDIT gate"
}

# ─── Gate: AUDIT → SHIP ───
gate_audit_to_ship() {
    log "Checking AUDIT → SHIP gate for cycle $CYCLE"

    # 0. Anti-forgery checks (added after Gemini forgery incident)
    check_no_forgery_scripts
    verify_state_checksum
    check_git_diff_substance

    # 1. Audit report must exist, be fresh, and have substantive content
    check_file_exists "$WORKSPACE/audit-report.md" "Audit report"
    check_file_fresh "$WORKSPACE/audit-report.md" "Audit report"
    check_artifact_substance "$WORKSPACE/audit-report.md" "Audit report"
    check_subagent_ledger_match "auditor"

    # 2. Audit verdict must be PASS (not WARN or FAIL)
    if grep -qi "Verdict:.*FAIL\|## Verdict.*FAIL" "$WORKSPACE/audit-report.md"; then
        # v8.58.0 Layer E1: write .cycle-verdict before failing so dispatcher
        # forensics can see what verdict was observed even when the gate aborts.
        echo "FAIL" > "$WORKSPACE/.cycle-verdict"
        fail "Audit verdict is FAIL — cannot ship"
    fi
    if grep -qi "Verdict:.*WARN\|## Verdict.*WARN" "$WORKSPACE/audit-report.md"; then
        echo "WARN" > "$WORKSPACE/.cycle-verdict"
        fail "Audit verdict is WARN — MEDIUM+ issues block shipping"
    fi
    # v8.58.0 Layer E1: write .cycle-verdict=PASS as the canonical signal that
    # downstream gates (gate_ship_to_learn) and the dispatcher's verify_cycle
    # consume to enforce Layer P (memo on PASS).
    echo "PASS" > "$WORKSPACE/.cycle-verdict"
    log "OK: Audit verdict is PASS"

    # 3. Independent eval verification (CRITICAL — this is the main anti-cheating gate)
    if [ -f "scripts/verification/verify-eval.sh" ]; then
        log "Running independent eval verification..."
        local verify_result
        verify_result=$(bash scripts/verification/verify-eval.sh "$EVOLVE_DIR/evals" "$WORKSPACE" 2>&1) || {
            local exit_code=$?
            fail "Independent eval verification FAILED (exit $exit_code): $verify_result"
        }
        log "OK: Independent eval verification PASSED"
    else
        log "WARN: verify-eval.sh not found — skipping independent verification"
    fi

    # 4. Cycle health check (11-signal fingerprint)
    if [ -f "scripts/observability/cycle-health-check.sh" ]; then
        log "Running cycle health check..."
        local health_result
        health_result=$(bash scripts/observability/cycle-health-check.sh "$CYCLE" "$WORKSPACE" "$EVOLVE_DIR" 2>&1) || {
            local exit_code=$?
            if [ "$exit_code" -eq 2 ]; then
                anomaly "Health check detected ANOMALY: $health_result"
            fi
            fail "Health check FAILED (exit $exit_code)"
        }
        log "OK: Cycle health check PASSED"
    fi

    # 4b. E2E artifact check (only when evals reference playwright)
    # Narrow guard: if any eval file declares an E2E Graders section or playwright
    # command, require that the Builder produced real e2e artifacts. Non-UI
    # cycles are unaffected because the grep returns no match.
    if grep -rql -e '## E2E Graders' -e 'playwright' "$EVOLVE_DIR/evals/" 2>/dev/null; then
        local e2e_report="playwright-report/index.html"
        local e2e_verification_documented="no"
        if grep -q '## E2E Verification' "$WORKSPACE/build-report.md" 2>/dev/null; then
            e2e_verification_documented="yes"
        fi
        if [ ! -s "$e2e_report" ] && [ "$e2e_verification_documented" = "no" ]; then
            fail "Eval references playwright but no e2e artifacts found ($e2e_report missing/empty) and build-report.md lacks '## E2E Verification' section"
        fi
        # If artifacts are missing but Builder explicitly documented SKIPPED with reason, allow through as WARN.
        if [ ! -s "$e2e_report" ] && [ "$e2e_verification_documented" = "yes" ]; then
            if ! grep -qE 'SKIPPED.*reason' "$WORKSPACE/build-report.md"; then
                fail "E2E Verification section present but status is not PASS or SKIPPED-with-reason, and no playwright-report/index.html found"
            fi
            log "WARN: E2E Verification marked SKIPPED with reason — allowing ship"
        else
            log "OK: E2E artifacts present or documented"
        fi
    fi

    # 5. Eval checksum integrity (detect tampering)
    if [ -f "$WORKSPACE/eval-checksums.json" ]; then
        if command -v sha256sum &>/dev/null; then
            sha256sum -c "$WORKSPACE/eval-checksums.json" --quiet 2>/dev/null || \
                anomaly "Eval file checksums don't match — possible eval tampering"
        elif command -v shasum &>/dev/null; then
            shasum -a 256 -c "$WORKSPACE/eval-checksums.json" --quiet 2>/dev/null || \
                anomaly "Eval file checksums don't match — possible eval tampering"
        fi
        log "OK: Eval checksums match"
    fi

    # 6. Optional advisory code-review pass (EVOLVE_AUDIT_ADVISORY_REVIEW=1, default OFF)
    # Runs AFTER verdict is bound — purely informational; does not affect ship decision.
    if [ "${EVOLVE_AUDIT_ADVISORY_REVIEW:-0}" = "1" ]; then
        if [ -f "scripts/lifecycle/audit-advisory-review.sh" ]; then
            log "Running advisory code-review pass (EVOLVE_AUDIT_ADVISORY_REVIEW=1)..."
            bash scripts/lifecycle/audit-advisory-review.sh "$CYCLE" "$WORKSPACE" 2>/dev/null || true
            log "OK: Advisory pass complete (result in $WORKSPACE/audit-advisory-review.md)"
        else
            log "WARN: audit-advisory-review.sh not found; advisory pass skipped"
        fi
    fi

    # C2-handoff-schemas: audit-report schema violations fail the gate (C5 enforced)
    if [ -x "scripts/tests/validate-handoff-artifact.sh" ]; then
        local _schema_out
        _schema_out=$(bash scripts/tests/validate-handoff-artifact.sh \
            --artifact "$WORKSPACE/audit-report.md" --type audit 2>&1) || true
        if [ -z "$_schema_out" ]; then
            log "OK: audit-report.md passes handoff schema (C2)"
        else
            fail "audit-report.md schema violations (C5 enforcement): $_schema_out"
        fi
    fi

    log "PASS: AUDIT → SHIP gate"
}

# ─── Gate: AUDIT → RETROSPECTIVE (v8.45.0+) ───
# Allows the orchestrator to advance to the retrospective phase after a FAIL or
# WARN audit verdict. Mirrors gate_audit_to_ship's anti-forgery + audit-report
# verification, but accepts FAIL/WARN verdicts (PASS goes to ship instead).
gate_audit_to_retrospective() {
    log "Checking AUDIT → RETROSPECTIVE gate for cycle $CYCLE"

    # Anti-forgery checks (same as audit-to-ship)
    check_no_forgery_scripts
    verify_state_checksum
    check_git_diff_substance

    # Audit report must exist + match ledger SHA + have substantive content
    check_file_exists "$WORKSPACE/audit-report.md" "Audit report"
    check_file_fresh "$WORKSPACE/audit-report.md" "Audit report"
    check_artifact_substance "$WORKSPACE/audit-report.md" "Audit report"
    check_subagent_ledger_match "auditor"

    # Verdict must be FAIL or WARN (PASS uses gate_audit_to_ship), UNLESS
    # abnormal-events.jsonl is non-empty — PASS cycles with abnormal events
    # must also run retrospective to capture structured lessons (v46+).
    local _has_abnormal=0
    if [ -s "$WORKSPACE/abnormal-events.jsonl" ]; then
        _has_abnormal=1
        log "INFO: abnormal-events.jsonl non-empty — allowing retro even on PASS verdict"
    fi
    if grep -qiE "Verdict:[[:space:]]*\*?\*?[[:space:]]*PASS|## Verdict[[:space:]]*\*?\*?[[:space:]]*PASS" "$WORKSPACE/audit-report.md"; then
        # PASS without FAIL/WARN — wrong gate unless abnormal events present
        if ! grep -qiE "Verdict:.*FAIL|Verdict:.*WARN|## Verdict.*FAIL|## Verdict.*WARN" "$WORKSPACE/audit-report.md"; then
            if [ "$_has_abnormal" -eq 0 ]; then
                fail "Audit verdict is PASS — use audit-to-ship gate, not audit-to-retrospective (no abnormal events present)"
            fi
            # PASS + abnormal events: allow retro and set a WARN-class verdict marker
            echo "PASS-WITH-ABNORMAL" > "$WORKSPACE/.cycle-verdict"
            log "OK: PASS verdict with non-empty abnormal-events.jsonl — retrospective triggered"
            log "OK: AUDIT → RETROSPECTIVE gate passed (abnormal-events-triggered)"
            return 0
        fi
    fi
    if ! grep -qiE "Verdict:.*FAIL|Verdict:.*WARN|## Verdict.*FAIL|## Verdict.*WARN" "$WORKSPACE/audit-report.md"; then
        fail "Audit verdict not FAIL or WARN — retrospective requires a failure-class verdict (or non-empty abnormal-events.jsonl)"
    fi
    # v8.58.0 Layer E1: write .cycle-verdict so downstream gates know which
    # failure-class verdict was observed (FAIL vs WARN). Disambiguates so memo
    # enforcement can be skipped on non-PASS cycles.
    if grep -qiE "Verdict:.*FAIL|## Verdict.*FAIL" "$WORKSPACE/audit-report.md"; then
        echo "FAIL" > "$WORKSPACE/.cycle-verdict"
    else
        echo "WARN" > "$WORKSPACE/.cycle-verdict"
    fi
    log "OK: Audit verdict is FAIL or WARN — retrospective phase appropriate"

    # C3-handoff-schemas: validate handoff-auditor.json sidecar (WARN-only; C5 promotes to FAIL)
    local _handoff_json="$WORKSPACE/handoff-auditor.json"
    if [ ! -f "$_handoff_json" ]; then
        log "WARN: handoff-auditor.json missing — auditor did not emit JSON sidecar (C3)"
    else
        if ! jq empty "$_handoff_json" 2>/dev/null; then
            log "WARN: handoff-auditor.json is not valid JSON (C3)"
        else
            local _verdict_json; _verdict_json=$(jq -r '.verdict // empty' "$_handoff_json" 2>/dev/null)
            if [ -z "$_verdict_json" ]; then
                log "WARN: handoff-auditor.json missing required field 'verdict' (C3)"
            else
                log "OK: handoff-auditor.json present and valid (verdict=$_verdict_json)"
            fi
        fi
    fi

    log "OK: AUDIT → RETROSPECTIVE gate passed"
}

# ─── Gate: RETROSPECTIVE → COMPLETE ───
gate_retrospective_to_complete() {
    log "Checking RETROSPECTIVE → COMPLETE gate for cycle $CYCLE"

    check_file_exists "$WORKSPACE/retrospective-report.md" "Retrospective report"
    check_file_fresh "$WORKSPACE/retrospective-report.md" "Retrospective report"
    check_artifact_substance "$WORKSPACE/retrospective-report.md" "Retrospective report"
    check_subagent_ledger_match "retrospective"

    local _handoff="$WORKSPACE/handoff-retrospective.json"
    check_file_exists "$_handoff" "handoff-retrospective.json"

    if ! jq empty "$_handoff" 2>/dev/null; then
        fail "handoff-retrospective.json is not valid JSON"
    fi

    # Verify lessonIds[] ↔ on-disk YAML 1:1 (INTEGRITY check — root cause of instinctSummary freeze)
    local _lesson_ids
    _lesson_ids=$(jq -r '.lessonIds[]? // empty' "$_handoff" 2>/dev/null)

    local _missing=0
    local _id
    for _id in $_lesson_ids; do
        local _found=0
        for _f in "$EVOLVE_DIR/instincts/lessons/${_id}"*.yaml; do
            [ -f "$_f" ] && _found=1 && break
        done
        if [ "$_found" -eq 0 ]; then
            log "INTEGRITY_FAIL: lessonId '$_id' has no YAML at $EVOLVE_DIR/instincts/lessons/${_id}*.yaml"
            _missing=$(( _missing + 1 ))
        else
            log "OK: lesson YAML exists for '$_id'"
        fi
    done

    if [ "$_missing" -gt 0 ]; then
        fail "INTEGRITY_FAIL: $_missing lesson YAML(s) in handoff-retrospective.json not found on disk — retrospective YAML write contract violated (see agents/evolve-retrospective.md Step 5)"
    fi

    log "OK: RETROSPECTIVE → COMPLETE gate passed (all lesson YAMLs verified on disk)"
}

# ─── Gate: SHIP → LEARN ───
gate_ship_to_learn() {
    log "Checking SHIP → LEARN gate for cycle $CYCLE"

    # 1. Git status should be clean (changes committed)
    local dirty
    dirty=$(git status --porcelain 2>/dev/null | head -5)
    if [ -n "$dirty" ]; then
        fail "Git working tree is dirty after ship — changes not committed"
    fi
    log "OK: Git working tree is clean"

    # v8.58.0 Layer E2: PASS-cycle memo enforcement. The v8.57 contract said
    # PASS cycles MUST emit carryover-todos.json via the memo subagent so the
    # next cycle's Scout sees the deferred backlog. The orchestrator persona
    # advised this but skipped it 3/3 times in production verification. The
    # only structural enforcement is here.
    #
    # Backward-compat: cycles without .cycle-verdict (pre-v8.58 fixtures) skip
    # this check. After v8.58 ships, gate_audit_to_ship always writes it.
    if [ -f "$WORKSPACE/.cycle-verdict" ]; then
        local _v
        _v=$(cat "$WORKSPACE/.cycle-verdict" 2>/dev/null)
        if [ "$_v" = "PASS" ]; then
            if ! grep -q "\"role\":\"memo\".*\"cycle\":$CYCLE\|\"cycle\":$CYCLE.*\"role\":\"memo\"" "$LEDGER" 2>/dev/null; then
                fail "PASS verdict but no memo ledger entry for cycle $CYCLE — orchestrator skipped Layer P (v8.57 contract violation; see agents/evolve-orchestrator.md PASS branch)"
            fi
            if [ ! -f "$WORKSPACE/carryover-todos.json" ]; then
                fail "PASS verdict + memo ran but $WORKSPACE/carryover-todos.json missing"
            fi
            log "OK: PASS cycle has memo ledger entry + carryover-todos.json"
        else
            log "OK: $_v cycle (memo not required on non-PASS verdicts)"
        fi
    else
        log "INFO: no .cycle-verdict file (pre-v8.58 cycle or audit-to-ship gate not run); skipping memo check"
    fi

    # 2. Increment state.json cycle number (SCRIPT does this, not LLM)
    if [ -f "$STATE" ]; then
        local current_cycle
        current_cycle=$(python3 -c "import json; print(json.load(open('$STATE'))['lastCycleNumber'])")
        if [ "$current_cycle" -ge "$CYCLE" ]; then
            log "OK: state.json lastCycleNumber already at $current_cycle"
        else
            python3 -c "
import json, datetime
with open('$STATE') as f:
    state = json.load(f)
state['lastCycleNumber'] = $CYCLE
state['version'] = state['version'] + 1
state['lastUpdated'] = datetime.datetime.now(datetime.UTC).strftime('%Y-%m-%dT%H:%M:%SZ')
with open('$STATE', 'w') as f:
    json.dump(state, f, indent=2)
print('state.json updated: lastCycleNumber=$CYCLE')
"
            log "OK: state.json lastCycleNumber updated to $CYCLE"
        fi
    fi

    log "PASS: SHIP → LEARN gate"
}

# ─── Gate: CYCLE COMPLETE ───
gate_cycle_complete() {
    log "Checking CYCLE COMPLETE gate for cycle $CYCLE"

    # 1. All 3 workspace artifacts must exist
    check_file_exists "$WORKSPACE/scout-report.md" "Scout report"
    check_file_exists "$WORKSPACE/build-report.md" "Build report"
    check_file_exists "$WORKSPACE/audit-report.md" "Audit report"

    # 2. Archive workspace to history
    local history_dir="$EVOLVE_DIR/history/cycle-$CYCLE"
    mkdir -p "$history_dir"
    cp "$WORKSPACE"/*.md "$history_dir/" 2>/dev/null
    log "OK: Workspace archived to $history_dir"

    # 3. Update mastery ONLY if audit genuinely passed
    if grep -qi "Verdict:.*PASS" "$WORKSPACE/audit-report.md" 2>/dev/null; then
        python3 -c "
import json
with open('$STATE') as f:
    state = json.load(f)
state['mastery']['consecutiveSuccesses'] = state['mastery'].get('consecutiveSuccesses', 0) + 1
cs = state['mastery']['consecutiveSuccesses']
if cs >= 6:
    state['mastery']['level'] = 'proficient'
elif cs >= 3:
    state['mastery']['level'] = 'competent'
state['ledgerSummary']['totalTasksShipped'] = state['ledgerSummary'].get('totalTasksShipped', 0) + 1
state['version'] = state['version'] + 1
with open('$STATE', 'w') as f:
    json.dump(state, f, indent=2)
print(f'Mastery updated: consecutiveSuccesses={cs}, level={state[\"mastery\"][\"level\"]}')
"
        log "OK: Mastery incremented (audit-verified PASS)"
    else
        python3 -c "
import json
with open('$STATE') as f:
    state = json.load(f)
state['mastery']['consecutiveSuccesses'] = 0
state['version'] = state['version'] + 1
with open('$STATE', 'w') as f:
    json.dump(state, f, indent=2)
print('Mastery RESET: audit did not PASS')
"
        log "WARN: Mastery reset — audit verdict was not PASS"
    fi

    log "PASS: Cycle $CYCLE complete"
}

# ─── Gate: CALIBRATE → INTENT (v8.19.0, opt-in) ───
#
# Fires only when cycle-state.intent_required==true (set at cycle init from
# EVOLVE_REQUIRE_INTENT env). Always passes structurally — its job is to
# *acknowledge* that the cycle is on the intent-enabled path. The real
# verification happens at gate_intent_to_research below.
gate_calibrate_to_intent() {
    log "Gate: CALIBRATE → INTENT (cycle $CYCLE)"
    local cycle_state="${EVOLVE_CYCLE_STATE_FILE:-$EVOLVE_PROJECT_ROOT/.evolve/cycle-state.json}"
    if [ ! -f "$cycle_state" ]; then
        log "WARN: cycle-state.json missing — gate passes (caller responsible)"
        return 0
    fi
    if ! command -v jq >/dev/null 2>&1; then
        log "WARN: jq missing — gate passes"
        return 0
    fi
    local ir
    ir=$(jq -r '.intent_required // false' "$cycle_state" 2>/dev/null)
    if [ "$ir" != "true" ]; then
        log "INFO: cycle has intent_required=$ir — gate not applicable, default flow"
        return 0
    fi
    log "OK: cycle is intent-enabled (intent_required=true)"
}

# ─── Gate: INTENT → RESEARCH (v8.19.0, opt-in) ───
#
# Verifies the structured intent.md the intent persona produced is sound:
#   - Exists in workspace
#   - Has YAML frontmatter with awn_class
#   - awn_class is NOT IBTC (out-of-scope short-circuit)
#   - challenged_premises has >= 1 entry
#   - Latest intent ledger entry SHA matches the on-disk file (no tampering)
#
# This is purely structural — no human approval needed. Autonomy is preserved.
gate_intent_to_research() {
    log "Gate: INTENT → RESEARCH (cycle $CYCLE)"
    local intent_file="$WORKSPACE/intent.md"
    [ -f "$intent_file" ] || fail "intent.md missing at $intent_file — intent persona did not produce artifact"

    # Extract YAML frontmatter (between first two --- lines)
    local fm
    fm=$(awk '/^---$/{n++; next} n==1' "$intent_file")
    [ -n "$fm" ] || fail "intent.md has no YAML frontmatter"

    # awn_class must be present and not IBTC
    local awn_class
    awn_class=$(printf '%s\n' "$fm" | awk -F': *' '/^awn_class: */ {print $2; exit}' | tr -d " '\"")
    [ -n "$awn_class" ] || fail "intent.md frontmatter missing awn_class field"
    case "$awn_class" in
        IMKI|IMR|IwE|CLEAR) ;;
        IBTC)
            fail "intent.md awn_class=IBTC (Instructions Beyond Tool Capabilities) — scope rejection. Cycle should not proceed; user goal is out of scope for this system."
            ;;
        *)
            fail "intent.md awn_class=$awn_class is invalid (must be IMKI|IMR|IwE|IBTC|CLEAR)"
            ;;
    esac

    # challenged_premises must have at least one entry. We count list items
    # that start with "- premise:" (canonical schema form).
    local premise_count
    premise_count=$(awk '/^challenged_premises:/{flag=1; next} /^[a-z_]+:/{flag=0} flag && /^[[:space:]]*- premise:/{n++} END{print n+0}' "$intent_file")
    [ "$premise_count" -ge 1 ] || fail "intent.md challenged_premises must have >= 1 entry (found $premise_count); per Karpathy + Socratic literature, premise-challenging is mandatory"

    log "OK: intent.md structure valid (awn_class=$awn_class, challenged_premises=$premise_count)"

    # SHA verification: latest intent ledger entry must match on-disk file
    if [ -f "$LEDGER" ] && command -v jq >/dev/null 2>&1; then
        local entry recorded_sha actual_sha
        entry=$(grep '"kind":"agent_subprocess"' "$LEDGER" 2>/dev/null \
            | jq -c --argjson cycle "$CYCLE" 'select(.cycle == $cycle and .role == "intent")' 2>/dev/null \
            | tail -1)
        if [ -n "$entry" ]; then
            recorded_sha=$(echo "$entry" | jq -r '.artifact_sha256')
            if command -v sha256sum >/dev/null 2>&1; then
                actual_sha=$(sha256sum "$intent_file" | awk '{print $1}')
            else
                actual_sha=$(shasum -a 256 "$intent_file" | awk '{print $1}')
            fi
            [ "$recorded_sha" = "$actual_sha" ] || fail "intent.md tampered post-write: ledger=$recorded_sha actual=$actual_sha"
            log "OK: intent.md SHA matches latest ledger entry ($recorded_sha)"
        else
            log "INFO: no intent ledger entry yet (legacy or pre-runner path)"
        fi
    fi
}

# ─── Dispatch ───
case "$GATE" in
    calibrate-to-intent)  gate_calibrate_to_intent ;;
    intent-to-research)   gate_intent_to_research ;;
    research-to-discover) gate_research_to_discover ;;
    discover-to-build)    gate_discover_to_build ;;
    discover-to-triage)   gate_discover_to_triage ;;
    triage-to-plan-review) gate_triage_to_plan_review ;;
    build-to-tester)      gate_build_to_tester ;;
    tester-to-audit)      gate_tester_to_audit ;;
    build-to-audit)       gate_build_to_audit ;;
    audit-to-ship)        gate_audit_to_ship ;;
    audit-to-retrospective) gate_audit_to_retrospective ;;
    retrospective-to-complete) gate_retrospective_to_complete ;;
    ship-to-learn)        gate_ship_to_learn ;;
    cycle-complete)       gate_cycle_complete ;;
    *)                    fail "Unknown gate: $GATE" ;;
esac
