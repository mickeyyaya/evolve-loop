# Comment history: `acs/cycle1441`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1441/predicates_test.go:3` — above `package cycle1441`

```text
// Package cycle1441 materialises the acceptance criteria for this lane's two
// fleet-scoped tasks (triage-report.md ## top_n):
//
//	salvage-extraction-stage-port   — land the stranded extraction/coercion stage
//	salvage-report-cli-and-docs     — a `saved` counter distinct from `recoverable`
//
// What this cycle is. Not new design: a LANDING. The instrumentation half of
// `schema-aligned-salvage-layer` (ClassifyBadVerdict, SummarizeBadVerdictBaseline,
// `evolve salvage report`) is already on main. The EXTRACTION half — the pass
// that repairs a sole, unambiguous, recoverable bad_verdict and re-verifies the
// repaired bytes before approving — is built, green, and stranded in ten+
// continuation worktrees, most advanced being .evolve/worktrees/cycle-42824668-1434
// (snapshot a2d65920). Nothing on main calls it; no PR was ever opened for it.
//
// Predicate strategy — wiring proof, not unit proof. 001-003 drive the REAL
// production entry point, `Reviewer.Review` (the contract gate), through the
// exported constructor, and assert on the gate's own decision plus the sidecar
// it wrote. They deliberately do NOT call SalvageVerdict directly: a salvage
// stage whose only caller is a test is dead code, and the entire defect this
// cycle closes is that the stage exists but no production path reaches it.
// 004-005 exercise the exported seams directly for the fail-closed and operator
// -surfacing contracts; 006 runs the ported package's own named tests; 007-008
// build and drive the real CLI binary. No predicate here is load-bearing on a
// source grep — the cycle-85 degenerate-predicate ban. The only greps present
// are auxiliary git-tracking checks (cycle-93: on-disk-but-untracked files are
// silently dropped at ship).
//
// RED baseline (this worktree, main-based):
//   - 001 fails: reviewer.go has no salvage call at all, so a sole recoverable
//     bad_verdict blocks and no salvage-applied.jsonl is ever written.
//   - 004/005 fail to COMPILE: deliverable.SalvageVerdict and
//     deliverable.SalvageSummaryLine do not exist on main. A predicate package
//     that fails to compile is a hard RED for the whole package (acs/README.md),
//     which is the correct signal here — the ported symbols are the deliverable.
//   - 006 fails: the ported test files are absent/untracked.
//   - 007/008 fail: `evolve salvage report -json` emits no `saved` key.
//   - 002/003 are the REGRESSION guards. They are pre-existing GREEN on main for
//     the trivial reason that main salvages nothing at all — so they are only
//     load-bearing AFTER the port, where they pin the two refusals a careless
//     port would drop: multi-violation salvage is a report-forgery bypass
//     (cycle-1392 CRITICAL-1) and multi-candidate salvage silently picks a
//     verdict the report never gave (cycle-1406 CRITICAL-1). They must stay
//     green THROUGH the landing; the port is not done if either flips.
```

### `go/acs/cycle1441/predicates_test.go:78` — above `const multiViolationFenced = "## Summary\n" +`

```text
// multiViolationFenced is the SAME recoverable verdict shape with the required
// "## Verdict" section removed, so bad_verdict co-occurs with missing_section.
// Salvage repairs the VERDICT and nothing else; acting here would erase the
// co-occurring violation wholesale (cycle-1392 audit CRITICAL-1).
```

### `go/acs/cycle1441/predicates_test.go:85` — above `const ambiguousFenced = "## Verdict\n" +`

```text
// ambiguousFenced carries TWO candidate verdict spans disagreeing on the
// outcome. The stage must refuse rather than pick one — approving here means the
// gate reports a verdict the report never unambiguously gave (cycle-1406
// audit CRITICAL-1).
```

### `go/acs/cycle1441/predicates_test.go:185` — above `func TestC1441_002_ReviewNeverSalvagesMultiViolation(t *testing.T) {`

```text
// TestC1441_002_ReviewNeverSalvagesMultiViolation — the anti-forgery regression
// guard (cycle-1392 audit CRITICAL-1). Salvage acts on the SOLE-violation case
// or not at all: a bad_verdict co-occurring with any other violation must fall
// through to block, because approving via the salvaged Result erases ALL
// violations, including the anti-forgery proof-of-read check.
```

### `go/acs/cycle1441/predicates_test.go:207` — above `func TestC1441_003_ReviewRefusesAmbiguousCandidates(t *testing.T) {`

```text
// TestC1441_003_ReviewRefusesAmbiguousCandidates — the ambiguity regression
// guard (cycle-1406 audit CRITICAL-1). Two candidate spans disagreeing PASS vs
// FAIL: silently picking one manufactures a verdict the report never gave.
```

### `go/acs/cycle1441/predicates_test.go:228` — above `func TestC1441_004_SalvageVerdictFailsClosedOnUnresolvablePhase(t *testing.T) {`

```text
// TestC1441_004_SalvageVerdictFailsClosedOnUnresolvablePhase — the edge/OOD
// case. A phase whose contract cannot be resolved cannot be re-verified, and
// salvage must therefore refuse: it may never flip OK from the classification
// alone, because that skips every content check the strict parse never reached
// (cycle-1392 MEDIUM-3). The refused Result must come back byte-identical.
```

### `go/acs/cycle1441/predicates_test.go:285` — above `func TestC1441_006_PortedSalvageSuiteGreenAndTracked(t *testing.T) {`

```text
// TestC1441_006_PortedSalvageSuiteGreenAndTracked runs the ported package's own
// salvage tests against ONE named package (never a ./... sweep — flaky-predicate
// shape rules) and pins that every ported file is git-TRACKED, not merely on
// disk: an untracked file is silently dropped at ship (cycle-93).
```
