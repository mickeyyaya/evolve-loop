---
score_cap:
  - criterion: "evolve ci watch --sha S returns 0 on a green required run and 1 on a red one, printing each workflow's conclusion"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1801_00[56]' ./acs/cycle1801"
  - criterion: "--tag waits for both the required and the release workflows, and either one red makes the watch red"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1801_009 ./acs/cycle1801"
  - criterion: "a red PR run files no inbox item; a red main SHA or tag files exactly one ciwatch fix-forward item naming the SHA and --cycle"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1801_00[689]' ./acs/cycle1801"
  - criterion: "exit 2 when gh fails or the run outlives the policy ci_watch timeout, with no inbox item"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1801_010 ./acs/cycle1801"
  - criterion: "malformed targets are usage errors (exit 10) that never call gh, and ci watch is listed in help"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1801_01[12]' ./acs/cycle1801"
  - criterion: "a red watch prints the ci classify verdict, and --workflow W is watched"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1801_0(07|13)' ./acs/cycle1801"
---

# Eval: evolve ci watch

> Pins `evolve ci watch (--sha S | --pr N | --tag T) [--workflow W]... [--cycle N]`
> from cycle 1801 (inbox item cli-ci-watch). Before this, ciwatch.Watch had no
> production caller, and /commit and /evo:publish watched CI with `gh run watch`
> by hand. The predicates drive the real binary. A fake `gh` (the predicate test
> binary itself, symlinked as `gh` on PATH) serves `run list` per workflow, can
> report a run as in progress first, and answers the `run view`, `pr view` and
> `api` calls that ci classify makes. A policy.json `ci_watch` block sets
> `poll_s=1`. The predicates assert exit codes, per-workflow conclusions,
> polling until complete, and inbox side effects: none for a PR, and exactly one
> `source: ciwatch` item for a main SHA or tag.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| sha-verdict | green 0 / red 1 with conclusions | 3/10 | `go test -tags acs -run 'TestC1801_00[56]' ./acs/cycle1801` |
| tag-both | required + release both gate the tag | 5/10 | `go test -tags acs -run TestC1801_009 ./acs/cycle1801` |
| inbox-policy | PR files none, main/tag file one | 4/10 | `go test -tags acs -run 'TestC1801_00[689]' ./acs/cycle1801` |
| unobservable | gh failure / timeout → exit 2 | 6/10 | `go test -tags acs -run TestC1801_010 ./acs/cycle1801` |
| usage | malformed → 10, no gh call; help lists it | 7/10 | `go test -tags acs -run 'TestC1801_01[12]' ./acs/cycle1801` |
| classify-and-workflow | red prints classify; --workflow watched | 7/10 | `go test -tags acs -run 'TestC1801_0(07|13)' ./acs/cycle1801` |
