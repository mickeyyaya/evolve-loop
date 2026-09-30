# Comment history: `acs/cycle1544`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1544/predicates_test.go:3` — above `package cycle1544`

```text
// Package cycle1544 materialises the cycle-1544 acceptance criteria for the
// three fleet-scoped tasks triage committed on the lost-landing / closeout
// surface:
//
//   - lost-ship-closeout-universal-landing-witness → go/internal/core/lost_landing_floor.go
//   - lost-ship-dossier-evidence                   → go/internal/core/dossier_producer.go
//   - continuation-create-reuse-snapshot-base-guard → go/internal/core/worktree.go
//
// What is NOT yet proven today. `detectLostLanding` treats the presence of
// ONE artifact — ship-binding.json — as the sole witness of a landing, while
// the real ship state machine has more than one successful exit
// (SHIPPED_VIA_BUILD, and a clean no-op completion). Its landed-cycle
// coverage (lost_landing_floor_test.go) exercises only the binding-present
// and never-shipped shapes, so a shipped-via-build cycle that also recorded a
// transient ship error is undefended: nothing today asserts it must NOT be
// downgraded. Second, `finalizeCycle` produces the `landing-lost`
// SystemFailureSignal onto CycleResult and `writeCycleDossier` receives only
// the terminal outcome string (dossier_producer.go:67) — the signal has no
// durable consumer, so the operator's committed closeout record cannot
// distinguish a destroyed landing from an ordinary WARN. Third, `Create`
// reuses a valid same-cycle worktree and returns it, after which cyclerun.go
// captures its current HEAD verbatim as WorktreeBaseSHA (cyclerun.go:496-497);
// when that HEAD is a salvage snapshot the normalization base becomes the
// preserved work itself, so the very work salvage exists to protect
// normalizes away to nothing.
//
// Predicate strategy — every predicate exercises the SYSTEM, never a source
// grep of production code (the cycle-85 degenerate-predicate ban). The seams
// under test (detectLostLanding, finalizeCycle, writeCycleDossier,
// gitWorktree.Create + the cyclerun base capture) are ALL unexported inside
// internal/core and therefore unreachable from a leaf acs package; each
// predicate drives them through the sanctioned behavioural-via-subprocess
// shape (cycle-987/997/1532 precedent): a `-run`-narrowed, single-named-package
// `go test -v` that must print `--- PASS: <name>` for each binding test the
// Builder authors. Asserting on the PASS LINE — never on exit 0 — is
// load-bearing: `go test -run` against a pattern matching NO test exits 0 with
// "no tests to run", so a still-missing binding test would false-GREEN.
//
// Every invocation is `-run`-narrowed against ONE named package, never a
// `/...` sweep and never a whole-package run of the 40s+ core suite, so a
// concurrent lane's contamination in untouched code can never red this cycle
// (the flaky-predicate-shape contract).
```

### `go/acs/cycle1544/predicates_test.go:80` — above `func TestC1544_004_LostLandingEvidenceReachesTheCommittedDossier(t *testing.T) {`

```text
// NOTE (2026-08-23 console salvage): predicates 001-005 (lost-landing witness +
// dossier evidence) were REMOVED from this suite when the snapshot-base-guard
// slice was salvaged to main. They accept work that was triage-DEFERRED and
// excised for HIGH defects (d22c3331: the witness clears the integrity floor on
// a bare os.Stat of an agent-writable gitignored file; d6ed7e1d: unvalidated
// worktree path join). Their acceptance travels WITH those tasks (inbox:
// lost-ship-closeout-universal-landing-witness, lost-ship-dossier-evidence) and
// must return alongside the redesigned code — landing them now would put five
// permanently-red predicates on main. 006-008 accept the DELIVERED guard slice;
// 006/007's bindings are repointed at the test names the delivering cycle
// (1546) actually used, because the original names were phantoms that
// red-blocked every continuation cycle from 1544 onward (the 1539-1546 streak).
//
// RESTORED (cycle 1663, inbox lost-ship-dossier-evidence AC3): 004-005, the
// dossier-evidence half. Their producer — detectLostLanding + the finalizeCycle
// stamp — landed with PR #482 (TestFinalizeCycle_LostLandingDowngrades… is
// green in 008 above), so the only gap left is the SINK: writeCycleDossier
// still receives the outcome string alone. 004/005 bind to the cycle-1663 core
// tests that drive the two REAL production callers (completeCycle and
// abnormalEpilogue) over the vendored 1535/1536 artifacts. 001-003 (the
// witness-redesign slice) still travel with their own inbox item.
```

### `go/acs/cycle1544/predicates_test.go:102` — above `func TestC1544_004_LostLandingEvidenceReachesTheCommittedDossier(t *testing.T) {`

```text
// TestC1544_004_LostLandingEvidenceReachesTheCommittedDossier — AC1 of
// lost-ship-dossier-evidence: a landing-lost cycle's committed dossier carries
// the signal's structured category AND evidence text, through the REAL
// writeCycleDossier path — BOTH production callers, the normal closeout
// (cycle_closeout.go) and the abnormal epilogue (cyclerun_epilogue.go). A
// seam only a direct producer call reaches is dead code, so the binding
// tests drive cycleRun.completeCycle / cycleRun.abnormalEpilogue over
// cycle-1535's real artifacts and assert on the committed JSON + Markdown.
```

### `go/acs/cycle1544/predicates_test.go:117` — above `func TestC1544_005_LandedSiblingAndOrdinaryPassCarryNoEvidence(t *testing.T) {`

```text
// TestC1544_005_LandedSiblingAndOrdinaryPassCarryNoEvidence — AC2, the
// negative that keeps the feature from turning every contended wave into a
// wall of false evidence: cycle-1536 hit the SAME transient ship error and
// landed, so its dossier (same real path) carries no system-failure object
// and leaks neither the category nor the error code; and an ordinary PASS
// dossier stays BYTE-clean against a golden captured from the pre-change
// producer (a nil signal adds nothing — no null, no empty md section).
```

### `go/acs/cycle1544/predicates_test.go:131` — above `func TestC1544_006_ReusedSnapshotNeverBecomesTheWorktreeBase(t *testing.T) {`

```text
// TestC1544_006_ReusedSnapshotNeverBecomesTheWorktreeBase — AC7 of
// continuation-create-reuse-snapshot-base-guard. The reuse branch of
// gitWorktree.Create (worktree.go:75-90) returns a valid same-cycle worktree
// untouched, and cyclerun.go:496 then records `rev-parse HEAD` as
// WorktreeBaseSHA. When that HEAD is the salvage snapshot commit
// (continuation_stamp.go:44-45, "salvage snapshot (ADR-0076
// continuation-on-fail)"), the base and the preserved work are the SAME
// commit, so normalizeWorktreeToBase soft-resets to the snapshot and the
// salvaged diff reads as empty. The binding test must exercise the real
// provisioning → base-capture path against a real git repo whose worktree HEAD
// is a snapshot, and assert the captured base is the snapshot's first
// non-snapshot ANCESTOR — not the snapshot, and not empty.
```

### `go/acs/cycle1544/predicates_test.go:168` — above `func TestC1544_008_LandedRegressionsNotWeakened(t *testing.T) {`

```text
// TestC1544_008_LandedRegressionsNotWeakened is the ANTI-WEAKENING floor.
// This cycle re-keys a detector and edits a base-capture path that landed
// tests already pin, so the cheapest way to green AC2/AC7 is to relax or
// delete those tests — widen an assertion, drop a negative case, retire the
// real cycle-1535/1536 fixtures. Naming the landed tests here makes that path
// RED: a deleted or renamed test prints no PASS line, and a weakened one that
// starts failing prints FAIL.
//
// Deliberately NOT a whole-package sweep of internal/core: that samples
// untouched code and manufactures false reds under fleet load (the cycles
// 1115/1117 class). The invocation is `-run`-narrowed to the named tests in
// ONE package.
```
