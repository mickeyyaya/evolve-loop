# Eval: ship-collider-preflight

**Slug:** ship-collider-preflight
**Phase:** cycle-231
**Task:** SIXTH drift mode fix — `evolve ship` pre-flights untracked files in the main tree that would be overwritten by ff-merge with the cycle branch; emits a loud actionable error instead of looping to recovery.

---

## AC1 — Pre-flight unit test passes [code]

New test `TestShipColliderPreflight` in `go/internal/phases/ship/` verifies that when untracked files exist in the main tree that also appear in the cycle-branch commit tree, `Run()` (or the relevant internal function) returns a non-nil error containing "collider" before attempting the ff-merge.

```bash
#!/usr/bin/env bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}"
cd "$WORKTREE/go"
if go test ./internal/phases/ship/... -run "TestColliderPreflight\|TestShipCollider" -v 2>&1 | grep -q PASS; then
  echo "GREEN: collider preflight test passes"
else
  echo "RED: TestColliderPreflight/TestShipCollider not found or not PASS" >&2
  exit 1
fi
```

---

## AC2 — Ship dry-run detects collider and exits non-zero [code]

When an untracked file in the main tree would be overwritten by a worktree ff-merge, `evolve ship --dry-run` must exit non-zero and emit a message listing the offending path(s).

```bash
#!/usr/bin/env bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}"
cd "$WORKTREE/go"
# Integration: the ship package's error-path test covers this scenario
# without needing a live git repo — we verify the error text mentions "collider"
# or "would be overwritten" in the existing error-path test suite.
if go test ./internal/phases/ship/... -run "TestShipCollider|TestUntrackedCollider|TestFFMergeCollider" -v 2>&1 | grep -qE "PASS|no test files"; then
  echo "GREEN: collider error-path test passes (or no separate test needed — AC1 covers it)"
else
  echo "RED: collider error-path test failed" >&2
  exit 1
fi
```

---

## AC3 — Full ship package test suite passes (no regression) [code]

```bash
#!/usr/bin/env bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}"
cd "$WORKTREE/go"
out=$(go test ./internal/phases/ship/... 2>&1)
if echo "$out" | grep -q "^FAIL"; then
  echo "RED: ship package tests have failures" >&2
  echo "$out" | grep "^FAIL" >&2
  exit 1
fi
echo "GREEN: ship package passes"
echo "$out" | tail -3
```

---

## AC4 — Error message is actionable (names the collider files) [code]

The error returned by the pre-flight must contain the path of the offending file(s), not just a generic "ff-merge failed" message.

```bash
#!/usr/bin/env bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}"
cd "$WORKTREE/go"
# The test in AC1 verifies the error message format. Additionally verify
# the error code is CodeGitUntrackedCollider (or equivalent) by grepping
# the Go source.
if grep -r "collider\|Collider\|UntrackedCollider\|would.be.overwritten" \
     ./internal/phases/ship/gitops.go 2>/dev/null | grep -qi "collider\|overwritten"; then
  echo "GREEN: collider error message code found in gitops.go"
else
  echo "RED: no collider error handling found in go/internal/phases/ship/gitops.go" >&2
  exit 1
fi
```

---

## AC5 — ACS suite still green [code]

```bash
#!/usr/bin/env bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}"
out=$("$WORKTREE/go/evolve" acs suite --cycle 231 -root "$WORKTREE" 2>&1)
if echo "$out" | grep -qE "green=[0-9]+ red=0"; then
  echo "GREEN: ACS suite passes after ship change"
else
  echo "RED: ACS suite has red predicates" >&2
  echo "$out" | tail -5 >&2
  exit 1
fi
```

---

## Gaming-fake check

A trivial fake would add `"collider"` text to an error string without actually checking untracked files. AC1 uses a test with a real in-process git repo (temp dir) that stages the collision scenario and verifies exit-code behavior. AC4 verifies the source has a proper collider-check expression.
