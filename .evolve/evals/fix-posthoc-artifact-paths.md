# Eval: fix-posthoc-artifact-paths

## Task
Correct POSTHOC metric sentinel paths in `agents/evolve-builder-reference.md` and `docs/architecture/posthoc-schema.md` so the example commands reference artifact filenames that actually exist (`build-usage.json`, `phase-timing.json`), not stale names (`builder-usage.json`, `builder-timing.json`).

## Acceptance Criteria

### [code] Stale filename builder-usage.json absent from template files
```bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
count=$(grep -rn 'builder-usage\.json' \
  "$WORKTREE/agents/evolve-builder-reference.md" \
  "$WORKTREE/docs/architecture/posthoc-schema.md" 2>/dev/null | wc -l | tr -d ' ')
if [ "$count" -ne 0 ]; then
  echo "RED: $count occurrences of builder-usage.json remain in template files" >&2
  grep -rn 'builder-usage\.json' \
    "$WORKTREE/agents/evolve-builder-reference.md" \
    "$WORKTREE/docs/architecture/posthoc-schema.md" >&2
  exit 1
fi
echo "GREEN: builder-usage.json fully replaced in template files" >&2
exit 0
```

### [code] Correct filename build-usage.json present in evolve-builder-reference.md
```bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
if ! grep -q 'build-usage\.json' "$WORKTREE/agents/evolve-builder-reference.md"; then
  echo "RED: build-usage.json not found in evolve-builder-reference.md posthoc section" >&2
  exit 1
fi
echo "GREEN: build-usage.json present in builder reference" >&2
exit 0
```

### [code] Stale filename builder-timing.json absent from template files
```bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
count=$(grep -rn 'builder-timing\.json' \
  "$WORKTREE/agents/evolve-builder-reference.md" \
  "$WORKTREE/docs/architecture/posthoc-schema.md" 2>/dev/null | wc -l | tr -d ' ')
if [ "$count" -ne 0 ]; then
  echo "RED: $count occurrences of builder-timing.json remain in template files" >&2
  exit 1
fi
echo "GREEN: builder-timing.json fully absent from template files" >&2
exit 0
```

### [code] Negative — stale paths do NOT exist in a real cycle workspace (structural guard)
```bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
# Ensure the reference itself does not claim builder-usage.json exists
if grep -q 'builder-usage\.json' "$WORKTREE/agents/evolve-builder-reference.md" 2>/dev/null; then
  echo "RED: template still references the non-existent builder-usage.json" >&2
  exit 1
fi
echo "GREEN: no stale path references in builder reference template" >&2
exit 0
```
