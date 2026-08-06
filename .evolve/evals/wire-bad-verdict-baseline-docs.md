---
score_cap:
  - criterion: "the classifier is invoked from the real production caller (deliverable.Reviewer.Review, the host contract gate) on every bad_verdict, not left as a dead helper"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -run TestC1389_006_ReviewerWiring_LogsClassificationOnBadVerdict ./acs/cycle1389"
  - criterion: "instrumentation is observability-only — Reviewer.Review's block/approve decision is byte-identical to pre-instrumentation behavior"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -run TestC1389_006_ReviewerWiring_LogsClassificationOnBadVerdict ./acs/cycle1389"
  - criterion: "README.md gains a structural §7 baseline section naming the wiring call site and the baseline sidecar file"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -run TestC1389_007_ReadmeGainsBaselineSectionWithWiringExcerpt ./acs/cycle1389"
---

# Eval: Wire per-CLI bad_verdict baseline into audit report + docs

> Pins the wiring half of cycle 1389's `schema-aligned-salvage-layer` work:
> the classifier from `instrument-bad-verdict-classification` must actually be
> reached from `Reviewer.Review` (reviewer.go:102, the seam
> `core.DeliverableReviewer` wires behind the host contract gate) on every
> confirmed `bad_verdict`, appending one JSONL record to
> `<ProjectRoot>/.evolve/bad-verdict-baseline.jsonl` — reusing the existing
> `log.SidecarWriter`/`EmitAbnormal` pattern (go/internal/log/events.go) rather
> than inventing a new logging surface. This is the house-rule-2 reachability
> proof: a predicate that calls `ClassifyBadVerdict` directly proves nothing
> about production wiring, so the ACS predicate drives `Review` itself. The
> block/approve decision must stay byte-identical — this cycle is
> instrumentation-first, never a salvage/waiver mechanism (docs/research/
> deliverable-alignment-2026-08/README.md §3.3: "never invents values").
>
> The doc AC (README.md §7 with REAL, non-fabricated baseline counts pulled
> from a live run in this cycle's own worktree) is only PARTIALLY covered by
> a predicate: existence of the section, the wiring excerpt, and the sidecar
> filename are mechanically checkable and pinned here; whether the reported
> counts are genuinely measured vs. invented is not something any predicate
> can distinguish, and per the AC-Materialization Contract that half is
> dispositioned manual+checklist in test-report.md for the Auditor instead of
> being faked as a predicate here.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| wiring-reachability | Classifier reached from the real production caller | 7/10 | `TestC1389_006_...` |
| decision-unchanged | Block/approve decision unchanged by instrumentation | 9/10 | `TestC1389_006_...` |
| docs-structural | README §7 exists with wiring excerpt + sidecar filename | 5/10 | `TestC1389_007_...` |
