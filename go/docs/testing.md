# Test plan (Go-only)

> **Authoritative.** This document supersedes `docs/TEST_PLAN.md` (the Phase-1
> bash→Go parity plan). Once the Go-only consolidation lands, `docs/TEST_PLAN.md`
> is historical.

The Go binary is the only runtime. Tests are named for the **behavior** they pin
— never `TestCycle<N>_*` / `TestC<N>_*`. They are organized along **two
independent axes**:

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
| **fast** (default) | _(none)_ | All co-located tests NOT tagged integration/e2e + `test/component`. Sub-10s. | `make test` |
| **integration** | `//go:build integration` | Real FS / git / tmux subprocess tests + `test/integration`. | `make test-integration` |
| **e2e** | `//go:build e2e` | Full-cycle subprocess paths (`cmd/evolve/e2e_*`) + `test/e2e`. Live sub-tier self-skips without `EVOLVE_E2E_LIVE`. | `make test-e2e` |
| **everything** | both | Fast + integration + e2e (what CI runs). | `make test-all` |

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
| **Commit-gate** | `go/test/commitgate/` | `commit-gate-test.sh` over ephemeral repos (bash). | `bash go/test/commitgate/commit-gate-test.sh` |

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

The cost-axis split cut the default suite from **~206s to ~42s** (no race;
`make test` with `-race` ≈ 46s) by moving `cmd/evolve`'s full-cycle e2e
(205s→3.6s) and `internal/bridge`'s tmux/live tests (47s→0.47s) behind tags.

The remaining poles are **deliberately left fast-resident**:

- **`internal/phases/ship` (~22s)** — its tests do real `git` commit/merge in
  temp repos (integration-*cost*), but they are ship's primary safety coverage
  (audit-binding, gates). They stay in the fast suite so a developer can't break
  ship without immediate feedback. They are **not** parallelized: ship reads
  control-flow env via `t.Setenv`, which is incompatible with `t.Parallel`.
- **`internal/core` (~12s)** — volume (≈200 small orchestrator-sequencing
  tests). It overlaps under ship's wall, so parallelizing it would not lower the
  suite's wall time; the churn/flake risk isn't worth a no-op gain.

This is an intentional trade (determinism + safety coverage over shaving a
ship-bound wall), not an oversight. Reducing ship further means faking `git` —
a separate, larger refactor tracked outside this workstream.

### Known: real-tmux integration tests are load-sensitive

The `internal/bridge` `TestRealTmux_*` integration tests (`//go:build
integration`) drive a real `tmux` session and poll for a REPL prompt. They pass
reliably **in isolation** (`make test-integration`, or `go test -race -tags
integration ./internal/bridge/`), but can flake (`exit 80`, "REPL prompt never
appeared") inside a single `make test-all` invocation on a high-core machine,
where ~100 packages run concurrently (`go test -p`) and starve the prompt
detection. This is **pre-existing** (the test bodies predate the cost-axis split
and are unchanged) and **CI-neutral** (CI runners' lower parallelism tolerates
it, as they did before tagging). Triage a tmux failure by re-running
`make test-integration` alone before suspecting a code change. Hardening these
tests against load (retry / longer prompt timeout) is tracked as bridge follow-up.

### Where the legacy ACS predicates live

`go/acs/cycle*/predicates_test.go` (the cycle-pegged `TestC<N>_*` Go ports of the
bash EGPS predicates) and `acs/regression-suite/` (deprecated `.sh`) are **not yet
deleted** — they run in the live EGPS suite and are excluded from the unit gate
(`go list ./... | grep -v '/acs/'`) because they read runtime artifacts. Their
durable invariants are being ported into `go/test/trustkernel/`; the cruft is
retired at Stage 5. See `go/test/trustkernel/PORTING-LEDGER.md`.

## Coverage targets

- **Floor: ≥85% per `internal/*` package**, enforced in CI (`.github/workflows/go.yml`,
  "coverage gate" step).
- **Intent over surface (AGENTS.md Rule 9).** A test must probe the *behavior under
  change*, not merely re-walk lines for a coverage number. A passing test that
  would still pass if the invariant were broken is a no-op and is rejected in review.
- Coverage is a floor, not a goal. Trust-kernel pinning tests and behavioral
  integration tests carry more signal than line-chasing.

## Test-design conventions

1. **AAA** — Arrange, Act, Assert. Keep the three phases visually distinct.
2. **Behavior-naming** — `TestShipGate_BlocksWhenRedCountNonZero`, not
   `TestC102_003_*`. The name states the invariant and the condition. **No
   cycle-pegging** — a test name must never encode a cycle number; cycle context
   belongs in git history, not the permanent test surface.
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

- `go test -race -count=1 -tags integration -coverprofile=… $(go list ./... | grep -v '/acs/')`
  — fast + integration tiers + trustkernel, race detector on, coverage captured.
- `go test -count=1 -tags e2e ./cmd/... ./test/e2e/...` — e2e tier, no race
  (subprocess-heavy); live sub-tier self-skips without `EVOLVE_E2E_LIVE`.
- `bash go/test/commitgate/commit-gate-test.sh` — commit-gate runner tier.
- Per-package `internal/*` coverage gate at ≥85% (computed on the integration run).
