---
score_cap:
  - criterion: "An unverified item older than the window ranks below a fresh one"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestRank_AnUnverifiedItemOlderThanTheWindowRanksBelowAFreshOne$' ./internal/inboxrank"
  - criterion: "A premise verified inside the window keeps an old item fresh"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestRank_APremiseVerifiedInsideTheWindowKeepsAnOldItemFresh$' ./internal/inboxrank"
  - criterion: "Freshness halves each window from the verification date, else the filing date"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestRank_FreshnessHalvesEachWindowFromTheVerificationOrElseTheFiling$' ./internal/inboxrank"
  - criterion: "The policy reads the legacy age key as the freshness weight while the operator policy.json still names it"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestLoad_InboxPriorityReadsALegacyAgeKeyAsTheFreshnessWeight$' ./internal/policy"
  - criterion: "inbox lint flags a declared path that is gone on main at warn level only"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestLint_FlagsAPathThatIsGoneOnMain$' ./cmd/evolve"
---

# Eval: Freshness factor keyed on premise_verified_at and a dead-path lint

> Pins the behavior of AD2-8 (Graft C of docs/plans/architecture-decisions-2026-11.md). The `age` boost raised the rank of the oldest items, which are the items most likely to rest on a stale premise. The `freshness` factor decays from the premise verification date, or from the filing date when no premise was verified. `inbox lint` warns, and never fails, on a declared path that is no longer tracked on main. Source incident: docs/incidents/2026-09-26-stale-premise-reached-build.md.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| stale-ranks-low | An unverified old item ranks below a fresh one | 6/10 | `go test -run TestRank_AnUnverifiedItemOlderThanTheWindowRanksBelowAFreshOne ./internal/inboxrank` |
| verified-stays-fresh | A verified premise keeps an old item fresh | 6/10 | `go test -run TestRank_APremiseVerifiedInsideTheWindowKeepsAnOldItemFresh ./internal/inboxrank` |
| decay-curve | Half per window from the verification or filing date | 6/10 | `go test -run TestRank_FreshnessHalvesEachWindowFromTheVerificationOrElseTheFiling ./internal/inboxrank` |
| policy-key-migration | The legacy age key still loads | 7/10 | `go test -run TestLoad_InboxPriorityReadsALegacyAgeKeyAsTheFreshnessWeight ./internal/policy` |
| dead-path-warn | A gone path warns and the exit stays 0 | 6/10 | `go test -run TestLint_FlagsAPathThatIsGoneOnMain ./cmd/evolve` |
