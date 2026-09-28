# Build Explanation — Cycle 1748

## Build Binding
- Cycle: 1748
- Base SHA: 292d11bed29bc4d0c73bf51b3bcee505a66d20fe

## Summary
The `internal/phases/ship` and `internal/releasepipeline` test fixtures now build their git repositories with `gittest.Fixture`, `gittest.Bare` and `gittest.Clone` instead of a raw `git init`, and the eight migrated files are removed from the raw-git ratchet baseline.

## Rationale
From git 2.47, a commit, fetch, merge or push-receive can detach a `git maintenance run --auto` child that keeps writing in `.git/objects` after the test returns, so `t.TempDir()`'s single `RemoveAll` fails with `directory not empty`. This flake has reached CI three times. The `gittest` constructors persist `maintenance.auto=false` and `gc.auto=0` and retry teardown, so switching the fixtures to them removes the cause. The ratchet only lets a file's entry shrink, so each migrated file's entry has to be deleted in the same change. Adding quiet config to each raw helper was rejected: it would duplicate `gittest`, and the ratchet would still count those helpers as raw init sites.

## Changed Areas
- `go/internal/rawgitratchet/baseline.json` — the entries for the four ship files and the four releasepipeline files are removed, because the ratchet scanner now finds zero raw init sites in them.
- `go/internal/phases/ship/realgit_testhelpers_test.go` — `makeRepo` starts from `gittest.Fixture` and `addRemote` makes its origin with `gittest.Bare`. Every ship test built on these helpers therefore commits and pushes only into quiet repositories.
- `go/internal/phases/ship/pushonly_test.go` — `initPushOnlyRepo` uses `gittest.Bare` for its origin and `gittest.Fixture` for its work tree. The sync-main test's console clone comes from `gittest.Clone`, so the clone's own commit and push stay quiet too.
- `go/internal/phases/ship/stage_deleted_test.go` — both staging tests start from `gittest.Fixture`, which already provides the identity config the tests used to set.
- `go/internal/phases/ship/explanation_gate_test.go` — the standalone-identity test's worktree is a `gittest.Fixture` instead of a raw init plus identity loop.
- `go/internal/releasepipeline/bridges_changelog_test.go` — `makeHermeticGitRepo` starts from `gittest.Fixture`.
- `go/internal/releasepipeline/classify_test.go` — `initClassifyRepo` starts from `gittest.Fixture`.
- `go/internal/releasepipeline/default_release_verify_test.go` — `TestDefaultReleaseVerify_Success` starts from `gittest.Fixture`.
- `go/internal/releasepipeline/git_helpers_test.go` — `initTempRepoWithTag` starts from `gittest.Fixture`.
- `docs/explain/builds/cycle-1748-01m3mjy0fdren90rmb2wscfys1.md` — this explanation record.

## Design Decisions
The migration keeps each test's own git runner, environment and assertions and replaces only how the repository comes into existence. `gittest.Fixture` always initialises on `main`. The existing `branch -M main` steps are now no-ops but are kept, because some tests (for example `divergedWorktree`) rely on them. `tempRepoDir` stays, because it still serves non-repository directories such as a linked worktree target, and a linked worktree shares its main repository's quiet config.

## Verification
All five cycle-1748 ACS predicates pass (`go test -count=1 -tags acs ./acs/cycle1748/`). Their git shim observes every commit, fetch, merge and push the target tests make and finds each touched repository quiet. `TestRatchet_NoNewRawGitFixtures` passes, and both packages pass under `go test -count=1`.

## Compatibility
Tests and ratchet data only; no production code, flags or schemas changed.

## Limitations
The `internal/phases/audit` slice (escalated as a protected surface), `internal/core`, `cmd/evolve` and the smaller packages still carry raw fixtures and their baseline entries. `repair_resume_test.go` still makes one raw `git clone` into a `tempRepoDir`, which the ratchet does not count.
