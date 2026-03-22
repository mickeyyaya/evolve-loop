#!/usr/bin/env bash
#
# phase-gate.sh — Deterministic phase transition verification
#
# Runs between every phase transition in the evolve-loop. Verifies that
# required artifacts exist, agents actually ran, and integrity checks pass.
# The orchestrator (LLM) cannot skip this script because it's invoked by
# the host environment, not by the LLM itself.
#
# Usage: bash scripts/phase-gate.sh <gate> <cycle> <workspace_path>
#
# Gates:
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

GATE="${1:?Usage: phase-gate.sh <gate> <cycle> <workspace_path>}"
CYCLE="${2:?Missing cycle number}"
WORKSPACE="${3:?Missing workspace path}"
EVOLVE_DIR=".evolve"
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

# ─── Gate: DISCOVER → BUILD ───
gate_discover_to_build() {
    log "Checking DISCOVER → BUILD gate for cycle $CYCLE"

    # 1. Scout report must exist and be fresh
    check_file_exists "$WORKSPACE/scout-report.md" "Scout report"
    check_file_fresh "$WORKSPACE/scout-report.md" "Scout report"

    # 2. At least one eval definition must exist
    local eval_count
    eval_count=$(ls "$EVOLVE_DIR/evals/"*.md 2>/dev/null | wc -l | tr -d ' ')
    [ "$eval_count" -gt 0 ] || fail "No eval definitions found in $EVOLVE_DIR/evals/"
    log "OK: $eval_count eval definition(s) found"

    # 3. Run eval quality check on new evals
    if [ -f "scripts/eval-quality-check.sh" ]; then
        for eval_file in "$EVOLVE_DIR/evals/"*.md; do
            local result
            result=$(bash scripts/eval-quality-check.sh "$eval_file" 2>&1) || {
                local exit_code=$?
                if [ "$exit_code" -eq 2 ]; then
                    anomaly "Level 0 eval commands in $eval_file — possible specification gaming"
                fi
                log "WARN: eval-quality-check flagged $eval_file (exit $exit_code)"
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

    log "PASS: DISCOVER → BUILD gate"
}

# ─── Gate: BUILD → AUDIT ───
gate_build_to_audit() {
    log "Checking BUILD → AUDIT gate for cycle $CYCLE"

    # 1. Build report must exist and be fresh
    check_file_exists "$WORKSPACE/build-report.md" "Build report"
    check_file_fresh "$WORKSPACE/build-report.md" "Build report"

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

    log "PASS: BUILD → AUDIT gate"
}

# ─── Gate: AUDIT → SHIP ───
gate_audit_to_ship() {
    log "Checking AUDIT → SHIP gate for cycle $CYCLE"

    # 1. Audit report must exist and be fresh
    check_file_exists "$WORKSPACE/audit-report.md" "Audit report"
    check_file_fresh "$WORKSPACE/audit-report.md" "Audit report"

    # 2. Audit verdict must be PASS (not WARN or FAIL)
    if grep -qi "Verdict:.*FAIL\|## Verdict.*FAIL" "$WORKSPACE/audit-report.md"; then
        fail "Audit verdict is FAIL — cannot ship"
    fi
    if grep -qi "Verdict:.*WARN\|## Verdict.*WARN" "$WORKSPACE/audit-report.md"; then
        fail "Audit verdict is WARN — MEDIUM+ issues block shipping"
    fi
    log "OK: Audit verdict is PASS"

    # 3. Independent eval verification (CRITICAL — this is the main anti-cheating gate)
    if [ -f "scripts/verify-eval.sh" ]; then
        log "Running independent eval verification..."
        local verify_result
        verify_result=$(bash scripts/verify-eval.sh "$EVOLVE_DIR/evals" "$WORKSPACE" 2>&1) || {
            local exit_code=$?
            fail "Independent eval verification FAILED (exit $exit_code): $verify_result"
        }
        log "OK: Independent eval verification PASSED"
    else
        log "WARN: verify-eval.sh not found — skipping independent verification"
    fi

    # 4. Cycle health check (11-signal fingerprint)
    if [ -f "scripts/cycle-health-check.sh" ]; then
        log "Running cycle health check..."
        local health_result
        health_result=$(bash scripts/cycle-health-check.sh "$CYCLE" "$WORKSPACE" "$EVOLVE_DIR" 2>&1) || {
            local exit_code=$?
            if [ "$exit_code" -eq 2 ]; then
                anomaly "Health check detected ANOMALY: $health_result"
            fi
            fail "Health check FAILED (exit $exit_code)"
        }
        log "OK: Cycle health check PASSED"
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

    log "PASS: AUDIT → SHIP gate"
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

# ─── Dispatch ───
case "$GATE" in
    discover-to-build)   gate_discover_to_build ;;
    build-to-audit)      gate_build_to_audit ;;
    audit-to-ship)       gate_audit_to_ship ;;
    ship-to-learn)       gate_ship_to_learn ;;
    cycle-complete)      gate_cycle_complete ;;
    *)                   fail "Unknown gate: $GATE" ;;
esac
