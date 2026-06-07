#!/usr/bin/env bash
# ACS — cycle-252 task `auditor-context-diet`
# acs-predicate: config-check (grep waiver) — persona-prose deliverable
# (a "Handoff Reading Protocol" section in agents/evolve-auditor.md);
# behavior lives in the LLM. The NEGATIVE axis is the load-bearing half:
# a context diet that tells the auditor to skip ACS execution or skip
# `git diff HEAD` would gut the audit — those greps must stay at zero.
# Auditor: review waiver validity.
set -uo pipefail

top=$(git rev-parse --show-toplevel)
f="$top/agents/evolve-auditor.md"

fail=0
# Positive: the protocol section heading exists.
if ! grep -qE '^##+ .*Handoff Reading Protocol' "$f"; then
    echo "RED: evolve-auditor.md missing '## Handoff Reading Protocol' section heading" >&2
    fail=1
else
    # Scope the extraction-contract checks to the section BODY (heading →
    # next same-or-higher-level heading), so pre-existing prose elsewhere
    # in the persona cannot satisfy them (bash-3.2-safe awk slice).
    # (No ERE intervals — older BSD awk lacks {n,m}.)
    section=$(awk '/^##+ .*Handoff Reading Protocol/{inside=1; next} inside && /^#+ /{exit} inside{print}' "$f")
    for token in 'verdict' 'commit SHA|SHA' 'ACS|red.green'; do
        if ! echo "$section" | grep -qiE "$token"; then
            echo "RED: Handoff Reading Protocol section does not name extraction field /$token/" >&2
            fail=1
        fi
    done
    # The section must frame itself as extract-only (skip verbatim re-read).
    if ! echo "$section" | grep -qiE 'only|extract|top 3|verbatim'; then
        echo "RED: Handoff Reading Protocol section lacks the extract-only framing" >&2
        fail=1
    fi
fi

# Negative 1: the diet must NOT instruct skipping/ignoring ACS predicates.
if grep -qiE 'skip(ping)?[^.]{0,40}\bACS\b|ignore[^.]{0,40}\bACS\b' "$f"; then
    echo "RED: evolve-auditor.md instructs skipping/ignoring ACS — diet must not weaken the audit" >&2
    grep -inE 'skip(ping)?[^.]{0,40}\bACS\b|ignore[^.]{0,40}\bACS\b' "$f" >&2
    fail=1
fi
# Negative 2: the auditor must still run `git diff HEAD` itself.
if ! grep -q 'git diff HEAD' "$f"; then
    echo "RED: evolve-auditor.md no longer requires 'git diff HEAD' — diet removed a mandatory audit step" >&2
    fail=1
fi

[ "$fail" -eq 0 ] || exit 1
echo "GREEN: Handoff Reading Protocol present; ACS execution + git diff HEAD untouched"
exit 0
