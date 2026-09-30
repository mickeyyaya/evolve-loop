# Comment history: `acs/cycle447`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle447/predicates_test.go:3` — above `package cycle447`

```text
// Package cycle447 materialises the cycle-447 acceptance criteria (goal:
// 6fcbcd226cb22fe4ab63407907fb5e2f3a570b4939cb0a2d79f61e97e60d2f0a — model-
// switch through the abstract layer for every LLM CLI; close the one broken
// cell: agy-tmux, whose params.model_tier is channel:"noop" with a flat
// model_tier_map).
//
// Three ## top_n tasks (triage-report.md), strict dependency chain:
//
//	Task 1 (agy-model-channel-probe-and-wire, M, P1): probe the installed agy
//	  live (incident cycle-154 forbids trusting the stale KB `-m` claim), wire
//	  the least-code effective channel (preference flag→repl→picker-select),
//	  and replace the flat model_tier_map with real distinct per-tier ids.
//	Task 2 (model-tier-matrix-parity-pin, S, P2): parity test over the
//	  embedded *-tmux manifests — every tmux CLI translates intent.ModelTier
//	  through SOME effective channel; no multi-model CLI may be noop; includes
//	  a noop-rejection negative fixture and an integration-style agy tier=deep
//	  launch assertion.
//	Task 3 (model-channel-translation-docs, S, P3): per-CLI channel table in
//	  docs/architecture/model-discovery-and-catalog.md, cross-checked against
//	  the manifests (projection, never a second mapping).
//
// AC map (1:1, R9.3 floor-binding; predicates for ## top_n tasks only):
//
//	T1 AC1 fresh live probe evidence in build-report      → manual+checklist
//	                                          (Auditor judgment on liveness —
//	                                           see test-report.md checklist)
//	T1 AC2 agy tier=deep realizes an effective emission    → C447_001 (positive)
//	T1 AC3 manifest tier map has ≥2 distinct models        → C447_002 (edge)
//	T1 AC4 `auto` sentinel emits NO model for agy          → C447_003 (negative)
//	T1 AC5 builder unit tests: Agy auto-sentinel +
//	       unknown-tier named tests all green              → C447_004 (semantic)
//	T1 AC6 repl seed-path test exists & passes if repl     → C447_005 (edge,
//	                                                          conditional)
//	T2 AC1 parity: one subtest per *-tmux manifest         → C447_006 (positive)
//	T2 AC2 noop-rejection negative fixture subtest         → C447_007 (negative)
//	T2 AC3 dispatched agy tier=deep carries deep model     → C447_008 (semantic)
//	T2 AC4 bridge vet + -race suite green (regression)     → C447_009 (regression)
//	T3 AC1 doc channel row per manifest matches actual     → C447_010 (positive,
//	                                                          glob-driven edge)
//	T3 AC2 no table row documents a CLI as noop            → C447_011 (negative)
//	T3 AC3 doc states Realizer seam, tier vocabulary,
//	       auto-sentinel omission, overlay order           → C447_012 (semantic)
//
// 1:1 enforcement: 12 predicates + 1 manual+checklist = 13 ACs, each AC
// exactly one disposition, none double-counted. ✓
//
// Builder test-name contract (enforced by C447_004/005/006/008; also in the
// agent-mailbox handoff):
//
//	auto-sentinel unit test name contains  "Agy" and "AutoSentinel"
//	unknown-tier  unit test name contains  "Agy" and "UnknownTier"
//	repl seed-path test name contains      "SeedsREPL" or "REPLSeed"
//	parity test names contain              "Parity", subtests named by
//	                                       manifest base (e.g. "agy-tmux")
//	integration launch test name contains  "AgyTierDeep"
//
// RED strategy (verified in test-report.md "RED Run Output"): C447_001/002/
// 005 fail on the current manifest (channel=noop, flat map). C447_004/006/
// 007/008 fail via the no-matching-tests guard (requireTestsRan — a bare
// `go test -run NoMatch` exits 0, the degenerate-predicate trap). C447_010/
// 011/012 fail on the current doc (no channel table, no Realizer/sentinel
// statements). C447_003 and C447_009 are pre-existing GREEN pins: C447_003 is
// trivially green while the channel is noop but becomes the load-bearing
// negative the moment C447_001 forces a non-noop channel (the pair is only
// jointly satisfiable by a correct wiring); C447_009 pins no-regression.
//
// Adversarial diversity (skills/adversarial-testing SKILL §6):
//
//	Negative:   C447_003 (auto sentinel must emit nothing even once the
//	            channel is live), C447_007 (synthetic multi-model noop
//	            manifest must be REJECTED), C447_011 (no doc table row may
//	            say noop)
//	Edge/OOD:   C447_002 (flat map = boundary of "translates"), C447_005
//	            (channel-conditional seed-path coverage), C447_010's glob
//	            (a future *-tmux manifest without a doc row fails)
//	Semantic:   C447_004 (unknown-tier behavior is distinct from happy-path
//	            emission), C447_008 (model reaching the PANE is distinct from
//	            Realize() emitting it), C447_012 (doc states the seam/rules,
//	            not merely a table)
```

### `go/acs/cycle447/predicates_test.go:212` — above `func TestC447_003_AgyAutoSentinelOmitted(t *testing.T) {`

```text
// TestC447_003_AgyAutoSentinelOmitted (T1 AC4, negative): "auto" is the
// loop's resolve-me sentinel, never a concrete model (cycle-262). Whatever
// channel Task 1 wires, tier=auto must emit NO model realization for agy.
// Pre-existing GREEN pin today (noop emits nothing for every tier) — it
// becomes the load-bearing negative the moment C447_001 forces a non-noop
// channel: the pair is only jointly satisfiable by a correct wiring.
```

### `go/acs/cycle447/predicates_test.go:346` — above `func TestC447_009_BridgeRegressionVetAndRace(t *testing.T) {`

```text
// TestC447_009_BridgeRegressionVetAndRace (T2 AC4, regression, pre-existing
// GREEN pin): the touched package must stay vet-clean and -race green —
// claude/codex/ollama translation behavior byte-identical is a hard
// constraint of the goal, and cycle-413 taught that a cycle can pass its own
// scoped checks while breaking repo CI.
```
