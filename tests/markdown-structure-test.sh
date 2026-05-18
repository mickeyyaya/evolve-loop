#!/usr/bin/env bash
#
# markdown-structure-test.sh — Unit tests for lint-markdown-structure.sh and extract-tldr.sh.
#
# Creates fixture .md files in a temp dir, runs the tools, checks outputs.
# Reports N/N PASS.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LINT="$REPO_ROOT/scripts/utility/lint-markdown-structure.sh"
EXTRACT="$REPO_ROOT/scripts/utility/extract-tldr.sh"

PASS=0
FAIL=0
TMPDIR_TEST=$(mktemp -d)
trap 'rm -rf "$TMPDIR_TEST"' EXIT

ok()   { PASS=$((PASS + 1)); echo "PASS  $1"; }
fail() { FAIL=$((FAIL + 1)); echo "FAIL  $1: $2"; }

# ── Fixtures ──────────────────────────────────────────────────────────────────

COMPLIANT="$TMPDIR_TEST/compliant.md"
cat > "$COMPLIANT" <<'EOF'
---
name: test-compliant
description: A compliant fixture file for testing
metadata:
  type: convention
---

> **Compliant fixture**

## TLDR

**Synopsis:** This file is compliant.

**Key points:**
- Has frontmatter
- Has TLDR
- Has imperative H2s

**Non-goals:** Nothing extra.

## Run the tests

Do something here.

## Configure the tool

Another section.
EOF

NO_FM="$TMPDIR_TEST/no-frontmatter.md"
cat > "$NO_FM" <<'EOF'
# No Frontmatter

## TLDR

**Synopsis:** No frontmatter here.

**Key points:**
- missing frontmatter

**Non-goals:** n/a

## Do something
EOF

NO_TLDR="$TMPDIR_TEST/no-tldr.md"
cat > "$NO_TLDR" <<'EOF'
---
name: no-tldr
description: Missing TLDR
metadata:
  type: convention
---

## Do something

No TLDR section here.
EOF

# A long file (>100 lines) without TOC
LONG_NO_TOC="$TMPDIR_TEST/long-no-toc.md"
{
    printf '%s\n' '---' 'name: long-no-toc' 'description: Long file without TOC' 'metadata:' '  type: convention' '---' ''
    printf '%s\n' '## TLDR' '' '**Synopsis:** Long file.' '' '**Key points:**' '- Has 100+ lines' '' '**Non-goals:** n/a' ''
    for i in $(seq 1 90); do
        printf '## Section %s\n\nContent here.\n\n' "$i"
    done
} > "$LONG_NO_TOC"

# A long file (>100 lines) WITH TOC
LONG_WITH_TOC="$TMPDIR_TEST/long-with-toc.md"
{
    printf '%s\n' '---' 'name: long-with-toc' 'description: Long file with TOC' 'metadata:' '  type: convention' '---' ''
    printf '%s\n' '## TLDR' '' '**Synopsis:** Long file with TOC.' '' '**Key points:**' '- Has TOC' '' '**Non-goals:** n/a' ''
    printf '%s\n' '## Table of Contents' '' '1. [Section 1](#section-1)' ''
    for i in $(seq 1 90); do
        printf '## Section %s\n\nContent here.\n\n' "$i"
    done
} > "$LONG_WITH_TOC"

# ── Tests: lint-markdown-structure.sh ────────────────────────────────────────

# T1: compliant file → 0 warnings
output=$(bash "$LINT" "$COMPLIANT" 2>&1)
if echo "$output" | grep -q "0 warn(s)"; then
    ok "T1: compliant file → 0 warnings"
else
    fail "T1: compliant file → 0 warnings" "got: $output"
fi

# T2: no frontmatter → 1+ warning
output=$(bash "$LINT" "$NO_FM" 2>&1)
if echo "$output" | grep -q "WARN.*missing YAML frontmatter"; then
    ok "T2: no-frontmatter file → WARN for missing frontmatter"
else
    fail "T2: no-frontmatter file → WARN for missing frontmatter" "got: $output"
fi

# T3: no TLDR → 1+ warning
output=$(bash "$LINT" "$NO_TLDR" 2>&1)
if echo "$output" | grep -q "WARN.*missing ## TLDR"; then
    ok "T3: no-TLDR file → WARN for missing TLDR"
else
    fail "T3: no-TLDR file → WARN for missing TLDR" "got: $output"
fi

# T4: long file without TOC → warning
output=$(bash "$LINT" "$LONG_NO_TOC" 2>&1)
if echo "$output" | grep -q "WARN.*Table of Contents"; then
    ok "T4: long file without TOC → WARN for missing TOC"
else
    fail "T4: long file without TOC → WARN for missing TOC" "got: $output"
fi

# T5: long file with TOC → no TOC warning
output=$(bash "$LINT" "$LONG_WITH_TOC" 2>&1)
if echo "$output" | grep -q "WARN.*Table of Contents"; then
    fail "T5: long file with TOC → no TOC warning" "got unexpected TOC warning: $output"
else
    ok "T5: long file with TOC → no TOC warning"
fi

# T6: lint always exits 0
bash "$LINT" "$NO_FM" >/dev/null 2>&1
rc=$?
if [ "$rc" -eq 0 ]; then
    ok "T6: lint exits 0 even on warnings (WARN-only mode)"
else
    fail "T6: lint exits 0 even on warnings" "exit code was $rc"
fi

# T7: directory input is accepted
output=$(bash "$LINT" "$TMPDIR_TEST" 2>&1)
rc=$?
if [ "$rc" -eq 0 ]; then
    ok "T7: directory input accepted, exits 0"
else
    fail "T7: directory input accepted" "exit code was $rc"
fi

# ── Tests: extract-tldr.sh ───────────────────────────────────────────────────

# T8: compliant file → extracts TLDR content
output=$(bash "$EXTRACT" "$COMPLIANT" 2>/dev/null)
if echo "$output" | grep -q "Synopsis:"; then
    ok "T8: extract-tldr extracts TLDR from compliant file"
else
    fail "T8: extract-tldr extracts TLDR from compliant file" "got: $output"
fi

# T9: file without TLDR → exits 1
bash "$EXTRACT" "$NO_TLDR" >/dev/null 2>&1
rc=$?
if [ "$rc" -eq 1 ]; then
    ok "T9: extract-tldr exits 1 for file without TLDR"
else
    fail "T9: extract-tldr exits 1 for file without TLDR" "exit code was $rc"
fi

# T10: nonexistent file → exits 1
bash "$EXTRACT" "$TMPDIR_TEST/nonexistent.md" >/dev/null 2>&1
rc=$?
if [ "$rc" -eq 1 ]; then
    ok "T10: extract-tldr exits 1 for nonexistent file"
else
    fail "T10: extract-tldr exits 1 for nonexistent file" "exit code was $rc"
fi

# T11: extract-tldr does not include the next ## heading content
output=$(bash "$EXTRACT" "$COMPLIANT" 2>/dev/null)
if echo "$output" | grep -q "## Run the tests"; then
    fail "T11: extract-tldr stops at next ## heading" "got content past next heading: $output"
else
    ok "T11: extract-tldr stops at next ## heading"
fi

# ── Summary ──────────────────────────────────────────────────────────────────

TOTAL=$((PASS + FAIL))
echo ""
echo "markdown-structure-test: $PASS/$TOTAL PASS"

[ "$FAIL" -eq 0 ]
