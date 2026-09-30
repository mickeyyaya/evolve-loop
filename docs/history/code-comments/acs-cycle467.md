# Comment history: `acs/cycle467`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle467/predicates_test.go:3` — above `package cycle467`

```text
// Package cycle467 materialises the cycle-467 acceptance criteria for the
// single triage-committed task (## top_n only, operator priority override):
//
//	fleet-s3-guards (go/internal/fleet/preflight.go+quota,
//	go/cmd/evolve/cmd_loop_wave.go ctx threading, wave-level disjointness
//	pin) → C467_001..006
//
// FLEET-AS-POLICY S3: (a) dirty-control-plane wave preflight via
// guards.IsProtectedSurface — helper OUTSIDE internal/guards, which is itself
// protected surface; (b) quota-aware Count shrink off clihealth.Store.Active()
// (min 1, WARN naming family+reason); (c) wave-level file-disjointness
// regression pin on PlanWaves/PlanFromTriage output specs; (d) PR #298
// reviewer note — thread the loop's cancellable ctx through wavePlanFn and
// kill the context.Background() mint at cmd_loop_wave.go:116.
//
// 1:1 AC-materialization: 6 predicates + 0 manual+checklist + 0 removed = 6
// ACs total (see .evolve/evals/fleet-s3-guards.md), none double-counted.
//
// RED strategy (verified in test-report.md "RED Run Output"):
// go/internal/fleet fails to COMPILE (preflight_test.go / quota_test.go
// reference PreflightControlPlane / QuotaAwareCount, which do not exist yet)
// and go/cmd/evolve fails to COMPILE (cmd_loop_wave_s3_test.go pins the
// post-S3 dispatchIteration/wavePlanFn signatures) — so C467_001..004 and
// C467_006 are red on those subprocess compile failures. C467_005 is
// additionally red on its own direct assertion: context.Background() is
// still PRESENT in cmd_loop_wave.go. The two wave-disjointness pins
// (C467_004's inner tests) passed standalone BEFORE the contract files
// landed — they are pre-existing-GREEN regression pins by design (AC4 pins
// existing behavior at a previously-unpinned level).
//
// Adversarial diversity (skills/adversarial-testing SKILL §6):
//
//	Negative:   C467_001 (dirty control plane MUST refuse, naming file +
//	            remediation, launcher/planFn never invoked — kills a
//	            preflight that always passes or fires after launch),
//	            C467_005's cancelled-ctx leg (cancellation must surface,
//	            never a silent launch)
//	Edge/OOD:   C467_002's not-a-git-repo fail-loud leg, C467_003's min-1
//	            clamp (more benches than count; count already 1),
//	            C467_004's duplicate-id collapse
//	Semantic:   C467_001 vs C467_002 (refusing dirt is DISTINCT from not
//	            false-positiving on clean/innocent dirt — a guard that
//	            refuses everything passes 001 but fails 002); C467_003's
//	            shrink vs no-bench pass-through
```

### `go/acs/cycle467/predicates_test.go:159` — above `func TestC467_005_CtxThreadedThroughPlanPath(t *testing.T) {`

```text
// TestC467_005_CtxThreadedThroughPlanPath (AC5, PR #298 reviewer note): the
// caller's ctx must reach the plan function (context-value probe) and a
// cancelled ctx must be observable there and surface errors.Is-matchably —
// AND the context.Background() mint must be GONE from cmd_loop_wave.go
// (absence via FileNotContains, the verifiableBy `grep -c == 0` clause).
```

### `go/acs/cycle467/predicates_test.go:174` — above `func TestC467_006_RaceVetApicoverCleanWithZeroGuardsEdits(t *testing.T) {`

```text
// TestC467_006_RaceVetApicoverCleanWithZeroGuardsEdits (AC6, CI-parity +
// boundary): full -race regression on the two touched packages, go vet
// clean, apicover -enforce on internal/fleet (new exported symbols
// PreflightControlPlane/QuotaAwareCount must be named by tests AND executed
// — kills the cycle-413 WARN-ship class), and ZERO edits under
// go/internal/guards/ (the preflight must IMPORT the protected predicate,
// never modify its package): neither uncommitted nor committed-since-main.
```
