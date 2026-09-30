# Comment history: `acs/cycle274`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle274/predicates_test.go:3` — above `package cycle274`

```text
// Package cycle274 materializes the cycle-274 acceptance criteria for the three
// committed top_n tasks (triage-report.md):
//
//	T1  bridge-transport-manifest         — callers stop branching on CLI-name
//	                                          strings; transport is manifest data
//	G   inserted-phase-treediff-guard-gap — a phase that writes the main tree
//	                                          outside its worktree FAILs the cycle
//	                                          REGARDLESS of phase identity
//	C   bridge-coverage-95                — go/internal/bridge total >= 95%
//
// These predicates are BEHAVIORAL (cycle-85 lesson): the load-bearing
// correctness checks (C274_003..C274_007) RUN the system under test as a real
// subprocess — `go test` over the bridge / core packages and a `go tool cover`
// total — and assert on the real `--- PASS:` lines, sub-case counts, and the
// coverage number. A magic string in a .go file cannot produce a named PASS
// line nor move the coverage total, so none of these is gameable by source
// editing alone.
//
// Two structural predicates carry explicit waivers because they assert an
// INVARIANT over source rather than a magic-string presence:
//   - C274_001 is a structural-ABSENCE check (the refactor must REMOVE the
//     `HasSuffix(..,"-tmux")` leak sites) — un-gameable by adding text, it can
//     only pass once the leaks are gone. This is scout T1's exact verifiableBy.
//   - C274_002 is a config-presence check (the manifest data file declares the
//     new single-source `transport` field) — the allowed-with-waiver category.
//
// AC map (1:1 with the architecture-design Requirements R1–R9):
//
//	R1            → TestC274_001 (no CLI-name transport leaks outside bridge)
//	R2,R4         → TestC274_002 (manifests declare transport: single source)
//	R2,R3         → TestC274_003 (IsTmuxDriver/IsTmux classify + preserve ""→false)
//	R9            → TestC274_004 (extracted isLegitimateMainTreePath classifier)
//	R5,R6         → TestC274_005 (cycle-270 replay: inserted/untracked leak FAILs)
//	R7            → TestC274_006 (no-fire companion: legit .evolve workspace write)
//	R8            → TestC274_007 (bridge total statement coverage >= 95%)
```

### `go/acs/cycle274/predicates_test.go:81` — above `func runCoreSuite(t *testing.T) string {`

```text
// runCoreSuite runs the orchestrator (core) guard tests ONCE per process. The
// builder's cycle-270 replay + classifier + no-fire companion land here.
```

### `go/acs/cycle274/predicates_test.go:128` — above `func TestC274_001_NoCLINameTransportLeaks(t *testing.T) {`

```text
// --- C274_001 (R1): no package outside bridge branches on a CLI-name string ---
//
// acs-predicate: structural-absence — this is NOT a magic-string presence grep
// (the cycle-85 ban); it asserts the refactor REMOVED the four
// `HasSuffix(..,"-tmux")` leak sites + the `isTmuxFamilyCLI` helper from
// swarm/adapters/looppreflight. Adding text can never make it pass — only
// deleting the leaks can. Scout T1's exact verifiableBy. RED at baseline: 4
// HasSuffix sites + the helper definition still present.
```

### `go/acs/cycle274/predicates_test.go:235` — above `func TestC274_005_GuardCatchesInsertedUntrackedLeak(t *testing.T) {`

```text
// --- C274_005 (R5,R6): cycle-270 replay — an INSERTED, non-worktree phase that
// writes a NEW UNTRACKED main-tree file FAILs the cycle with the path named ---
//
// THE core gap. Behavioral: the builder's `TestGuardCatchesInsertedPhaseLeak`
// drives the orchestrator (real git repo + a fake inserted/non-worktree runner
// that writes an untracked .go file into the main tree) and asserts the cycle
// aborts with the leaked path named. Requiring >= 2 passing sub-cases forces
// BOTH the untracked-file leak (R6, porcelain granularity — the tracked-only
// `git diff --name-only HEAD` baseline missed it) AND the inserted-phase-
// identity dimension (R5 — the phase is neither tdd/build nor a WritesSource
// catalog phase, the exact cycle-270 escape). RED: test absent.
```
