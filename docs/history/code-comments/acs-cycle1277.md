# Comment history: `acs/cycle1277`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1277/predicates_test.go:3` — above `package cycle1277`

```text
// Package cycle1277 materialises the cycle-1277 acceptance criteria.
//
// Fleet scope pins this lane to one todo-id, `retro-fleet-stale-worktree-fallback`,
// and triage committed exactly one task from it (`triage-report.md` ## top_n):
//
//	wire-c1270-stale-worktree-regression → 001, 002, 003 (+ 004 as anti-goal)
//
// The subject is NOT the fix. Cycle-1255 defect D1 (CRITICAL — "retroWorktree
// gates the scratch-cwd fallback on req.Worktree != "", so a torn-down lane's
// stale worktree loses its retro") is landed at go/internal/phases/retro/retro.go
// and proven end-to-end by TestC1270_006/007. The subject is that the proof does
// not RUN: CI's durable ACS gate walks exactly one glob —
//
//	.github/workflows/ci.yml:57  →  go test -count=1 -tags acs ./acs/regression/...
//	go/Makefile:108 (test-acs-durable) → the same command
//
// — and `go/acs/cycle1270` sits outside it, an orphaned sibling under go/acs/
// alongside ~250 other never-promoted cycle packages. So the D1 guard is
// green-by-skip: it passes when someone runs it by hand and enforces nothing.
//
// That distinction is what these predicates are built to catch, and it is why
// none of them asserts on the CONTENT of the moved file. A predicate that
// grepped `go/acs/regression/cycle1270/predicates_test.go` for a test name would
// pass on a hand-copied stub that never executes; 001 and 003 instead RUN the
// CI-enforced command and require the real `--- PASS:` lines for the two named
// tests, so a stub, an empty file, or a `-run` pattern that matches nothing all
// stay RED. `go test -run` exits 0 while printing "no tests to run", so exit
// status alone is not evidence here and is never the sole assertion.
//
// 002 is the negative half: the failure mode of a "move" is a COPY. A duplicated
// package leaves the orphan in place (still unenforced, now divergent) while the
// promoted copy goes green, so the predicate demands exactly one declaration
// site for TestC1270_006 across the whole go/acs tree, on disk AND in the git
// index, plus all nine TestC1270_* functions at the new site — a cherry-picked
// two-test extract is a different artifact than the one cycle-1270 shipped.
//
// 004 is an anti-goal and is expected GREEN at RED time (recorded as
// pre-existing GREEN in test-report.md). This task wires coverage; it must not
// re-touch the fix. If the retro fallback contract regresses while the wiring
// lands, 004 says so.
//
// No new package under ./internal/... is created here (go/acs/regression/cycle1270
// is test-only, outside ./internal/...), so ADR-0069's repo-wide apicover
// enrollment does not apply — the same exemption acs/regression/noorphan and
// acs/regression/flagreaders document.
```

### `go/acs/cycle1277/predicates_test.go:61` — above `promotedPkg = "go/acs/regression/cycle1270"`

```text
// promotedPkg is the CI-reachable location the cycle-1270 predicates must end up in.
```

### `go/acs/cycle1277/predicates_test.go:99` — above `func TestC1277_001_D1ProofExecutesUnderTheCIEnforcedGlob(t *testing.T) {`

```text
// TestC1277_001_D1ProofExecutesUnderTheCIEnforcedGlob is the criterion:
// TestC1270_006/007 execute under the glob CI actually walks.
//
// It establishes that in three linked steps rather than by shelling the durable
// tier's whole-subtree sweep. Running `go test ./acs/regression/...` here would
// be the most literal restatement of the CI command, but it is also the shape
// the host's flaky-predicate lint bans on evidence — a recursive sweep inside a
// cycle predicate is contention-sensitive under fleet load and produced the
// false REDs of cycles 1173/1175/1178. Sidestepping that lint by splitting the
// pattern string would be worse than either option, so the coverage claim is
// decomposed into checks that each stand on their own:
//
//  1. the enforced command really is `-tags acs ./acs/regression/...`, read out
//     of CI's own config and the Makefile target rather than assumed;
//  2. the promoted package resolves as a real acs-tagged package at a path that
//     glob covers (`go list`, one named package — a directory of .go files that
//     do not build, or whose build tag was lost, does not resolve);
//  3. the two D1 tests actually PASS there.
//
// Step 3 asserts on the `--- PASS:` lines, not the exit code. `go test -run`
// with a pattern that matches nothing exits 0 while printing "testing: warning:
// no tests to run" — which is exactly today's state, and exactly what a
// hand-copied stub would leave behind. Exit code alone would score that green.
```

### `go/acs/cycle1277/predicates_test.go:168` — above `func TestC1277_002_OrphanLocationIsGoneNotDuplicated(t *testing.T) {`

```text
// TestC1277_002_OrphanLocationIsGoneNotDuplicated is the negative half: a move,
// not a copy.
//
// Three distinct ways the wiring can be faked, each checked:
//   - the orphan stays on disk (proof duplicated, orphan still unenforced),
//   - the orphan stays in the git index while gone from disk (CI checks out the
//     index, so a disk-only delete regrows the duplicate on a fresh clone),
//   - only the two D1 tests are extracted (the promoted package is then a
//     different artifact than the one cycle-1270 shipped and audited).
```

### `go/acs/cycle1277/predicates_test.go:260` — above `func TestC1277_003_PromotedPackageVetsAndPassesInPlace(t *testing.T) {`

```text
// TestC1277_003_PromotedPackageVetsAndPassesInPlace closes the gap 001 cannot:
// 001 narrows with -run, so it never exercises the other seven cycle-1270
// predicates that ride along with the move. This runs the promoted package
// whole — one named package, never a ./... sweep — and vets it, so a package
// clause, import path, or build-tag left inconsistent by the move is caught
// here instead of on CI.
```
