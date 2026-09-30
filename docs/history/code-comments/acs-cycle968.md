# Comment history: `acs/cycle968`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle968/predicates_test.go:3` — above `package cycle968`

```text
// Package cycle968 materializes the cycle-968 acceptance criteria for fleet lane
// `carryforward-real-cherrypick-filter`. The lane's cycle-962 deliverables shipped
// INERT: core.CarryforwardCandidateLandable (weight 0.94) and its dependent
// core.PruneSupersededOrphans have ZERO non-test callers (grep-confirmed), which
// violates the pinned goal floor — "No inert API: every new exported surface ships
// with a caller and a naming test." This cycle WIRES the filter into the real
// production fleet-rebase recovery path and adds a guard so the inert class cannot
// silently regrow.
//
// SCOPE (Rule 3, surfaced): fleet_scope names one inbox id; triage expanded it into
// two top_n tasks in ONE worktree. Predicates are authored for BOTH (both top_n,
// built together). The DEFERRED beyond-ask idea (acs-verdict skip-reason emission)
// gets ZERO predicates (R9.3 floor-binding).
//
// ---------------------------------------------------------------------------
// DESIGN DECISION surfaced to Builder (Rule 1 + Rule 3 — do NOT silently deviate)
// ---------------------------------------------------------------------------
// The scout's Task-1 title says "wire CarryforwardCandidateLandable so a superseded
// candidate short-circuits". Taken LITERALLY (landable==false ⇒ short-circuit) that
// is INCORRECT: CarryforwardCandidateLandable returns false for BOTH an already-
// landed (superseded) candidate AND a genuine 3-way CONFLICT. Short-circuiting a
// genuine conflict as "already landed" would SILENTLY DROP real overlapping work
// that must instead route to the debugger (CodeGitFleetRebaseConflict). The
// scout's own Key Finding (report lines 32-35) is the precise, correct framing:
// it is the SUPERSESSION branch that must short-circuit, while a real conflict must
// still flow to the existing conflict route.
//
// Correct, minimal contract the Builder MUST add to package core WITHOUT modifying
// this file (natural home: go/internal/core/carryforward_filter.go):
//
//	type FleetRebaseVerdict int
//	const (
//	    FleetRebaseAlreadyLanded FleetRebaseVerdict = iota // superseded → short-circuit, NO replay/re-audit
//	    FleetRebaseClean                                    // clean & not landed → rebase & replay (existing path)
//	    FleetRebaseConflict                                 // genuine 3-way conflict → debugger route
//	)
//	func ClassifyFleetRebaseCandidate(ctx context.Context, dir, candidateRef, base string) (FleetRebaseVerdict, error)
//	    // Deterministic, zero-LLM. Internally REUSES the inert cycle-962 surface:
//	    //   landable, err := CarryforwardCandidateLandable(...)  // gives it a caller ⇒ not inert
//	    //   if err  → propagate (git-infra failure; NEVER masked as a verdict)
//	    //   if landable                 → FleetRebaseClean
//	    //   else if refSuperseded(...)  → FleetRebaseAlreadyLanded
//	    //   else                        → FleetRebaseConflict
//	    // All git through the gitCapture seam.
//
// Production wiring seam: (*Orchestrator).recoverFromShipError in ship_recovery.go,
// the CodeGitFleetRebaseNeeded branch, MUST call ClassifyFleetRebaseCandidate BEFORE
// rebaseCycleBranchOntoMain and map: AlreadyLanded → short-circuit (no wasted
// re-audit — the explicit 948 "PASS-but-unlanded duplicate" fix); Clean → the
// existing replay; Conflict → the existing debugger reclassification.
//
// core.PruneSupersededOrphans is NOT wired this cycle (branch-deleting housekeeping
// in the hot recovery path is out of a focused fleet-rebase lane's scope). Its
// caller is dispositioned as tracked carryover (manual+checklist, see test-report),
// NOT a fabricated caller — per scout Task-2's explicit allowance and
// no_workaround_root_cause_redesign. Its identity/signature is still pinned below so
// the surface cannot drift while the carryover is open.
//
// ---------------------------------------------------------------------------
// PREDICATE STYLE (cycle-85 rule): go/internal/core is importable from go/acs, so
// every BEHAVIORAL predicate EXERCISES the SUT (calls ClassifyFleetRebaseCandidate
// against a REAL git repo built in a temp dir — git is always present) and asserts
// on the returned verdict. RED here is a COMPILE failure (undefined:
// core.ClassifyFleetRebaseCandidate / core.FleetRebaseVerdict / the consts), which
// fails for the RIGHT reason: the production symbols are absent. The two structural
// WIRING-PROOF predicates (caller-exists, the "no inert API" floor) carry a
// `// acs-predicate: config-check` waiver because a caller-existence assertion is
// inherently a source-structure check; each is PAIRED with the behavioral tests
// above (never the sole load-bearing assertion for the feature).
//
// Adversarial diversity (skills/adversarial-testing §6):
//
//	POSITIVE → C968_001 (clean, non-superseded candidate ⇒ FleetRebaseClean — the
//	           anti-`always-conflict/always-landed` signal a degenerate map cannot fake).
//	NEGATIVE → C968_002 (patch-id-dup ⇒ AlreadyLanded), C968_003 (ancestor ⇒
//	           AlreadyLanded), C968_004 (genuine conflict ⇒ Conflict, NOT AlreadyLanded
//	           — the strongest anti-drop-work signal), C968_005 (bad ref ⇒ error, never
//	           masked as a verdict).
//	SEMANTIC → the three verdicts are DISTINCT outcomes, each asserted separately.
//
// AC map (1:1 with the disposition table in test-report.md):
//
//	T1-AC1 clean, not superseded         → FleetRebaseClean          → C968_001 (POSITIVE)
//	T1-AC2 patch-id-dup already landed   → FleetRebaseAlreadyLanded  → C968_002 (NEGATIVE)
//	T1-AC3 is-ancestor already landed    → FleetRebaseAlreadyLanded  → C968_003 (NEGATIVE/EDGE)
//	T1-AC4 genuine 3-way conflict        → FleetRebaseConflict       → C968_004 (NEGATIVE, critical)
//	T1-AC5 git-infra error propagates    → non-nil error             → C968_005 (EDGE/NEGATIVE)
//	T1-AC6 recoverFromShipError CALLS ClassifyFleetRebaseCandidate   → C968_006 (WIRING, config-check)
//	T2-AC1 ClassifyFleetRebaseCandidate CALLS CarryforwardCandidateLandable (kills inert) → C968_007 (WIRING)
//	T2-AC2 CarryforwardCandidateLandable identity/signature pinned    → C968_008 (compile-pin)
//	T2-AC3 PruneSupersededOrphans identity/signature pinned (carryover)→ C968_009 (compile-pin)
//	T1-AC7 / T2-AC4 -race + go vet + repo-wide apicover clean         → manual+checklist (Auditor CI-parity)
```

### `go/acs/cycle968/predicates_test.go:109` — above `var (`

```text
// Compile-time identity/signature pins (T2-AC2 / T2-AC3). These fail to compile if
// the Builder renames or changes the shape of either cycle-962 surface, freezing the
// public contract the naming guard defends. They sit at package scope so they are
// part of the same RED compile failure until the whole package builds.
```

### `go/acs/cycle968/predicates_test.go:287` — above `func TestCarryforwardCandidateLandable_HasProductionCaller(t *testing.T) {`

```text
// C968_007 (T2-AC1, WIRING PROOF — kills the cycle-962 inert surface) —
// ClassifyFleetRebaseCandidate MUST call CarryforwardCandidateLandable, giving the
// weight-0.94 cycle-962 filter its first production caller (via the wired classifier).
// This is the assertion that closes the "no inert API" floor violation for the filter.
// acs-predicate: config-check — caller-existence is an inherent source-structure check;
// the filter's behavior is already pinned by cycle962's behavioral predicates.
```
