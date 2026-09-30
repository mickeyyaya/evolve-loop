# Comment history: `acs/cycle422`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle422/predicates_test.go:3` — above `package cycle422`

```text
// Package cycle422 materializes the cycle-422 acceptance criteria for two prompt-compaction tasks:
//
//   - intent-prompt-reference-index-expansion (T1) — relocate ## Output contract (INTENT_MODE),
//     ## Re-run behavior, and ## Reflection Authoring below ## Reference Index in
//     evolve-intent.md so ≥2200B is stripped per intent dispatch (up from current ~755B).
//
//   - triage-prompt-reference-index-expansion (T2) — relocate additional on-demand reference
//     (step-3b predicate-graph bash example, and other verbose on-demand content) below
//     ## Reference Index in evolve-triage.md so ≥4200B is stripped (up from current ~3209B).
//
// AC map (1:1 with scout-report.md top_n; R9.3 floor-binding):
//
//	intent-prompt-reference-index-expansion (T1):
//	  AC1  evolve-intent.md saves ≥2200B after StripOnDemandSections               → C422_001 (RED)
//	  AC2  ## Output contract (INTENT_MODE) absent from stripped body (negative)   → C422_002 (RED)
//	  AC3  ## Re-run behavior absent from stripped body (negative)                 → C422_003 (RED)
//	  AC4  ## Reflection Authoring detail absent from stripped body (negative)     → C422_004 (RED)
//	  AC5  Required anchors survive above marker (regression guard)                → C422_005 (pre-existing GREEN)
//	  AC6  Synthetic buried-anchor negative (non-vacuity proof)                    → C422_006 (pre-existing GREEN)
//
//	triage-prompt-reference-index-expansion (T2):
//	  AC1  evolve-triage.md saves ≥4200B after StripOnDemandSections               → C422_007 (RED)
//	  AC2  Decision anchors survive above marker (regression guard)                → C422_008 (pre-existing GREEN)
//	  AC3  Synthetic buried-anchor negative (non-vacuity proof)                    → C422_009 (pre-existing GREEN)
//	  AC4  On-demand bash example absent from stripped body (negative)             → C422_010 (RED)
//
// Adversarial diversity (SKILL §6):
//
//	Negative: C422_002/003/004 (on-demand sections still above marker → fail "absent");
//	          C422_006/009 (synthetic buried-rule — StripOnDemandSections must remove it);
//	          C422_010 (bash example still above marker → fail "absent").
//	Edge/OOD: C422_005 (anchor guard: fires if builder over-relocates required anchors);
//	          C422_008 (decision-anchor guard: fires if builder relocates process-critical content).
//	Semantic:  10 distinct dimensions: byte-delta-intent / output-contract-absent /
//	           rerun-absent / reflection-absent / intent-anchor-survival / synthetic-intent /
//	           byte-delta-triage / triage-anchor-survival / synthetic-triage / bash-example-absent.
//
// 1:1 enforcement:
//
//	T1: predicate=6 (C422_001–C422_006), manual+checklist=0, unverifiable-remove=0 → total AC=6 ✓
//	T2: predicate=4 (C422_007–C422_010), manual+checklist=0, unverifiable-remove=0 → total AC=4 ✓
```

### `go/acs/cycle422/predicates_test.go:75` — above `if saved < 256 {`

```text
// Recalibrated 2026-08-10 (was 2200): that floor DEMANDED burying intent's
// own output contract, re-run protocol, and reflection duty below the
// strip marker (persona-strip lobotomy incident). Tail = Composition +
// Reference only; the phasecoherence keep-guard governs the rest.
```

### `go/acs/cycle422/predicates_test.go:102` — above `if !strings.Contains(stripped, "intent-delta.md") {`

```text
// INVERTED 2026-08-10 (persona-strip lobotomy incident): the original
// negative demanded the phase's own OUTPUT CONTRACT be stripped —
// intent-delta.md is the delta-mode deliverable name, and deleting it from
// dispatched prompts is a plausible cause of the intent-delta
// contract-path-skew defect. The contract must SURVIVE stripping.
```

### `go/acs/cycle422/predicates_test.go:129` — above `if !strings.Contains(stripped, "## Re-run behavior") {`

```text
// INVERTED 2026-08-10 (persona-strip lobotomy incident): the original
// negative demanded this section be STRIPPED — but re-run behavior is the
// operational protocol for every priorIntent re-dispatch; deleting it from
// the prompt left re-runs blind. Operational directives survive stripping.
```

### `go/acs/cycle422/predicates_test.go:138` — above `func TestC422_004_IntentReflectionAuthoringAbsentAfterStrip_Negative(t *testing.T) {`

```text
// TestC422_004_IntentReflectionAuthoringAbsentAfterStrip_Negative asserts that the
// ## Reflection Authoring (v10.20.0+) detail is relocated BELOW the ## Reference Index
// marker (absent from the stripped body) in evolve-intent.md.
// RED: ## Reflection Authoring (v10.20.0+) is currently ABOVE line-203 marker → its
// sidecar reference (intent-reflection.yaml) present in stripped body → FAIL.
// GREEN after Builder: section moved below marker → intent-reflection.yaml absent.
```

### `go/acs/cycle422/predicates_test.go:155` — above `if !strings.Contains(stripped, "intent-reflection.yaml") {`

```text
// INVERTED 2026-08-10 (persona-strip lobotomy incident): the original
// negative demanded the Reflection Authoring section be STRIPPED — but it
// instructs emitting the intent-reflection.yaml sidecar on EVERY run;
// stripping it silently killed the sidecar. Operational directives survive.
```

### `go/acs/cycle422/predicates_test.go:229` — above `if saved < 64 {`

```text
// Recalibrated 2026-08-10 (was 4200): that floor REQUIRED burying the
// inbox-ingestion + idempotency-skip-list instructions below the strip
// marker — deleting them from every dispatched triage prompt
// (docs/incidents/2026-08-10-persona-strip-lobotomy.md, queue-starvation
// half). Marker now at EOF; the phasecoherence keep-guard governs
// strippable content. The predicate keeps asserting the marker EXISTS.
```
