# Test plan (Go-only)

> **Authoritative.** This document supersedes `docs/TEST_PLAN.md` (the Phase-1
> bash→Go parity plan). Once the Go-only consolidation lands, `docs/TEST_PLAN.md`
> is historical.

The Go binary is the only runtime. Name new permanent tests for the **behavior**
they pin. Existing cycle/ACS identities remain compatibility contracts until a
reviewed replacement preserves their selectors and provenance. Tests are
organized along **two independent axes**:

- **Cost axis** (enforced by Go **build tags**) — controls what the default
  suite runs and therefore wall-clock time: `default` (fast) → `integration` →
  `e2e`. This is the only mechanism that gates the fast/slow split.
- **Granularity axis** (a **convention + harness**, documented, not a build
  tag) — describes *what a test exercises*: unit → functional → component →
  integration → e2e.

Why two axes? If granularity were a build tag, tagging every unit file
`//go:build unit` would make a bare `go test ./...` compile **zero** tests and
silently void the coverage gate. So granularity is how you *write* a test;
cost is how it's *selected*.

### Cost axis (build tags + Make targets)

| Cost layer | Build tag | What runs | Command |
|---|---|---|---|
| **fast** (default) | _(none)_ | All buildable non-ACS packages, including co-located untagged tests, component, fixtures, trustkernel, public packages and examples. | `make test` |
| **integration** | `//go:build integration` | Complete non-ACS runtime selection, including untagged tests and real FS / git / tmux cases; race detection and `coverage.txt`. Shared by CI and release. | `make test-integration` |
| **e2e** | `//go:build e2e` (+ `evolve_test_phases`) | Full-cycle subprocess paths (`cmd/evolve/e2e_*`) + `test/e2e`. Live sub-tier self-skips without `EVOLVE_E2E_LIVE`. CI passes `evolve_test_phases` alongside `e2e` so the serve-phase subprocess round-trip test (which registers a test-only `echo` phase into the phases registry) compiles + runs — it was previously orphaned (no runner passed its tag). | `make test-e2e` |
| **durable-acs** | `//go:build acs` | Artifact-free ACS regression predicates under `acs/regression/...` — flag ceilings/readers/progress, doc-comment coverage, no-orphan-scripts, per-cycle source invariants. A standing push/PR gate (ci.yml `acs-durable` job, **`fetch-depth:0`** for the git-diffing flag predicates) that complements the per-cycle EGPS run (`internal/acssuite`). Artifact-dependent predicates (`buildselfcheck`, `cycle57/80/99`) `t.Skip` cleanly; `acs/redteam` is excluded (needs a live ledger). | `make test-acs-durable` |
| **everything** | composed tiers | Integration with race, then E2E without race using `e2e evolve_test_phases`, then durable ACS. CI runs durable ACS in its own `ci.yml` job. | `make test-all` |
| **spike** (manual diagnostic) | `//go:build spike` | Live-LLM diagnostics (e.g. `TestSpikeAdvisorLive` — the real advisor on Opus). **Not in CI**: expensive (real quota) and never asserts failure — a developer probe, run on demand. | `go test ./cmd/evolve/ -tags spike -run TestSpikeAdvisorLive -v -timeout 300s` |

**Git's background maintenance is off in every Make recipe.** `internal/gittest.MaintenanceConfig()` is the single Go source of the settings (`maintenance.auto=false`, `gc.auto=0`); it returns a fresh copy on every call, so no caller can change what the next one sees. `go/Makefile` mirrors it: it exports `GIT_CONFIG_COUNT` with the same pairs, so every git a test runs under a Make recipe (CI's `make test-integration` and `make test-e2e`, and local `make test`) sees them at command scope. `GIT_CONFIG_COUNT` needs git 2.31 or later; an older git silently ignores it, and raw fixtures then run with maintenance on. From git 2.47 a commit otherwise detaches `git maintenance run --auto`, which keeps writing in a fixture's `.git` after the command returned and fails `t.TempDir`'s cleanup with `directory not empty`. That flake class failed CI four times (the dossier fixture twice, then `internal/core` on #698 and #705), in fixtures built with a raw `git init`. `internal/gittest` fixtures persist the same settings in each repo's own config (built from the same list), so they are quiet under a direct `go test` too; raw fixtures are quiet only under a Make recipe, which is why the raw-git ratchet still shrinks them (inbox item `raw-git-fixtures-migrate-to-gittest`).

A test that sets its own `GIT_CONFIG_COUNT` replaces the recipe's whole table, not just the keys it names. Such a test builds its variables with `gittest.ConfigEnv(extra...)`: the `MaintenanceConfig()` pairs plus its own, with a count that also shuts out any ambient command-scope entry past the table. `internal/core`'s cycle-source fixture (extras `commit.gpgsign` and `core.hooksPath`) and its signal-checkpoint test (no extras; it once set `GIT_CONFIG_COUNT=0` to clear ambient config) use it.

Pins: `TestMakeTestRecipes_RunGitWithBackgroundMaintenanceOff` runs a probe through the real `test` and `test-integration` recipes, with the variables removed from its own environment and global and system config off. The probe's expectations are rendered from `MaintenanceConfig()`, so a key added there fails the test until the Makefile exports it too. gittest's fixture pins read with `git config --local`, so the recipe's command-scope values cannot satisfy them. `TestConfigEnv_ReplacesAmbientCommandScopeWithMaintenanceConfigAndExtras` pins the helper.

A file's build tag must sit at the very top, followed by a blank line:

```go
//go:build integration

package bridge
```

**Self-containment rule:** a build-tagged file is excluded from the default
build, so a *fast* (untagged) test must never reference a symbol defined only
in a tagged file (the default build would fail to compile). Keep shared helpers
that fast tests need in an untagged file. Tagged files in the same package +
tag compile together, so they freely share helpers among themselves.

### Granularity axis (convention + the `go/test/fixtures` harness)

| Granularity | Where | Package style | Collaborators |
|---|---|---|---|
| **unit** | co-located `*_test.go` | `package foo` (white-box) | none / fixtures fakes |
| **functional** | co-located `*_test.go` | `package foo_test` (black-box) | the package's exported API only |
| **component** | `go/test/component/` | `package component` | several real adapters wired via `fixtures.NewWorkspace`, temp FS, no subprocess |
| **integration** | `go/test/integration/` | `package integration` (`+integration`) | real git / tmux / FS |
| **e2e** | `go/test/e2e/` + `cmd/evolve/e2e_*` | `package e2e` (`+e2e`) | the built `evolve` binary, full cycle |

Two tiers stand outside the axes:

| Tier | Location | Scope | How to run |
|---|---|---|---|
| **Trust-kernel** | `go/test/trustkernel/` | Black-box pinning of safety invariants (ship gate, audit-binding, routing floor, transition legality, profile validity). | `go test ./test/trustkernel/` |
| **Commit-gate** | `go/internal/commitgate/` | Unit lanes + a Go-only golden (`parity_test.go`) pinning the attestation byte layout + tree-SHA binding over a real git repo. | `go test ./internal/commitgate/...` |

## The `go/test/fixtures` harness — single source of truth

Every layer builds on one harness so adding a test is fast and duplication
can't regrow. Read `go/test/fixtures/*.go` for the full surface; the
load-bearing pieces:

- **`NewWorkspace(t)`** — Builder for an isolated temp project root + `.evolve/`.
  `.WithState(...).WithCycleState(...).WithFiles(...).WithCycleFiles(n,...).WithGitInit().Build()`.
  Replaces the old `newStore()` / `SetupTempProject()` copies. Storage-free by
  design (callers construct `storage.New(ws.EvolveDir)` themselves).
- **`FakeStorage` / `FakeLedger` / `FakeRunner` / `FakeBridge`** — canonical
  `core.*` test doubles (supersets with opt-in error/lock injection). One
  implementation, not three.
- **`BuildRunners(verdicts)`** — full per-phase runner map for orchestrator tests.
- **`FixedClock(start, step)`** — deterministic clock for `DurationMS` assertions.
- **`RequireNoErr` / `RequireErrContains` / `MustWrite` / `MustRead` /
  `WantFileContains` / `FilePresent`** — the assertion facade. `FilePresent` is
  the pure-bool existence check for genuine skip preconditions (do **not** use
  `acsassert.FileExists`, which logs an `Errorf`, as a skip guard).
- **`NewLedgerEntry(opts...)`** — Object-Mother for valid `core.LedgerEntry`s.
- **`StressN(t, n, k, fn)`** — concurrency-stress harness: launch `n` goroutines
  × `k` iterations of `fn(g, i)`, released simultaneously by a closed-channel
  barrier (so contention is maximized, not accidentally serialized by staggered
  startup), joined before return. Pair it with an invariant assertion under
  `-race`; it is the canonical primitive for the mutex/flock stress backfill.

**Import-cycle note:** `fixtures` imports `core`, so a white-box `package core`
test cannot import it (cycle). Such tests use `package core_test` (black-box) —
which is exactly the "functional" granularity. **Exception:**
`internal/core/orchestrator_test.go` exercises unexported internals
(`recordAuditBinding`, `runGit`, …), so it must stay white-box and keeps its own
private fakes — the one unavoidable duplicate of the harness doubles.

**Duplication status / migration rule.** `fixtures` is the single source of
truth for test doubles, workspace setup, clocks, and assertions. New tests use
it. Existing duplicates are being migrated incrementally: the 8 copy-pasted
`fixedClock` helpers (now `fixtures.FixedClock`) and `storage`'s `newStore` (now
`fixtures.NewWorkspace`) are done; the remaining per-package `newStore`-style
`.evolve` builders and scattered `must*`/`assert*` helpers should route through
`fixtures` whenever a file is next touched (don't churn untouched files purely to
migrate — KISS). Do **not** reintroduce a local fake/clock/temp-dir builder when
the harness already provides one.

### How to add a test at each layer

- **unit** — add `TestFoo_Behavior` to `internal/foo/foo_test.go` (`package foo`).
  Use `fixtures` fakes for `core.*` collaborators. No build tag.
- **functional** — same dir, `package foo_test`; call only exported API.
- **component** — add to `go/test/component/`; `fixtures.NewWorkspace(t).Build()`,
  construct the real adapter(s), assert a cross-cutting property. No build tag.
- **integration** — add to `go/test/integration/` with `//go:build integration`;
  `t.Skip` if the external tool is absent; shell out and assert.
- **e2e** — add to `go/test/e2e/` (or `cmd/evolve/e2e_*`) with `//go:build e2e`;
  build the binary into `t.TempDir()`, exec it, assert real stdout/exit.

## Latency measurement

`go/cmd/testlatency` turns a `go test -json` stream into a Markdown report
(per-package wall time, longest-path test per package, slowest tests, threshold
flags). Regenerate the fast-suite report any time:

```
make test-latency   # → go/test/latency-report.md
```

`go/test/latency-baseline.md` is the pre-split snapshot; `go/test/latency-report.md`
is the post-split snapshot. Both are machine-dependent and regenerable — they
are tracked as comparison points, not gates.

### Result and the known fast-suite poles

The cost-axis split + seam work cut the default suite from **~206s** to **~20s
warm** (`go test ./...`, fast tier; a cold run adds compile). The headline wins
are **banked**: `cmd/evolve`'s full-cycle e2e (205s→behind `e2e`),
`internal/bridge`'s tmux/live tests (47s→0.47s), `internal/phases/ship`'s real-git
suite (**~22s→2.4s** fast — the 23-case `TestNative_*` parity matrix and the other
git-driven files now carry `//go:build integration`), and `internal/core`'s retry
backoff (`backoffSleep` is swapped to a no-op in `TestMain`, see
`orchestrator_main_test.go` — the single highest-leverage core knob).

**One tiering mechanism: build tags.** `testing.Short()` is *not* used to tier
tests here. A `-short` skip leaves the test compiled-and-linked, and because CI
runs `-tags integration` and **never** `-short`, the guard does nothing (it is
inert). To keep a slow test out of the fast tier, either tag the file
`//go:build integration` (it does real IO) **or** seam the slow dependency
(`sysexec.RunFunc` + `fixtures.FakeExec`, a `func() time.Time` clock, an injected
interval) so the test runs fast *in the default tier with real coverage*. Do not
reintroduce `if testing.Short() { t.Skip() }` as a tiering device.

The remaining aggregate floor (measured isolated, 2026-06-15):

- **`internal/core` (~10s)** — genuine, already-optimized compute: ~317 in-memory
  orchestrator-cycle tests. `-parallel 1` = 23.5s of CPU work that parallelism
  caps at ~10s on 8 cores; 78% already take `t.Parallel`. Not forks (the
  git-driven leak tests are `//go:build integration`) and not sleep
  (`backoffSleep` no-op). No cheap lever remains — only reducing per-cycle test
  cost (deep, risky) would move it, and it would still overlap under the aggregate.
- **`cmd/evolve` (~10s)** — diffuse: ~86 untagged CLI test files each fork real
  `git` for temp-repo setup; the slowest single test is ~0.7s, so it is
  death-by-a-thousand-forks, not one hot test. The lever is tagging the genuinely
  end-to-end tests `//go:build integration`/`e2e` (CLI-logic coverage lives in the
  underlying `internal/*` units) or seaming the CLI git layer — a sizable refactor.
- **`internal/looppreflight` (~5s) / `phaseobserver` (~3s) / `adapters/observer`
  (~2s)** — integer-second timing granularity: host-probe / poll intervals floor
  at 1s (`EVOLVE_OBSERVER_POLL_S`, `Config.PollS`), so a test waits ~1s for one
  poll tick. The clean fix is sub-second `time.Duration` injection into those
  intervals; until then they self-cap at the 1s floor.

Because packages overlap under `go test -p`, the aggregate is **floored by the two
~10s poles running concurrently with everything else** — the sub-5s poles do not
move it. Per-package seams still pay off for *that package's* dev-iteration loop
even when they don't lower the aggregate. The cost-axis split is an intentional
trade (determinism + safety coverage over shaving a subprocess-bound wall).

**Why ship's fast tier was hard to parallelize (now historical).** When ship's
real-git tests were fast-resident they could not take `t.Parallel`: `t.Setenv`
panics under parallel, package-var seam save/restore races, and the darwin
**EBADF** FD-teardown flake (`close …: bad file descriptor`) under
`-race -count=3 -shuffle=on`. Tagging them `//go:build integration` removed them
from the fast tier entirely, so this no longer constrains the fast suite;
`tempRepoDir`'s best-effort chmod-walk cleanup still contains the EBADF surface in
the integration tier.

### Known: real-tmux integration tests are load-sensitive

The `internal/bridge` `TestRealTmux_*` integration tests drive real `tmux`
sessions and poll for a REPL prompt. Package/process concurrency can starve
prompt detection and produce `exit 80` ("REPL prompt never appeared"). Preserve
the failing run and diagnose with `go test -race -count=1 -tags integration
./internal/bridge/`; `make test-integration` now runs the complete runtime suite,
so it is not an isolated bridge probe. A passing diagnostic rerun does not erase
the original failure or establish its cause. Load hardening remains separate
from test consolidation.

### Where the legacy ACS predicates live

`go/acs/cycle*/predicates_test.go` (the cycle-pegged `TestC<N>_*` Go ports of the
bash EGPS predicates) and `acs/regression-suite/` (deprecated `.sh`) are **not yet
deleted** — they run in the live EGPS suite and are excluded from the unit gate
(`go list ./... | grep -v '/acs/'`) because they read runtime artifacts. Their
durable invariants are being ported into `go/test/trustkernel/`; the cruft is
retired at Stage 5. See `go/test/trustkernel/PORTING-LEDGER.md`.

## Coverage targets

- **Strict package floors:** `make cover-strict` enforces the thresholds in
  `go/.cover-strict` and rejects failed tests even when they emit high coverage.
- **Public API coverage:** `make apicover-enforce` measures and enforces the
  packages in `go/.apicover-enforce`. CI calls `make apicover-check` after its
  successful integration run to inspect that run's fresh `coverage.txt`.
- **Advisory diagnostics:** the ordinary 85% sweep reports function-level
  warnings; it does not enforce an 85% floor on every internal package.
  `make cover` produces reports; the strict/API targets supply the hard gates.
- **Intent over surface (AGENTS.md Rule 9).** A test must probe the *behavior under
  change*, not merely re-walk lines for a coverage number. A passing test that
  would still pass if the invariant were broken is a no-op and is rejected in review.
- Coverage is a floor, not a goal. Trust-kernel pinning tests and behavioral
  integration tests carry more signal than line-chasing.

## Test-design conventions

1. **AAA** — Arrange, Act, Assert. Keep the three phases visually distinct.
2. **Behavior-naming** — prefer `TestShipGate_BlocksWhenRedCountNonZero` for new
   permanent cases. Keep incident context in comments/history. Preserve existing
   ACS names and migrate every selecting caller before renaming a legacy case.
3. **No live-repo / runtime-state dependence.** A test must construct its own
   isolated state (`t.TempDir()` + `git init`) rather than reading the live
   repository or `.evolve/runs/`. Determinism is non-negotiable.

   > **Cautionary example.** `TestResolvePrevTag_ValidGitRepo` originally
   > `git describe`'d the *live* worktree and asserted a `v*` tag. It broke the
   > moment a non-semver tag (`pre-consolidation-2026-05-30`) shadowed the
   > expected one — the test depended on ambient repo state it did not control.
   > The fix builds an isolated temp repo (`git init` + one commit + `git tag
   > v1.2.3`) and asserts `resolvePrevTag` returns exactly `v1.2.3`. See
   > `go/internal/releasepipeline/git_helpers_test.go`.

4. **Reuse existing helpers; read real exports first.** Before calling into a
   package, read its source and use the actual exported API — do not invent
   signatures (AGENTS.md Rule 8).
5. **Skip, don't fail, on missing environment.** Tier tests that need git or
   on-disk fixtures `t.Skip` when the environment is absent (e.g. a source
   tarball with no git) rather than failing on a machine-specific path.
6. **Concurrency tests assert an invariant, not "didn't crash."** Use
   `fixtures.StressN(t, n, k, fn)` (closed-channel barrier — maximizes real
   overlap) and assert the protected invariant holds *after* the storm: the
   ledger hash-chain still verifies, a quota never overspends, a sidecar writer
   emits no torn lines. Name them `Test<Unit>_Concurrent<Action>_NoRace<Invariant>`
   (e.g. `TestLedger_ConcurrentAppend_NoRaceChainIntact`) and run under `-race`.
   Every `sync.Mutex`/`RWMutex`/flock owner should have one — the Phase-2
   `TestEveryMutexHasStressTest` guard enforces it.

## G3 — Invariant → test → knowledge-doc map

Each trust-kernel invariant has a pinning test in `go/test/trustkernel/` and a
documenting knowledge doc under `knowledge/architecture/`.

| Invariant | Pinning test | Knowledge doc |
|---|---|---|
| Ship is eligible only when EGPS `red_count == 0` (all-green ⇒ ship-eligible) | `TestShipGate_ShipEligibleOnlyWhenRedCountZero` | `knowledge/architecture/trust-kernel-and-egps.md` |
| Any RED predicate ⇒ verdict FAIL, ship blocked | `TestShipGate_BlocksWhenRedCountNonZero` | `knowledge/architecture/trust-kernel-and-egps.md` |
| `reach(ship) ⇒ build ∧ audit` (integrity floor) | `TestRoutingFloor_ShipRequiresBuildAndAudit` | `knowledge/architecture/routing-and-advisor.md` |
| No-ship cycle imposes no floor (scout-only is legitimate) | `TestRoutingFloor_NoShipCycleIsUnconstrained` | `knowledge/architecture/routing-and-advisor.md` |
| Trivial cycle exempts tdd but never build/audit | `TestRoutingFloor_TrivialCycleExemptsTDDNotBuildAudit` | `knowledge/architecture/routing-and-advisor.md` |
| Ship reachable only after audit (no spine bypass) | `TestStateMachine_ShipFollowsAuditOnlyViaShippableVerdict` | `knowledge/architecture/phase-pipeline.md` |
| Audit verdict routes ship (PASS/WARN) or retro (FAIL) | `TestStateMachine_AuditVerdictRoutesShipOrRetro` | `knowledge/architecture/phase-pipeline.md` |
| Every phase profile on disk is valid JSON with name+cli | `TestProfile_AllPhaseProfilesValid` | `knowledge/architecture/cli-matrix-and-drivers.md` |

Pending-port invariants (audit-binding tree-SHA, single-writer/worktree isolation,
schema-filter enforcement) are tracked in `PORTING-LEDGER.md` and map to
`knowledge/architecture/state-and-ledger.md` and `bridge-and-adapters.md`.

## CI shape

`.github/workflows/go.yml`:

- Linux/macOS Go 1.23 compatibility lanes plus Linux Go 1.27; the module language
  baseline remains Go 1.23. Checkout includes full history.
- `make test-integration` — all non-ACS runtime packages with integration,
  race detection, uncached execution and coverage. This includes commit-gate,
  component, fixtures, public-package and trustkernel tests.
- `make test-e2e` — existing no-race, uncached E2E tier with
  `e2e evolve_test_phases` and its 45-minute timeout. Live cases retain their
  individual explicit opt-ins and skip behavior.
- `make apicover-check` and `make cover-strict` — hard API and enrolled package
  coverage gates; the separate 85% function diagnostics remain advisory.

`ci.yml` runs plugin validation (`validate`) and durable ACS (`acs-durable`).
Neither `go.yml` nor `ci.yml` triggers on a push or pull request of its own;
both keep `workflow_call` and `workflow_dispatch`.

`.github/workflows/required.yml` (workflow `required CI`) is the one unfiltered
entry point for every pull request and for pushes to `main` and
`go-rewrite-phase-1`. Its jobs:

- `changes` diffs the event locally (PR: merge-base to head; push: before to
  head; `--no-renames`, NUL-delimited) and selects the optional suites. A Git
  failure or an invalid commit ID fails the job; manual runs, initial pushes and
  empty diffs select everything. After a force-push the `before` commit is
  unreachable, so routing fails closed and `CI required` stays red until a
  manual `workflow_dispatch` run of `required CI` posts a green result.
- `validate` calls `ci.yml` on every event, documentation-only changes included.
- `go` calls `go.yml` when `go/`, `skills/`, `agents/` or any unlisted path
  changed. `landing` calls `landing-validation.yml` when `landing/`,
  `docs/explain/` or any unlisted path changed. Markdown under `docs/reports/`,
  `docs/research/` and `docs/private/` is the only explicit skip of both;
  other `docs/` Markdown runs both, because Go tests read some of it (for
  example `docs/incidents/`).
  Everything else, including `.github/`, `.evolve/`, `.goreleaser.yml` and
  `install.sh`, runs both. Go tests read these inputs: `skills/` through
  `TestSkills_NoDrift`, `agents/` through the persona size budgets
  (`TestPersonaStopCriterionDedupe_*`), `.evolve/phases/` through
  `TestPhaseCatalog_*`, `.evolve/profiles/` through `TestSmoke_RealProfiles`
  and `TestRepoPersonaProfilePairing`, and the workflows through `ciparity`.
  The durable ACS predicates that read `docs/research/` Markdown run in
  `acs-durable`, which runs on every event.
- `CI required` runs with `if: always()` after all four. It passes only when
  routing and the `validate` call succeeded, `validate` and `acs-durable`
  each reported `success` through `ci.yml`'s outputs, and each optional suite
  either ran and succeeded or was deliberately unselected. A selected suite
  needs both its call result (`needs.go.result`, which fails when any matrix
  leg fails) and its inner job's `job.status` output, which is empty when the
  inner job was skipped. An unselected suite must be `skipped` with an empty
  output. A failed, cancelled, skipped or missing required job fails it
  (`TestRequiredResult_RejectsMissingOrUnsuccessfulWork`). Every other job in
  `required.yml` must be in its `needs`
  (`TestRequiredResult_WaitsForEveryOtherJob` derives the set from the YAML).

Through the reusable calls, the check names carry the caller's prefix:
`plugin and durable ACS / validate`, `plugin and durable ACS / acs-durable`,
`go / build + test (Go) (<os>, <go>)` and `landing / test landing module`.
The job `CI required` (not the workflow name `required CI`) is the stable
check name for a branch rule. Tools that read the CI verdict of a commit
(`releasepreflight`, `ciwatch`, the `/evo:publish` and `/evo:release` skills)
query `gh run list --workflow required.yml`, named once in Go as
`ciparity.RequiredWorkflow`; the newest run of any workflow can be a green
`landing-pages` run that hides a red `required CI`.

`ciparity`'s `required_workflow_test.go`, `routing_workflow_test.go` and the
integration-tagged
`required_result_integration_test.go` and `routing_workflow_integration_test.go`
pin the graph and execute the exact routing and result scripts from the YAML.
The design is
[test-ci-required-results-design-2026-09-14.md](../../docs/reports/test-ci-required-results-design-2026-09-14.md).

`release.yml` calls both reusable Go and general CI workflows on the tagged
revision, and publication depends on both succeeding. `landing-pages.yml`
(push to `main` and manual only) calls the shared read-only
`landing-validation.yml`, which tests, vets and renders the landing module and
uploads `landing-dist`; its build job deploys that artifact. Pull requests
validate the landing module through `required CI` and never deploy.

For test refactors, follow the [design and preservation protocol](../../docs/architecture/test-refactoring-design-2026-09-14.md):
new defect assertions fail first, existing behavior stays green, renamed cases
have a selector map, unchanged source retains hit-block coverage, and isolated
fault experiments verify that replacement assertions detect the same failures.
