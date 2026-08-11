---
score_cap:
  - criterion: "The cycle-1298 quoted-decoy corpus alone never classifies as Recoverable"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1407_005 ./acs/cycle1407"
  - criterion: "ClassifyBadVerdict classifies from the report's OWN tail sentinel, not from the first quoted decoy echoed in prose"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1407_006 ./acs/cycle1407"
  - criterion: "A decoy quoted AFTER the real sentinel is ignored too — last-wins is not decoy immunity"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1407_007 ./acs/cycle1407"
  - criterion: "The decoy regression case lives in the package's own suite and reads the canonical fixture by path"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1407_008 ./acs/cycle1407"
---

# Eval: salvage-classifier-quoted-decoy-case

> Pins **decoy immunity** in `ClassifyBadVerdict`: a classifier must not key off
> a sentinel-shaped span that is a verbatim echo of another phase's sentinel,
> quoted into prose as evidence being discussed.
>
> The carryover todo that motivated this sat unpicked for 14 cycles asking only
> that the cycle-1298 quoted-decoy corpus be exercised against the classifier.
> Probing HEAD before freezing the contract showed why that framing was too
> weak: the corpus already classified `Recoverable=false` — but with
> `reason="evolve-verdict sentinel present but its payload is not recoverably
> malformed"`, which proves the classifier **never reached the report's own tail
> sentinel at all**. `sentinelPayloadRE.FindStringSubmatch` takes the *first*
> match in the document, and the first match in that corpus is a quoted decoy.
> The right answer for the wrong reason: the classifier had reproduced, inside
> itself, the very first-sentinel-wins bypass (F-1) that the fixture was landed
> to document.
>
> Hence the 8/10 cap on the crux criterion, which is a *false-negative* probe —
> append a genuinely malformed tail sentinel to the real corpus and require the
> classifier to reach it — and the companion 7/10 anti-overcorrection cap: "take
> the last match instead of the first" satisfies the crux and is still wrong,
> because a decoy quoted after the real sentinel must be ignored too.
>
> Source incidents: cycle-1298 (the F-1 first-sentinel-wins gate bypass this
> corpus documents); cycle-641
> (`instincts/lessons/cycle-641-infra-incident-classifier-matches-echoed-prompt-keywords.yaml`
> — "classifiers MUST exclude any span that is a verbatim echo of injected
> prompt/instruction text"); cycle-603 (the placeholder-echo guard, same class).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| corpus-not-recoverable | Quoted decoys alone never make a report salvageable | 6/10 | `go test -tags acs -run TestC1407_005 ./acs/cycle1407` |
| decoy-immunity-crux | The real tail sentinel is classified through the decoys | 8/10 | `go test -tags acs -run TestC1407_006 ./acs/cycle1407` |
| anti-overcorrection | A decoy quoted after the real sentinel is still ignored | 7/10 | `go test -tags acs -run TestC1407_007 ./acs/cycle1407` |
| durable-regression | The case lives in the package suite, reading the one canonical fixture | 6/10 | `go test -tags acs -run TestC1407_008 ./acs/cycle1407` |
