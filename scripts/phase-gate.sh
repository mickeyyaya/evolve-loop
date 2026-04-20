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
        | grep -v 'scripts/phase-gate.sh' \
        | grep -v 'scripts/cycle-health-check.sh' \
        | grep -v 'scripts/verify-eval.sh' \
        | grep -v 'scripts/eval-quality-check.sh' \
        | grep -v 'scripts/setup-skill-inventory.sh' \
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

    log "PASS: DISCOVER → BUILD gate"
}

# ─── Gate: BUILD → AUDIT ───
gate_build_to_audit() {
    log "Checking BUILD → AUDIT gate for cycle $CYCLE"

    # 1. Build report must exist, be fresh, and have substantive content
    check_file_exists "$WORKSPACE/build-report.md" "Build report"
    check_file_fresh "$WORKSPACE/build-report.md" "Build report"
    check_artifact_substance "$WORKSPACE/build-report.md" "Build report"

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

    # 0. Anti-forgery checks (added after Gemini forgery incident)
    check_no_forgery_scripts
    verify_state_checksum
    check_git_diff_substance

    # 1. Audit report must exist, be fresh, and have substantive content
    check_file_exists "$WORKSPACE/audit-report.md" "Audit report"
    check_file_fresh "$WORKSPACE/audit-report.md" "Audit report"
    check_artifact_substance "$WORKSPACE/audit-report.md" "Audit report"

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
    research-to-discover) gate_research_to_discover ;;
    discover-to-build)    gate_discover_to_build ;;
    build-to-audit)       gate_build_to_audit ;;
    audit-to-ship)        gate_audit_to_ship ;;
    ship-to-learn)        gate_ship_to_learn ;;
    cycle-complete)       gate_cycle_complete ;;
    *)                    fail "Unknown gate: $GATE" ;;
esac
