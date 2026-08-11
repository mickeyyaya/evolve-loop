---
score_cap:
  - criterion: "§6 of the deliverable-alignment README no longer summarises the recoverable-malformed rate as 'not yet instrumented', while §7 keeps its historical quote of that wording"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1437_001_Section6DropsStalePlaceholderWhileSection7KeepsHistory ./acs/cycle1437"
  - criterion: "§6 states the measured recoverable rate using the exact figures parsed from §7's committed measured table (no drift between the two sections)"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1437_002_Section6StatesMeasuredRateDerivedFromSection7 ./acs/cycle1437"
  - criterion: "§6's measured statement carries provenance — the evidence path §7 cites, and/or a §7 cross-reference — and never cites a different baseline file than §7"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1437_003_Section6CitesEvidenceByPathMatchingSection7 ./acs/cycle1437"
  - criterion: "§6.3 exists, follows §6.1's issue/gap/solution template (§3.8 convention), and points at a git-tracked, on-disk producing-code path plus §7"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1437_004_Section63FollowsTemplateAndCitesLiveCode ./acs/cycle1437"
  - criterion: "No count or percentage appears in §6's prose or §6.3 that §7's measured table does not license — figures are cited, never invented"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1437_005_Section6InventsNoCountsBeyondSection7 ./acs/cycle1437"
---

# Eval: salvage baseline — §6 must cite the measured rate, not the placeholder

> Pins the internal consistency of `docs/research/deliverable-alignment-2026-08/README.md`
> between §6 (the experience record's baseline summary) and §7 (the audited
> measured `bad_verdict` baseline landed in cycle-1389). §6 shipped a
> "not yet instrumented (the salvage layer's first deliverable is the
> *measurement*)" placeholder that §7 later contradicted outright, leaving the
> document asserting both that the rate was unknown and that it was 15/167
> (9.0%). Cycle-1434 bound the fix as top_n #1 and produced correct work, but
> FAILed on an unrelated false-RED gate — the ACS wrong-project-root bug fixed
> by #449 — so the placeholder survived into cycle-1437.
>
> The durable risk this eval guards is **not** "did someone edit the sentence"
> but **figure drift and figure invention**: a future edit to either section
> that leaves the two disagreeing, or that states a plausible-sounding rate no
> measurement supports. Every predicate therefore derives its expected values
> from §7's committed table at run time rather than hardcoding them — §6 can
> only go green by agreeing with the audited source of record, and §7 can only
> be restated by re-measuring. The anti-invention cap (8/10) is the highest
> because a fabricated baseline in a research writeup is a correctness failure
> of the document itself, not a formatting lapse.
>
> Source incident: cycle-1434 (blocked, `retrospective-report.md` H1); measured
> data from cycle-1389 (`.evolve/runs/cycle-1389/bad-verdict-baseline.jsonl`).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| stale-placeholder-scoped-removal | §6 drops the placeholder; §7 keeps the historical quote (a document-wide replace destroys provenance) | 7/10 | `go test -tags acs -run TestC1437_001 ./acs/cycle1437` |
| cross-section-figure-agreement | §6's figures are the ones parsed live from §7's table | 7/10 | `go test -tags acs -run TestC1437_002 ./acs/cycle1437` |
| evidence-provenance | §6 cites §7 and/or §7's own evidence path, never a divergent one | 6/10 | `go test -tags acs -run TestC1437_003 ./acs/cycle1437` |
| template-conformance-and-live-xref | §6.3 follows §6.1's derived issue/gap/solution markers; cited code path exists and is tracked | 5/10 | `go test -tags acs -run TestC1437_004 ./acs/cycle1437` |
| no-invented-counts | every ratio/percentage in §6 is licensed by §7 | 8/10 | `go test -tags acs -run TestC1437_005 ./acs/cycle1437` |
