# Eval: cherry-pick-cycle-230

**Slug:** cherry-pick-cycle-230
**Phase:** cycle-231
**Task:** P0 recovery — merge cycle-230 commit 201f7cb (auditor-doc-trim, phase-naming-lint, acs-suite-root-autosolve, ledger-skip-source) into main and ship.

---

## AC1 — Go test suite passes [code]

```bash
cd "$WORKTREE/go" && go test ./... 2>&1 | tail -10
```

Expected: All lines are `ok` or `(cached)`, zero lines containing `FAIL`.

Grader: exit 0 iff `go test ./...` produces no `^FAIL` lines.

```bash
#!/usr/bin/env bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}"
cd "$WORKTREE/go"
out=$(go test ./... 2>&1)
if echo "$out" | grep -q '^FAIL'; then
  echo "RED: go test ./... has failures" >&2
  echo "$out" | grep '^FAIL' >&2
  exit 1
fi
echo "GREEN: all packages pass"
```

---

## AC2 — ACS suite green (red=0) [code]

```bash
"$WORKTREE/go/evolve" acs suite --cycle 231 -root "$WORKTREE" 2>&1 | grep -E "green=[0-9]+ red=0"
```

Expected: Output contains `red=0`.

```bash
#!/usr/bin/env bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}"
out=$("$WORKTREE/go/evolve" acs suite --cycle 231 -root "$WORKTREE" 2>&1)
if echo "$out" | grep -qE "green=[0-9]+ red=0"; then
  echo "GREEN: ACS suite red=0"
else
  echo "RED: ACS suite has red predicates" >&2
  echo "$out" | tail -5 >&2
  exit 1
fi
```

---

## AC3 — Cycle-230 commit present in history [code]

```bash
git -C "$WORKTREE" log --oneline | grep -q "201f7cb\|evolve-cycle 230"
```

Expected: The cycle-230 commit (auditor-doc-trim, phase-naming-lint, acs-suite-root-autosolve, ledger-skip-source) is reachable from HEAD.

```bash
#!/usr/bin/env bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}"
if git -C "$WORKTREE" log --oneline | grep -qE "201f7cb|evolve-cycle 230"; then
  echo "GREEN: cycle-230 commit present in history"
else
  echo "RED: cycle-230 commit 201f7cb not found in git log" >&2
  exit 1
fi
```

---

## AC4 — Phase naming lint test passes (negative case fixed) [code]

The cycle-229 red-anchor test `TestBugRepro_Cycle229_TwoTierNamingMissing` previously failed on main because `ValidateUserSpec` accepted single-word user-phase names. After cherry-pick it must pass.

```bash
#!/usr/bin/env bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}"
cd "$WORKTREE/go"
if go test ./internal/phasespec/... -run "TestBugRepro_Cycle229_TwoTierNamingMissing|TestTwoTierNaming" -v 2>&1 | grep -q PASS; then
  echo "GREEN: two-tier naming tests pass"
else
  echo "RED: two-tier naming tests not PASS" >&2
  exit 1
fi
```

---

## AC5 — Auditor persona ≤300 lines [code]

```bash
#!/usr/bin/env bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}"
lines=$(wc -l < "$WORKTREE/agents/evolve-auditor.md")
if [ "$lines" -le 300 ]; then
  echo "GREEN: evolve-auditor.md = $lines lines (≤300)"
else
  echo "RED: evolve-auditor.md = $lines lines (>300, regression gate will fail)" >&2
  exit 1
fi
```

---

## Gaming-fake check

A trivial fake would `git cherry-pick` without running tests. AC1 + AC2 catch this: the ACS suite runs phasespec tests as part of its predicates, and `go test ./...` requires all unit tests to pass.
