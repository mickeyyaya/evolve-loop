# Comment history: `acs/cycle783`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle783/predicates_test.go:3` — above `package cycle783`

````text
// Package cycle783 materializes the cycle-783 acceptance criteria for the sole
// committed task of this fleet lane, verify-and-close-token-cache-fidelity
// (scout-report.md ## Selected Tasks; fleet_scope pins this lane to the todo-id
// token-telemetry-input-cache-fidelity, so per R9.3 no predicates bind to any
// other lane's items).
//
// Task nature: CONVERGENCE. Scout found the underlying feature task
// (token-telemetry-input-cache-fidelity, inbox weight 0.96, cycle-779 TDD
// contract) already fully implemented on main — all 8 cycle-779 ACS predicates
// green under -race. This cycle's job is to (a) re-verify that claim
// behaviorally and (b) durably record closure so the id stops being
// re-proposed, without re-implementing anything.
//
// SEAM CORRECTION (surfaced per Core Rule 3): scout-report.md proposes marking
// `decision: "completed"` in `.evolve/state.json:evaluatedTasks`, but that key
// exists nowhere — not in state.json and not in any Go source (grep
// evaluatedTasks/EvaluatedTasks over go/internal: zero hits). Predicates bind
// to seams that actually exist instead: the live inbox (the real re-proposal
// source; the item already sits in inbox/processed/cycle-779/) and this
// cycle's build-report closure record.
//
// AC map (1:1, from scout-report.md Selected Task 1 verifiableBy + Acceptance
// Criteria Summary + Deferred):
//
//	AC1 "go test -race -tags acs ./acs/cycle779/... reports 8/8 PASS"
//	    → C783_001 re-runs the cycle-779 suite as a subprocess and counts the
//	      eight individual "--- PASS: TestC779_" markers (a bare exit-0 could
//	      hide a skipped/renamed predicate). PRE-EXISTING GREEN by design —
//	      this IS the verification half of a verify-and-close task; bound so
//	      audit re-proves the claim instead of trusting the scout.
//	AC2 "record completion so the task stops being re-proposed"
//	    → C783_002 (closure record in this cycle's build-report naming the id,
//	      the 8/8 evidence, and the completed decision — RED until Builder
//	      writes it) + C783_003 (negative: no LIVE inbox item for the id —
//	      the actual re-proposal channel; processed/ items don't re-surface).
//	AC3 "evolve eval quality-check confirms the eval asserts real command
//	    output, not existence checks"
//	    → C783_004 runs the SSOT checker (internal/evalqualitycheck, the exact
//	      code behind `evolve eval quality-check`) against the task's eval
//	      file and requires Overall==PASS over a NON-EMPTY command set (an
//	      eval with no ```bash block passes vacuously — that hole is closed
//	      here). RED until the eval file exists with classified-PASS commands;
//	      authored during the TDD phase per Step 6b.
//	AC4 "live-soak: evolve tokens report shows non-zero input / cache-hit
//	    ratio against a real soaked batch"
//	    → manual+checklist in test-report.md (needs a live batch; carried
//	      over verbatim from the cycle-779 contract's own deferral).
//
// Adversarial axes: negative (C783_003 live-inbox absence; C783_004 rejects
// the vacuous zero-command PASS), edge (C783_001 rejects exit-0-with-fewer-
// than-8-PASS — rename/skip gaming), semantic (re-verification vs closure
// bookkeeping vs eval rigor are distinct behaviors). No source-grep
// predicates (cycle-85 rule): C783_001/004 execute the system under test;
// C783_002/003 assert on real emitted runtime artifacts (build-report,
// inbox), not source files.
````

### `go/acs/cycle783/predicates_test.go:90` — above `func stateRoot(t *testing.T) string {`

```text
// stateRoot resolves the MAIN project root (the STATE root): the suite exports
// EVOLVE_PROJECT_ROOT (issue #12), else the repo root (redteam idiom).
```

### `go/acs/cycle783/predicates_test.go:100` — above `func TestC783_001_cycle779_suite_reverified_8of8(t *testing.T) {`

```text
// AC1: the cycle-779 acceptance suite is 8/8 green under -race in THIS tree.
// Pre-existing GREEN by design (verification task); rejects exit-0 gaming by
// counting each named PASS marker.
```
