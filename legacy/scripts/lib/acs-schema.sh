#!/usr/bin/env bash
#
# acs-schema.sh — Shared constants for Execution-Grounded Process Supervision (EGPS) v10.0.0+.
#
# Defines the predicate vocabulary, banned patterns, exit-code semantics, and
# JSON envelope shape that the entire EGPS pipeline uses. Source from any
# script that produces or consumes acs-verdict.json or predicates.
#
# Canonical reference: docs/architecture/egps-v10.md
# Research basis:      knowledge-base/research/execution-grounded-process-supervision-2026.md
#
# DO NOT change constant names without bumping all consumers. The numeric
# values and string labels here are load-bearing across the pipeline.

if [ -n "${EVOLVE_ACS_SCHEMA_SH_LOADED:-}" ]; then
    return 0 2>/dev/null || exit 0
fi
EVOLVE_ACS_SCHEMA_SH_LOADED=1

# ── Exit-code semantics ─────────────────────────────────────────────────────
readonly ACS_EXIT_GREEN=0          # Acceptance criterion verified; cycle may proceed
readonly ACS_EXIT_RED=1            # Acceptance criterion violated; cycle MUST fail
readonly ACS_EXIT_INVALID=2        # Predicate itself malformed (e.g., not executable)
readonly ACS_EXIT_BANNED=3         # Predicate uses a banned pattern (grep-only, network, etc.)

# ── Verdict labels (binary; no WARN, no scalar confidence) ─────────────────
readonly ACS_VERDICT_PASS="PASS"
readonly ACS_VERDICT_FAIL="FAIL"

# ── Schema versions ────────────────────────────────────────────────────────
readonly ACS_PREDICATE_SCHEMA_VERSION="1.0"
readonly ACS_VERDICT_SCHEMA_VERSION="1.0"

# ── Banned patterns inside predicates ──────────────────────────────────────
# Each pattern is a regex matched against the predicate's source. If matched
# AND not accompanied by an execution-following line, the predicate is rejected
# at lint time and at runtime. validate-predicate.sh enforces these.
#
# The first column is the regex; the second column is a human-readable name;
# the third column is the rejection rationale.
acs_banned_patterns_table() {
    cat <<'TABLE'
grep_only_check|grep-only verification|presence ≠ execution; predicate MUST run the code path being verified
trivial_pass|trivial echo PASS|"echo PASS; exit 0" is a tautology — predicate MUST do real work
network_call|network access|curl/wget/gh API breaks hermetic-determinism; predicates MUST be deterministic
long_sleep|sleep longer than 1s|predicates MUST complete fast (operator-feedback responsiveness)
filesystem_write|write outside acs-output|predicates are read-only on repo state; write under .evolve/runs/cycle-N/acs-output/ only
TABLE
}

# ── Regex strings for the matchers (used by validate-predicate.sh) ─────────
readonly ACS_BANNED_REGEX_GREP_ONLY='^[[:space:]]*grep[[:space:]]+-q[[:space:]]'
readonly ACS_BANNED_REGEX_TRIVIAL_PASS='echo[[:space:]]+["'"'"']PASS["'"'"'][[:space:]]*[;\n].*exit[[:space:]]+0'
readonly ACS_BANNED_REGEX_NETWORK='\b(curl|wget|gh[[:space:]]+(api|pr|release))\b'
readonly ACS_BANNED_REGEX_LONG_SLEEP='^[[:space:]]*sleep[[:space:]]+([2-9]|[1-9][0-9]+)'
readonly ACS_BANNED_REGEX_FS_WRITE='>[[:space:]]*/(etc|var|usr|home|tmp/[^.]|Users/[^/]+/[^/]+)'

# ── Acceptance criterion ID format ─────────────────────────────────────────
# Predicates live at acs/cycle-N/{NNN}-{slug}.sh
#   N    = cycle number (positive integer)
#   NNN  = zero-padded 3-digit ordinal (001, 002, …)
#   slug = kebab-case identifier (max 40 chars)
#
# Example: acs/cycle-40/001-tests-pass.sh
readonly ACS_PREDICATE_FILENAME_REGEX='^[0-9]{3}-[a-z0-9-]{1,40}\.sh$'

# ── Required predicate metadata header keys ────────────────────────────────
# Each predicate's first ~10 lines must declare:
#   AC-ID:        cycle-N-NNN
#   Description:  one-line summary
#   Evidence:     pointer (file:line OR commit-SHA OR test-name)
#   Author:       agent role (builder/tester/auditor) + persona name
#   Created:      ISO-8601 timestamp
#   Acceptance-of: link to the build-report.md AC this verifies (line number or AC# token)
readonly ACS_REQUIRED_HEADERS="AC-ID Description Evidence Author Created Acceptance-of"

# ── Helper: extract a header value from a predicate file ──────────────────
acs_predicate_header() {
    local key=$1 file=$2
    # Match lines like "# AC-ID:  cycle-40-001" — comment, key, optional whitespace, colon, value.
    grep -E "^#[[:space:]]*${key}[[:space:]]*:" "$file" 2>/dev/null \
        | head -1 \
        | sed -E "s/^#[[:space:]]*${key}[[:space:]]*:[[:space:]]*//" \
        | sed -E 's/[[:space:]]+$//'
}

# ── Helper: validate filename matches the cycle predicate naming rule ─────
acs_filename_valid() {
    local fname=$1
    [[ "$fname" =~ $ACS_PREDICATE_FILENAME_REGEX ]]
}
