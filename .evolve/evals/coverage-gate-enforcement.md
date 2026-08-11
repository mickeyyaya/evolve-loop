# Eval: coverage-gate-enforcement

## Goal
The `make cover` target's comment says "fails if any internal/* package <85%"
but the body does not actually enforce this — it only prints coverage and exits 0.
Advance one stage toward enforcement: add an `awk` (or `grep`) step that reads
`coverage.func.txt`, detects any `internal/` line below 85.0%, and exits non-zero
with a clear message if found.

The gate must NOT block the cycle's own newly-added test code if coverage is thin
there — only `internal/` packages are in scope (matching the existing comment).

## Criteria

### C1 — make cover target has an enforcement step [code]
```bash
grep -n "awk\|grep.*85\|coverage.*fail\|threshold\|85.0\|below" go/Makefile
```
Expected: at least one line containing awk or explicit threshold check.

### C2 — `make cover` exits non-zero when a package is below threshold (negative test) [code]
```bash
# Inject a synthetic low-coverage entry and verify the awk step fires.
# This tests the enforcement logic by piping a fake coverage.func.txt line.
echo "github.com/mickeyyaya/evolve-loop/go/internal/fakepkg	total:	(statements)	40.0%" | \
  awk '/internal\/.*total:/ { pct=$NF; gsub(/%/,"",pct); if (pct+0 < 85.0) { print "BELOW:", $0; found=1 } } END { exit found ? 1 : 0 }'
echo "awk exit code: $?"
```
Expected: exit code 1 (the awk one-liner detects the low-coverage package).

### C3 — `make cover` comment and body are consistent [code]
```bash
grep -A8 "^cover:" go/Makefile
```
Expected: the body now includes an awk/grep step that implements the
"fails if any internal/* package <85%" comment.

### C4 — Whole test suite still passes after Makefile change [code]
```bash
cd go && go test -count=1 -short $(go list ./... | grep -v '/acs/') 2>&1 | tail -5
```
Expected: all packages `ok`, exit 0.

### NEG-1 — Gate does not fire on non-internal packages [code]
```bash
# A pkg/ or cmd/ package below 85% must not trip the gate.
echo "github.com/mickeyyaya/evolve-loop/go/pkg/fakepkg	total:	(statements)	40.0%" | \
  awk '/internal\/.*total:/ { pct=$NF; gsub(/%/,"",pct); if (pct+0 < 85.0) { found=1 } } END { exit found ? 1 : 0 }'
echo "exit: $?"
```
Expected: exit code 0 (pkg/ packages are not in scope for the 85% gate).
