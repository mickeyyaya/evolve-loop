# Comment history: `internal/triagecap`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/triagecap/cardfiles.go:3` — above `import (`

```text
// cardfiles.go — the loud half of triage-cards-carry-files. project.go carries a
// DECLARED `files=` footprint from the report item into the companion's top_n
// card; this file makes an OMITTED footprint visible.
//
// Silence was the whole defect: cycle-1130 committed a card whose action prose
// named go/internal/phases/audit/audit.go and whose files[] was absent, so
// fleet.TodosFromTriage saw an id island and let a second lane take the same
// file. Nothing warned, in any log, for a whole batch. The check is deliberately
// NARROW — it fires only when the card's own prose already names a repo path, so
// legitimately footprint-free work (research, docs sweeps) stays silent and the
// warning keeps meaning something.
```

### `go/internal/triagecap/cardfiles_test.go:3` — above `import (`

```text
// cardfiles_test.go — RED contract for triage-cards-carry-files (inbox weight
// 0.89, campaign convergence-2026-07).
//
// Live evidence (batch-14, cycles 1133/1134): cycle-1130's triage-decision.json
// top_n card {id: surface-verdict-conflict-in-audit-classify, action: "capture
// pre-override agent verdict in Classify (go/internal/phases/audit/audit.go)…"}
// carried NO files[]. That {id, action} shape is exactly what
// ProjectDecisionJSON emits — the orchestrator's projection IS the de-facto
// writer, since the agent "in practice almost never" authors the companion
// (project.go header). With no files[], fleet.TodosFromTriage falls back to the
// id island, so the card looked disjoint from a backfilled card whose files[]
// named the SAME audit.go: two concurrent lanes editing one file, the 948
// lost-work class the disjointness planner exists to prevent.
//
// The fix is at the WRITER and it is STRUCTURED, never inferred: the report item
// carries a `files=` metadata field (the agent already names the paths in prose)
// and the projection parses it. Guessing paths out of the action prose is
// explicitly out of bounds — a wrong inferred file is worse than an island.
//
// RED today: projTopN has no Files field and MissingCardFilesWarning does not
// exist — this file does not compile.
```

### `go/internal/triagecap/cardfiles_test.go:38` — above `const cardFilesReport = '<!-- challenge-token: abc -->`

```text
// cardFilesReport is a production-shaped report whose FIRST top_n item declares
// its footprint and whose SECOND names a path in prose only — the exact defect
// shape observed on cycle-1130.
```

### `go/internal/triagecap/cardfiles_test.go:59` — above `func TestProjectDecisionJSON_TopNCardsCarryDeclaredFiles(t *testing.T) {`

```text
// TestProjectDecisionJSON_TopNCardsCarryDeclaredFiles is the writer contract: a
// declared files= footprint must reach the companion's top_n card as files[],
// which is the ONLY channel the fleet disjointness planner can read (menus are
// exact-file-overlap only, PR #366).
```

### `go/internal/triagecap/cardfiles_test.go:212` — above `func TestMissingCardFilesWarning_AgentCompanionIsTheAuthority(t *testing.T) {`

```text
// TestMissingCardFilesWarning_AgentCompanionIsTheAuthority pins the source of
// truth: ship/postship PREFERS an agent-authored triage-decision.json over the
// projection, so the companion is what the lane planner reads. A companion whose
// cards declare files[] must silence the report-based check (no false alarm), and a
// companion with the live cycle-1130 shape ({id, action} only) must be caught even
// when the report is unreadable.
```

### `go/internal/triagecap/declarative_floors_test.go:14` — above `const triageDecisionFile = "triage-decision.json"`

```text
// declarative_floors_test.go — RED tests for the cycle-304 declarative-floor-
// counter task (ADR-0046 Layer 1; inbox declarative-floor-commitment; phantom-
// floor incident 2026-06-12). The structural fix: floor counting becomes
// DECLARATION-primary (the triage-decision.json companion's committed_floors[])
// rather than prose-regex-primary, with the existing prose counter retained as
// fallback. Declarations are agent-owned ground truth — not constructable by the
// bullet contract's mandated metadata tokens — so they eliminate the phantom-
// floor class (cycle 301: an honest 2-bullet commitment counted 6 because
// evidence=/source=scout/"paths" matched real package basenames, making the
// correction directive unsatisfiable and burning the cycle).
//
// New API this file pins (Builder implements in floors.go):
//   - ReadDeclaredFloors(companionPath) ([]string, bool, error)
//   - CommittedFloorCount(artifact, companionPath, knownPkgs) int   (decl-primary)
//   - FloorDivergenceCorrective(artifact, companionPath, knownPkgs) string
// and the reviewer/recorder switch from CountCommittedFloors (prose) to
// CommittedFloorCount (declaration-primary) using the workspace companion at
// "<workspace>/triage-decision.json".
```

### `go/internal/triagecap/declarative_floors_test.go:118` — above `artifact := readFixture(t, "triage-cycle301.md")`

```text
// The incident replay: cycle-301 prose paired with its honest declaration.
```

### `go/internal/triagecap/declarative_floors_test.go:247` — above `func TestReviewer_UsesDeclaredFloors(t *testing.T) {`

```text
// TestReviewer_UsesDeclaredFloors pins that the R9.2 capacity clamp reads the
// companion declaration. The overpacked cycle-283 PROSE (12 floors) paired with
// an honest 2-floor declaration must now be APPROVED (declared 2 <= cap 7) — the
// phantom-floor false-reject the incident describes. The converse (lean prose,
// over-declared companion) must REJECT, proving the clamp truly uses the
// declaration in both directions (a no-op that ignored the companion would
// approve the 1-floor prose).
```

### `go/internal/triagecap/declarative_floors_test.go:283` — above `func TestRecorder_DeclaredFloors(t *testing.T) {`

```text
// TestRecorder_DeclaredFloors pins H2: the rolling throughput window must record
// the DECLARED floor count, not the prose count. Recording phantom prose floors
// poisoned K (K=4 from a true-K=1 cycle, cycle 298). With a declaration of 1 the
// recorded entry must be 1, even though the prose would have counted 3.
```

### `go/internal/triagecap/deferred.go:12` — above `var deferredHeadingRE = regexp.MustCompile('(?mi)^## (?:deferred|dropped)\b')`

```text
// deferred.go — R9.3: the deferred/dropped floor vocabulary. Triage's
// ## deferred and ## dropped sections are where the capacity clamp pushes
// overpacked floors; a TDD floor predicate that binds one of THOSE packages
// gates work the cycle never committed to (the cycle-280 failure mode).
```

### `go/internal/triagecap/deferred_test.go:10` — above `func TestDeferredFloorPackages_Cycle281Replay(t *testing.T) {`

```text
// R9.3: the deferred/dropped floor vocabulary — packages whose coverage
// floors triage explicitly pushed OUT of this cycle. TDD predicates binding
// these floors is the cycle-280 failure mode (builder starved the committed
// task while clearing deferred-task gates).
```

### `go/internal/triagecap/deferred_test.go:16` — above `pkgs := append([]string{"evolve"}, knownPkgsFixture...)`

```text
// cycle-281 deferred floor items name cmd/evolve ("evolve" is the package
// basename); the bridge item's only package reference is inside hyphenated
// slug compounds, which are single tokens — not mentions.
```

### `go/internal/triagecap/deferred_test.go:59` — above `name: "deferred metadata stripped including full defer_reason prose",`

```text
// Pins the metadata-strip semantics on deferred items: the
// contract fields' own vocabulary never counts, and the ENTIRE
// defer_reason value (to end of line) is stripped — defer
// reasons are scheduling prose that routinely references OTHER
// work. Cycle 310 (soak #3d) proved the earlier tail-matchable
// reading wrong: "co-scheduling with the looppreflight blocker
// fix" made Gate C block the COMMITTED package's predicates.
```

### `go/internal/triagecap/deferred_test.go:83` — above `name: "cycle-310 replay: defer_reason referencing committed work does not count",`

```text
// Cycle-310 verbatim replay (soak #3d): the deferred ledger-seal
// item's defer_reason references the committed looppreflight
// blocker — that mention must NOT make looppreflight a deferred
// floor package (it blocked the committed task's own predicates).
```

### `go/internal/triagecap/deferred_test.go:105` — above `func writeDeferredCompanion(t *testing.T, dir string, deferredFloors []string) string {`

```text
// ----------------------------------------------------------------------------
// ADR-0046 Layer 1 (cycle 305): declaration-primary deferred floors.
//
// New API this file pins (Builder implements in deferred.go, mirroring the
// shipped committed-floor path in floors.go):
//
//   - ReadDeferredFloors(companionPath) ([]string, bool, error)
//       reads triage-decision.json's deferred_floors[]; missing file / missing
//       field is NOT an error (returns nil,false,nil → caller falls back to
//       prose), parallel to ReadDeclaredFloors.
//   - DeferredFloorPackagesDecl(artifact, companionPath, candidatePkgs) []string
//       declaration-PRIMARY: when deferred_floors[] is present, the declared
//       packages (filtered to candidatePkgs, sorted, distinct) are authoritative
//       and prose is ignored; otherwise it falls back to prose DeferredFloorPackages.
//   - DeferredFloorDivergence(artifact, companionPath, knownPkgs) string
//       the deferred analog of FloorDivergenceCorrective: an actionable, non-empty
//       message when prose-deferred packages and deferred_floors[] disagree; ""
//       when they agree or no declaration exists. The triage-floors guard prints it.
//
// These are RED until Builder adds the three functions: the test package
// fails to compile against the absent symbols.
// ----------------------------------------------------------------------------
```

### `go/internal/triagecap/demotion.go:16` — above `var digitRunRE = regexp.MustCompile('[0-9]+')`

```text
// demotion.go — ADR-0046 Layer 2 for the one production heuristic gate (this
// capacity clamp). A heuristic gate rejecting with a byte-identical reason
// TEMPLATE across two consecutive cycles is treated as a gate defect, not a
// work defect: real overpacking varies cycle to cycle (different tasks,
// different counts); identical rejections are a determinism artifact. The
// response is bounded relief: the gate runs SHADOW for exactly ONE cycle —
// the first cycle reviewed after the pair, where operator resets that seal
// intermediate cycles without a rejection record are transparent gaps
// (cycle 450: SIGINT + `cycle reset --force` after the 448/449 pair left a
// hole the old -1/-2 adjacency demand could not see across, so demotion
// could not fire until two MORE cycles burned). The pair's auto-filed inbox
// defect doubles as the relief-consumption marker, so the loop fixes the
// gate instead of burning more cycles against it (cycles 301/302, soak #2:
// the phantom-floor counter killed two cycles — including the one carrying
// its own fix — before an operator intervened).
//
// Demotion lives INSIDE the reviewer (consulted at rejection time in
// Review), not in a separate constructor: cycle 307 built this logic as a
// helper the composition root never called, and the audit rejected the dead
// wiring. There is nothing to forget here — NewReviewer is the production
// constructor and demotion ships with it.
//
// The full ADR-0046 fact-vs-heuristic GateClass taxonomy is deliberately
// NOT built: exactly one heuristic gate exists today, and a registry for
// one member is design-for-hypothetical-futures. When a second heuristic
// gate appears, lift ReasonTemplateHash/ShouldDemote into the shared seam
// the ADR describes.
```

### `go/internal/triagecap/demotion.go:47` — above `func ReasonTemplateHash(reason string) string {`

```text
// ReasonTemplateHash collapses a rejection reason to its template identity:
// every digit run is replaced by a token carrying only its LENGTH, then the
// result is hashed. Same-magnitude jitter ("6 floors / cap 5" vs "7 floors /
// cap 5") collapses to one template; order-of-magnitude differences ("7" vs
// "700") survive as D1 vs D3 — the cycle-306 lesson: jitter-insensitive but
// magnitude-sensitive, never erase digits wholesale.
```

### `go/internal/triagecap/demotion.go:79` — above `const demotionWindow = 3`

```text
// demotionWindow bounds how stale the recorded pair may be: the newer
// rejection must lie within this many cycles of currentCycle. 1 covers the
// no-gap case (the pair immediately precedes the review); 3 tolerates up to
// two reset-sealed cycles between the pair and now (the cycle-450 incident
// shape). Pairs staler than the window never demote — an auto-filed defect,
// not standing relief, is the durable response.
```

### `go/internal/triagecap/demotion_test.go:15` — above `const (`

```text
// demotion_test.go — ADR-0046 Layer 2: identical-rejection demotion for the
// triage capacity clamp (the one production heuristic gate). A heuristic
// gate rejecting with a byte-identical reason TEMPLATE across two
// consecutive cycles is a gate defect, not a work defect: real overpacking
// varies cycle to cycle; identical rejections are a determinism artifact
// (cycles 301/302, soak #2 — the phantom-floor counter re-rejected an
// honest commitment until both cycles burned their corrections and died).
//
// Two prior loop attempts at this slice failed and are pinned here:
//   - cycle 306: the hash erased ALL digits, collapsing "7 floors / cap 6"
//     with "700 floors / cap 600" — the template must be jitter-insensitive
//     but MAGNITUDE-sensitive (digit-run length survives, digit values do
//     not).
//   - cycle 307: the demotion helper existed but was never called from the
//     composition root. Demotion therefore lives INSIDE NewReviewer — there
//     is no separate constructor to forget.
```

### `go/internal/triagecap/demotion_test.go:32` — above `const (`

```text
// Verbatim summaries from state.json:failedApproaches, cycles 301/302.
```

### `go/internal/triagecap/demotion_test.go:79` — above `t.Run("window scope: fires through a reset-sealed gap (cycle 304)", func(t *testing.T) {`

```text
// Adapted for F4 (cycle 459, inbox triagecap-prose-counter-defect):
// ShouldDemote is now window-scoped so reset-sealed cycles between the
// pair and the review are transparent gaps; the one-cycle relief bound
// moved to the Review seam, which tracks consumption via the pair's
// auto-filed defect marker (TestCapReviewer_ReliefIsOneCycleThenEnforces
// pins that production behavior).
```

### `go/internal/triagecap/demotion_test.go:191` — above `r, in, _ := newDemotionFixture(t, "303", nil)`

```text
// Same overpacked artifact, no failure history → the clamp still BLOCKs.
// Demotion must never weaken first-offense enforcement (cycle-307
// composition-root lesson: this exercises the REAL production reviewer,
// not a helper that wiring can forget).
```

### `go/internal/triagecap/demotion_test.go:211` — above `func TestNormalizeRemedyStatus(t *testing.T) {`

```text
// --- cycle-1301: remedy_status on the demotion ledger record ---------------
//
// The auto-filed record is the DURABLE ledger entry for a demotion event, but
// it only ever carried a prose `action` narrative: nothing on it says whether a
// salvage of the suspected gate defect was ATTEMPTED or whether the loop
// concluded NO REMEDY was possible. Commit 29915424 had to explain two gate
// demotions in a queue chore commit body for exactly that reason. remedy_status
// is caller-declared (the writer has no way to infer it), defaults to `pending`
// at file time, and carries a CLOSED vocabulary — an unknown value normalises
// to pending rather than being written through, so a downstream reader can
// switch on three cases and no more.
```

### `go/internal/triagecap/dispatch_rank_test.go:10` — above `func ids(cs []FleetCandidate) []string {`

```text
// dispatch_rank_test.go — F29: among equal operator weights, the seed commits
// lanes to work whose admissibility is PROVEN before work whose admissibility is
// unknown. A lane's fleet scope is ONE item, so an item whose surface triage
// later derives as protected has no alternative to fall back on. The weight
// stays the priority (architecture review: admissibility must never silently
// override it); the queue clusters at equal weights, so the tie-break matters.
```

### `go/internal/triagecap/floors.go:1` — above `package triagecap`

```text
// Package triagecap bounds per-cycle coverage-floor commitments by observed
// builder throughput (R9; inbox coverage-floor-overpacking). Three consecutive
// coverage cycles failed on the same shape — triage committing ~12 package
// floors when the observed sustainable throughput is ~5 per builder turn
// (cycle 281, the PASS baseline). The package has three parts:
//
//   - floors.go  — deterministic committed-floor counter over the triage
//     artifact's ## top_n section (deferred/dropped floors do not count);
//   - window.go  — the rolling throughput window persisted in
//     state.json:triageThroughput (core.TriageThroughputEntry);
//   - reviewer.go — the capacity clamp at the orchestrator's deliverable-
//     review seam, rejecting overpacked triage through the correction ladder.
```

### `go/internal/triagecap/floors.go:171` — above `func CommittedFloorPackages(artifact, companionPath string, candidatePkgs []string) []string {`

```text
// CommittedFloorPackages returns the candidate packages committed as floors
// this cycle — declaration-primary (committed_floors filtered to the
// candidates), prose fallback otherwise. Gate C subtracts this set from the
// deferred set so a package listed on BOTH sides resolves committed-wins:
// a floor predicate on this cycle's own committed package is a legitimate
// ratchet, never the cycle-280 starvation the gate exists to block.
```

### `go/internal/triagecap/floors.go:299` — above `var metadataFieldRE = regexp.MustCompile('\bdefer_reason=[^\n]*|\b(?:source|priority)=\S+|\bevidence=')`

```text
// metadataFieldRE strips the bullet contract's own metadata before package
// matching: the contract REQUIRES every item to carry evidence=/source=scout
// fields, and those literals collide with the real packages core/evidence and
// phases/scout — every conformant bullet counted +2 phantom floors, which made
// the cap's correction directive unsatisfiable (cycle 301: an honest 2-bullet
// commitment counted 6, burned both corrections, failed the cycle). The
// source=/priority= values are closed contract vocabulary, dropped whole;
// defer_reason= is stripped to END OF LINE — defer reasons are free-form
// scheduling prose that routinely references OTHER work ("co-scheduling
// with the looppreflight blocker fix", cycle 310: that mention made Gate C
// block the COMMITTED package's own predicates — a reason naming a package
// is NOT a floor on that package). The evidence= VALUE is kept because
// evidence pointers carry real package paths
// ("evidence=go/internal/clihealth/clihealth.go"). RE2's ASCII \b also fires
// after a hyphen, so a hypothetical slug like "low-priority=x" is stripped
// too — that only ever undercounts (fail-open direction).
```

### `go/internal/triagecap/floors.go:341` — above `var pathOnlyPkgs = map[string]*regexp.Regexp{`

```text
// pathOnlyPkgs are packages whose basenames are also ordinary coverage prose;
// they count only when slash-qualified ("internal/paths"), never as bare
// tokens — cycle 298's "safety-critical paths" counted a phantom floor for
// go/internal/paths and poisoned the throughput window (K=4, true K=1).
// Each pattern requires a token boundary after the name (same character
// class as tokenRE), so "internal/pathsX" is not a mention of "paths".
// Matching runs on the metadata-stripped item, in which evidence= VALUES
// survive — "evidence=go/internal/paths/util.go" therefore counts paths,
// deliberately: that is a real package reference, the mirror image of the
// prose phantom this list suppresses. Read-only after init.
```

### `go/internal/triagecap/floors_test.go:9` — above `var knownPkgsFixture = []string{`

```text
// knownPkgsFixture mirrors the package basenames that exist in the repo —
// the subset relevant to the replay fixtures plus common English-word
// packages to prove word-boundary matching does not overcount. It includes
// the names that collide with the triage bullet contract's own vocabulary
// (`evidence`, `scout` — every bullet must carry evidence=/source=scout)
// and with coverage prose (`paths` — "error paths"): cycle 301 failed on
// exactly these phantoms, so the production vocabulary must be represented
// here or the replay pins prove nothing.
```

### `go/internal/triagecap/floors_test.go:56` — above `func TestCountCommittedFloors_Cycle301Replay(t *testing.T) {`

```text
// TestCountCommittedFloors_Cycle301Replay pins the soak-#2 incident
// (2026-06-12): a correctly-sized 2-bullet coverage commitment was counted
// as 6 floors because the contract-mandated evidence=/source=scout fields
// and the prose word "paths" matched real package basenames. The phantom
// floors made the correction directive unsatisfiable — triage could not
// remove tokens its own bullet contract requires — so the cycle burned both
// corrections and failed. True count: one package per bullet.
```

### `go/internal/triagecap/floors_test.go:71` — above `func TestCountCommittedFloors_Cycle298Bullet(t *testing.T) {`

```text
// TestCountCommittedFloors_Cycle298Bullet pins the window-poisoning shape:
// cycle 298's single floor-bearing bullet was recorded as 4 floors
// (gc + evidence + scout + "safety-critical paths"), fabricating K=4 for
// the throughput window. True count: 1.
```

### `go/internal/triagecap/gate_defect_chain_amplified_test.go:14` — above `func TestCountCommittedFloors_TargetScopeAmplified(t *testing.T) {`

```text
// gate_defect_chain_amplified_test.go — cycle-459 test-amplification lane for
// the F1–F5 gate-defect-chain contract (inbox triagecap-prose-counter-defect).
// Authored black-box from the inbox spec, the TDD contract
// (gate_defect_chain_test.go), and the package's exported doc surface — not
// from the implementation. The TDD tests replay the cycle-448/449 incident;
// these probe the boundaries the incident did not exercise: alternate target
// markers, clause cut-off on either side of a target, per-item dedup, the
// ## dropped section, artifact scale, the ShouldDemote window/template
// bounds, the declaration-primary reject reason, the undeclared-companion
// WARN, and relief idempotency under re-review.
```

### `go/internal/triagecap/gate_defect_chain_test.go:15` — above `var gapGoldenPkgs = []string{`

```text
// gate_defect_chain_test.go — cycle-459 TDD contract for the triage-cap gate
// defect chain F1–F5 (inbox triagecap-prose-counter-defect; post-mortem of
// cycles 448/449, which both died at this gate after complying with its
// correction twice). These tests are authored RED-FIRST by the TDD engineer;
// the Builder makes them GREEN without modifying them.
//
//   F1 — the prose counter counts packages named in EVIDENCE citations as
//        committed floors (cycle 449: 3 true floors counted as 7).
//   F2 — the reject reason states neither the counting rule nor the
//        declaration-primary escape (triage-decision.json committed_floors[]).
//   F3 — nothing checks that a floor-bearing report carries the declaration
//        companion (the declaration-primary design has no producer check).
//   F4 — ShouldDemote requires rejections at currentCycle-1 AND -2, so an
//        operator reset that seals a cycle without a rejection record breaks
//        the identical-rejection demotion chain (cycle 450 reset ⇒ 451/452
//        cannot demote).
//   F5 — the reject reason shows the count but not WHICH packages were
//        counted, so corrections are not self-explanatory.
//
// The goldens are the EXACT preserved cycle-448/449 triage artifacts
// (testdata/triage-cycle44{8,9}-golden.md, byte-identical to
// .evolve/runs/cycle-44{8,9}/triage-report.md).
```

### `go/internal/triagecap/gate_defect_chain_test.go:38` — above `var gapGoldenPkgs = []string{`

```text
// gapGoldenPkgs mirrors the production package vocabulary relevant to the
// cycle-448/449 goldens: the three true floor targets (core, bridge, audit)
// plus the phantom sources the defective counter attributed (scout via kept
// evidence= values, sysexec via "sysexec.RunFunc" in item prose) and
// distractors that must never count. With this vocabulary the unfixed
// counter yields 7 on the cycle-449 golden — the exact incident count.
```

### `go/internal/triagecap/gate_defect_chain_test.go:50` — above `func TestCountCommittedFloors_Cycle449GoldenReplay(t *testing.T) {`

```text
// TestCountCommittedFloors_Cycle449GoldenReplay (F1, golden) — the normative
// acceptance of the fix: cycle 449 committed exactly three coverage floors
// (core 85.0 / bridge 94.5 / audit 96.0, one per top_n item) and was killed
// because evidence citations ("core 83.1%", "bridge 93.5%; matchExhausted
// 66.7%", "audit 92.6%", "scout fresh cover-func", "sysexec.RunFunc")
// inflated the count to 7. The EXACT report must count 3 after the fix.
```

### `go/internal/triagecap/gate_defect_chain_test.go:64` — above `func TestCountCommittedFloors_Cycle448GoldenReplay(t *testing.T) {`

```text
// TestCountCommittedFloors_Cycle448GoldenReplay (F1, anti-weakening golden) —
// cycle 448 genuinely committed FOUR floor targets (item 1: "coverage floors
// core ≥85.0%, audit ≥96.0%, bridge ≥94.5%" = 3; item 2: "core total
// coverage ... to ≥86.0%" = 1). The fix must scope out evidence citations
// WITHOUT collapsing true multi-package floor commitments: this golden counts
// 4 before AND after the fix. The gate's purpose (cycles 280/282/283 real
// overpacking) stays intact.
```

### `go/internal/triagecap/gate_defect_chain_test.go:113` — above `func TestCapReviewer_RejectReasonStatesDeclarationEscape(t *testing.T) {`

```text
// TestCapReviewer_RejectReasonStatesDeclarationEscape (F2) — the corrective
// must be actionable: an agent that cannot see the counting rule cannot
// comply with it (cycles 448/449 complied with the natural reading twice and
// were killed twice). The reject reason must name the declaration-primary
// escape: emit triage-decision.json with committed_floors[].
```

### `go/internal/triagecap/gate_defect_chain_test.go:154` — above `const (`

```text
// Same-template rejection summaries for cycles 448/449, shaped like the real
// state.json:failedApproaches records (same digit-run lengths throughout, so
// ReasonTemplateHash collapses them — the incident's determinism artifact).
```

### `go/internal/triagecap/gate_defect_chain_test.go:189` — above `func TestCapReviewer_ResetSealedGapStillDemotes(t *testing.T) {`

```text
// TestCapReviewer_ResetSealedGapStillDemotes (F4) — the incident replay:
// cycles 448 and 449 recorded same-template rejections from this gate; an
// operator SIGINT + `cycle reset --force` sealed cycle 450 WITHOUT a
// rejection record. Reviewing cycle 451, the last two RECORDED same-template
// rejections (448, 449) are within the demotion window, so the gate must
// treat the reset-sealed 450 as a transparent gap: demote to shadow
// (approve) and auto-file exactly one defect. Today ShouldDemote demands
// records at 450 AND 449, so 451 enforces and burns — RED.
```

### `go/internal/triagecap/gate_defect_chain_test.go:212` — above `func TestCapReviewer_ReliefIsOneCycleThenEnforces(t *testing.T) {`

```text
// TestCapReviewer_ReliefIsOneCycleThenEnforces (F4, bounded relief) — the
// demotion's one-cycle relief semantics survive the gap fix: after cycle 451
// consumes the 448/449 pair's relief (shadow + auto-filed defect), cycle 452
// with the SAME history must enforce again. This pins against the naive
// pure-window implementation, under which the pair would keep granting
// relief to every cycle in the window.
```

### `go/internal/triagecap/gate_defect_chain_test.go:231` — above `func TestCapReviewer_StaleRejectionPairOutsideWindowEnforces(t *testing.T) {`

```text
// TestCapReviewer_StaleRejectionPairOutsideWindowEnforces (F4, negative) — a
// same-template pair far in the past grants no relief: the demotion window
// is small (the fix's own spec suggests ~3 cycles). A 444/445 pair reviewed
// at cycle 451 must keep enforcing. GREEN today and must stay GREEN — this
// is the anti-overcorrection bound on the gap transparency.
```

### `go/internal/triagecap/lane_menu.go:3` — above `import (`

```text
// lane_menu.go — fleet lanes consume a MENU, not a single todo
// (fleet-lane-batch-menu). SelectFleetWidthTopN hands each lane one
// representative and discards its partition bucket-mates, so a fleet cycle
// could never amortize its worktree/build/audit across the same-file items
// the batching layer deliberately groups (live proof: batch-14 wave-1, both
// lane scopes single-element while the triage prompt's "prefer a whole batch
// as top_n" guidance sat unreachable). Expansion deepens each lane with its
// same-file cluster mates; triage inside the lane keeps full authority to
// commit a subset and leave the rest pending — unworked menu ids are never
// claimed, so they simply remain dispatchable backlog.
```

### `go/internal/triagecap/lane_menu.go:113` — above `func PruneConsumed(evolveDir string, committed []FleetCandidate) []FleetCandidate {`

```text
// PruneConsumed drops carried-over committed ids the inbox lifecycle has
// already consumed (wave-planner-pass-scope-prune). WidenTopNToFleetWidth
// copies the committed prefix through VERBATIM, so without this a consumed id
// is re-pinned into a later wave's lane-scope.json — cycle-1116 re-pinned
// tdd-topn-binding-gate after cycle-1113 consumed it. Pruning at the SEED makes
// the plan artifact itself honest instead of leaning on the launch-time
// freshness gate to skip a lane that should never have been planned.
//
// Exported because the PRIMARY per-wave seam (widenNarrowDecision in package
// main, go/cmd/evolve/cmd_loop_wave.go) must reuse this exact primitive rather
// than reimplement it — this is the single source for "is a committed id still
// live", not a seed-path-only helper.
//
// Only TERMINAL states prune: processed/consumed/rejected/quarantine. pending and
// processing stay (still live work), and — load-bearing — an id with no
// lifecycle evidence at all stays too. A prune that dropped what it cannot
// resolve would starve every wave of non-inbox-backed cards.
```

### `go/internal/triagecap/lane_menu_prune_export_test.go:3` — above `import (`

```text
// lane_menu_prune_export_test.go — cycle-1182 RED contract for
// wave-planner-pass-scope-prune.
//
// pruneConsumed already implements the terminal-state prune, but it is
// package-private, so the PRIMARY per-wave planning seam
// (widenNarrowDecision in package main, go/cmd/evolve/cmd_loop_wave.go) cannot
// reach it and carries consumed ids forward verbatim (cycle-1116 re-pinned
// tdd-topn-binding-gate after cycle-1113 consumed it).
//
// CONTRACT for Builder (do NOT modify these tests — implement production code):
//   - triagecap exports PruneConsumed(evolveDir string, committed []FleetCandidate) []FleetCandidate
//     with pruneConsumed's exact semantics (terminal states drop; everything
//     else, including no-evidence ids, is retained — fail open).
//   - The existing seed path keeps its behaviour; this is a single-source
//     export, not a second implementation (never_duplicate_centralize).
```

### `go/internal/triagecap/lane_menu_test.go:3` — above `import (`

```text
// lane_menu_test.go — fleet lanes consume a MENU, not a single todo
// (fleet-lane-batch-menu). Live wave-1 of batch-14 (cycles 1127/1128): both
// lane scopes carried a single-element todo_ids while the triage prompt's
// "prefer selecting a whole batch as top_n" guidance sat unreachable —
// SelectFleetWidthTopN kept only b[0] per partition bucket, discarding the
// bucket-mates that share the lane's files. Expansion deepens each lane with
// its SAME-FILE cluster mates (one worktree, one build, one audit amortized)
// while never touching width: independent work stays width-material, a
// bridge candidate joins nothing, and cross-lane file-disjointness is
// preserved by construction.
```

### `go/internal/triagecap/lane_menu_test.go:205` — above `func TestSelectWaveSeedMenus_PreservesCommittedPrefix(t *testing.T) {`

```text
// --- cycle 1159: menu-pass-preserve-committed-ids ------------------------
//
// RED tests for the committed-prefix gap: SelectWaveSeedMenus seeded its base
// selection with SelectFleetWidthTopN(backlog, count) — a FRESH top-N pick with
// no committed input — while the sibling seam WidenTopNToFleetWidth(committed,
// backlog, count) exists precisely to preserve an already-committed prefix while
// widening to fleet width. A menu-pass invocation could therefore silently drop
// or reorder ids a caller had already committed to.
//
// CONTRACT for Builder (do NOT modify these tests — implement production code):
//   - SelectWaveSeedMenus gains a `committed []FleetCandidate` parameter:
//     SelectWaveSeedMenus(evolveDir string, committed []FleetCandidate,
//                         count, perLane int, isProtected func(string) bool)
//   - It seeds via WidenTopNToFleetWidth(committed, backlog, count) instead of
//     SelectFleetWidthTopN(backlog, count), then deepens with
//     ExpandWithClusterMates as before.
//   - committed == nil must reproduce today's behavior byte-identically (the
//     one production caller, seedWavePlanFromInbox, has no committed prefix).
//   - ExpandWithClusterMates keeps its signature: it already honors an
//     overlapping committed prefix ("first lane keeps the file").
```

### `go/internal/triagecap/malformed_floors_amplified_test.go:3` — above `import (`

```text
// malformed_floors_amplified_test.go — cycle-308 adversarial amplification
// for MalformedCommittedFloorWarning and MalformedDeferredFloorWarning
// (companion-malformed-must-surface task).
//
// Targets gaps in the TDD contract: empty-file (0 bytes), truncated JSON, and
// cross-function consistency on the same malformed companion.
```

### `go/internal/triagecap/malformed_floors_test.go:3` — above `import (`

```text
// malformed_floors_test.go — RED tests for cycle-308 task
// `companion-malformed-must-surface` (inbox item 2026-06-12T16-13-51Z; cycle-305
// audit M1).
//
// Both CommittedFloorCount (floors.go) and DeferredFloorPackagesDecl
// (deferred.go) read the triage-decision.json companion with the pattern
// `if declared, ok, err := Read...; err == nil && ok { ... }`. When err != nil
// (the companion is present but its JSON is malformed) the count SILENTLY falls
// through to the prose scanner — indistinguishable from a missing file. A
// corrupt companion that the agent THINKS is governing the cycle therefore has
// zero effect with no signal.
//
// The fix separates three cases:
//
//	absent file            → silent fallback (backward compat)
//	present, field absent  → silent fallback (backward compat)
//	present-but-malformed  → SURFACE a non-empty correction string carrying the
//	                         JSON parse error
//
// New API this file pins (Builder implements):
//
//	MalformedCommittedFloorWarning(companionPath string) string   (floors.go)
//	MalformedDeferredFloorWarning(companionPath string) string    (deferred.go)
//
// — each returns a non-empty parse-error detail ONLY when the companion is
// present-but-malformed; "" for absent file, absent field, and well-formed.
// CommittedFloorCount / DeferredFloorPackagesDecl keep their fail-open prose
// fallback unchanged (no regression). writeCompanion lives in
// declarative_floors_test.go (same package).
```

### `go/internal/triagecap/project.go:9` — above `var idSlugRE = regexp.MustCompile('^[a-z0-9][a-z0-9-]*$')`

```text
// project.go — deterministic projection of triage-report.md into the
// triage-decision.json companion the inbox-lifecycle hook (ship.postship
// promoteInbox) consumes. The triage AGENT is instructed to emit the companion
// but in practice almost never does (cycles 308/316/320-322 all missing it), so
// promote-to-processed never ran and claimed items ping-ponged back to inbox
// every cycle. This is the robust fallback: parse the markdown the agent DID
// write — single source (the report), guaranteed present — instead of trusting
// the LLM to also hand-author a parallel JSON that can drift from it.
//
// SUBSET BY DESIGN — each omitted field has absent-safe consumer semantics, so a
// projected companion is behaviourally identical to the no-companion baseline
// for every gate while still being PRESENT for promotion:
//   - committed_floors / deferred_floors are OMITTED: ReadDeclaredFloors /
//     ReadDeferredFloors treat an absent field as "fall back to the prose
//     counter", so the cap + eval gates keep counting the markdown directly.
//   - skip_shipped / skip_rejected / escalate_block require Step-0a's git-log
//     idempotency judgment, which the markdown does not carry; they are left
//     absent, so extractIDs walks top_n only — the documented inbox-lifecycle
//     behaviour (a shipped cycle is deemed to have addressed its top_n).
```

### `go/internal/triagecap/project.go:53` — above `Files []string 'json:"files,omitempty"'`

```text
// Files is the card's DECLARED repo footprint, parsed from the item's
// `files=a;b` metadata field. It is the only channel the fleet disjointness
// planner can read (fleet.TodosFromTriage → Todo.Files → Partition/menus are
// exact-file-overlap only): a card without it becomes an id island, so a
// second lane can concurrently edit the same file it names in prose
// (triage-cards-carry-files; live on cycles 1130/1133/1134). omitempty and
// NEVER inferred from the action prose — an absent footprint is honest, a
// guessed one silently merges or splits real lanes.
```

### `go/internal/triagecap/project.go:117` — above `if body, ok := sectionBody(artifact, supersededSectionRE); ok {`

```text
// superseded[] names inbox ids whose work already shipped under a different
// id — retired by id ALONE at ship (inboxmover.ReconcileSuperseded), the
// durable close of the cycle-544..548 orphan gap. Deduped here so the ship
// hook receives a clean list even if the report repeats an id.
```

### `go/internal/triagecap/project_test.go:9` — above `const realReport = '<!-- challenge-token: abc -->`

```text
// realReport mirrors a production triage-report.md (cycle-322 shape): the
// canonical headings, the "- {id}: {action} — metadata" item format, and a
// dropped section with reason= tails.
```

### `go/internal/triagecap/reviewer.go:15` — above `type CapReviewer struct {`

```text
// reviewer.go — the R9.2 capacity clamp at the orchestrator's per-phase
// deliverable-review seam (chained with the evalgate + contract gates via
// core.ChainReviewers). A reject here enters the existing correction ladder:
// triage is re-dispatched with the cap directive injected as a
// "## Correction" block, so the agent re-shapes top_n instead of the cycle
// burning on an overpacked commitment (inbox coverage-floor-overpacking —
// cycles 280/282/283 all failed on floors the builder demonstrably cannot
// clear in one turn).
//
// Posture (matches the contract gate):
//   - Only the triage phase is in scope; everything else is approved.
//   - Ambiguity (missing/unreadable artifact) → fail OPEN.
//   - StageShadow → log-only.
//   - StageEnforce → reject with an actionable cap directive. The ladder
//     bounds retries, so a miscalibrated clamp costs corrections, not a
//     bricked loop.
```

### `go/internal/triagecap/reviewer.go:59` — above `func readWindow(projectRoot string) []core.TriageThroughputEntry {`

```text
// readWindow loads the rolling throughput window from state.json. The read
// does NOT acquire the project lock — safe because the orchestrator's own
// lock prevents concurrent runs, and WriteState's atomic rename prevents
// torn reads; we deliberately see the window as of the last completed
// cycle. Any read failure yields an empty window — i.e. the cycle-281 seed.
```

### `go/internal/triagecap/reviewer.go:115` — above `counted := CommittedFloorPackages(string(data), companionPath, pkgs)`

```text
// F2+F5: the correction must be actionable and self-explanatory — state
// the counting rule, WHICH packages the counter attributed, and the
// declaration-primary escape. Cycles 448/449 complied with the natural
// reading of the bare cap directive twice and were killed twice because
// they could not see either the rule or the escape.
```

### `go/internal/triagecap/reviewer.go:135` — above `if cycle, ok := workspaceCycleID(in.Workspace); ok {`

```text
// ADR-0046 Layer 2: before enforcing, consult the identical-rejection
// demotion (demotion.go). The last two RECORDED cycles rejected with
// this exact template (reset-sealed cycles are transparent gaps, F4) ⇒
// the gate is the suspect ⇒ shadow for ONE cycle, with an auto-filed
// defect. Consulted at rejection time so a healthy approve path never
// pays the state.json read.
```

### `go/internal/triagecap/reviewer_test.go:42` — above `func TestCapReviewer_Cycle283ShapeRejected(t *testing.T) {`

```text
// TestCapReviewer_Cycle283ShapeRejected — the R9.2 acceptance replay: the
// overpacked cycle-283 artifact (12 floors) against the empty-window seed
// (K=5, cap=7) must reject at enforce with an actionable cap directive.
```

### `go/internal/triagecap/topn_width.go:10` — above `type FleetCandidate struct {`

```text
// topn_width.go — fleet-width-aware, file-disjoint top_n selection (inbox
// triage-supply-disjoint-topn-for-fleet-width). Cycle-503 triage committed
// exactly 1 top_n task and starved the fleet wave planner of the >=2 disjoint
// tasks it needs to fan out 2 concurrent lanes. SelectFleetWidthTopN is the
// SSOT: it greedily packs the highest-weight candidates into up to `count`
// mutually FILE-DISJOINT lanes and returns one representative per non-empty
// lane, so the returned set is always safe to fan out 1:1 into concurrent
// `evolve cycle run` lanes without a cross-lane file collision.
```

### `go/internal/triagecap/topn_width.go:27` — above `Declared bool`

```text
// Declared is inboxbatch.Item.DeclaredSurface as the backlog read found it —
// the item declares a path-shaped surface the console classifier already
// cleared (F29). Candidates built from a triage decision keep the zero
// value: unverified.
```

### `go/internal/triagecap/topn_width.go:34` — above `func SelectFleetWidthTopN(candidates []FleetCandidate, count int) []FleetCandidate {`

```text
// SelectFleetWidthTopN returns up to `count` mutually file-disjoint top_n
// representatives, highest-weight first. It delegates the disjoint packing to
// fleet.Partition (the SSOT greedy file-ownership algorithm) rather than
// duplicating it, then lifts one representative — the highest-weight member —
// out of each non-empty bucket.
//
// count<2 reproduces the legacy single-focus behavior: exactly the single
// highest-weight candidate, independent of file overlap (among equal weights,
// rankForDispatch prefers the verified-admissible one — F29). When the
// backlog cannot fill `count` disjoint lanes, the widest disjoint set (>=1) is
// returned — never a fabricated/overlapping pairing.
```

### `go/internal/triagecap/topn_width.go:146` — above `func rankForDispatch(cands []FleetCandidate) []FleetCandidate {`

```text
// rankForDispatch is the ONE ordering every seed path shares — the wave seed
// (SelectFleetWidthTopN), the per-wave widen seam (WidenTopNToFleetWidth) and
// the lane menus (ExpandWithClusterMates): highest operator weight first — the
// weight IS the priority, and admissibility never silently overrides it — then,
// among equal weights (common: the queue clusters at 0.80/0.84/0.85), a
// candidate whose declared surface the console classifier already cleared
// before one whose surface is unknown (F29), then input order. A lane's fleet
// scope is one item, so among otherwise-equal work prefer the proven kind.
// Returns a new slice; the input is never reordered.
```

### `go/internal/triagecap/topn_width_amplified_test.go:10` — above `func c541ampCand(id string, weight float64, files ...string) FleetCandidate {`

```text
// topn_width_amplified_test.go — cycle-541 test-amplification lane for
// WidenTopNToFleetWidth (inbox triage-supply-disjoint-topn-for-fleet-width).
// Authored black-box from the exported godoc contract and test-report.md's
// builder contract (preserve committed verbatim; count<2 no-op; backfill
// highest-weight-first skipping duplicate-ID/file-overlap; never fabricate a
// colliding lane) — not from topn_width.go's implementation. These probe
// boundaries the RED predicates (TestC541_004..006) did not exercise:
// negative counts, nil/empty inputs, multi-file partial overlap, duplicate
// IDs inside the backlog itself, mutation safety, and large-scale backfills.
```

### `go/internal/triagecap/topn_width_test.go:5` — above `func TestTopN_FleetWidthAware_ProducesDisjointSet(t *testing.T) {`

```text
// topn_width_test.go pins the fleet-width-aware, file-disjoint top_n
// selection (inbox triage-supply-disjoint-topn-for-fleet-width, weight 0.94):
// cycle-503 triage committed exactly 1 top_n task and starved the fleet wave
// planner of the >=2 disjoint tasks it needs to fan out 2 concurrent lanes.
// SelectFleetWidthTopN(candidates, count) is the new SSOT: it greedily packs
// the highest-weight candidates into up to `count` mutually FILE-DISJOINT
// lanes (delegating to fleet.Partition's greedy-ownership algorithm — see
// go/internal/fleet/partition.go) and returns one representative per
// non-empty lane, so the returned set is always safe to fan out 1:1 into
// concurrent `evolve cycle run` lanes without a cross-lane file collision.
//
// count<2 must reproduce today's single-focus behavior byte-identically
// (inbox acceptance #3): exactly the single highest-weight candidate,
// independent of file overlap.
```

### `go/internal/triagecap/wave_seed.go:32` — above `func ReadInboxBacklog(evolveDir string, isProtected func(string) bool) []FleetCandidate {`

```text
// ReadInboxBacklog reads every <evolveDir>/inbox/*.json todo into an unpacked
// []FleetCandidate (id + weight + declared files), in filename order so
// equal-weight ties stay deterministic. Unreadable / malformed files and
// empty-id todos are skipped (best-effort — a bad inbox never breaks dispatch).
// It is the single source for "inbox backlog as fleet candidates," shared by the
// wave-seed fallback (SelectWaveSeedTopN) and the widen-narrow-decision seam
// (WidenTopNToFleetWidth's caller).
// isProtected is the ADR-0074 control-plane scope predicate (guards.IsProtectedScope
// at composition roots; nil disables only the files-derived routing rule).
// Console-routed items are EXCLUDED at read time — batch-7 wave-0 starved
// ("planned zero lanes") because the raw top-N seeded exclusively console
// items the plan-time gate then rightly refused; skipping them here lets the
// seed backfill from the next dispatchable candidates instead.
```

### `go/internal/triagecap/wave_seed_amplified_test.go:11` — above `type c541ampInboxTodo struct {`

```text
// wave_seed_amplified_test.go — cycle-541 test-amplification lane for
// ReadInboxBacklog (extracted this cycle as the single inbox-JSON reader
// shared by SelectWaveSeedTopN and WidenTopNToFleetWidth's caller) and its
// SelectWaveSeedTopN regression path. Authored black-box from the exported
// godoc contract (filename-order tie-break; unreadable/malformed files and
// empty-id todos skipped, best-effort) plus the on-disk .evolve/inbox/*.json
// schema (id/weight/files) — not from wave_seed.go's implementation.
```

### `go/internal/triagecap/wave_seed_consoleroute_test.go:3` — above `import (`

```text
// wave_seed_consoleroute_test.go — batch-7 wave-0 pin: the inbox seed must
// skip console-routed items and backfill from the next dispatchable
// candidates. The raw top-N seeded exclusively console items, the plan-time
// gate rightly refused them all, and the wave "planned zero lanes" —
// starvation by correct refusal.
```

### `go/internal/triagecap/window.go:9` — above `const seedK = 5`

```text
// seedK is the throughput assumed before any observation: the cycle-281 PASS
// baseline (~5 floors per builder turn, the only coverage-campaign cycle that
// shipped at full scope).
```

### `go/internal/triagecap/window.go:17` — above `func K(window []core.TriageThroughputEntry) int {`

```text
// K is the observed builder throughput estimate: the rounded (half-up) mean
// of floors passed per cycle over the window. An empty window yields the
// cycle-281 seed; a degenerate mean is clamped to 1 so the cap never
// collapses to reject-everything.
```

### `go/internal/triagecap/window_test.go:10` — above `func TestK_EmptyWindowSeedsCycle281Baseline(t *testing.T) {`

```text
// TestK_EmptyWindowSeedsCycle281Baseline: with no observed history the
// throughput estimate is the cycle-281 PASS baseline (~5 floors/turn).
```
