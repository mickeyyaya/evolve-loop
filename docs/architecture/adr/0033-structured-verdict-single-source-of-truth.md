# ADR-0033: Structured Verdicts from a Single Source of Truth (kill classifier↔template drift)

**Status:** Proposed | **Date:** 2026-06-03 | **PR:** _pending_ | **Supersedes:** N/A | **Builds on:** the existing `phasespec.ClassifyRules` (`phasespec.go:37`) and `core.VerdictReason`/`Taxonomy` (`verdict.go`)

---

## Context

Every phase derives its verdict by **grepping a prose section heading out of an LLM-authored markdown report**. Six classifiers do this by hand:

| Phase | Greps for | Site |
|---|---|---|
| build | `## Changes` / `## Files Changed` / `## Files Modified` | `build.go:80` |
| scout | `## (Proposed\|Selected) Tasks` + a list item | `scout.go:35` |
| tdd | `## Acceptance`/`## AC-Materialization`/`## Coverage Map` + RED variants | `tdd.go:90` |
| audit | `## Verdict` + `PASS\|FAIL\|WARN\|SKIPPED` | `audit.go:46` |
| intent | `goal:` + `acceptance_checks:` | `intent.go:82` |
| triage | `## top_n` + a list item | `triage.go:30` |

This is a brittle producer→consumer contract: the **producer** is an LLM following a markdown template that humans edit; the **consumer** is a Go regex pinned to a heading string. When a template heading is edited, the classifier silently false-FAILs a valid report. In cycle-192 this cascaded — build's spurious FAIL contradicted its own `Status: PASS`, tripping the adversarial auditor's report-vs-telemetry cross-check → audit FAIL → **no cycle shipped**. The build changed-files heading has already drifted twice (`## Files Modified → ## Files Changed → ## Changes`).

The `64b2d95` fix patched 3 of the 6 by widening each regex into a tolerant allow-list. That does not address the class: `audit`/`intent`/`triage` carry the identical latent drift, the allow-lists accumulate stale variants, tolerant matching weakens precision, and **no contract test exists to catch the next drift** — the `64b2d95` message recommended a golden-fixture test that was never built.

Two enabling mechanisms already landed but are not used by these phases:

- **`phasespec.ClassifyRules{RequireSections, VerdictOnPass}`** — documented as *"the declarative verdict spec — replaces per-phase Go Classify"*; `specrunner.evaluateClassify` runs it (`specrunner.go:100`).
- **`core.VerdictReason{Status, Summary, Taxonomy}`** (smart-advisor framework) — structures the verdict *output*, but `ReasonFromDiagnostics` consumes the prose-grep classifiers' Diagnostics (`verdict.go:48`). It enriches the output of the brittle front door without fixing the input.

Full failure-class analysis: [knowledge-base/research/verdict-and-gate-proxy-failure-class-2026-06-03.md](../../../knowledge-base/research/verdict-and-gate-proxy-failure-class-2026-06-03.md).

## Decision

**Make the phase verdict read a single source of truth shared with the agent template, and prove it with a golden-fixture contract test per phase.** Three parts:

### 1. One declarative spec per phase, shared by producer and consumer

Each phase's required-sections live in **one place** — the phase's `ClassifyRules` (in registry/phase JSON, or a phase-owned spec constant). The agent prompt template renders its "Required sections" checklist **from that same spec** rather than hard-coding headings in the markdown. The classifier evaluates `ClassifyRules.RequireSections` against the report. A template author who renames a section edits the spec once; producer and consumer move together by construction.

Where a section heading is a true contract (not cosmetic prose), prefer a **machine-readable verdict token the agent owns** — e.g. a single `evolve-verdict: PASS` sentinel line or a `<phase>-verdict.json` sidecar — so the classifier reads a *field*, not a heading. Human-facing headings then change freely.

### 2. Migrate the 6 hand-rolled classifiers onto the shared spec

`build`, `scout`, `tdd`, `audit`, `intent`, `triage` stop hand-rolling regex and route through the declarative path (`specrunner`-style evaluation, or a shared `classifyBySpec(artifact, rules)` helper). `core.VerdictReason`/`Taxonomy` continues to wrap the result — now fed by a spec-driven classifier instead of a brittle one. Bare-string `Verdict*` constants and wire types are unchanged (ledger / hash-chain / JSON stability preserved, exactly as `verdict.go` already requires).

### 3. Golden-fixture contract test per phase (the drift alarm)

For each phase, check in a real recent report (PASS, legacy-PASS, and incomplete-FAIL fixtures) and assert the classifier's verdict. The test fails CI the moment a template/spec edit diverges from the fixtures — converting silent cycle-200-discovery into a red build at PR time. This is the alarm `64b2d95` recommended.

## Rollout

Per the repo's standard shadow → advisory → enforce discipline, and one phase at a time (build/scout/tdd first — already touched by `64b2d95`; then audit/intent/triage):

1. **Shadow:** spec-driven classifier computes alongside the legacy regex; log on disagreement; legacy still authoritative. Golden tests land here.
2. **Advisory:** spec-driven classifier authoritative; legacy regex retained as a fallback that WARNs on use.
3. **Enforce:** legacy regex deleted; the tolerant allow-lists from `64b2d95` go with it.

No env flag sprawl — reuse an existing rollout dial or a single `EVOLVE_VERDICT_SPEC_STAGE`. (See [[feedback_no_feature_flag_sprawl]].)

## Consequences

**Positive**
- The classifier-drift class is closed for all 6 phases, not 3 — the next template edit cannot silently false-FAIL.
- Completes the smart-advisor structured-verdict direction by fixing its *input*.
- Removes the accumulating tolerant-heading allow-lists.
- The golden tests document the producer↔consumer contract executably.

**Negative / risks**
- Migration touches 6 phases + their templates; must be staged behind shadow-mode and golden tests to avoid trading one verdict bug for another.
- A machine-readable sentinel requires the agent to emit it reliably; mitigated by keeping `RequireSections` as the floor and the sentinel as the precise signal.
- One-time effort to author fixtures from real reports (cycle-192/195 reports are available).

## Alternatives considered

- **Keep widening allow-lists (status quo, `64b2d95`).** Rejected: O(drifts) maintenance, no alarm, weakens precision, leaves 3 phases unpatched.
- **Structure only the output (`verdict.go` as-is).** Rejected as sufficient: enriches a verdict whose *input* is still a prose grep — the cycle-192 failure happens before `VerdictReason` is constructed.
- **Force every agent to emit JSON only (no markdown report).** Rejected as too broad now: the markdown reports are human-debugging artifacts; a verdict sentinel/sidecar gets the determinism without losing them.

## Implementation note (as-built)

Move 1 was implemented as a **new `internal/phasecontract` package that centralizes
the heading STRINGS** (shared by the 6 classifiers and the `TestProducersDeclareCanonical`
drift alarm), **not** as a migration of the classifiers onto `phasespec.ClassifyRules`.
Rationale: `ClassifyRules.RequireSections` is a flat AND-of-substrings and cannot
express build's OR-of-headings, scout/triage's heading-plus-≥1-item, tdd's
OR-within-AND, or audit's verdict-token extraction; forcing those onto the
declarative schema would have required extending it (OR-groups, item-count
predicates) — a larger change than the drift problem warranted. The stable matching
*logic* stays in Go; only the drift-prone heading *strings* are centralized. This
leaves `phasespec.ClassifyRules` as the declarative path for user/registry phases
and `phasecontract` for the 6 built-ins — a deliberate split, not unfinished work.
A future unification (extend `ClassifyRules` to subsume `phasecontract`) is possible
but not required.

## Related

- Failure-class note: [verdict-and-gate-proxy-failure-class-2026-06-03.md](../../../knowledge-base/research/verdict-and-gate-proxy-failure-class-2026-06-03.md)
- Instance diagnosis: [verdict-classifier-template-drift-2026-06-02.md](../../../knowledge-base/research/verdict-classifier-template-drift-2026-06-02.md)
- Mechanisms: `go/internal/phasespec/phasespec.go` (`ClassifyRules`), `go/internal/core/verdict.go` (`VerdictReason`), `go/internal/phases/specrunner/specrunner.go`
