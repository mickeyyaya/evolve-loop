---
score_cap:
  - criterion: "`evolve dossier publish --project-root P` commits a fixture plane's pending pair as one closeout commit, exits 0, leaves a half pair (a lone .md) pending untouched, and names both cycles"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1826_001_' ./acs/cycle1826"
  - criterion: "With a live run lease owned by another process the verb exits 1, names the run and its live pid, and publishes nothing; a lease whose owner exited does not hold it"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1826_002_' ./acs/cycle1826"
  - criterion: "--dry-run changes nothing in the plane (HEAD, status, every file) and lists the pending cycles, the half pairs and, when held, why"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1826_003_' ./acs/cycle1826"
  - criterion: "A busy git-mutation lock or a plane behind origin/main exits 1 naming the hold without waiting; nothing pending exits 0 even while the lock is busy"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1826_004_' ./acs/cycle1826"
  - criterion: "A pending directory that cannot be listed is an I/O failure: exit 2 naming it, with or without --dry-run"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1826_005_' ./acs/cycle1826"
  - criterion: "The verb and publishPendingDossiers both call one function that runs every check (plane.Classify, the live-run check, the origin/main relation)"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1826_006_' ./acs/cycle1826"
  - criterion: "The dossier usage names publish and keeps exit 10 for usage errors; runtime-reference.md's wave-boundary procedure documents the verb"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1826_00[78]_' ./acs/cycle1826"
  - criterion: "The loop exit's own publish behaviour is unchanged by the extraction"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestPublishPendingDossiers_|TestLoopSummary|TestPrepareIteration_PublishesPendingCloseouts|TestCampaignRun_PublishesTheWaves|TestRunFleet_PublishesTheLanes)' ./cmd/evolve"
---

# Eval: `evolve dossier publish` flushes pending closeouts through the loop exit's checks

> Pins the inbox item `cli-dossier-publish` (2026-09-30, operator request for a
> CLI verb per core function). `publishPendingDossiers` committed the closeouts
> in `.evolve/dossiers-pending` only when a fleet run's lanes finished, so after a
> killed loop the pairs waited for the next launch or for raw git, which skips
> the ship lock, the live-run check and the publish hold. The verb runs the same
> checks, through one function the loop exit also calls, then
> `dossier.PublishPending`: exit 0 published or nothing pending, exit 1 held
> (live run, lock busy, publish hold) naming the reason, exit 2 on I/O. RED
> authored in cycle 1826.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| publish-and-leave-half | pair committed, half pair left, exit 0 | 7/10 | `go test -tags acs -run '^TestC1826_001_' ./acs/cycle1826` |
| live-run-hold | exit 1 naming the run; dead owner does not hold | 7/10 | `go test -tags acs -run '^TestC1826_002_' ./acs/cycle1826` |
| dry-run-inert | nothing changes; lists pending, half pairs, hold | 6/10 | `go test -tags acs -run '^TestC1826_003_' ./acs/cycle1826` |
| lock-and-relation-holds | exit 1 without waiting; nothing pending exits 0 | 6/10 | `go test -tags acs -run '^TestC1826_004_' ./acs/cycle1826` |
| io-exit-2 | unlistable pending dir exits 2 | 5/10 | `go test -tags acs -run '^TestC1826_005_' ./acs/cycle1826` |
| one-checks-function | verb and loop exit share the checks | 5/10 | `go test -tags acs -run '^TestC1826_006_' ./acs/cycle1826` |
| usage-and-doc | usage names publish, exit 10; boundary doc | 4/10 | `go test -tags acs -run '^TestC1826_00[78]_' ./acs/cycle1826` |
| loop-exit-regression | loop-exit publish tests stay green | 6/10 | `go test -run '^(TestPublishPendingDossiers_\|…)' ./cmd/evolve` |
