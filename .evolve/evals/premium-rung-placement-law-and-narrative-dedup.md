---
score_cap:
  - criterion: "violatesPremiumPlacement(Profile) bool exists in package profiles with a passing table test, TestViolatesPremiumPlacement, in the untagged suite"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestViolatesPremiumPlacement$' -v ./internal/profiles/ 2>&1 | grep -q -- '--- PASS: TestViolatesPremiumPlacement'"
  - criterion: "The real-tree placement guard runs over the live profiles and passes: max and ultra appear only on deep/top profiles"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'EffortOnlyOnDeepOrTopProfiles$' -v ./internal/profiles/ 2>&1 | grep -q -- '--- PASS: Test.*EffortOnlyOnDeepOrTopProfiles'"
  - criterion: "The whole profiles package stays green after the law is widened (no live profile trips the {max, ultra} law)"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/profiles/..."
  - criterion: "The loop_unblock satellite no longer restates the codex deep-tier model history (its message contradicted itself: sol per 2026-09-10, astra since 2026-09-09)"
    max_if_missing: 4
    evidence: "! grep -q 'gpt-6-astra since 2026-09-09' go/internal/profiles/loop_unblock_contract_test.go"
  - criterion: "The runtime-reference Deep-tier family arrangement row states current state only and points at the placement guard instead of restating rung history"
    max_if_missing: 4
    evidence: "row=$(grep 'Deep-tier family arrangement' docs/operations/runtime-reference.md) && ! printf '%s' \"$row\" | grep -q 'withdrawn the next day' && ! printf '%s' \"$row\" | grep -q \"superseding 2026-08-28's max\" && printf '%s' \"$row\" | grep -Eq 'violatesPremiumPlacement|EffortOnlyOnDeepOrTopProfiles'"
---

# Eval: Widen the effort placement law to {max, ultra} and de-duplicate the rung narrative

> Pins the premium-rung placement law found in the retro architecture review of #526
> (inbox `premium-rung-placement-law-and-narrative-dedup`, HIGH-1 still open on
> 2026-09-27). The codex manifest maps `ultra` above `max`, but
> `TestMaxEffortOnlyOnDeepOrTopProfiles` only checked profiles whose effort was
> exactly `max`. So a balanced profile at `ultra` passed every guard while raising
> spend on the cheapest phases. Cycle 1816 confirmed this live: a shadow profile
> tree with a balanced `ultra` profile passed the current guard, and one with a
> balanced `max` profile failed it. The fix extracts the law as a table-tested
> Specification, `violatesPremiumPlacement(Profile) bool`, with premium rungs
> {max, ultra}. The guard gets its verdict from that function, and the prose homes
> that restated the rung now point at the guard.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| specification-table | `violatesPremiumPlacement` exists, and a passing table test runs in the untagged suite | 7/10 | `go test -run '^TestViolatesPremiumPlacement$' -v ./internal/profiles/` shows PASS |
| live-tree-guard | The placement guard runs and the live tree complies | 6/10 | `go test -run 'EffortOnlyOnDeepOrTopProfiles$' -v ./internal/profiles/` shows PASS |
| no-regression | The whole profiles package stays green | 6/10 | `go test -count=1 ./internal/profiles/...` |
| satellite-dedup | `loop_unblock_contract_test.go` no longer restates the model history | 4/10 | `! grep -q 'gpt-6-astra since 2026-09-09' …` |
| docs-current-state | The runtime-reference row drops dated history and names the guard | 4/10 | Row grep: no `withdrawn the next day`, no `superseding 2026-08-28's max`, names the guard |
