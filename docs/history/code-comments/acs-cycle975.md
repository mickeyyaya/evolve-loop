# Comment history: `acs/cycle975`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle975/predicates_test.go:3` — above `package cycle975`

```text
// Package cycle975 materializes the cycle-975 acceptance criteria for the sole
// inbox item this fleet lane is pinned to: prefix-speculation-landing-queue
// (.evolve/inbox/2026-07-13T14-21-00Z-prefix-speculation-landing-queue.json,
// weight 0.93, campaign merge-efficiency-2026-07). Per R9.3 no predicate here
// binds to any other lane's items — fleet_scope pins this lane to exactly this id.
//
// DESIGN (item summary, Zuul/GitHub-queue prefix model). Lanes never push main.
// A single-writer composer (Cognition single-writer principle) maintains a FIFO
// of PASS lane candidates and builds composed candidate trees as queue PREFIXES
// (L1, L1+L2, L1+L2+L3), verifying them concurrently against the native gate set.
// First failing prefix names the culprit positionally (Zuul NNFI); lanes behind
// re-form without it — no bisection subsystem. The window is an AIMD control loop
// (start 3, +1 per green landing, halve on red, floor 1). Lanes are risk-tiered
// like Rust rollups: iffy (core/cross-cutting) and overlap-zone lanes get a solo
// prefix slot. Landing strategy is policy config (fleet.landing: per-lane |
// prefix-queue), NOT an env flag (standing rule no_feature_flags_use_design_patterns).
//
// SUT SURFACE the Builder must add to package go/internal/fleet WITHOUT modifying
// this file (this is the RED contract — the package does not exist yet, so this
// predicate package FAILS TO COMPILE now, which is the correct greenfield RED per
// go/acs/README.md "a predicate package that fails to compile is a HARD suite
// error"):
//
//	type RiskTier int
//	const ( TierRollup RiskTier = iota; TierMaybe; TierIffy )
//	type LaneCandidate struct { ID string; Tier RiskTier; Files []string }
//	type PrefixQueue struct { ... }
//	func NewPrefixQueue() *PrefixQueue
//	func (q *PrefixQueue) Enqueue(c LaneCandidate)
//	func (q *PrefixQueue) Window() int            // AIMD window, starts at 3
//	func (q *PrefixQueue) OnGreen()               // window += 1
//	func (q *PrefixQueue) OnRed()                 // window = max(1, window/2)
//	func (q *PrefixQueue) ComposePrefixes() [][]string  // prefix k = lane IDs [0..k]
//	func (q *PrefixQueue) ResolveCulprit(verify func(laneIDs []string) bool) (landed, ejected []string)
//	type LandingMode string
//	const ( LandingPerLane LandingMode = "per-lane"; LandingPrefixQueue LandingMode = "prefix-queue" )
//	func DefaultLandingMode() LandingMode         // compiled default = per-lane
//	func ParseLandingMode(s string) (LandingMode, error)
//
// PREDICATE STYLE (cycle-85 anti-gaming rule): every predicate CALLS the SUT and
// asserts on its return value — no source-grep predicate exists here. go/internal
// is importable from go/acs (cycle-962 precedent imports internal/core). The
// composer is pure/deterministic logic, so the ACS package IS the behavioral test
// and the Builder cannot game it by weakening a colocated unit test — the
// assertions live here, out of the Builder's edit surface.
//
// Adversarial diversity (skills/adversarial-testing §6):
//
//	POSITIVE → C975_001 (good lanes land around a poisoned middle), C975_002
//	           (window grows on green), C975_004 (canonical modes parse).
//	NEGATIVE → C975_001 (the poisoned lane must NOT land — strongest anti-no-op:
//	           a composer that lands everything fails this), C975_003 (an iffy /
//	           overlap-zone lane must NEVER appear in a multi-lane prefix),
//	           C975_004 (a bogus landing mode must ERROR, never default-accept).
//	EDGE     → C975_001 (verify-call count is bounded LINEARLY — proves NNFI, not
//	           an exponential powerset/bisection sweep), C975_002 (window floors at
//	           1 and never below under repeated reds).
//	SEMANTIC → culprit-ejection / AIMD-window / risk-tier-slotting / config-
//	           vocabulary are four DISTINCT behaviors, each asserted apart.
//
// AC map (1:1 with the disposition table in test-report.md):
//
//	AC1 TestPrefixQueue_PositionalCulpritEjectsAndReforms  → C975_001 (predicate)
//	AC2 TestPrefixQueue_AIMDWindowAdaptsToPassRate         → C975_002 (predicate)
//	AC3 TestPrefixQueue_IffyTierGetsSoloSlot + overlap-zone→ C975_003 (predicate)
//	AC4 single-writer only main-push path + ledger chaining→ manual+checklist (Auditor)
//	AC5 batch soak width 3: zero AUDIT_BINDING_HEAD_MOVED,  → manual+checklist (Auditor)
//	    watches==prefixes, go test -race PASS, apicover clean
//	AC6 config via policy fleet.landing (per-lane|prefix-queue), not env flag
//	                                                        → C975_004 (predicate)
```
