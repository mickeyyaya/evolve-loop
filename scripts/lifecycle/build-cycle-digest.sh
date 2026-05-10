#!/usr/bin/env bash
#
# build-cycle-digest.sh — write the per-cycle digest manifest.
#
# v8.62.0 Campaign B Cycle B1 (Tier 2 — digest layer).
#
# Writes .evolve/runs/cycle-N/cycle-digest.json — a small structured manifest
# that subsequent role-context-builder calls reference instead of re-reading
# full state.json arrays. The digest contains:
#
#   cycle              (number)
#   built_at           (ISO 8601 UTC timestamp)
#   intent_anchor      (raw goal text, ≤500 chars; never paraphrased — Factory pattern)
#   top_task           (Scout's chosen task, ≤200 chars)
#   acceptance_criteria(≤500 chars; from intent.md or scout-report.md)
#   recent_failures    (last 3 failedApproaches: id + 1-line summary)
#   instinct_pointers  (top 5 instinct IDs + headline + confidence; NO full pattern body)
#   todos_summary      (count, IDs, oldest_unpicked_cycle)
#   ledger_tip         (most recent ledger entry SHA, for tamper-evident binding)
#
# Idempotent: safe to invoke multiple times per cycle; output is a function
# of state.json + workspace artifacts at invocation time.
#
# Anchored summary pattern (Factory, scored 4.04 for technical-detail
# preservation across compression cycles): we DO NOT paraphrase intent.
# The intent_anchor field is verbatim text from the user's goal.
#
# Usage:
#   bash build-cycle-digest.sh <cycle> <workspace_path>
#
# Exit codes:
#   0 — digest written
#   1 — workspace not found
#   2 — missing arguments
#
# Environment:
#   EVOLVE_PROJECT_ROOT — auto-detected via resolve-roots.sh
#
# References:
#   docs/architecture/cycle-digest-schema.md (canonical schema)
#   memory/reference_token_optimization_research.md (Factory anchor pattern)

set -uo pipefail

__bcd_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$__bcd_self/resolve-roots.sh" 2>/dev/null || {
    EVOLVE_PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$(pwd)}"
}
unset __bcd_self

CYCLE="${1:-}"
WORKSPACE="${2:-}"

if [ -z "$CYCLE" ] || [ -z "$WORKSPACE" ]; then
    echo "usage: build-cycle-digest.sh <cycle> <workspace_path>" >&2
    exit 2
fi

if [ ! -d "$WORKSPACE" ]; then
    echo "build-cycle-digest: workspace not found: $WORKSPACE" >&2
    exit 1
fi

command -v jq >/dev/null 2>&1 || { echo "build-cycle-digest: jq required" >&2; exit 1; }

STATE="$EVOLVE_PROJECT_ROOT/.evolve/state.json"
LEDGER="$EVOLVE_PROJECT_ROOT/.evolve/ledger.jsonl"
INTENT_FILE="$WORKSPACE/intent.md"
SCOUT_FILE="$WORKSPACE/scout-report.md"
DIGEST_FILE="$WORKSPACE/cycle-digest.json"

# --- Field extractors --------------------------------------------------------

# Cap a text field at N chars; preserve UTF-8 boundary safety by truncating
# on byte count (acceptable for our anchor fields which are ASCII-heavy).
_cap() {
    local text="$1" max="$2"
    if [ ${#text} -le "$max" ]; then
        printf '%s' "$text"
    else
        printf '%s…' "${text:0:$max}"
    fi
}

# Extract intent_anchor: prefer raw goal from intent.md. The intent file
# uses YAML-like frontmatter with `goal: |` blocks (see scripts/lifecycle/
# intent-test.sh fixtures); fall back to markdown ## Goal header form, then
# scout-report's first non-blank line.
_extract_intent_anchor() {
    local text=""
    if [ -f "$INTENT_FILE" ]; then
        # YAML `goal: |` block — captures indented continuation lines until
        # the next top-level YAML key (line starting at col 0 with `key:`).
        text=$(awk '
            /^goal:[[:space:]]*\|/ {flag=1; next}
            flag && /^[a-zA-Z_]+:[[:space:]]*([|>]|$)/ {flag=0}
            flag {sub(/^[[:space:]]+/,""); print}
        ' "$INTENT_FILE" | sed '/^[[:space:]]*$/d' | head -4 | tr '\n' ' ' | sed 's/[[:space:]]\+$//')
        # Fall back to inline `goal: "..."` form.
        if [ -z "$text" ]; then
            text=$(awk '/^goal:[[:space:]]*"/{ sub(/^goal:[[:space:]]*"/,""); sub(/"[[:space:]]*$/,""); print; exit }' "$INTENT_FILE")
        fi
        # Markdown ## Goal fallback.
        if [ -z "$text" ]; then
            text=$(awk '/^## Goal$/{flag=1;next} /^## /{flag=0} flag' "$INTENT_FILE" \
                | sed '/^[[:space:]]*$/d' | head -3 | tr '\n' ' ' | sed 's/[[:space:]]\+$//')
        fi
    fi
    if [ -z "$text" ] && [ -f "$SCOUT_FILE" ]; then
        text=$(awk '!/^<!--/ && !/^# / && NF{print; exit}' "$SCOUT_FILE")
    fi
    [ -z "$text" ] && text="(no intent)"
    _cap "$text" 500
}

# Extract top_task: Scout's chosen task (## Top Task section or first
# "### Task:" line in scout-report.md).
_extract_top_task() {
    local text=""
    if [ -f "$SCOUT_FILE" ]; then
        text=$(awk '/^## Top Task$|^## Selected Task$|^### Task:/{flag=1;next} /^## |^### /{if(flag){flag=0}} flag' "$SCOUT_FILE" \
            | sed '/^[[:space:]]*$/d' | head -2 | tr '\n' ' ' | sed 's/[[:space:]]\+$//')
        if [ -z "$text" ]; then
            # Fallback: first line of scout-report after the title.
            text=$(awk 'NR>3 && !/^<!--/ && !/^# / && !/^## / && NF{print; exit}' "$SCOUT_FILE")
        fi
    fi
    [ -z "$text" ] && text="(no top task)"
    _cap "$text" 200
}

# Extract acceptance_criteria: intent.md uses YAML `acceptance_checks:`
# list (each entry has a `- check: "..."` line). Take the first 2 checks'
# text. Fall back to markdown ## Acceptance Criteria header form.
_extract_acceptance() {
    local text=""
    for src in "$INTENT_FILE" "$SCOUT_FILE"; do
        [ -f "$src" ] || continue
        # YAML acceptance_checks list — collect `- check: "..."` lines.
        text=$(awk '
            /^acceptance_checks:[[:space:]]*$/ {flag=1; next}
            flag && /^[a-zA-Z_]+:/ {flag=0}
            flag && /^[[:space:]]*-[[:space:]]*check:[[:space:]]*"/ {
                sub(/^[[:space:]]*-[[:space:]]*check:[[:space:]]*"/,"")
                sub(/"[[:space:]]*$/,"")
                print
            }
        ' "$src" | head -2 | tr '\n' ' ' | sed 's/[[:space:]]\+$//')
        # Markdown fallback.
        if [ -z "$text" ]; then
            text=$(awk '/^## Acceptance Criteria$/{flag=1;next} /^## /{flag=0} flag' "$src" \
                | sed '/^[[:space:]]*$/d' | head -8 | tr '\n' ' ' | sed 's/[[:space:]]\+$//')
        fi
        [ -n "$text" ] && break
    done
    [ -z "$text" ] && text="(no acceptance criteria)"
    _cap "$text" 500
}

# Recent failures: last 3 failedApproaches as id + 1-line summary.
# Schema reality (state.json:failedApproaches[]): each entry has cycle,
# verdict, classification, recordedAt, defects[]. There is no `id` field —
# we synthesize one as "cycle-N-classification". Summary uses classification
# + verdict + first defect title (if any).
_extract_recent_failures() {
    if [ ! -f "$STATE" ]; then
        echo "[]"
        return
    fi
    jq -c '
        (.failedApproaches // [])
        | (if length > 3 then .[-3:] else . end)
        | map(
            # Tolerate both new-shape (verdict / classification) and
            # legacy-shape (auditVerdict / errorCategory) entries.
            ((.classification // .errorCategory // "unknown") | tostring) as $cat
            | ((.verdict // .auditVerdict // "?") | tostring) as $v
            | {
                id: ("cycle-" + ((.cycle // 0) | tostring) + "-" + $cat),
                cycle: (.cycle // 0),
                verdict: $v,
                classification: $cat,
                summary: (
                    ($cat + " (" + $v + ")"
                     + (if (.failedStep // null) then "; step=" + (.failedStep | tostring) else "" end)
                     + (if (.defects // []) | length > 0
                        then "; " + ((.defects[0].title // .defects[0].id // "defect") | tostring)
                        else "" end))
                    | .[0:120]
                )
              }
          )
    ' "$STATE" 2>/dev/null || echo "[]"
}

# Instinct pointers: top N (by recency) — id + headline + confidence ONLY.
_extract_instinct_pointers() {
    local n="${1:-5}"
    if [ ! -f "$STATE" ]; then
        echo "[]"
        return
    fi
    jq -c --argjson n "$n" '
        (.instinctSummary // [])
        | (if length > $n then .[-$n:] else . end)
        | map({id: (.id // "?"),
               headline: ((.pattern // .headline // "") | tostring | .[0:80]),
               confidence: (.confidence // 0)})
    ' "$STATE" 2>/dev/null || echo "[]"
}

# Todos summary: count + ID list + oldest cycle pointer.
_extract_todos_summary() {
    if [ ! -f "$STATE" ]; then
        echo '{"count":0,"ids":[],"oldest_unpicked_cycle":null}'
        return
    fi
    jq -c '
        (.carryoverTodos // []) as $t
        | {count: ($t | length),
           ids: ($t | map(.id // "?")),
           oldest_unpicked_cycle: ($t | map(.cycles_unpicked // 0) | max // null)}
    ' "$STATE" 2>/dev/null || echo '{"count":0,"ids":[],"oldest_unpicked_cycle":null}'
}

# Ledger tip: most recent SHA from ledger.tip (the tamper-evident pointer)
# or recompute from last ledger line if tip missing.
_extract_ledger_tip() {
    local tip=""
    if [ -f "$EVOLVE_PROJECT_ROOT/.evolve/ledger.tip" ]; then
        tip=$(cat "$EVOLVE_PROJECT_ROOT/.evolve/ledger.tip" 2>/dev/null | tr -d '[:space:]')
    fi
    [ -z "$tip" ] && tip="(no ledger tip)"
    printf '%s' "$tip"
}

# --- Assemble ---------------------------------------------------------------

INTENT_ANCHOR=$(_extract_intent_anchor)
TOP_TASK=$(_extract_top_task)
ACCEPTANCE=$(_extract_acceptance)
RECENT_FAILURES=$(_extract_recent_failures)
INSTINCT_POINTERS=$(_extract_instinct_pointers 5)
TODOS_SUMMARY=$(_extract_todos_summary)
LEDGER_TIP=$(_extract_ledger_tip)
BUILT_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ)

# Atomic write via tmp + mv (bash 3.2 portable; never leaves partial JSON
# on disk if interrupted).
TMP="${DIGEST_FILE}.tmp.$$"

jq -n \
    --argjson cycle "$CYCLE" \
    --arg built_at "$BUILT_AT" \
    --arg intent_anchor "$INTENT_ANCHOR" \
    --arg top_task "$TOP_TASK" \
    --arg acceptance_criteria "$ACCEPTANCE" \
    --argjson recent_failures "$RECENT_FAILURES" \
    --argjson instinct_pointers "$INSTINCT_POINTERS" \
    --argjson todos_summary "$TODOS_SUMMARY" \
    --arg ledger_tip "$LEDGER_TIP" \
    '{
        schema_version: "1.0",
        cycle: $cycle,
        built_at: $built_at,
        intent_anchor: $intent_anchor,
        top_task: $top_task,
        acceptance_criteria: $acceptance_criteria,
        recent_failures: $recent_failures,
        instinct_pointers: $instinct_pointers,
        todos_summary: $todos_summary,
        ledger_tip: $ledger_tip
    }' > "$TMP" && mv -f "$TMP" "$DIGEST_FILE"

echo "[build-cycle-digest] wrote $DIGEST_FILE ($(wc -c < "$DIGEST_FILE" | tr -d ' ') bytes)" >&2
