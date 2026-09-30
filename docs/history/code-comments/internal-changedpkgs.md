# Comment history: `internal/changedpkgs`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/changedpkgs/changedfiles.go:22` — above `func ChangedFilesChecked(repoRoot, baseRef string) ([]ChangedFile, bool) {`

```text
// ChangedFilesChecked is the ONE git derivation of "what does this tree change
// versus baseRef"; FromGitChecked and the ship gate's backstops are projections
// of it. It reads the working tree, never the index: a lane's build output is
// unstaged, and its new files untracked, until the ship itself stages them —
// an index-based seed at gate time sees nothing (the 2026-09-14 ship-gate
// incident). ok is false whenever git could not answer: empty inputs, no
// repository, a bad ref, a concurrent index.lock — never "nothing changed".
```

### `go/internal/changedpkgs/changedpkgs.go:1` — above `package changedpkgs`

```text
// Package changedpkgs derives the set of Go package patterns touched by a
// cycle, from the builder's handoff-build.json, so an EGPS predicate can run
// `go test` scoped to the changed packages (O(change)) instead of the whole
// repo (`go test ./...`, O(repo)) — the latter exceeds the per-predicate
// timeout on a large repo and flakes to a false RED (cycle-200; the
// EVOLVE_ACS_PREDICATE_TIMEOUT_S band-aid only widens the window).
//
// The package list is exported to predicates as the CHANGED_PACKAGES env var
// (a Go predicate can scope its `go test` to it). All functions are pure +
// best-effort: an absent/unparseable handoff yields an empty list (the predicate
// then falls back to its own scope), never an error.
```

### `go/internal/changedpkgs/changedpkgs.go:96` — above `func FromGit(repoRoot, baseRef string) []string {`

```text
// FromGit derives the changed-package set deterministically from git — the
// Rule-5 replacement for the LLM-emitted handoff-build.json, which has been
// extinct since ~cycle 215 and left the apicover CI-parity gate silently
// fail-open on every real cycle. It returns the sorted, deduped go test patterns
// for .go files that differ between baseRef and the working tree: tracked
// modifications (`git diff --name-only <baseRef>`) plus untracked new files
// (`git ls-files --others --exclude-standard`), each mapped through
// FileToPackage. Best-effort like the rest of this package: any git error yields
// an empty list (the caller falls back), never a panic. Callers that must
// distinguish "0 files changed" from "git failed" should use FromGitChecked.
```

### `go/internal/changedpkgs/changedpkgs.go:111` — above `func FromGitChecked(repoRoot, baseRef string) ([]string, bool) {`

```text
// FromGitChecked is FromGit plus a derivability signal: it returns
// (pkgs, derivable) where derivable is false whenever the changed-package set
// could NOT be trusted — an empty repoRoot/baseRef (a config error, not a
// verified-clean tree) or ANY git invocation failing (no repo, bad baseRef, a
// concurrent-fleet `.git/index.lock` race). A clean tree that git reports
// successfully is (nil, true): genuinely nothing changed, NOT underivable.
//
// FromGit's swallow-every-error behavior conflates those two cases, which makes
// the apicover CI-parity gate fail-open on the very cycle that most needs it
// (cycle-581 audit D1/D2, warnship_apicover_ci_gap 3rd recurrence). A gate that
// must FAIL loud on an underivable set uses this; the CHANGED_PACKAGES predicate
// path keeps FromGit's best-effort empty-on-error contract.
```

### `go/internal/changedpkgs/changedpkgs_derivability_test.go:3` — above `import (`

```text
// changedpkgs_derivability_test.go — RED contract for cycle-582's
// changedpkgs-derivability-failloud task (scout-report.md Task 1).
//
// FromGit swallows every git error internally (`if out, err := …; err == nil {
// add(out) }`), so a caller cannot distinguish "0 files changed" (git-clean
// tree) from "git command failed" (underivable — e.g. concurrent-fleet
// `.git/index.lock`, a non-git worktree, or a bad baseRef). That conflation is
// exactly what makes the apicover CI-parity gate fail-open (cycle-581 audit
// D1/D2, warnship_apicover_ci_gap 3rd recurrence).
//
// FIX CONTRACT (new surface this cycle — undefined until Builder adds it, so
// this file fails to compile today; that compile failure IS the RED
// evidence):
//
//   - FromGitChecked(repoRoot, baseRef string) (pkgs []string, derivable bool)
//     propagates git diff/ls-files errors into derivable=false instead of
//     swallowing them. derivable=true whenever every git invocation the
//     function needs succeeded (even if the resulting package set is empty on
//     a clean tree).
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Negative: TestFromGitChecked_NonGitRepo_ReturnsUnderivable (the
//     strongest anti-no-op signal — a naive derivable-always-true
//     implementation fails this)
//   - Edge:     TestFromGitChecked_EmptyArgs_ReturnsUnderivable
//   - Semantic: TestFromGitChecked_CleanRepo_ReturnsDerivableEmpty (derivable
//     but zero packages — must not be conflated with the negative case) and
//     TestFromGitChecked_TrackedGoChange_ReturnsDerivableWithPackages (a real
//     tracked change is both derivable AND non-empty)
```

### `go/internal/changedpkgs/covering_tests_test.go:1` — above `package changedpkgs`

```text
// RED contract for cycle-1255 task `test-amplification-covering-tests-scope`.
//
// test-amplification is the pipeline's per-run context outlier (81.2M cache-read
// tokens / 15 runs = 5.4M per run, 13.5 min avg — knowledge-base/research/
// token-usage-history-2026-07-20.md). Root cause: its phase spec declares only
// tdd-contract.md + build-report.md as inputs, and the agent is forbidden from
// reading diffs/implementation, so it must Grep/Glob the WHOLE repo to discover
// which existing test files cover the paths it was handed.
//
// The fix: derive the covering-test set deterministically (changed packages →
// their _test.go files) and inject it as an explicit input, so the agent is TOLD
// the in-scope tests instead of searching for them. The black-box constraint is
// untouched: a list of test-file PATHS is not the diff and not the implementation.
//
// CoveringTests is the deriver. Contract pinned below:
//
//	CoveringTests(repoRoot string, pkgPatterns []string) []string
//
// It maps go-test package patterns (the exact strings ChangedPackages/FromGit
// already emit, e.g. "./internal/foo/...") to the sorted, deduped, repo-relative
// slash-separated paths of the _test.go files in those packages. It is fail-open
// like the rest of this package: any unusable input yields nil, never an error
// and never a panic — the phase then behaves exactly as it does today.
//
// Import-shape probe (the cycle-644 obligation): this test pins the symbol from
// INSIDE package changedpkgs (no new import at all), and the reachability test
// below resolves callers from the parsed import graph rather than by importing
// them, so no new edge is introduced in either direction. changedpkgs imports
// only internal/gitexec, so a caller in core/phases/router adds no cycle.
```

### `go/internal/changedpkgs/direct_importers_test.go:3` — above `import (`

```text
// RED contract for cycle-1267 Task 1 (`scope-test-amplification-context`,
// inbox test-amplification-context-scope, w=0.89) — the HALF that is still
// missing.
//
// The inbox item's how_to_apply spells the corpus as:
//
//	packages touched by the cycle diff -> their *_test.go files
//	                                   +  direct reverse-import test packages
//
// The first half landed (CoveringTests, covering_tests.go, cycle-1255). The
// second half did NOT: CoveringTests walks ONLY the changed packages' own
// directories, so a package whose *_test.go imports the changed package — the
// literal definition of a covering test — is still invisible to the
// amplification agent, which then falls back to the whole-repo Grep this task
// exists to remove. There is no reverse-dependency seam in the module today
// (the cycle-1267 fault-localization report cites a
// `changedpkgs.ImporterClosure`; it does not exist — verified by grep over
// go/internal, go/cmd and go/pkg).
//
// This file pins the missing seam:
//
//	DirectImporters(repoRoot string, pkgPatterns []string) []string
//
// Given the same go-test package patterns the rest of this package speaks
// ("./internal/foo/..." or the bare "./internal/foo"), it returns the sorted,
// deduped, bare-form patterns of the module packages that DIRECTLY import any
// of them — counting imports from *_test.go files, which is precisely how a
// covering test package depends on the code it covers. The input packages
// themselves are never returned (they are already in the corpus), and
// transitive importers are not (DIRECT means one hop: the item says "direct
// reverse-import", and an unbounded closure would re-inflate the very context
// this task shrinks).
//
// Fail-open, like every other deriver here: any unusable input yields nil —
// never an error, never a panic — and the corpus degrades to exactly today's
// changed-packages-only set.
//
// Import-shape probe (the cycle-644 obligation): every symbol pinned here is
// pinned from INSIDE package changedpkgs, so no new import edge is introduced
// in either direction; the reachability test below resolves its caller from the
// parsed import graph rather than by importing it. changedpkgs imports only
// internal/gitexec and internal/gopkgpattern, so the production caller this
// contract requires (internal/core, which already imports changedpkgs) adds no
// cycle. Confirmed against the current graph before freezing this pin.
```

### `go/internal/changedpkgs/direct_importers_test.go:224` — above `func TestDirectImporters_ReachableFromProduction(t *testing.T) {`

```text
// TestDirectImporters_ReachableFromProduction — AC6, the WIRING proof. A widening
// seam whose only caller is a test injects nothing into the phase and saves zero
// tokens (the cycle-1255 precedent for CoveringTests itself). Callers are
// resolved from the parsed import graph of the whole go/ module: at least one
// NON-test file outside package changedpkgs — and outside go/acs, whose
// predicates are the gate, not the product — must reference
// changedpkgs.DirectImporters.
```

### `go/internal/changedpkgs/fromgit_test.go:10` — above `func gitCmd(t *testing.T, dir string, args ...string) {`

```text
// fromgit_test.go — RED contract for cycle-573 Task 2
// (builder-handoff-extinct-deterministic-changedpkgs, inbox weight 0.96
// critical; standing memory warnship_apicover_ci_gap, 3rd recurrence).
//
// Today changedPackagesForAudit derives the cycle's changed-package set from an
// LLM-emitted handoff-build.json that has been extinct since ~cycle 215, so the
// apicover CI-parity gate is silently fail-open (nil, nil) on every real cycle.
// Rule 5 (deterministic work must not depend on an LLM artifact) says the source
// must be pure git. This task adds changedpkgs.FromGit(repoRoot, baseRef) — the
// deterministic replacement: the set of go test patterns for .go files that
// differ between baseRef and the working tree (tracked edits + untracked new
// files), mapped through the existing FileToPackage.
//
// RED today: FromGit is undefined, so this whole package fails to COMPILE — the
// intended RED signal (a compile failure is a hard non-zero exit, never a silent
// pass). GREEN once Builder adds FromGit.
```

### `go/internal/changedpkgs/importerclosure.go:36` — above `func ImporterClosure(repoRoot string, pkgs []string) []string {`

```text
// ImporterClosure widens a changed-package set with its REVERSE dependencies:
// the sorted, deduped union of pkgs and a "./dir/..." pattern for every module
// package that transitively imports one of them — through its build
// dependencies or through the imports of its own tests (an untouched
// `_test.go` asserting a changed package's contract is the 2026-09-14 ship-gate
// incident; a routingtest that imports router is the cycle-1250 one).
//
// Every other derivation in this package is forward-only — FileToPackage maps a
// changed file to the package it lives in, and nothing walks the import graph.
// Test-impact selection built on a forward-only set silently hides that whole
// regression class.
//
// repoRoot is the REPOSITORY root (the dir containing the go/ module dir), the
// same parameter meaning as FromGit/FromGitChecked. pkgs are "./dir/..."
// patterns as emitted by FileToPackage.
//
// Best-effort, like the rest of this package: an empty or nonexistent repoRoot,
// a junk pattern, or any `go list` failure yields the input set unchanged (an
// EMPTY added closure) — never an error, never a panic, never a lost input
// entry. Closure only ever widens; narrowing below the forward-only baseline
// would be strictly worse than not having this function at all. Callers that
// must distinguish "nothing imports it" from "go list failed" use
// ImporterClosureChecked.
```

### `go/internal/changedpkgs/importerclosure_test.go:9` — above `func repoRootForTest(t *testing.T) string {`

```text
// importerclosure_test.go — RED contract for cycle-1253 Task 1
// (`tia-importer-closure`, from inbox item
// .evolve/inbox/2026-07-30T09-00-00Z-egps-regression-tia-selection.json,
// P1 weight 0.91, 3rd live instance).
//
// The defect. Every derivation in this package is FORWARD-ONLY: FileToPackage
// maps a changed file to the package it LIVES in, and ChangedPackages/FromGit/
// FromGitChecked never walk the import graph. So a change confined to
// `internal/router` never selects `internal/routingtest` — even though
// routingtest imports router and holds the keystone parity invariant. That is
// exactly the cycle-1250 miss: main stayed red for 5 commits because the only
// thing that would have caught it was a package the changed-package set could
// not name. Test-impact selection built on a forward-only set silently hides a
// whole regression class.
//
// The contract these tests freeze:
//
//	func ImporterClosure(repoRoot string, pkgs []string) []string
//
//   - repoRoot is the REPOSITORY root (the dir containing the `go/` module
//     dir) — same parameter meaning as FromGit/FromGitChecked, so callers that
//     already hold one can pass it straight through.
//   - pkgs are `./dir/...` go test patterns as emitted by FileToPackage.
//   - the result is the sorted, deduped UNION of the input patterns and a
//     `./dir/...` pattern for every module package that TRANSITIVELY imports
//     any input package. The input is never dropped: closure only ever widens.
//   - best-effort, exactly like the rest of this package: an empty or
//     nonexistent repoRoot, a junk pattern, or any `go list` failure yields the
//     input set unchanged (an EMPTY added closure) — never an error, never a
//     panic, never a lost input entry.
//
// RED today: ImporterClosure is undefined, so this package fails to COMPILE —
// a hard non-zero exit, never a silent pass. GREEN once Builder adds it.
```

### `go/internal/changedpkgs/importerclosure_test.go:57` — above `func TestImporterClosure_RouterRoutingtest(t *testing.T) {`

```text
// TestImporterClosure_RouterRoutingtest is the cycle-1250 reproducer and the
// crux of this task: a change confined to internal/router MUST select
// internal/routingtest, because routingtest imports router (non-test edge, in
// agent.go/bricks.go/engine.go) and owns the keystone parity test that a
// forward-only set never runs. The input pattern must also survive.
```

### `go/internal/changedpkgs/importerclosure_test.go:176` — above `func TestImporterClosure_TestOnlyImporter(t *testing.T) {`

```text
// TestImporterClosure_TestOnlyImporter is the 2026-09-14 ship-gate shape: a
// package whose NON-test code never imports the changed package, but whose
// tests do, is linked into a test binary the change can break. internal/
// routingeval's tests import internal/core (its build deps do not), so a
// change confined to core must select routingeval; a .Deps-only walk cannot.
```
