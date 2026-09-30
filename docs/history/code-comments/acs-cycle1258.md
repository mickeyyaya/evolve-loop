# Comment history: `acs/cycle1258`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1258/predicates_test.go:3` — above `package cycle1258`

```text
// Package cycle1258 materialises the cycle-1258 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//	artifact-ready-crosspoll-debounce — artifactDetector.poll must not declare a
//	phase complete on the FIRST tick it sees a non-empty deliverable; the
//	(path, size, mtime) key must be observed unchanged across artifactStableTicks
//	consecutive wait-loop ticks first.
//
// # Standing state this lane inherited (read this before diagnosing a red)
//
// The production change is ALREADY PRESENT in this worktree's base commit
// (29472ec3, the ADR-0076 continuation-on-fail salvage snapshot): completion.go
// carries artifactStableTicks and the cross-poll window, salvaged forward from
// the cycle-1233 → 1249 → 1252 → 1254 chain. Scout re-derived the gap from the
// deliverable.go:178-181 comment and did not observe that the fix had landed.
//
// That does NOT make these predicates ceremonial. They are the REGRESSION LOCK
// on a fix that has now been salvaged across four cycles without ever being
// bound by a cycle predicate — precisely the shape that lets a rebase, a
// re-salvage, or a "simplify the detector" refactor silently reopen cycle-1198.
// Predicate 005 is the genuine RED: the permanent eval entry that survives THIS
// cycle (.evolve/evals/artifact-ready-crosspoll-debounce.md) does not exist, so
// nothing caps a future audit when this behaviour breaks.
//
// # Predicate strategy
//
// artifactDetector, artifactStableTicks and runTmuxREPL are all UNEXPORTED in
// internal/bridge, so a predicate in this package cannot call them directly.
// Each behavioural predicate therefore drives the real production code through a
// NARROW, named-package `go test` subprocess (never a `./...` sweep, never a
// known-slow suite — see the flaky-predicate-shape rules), with cmd.Dir pinned
// to the worktree's go/ directory rather than inherited from process cwd.
//
//	001 → the stability window itself: positive + BOTH negative axes (size, mtime)
//	002 → the caller proof: the debounce is reached from the production wait loop
//	003 → no false-timeout regression + the window constant is a real window
//	004 → AC-3: the window gained no operator dial (registry is the SSOT)
//	005 → the permanent eval entry exists, is well formed, and its evidence RUNS
```

### `go/acs/cycle1258/predicates_test.go:135` — above `func TestC1258_001_ArtifactDebounceStabilityWindow(t *testing.T) {`

```text
// TestC1258_001_ArtifactDebounceStabilityWindow is AC-1, all three axes at once.
// It drives the REAL artifactDetector against a real temp-dir artifact:
//
//   - positive — a settled file completes, but never on the tick it is first
//     seen (cycle-1198's truncated deliverable was a perfectly non-empty file on
//     exactly that tick);
//   - negative, SIZE axis — a still-growing deliverable never completes, however
//     many ticks pass. This is the anti-no-op assertion: a detector that keeps
//     firing on first sight passes the positive half and fails here;
//   - negative, MTIME axis — an equal-length fix-up Edit (a flipped verdict word)
//     must also reset the window. A size-only key silently degrades back to the
//     cycle-1198 bug for exactly that shape, which is why mtime is in the key.
```

### `go/acs/cycle1258/predicates_test.go:163` — above `func TestC1258_002_ArtifactDebounceWiredIntoWaitLoop(t *testing.T) {`

```text
// TestC1258_002_ArtifactDebounceWiredIntoWaitLoop is the CALLER PROOF, and it is
// the predicate that matters most. A stability window implemented on a struct
// that no production path reaches is dead code that greens 001 and changes
// nothing at runtime. The tests it drives go through Engine.LaunchArgs →
// runTmuxREPL → detector.poll — the real wait loop — and assert that a
// deliverable rewritten on every tick exits ExitArtifactTimeout rather than
// ExitOK.
//
// The hermetic twin is bound here deliberately rather than left to general
// hygiene: cycles 1252 and 1254 were both FAILed at audit by this exact caller
// proof going red inside the ACS gate (which shells `go test` with no cmd.Env
// and so inherits the orchestrator's EVOLVE_FLEET=1) while every interactive run
// was green. The invariant that fixture construction must not read the ambient
// environment is part of this task's contract, not an unrelated cleanup.
```

### `go/acs/cycle1258/predicates_test.go:236` — above `func TestC1258_004_StabilityWindowIsNotConfigurable(t *testing.T) {`

```text
// TestC1258_004_StabilityWindowIsNotConfigurable is AC-3. The window is a
// compiled constant on purpose, matching readGraceWindow's "deliberately NOT
// configurable" convention already established in this codebase and the standing
// no-feature-flags rule: cross-component timing behaviour belongs in the code
// path, not in an env dial an operator can quietly set to 1 and reopen
// cycle-1198 in production without changing a line of source.
//
// The load-bearing assertion executes flagregistry.All — the SSOT the
// control-flags doc is GENERATED from and that `evolve flags check` enforces —
// so a dial registered anywhere on any reader surface is caught. The source-form
// check that follows is a secondary net for a dial added WITHOUT registering it
// (which would itself fail the registry drift test in normal CI).
```

### `go/acs/cycle1258/predicates_test.go:302` — above `func TestC1258_005_PermanentEvalEntryExistsAndItsEvidenceRuns(t *testing.T) {`

```text
// TestC1258_005_PermanentEvalEntryExistsAndItsEvidenceRuns is this cycle's
// genuine RED, and the reason the lane is worth running at all.
//
// ACS predicates are CYCLE-SCOPED: this whole package is read once, by cycle
// 1258's audit, and never replayed. The debounce has now been carried forward by
// salvage across cycles 1233 → 1249 → 1252 → 1254 with no permanent artifact
// binding it, which is exactly how a fix survives four cycles and still has
// nothing to stop the fifth from reverting it. The eval entry under
// .evolve/evals/ is the durable half: it caps a FUTURE cycle's audit score when
// this evidence stops holding.
//
// The predicate does not merely assert the file exists — a plausible stub would
// pass that. It EXECUTES every `evidence` command the eval declares and requires
// each to exit 0, so an eval whose evidence is aspirational, misspelled, or
// pointing at a renamed test fails here rather than silently capping nothing
// forever.
```

### `go/acs/cycle1258/predicates_test.go:356` — above `if !strings.Contains(string(raw), "1198") {`

```text
// The entry must name the incident it descends from, so a future reader can
// tell a load-bearing cap from a decorative one.
```
