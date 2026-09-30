# Comment history: `acs/cycle969`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle969/predicates_test.go:3` — above `package cycle969`

```text
// Package cycle969 materializes the cycle-969 acceptance criteria for the sole
// committed task of this fleet lane, wire-carryforward-prune-cli (triage
// top_n; fleet_scope pins this lane to the todo-id prune-superseded-orphans-
// lane, so per R9.3 no predicates bind to any other lane's items).
//
// DEFECT CLOSED. cycle-962 shipped two fully-implemented, unit-tested exports
// in go/internal/core — CarryforwardCandidateLandable (carryforward_filter.go)
// and PruneSupersededOrphans (prune_superseded_orphans.go) — with ZERO non-test
// production callers and no CLI surface to invoke either. This cycle adds an
// `evolve branches audit|prune` subcommand (new go/cmd/evolve/cmd_branches.go,
// registered in registry.go mirroring the `worktree` row) that dispatches to
// BOTH core functions, giving them their first live caller and letting an
// operator actually walk the stale orphan `cycle-*` ref backlog.
//
// SUT surface the Builder must add WITHOUT modifying this file:
//   - go/cmd/evolve/cmd_branches.go: func runBranches(args []string, stdin
//     io.Reader, stdout, stderr io.Writer) int — dispatches `audit` and `prune`
//     subcommands (mirrors runWorktree). `audit` is read-only and prints, per
//     local cycle-* branch, BOTH a supersession verdict (via
//     core.PruneSupersededOrphans / its refSuperseded screen) and a
//     carry-forward verdict (via core.CarryforwardCandidateLandable). `prune`
//     defaults to dry-run (deletes nothing) and deletes a superseded branch
//     ONLY under explicit --dry-run=false AND when hasOpenPR reports false.
//   - registry.go: a {Name:"branches", Run: runBranches} row.
//
// OUTPUT CONTRACT the predicates below bind (also in agent-mailbox.md):
//
//	audit                 → one line per branch: `<ref> superseded=<t|f> landable=<t|f>`
//	prune (dry-run)       → lists each superseded ref (`<ref> ... would-prune`), deletes nothing
//	prune --dry-run=false → deletes each superseded ref with no open PR (`<ref> ... pruned`)
//
// hasOpenPR DETERMINISM (required contract, surfaced per Core Rule 3): the
// prune fixtures below are git repos with NO configured remote. hasOpenPR MUST
// degrade to (false, nil) when there is no remote / `gh` is unavailable — a
// branch provably has no reachable remote PR, so a locally-superseded ref is
// safe to delete (this is exactly verify_remote_pr_before_branch_delete: we
// deleted only after confirming no open PR). A hasOpenPR that ERRORS on a
// no-remote repo leaves C969_004 RED — legitimate TDD pressure toward the
// graceful-degradation the tool needs to run in CI.
//
// PREDICATE STYLE (cycle-85 rule): every predicate BUILDS the real `evolve`
// binary and RUNS the new subcommand against a REAL git repo built in a temp
// dir, asserting on exit code + emitted stdout + the post-run branch list — no
// source-grep predicate exists here. A correct `evolve branches audit` output
// necessarily proves the CLI calls BOTH core functions (their verdicts appear
// in stdout), so these behavioral predicates ARE the caller-existence proof
// that closes the inert-API gap. RED before the Builder acts: `evolve branches`
// is an unknown subcommand → non-zero exit / empty stdout → every assertion
// fails for the right reason (the subcommand is absent). Each predicate folds
// exit==0 into its assertion so none can false-green on the pre-impl repo.
//
// Adversarial diversity (skills/adversarial-testing §6):
//
//	POSITIVE → C969_001 (audit reports a superseded branch — anti-`always-false`
//	           for PruneSupersededOrphans), C969_002 (audit's landable column
//	           distinguishes superseded vs divergent — binds the SECOND core
//	           func with two distinct outcomes), C969_004 (--dry-run=false
//	           actually deletes — the anti-no-op for prune).
//	NEGATIVE → C969_003 (default prune is dry-run: a superseded branch SURVIVES
//	           — the strongest safety signal; a prune ignoring the dry-run
//	           default would delete it), C969_005 (--dry-run=false never deletes
//	           a NON-superseded divergent branch — anti-overreach).
//	EDGE     → superseded-via-ancestor vs divergent-clean fixtures exercise both
//	           supersession paths and the clean-merge landable path.
//	SEMANTIC → audit(read-only report) / prune-dry-run(report, no delete) /
//	           prune-force(delete) are DISTINCT behaviors, each asserted apart.
//
// AC map (1:1 with the disposition table in test-report.md):
//
//	AC1 subcommands exist, registered, dispatch to BOTH core funcs → C969_001 + C969_002
//	AC2 prune dry-run default; --dry-run=false deletes only non-open-PR   → C969_003 + C969_004 + C969_005
//	AC3 unit tests red-first + caller-existence for both funcs            → C969_001/002 (live callers proven behaviorally) + manual (unit suite)
//	AC4 go vet ./... and go test ./cmd/evolve/... green, no regression    → manual+checklist (Auditor CI-parity)
```

### `go/acs/cycle969/predicates_test.go:152` — above `func fixtureRepo(t *testing.T) string {`

```text
// fixtureRepo builds a fresh git repo (NO remote) on branch `main` with:
//
//	cycle-100 — SUPERSEDED: branched from an earlier main commit, then main
//	            advanced past it, so cycle-100 is a strict ancestor of main
//	            (refSuperseded true; CarryforwardCandidateLandable false).
//	cycle-200 — DIVERGENT: a unique, cleanly-mergeable commit not on main
//	            (refSuperseded false; CarryforwardCandidateLandable true).
```

### `go/acs/cycle969/predicates_test.go:168` — above `git(t, dir, "branch", "cycle-100")`

```text
// cycle-100 at the current (soon-to-be-ancestor) tip.
```

### `go/acs/cycle969/predicates_test.go:171` — above `git(t, dir, "checkout", "-q", "-b", "cycle-200")`

```text
// cycle-200 diverges with a unique, non-conflicting file.
```

### `go/acs/cycle969/predicates_test.go:176` — above `writeCommit(t, dir, "base.txt", "line1\nline2\n", "advance main")`

```text
// main advances past cycle-100, making cycle-100 a strict ancestor.
```

### `go/acs/cycle969/predicates_test.go:228` — above `func TestC969_002_AuditLandableColumnDistinguishes(t *testing.T) {`

```text
// --- C969_002 (AC1/AC3, POSITIVE/SEMANTIC): audit's landable column binds the
// SECOND core func (core.CarryforwardCandidateLandable) with two DISTINCT
// outcomes — superseded cycle-100 is landable=false, divergent-clean cycle-200
// is landable=true. A no-op that hardcodes one value cannot satisfy both.
```

### `go/acs/cycle969/predicates_test.go:247` — above `func TestC969_003_PruneDryRunDefaultKeepsSuperseded(t *testing.T) {`

```text
// --- C969_003 (AC2, NEGATIVE): default `prune` (no flags) is DRY-RUN — the
// superseded cycle-100 SURVIVES. Folds exit==0 into the assertion so it cannot
// false-green pre-impl (an unknown subcommand fails, leaving cycle-100 present
// too — the exit check rejects that).
```

### `go/acs/cycle969/predicates_test.go:266` — above `func TestC969_004_PruneForceDeletesSuperseded(t *testing.T) {`

```text
// --- C969_004 (AC2, POSITIVE): `prune --dry-run=false` DELETES the superseded
// cycle-100 (no remote → no open PR → safe to delete). The anti-no-op for the
// actual prune path.
```

### `go/acs/cycle969/predicates_test.go:281` — above `func TestC969_005_PruneForceKeepsDivergent(t *testing.T) {`

```text
// --- C969_005 (AC2, NEGATIVE): `prune --dry-run=false` NEVER deletes a
// NON-superseded (divergent) branch — cycle-200 SURVIVES. Anti-overreach:
// pruning must be gated on the supersession screen, not blanket-delete cycle-*.
```
