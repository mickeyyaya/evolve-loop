---
score_cap:
  - criterion: "The internal/phases/ship target test files (explanation_gate, pushonly, realgit_testhelpers, stage_deleted) are out of go/internal/rawgitratchet/baseline.json, the ratchet scanner finds no raw git init site in them, and TestRatchet_NoNewRawGitFixtures passes"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags acs -run 'TestC1748_001_' ./acs/cycle1748/"
  - criterion: "The ship target files' tests (plus the addRemote-backed ship test) pass under go test -count=1, and every repository they commit, fetch, merge or push in under the test temp root carries maintenance.auto=false and gc.auto=0"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -tags acs -run 'TestC1748_002_' ./acs/cycle1748/"
  - criterion: "The internal/releasepipeline target test files (bridges_changelog, classify, default_release_verify, git_helpers) are out of the ratchet baseline, the scanner finds no raw git init site in them, and TestRatchet_NoNewRawGitFixtures passes"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags acs -run 'TestC1748_003_' ./acs/cycle1748/"
  - criterion: "The internal/releasepipeline package passes under go test -count=1, and every repository its tests commit, fetch, merge or push in under the test temp root carries maintenance.auto=false and gc.auto=0"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -tags acs -run 'TestC1748_004_' ./acs/cycle1748/"
  - criterion: "The maintenance probe tells a raw fixture (no quiet config) from one that persists maintenance.auto=false and gc.auto=0"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -tags acs -run 'TestC1748_005_' ./acs/cycle1748/"
---

# Eval: Migrate the ship and releasepipeline raw git fixtures to gittest

> Pins inbox item `raw-git-fixtures-migrate-to-gittest` for the two slices
> cycle 1748 committed: `internal/phases/ship` (4 files, 7 init sites) and
> `internal/releasepipeline` (4 files, 4 init sites). From git 2.47 a commit,
> fetch, merge or push-receive can leave a detached `git maintenance run
> --auto` child writing in `.git/objects` after the test returns, so
> `t.TempDir()`'s single `RemoveAll` fails with `unlinkat ...: directory not
> empty`. `gittest.Fixture` / `Bare` / `Clone` persist `maintenance.auto=false`
> and `gc.auto=0` and retry teardown; raw `git init` fixtures do neither.
>
> Source incidents: the flake failed CI three times (the dossier fixture
> twice, then `internal/core`'s shipped-lane fixture on PR #698, run
> 36427299523). Cycle 1748's bug-reproduction phase reproduced the race
> against a raw fixture (16-20 of 20 attempts) and showed `gittest.Fixture`
> survive the same forced race (0 of 20).
>
> The baseline half is checked with the ratchet's own scanner. The fixture
> half is observed, not read from source: the tests run under a git shim on
> PATH that records, after each successful commit, fetch, merge or push, the
> touched repository's config (a push is judged at its destination). A raw
> fixture renamed past the scanner still fails it, because its repositories
> keep maintenance on. The `internal/phases/audit` slice was escalated
> (protected surface) and `internal/core`, `cmd/evolve` and the smaller
> packages were deferred; none of them is pinned here.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| ship-left-baseline | ship target files unlisted, zero scanner sites, ratchet green | 7/10 | `TestC1748_001` |
| ship-repos-quiet | ship target tests pass and every touched repo is quiet | 8/10 | `TestC1748_002` |
| releasepipeline-left-baseline | releasepipeline target files unlisted, zero scanner sites, ratchet green | 7/10 | `TestC1748_003` |
| releasepipeline-repos-quiet | releasepipeline package passes and every touched repo is quiet | 8/10 | `TestC1748_004` |
| probe-validity | the probe flags a raw repo and clears a quiet one | 5/10 | `TestC1748_005` |
