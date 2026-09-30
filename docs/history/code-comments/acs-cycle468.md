# Comment history: `acs/cycle468`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle468/predicates_test.go:3` — above `package cycle468`

```text
// Package cycle468 materialises the cycle-468 acceptance criteria for the
// single triage-committed task (operator priority override, T1 of 2):
//
//	egps-flake-retry-once (go/internal/acssuite retry-once for test-failure
//	RED predicates with a visible flaky annotation + WARN;
//	go/internal/phases/audit WARN surfacing) → C468_001..005
//
// 1:1 AC-materialization: 5 predicates + 0 manual+checklist + 0 removed = 5
// ACs total (see the cycle workspace .evolve/evals/egps-flake-retry-once.md),
// none double-counted.
//
// CONTROL-PLANE NOTE (why these predicates live here and pin the WIRE
// contract, not struct fields): go/internal/acssuite/ is protected integrity
// surface (guards.IsProtectedSurface, ADR-0064) — no autonomous phase may
// write there, so the cycle cannot host unit tests inside the package.
// go/acs/cycle<N>/ is the sanctioned per-cycle predicate surface, and the
// acssuite seam API (Run / Options.GoExec / WriteVerdict) is exported, so
// every criterion is encoded here by exercising Run in-process with a
// scripted, invocation-counting GoExec seam and asserting on the verdict
// STRUCT tallies plus the WRITTEN acs-verdict.json bytes — the contract the
// audit + ship gates actually consume. Field names are left to the
// implementation; the JSON keys ("flaky", "warnings") are the pinned API.
//
// RED strategy (verified in test-report.md "RED Run Output"): the package
// COMPILES against the current acssuite API, so C468_001 and C468_002 are red
// on their own ASSERTIONS (no retry exists: the flaky fixture yields
// verdict=FAIL, and the seam records 1 invocation where the bound demands
// exactly 2) — the right-reason RED. C468_003/004/005 are pre-existing-GREEN
// regression pins by design (they pin behavior the change must NOT alter:
// parse-error REDs stay non-retried, the no-flake wire bytes stay identical,
// repo gates stay green).
//
// Adversarial diversity (skills/adversarial-testing SKILL §6):
//
//	Negative:   C468_002 — a deterministic red MUST stay RED (kills the
//	            "always green on retry" gaming fake) and the seam must record
//	            EXACTLY 2 invocations (kills unlimited-retry-until-green);
//	            C468_001's annotation must be in the WRITTEN JSON (kills
//	            "flip silently / strip before write").
//	Edge / OOD: C468_003 — the synthetic egps/go-lane-parse-error RED
//	            (oversized NDJSON line breaking the scanner) is not a test
//	            failure and must NOT trigger a retry (1 invocation, stays
//	            FAIL); C468_002's exact-2 bound.
//	Semantic:   C468_001 (flake → GREEN + visible annotation + WARN) vs
//	            C468_002 (deterministic red → unchanged FAIL) are DISTINCT
//	            behaviors — a retry that flips everything passes 001 but
//	            fails 002; C468_004 pins the degrade path (no flake ⇒
//	            byte-identical wire output, omitempty invisibility).
```

### `go/acs/cycle468/predicates_test.go:66` — above `const (`

```text
// Fixture identities mirror the real cycle-466 burned-cycle evidence
// (.evolve/runs/cycle-466/acs-verdict.json): the -race full-suite contention
// flake this task exists to absorb.
```

### `go/acs/cycle468/predicates_test.go:97` — above `const raceContentionOutput = "    predicates_test.go:137: full-package -race regression on cmd/evolve, internal/fleet, i…`

```text
// raceContentionOutput is the real cycle-466 red evidence shape.
```

### `go/acs/cycle468/predicates_test.go:186` — above `func TestC468_001_FlakyRedFlipsGreenWithVisibleAnnotation(t *testing.T) {`

```text
// TestC468_001_FlakyRedFlipsGreenWithVisibleAnnotation (AC1, positive): a
// scope red on the first GoExec invocation (cycle-466 -race contention shape)
// and green on the second must yield verdict PASS / red_count 0 /
// ship_eligible, with the retry visible on the wire: the flipped result
// carries flaky="passed-on-retry" IN THE WRITTEN acs-verdict.json (not only
// in-memory — kills strip-before-write), a top-level warning names the test,
// the untouched green carries no annotation, and the seam records exactly 2
// current-cycle invocations.
```

### `go/acs/cycle468/predicates_test.go:314` — above `func TestC468_004_NoFlakeDegradePathByteIdentical(t *testing.T) {`

```text
// TestC468_004_NoFlakeDegradePathByteIdentical (AC4, regression pin —
// pre-existing GREEN): for an all-green suite the WRITTEN acs-verdict.json
// must byte-compare equal to the pre-change golden serialization — the new
// flaky/warnings fields must be omitempty-invisible when nothing flakes, so
// every existing consumer (audit, ship gate, dossiers) sees identical bytes.
// Golden captured from the pre-change schema at cycle 468.
```

### `go/acs/cycle468/predicates_test.go:358` — above `func TestC468_005_RaceVetApicoverCleanOnTouchedPackages(t *testing.T) {`

```text
// TestC468_005_RaceVetApicoverCleanOnTouchedPackages (AC5, CI-parity gates —
// pre-existing GREEN baseline): full -race regression on the two packages the
// task touches, go vet clean on both, and apicover -enforce over
// internal/acssuite (any NEW exported symbol the implementation adds must be
// named by a test AND executed — kills the cycle-413 WARN-ship class).
```
