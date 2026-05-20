#!/usr/bin/env bash
# AC-ID: cycle-100-004-phase-observer-doc-migration-note
# AC-source: cycle-100/intent.md AC "phase-observer.md ... updated and internally consistent"
# acs-predicate: config-check
#
# Predicate: docs/architecture/phase-observer.md must reflect the
# v10.18.0 / cycle-100 migration. Specifically:
#   - The pre-migration blockquote "Known limitation (open, v10.17.0)"
#     must be removed (or replaced).
#   - A new statement marking phase-observer as the default detector
#     since v10.18.0 (cycle 100) must be present.
#   - The doc must reference the watchdog incident file
#     (cycle-94-98-watchdog-overfiring.md) so a reader can trace
#     migration motivation.
#
# This is a doc-content predicate; behavioral execution is N/A.
# Marked `acs-predicate: config-check` per acs/AGENTS.md waiver policy.
# However we still perform two independent checks (presence + absence)
# so a partial doc fix RED-fails the predicate.
#
# Bash 3.2 compatible.
#
# Exit codes:
#   0 = GREEN
#   1 = RED
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

DOC="docs/architecture/phase-observer.md"
if [ ! -f "$DOC" ]; then
  echo "RED: $DOC missing on disk" >&2
  exit 1
fi
if ! git ls-files --error-unmatch "$DOC" >/dev/null 2>&1; then
  echo "RED: $DOC exists but is not git-tracked" >&2
  exit 1
fi

# (a) Old "Known limitation (open, v10.17.0)" line must be GONE.
if grep -q 'Known limitation (open, v10.17.0)' "$DOC"; then
  echo "RED: $DOC still contains 'Known limitation (open, v10.17.0)' — must be removed/replaced post-migration" >&2
  exit 1
fi

# (b) Must contain a phrase marking the new default since v10.18.0 OR
# cycle-100. Accept either anchor — Builder might phrase it either way.
if ! grep -Eq 'v10\.18\.0|cycle[ -]?100' "$DOC"; then
  echo "RED: $DOC does not mention 'v10.18.0' or 'cycle 100' as the migration anchor" >&2
  exit 1
fi
if ! grep -Eqi 'default[^.]{0,40}detector|default[^.]{0,40}observer|observer[^.]{0,40}default' "$DOC"; then
  echo "RED: $DOC does not state phase-observer is the default detector now" >&2
  exit 1
fi

# (c) Must reference the watchdog incident file by name so the migration
# motivation is traceable from the architecture doc.
if ! grep -q 'cycle-94-98-watchdog-overfiring\.md\|cycle-94-99-watchdog-overfiring\.md' "$DOC"; then
  echo "RED: $DOC does not cross-reference the watchdog incident doc" >&2
  exit 1
fi

echo "GREEN: $DOC reflects v10.18.0/cycle-100 migration (limitation note removed, default-detector statement present, incident cross-ref present)"
exit 0
