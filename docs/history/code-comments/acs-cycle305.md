# Comment history: `acs/cycle305`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle305/predicates_test.go:3` — above `package cycle305`

```text
// Package cycle305 materializes the cycle-305 acceptance criteria for the single
// committed-AND-buildable task in this slice (triage-report.md ## top_n, narrowed
// by architecture-design.md to Option A — Layer 1 only):
//
//	evalgate-floor-declarations — complete ADR-0046 Layer 1 by making deferred
//	    floor lookup DECLARATION-primary. Add triagecap.ReadDeferredFloors +
//	    DeferredFloorPackagesDecl (companion deferred_floors[] authoritative, prose
//	    fallback) + DeferredFloorDivergence (the guard's reporter); rewire
//	    evalgate's floorBindingGate to read <workspace>/triage-decision.json; add
//	    deferred_floors to the schema + triage persona; add the
//	    `evolve guard triage-floors <workspace>` self-check CLI. Closes the last
//	    prose-scrape path that left the cycle-280 binding class open.
//
// The second top_n task (heuristic-gate-demotion-instinct, ADR-0046 Layer 2) is
// EXPLICITLY DEFERRED by the architecture-design phase (Option A; design.md:92,
// 152-156): "do not implement GateClass, HeuristicDemotionChecker, inbox demotion
// filing, or orchestrator demotion wiring in this Builder slice." It therefore
// gets ZERO predicates here — authoring demotion pins would gate work the Builder
// is instructed not to do, RED-locking the cycle (the cycle-280 starvation class
// in spirit). Its H1-H4 ACs carry to the next cycle with their own TDD pins.
//
// These predicates are BEHAVIORAL (cycle-85 lesson; cycle281/300/304 pattern). The
// load-bearing gates RUN the real internal/triagecap, internal/evalgate, and
// cmd/evolve test suites as subprocesses and assert on the real `--- PASS: <name>`
// lines the builder's implementation produces. Those pins construct companions,
// run the real readers + the real floorBindingGate + the real guard CLI — a magic
// string in a .go file can neither emit a named PASS line nor make a
// declaration-primary block decision, and an EMPTY tree lacks the functions
// entirely (compile failure → no PASS lines → RED). The config-check predicate
// (schema + persona declare deferred_floors) carries an explicit waiver.
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary", evalgate-floor-
// declarations rows C1-C5, plus the triagecap reader pins from the design blueprint):
//
//	C1 companion blocks predicate   TestFloorBinding_DeferredFromCompanion        \
//	C2 missing companion fail-open  TestFloorBinding_MissingCompanion_FailOpen     |
//	N1 prose ignored w/ companion   TestFloorBinding_ProseIgnoredWithCompanion     } C305_001
//	E1 no-field -> prose fallback   TestFloorBinding_CompanionNoField_FallbackProse|
//	C5 divergence reporter          TestFloorBinding_DeclaredDivergenceMessage     |
//	   reader contract              TestReadDeferredFloors / *PackagesDecl* / *Divergence
//	C4 guard CLI self-check         TestGuardTriageFloors_*                         -> C305_002
//	   contract surface             schema + persona declare deferred_floors        -> C305_003
//
// Floor binding (R9.3): evalgate-floor-declarations commits ZERO package coverage
// floors, so no coverage-floor predicate is authored here. All package-path string
// literals below live in non-Test helpers (runDataPins/runGuardPins) so the
// floorBindingGate's Test-function scan never misreads these behavioral gates as
// coverage-floor predicates (cycle304 pattern).
```
