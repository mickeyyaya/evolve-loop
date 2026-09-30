# Comment history: `acs/cycle466`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle466/predicates_test.go:3` — above `package cycle466`

```text
// Package cycle466 materialises the cycle-466 acceptance criteria for the
// single triage-committed task (## top_n only, operator priority override):
//
//	s2-wave-salvage-fix-d1 (go/internal/fleet/triageplan.go,
//	go/cmd/evolve/cmd_loop_wave.go) → C466_001..006
//
// Salvages cycle 465's preserved worktree wave-semantics work and fixes the
// D1 defect that failed cycle 465's audit: dispatchIteration did not guard
// len(specs)==0, so an empty adapted triage plan invoked launcher.Run with a
// zero-lane spec list and returned ran=true — silently consuming a
// --max-cycles iteration doing zero work (livelock). Also fixes
// productionWavePlanFn's hardcoded cardPackages=nil by threading real
// top_n[].id card ids through PlanFromTriage, since real triage-decision.json
// artifacts (e.g. .evolve/runs/cycle-464/triage-decision.json) commonly carry
// NO committed_floors field at all — the floorless+cardless livelock is the
// COMMON path, not an edge case.
//
// 1:1 AC-materialization: 6 predicates + 0 manual+checklist + 0 removed = 6
// ACs total (see .evolve/evals/s2-wave-salvage-fix-d1.md), none
// double-counted.
//
// RED strategy (verified in test-report.md "RED Run Output"): C466_001-003
// and C466_006 fail because go/cmd/evolve/cmd_loop_wave_test.go and
// go/internal/fleet/triageplan_test.go reference dispatchIteration/
// shouldRunWave/PlanFromTriage, which do not exist yet in this worktree —
// go/cmd/evolve and go/internal/fleet both fail to COMPILE. C466_004
// (repo-gates regression) and C466_005 (apicover -enforce) are red for the
// same two compile failures.
//
// Adversarial diversity (skills/adversarial-testing SKILL §6):
//
//	Negative:   C466_001 (an empty adapted plan must fall through to
//	            sequential, never claim a do-nothing wave — kills a guard
//	            that still returns ran=true with empty results),
//	            C466_003 (malformed triage-decision.json must be REJECTED,
//	            not silently guessed into an unscoped launch)
//	Edge/OOD:   C466_006 (a single top_n card at count=4 yields exactly ONE
//	            spec, not four — never pad unused lanes to fc.Count)
//	Semantic:   C466_001 vs C466_002 (the empty-plan GUARD is a distinct
//	            requirement from the top_n CARD-FALLBACK path — a fix that
//	            only adds the guard still livelocks on every real
//	            cycle-464-shaped decision; both must hold independently)
```

### `go/acs/cycle466/predicates_test.go:102` — above `func TestC466_002_PlanFromTriageProductionFixtureThreadsRealCards(t *testing.T) {`

```text
// TestC466_002_PlanFromTriageProductionFixtureThreadsRealCards (AC2): a
// triage-decision.json shaped like the REAL cycle-464 artifact (top_n[].id
// cards, no committed_floors) with cardPackages=nil must plan >=1 lane.
// Shells the named RED unit test written this cycle.
```

### `go/acs/cycle466/predicates_test.go:146` — above `func TestC466_005_VetAndApicoverEnforceCleanOnTouchedPackages(t *testing.T) {`

```text
// TestC466_005_VetAndApicoverEnforceCleanOnTouchedPackages (AC5,
// CI-parity): mirrors .github/workflows/go.yml's "api-coverage enforce"
// step scoped to internal/fleet — PlanFromTriage (the new exported symbol)
// must be named by a test AST AND show >0% executed coverage. Kills the
// cycle-413 gaming class (a new exported symbol shipped without a naming
// test breaks main CI's repo-wide apicover -enforce).
```
