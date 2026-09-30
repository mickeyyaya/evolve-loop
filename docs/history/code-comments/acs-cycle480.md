# Comment history: `acs/cycle480`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle480/predicates_test.go:3` — above `package cycle480`

```text
// Package cycle480 materialises the cycle-480 acceptance criteria.
//
// TRIAGE COMMITTED TWO ## top_n TASKS, but only ONE is materializable this
// cycle:
//
//	universal-envelope-floor        (go/internal/router/model_routing_clamp.go)
//	  → C480_001, C480_002, C480_003
//	egps-timeout-loud-diagnostic    (go/internal/acssuite/acssuite.go) — DROPPED
//	  → the fix locus is the PROTECTED CONTROL-PLANE SURFACE
//	    `/go/internal/acssuite/` (the EGPS gate runner; see
//	    go/internal/guards/integrity_surface.go). A cycle may not edit the gate
//	    that grades it (ADR-0064). Builder would be denied identically, so no
//	    predicate can bind to it this cycle. Its ACs are dispositioned
//	    `unverifiable-remove` (cycle-scoped) in test-report.md with the
//	    recommendation to route it via `evolve ship --class manual` OUTSIDE a
//	    cycle. Predicates bind ONLY to buildable committed work (R9.3).
//
// Task 1 root cause (scout Key Finding 3): the operator low-model floor at
// model_routing_clamp.go:52 gates the clamp-up entirely on
// `prof.ModelTierEnvelope != nil`. 72/91 profiles declare NO envelope, so a
// below-floor `tier:fast` proposal against a nil-envelope profile falls through
// to policy.ValidatePin, which (by design B2) treats "no envelope configured" as
// a PREFERENCE — and is never clamped up to the balanced floor. The fix
// substitutes a compiled-default envelope {min:balanced, max:deep} at the clamp
// site when the profile declares none, so the floor is UNIVERSAL.
//
// 1:1 AC-materialization (Task 1): 4 predicate ACs + 1 CI-parity AC = 5 ACs,
// none double-counted (see .evolve/evals/universal-envelope-floor.md).
//
// Adversarial diversity (skills/adversarial-testing SKILL §6):
//
//	Negative:  C480_002's ExplicitEnvelopeNotOverridden — a fix that applies the
//	           default UNCONDITIONALLY (clobbering an explicit envelope that
//	           permits fast) must NOT survive.
//	Edge/OOD:  C480_002's WithinCeilingPassesThrough — a nil-envelope proposal
//	           already within the default ceiling ("deep") must pass unclamped.
//	Anti-game: C480_001's AppliesAcrossPhases — the floor must be universal, not
//	           hardcoded to the single phase name AC1 uses.
//	Semantic:  the clamp-UP surface (C480_001) is distinct from the no-over-clamp
//	           surface (C480_002); satisfying one must not silently satisfy the
//	           other.
//
// RED strategy (verified in test-report.md "RED Run Output"): C480_001 is RED
// because model_routing_clamp.go has no universal floor today — a nil-envelope
// fast proposal stays fast (zero clamps). C480_003 (full-package CI-parity) is
// RED for the same reason (it runs the new RED unit tests). C480_002's two legs
// are pre-existing-correct boundary pins (GREEN today) that a plausible buggy
// fix would break — declared as such per the AC-Materialization Contract.
```

### `go/acs/cycle480/predicates_test.go:122` — above `func TestC480_003_RouterCIParity(t *testing.T) {`

```text
// TestC480_003_RouterCIParity (Task1-AC5 CI-parity + boundary): the full
// internal/router package must pass under -race, go vet must be clean, and
// apicover -enforce over internal/router must stay clean (the cycle adds only
// test files + an unexported clamp-site change — zero new exported symbols — so
// apicover must not regress; guards the cycle-413 WARN-ship class). Mirrors the
// exact repo-wide CI on the touched package.
```
