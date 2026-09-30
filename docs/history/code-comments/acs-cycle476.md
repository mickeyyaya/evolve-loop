# Comment history: `acs/cycle476`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle476/predicates_test.go:3` — above `package cycle476`

```text
// Package cycle476 materialises the cycle-476 acceptance criteria for the two
// triage-committed tasks (## top_n only, operator priority override T2 —
// advisor tier-emission determinism):
//
//	advisor-real-persona-liveness-golden  (go/internal/core/
//	  phase_advisor_tier_elicitation_test.go) → C476_001, C476_002
//	advisor-persona-tier-example-harmonize (agents/evolve-router.md,
//	  config-only) → turns C476_001/C476_002 GREEN
//
// Root cause (scout): the operator's three literal deliverables already ship on
// main (Go {cli,tier} example #293, liveness golden, overlay-log goldens). The
// RESIDUAL intermittency is a competing bare existing-phase example INSIDE the
// shipped persona agents/evolve-router.md (frontmatter output-format :10 and the
// body example :35), which appears BEFORE and undermines the Go-appended
// {cli,tier} example. Every prior advisor-prompt test used a STUB persona
// (WithPersona("PERSONA BODY")) and was structurally blind to it. Task 1 adds the
// missing real-persona test class (RED today); Task 2 harmonizes the persona
// (config-only) to turn it GREEN.
//
// 1:1 AC-materialization: 4 predicates + 1 manual+checklist + 0 removed = 5 ACs
// total (see .evolve/evals/advisor-real-persona-liveness-golden.md and
// .evolve/evals/advisor-persona-tier-example-harmonize.md), none double-counted.
//
// RED strategy (verified in test-report.md "RED Run Output"):
// C476_001 and C476_002 are RED because the two real-persona goldens fail their
// assertions today — the persona body example :35 lacks {cli,tier} and the
// frontmatter output-format :10 omits cli/tier. C476_003 (overlay-log goldens)
// and the degrade/tier-confinement legs of C476_002 are pre-existing GREEN
// regression pins (they lock behavior Task 2 must NOT relax) — declared as such
// per the AC-Materialization Contract. C476_004 (CI-parity) greens once
// internal/core compiles and the goldens pass.
//
// Adversarial diversity (skills/adversarial-testing SKILL §6):
//
//	Negative:   C476_001's >=2-examples floor — deleting the persona example
//	            (leaving only the Go one) must NOT green the golden; the persona
//	            itself must teach the tiered schema. C476_002's
//	            AbsentCLITierFieldsStayEmpty — a "add the schema text but break
//	            absent-field parsing" fake must not survive.
//	Edge/OOD:   C476_002's sanitizeAdvisorTier confinement (high/top/raw-model/
//	            empty all rejected — no clamp/floor relaxation widening the
//	            vocabulary).
//	Semantic:   C476_001's body-example surface vs the frontmatter output-format
//	            surface are DISTINCT — a fix to the body alone must not silently
//	            satisfy the frontmatter enumeration, and vice versa.
```

### `go/acs/cycle476/predicates_test.go:138` — above `func TestC476_004_CIParityCoreRaceVetApicover(t *testing.T) {`

```text
// TestC476_004_CIParityCoreRaceVetApicover (AC5, CI-parity + boundary): the full
// internal/core package must pass under -race, go vet must be clean, and apicover
// -enforce over internal/core must stay clean (the cycle adds only unexported
// test helpers — zero new exported symbols — so apicover must not regress; guards
// the cycle-413 WARN-ship class). Mirrors the exact repo-wide CI on the touched
// package.
```
