---
score_cap:
  - criterion: "ClassifyBadVerdict recognizes a fenced-JSON-wrapped verdict as recoverable"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -run TestC1389_001_ClassifyBadVerdict_FencedJSON_Recoverable ./acs/cycle1389"
  - criterion: "ClassifyBadVerdict recognizes a trailing-comma sentinel payload as recoverable"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -run TestC1389_002_ClassifyBadVerdict_TrailingComma_Recoverable ./acs/cycle1389"
  - criterion: "ClassifyBadVerdict recognizes a bare/displaced (unwrapped) verdict object as recoverable"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -run TestC1389_003_ClassifyBadVerdict_Displaced_Recoverable ./acs/cycle1389"
  - criterion: "ClassifyBadVerdict returns Recoverable=false for a genuinely absent verdict (the negative control)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -run TestC1389_004_ClassifyBadVerdict_GenuinelyAbsent_NotRecoverable ./acs/cycle1389"
  - criterion: "the new classifier makes zero change to existing Result.OK/Violations semantics (full package regression)"
    max_if_missing: 9
    evidence: "cd go && go test ./internal/deliverable"
---

# Eval: Instrument bad_verdict classification (recoverable-malformed detector, log-only)

> Pins the public-API contract of `deliverable.ClassifyBadVerdict` introduced in
> cycle 1389 for the `schema-aligned-salvage-layer` inbox item. Per the item's
> own text and docs/research/deliverable-alignment-2026-08/README.md §3.3
> (SAP: "lenient, LOGGED, bounded coercion... never invents values"), this
> FIRST deliverable is instrumentation only — a pure, observability-only
> classifier over `Result.Content` (the single-read seam) that recognizes
> three recoverable-malformed verdict shapes (fenced-JSON, trailing-comma,
> displaced/bare-JSON) vs. a genuinely absent verdict, with ZERO mutation of
> the existing well-formedness verdict (`Result.OK`/`Violations`). The
> negative-control criterion (absent verdict ⇒ Recoverable=false) is the
> highest-leverage anti-no-op signal (SKILL adversarial-testing §6): a
> classifier that marks everything recoverable would pass the other three
> criteria vacuously.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| fenced-json-positive | Fenced JSON verdict classified recoverable | 6/10 | `TestC1389_001_...` |
| trailing-comma-positive | Trailing-comma sentinel classified recoverable | 6/10 | `TestC1389_002_...` |
| displaced-positive | Bare/displaced verdict object classified recoverable | 6/10 | `TestC1389_003_...` |
| absent-negative | Genuinely absent verdict classified NOT recoverable | 8/10 | `TestC1389_004_...` |
| zero-mutation-regression | Existing deliverable package tests stay green | 9/10 | `go test ./internal/deliverable` |
