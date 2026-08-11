---
title: gitignore-evals-exception-and-commit
cycle: 176
score_cap: 1.0
---

# Eval: gitignore evals exception and commit

## Context

`.evolve/*` in `.gitignore` silently drops eval files at ship (cycle-92 class defect).
The `011-eval-files-persisted` regression predicate (cycle-173) catches this and stays
RED until the `.gitignore` has negation exceptions and eval files are git-tracked.

## Acceptance Criteria

### AC-1: `.gitignore` has `!.evolve/evals/` negation [code]

```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
grep -q '!\.evolve/evals/' "$REPO_ROOT/.gitignore" \
  || { echo "RED: .gitignore missing !.evolve/evals/ exception"; exit 1; }
echo "GREEN: .gitignore has !.evolve/evals/ exception"
```

### AC-2: `.gitignore` has `!.evolve/evals/*.md` negation [code]

```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
grep -q '!\.evolve/evals/\*\.md' "$REPO_ROOT/.gitignore" \
  || { echo "RED: .gitignore missing !.evolve/evals/*.md exception"; exit 1; }
echo "GREEN: .gitignore has !.evolve/evals/*.md exception"
```

### AC-3: cycle-173 eval files are git-tracked [code]

```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT"
for f in .evolve/evals/transient-bridge-retry.md .evolve/evals/backfill-ledger-and-docs.md; do
  [ -f "$f" ] || { echo "RED: $f missing on disk"; exit 1; }
  git ls-files --error-unmatch "$f" >/dev/null 2>&1 \
    || { echo "RED: $f untracked (gitignored) — must be committed"; exit 1; }
done
echo "GREEN: cycle-173 eval files exist on disk and are git-tracked"
```

### AC-4: at least 2 eval files are tracked under `.evolve/evals/` [code]

```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT"
count=$(git ls-files .evolve/evals/*.md 2>/dev/null | wc -l | tr -d ' ')
[ "$count" -ge 2 ] \
  || { echo "RED: only $count eval files tracked (need >=2)"; exit 1; }
echo "GREEN: $count eval files tracked under .evolve/evals/"
```

## Negative Cases

### NC-1: bare `.evolve/*` without negation would still ignore evals [code]

```bash
# This verifies the NEGATION is correctly placed AFTER the .evolve/* deny rule.
REPO_ROOT=$(git rev-parse --show-toplevel)
# The deny line must precede the negation (gitignore order matters).
deny_line=$(grep -n '\.evolve/\*' "$REPO_ROOT/.gitignore" | grep -v '!' | head -1 | cut -d: -f1)
neg_line=$(grep -n '!\.evolve/evals/' "$REPO_ROOT/.gitignore" | head -1 | cut -d: -f1)
[ -n "$deny_line" ] || { echo "RED: .evolve/* deny line not found"; exit 1; }
[ -n "$neg_line" ]  || { echo "RED: !.evolve/evals/ negation not found"; exit 1; }
[ "$deny_line" -lt "$neg_line" ] \
  || { echo "RED: negation (line $neg_line) appears before deny (line $deny_line) — order wrong"; exit 1; }
echo "GREEN: .evolve/* deny (line $deny_line) precedes !.evolve/evals/ negation (line $neg_line)"
```
