# Test plan (Go-only)

> **Authoritative.** This document supersedes `docs/TEST_PLAN.md` (the Phase-1
> bash→Go parity plan). Once the Go-only consolidation lands, `docs/TEST_PLAN.md`
> is historical.

The Go binary is the only runtime. Tests are organized into **tiers** by scope
and cost, not by cycle number. Tests are named for the **behavior** they pin —
never `TestCycle<N>_*` / `TestC<N>_*`.

## Tier model

| Tier | Location | Scope | How to run |
|---|---|---|---|
| **Unit** (co-located) | `go/internal/<pkg>/*_test.go`, `go/cmd/<pkg>/*_test.go`, `go/pkg/<pkg>/*_test.go` | One package, exercised in-package (white-box). The bulk of coverage. | `go test ./internal/... ./cmd/... ./pkg/...` |
| **Integration** | `go/test/integration/` | Several `internal/` packages wired together against real temp-dir filesystem state; no CLI subagent spawned. | `go test ./test/integration/` |
| **E2E** | `go/test/e2e/` | A full cycle path through orchestrator/ship against fixture workspaces; may shell out to git. Heaviest, slowest. | `go test ./test/e2e/` |
| **Trust-kernel** | `go/test/trustkernel/` | Black-box pinning of the small set of safety invariants (ship gate, audit-binding, routing floor, transition legality, profile validity). Exercises real exported Go code. | `go test ./test/trustkernel/` |
| **Commit-gate** | `go/test/commitgate/` | `commit-gate-test.sh` exercises `commit-gate/commit-gate-runner.sh` over ephemeral repos (bash); enforcement at commit time is in `internal/phases/ship/commitgate_test.go`. | `bash go/test/commitgate/commit-gate-test.sh` |
| **Fixtures** | `go/test/fixtures/` | Shared builders (temp workspaces, sample `acs-verdict.json` / ledger / phase-plans) for the tiers above. No tests of its own. | n/a |

The co-located unit tests are the foundation and stay put: high cohesion with the
code they test, white-box access to unexported helpers. The `go/test/*` tiers are
black-box — they may call only **exported** APIs, which keeps them honest about
the public contract.

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

- `go test -race -count=1 -coverprofile=… $(go list ./... | grep -v '/acs/')` —
  unit + integration + e2e + trustkernel tiers, race detector on.
- `bash go/test/commitgate/commit-gate-test.sh` — commit-gate runner tier.
- Per-package `internal/*` coverage gate at ≥85%.
