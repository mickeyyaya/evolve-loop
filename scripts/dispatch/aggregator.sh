#!/usr/bin/env bash
#
# aggregator.sh — Pure-shell merge of fan-out worker artifacts (Sprint 1).
#
# Merges N worker artifacts into a single canonical phase artifact according
# to per-phase rules. NO LLM CALL — this script cannot be coerced by a
# malicious worker into accepting forged output. The trust kernel still
# binds the AGGREGATE artifact via phase-gate.sh's existing
# check_subagent_ledger_match (the caller writes one parent ledger entry
# whose SHA covers the canonical output).
#
# Per-phase merge rules:
#   scout / research / discover → concat with "## Worker: <name>" headers
#   audit                       → ALL-PASS verdict; any FAIL fails the aggregate
#   learn / retrospective       → union of "## Lesson:" sections, deduped by title
#   audit-consensus / cross-cli-vote (v8.53+)
#                               → MAJORITY-PASS with FAIL-VETO across CLIs;
#                                 quorum = ceil(N/2); any FAIL → consensus FAIL
#
# Usage:
#   aggregator.sh <phase> <output-path> <worker-artifact-1> [<worker-2> ...]
#
# Exit codes:
#   0 — merge succeeded; for audit phase, every worker reported PASS
#   1 — merge succeeded but verdict is FAIL (audit phase only)
#   2 — usage error or input file missing/empty
#
# Bash 3.2 compatible per CLAUDE.md.

set -uo pipefail

PHASE="${1:-}"
OUTPUT="${2:-}"

if [ -z "$PHASE" ] || [ -z "$OUTPUT" ]; then
    echo "[aggregator] usage: $0 <phase> <output> <worker-artifact>..." >&2
    exit 2
fi

shift 2

if [ "$#" -lt 1 ]; then
    echo "[aggregator] error: at least one worker artifact required" >&2
    exit 2
fi

# Validate every worker artifact is present and non-empty before any output.
for w in "$@"; do
    if [ ! -f "$w" ]; then
        echo "[aggregator] error: worker artifact not found: $w" >&2
        exit 2
    fi
    if [ ! -s "$w" ]; then
        echo "[aggregator] error: worker artifact is empty: $w" >&2
        exit 2
    fi
done

mkdir -p "$(dirname "$OUTPUT")"
TMP="$OUTPUT.tmp.$$"
trap 'rm -f "$TMP"' EXIT

# Normalize phase aliases.
case "$PHASE" in
    scout|research|discover)         MERGE_MODE=concat ;;
    audit)                            MERGE_MODE=verdict ;;
    learn|retrospective|retro)        MERGE_MODE=lessons ;;
    plan-review)                      MERGE_MODE=plan_review ;;
    audit-consensus|cross-cli-vote)   MERGE_MODE=cross_cli_vote ;;
    *)
        echo "[aggregator] error: unknown phase '$PHASE'" >&2
        exit 2
        ;;
esac

NOW="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

case "$MERGE_MODE" in
    concat)
        # Scout-style merge: concat with per-worker headings.
        {
            printf '# Aggregated %s Report\n\n' "$PHASE"
            printf '_Aggregated by aggregator.sh at %s. Workers: %d._\n\n' "$NOW" "$#"
            for w in "$@"; do
                local_name="$(basename "$w" .md)"
                printf '## Worker: %s\n\n' "$local_name"
                cat "$w"
                printf '\n\n'
            done
        } > "$TMP"
        ;;

    verdict)
        # Audit-style merge: parse each worker's verdict (line starting `Verdict: ...`),
        # aggregate to ALL-PASS / any-FAIL / mixed=WARN. Body retains all worker reports.
        ANY_FAIL=0
        ANY_WARN=0
        ALL_PASS=1
        for w in "$@"; do
            v=$(awk 'tolower($0) ~ /^[[:space:]]*verdict:/ { print; exit }' "$w" \
                | sed -E 's/^[[:space:]]*[Vv]erdict:[[:space:]]*//' \
                | tr '[:lower:]' '[:upper:]' \
                | awk '{print $1}')
            case "$v" in
                PASS) : ;;
                FAIL) ANY_FAIL=1; ALL_PASS=0 ;;
                WARN) ANY_WARN=1; ALL_PASS=0 ;;
                *)    ANY_WARN=1; ALL_PASS=0 ;;
            esac
        done
        if [ "$ANY_FAIL" = "1" ]; then
            VERDICT="FAIL"
        elif [ "$ALL_PASS" = "1" ]; then
            VERDICT="PASS"
        else
            VERDICT="WARN"
        fi
        {
            printf 'Verdict: %s\n\n' "$VERDICT"
            printf '# Aggregated Audit Report\n\n'
            printf '_Aggregated by aggregator.sh at %s. Workers: %d. Aggregate verdict: %s._\n\n' \
                "$NOW" "$#" "$VERDICT"
            for w in "$@"; do
                local_name="$(basename "$w" .md)"
                printf '## Worker: %s\n\n' "$local_name"
                cat "$w"
                printf '\n\n'
            done
        } > "$TMP"
        ;;

    plan_review)
        # Sprint 2 plan-review merge: each lens worker emits 'Score: <0-10>'
        # and 'Verdict: <PROCEED|REVISE|ABORT>'. Aggregate verdict rules:
        #   ABORT  if any lens explicitly says ABORT, OR average score < 5
        #   REVISE if average score >= 5 AND at least one lens has score < 5
        #   PROCEED if average score >= 7 AND no lens score < 5
        # The body retains every lens report so the orchestrator can route
        # revisions back to Scout with concrete suggestions.
        ANY_ABORT=0
        WEAK_LENS=0
        SCORE_SUM=0
        SCORE_COUNT=0
        for w in "$@"; do
            local_score=$(awk 'tolower($0) ~ /^[[:space:]]*score:/ {
                gsub(/^[[:space:]]*[Ss]core:[[:space:]]*/, "")
                gsub(/[^0-9.].*/, "")
                print
                exit
            }' "$w")
            local_verdict=$(awk 'tolower($0) ~ /^[[:space:]]*verdict:/ {
                gsub(/^[[:space:]]*[Vv]erdict:[[:space:]]*/, "")
                gsub(/[[:space:]].*/, "")
                print toupper($0)
                exit
            }' "$w")
            # Default missing scores to 0 (treat as ABORT-worthy).
            [ -z "$local_score" ] && local_score=0
            [ "$local_verdict" = "ABORT" ] && ANY_ABORT=1
            # Use awk for float comparison (bash builtin can't).
            if awk -v s="$local_score" 'BEGIN{exit !(s+0 < 5)}'; then
                WEAK_LENS=1
            fi
            # Sum scores via awk (also handles fractional input).
            SCORE_SUM=$(awk -v a="$SCORE_SUM" -v b="$local_score" 'BEGIN{printf "%.2f", a+b}')
            SCORE_COUNT=$((SCORE_COUNT + 1))
        done
        AVG=$(awk -v s="$SCORE_SUM" -v c="$SCORE_COUNT" 'BEGIN{ if (c==0) print "0"; else printf "%.2f", s/c }')
        # Determine final verdict.
        if [ "$ANY_ABORT" = "1" ] || awk -v a="$AVG" 'BEGIN{exit !(a+0 < 5)}'; then
            VERDICT="ABORT"
        elif [ "$WEAK_LENS" = "1" ]; then
            VERDICT="REVISE"
        elif awk -v a="$AVG" 'BEGIN{exit !(a+0 >= 7)}'; then
            VERDICT="PROCEED"
        else
            VERDICT="REVISE"
        fi
        {
            printf 'Verdict: %s\n' "$VERDICT"
            printf 'Average Score: %s\n\n' "$AVG"
            printf '# Aggregated Plan-Review Report\n\n'
            printf '_Aggregated by aggregator.sh at %s. Lenses: %d. Average: %s. Verdict: %s._\n\n' \
                "$NOW" "$#" "$AVG" "$VERDICT"
            for w in "$@"; do
                local_name="$(basename "$w" .md)"
                printf '## Worker: %s\n\n' "$local_name"
                cat "$w"
                printf '\n\n'
            done
        } > "$TMP"
        ;;

    lessons)
        # Retrospective merge: union of "## Lesson: <title>" blocks, deduped on title.
        # awk tracks seen titles and emits each block once. A "block" is the heading
        # plus everything until the next "## Lesson:" or EOF.
        TMP_LESSONS="$TMP.lessons"
        {
            for w in "$@"; do cat "$w"; printf '\n'; done
        } | awk '
            /^## Lesson:/ {
                # Flush current block before starting new one.
                if (in_block && !(title in seen)) {
                    print block
                    seen[title] = 1
                }
                title = $0
                block = $0
                in_block = 1
                next
            }
            in_block { block = block "\n" $0 }
            END {
                if (in_block && !(title in seen)) {
                    print block
                    seen[title] = 1
                }
            }
        ' > "$TMP_LESSONS"
        {
            printf '# Aggregated Retrospective Report\n\n'
            printf '_Aggregated by aggregator.sh at %s. Workers: %d._\n\n' "$NOW" "$#"
            cat "$TMP_LESSONS"
        } > "$TMP"
        rm -f "$TMP_LESSONS"
        ;;

    cross_cli_vote)
        # v8.53.0+: cross-CLI consensus merge — multi-LLM Auditor fan-out.
        #
        # Each worker is an Auditor running under a DIFFERENT CLI (Claude,
        # Gemini, Codex). The merge logic enforces consensus to defeat
        # same-vendor sycophancy (the failure mode the cycle 102-111 tautological-
        # eval incident and the 2026-03-23 Gemini Forgery both touch on).
        #
        # Protocol: MAJORITY-PASS with FAIL-VETO.
        #   - Any single FAIL → consensus FAIL (any one CLI smelled blood)
        #   - >= ceil(N/2) PASS verdicts AND no FAIL → consensus PASS
        #   - Otherwise → consensus WARN (mixed signal; ship per fluent default
        #     unless EVOLVE_STRICT_AUDIT=1 elevates WARN → block)
        #
        # Quorum override: profile.consensus.quorum (in caller's profile JSON;
        # not read here — caller sizes the worker count to match the quorum).
        #
        # Schema parity with verdict mode: Verdict: <PASS|WARN|FAIL> on first
        # line so ship.sh's audit-binding logic works unchanged.
        ANY_FAIL=0
        PASS_COUNT=0
        TOTAL_COUNT=0
        WORKER_VERDICTS=""
        for w in "$@"; do
            v=$(awk 'tolower($0) ~ /^[[:space:]]*verdict:/ { print; exit }' "$w" \
                | sed -E 's/^[[:space:]]*[Vv]erdict:[[:space:]]*//' \
                | tr '[:lower:]' '[:upper:]' \
                | awk '{print $1}')
            local_name="$(basename "$w" .md)"
            WORKER_VERDICTS="${WORKER_VERDICTS}${WORKER_VERDICTS:+, }${local_name}=${v:-MISSING}"
            TOTAL_COUNT=$((TOTAL_COUNT + 1))
            case "$v" in
                PASS) PASS_COUNT=$((PASS_COUNT + 1)) ;;
                FAIL) ANY_FAIL=1 ;;
                *) : ;;  # WARN, MISSING, or unrecognized — counts toward TOTAL but not PASS
            esac
        done

        # Compute quorum threshold (default: ceil(N/2) = (N+1)/2 with integer math).
        QUORUM_THRESHOLD=$(( (TOTAL_COUNT + 1) / 2 ))

        if [ "$ANY_FAIL" = "1" ]; then
            VERDICT="FAIL"
            CONSENSUS_REASON="cross-cli-vote: at least one CLI returned FAIL (veto rule)"
        elif [ "$PASS_COUNT" -ge "$QUORUM_THRESHOLD" ]; then
            VERDICT="PASS"
            CONSENSUS_REASON="cross-cli-vote: $PASS_COUNT of $TOTAL_COUNT CLIs returned PASS (quorum=$QUORUM_THRESHOLD)"
        else
            VERDICT="WARN"
            CONSENSUS_REASON="cross-cli-vote: $PASS_COUNT of $TOTAL_COUNT PASS (below quorum=$QUORUM_THRESHOLD); ships per fluent default unless EVOLVE_STRICT_AUDIT=1"
        fi

        {
            printf 'Verdict: %s\n\n' "$VERDICT"
            printf '# Aggregated Cross-CLI Consensus Audit\n\n'
            printf '_Aggregated by aggregator.sh at %s. CLIs voting: %d (PASS=%d, FAIL=%d-veto-active=%s, quorum=%d)._\n\n' \
                "$NOW" "$TOTAL_COUNT" "$PASS_COUNT" "$ANY_FAIL" "$([ "$ANY_FAIL" = "1" ] && echo yes || echo no)" "$QUORUM_THRESHOLD"
            printf '## Consensus Decision\n\n'
            printf '**Verdict**: %s\n\n' "$VERDICT"
            printf '**Reason**: %s\n\n' "$CONSENSUS_REASON"
            printf '**Per-CLI verdicts**: %s\n\n' "$WORKER_VERDICTS"
            printf '**Protocol**: MAJORITY-PASS with FAIL-VETO. Any FAIL forces consensus FAIL (defends against false-positive PASS from sycophantic same-vendor agreement). >= quorum PASS with no FAIL → consensus PASS. Otherwise WARN.\n\n'
            printf '## Per-CLI Audit Reports\n\n'
            for w in "$@"; do
                local_name="$(basename "$w" .md)"
                printf '### Worker: %s\n\n' "$local_name"
                cat "$w"
                printf '\n\n'
            done
        } > "$TMP"
        ;;
esac

mv -f "$TMP" "$OUTPUT"
trap - EXIT

# Exit code: audit FAIL and plan-review ABORT signal failure. Other phases
# succeed if files merged cleanly.
if [ "$MERGE_MODE" = "verdict" ] && [ "$VERDICT" = "FAIL" ]; then
    exit 1
fi
if [ "$MERGE_MODE" = "plan_review" ] && [ "$VERDICT" = "ABORT" ]; then
    exit 1
fi
# v8.53.0: cross_cli_vote FAIL is a hard signal (any one CLI smelled blood);
# WARN follows fluent-default policy and exits 0 (ship.sh's audit-binding
# decides whether to ship based on its own EVOLVE_STRICT_AUDIT logic).
if [ "$MERGE_MODE" = "cross_cli_vote" ] && [ "$VERDICT" = "FAIL" ]; then
    exit 1
fi
exit 0
