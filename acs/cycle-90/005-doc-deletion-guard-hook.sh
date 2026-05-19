#!/usr/bin/env bash
# AC-ID: cycle-90-005-doc-deletion-guard-hook
# Description: Verifies that scripts/hooks/doc-deletion-guard.sh exists, is
#   executable, behaves correctly under three canonical PreToolUse scenarios,
#   AND is wired into .claude/settings.json:hooks.PreToolUse.
# Evidence: intent.md success-criteria row "scripts/hooks/doc-deletion-guard.sh
#   exists, is executable, blocks `rm docs/research/foo.md` with rc=2, allows
#   archival mv, allows when EVOLVE_ALLOW_DOC_DELETE=1"; trust-kernel-notes
#   bullet 1 (JSON stdin / JSON deny stderr / rc=2; CLI-agnostic).
# Author: tdd-engineer (cycle-90)
# Created: 2026-05-19
# Acceptance-of: build-report.md row "5E: doc-deletion-guard.sh authored,
#   chmodded, wired into .claude/settings.json, three-scenario behavior verified"
#
# Behavioral: this predicate executes the guard against synthetic stdin JSON
# matching the Claude Code PreToolUse contract. Mutants that hard-code rc=0
# (always allow) fail the deletion scenario; mutants that hard-code rc=2
# (always deny) fail the archival scenario; mutants that ignore the env-var
# escape hatch fail the EVOLVE_ALLOW_DOC_DELETE scenario.
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
HOOK="$REPO_ROOT/scripts/hooks/doc-deletion-guard.sh"
SETTINGS="$REPO_ROOT/.claude/settings.json"
AC_ID="cycle-90-005-doc-deletion-guard-hook"

# ---- Structural checks -----------------------------------------------------

if [ ! -f "$HOOK" ]; then
  echo "RED $AC_ID: hook not found at scripts/hooks/doc-deletion-guard.sh" >&2
  exit 1
fi

if [ ! -x "$HOOK" ]; then
  echo "RED $AC_ID: hook not executable (chmod +x missing)" >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "RED $AC_ID: jq not on PATH (needed for hook payload contract)" >&2
  exit 1
fi

# ---- Functional scenarios --------------------------------------------------

# Scenario A: `rm docs/research/foo.md` without archival companion → DENY (rc=2)
payload_deny=$(jq -nc '{tool_name: "Bash", tool_input: {command: "rm docs/research/foo.md"}}')
set +e
out_deny=$(printf '%s' "$payload_deny" | "$HOOK" 2>&1)
rc_deny=$?
set -e
if [ "$rc_deny" -ne 2 ]; then
  echo "RED $AC_ID: scenario A (rm without archival) expected rc=2, got rc=$rc_deny" >&2
  echo "  hook output: $out_deny" >&2
  exit 1
fi

# Scenario B: archival mv to knowledge-base/research/archived-YYYY-MM-DD/ → ALLOW (rc=0)
payload_allow=$(jq -nc '{tool_name: "Bash", tool_input: {command: "mv docs/research/foo.md knowledge-base/research/archived-2026-05-19/foo.md"}}')
set +e
out_allow=$(printf '%s' "$payload_allow" | "$HOOK" 2>&1)
rc_allow=$?
set -e
if [ "$rc_allow" -ne 0 ]; then
  echo "RED $AC_ID: scenario B (archival mv) expected rc=0, got rc=$rc_allow" >&2
  echo "  hook output: $out_allow" >&2
  exit 1
fi

# Scenario C: same destructive command, but operator escape hatch set → ALLOW
set +e
out_escape=$(EVOLVE_ALLOW_DOC_DELETE=1 printf '%s' "$payload_deny" | EVOLVE_ALLOW_DOC_DELETE=1 "$HOOK" 2>&1)
rc_escape=$?
set -e
if [ "$rc_escape" -ne 0 ]; then
  echo "RED $AC_ID: scenario C (EVOLVE_ALLOW_DOC_DELETE=1 escape) expected rc=0, got rc=$rc_escape" >&2
  echo "  hook output: $out_escape" >&2
  exit 1
fi

# Scenario D: non-doc-tree operation (e.g., editing src code) → ALLOW (passthrough)
payload_unrelated=$(jq -nc '{tool_name: "Edit", tool_input: {file_path: "scripts/some-other-file.sh", old_string: "x", new_string: "y"}}')
set +e
out_unrelated=$(printf '%s' "$payload_unrelated" | "$HOOK" 2>&1)
rc_unrelated=$?
set -e
if [ "$rc_unrelated" -ne 0 ]; then
  echo "RED $AC_ID: scenario D (unrelated file edit passthrough) expected rc=0, got rc=$rc_unrelated" >&2
  echo "  hook output: $out_unrelated" >&2
  exit 1
fi

# ---- Wiring check ----------------------------------------------------------

if [ ! -f "$SETTINGS" ]; then
  echo "RED $AC_ID: .claude/settings.json missing — hook not wired" >&2
  exit 1
fi

# Confirm the hook is referenced under hooks.PreToolUse (any matcher).
# Tolerate either the relative form (scripts/hooks/doc-deletion-guard.sh) or
# an env-prefixed form ($EVOLVE_PLUGIN_ROOT/scripts/hooks/doc-deletion-guard.sh).
wired=$(jq -r '
  (.hooks // {}).PreToolUse // []
  | map(.hooks // [])
  | flatten
  | map(.command // "")
  | map(select(test("doc-deletion-guard\\.sh")))
  | length
' "$SETTINGS" 2>/dev/null)

if [ -z "$wired" ] || [ "$wired" -lt 1 ]; then
  echo "RED $AC_ID: doc-deletion-guard.sh not referenced under .hooks.PreToolUse in .claude/settings.json" >&2
  exit 1
fi

echo "GREEN $AC_ID: hook present + executable; 4/4 behavioral scenarios pass; wired into PreToolUse"
exit 0
