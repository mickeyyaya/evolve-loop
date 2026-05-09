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
#   bash scripts/failure/merge-lesson-into-state.sh <workspace_path>
#
# Exit codes:
#   0  — merge complete (or no-op if no handoff JSON)
#   1  — runtime failure (missing jq, malformed handoff, etc.)
#   2  — referenced lesson YAML missing on disk (integrity failure)

set -uo pipefail

# v8.18.0: dual-root — state, ledger, and lessons are writable artifacts under
# the user's project. Distinct from the plugin's read-only scripts/agents.
__rr_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$__rr_self/../lifecycle/resolve-roots.sh"
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

# v8.56.0 Layer A: cap instinctSummary at N=5 most-recent; archive evictees.
# Rationale: pre-v8.56 instinctSummary grew unbounded; agents received
# >10k tokens of stale lesson context per cycle.  Cap at N=5 keeps the
# context lean.  Older entries land in
# .evolve/archive/lessons/instinct-summary-archive.jsonl (gitignored)
# so retrospective lookup is still possible offline.
SUMMARY_CAP="${EVOLVE_INSTINCT_SUMMARY_CAP:-5}"
CURRENT_LEN=$(jq -r '.instinctSummary | length' "$STATE")
if [ "$CURRENT_LEN" -gt "$SUMMARY_CAP" ]; then
    EVICT_COUNT=$((CURRENT_LEN - SUMMARY_CAP))
    ARCHIVE_DIR="$EVOLVE_PROJECT_ROOT/.evolve/archive/lessons"
    ARCHIVE_FILE="$ARCHIVE_DIR/instinct-summary-archive.jsonl"
    mkdir -p "$ARCHIVE_DIR"
    # Append evicted (oldest, FIFO) to archive — one JSON object per line.
    jq -c --argjson n "$EVICT_COUNT" \
       '.instinctSummary[0:$n] | .[] | . + {evicted_at: now | todate, evicted_at_cycle: '"$CYCLE"'}' \
       "$STATE" >> "$ARCHIVE_FILE"
    # Truncate state to most-recent SUMMARY_CAP.
    jq --argjson n "$SUMMARY_CAP" '.instinctSummary = .instinctSummary[-$n:]' "$STATE" > "$TMP_STATE"
    mv "$TMP_STATE" "$STATE"
    log "OK: instinctSummary capped to $SUMMARY_CAP entries; $EVICT_COUNT evicted to $ARCHIVE_FILE"
fi

# v8.56.0 Layer A: merge carryover-todos.json into state.json:carryoverTodos[].
# Workflow: retrospective subagent emits carryover-todos.json as a separate
# artifact alongside lessons-detail YAML.  Each todo is `{id, action,
# priority, evidence_pointer}`.  We track first_seen_cycle (constant across
# re-defers) and defer_count (bumped on re-encounter).  WARN at >= 3.
TODOS_PATH="$WORKSPACE/carryover-todos.json"
if [ -f "$TODOS_PATH" ]; then
    if ! jq empty "$TODOS_PATH" 2>/dev/null; then
        log "WARN: carryover-todos.json malformed; skipping"
    else
        # Ensure carryoverTodos exists in state.json
        if ! jq -e '.carryoverTodos' "$STATE" >/dev/null 2>&1; then
            jq '. + {carryoverTodos: []}' "$STATE" > "$TMP_STATE" && mv "$TMP_STATE" "$STATE"
        fi
        TODO_COUNT=$(jq -r 'length' "$TODOS_PATH")
        WARN_IDS=()
        for i in $(seq 0 $((TODO_COUNT - 1))); do
            new_todo=$(jq -c ".[$i]" "$TODOS_PATH")
            new_id=$(echo "$new_todo" | jq -r '.id')
            [ -z "$new_id" ] || [ "$new_id" = "null" ] && continue
            existing=$(jq -c --arg id "$new_id" '.carryoverTodos[] | select(.id==$id)' "$STATE")
            if [ -n "$existing" ]; then
                # Re-defer: bump defer_count.
                cur_dc=$(echo "$existing" | jq -r '.defer_count // 0')
                new_dc=$((cur_dc + 1))
                jq --arg id "$new_id" --argjson dc "$new_dc" --argjson cyc "$CYCLE" \
                   '.carryoverTodos = [.carryoverTodos[] | if .id==$id then .defer_count = $dc | .last_seen_cycle = $cyc else . end]' \
                   "$STATE" > "$TMP_STATE"
                mv "$TMP_STATE" "$STATE"
                if [ "$new_dc" -ge 3 ]; then
                    WARN_IDS+=("$new_id (defer=${new_dc})")
                fi
            else
                # First sighting: tag with defer_count=0, first_seen_cycle=CYCLE,
                # cycles_unpicked=0 (v8.57.0 Layer D — used by reconcile-carryover-todos.sh
                # to track natural die-out via cycles-unpicked decay).
                jq --argjson todo "$new_todo" --argjson cyc "$CYCLE" \
                   '.carryoverTodos += [($todo + {defer_count: 0, cycles_unpicked: 0, first_seen_cycle: $cyc, last_seen_cycle: $cyc})]' \
                   "$STATE" > "$TMP_STATE"
                mv "$TMP_STATE" "$STATE"
            fi
        done
        log "OK: merged $TODO_COUNT carryoverTodo(s) into state.json"
        if [ "${#WARN_IDS[@]}" -gt 0 ]; then
            log "WARN: carryoverTodos with defer_count >= 3 (operator review needed): ${WARN_IDS[*]}"
        fi
    fi
fi

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

# Patch 5 (v8.46.0+): populate .evolve/audit-investigations/<slug>/ for
# human-readable failure review. Each FAIL/WARN cycle gets a dated dir
# with frozen evidence + investigation narrative + improvements + status.
# This is the operator-facing surface; lesson YAML in instincts/lessons/
# remains the runtime/agent-facing surface (different audiences, same
# source-of-truth).
INV_BASE="$EVOLVE_PROJECT_ROOT/.evolve/audit-investigations"
if [ -d "$INV_BASE" ] && [ -n "${VERDICT:-}" ]; then
    case "$VERDICT" in
        FAIL|WARN|WARN-NO-AUDIT)
            INV_SLUG=$(jq -r '.errorCategory // .lessons[0].id // "unspecified"' "$HANDOFF" 2>/dev/null \
                | sed 's/[^a-zA-Z0-9-]/-/g; s/--*/-/g; s/^-//; s/-$//' | head -c 50)
            [ -z "$INV_SLUG" ] && INV_SLUG="unspecified"
            INV_DATE=$(date -u +"%Y-%m-%d")
            INV_DIR="$INV_BASE/${INV_DATE}-cycle-${CYCLE}-${VERDICT}-${INV_SLUG}"
            mkdir -p "$INV_DIR/evidence"
            for art in audit-report.md build-report.md orchestrator-report.md intent.md scout-report.md retrospective-report.md; do
                [ -f "$WORKSPACE/$art" ] && cp "$WORKSPACE/$art" "$INV_DIR/evidence/$art" 2>/dev/null || true
            done
            if [ -f "$WORKSPACE/retrospective-report.md" ]; then
                cp "$WORKSPACE/retrospective-report.md" "$INV_DIR/investigation.md"
            else
                cat > "$INV_DIR/investigation.md" <<INVEOF
# Investigation — Cycle $CYCLE ($VERDICT)

**Date**: $INV_DATE
**Verdict**: $VERDICT
**Cycle**: $CYCLE
**Lesson IDs**: ${LESSON_IDS[@]:-(none)}

(retrospective-report.md not produced; this is a stub. See evidence/ for raw artifacts.)
INVEOF
            fi
            IMP_TEXT=$(jq -r '.improvementSuggestions // .lessons[0].preventiveAction // empty' "$HANDOFF" 2>/dev/null)
            if [ -n "$IMP_TEXT" ]; then
                cat > "$INV_DIR/improvements.md" <<IMPEOF
# Improvement Suggestions — Cycle $CYCLE

$IMP_TEXT
IMPEOF
            else
                cat > "$INV_DIR/improvements.md" <<IMPEOF
# Improvement Suggestions — Cycle $CYCLE

(No structured suggestions in handoff. Operator: extract from investigation.md and document concrete fixes here.)
IMPEOF
            fi
            FIRST_LESSON_ID="${LESSON_IDS[0]:-}"
            if [ -n "$FIRST_LESSON_ID" ] && [ -f "$LESSONS_DIR/${FIRST_LESSON_ID}.yaml" ]; then
                cp "$LESSONS_DIR/${FIRST_LESSON_ID}.yaml" "$INV_DIR/lesson.yaml"
            fi
            jq -n --arg state "open" \
                  --arg verdict "$VERDICT" \
                  --argjson cycle "$CYCLE" \
                  --arg lesson_id "${FIRST_LESSON_ID:-unspecified}" \
                  --arg opened "$NOW_TS" \
                  '{state: $state, verdict: $verdict, cycle: $cycle, lesson_id: $lesson_id, opened_at: $opened, actioned_at: null, action_refs: []}' \
                  > "$INV_DIR/status.json"
            log "OK: investigation dir created at $INV_DIR"
            INDEX_SCRIPT="$EVOLVE_PLUGIN_ROOT/scripts/failure/index-investigations.sh"
            [ -x "$INDEX_SCRIPT" ] && bash "$INDEX_SCRIPT" >/dev/null 2>&1 || \
                log "WARN: index-investigations.sh not refreshed"
            ;;
        *)
            log "verdict $VERDICT not in {FAIL,WARN,WARN-NO-AUDIT} — skipping investigation dir"
            ;;
    esac
fi

exit 0
