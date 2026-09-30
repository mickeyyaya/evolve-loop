# Comment history: `internal/prompts`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/prompts/apicover_named_test.go:9` — above `func seedAgentDir(t *testing.T, name, contents string) string {`

```text
// apicover_named_test.go — public-API coverage (ADR-0050 Phase 5). Names and
// exercises exported symbols apicover flagged uncovered in this package:
//   - func NewForProject (prompts.go)
//   - type Loader (prompts.go)
//   - type Prompt (prompts.go)
// Each test asserts a real contract (Rule 9), not a no-op reference.
```

### `go/internal/prompts/compact_marker_gate_test.go:3` — above `import (`

```text
// compact_marker_gate_test.go — RED contract for cycle-415 tasks (tdd/triage marker) and
// cycle-422 triage threshold raise (triage-prompt-reference-index-expansion).
//
// RED state (before builder):
//   - evolve-tdd-engineer.md has no ## Reference Index heading → 0 bytes stripped (want ≥1500)
//   - evolve-triage.md has no ## Reference Index heading → 0 bytes stripped (want ≥1200)
//   - evolve-triage.md's versioned-historical sections appear in stripped body (want: absent after strip)
//   - TestAlwaysOnPhaseDocsHaveCompactMarker fails for tdd-engineer + triage (no heading)
```

### `go/internal/prompts/compact_marker_gate_test.go:35` — above `if saved < 64 {`

```text
// Floor recalibrated 2026-08-10 (was 1500): the old floor required the full
// Predicate Quality Requirements section (REQUIRED reading) to sit below the
// strip marker, deleting it from every dispatched tdd prompt with only the
// above-marker summary anchors surviving (persona-strip lobotomy incident).
// The marker now sits at EOF.
```

### `go/internal/prompts/compact_marker_gate_test.go:51` — above `"MUST exercise the system under test",`

```text
// cycle-85 anti-degenerate-predicate safeguard: cycle 415 buried the full
// "Predicate Quality Requirements" section below the marker (compact mode
// strips it). The above-marker REQUIRED summary must survive compaction so
// the tdd-engineer never authors grep-only predicates blind to this rule.
```

### `go/internal/prompts/compact_marker_gate_test.go:79` — above `func TestTriageCompaction(t *testing.T) {`

```text
// TestTriageCompaction asserts that evolve-triage.md has a line-anchored ## Reference Index
// heading enabling ≥4200 bytes of compaction, required output sections and gate-bearing
// rules survive above the heading, and versioned-historical subsections are relocated below it.
// Cycle-415 RED: heading absent → 0 bytes stripped (0 < 1200).
// Cycle-422 RED: ~3209B currently stripped; 3209 < 4200 → FAIL (threshold raised).
```

### `go/internal/prompts/compact_marker_gate_test.go:96` — above `if saved < 64 {`

```text
// Floor recalibrated 2026-08-10 (was 4200): the old floor REQUIRED burying
// the inbox-ingestion and idempotency-skip-list instructions — operational
// directives misclassified as "versioned-historical" because their headings
// carry version tags — below the strip marker, deleting them from every
// dispatched triage prompt (docs/incidents/2026-08-10-persona-strip-lobotomy.md;
// the queue-starvation mechanism). The marker now sits at EOF; what may be
// stripped is governed by phasecoherence/persona_strip_operational_test.go.
```

### `go/internal/prompts/compaction_coverage_test.go:9` — above `func TestAllPerCycleAgentsStrictlyCompact(t *testing.T) {`

```text
// compaction_coverage_test.go — RED contract for cycle-416 task prompt-compaction-coverage-gate.
//
// Regression guard: every per-cycle phase agent must have a ## Reference Index heading so
// StripOnDemandSections fires on every cycle dispatch. Fails loudly naming any agent whose
// marker is missing or whose body is not strictly shortened.
//
// RED state (before builder): evolve-intent.md has no ## Reference Index heading;
// StripOnDemandSections returns its body unchanged — len(stripped) == len(body) → FAIL for intent.
```

### `go/internal/prompts/cycle415_amplified_test.go:3` — above `import (`

```text
// cycle415_amplified_test.go — Adversarial amplification for cycle-415 tasks.
//
// Probes gaps NOT covered by:
//   strip_test.go (8 synthetic cases for StripOnDemandSections),
//   strip_amplified_test.go (adversarial boundary cases + idempotent + no-heading-in-output),
//   compact_marker_gate_test.go (real-doc compaction + gate + inline-mention),
//   realdoc_strip_test.go (6 real docs mustStrip with minSave).
//
// New adversarial angles:
//   - Stripped body minimum floor (prevents over-stripping that deletes required instructions)
//   - Reference stub file existence (cycle-415 created evolve-tdd-engineer-reference.md)
//   - CRLF bare-heading detection gap (bodyHasCompactMarker trims \r; StripOnDemandSections may not)
//   - bodyHasCompactMarker edge forms (bare heading, trailing-space-only heading)
//   - Large synthetic body correctness with exact boundary check
//   - Newline termination preservation (off-by-one in line reconstruction)
//   - Additional tdd-engineer behavior-anchors not in TestTddEngineerCompaction
//   - Canonical always-on doc file existence (guards against silent renames/deletions)
```

### `go/internal/prompts/cycle415_amplified_test.go:68` — above `func TestTddEngineerReferenceStubExists(t *testing.T) {`

```text
// TestTddEngineerReferenceStubExists verifies the evolve-tdd-engineer-reference.md file
// created in cycle-415 is non-empty. An empty or missing stub breaks the Layer 3 on-demand
// lookup contract for operators who fetch it via the reference index.
```

### `go/internal/prompts/cycle415_amplified_test.go:178` — above `func TestTddEngineerAdditionalAnchors(t *testing.T) {`

```text
// TestTddEngineerAdditionalAnchors verifies structural section anchors beyond the 4 phrases
// in TestTddEngineerCompaction. Confirmed above the marker by the cycle-415 build-report
// "Behavior Anchor Verification" section.
```

### `go/internal/prompts/cycle415_amplified_test.go:193` — above `for _, anchor := range []string{`

```text
// Section names confirmed present above the marker by the cycle-415 build-report.
```

### `go/internal/prompts/cycle416_amplified_test.go:3` — above `import (`

```text
// cycle416_amplified_test.go — Adversarial amplification for cycle-416 tasks.
//
// Probes gaps NOT covered by:
//   intent_compaction_test.go (4 tests: ≥500B, 5-anchor check, ## Composition absent, synthetic anti-gaming),
//   compaction_coverage_test.go (2 tests: all 7 agents strict-decrease, markerless unchanged),
//   compact_marker_gate_test.go (bodyHasCompactMarker gate for 6 agents, excluding intent).
//
// New adversarial angles:
//   - evolve-intent.md stripped body floor (≥5000B prevents over-stripping the behavior-bearing head)
//   - Reflection Authoring anchor in stripped intent body (eval spec mandates it; not in the 5-anchor list)
//   - Ask-when-Needed (AwN) classifier in stripped intent body (eval spec mandates; no Go test covers it)
//   - ## Reference section absent after strip (build-report moved BOTH ## Composition and ## Reference below marker)
//   - bodyHasCompactMarker gate applied to intent (TestAlwaysOnPhaseDocsHaveCompactMarker covers 6 agents not 7)
//   - Tighter byte-savings bound ≥600B (build-report states ~650B; adversarially tighter than existing ≥500B)
//   - Real-doc idempotency: strip applied twice to real evolve-intent.md (synthetic idempotency exists; real-doc does not)
//   - evolve-intent.md file existence guard (TestAlwaysOnDocFilesExist covers 6 agents; intent is absent)
```

### `go/internal/prompts/cycle416_amplified_test.go:48` — above `func TestIntentCompaction_ReflectionAuthoringNotDeleted(t *testing.T) {`

```text
// TestIntentCompaction_ReflectionAuthoringNotDeleted asserts that the Reflection Authoring
// section still exists in the raw evolve-intent.md body (relocated, not deleted).
//
// Cycle-416 placed ## Reflection Authoring (v10.20.0+) above the marker.
// Cycle-422 intentionally moves it BELOW the marker as on-demand reference — so checking
// for it in the stripped body would fail correctly. This test guards the complementary
// invariant: the section must still exist in the full document (relocated, not removed).
```

### `go/internal/prompts/cycle416_amplified_test.go:117` — above `func TestIntentHasCompactMarkerViaGate(t *testing.T) {`

```text
// TestIntentHasCompactMarkerViaGate asserts that evolve-intent.md's body is recognized by
// bodyHasCompactMarker — the same gate function used in TestAlwaysOnPhaseDocsHaveCompactMarker.
//
// Gap: TestAlwaysOnPhaseDocsHaveCompactMarker covers 6 per-cycle agents (scout, builder,
// auditor, orchestrator, tdd-engineer, triage) but NOT evolve-intent, which was added in
// cycle-416. TestAllPerCycleAgentsStrictlyCompact covers intent via length-decrease, not
// via the bodyHasCompactMarker gate function itself.
```

### `go/internal/prompts/cycle417_amplified_test.go:3` — above `import (`

```text
// cycle417_amplified_test.go — Adversarial amplification for cycle-417 tasks.
//
// Probes gaps NOT covered by:
//   router_compaction_test.go (3 tests: <8000B, 66 rows, no empty triggers),
//   reflector_compaction_test.go (6 tests: marker present, strip saves ≥200B,
//     operational anchors above marker, narrative absent after strip, reference stub
//     exists, synthetic anti-gaming).
//
// New adversarial angles:
//   Router:
//     - Section byte floor (≥4000B): prevents gaming via row over-deletion
//     - No duplicate phase names: catches row merging or silent deduplication
//     - Trigger minimum length (≥10 chars): catches over-trimming to meaningless stubs
//   Reflector:
//     - Stripped body floor (≥3000B): parallel to triage/tdd-engineer floor guards
//     - Real-doc idempotency: strip twice == strip once on the live file
//     - bodyHasCompactMarker gate recognizes reflector (canonical gate, not custom helper)
//     - Reference stub carries the narrative heading (stronger than size-only guard)
//     - Marker position: marker must appear AFTER "## What NOT to do" (position ordering)
```

### `go/internal/prompts/cycle417_amplified_test.go:149` — above `func TestReflectorStrippedBodyFloor(t *testing.T) {`

```text
// TestReflectorStrippedBodyFloor asserts that the stripped evolve-reflector.md body
// retains at least 3,000 bytes — guarding against a misplaced ## Reference Index marker
// that would strip required operational content (Workflow, Ledger Entry, Core Principles).
// Parallel to TestTddEngineerStrippedBodyFloor and TestTriageStrippedBodyFloor (cycle-415).
```

### `go/internal/prompts/cycle417_amplified_test.go:172` — above `func TestReflectorCompaction_RealDocIdempotent(t *testing.T) {`

```text
// TestReflectorCompaction_RealDocIdempotent asserts that applying StripOnDemandSections
// twice to the real evolve-reflector.md body produces the same result as once.
// Distinct from TestReflectorCompaction_SyntheticBuriedNarrativeNegative (synthetic body);
// exercises the real "## Reference Index (Layer 3, on-demand)" heading in the live file.
// Parallel to TestIntentCompaction_RealDocIdempotent (cycle-416).
```

### `go/internal/prompts/cycle420_amplified_test.go:3` — above `import (`

```text
// cycle420_amplified_test.go — Adversarial amplification for cycle-420 task T2.
//
// Probes gaps NOT covered by router_persona_test.go (AC1–AC5):
//
//   - GENERATED block markers: <!-- GENERATED:goal-recipes BEGIN/END --> must survive TSC
//     (build-report: "NOT touched" — no regression guard existed before this test).
//   - Section headings: ## Your job, ## Output contract, ## Goal-Type Recipes must be present
//     after TSC (headings are the structural skeleton, distinct from prose content).
//   - Prose floor: prose region must be >2500 bytes (prevents over-deletion gaming that
//     preserves domain tokens while stripping all decision logic context).
//   - Extended vocab — run: field: domain token "run:" preserved (build-report vocab list
//     item not covered by the 4-token TestRouterPersona_DomainVocabPreserved test).
```

### `go/internal/prompts/cycle420_amplified_test.go:66` — above `func TestRouterPersona_ProseFloor(t *testing.T) {`

```text
// TestRouterPersona_ProseFloor asserts that the prose region of agents/evolve-router.md
// (from end of frontmatter to ## Phase Catalog — Core Values, EXCLUDING the
// generated goal-recipes table — see routerProseBytes) is at least 1200 bytes.
//
// Amplification angle: AC2 asserts an upper bound (<5243 bytes, ≥15% reduction).
// Without a floor, a builder could game the byte limit by stripping all prose content
// except domain tokens, passing AC2 while destroying all decision-routing context.
// 1200 bytes ≈ 50% of the 2349-byte prose-only region measured on main at the
// 2026-09-09 re-baseline (ADR-0099; before that the region included the table
// and the floor was 2500 ≈ 48% of 5235) — a generous floor that
// catches catastrophic over-deletion without constraining legitimate future compression.
```

### `go/internal/prompts/cycle421_amplified_test.go:3` — above `import (`

```text
// cycle421_amplified_test.go — Adversarial amplification for cycle-421 tasks.
//
// Probes gaps NOT covered by predicates_test.go (C421_001–C421_010) or
// retro_compaction_test.go:
//
//   - Retro doc head floor (≥8000B remains after strip) — C421_001 verifies ≥1500B SAVED
//     but no floor exists on what REMAINS; this is the complementary anti-over-strip guard.
//   - Strip idempotency on real retro doc — cycle-416 added idempotency for intent; retro uncovered.
//   - Strip idempotency on real orchestrator doc — existing tests verify savings/anchors, not stability.
//   - Versioned sections NOT deleted from retro doc — C421_003 verifies sections absent from STRIPPED
//     body; this verifies they still exist in the full raw body (moved, not removed).
//   - On-demand sections NOT deleted from orchestrator doc — same relocation-vs-deletion guard.
//   - Retro doc has line-anchored ## Reference Index marker — compaction_coverage_test.go covers
//     the 7 always-on agents but NOT evolve-retrospective (conditional phase).
//   - Orchestrator byte-savings floor tied to cycle-421 spec (>=2000B vs realdoc's pre-cycle 512B).
```

### `go/internal/prompts/cycle421_amplified_test.go:26` — above `func TestRetroCompaction_HeadFloor_Amplified(t *testing.T) {`

```text
// TestRetroCompaction_HeadFloor_Amplified asserts that the stripped evolve-retrospective.md
// body retains at least 8000 bytes after StripOnDemandSections.
//
// Amplification angle: C421_001 verifies >=1500B SAVED (upper bound on stripping).
// This test adds a complementary FLOOR on what REMAINS, modeled after C421_008's 9000B
// guard for the orchestrator. The retro body before cycle-421 was ~11732B; after stripping
// ~1970B the head should be ~9762B — well above the 8000B floor.
```

### `go/internal/prompts/cycle421_amplified_test.go:50` — above `func TestRetroCompaction_Idempotent(t *testing.T) {`

```text
// TestRetroCompaction_Idempotent asserts that applying StripOnDemandSections twice to
// the real evolve-retrospective.md body produces the same result as applying it once.
//
// Idempotency is the key safety property: once the on-demand tail is stripped, the
// remaining head no longer contains the ## Reference Index marker, so a second strip
// must be a complete no-op — not corrupt or truncate the head content.
//
// Amplification angle: cycle-416 adds real-doc idempotency for evolve-intent;
// no equivalent guard exists for evolve-retrospective.
```

### `go/internal/prompts/cycle421_amplified_test.go:76` — above `func TestOrchestratorCompaction_Idempotent(t *testing.T) {`

```text
// TestOrchestratorCompaction_Idempotent asserts that applying StripOnDemandSections twice
// to the real evolve-orchestrator.md body produces the same result as applying it once.
//
// Amplification angle: cycle-421 moved >=2965B of sections below the marker in orchestrator.
// Idempotency guards against a scenario where the moved sections contain another
// ## Reference Index line that would cause a second strip to further truncate the tail.
```

### `go/internal/prompts/cycle421_amplified_test.go:184` — above `func TestOrchestratorCompaction_ByteSavingsFloorAmplified(t *testing.T) {`

```text
// TestOrchestratorCompaction_ByteSavingsFloorAmplified asserts that StripOnDemandSections
// applied to the real evolve-orchestrator.md saves >=2000 bytes — the cycle-421 floor.
//
// Amplification angle: realdoc_strip_test.go (written before cycle-421) uses 512B as the
// orchestrator floor; C421_007 raises it to 2000B. This test explicitly names the 2000B
// floor in the prompts-package test suite so any future edit that reduces the on-demand
// tail back below 2000B is caught here as well as in the ACS suite.
```

### `go/internal/prompts/cycle422_amplified_test.go:3` — above `import (`

```text
// cycle422_amplified_test.go — Adversarial amplification for cycle-422 tasks.
//
// Probes gaps NOT covered by predicates_test.go (C422_001–C422_010) or the
// direct test files updated for cycle-422:
//
//   - Intent head floor (≥7000B remains after strip) — C422_001 verifies ≥2200B SAVED
//     but has no floor on what REMAINS; complementary anti-over-strip guard.
//   - Intent idempotency on real doc — cycle-416 added real-doc idempotency; re-verified
//     after cycle-422 expands the below-marker tail.
//   - Intent on-demand sections not deleted — C422_002/003/004 verify sections absent from
//     STRIPPED body; this verifies they still exist in the raw body (relocated, not removed).
//   - Triage byte-savings floor ≥4200B in prompts package (mirrors C422_007 in ACS suite).
//   - Triage head floor (≥6000B remains after strip) — anti-over-strip guard for triage.
//   - Triage idempotency on real doc.
//   - Triage on-demand section not deleted — bash example must exist in raw body after relocation.
```

### `go/internal/prompts/cycle422_amplified_test.go:49` — above `func TestIntentCompaction_RealDocIdempotent_PostCycle422(t *testing.T) {`

```text
// TestIntentCompaction_RealDocIdempotent_PostCycle422 asserts that applying
// StripOnDemandSections twice to the real evolve-intent.md body (after cycle-422 expansion)
// produces the same result as applying it once.
//
// Amplification angle: cycle-416 added real-doc idempotency. Cycle-422 expands the
// below-marker tail by ~1500B; this re-verifies the moved content contains no nested
// ## Reference Index heading that would cause a second strip to further truncate.
```

### `go/internal/prompts/cycle422_amplified_test.go:99` — above `func TestTriageCompaction_ByteSavings4200_Amplified(t *testing.T) {`

```text
// TestTriageCompaction_ByteSavings4200_Amplified asserts that StripOnDemandSections applied
// to the real evolve-triage.md body saves ≥4200 bytes — the cycle-422 floor.
//
// Amplification angle: compact_marker_gate_test.go raises the triage threshold to ≥4200B
// for the ACS-gated test; this mirrors it in the always-on CI prompts package.
```

### `go/internal/prompts/cycle422_amplified_test.go:116` — above `const minSaved = 64`

```text
// Recalibrated 2026-08-10 (was 4200): the cycle-422 floor required the
// inbox-ingestion + idempotency-skip-list instructions below the strip
// marker (persona-strip lobotomy incident — the queue-starvation half).
// The marker now sits at EOF; the keep-guard governs strippable content.
```

### `go/internal/prompts/intent_compaction_test.go:10` — above `func TestIntentCompaction_SavesAtLeast2200Bytes(t *testing.T) {`

```text
// intent_compaction_test.go — RED contract updated for cycle-422 task intent-prompt-reference-index-expansion.
//
// Cycle-416 original state: heading absent → 0 bytes stripped (want ≥500);
//   ## Composition and ## Reference appear in stripped body.
// Cycle-422 updated state:
//   - ## Output contract (INTENT_MODE), ## Re-run behavior, ## Reflection Authoring relocated below marker
//   - Byte floor raised from ≥500 to ≥2200 (from ~755B currently stripped)
//   - "Output contract" and "INTENT_MODE" removed from must-survive anchors (they move below marker);
//     "30-80 line" and "EMERGENCY EXIT" added (confirmed above marker)
```

### `go/internal/prompts/intent_compaction_test.go:20` — above `func TestIntentCompaction_SavesAtLeast2200Bytes(t *testing.T) {`

```text
// TestIntentCompaction_SavesAtLeast2200Bytes asserts that applying StripOnDemandSections to the
// real evolve-intent.md body saves ≥2200 bytes — the cycle-422 floor after relocating
// ## Output contract (INTENT_MODE), ## Re-run behavior, and ## Reflection Authoring below the marker.
// RED (cycle-422): currently only ~755B stripped; 755 < 2200 → FAIL.
```

### `go/internal/prompts/intent_compaction_test.go:36` — above `if saved < 256 {`

```text
// Floor recalibrated 2026-08-10 (was 2200): that floor DEMANDED burying the
// INTENT_MODE output contract and re-run behavior below the strip marker —
// deleting the phase's own output contract from every dispatched intent
// prompt, in direct contradiction of the (latent-red) C416 acs predicate
// that requires INTENT_MODE above the marker (persona-strip lobotomy
// incident). Tail = Composition + Reference only; the keep-guard
// phasecoherence/persona_strip_operational_test.go governs the rest.
```

### `go/internal/prompts/intent_compaction_test.go:48` — above `func TestIntentCompaction_OperationalAnchorsAboveMarker(t *testing.T) {`

```text
// TestIntentCompaction_OperationalAnchorsAboveMarker asserts that required every-cycle
// behavior-bearing anchors survive StripOnDemandSections (remain above the ## Reference Index
// marker) in evolve-intent.md.
// Pre-existing GREEN: all listed anchors are above the marker in current file.
// Regression guard: fires if builder accidentally buries any of these anchors below the marker.
// NOTE (cycle-422): "Output contract" and "INTENT_MODE" removed — those sections are
// intentionally relocated below the marker by the cycle-422 builder task. "30-80 line"
// and "EMERGENCY EXIT" added per cycle-422 AC5.
```

### `go/internal/prompts/intent_compaction_test.go:80` — above `func TestIntentCompaction_ReferenceContentAbsentAfterStrip_Negative(t *testing.T) {`

```text
// TestIntentCompaction_ReferenceContentAbsentAfterStrip_Negative asserts that reference-grade
// and on-demand sections are relocated BELOW the ## Reference Index marker and thus absent
// from the stripped body.
// Cycle-416: ## Composition (already below marker — pre-existing GREEN after cycle-416).
// Cycle-422 (RED): ## Output contract (INTENT_MODE), ## Re-run behavior, ## Reflection Authoring
// are still above the marker → their unique identifiers appear in stripped body → FAIL.
// GREEN after builder: sections relocated below marker → identifiers absent in stripped.
```

### `go/internal/prompts/intent_compaction_test.go:104` — above `for _, kept := range []string{`

```text
// INVERTED 2026-08-10 (persona-strip lobotomy incident): cycle-422 demanded
// these three be STRIPPED — deleting the phase's own OUTPUT CONTRACT
// (intent-delta.md is the delta-mode deliverable name; stripping it is a
// plausible cause of the intent-delta contract-path-skew defect), the
// re-run protocol, and the reflection sidecar duty from every dispatched
// prompt. Operational directives must SURVIVE stripping.
```

### `go/internal/prompts/persona_house_rules_test.go:3` — above `import (`

```text
// persona_house_rules_test.go — pins the two MANDATORY house rules that lived
// only in operator lore, so every cycle rediscovered them by failing:
//
//  1. apicover graduation (inbox acs-apicover-enrollment-in-builder-brief, 0.94)
//     — batch-21 HALTED at cycle-1218 because THREE lanes aborted on "new
//     internal package absent from go/.apicover-enforce"; console hit the same
//     class twice the same day on PR #372. The requirement appeared in NEITHER
//     the builder's nor the TDD engineer's brief.
//
//  2. caller proof (inbox builder-persona-requires-caller-proof, 0.90) — two
//     console implementer agents satisfied their stated contract exactly and
//     left integration unwired (a struct field with no consumer; a lint with no
//     caller). Same class in loop cycles: the P2 accounting seam wired only into
//     the sequential loop body was unreachable in fleet mode (#373), and
//     ADR-0074's typed routing shipped inert once.
//
// The assertions are deliberately phrase-level: an instruction the agent never
// reads is worthless, so each rule must be present AND must survive
// StripOnDemandSections (i.e. sit ABOVE the "## Reference Index" on-demand
// marker, which compaction truncates at).
```

### `go/internal/prompts/persona_stopcriterion_dedupe_test.go:12` — above `var personaFiles = []string{"evolve-scout.md", "evolve-builder.md", "evolve-auditor.md"}`

```text
// persona_stopcriterion_dedupe_test.go — cycle-646 Task 3
// (persona-stop-criterion-dedupe): agents/evolve-{scout,builder,auditor}.md
// each carry a structurally-identical "## STOP CRITERION" block (named
// completion gates + banned-post-report patterns) with zero shared wording —
// a token-size refactor, not a behavior change. Scope: extract the shared
// STRUCTURE into one reference doc; each persona file keeps only its
// phase-specific gate list + a pointer. Every existing gate name and banned
// pattern must survive verbatim somewhere under agents/ (the persona file
// itself or the new shared reference doc — the second test searches the
// whole agents/evolve-*.md corpus so it is agnostic to which file the text
// ends up in).
//
// RED today: nothing has been extracted — combined line count is the
// pre-dedupe baseline (751, measured this cycle: 202+275+274).
```

### `go/internal/prompts/persona_stopcriterion_dedupe_test.go:48` — above `const preDedupeBaseline = 751`

```text
// cycle-646 measured: evolve-scout.md(202) + evolve-builder.md(275) + evolve-auditor.md(274)
```

### `go/internal/prompts/prompts.go:1` — above `package prompts`

```text
// Package prompts loads agent and skill markdown with YAML frontmatter.
//
// The loader is fs.FS-backed so it can serve from three sources
// without API churn:
//
//  1. fstest.MapFS — unit tests
//  2. os.DirFS    — dev override at $EVOLVE_PROMPTS_DIR
//  3. embed.FS    — Phase 3 vendored copy of agents/ + skills/
//
// Plan §1 decision #13 wires the embed path; this Phase 2 layer
// commits to the fs.FS surface so the Phase 3 swap is one line at the
// orchestrator wire-up site.
//
// The frontmatter parser is intentionally minimal — it handles only
// the shapes observed in agents/*.md and skills/*/SKILL.md (flat
// key-value, inline bracketed arrays, quoted strings). Loading any of
// the existing 25 agent files must succeed; adding a full YAML
// dependency would inflate the binary for no gain.
```

### `go/internal/prompts/prompts.go:139` — above `return Prompt{}, fmt.Errorf("prompts: %w (%w)", fs.ErrNotExist, ErrNoSource)`

```text
// Both sentinels, deliberately: fs.ErrNotExist preserves the documented
// zero-loader contract (NewFromFS(nil): "every read returns
// fs.ErrNotExist"), while ErrNoSource lets callers distinguish a WIRING
// defect (misresolved prompts root — EVERY doc "missing") from one
// genuinely absent doc, so a nil loader can never masquerade as a
// skippable missing-persona (cycle-1551 class must stay narrow).
```

### `go/internal/prompts/realdoc_strip_test.go:35` — above `{"evolve-auditor", 256},`

```text
// Auditor floor recalibrated 2026-08-10: the old 4096 floor ("~70 % tail")
// fossilized a mid-file marker that stripped the verdict rules, STOP
// criterion, and MANDATORY disposition contract from every dispatched
// audit (cycles 1390-1429, 15/30 FAILs). The marker now sits at EOF; what
// may be stripped is governed by phasecoherence/persona_strip_operational_test.go.
```

### `go/internal/prompts/realdoc_strip_test.go:41` — above `{"evolve-builder", 256},`

```text
// Builder/scout/tdd/triage floors recalibrated 2026-08-10 alongside the
// auditor's (persona-strip lobotomy incident): the old floors fossilized
// operational tails (STOP CRITERION, POSTHOC, predicate-quality rules,
// inbox ingestion) below mid-file markers. Markers now sit at EOF; the
// keep-guard phasecoherence/persona_strip_operational_test.go governs
// what may be stripped.
```

### `go/internal/prompts/reflector_compaction_test.go:10` — above `func TestReflectorCompaction_MarkerPresent(t *testing.T) {`

```text
// reflector_compaction_test.go — RED contract for cycle-417 task reflector-reference-ondemand-split.
//
// RED state (before builder):
//   - evolve-reflector.md has no ## Reference Index heading → StripOnDemandSections returns body unchanged
//   - "## Why this agent exists" historical narrative (lines ~172–179) is inline (not below marker)
//   - evolve-reflector-reference.md does not exist
```

### `go/internal/prompts/reflector_compaction_test.go:118` — above `func TestReflectorReferenceStubExists(t *testing.T) {`

```text
// TestReflectorReferenceStubExists verifies that agents/evolve-reflector-reference.md
// exists and is non-empty. The stub must carry the "## Why this agent exists" narrative
// relocated from evolve-reflector.md, making it available for on-demand Layer 3 lookup.
// RED: file does not exist yet (written by builder as part of cycle-417).
```

### `go/internal/prompts/router_compaction_test.go:10` — above `func TestRouterCompaction_CoreValuesSectionUnder8000Bytes(t *testing.T) {`

```text
// router_compaction_test.go — RED contract for cycle-417 task router-catalog-prose-compaction.
//
// RED state (before builder):
//   - evolve-router.md "## Phase Catalog — Core Values" section is ~10507B (want <8000B)
//   - StripOnDemandSections is NOT used for the router (catalog is the working menu);
//     compaction is in-place prose trimming, not marker-based removal.
```

### `go/internal/prompts/router_persona_test.go:12` — above `func routerContent(t *testing.T) (raw []byte, body string) {`

```text
// router_persona_test.go — RED contract for cycle-420 task router-persona-tsc-compress.
//
// RED state (before builder):
//   - evolve-router.md has no "<!-- TSC applied" marker (TSC=0)
//   - prose region (frontmatter-end → "## Phase Catalog — Core Values") is 6169 bytes
//     (want < 5243, i.e. ≥15% reduction)
//   - catalog section (7988 bytes) must remain byte-identical
```

### `go/internal/prompts/router_persona_test.go:33` — above `func routerProseBytes(t *testing.T, body string) int {`

```text
// routerProseBytes returns the byte length of the PROSE region in
// evolve-router.md: from the end of the YAML frontmatter block to (not
// including) the "## Phase Catalog — Core Values" heading, MINUS the generated
// goal-recipes table between the GENERATED markers. The table is a projection
// of phase-registry.json:config.goal_recipes (locked by
// router.TestRouterPersonaRecipeTable_NoDrift), so it grows with the catalog by
// design and is not prose TSC governs — counting it made the pin fail on the
// first new recipe row (ADR-0099, 2026-09-09: main sat 4 bytes under the cap).
```

### `go/internal/prompts/router_persona_test.go:100` — above `func TestRouterPersona_ProseRegionByteReduction(t *testing.T) {`

```text
// TestRouterPersona_ProseRegionByteReduction asserts that the prose region of
// evolve-router.md (from end of frontmatter to the Phase Catalog heading,
// excluding the generated goal-recipes table) stays strictly under the
// anti-bloat ceiling.
//
// AC2 — router-persona-tsc-compress. The original pin was <5243 bytes over a
// region that INCLUDED the generated table (≥15% below the 6169-byte pre-TSC
// baseline). Re-baselined 2026-09-09 (ADR-0099): the table is registry-projected
// config, so the pin now measures prose only — 2349 bytes on main at the
// re-baseline; the ceiling is a deliberate anti-bloat bound (~+28%) so a
// regrowth wave still fails here while a legitimate sentence and recipe rows
// never do (the old pin died at +4 bytes).
```

### `go/internal/prompts/router_persona_test.go:115` — above `const baselineBytes = 2349`

```text
// prose-only size on main, 2026-09-09
```

### `go/internal/prompts/strip_amplified_test.go:8` — above `func TestStripOnDemandSections_Adversarial(t *testing.T) {`

```text
// strip_amplified_test.go — Adversarial amplification for StripOnDemandSections.
//
// Probes boundary conditions of the line-anchored prefix-match rule (cycle-413 Task A).
// Distinct from strip_test.go's 8 cases; targets failure modes a naive strings.Contains
// or wrong-space-delimiter implementation would exhibit.
```

### `go/internal/prompts/strip_test.go:5` — above `func TestStripOnDemandSections(t *testing.T) {`

```text
// strip_test.go — RED contract for cycle-256 task `prompt-ondemand-section-strip`.
//
// Agent docs carry a static "## Reference Index" tail (lookup tables, on-demand
// links) that is identical across cycles and re-sent on every dispatch.
// StripOnDemandSections removes that section (heading through EOF) so a compact
// prompt mode can drop the dead weight; a body WITHOUT the heading is returned
// byte-for-byte unchanged. The heading match is LINE-ANCHORED — an inline prose
// mention of "## Reference Index" must NOT trigger a strip (the anti-naive-
// substring guard; a bare strings.Index impl fails the "inline mention" case).
```
