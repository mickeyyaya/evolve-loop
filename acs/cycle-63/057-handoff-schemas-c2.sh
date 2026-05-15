#!/usr/bin/env bash
# ACS predicate 057 — cycle 63
# Verifies the C2 phase-handoff-schemas task: 5 new schemas exist, parse as
# JSON, declare the four canonical schema keys, and that the ADR + reference
# doc landed.
#
# AC-ID: cycle-63-057
# Description: Complete C2 phase handoff schemas (intent, triage, tdd, ship, retrospective) + ADR 0009 + output-contracts.md
# Evidence: 5 schema files jq-valid, ADR file exists, reference file exists
# Author: builder (cycle 63)
# Created: 2026-05-15T00:00:00Z
# Acceptance-of: scout-report.md Task 1
#
# metadata:
#   id: 057-handoff-schemas-c2
#   cycle: 63
#   task: c2-handoff-schemas
#   severity: HIGH

set -uo pipefail

if [ -n "${EVOLVE_PROJECT_ROOT:-}" ]; then
    REPO_ROOT="$EVOLVE_PROJECT_ROOT"
else
    REPO_ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
fi
if [ -f "$REPO_ROOT/.git" ]; then
    REPO_ROOT="$(cd "$REPO_ROOT" && cd "$(git rev-parse --git-common-dir)/.." && pwd)"
fi

rc=0

command -v jq >/dev/null 2>&1 || { echo "RED PRE: jq required for schema validation"; exit 1; }

# ── AC1–AC5: each new schema file exists, is jq-valid, declares the four
#              canonical schema keys (required_first_line, required_sections,
#              required_content, min_words). artifact_type field must match
#              the filename slug.
for slug in intent-report triage-decision tdd-report ship-report retrospective-report; do
    f="$REPO_ROOT/schemas/handoff/${slug}.schema.json"
    if [ ! -f "$f" ]; then
        echo "RED AC ${slug}: file missing at $f"
        rc=1
        continue
    fi
    if ! jq . "$f" >/dev/null 2>&1; then
        echo "RED AC ${slug}: invalid JSON"
        rc=1
        continue
    fi
    # Must declare the canonical keys
    for key in required_first_line required_sections required_content min_words artifact_type; do
        if ! jq -e "has(\"$key\")" "$f" >/dev/null 2>&1; then
            echo "RED AC ${slug}: missing schema key '$key'"
            rc=1
        fi
    done
    # artifact_type must match the filename slug (anti-tautology: forbids a
    # schema file from claiming a different type than its filename advertises).
    declared=$(jq -r '.artifact_type' "$f")
    if [ "$declared" != "$slug" ]; then
        echo "RED AC ${slug}: artifact_type='$declared' does not match filename slug '$slug'"
        rc=1
    fi
    # required_first_line must include the challenge-token pin (forgery guard)
    if ! jq -e '.required_first_line.pattern | test("challenge-token")' "$f" >/dev/null 2>&1; then
        echo "RED AC ${slug}: required_first_line.pattern missing challenge-token guard"
        rc=1
    fi
    [ "$rc" -eq 0 ] && echo "GREEN AC ${slug}: schema valid, declares all canonical keys, type matches slug, token guard present"
done

# ── AC6: ADR 0009 exists and references all 8 schemas
ADR="$REPO_ROOT/docs/adr/0009-phase-handoff-schemas.md"
if [ ! -f "$ADR" ]; then
    echo "RED AC6: $ADR missing"
    rc=1
else
    missing=0
    for slug in intent-report scout-report triage-decision tdd-report build-report audit-report ship-report retrospective-report; do
        if ! grep -q "$slug" "$ADR"; then
            echo "RED AC6: ADR does not reference '$slug'"
            missing=$((missing + 1))
            rc=1
        fi
    done
    [ "$missing" -eq 0 ] && echo "GREEN AC6: ADR references all 8 schemas"
fi

# ── AC7: output-contracts.md reference exists with phase × schema matrix
REF="$REPO_ROOT/.agents/skills/evolve-loop/reference/output-contracts.md"
if [ ! -f "$REF" ]; then
    echo "RED AC7: $REF missing"
    rc=1
elif ! grep -qE "(\| Phase \||Phase × schema)" "$REF"; then
    echo "RED AC7: output-contracts.md missing phase×schema table"
    rc=1
else
    # All 8 phase slugs must appear (lower-bound that the table is complete)
    for slug in intent-report scout-report triage-decision tdd-report build-report audit-report ship-report retrospective-report; do
        if ! grep -q "$slug" "$REF"; then
            echo "RED AC7: output-contracts.md does not reference '$slug'"
            rc=1
        fi
    done
    [ "$rc" -eq 0 ] && echo "GREEN AC7: reference matrix complete"
fi

# ── AC8 (anti-tautology): the schemas are NOT all byte-identical (would
#         signal a copy-paste with no per-phase tailoring). Compare a
#         hashable digest of required_sections[] across all 5 new files.
declare_sigs=""
for slug in intent-report triage-decision tdd-report ship-report retrospective-report; do
    f="$REPO_ROOT/schemas/handoff/${slug}.schema.json"
    [ -f "$f" ] || continue
    sig=$(jq -c '.required_sections | map(.name) | sort' "$f" 2>/dev/null)
    declare_sigs="${declare_sigs}${sig}\n"
done
unique=$(printf "$declare_sigs" | sort -u | grep -c .)
if [ "$unique" -lt 5 ]; then
    echo "RED AC8 (anti-tautology): only $unique unique required_sections signatures across 5 schemas — schemas not differentiated per phase"
    rc=1
else
    echo "GREEN AC8 (anti-tautology): 5/5 schemas have distinct required_sections signatures"
fi

exit "$rc"
