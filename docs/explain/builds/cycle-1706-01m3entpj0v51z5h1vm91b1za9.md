# Build Explanation — Cycle 1706

## Build Binding
- Cycle: 1706
- Base SHA: a05f6b81f5b651b29f4123e3e8e88364a43101da

## Summary
A new package, `internal/gittest`, now owns git-backed test fixtures from creation to teardown. The
three fixtures behind the recurring `TempDir RemoveAll cleanup: unlinkat …/.git/objects: directory not
empty` CI flake (PR #545, #548, #638, #643) now build their repos through it. Each fixture repo writes
`maintenance.auto=false` into its own config, so no git run inside it can detach a background
maintenance writer. Teardown retries with a bounded backoff. If the tree still cannot be removed, the
test fails with the path and the PIDs of the processes that hold it.

## Rationale
The TDD phase traced every `git commit` and `git fetch` on git 2.50. Since git 2.47 each one spawns
`git maintenance run --auto --detach`, a child that keeps writing in `.git/objects` after the command
returns. That writer races `t.TempDir()`'s single `RemoveAll`. The scout plan's `-c gc.auto=0` does not
stop the spawn. A per-call `-c` flag also cannot reach the commits that production code makes inside a
fixture: routingtest's `RunCycle` commits there without going through any helper. So the setting is
persisted in each repo's own config, which every git in that repo reads. The bounded retry absorbs any
other late writer. The holder diagnostic turns a future recurrence from a bare path into an actionable
PID.

## Changed Areas
- `go/internal/gittest/fixture.go` — new package. It adds `Fixture` (a work tree on main), `Bare`,
  `Clone` (the clone persists the settings through `clone -c`), `Repo`, and `Repo.Git`, which fails the
  test with its arguments on error. Teardown is an 8-attempt doubling-backoff `RemoveAll`, registered
  after `TempDir` so it runs first. On exhaustion the holders come from `lsof +D`, judged by its output
  and not its exit status, and "diagnostic unavailable" is stated when lsof is absent.
- `go/internal/gittest/fixture_test.go` — tests the retry outlasting a transient hold, the
  exhaustion message (path, attempt count, and the no-lsof fallback), and the lsof parser.
- `go/internal/gittest/apicover_named_test.go` — names and executes every export. It pins the
  no-ambient-identity commit, the persisted `maintenance.auto=false`, and the config that a clone of a
  bare repo carries.
- `go/.apicover-enforce` — enrolls `./internal/gittest`, which graduates the new package under the
  repo-wide unnamed-export gate.
- `go/internal/core/worktree_startref_test.go` — `startRefFixture` builds its origin, seed and runtime
  clone with `gittest.Bare`, `gittest.Fixture` and `gittest.Clone`. The hand-rolled `gitAt` wrapper is
  removed, so `laneStartRef`'s own fetch runs in a quiet clone.
- `go/internal/core/build_persona_budget_check_test.go` — the persona-budget floor test builds its
  worktree with `gittest.Fixture` instead of a raw `t.TempDir()` and an inline git closure.
- `go/internal/routingtest/engine.go` — `initScenarioRepo` now returns the root of a `gittest.Fixture`,
  which covers every scenario that `TestEngine_RunAllExecutesPureAndCycleSpecs` runs, including
  RunCycle's own commits.
- `go/acs/cycle1706/predicates_test.go` — the TDD phase's acceptance predicates for this task (not
  edited by Build).
- `.evolve/evals/tempdir-cleanup-vs-git-flake.md` — the TDD phase's eval for this task (not edited by
  Build).

## Design Decisions
The fixture root is a `repo` subdirectory of `tb.TempDir()`, not the TempDir root itself. The retried
removal therefore owns the tree, and TempDir's own cleanup finds nothing left. The settings live in
repo config rather than in environment variables, because production code under test builds its own
git environment. `Bare` and `Clone` are exported because the startref fixture needs a pushable origin
and a tracking clone. The alternative, init plus remote/fetch/checkout wiring in each caller, would
spread fixture logic back into tests. Teardown never uses chmod or chflags to force a removal. A tree
that cannot be removed is a real failure to report, not something to hide.

## Verification
All 9 cycle-1706 ACS predicates pass. That includes the shim-driven caller proof over the real sighted
tests, and a darwin `-race -count=20` stress run of both sighted packages and of the helper's own
suite. The helper's unit tests, `go vet`, and gofmt are clean.

## Compatibility
This change is confined to test code: `internal/gittest` is imported only by tests and by the
routingtest scenario driver. The fixtures now start on branch `main` with a fixed identity; none of the
migrated tests depended on the ambient default branch or identity.

## Limitations
The fourth sighting (`phases/audit` `TestApicoverNewPackageGraduationDefault_CleanGitTree_StaysNoOp`)
and the roughly 120 other raw `exec.Command("git", …)` test sites are not migrated. Triage deferred
them as `gittest-repo-wide-audit`. The Linux half of the stress criterion is the PR's ubuntu CI run and
is not verified by this darwin build.
