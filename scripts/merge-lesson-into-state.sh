#!/usr/bin/env bash
#
# merge-lesson-into-state.sh — Orchestrator post-processor for the
# evolve-retrospective subagent. Reads handoff-retrospective.json from a
# cycle's workspace and merges its outputs into .evolve/state.json:
#
#   - Appends each new failure-lesson ID to state.json.instinctSummary[]
#     (so future Scout/Builder/Auditor agents see them in their context).
#   - Appends a structured failedApproaches entry per failed task.
#   - If the retrospective flagged a systemic failure, logs a SYSTEMIC_FAILURE
#     ledger event (separate from the agent_subprocess entry that the runner
#     already wrote).
#
# Why a separate script: the retrospective subagent itself is denied write
# access to state.json and ledger.jsonl (its profile blocks Edit/Write to
# both). The orchestrator runs this helper afterwards under its own
# (broader) permissions to do the merge.
#
# Usage:
#   bash scripts/merge-lesson-into-state.sh <workspace_path>
#
# Exit codes:
#   0  — merge complete (or no-op if no handoff JSON)
#   1  — runtime failure (missing jq, malformed handoff, etc.)
#   2  — referenced lesson YAML missing on disk (integrity failure)

set -uo pipefail

# v8.18.0: dual-root — state, ledger, and lessons are writable artifacts under
# the user's project. Distinct from the plugin's read-only scripts/agents.
__rr_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$__rr_self/resolve-roots.sh"
unset __rr_self
STATE="${EVOLVE_STATE_OVERRIDE:-$EVOLVE_PROJECT_ROOT/.evolve/state.json}"
LEDGER="${EVOLVE_LEDGER_OVERRIDE:-$EVOLVE_PROJECT_ROOT/.evolve/ledger.jsonl}"
LESSONS_DIR="$EVOLVE_PROJECT_ROOT/.evolve/instincts/lessons"

log() { echo "[merge-lesson] $*" >&2; }
fail() { log "FAIL: $*"; exit 1; }

[ $# -ge 1 ] || fail "usage: merge-lesson-into-state.sh <workspace_path>"

WORKSPACE="$1"
[ -d "$WORKSPACE" ] || fail "workspace not found: $WORKSPACE"

HANDOFF="$WORKSPACE/handoff-retrospective.json"
if [ ! -f "$HANDOFF" ]; then
    log "no handoff JSON at $HANDOFF — nothing to merge (PASS cycle?)"
    exit 0
fi

command -v jq >/dev/null 2>&1 || fail "jq required"

# Validate the handoff structure first.
jq empty "$HANDOFF" 2>/dev/null || fail "handoff is not valid JSON: $HANDOFF"

CYCLE=$(jq -r '.cycle // empty' "$HANDOFF")
VERDICT=$(jq -r '.auditVerdict // empty' "$HANDOFF")
ERROR_CATEGORY=$(jq -r '.errorCategory // empty' "$HANDOFF")
FAILED_STEP=$(jq -r '.failedStep // empty' "$HANDOFF")
SYSTEMIC=$(jq -r '.systemic // false' "$HANDOFF")

[ -n "$CYCLE" ] || fail "handoff missing required field: cycle"
[ -n "$VERDICT" ] || fail "handoff missing required field: auditVerdict"

# Lesson IDs the retrospective wrote.
LESSON_IDS=()
while IFS= read -r id; do
    [ -n "$id" ] && LESSON_IDS+=("$id")
done < <(jq -r '.lessonIds[]?' "$HANDOFF")

if [ "${#LESSON_IDS[@]}" -eq 0 ]; then
    log "WARN: handoff lists zero lessonIds for $VERDICT cycle $CYCLE"
fi

# Verify each referenced lesson YAML exists on disk.
for id in "${LESSON_IDS[@]}"; do
    matches=("$LESSONS_DIR/${id}-"*.yaml)
    if [ ! -f "${matches[0]}" ]; then
        # Try exact-name fallback.
        if [ ! -f "$LESSONS_DIR/${id}.yaml" ]; then
            log "INTEGRITY-FAIL: lesson $id referenced in handoff but no file under $LESSONS_DIR"
            exit 2
        fi
    fi
done

# Initialize state.json if missing.
if [ ! -f "$STATE" ]; then
    log "creating new state.json (was missing)"
    echo '{"instinctSummary": [], "failedApproaches": []}' > "$STATE"
fi

# Build the patches.
NOW_TS=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Patch 1: append each lesson to instinctSummary.
TMP_STATE=$(mktemp)
jq_args=()
for id in "${LESSON_IDS[@]}"; do
    # Read the lesson YAML to extract pattern + confidence — fields agents
    # consume from instinctSummary. YAML parsing in shell is fragile; use
    # python3 with PyYAML if available, else awk-based extraction.
    matches=("$LESSONS_DIR/${id}-"*.yaml)
    yaml_path="${matches[0]}"
    [ -f "$yaml_path" ] || yaml_path="$LESSONS_DIR/${id}.yaml"

    pattern=""
    confidence=""
    error_cat=""
    if command -v python3 >/dev/null 2>&1; then
        # Try PyYAML; fall back to crude grep if unavailable.
        pattern=$(python3 -c "
import sys
try:
    import yaml
    with open('$yaml_path') as f:
        data = yaml.safe_load(f)
        item = data[0] if isinstance(data, list) else data
        print(item.get('pattern', ''))
except Exception:
    pass
" 2>/dev/null)
        confidence=$(python3 -c "
try:
    import yaml
    with open('$yaml_path') as f:
        data = yaml.safe_load(f)
        item = data[0] if isinstance(data, list) else data
        print(item.get('confidence', 0.5))
except Exception:
    print(0.5)
" 2>/dev/null)
        error_cat=$(python3 -c "
try:
    import yaml
    with open('$yaml_path') as f:
        data = yaml.safe_load(f)
        item = data[0] if isinstance(data, list) else data
        ctx = item.get('failureContext', {})
        print(ctx.get('errorCategory', ''))
except Exception:
    pass
" 2>/dev/null)
    fi

    # Crude fallback if PyYAML is unavailable.
    [ -z "$pattern" ] && pattern=$(grep -E '^[[:space:]]*pattern:' "$yaml_path" | head -1 | sed 's/.*pattern:[[:space:]]*//;s/^"//;s/"$//')
    [ -z "$confidence" ] && confidence=$(grep -E '^[[:space:]]*confidence:' "$yaml_path" | head -1 | sed 's/.*confidence:[[:space:]]*//' || echo "0.5")
    [ -z "$error_cat" ] && error_cat=$(grep -E '^[[:space:]]*errorCategory:' "$yaml_path" | head -1 | sed 's/.*errorCategory:[[:space:]]*//;s/^"//;s/"$//')

    [ -z "$pattern" ] && pattern="unknown-pattern"
    [ -z "$confidence" ] && confidence="0.5"

    jq --arg id "$id" --arg pattern "$pattern" \
       --argjson confidence "$confidence" \
       --arg error_cat "$error_cat" \
       '.instinctSummary += [{
            id: $id,
            pattern: $pattern,
            confidence: $confidence,
            type: "failure-lesson",
            errorCategory: $error_cat
        }]' "$STATE" > "$TMP_STATE"
    mv "$TMP_STATE" "$STATE"
done

# Patch 2: append failedApproaches entry (one per cycle, summarizing).
jq --argjson cycle "$CYCLE" \
   --arg verdict "$VERDICT" \
   --arg error_cat "$ERROR_CATEGORY" \
   --arg failed_step "$FAILED_STEP" \
   --arg ts "$NOW_TS" \
   --argjson lesson_ids "$(printf '%s\n' "${LESSON_IDS[@]}" | jq -R . | jq -s .)" \
   --argjson systemic "$SYSTEMIC" \
   '.failedApproaches += [{
        ts: $ts,
        cycle: $cycle,
        auditVerdict: $verdict,
        errorCategory: $error_cat,
        failedStep: $failed_step,
        lessonIds: $lesson_ids,
        systemic: $systemic
    }]' "$STATE" > "$TMP_STATE"
mv "$TMP_STATE" "$STATE"

log "OK: merged ${#LESSON_IDS[@]} lesson(s) into instinctSummary; appended failedApproaches entry for cycle $CYCLE"

# Patch 3: log systemic failure event to the ledger.
# v8.37.0: includes prev_hash + entry_seq for tamper-evident chain.
if [ "$SYSTEMIC" = "true" ]; then
    # Compute chain link inline (avoid sourcing subagent-run.sh).
    _ml_prev_hash="0000000000000000000000000000000000000000000000000000000000000000"
    _ml_entry_seq=0
    if [ -f "$LEDGER" ] && [ -s "$LEDGER" ]; then
        _ml_last_line=$(tail -1 "$LEDGER" 2>/dev/null || echo "")
        if [ -n "$_ml_last_line" ]; then
            if command -v sha256sum >/dev/null 2>&1; then
                _ml_prev_hash=$(printf '%s' "$_ml_last_line" | sha256sum | awk '{print $1}')
            else
                _ml_prev_hash=$(printf '%s' "$_ml_last_line" | shasum -a 256 | awk '{print $1}')
            fi
        fi
        _ml_entry_seq=$(wc -l < "$LEDGER" 2>/dev/null | tr -d ' ' || echo 0)
        [ -z "$_ml_entry_seq" ] && _ml_entry_seq=0
    fi
    _ml_new_line=$(jq -nc \
        --arg ts "$NOW_TS" \
        --argjson cycle "$CYCLE" \
        --arg verdict "$VERDICT" \
        --arg error_cat "$ERROR_CATEGORY" \
        --argjson lesson_ids "$(printf '%s\n' "${LESSON_IDS[@]}" | jq -R . | jq -s .)" \
        --argjson entry_seq "$_ml_entry_seq" \
        --arg prev_hash "$_ml_prev_hash" \
        '{ts: $ts, cycle: $cycle, role: "orchestrator", kind: "SYSTEMIC_FAILURE",
          auditVerdict: $verdict, errorCategory: $error_cat, lessonIds: $lesson_ids,
          entry_seq: $entry_seq, prev_hash: $prev_hash}')
    printf '%s\n' "$_ml_new_line" >> "$LEDGER"
    # Update tip
    if command -v sha256sum >/dev/null 2>&1; then
        _ml_new_sha=$(printf '%s' "$_ml_new_line" | sha256sum | awk '{print $1}')
    else
        _ml_new_sha=$(printf '%s' "$_ml_new_line" | shasum -a 256 | awk '{print $1}')
    fi
    _ml_tip_file="$(dirname "$LEDGER")/ledger.tip"
    _ml_tmp="${_ml_tip_file}.tmp.$$"
    printf '%s:%s\n' "$_ml_entry_seq" "$_ml_new_sha" > "$_ml_tmp" 2>/dev/null \
        && mv -f "$_ml_tmp" "$_ml_tip_file" 2>/dev/null \
        || rm -f "$_ml_tmp" 2>/dev/null
    log "OK: SYSTEMIC_FAILURE event written to ledger (seq=$_ml_entry_seq)"
fi

# Patch 4: contradicted instincts. The retrospective profile cannot mutate
# personal instincts; we surface contradictions in state.json so the
# orchestrator's next prune step can act on them.
CONTRADICTED=$(jq -r '.contradictedInstincts[]?' "$HANDOFF" 2>/dev/null)
if [ -n "$CONTRADICTED" ]; then
    log "INFO: lessons contradict instincts: $(echo "$CONTRADICTED" | tr '\n' ' ')"
    log "      run \`prune\` to reconcile (not auto-applied)."
fi

exit 0
