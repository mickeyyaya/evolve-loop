# Testing Strategy & Coverage Runbook

> Companion to [TEST_PLAN.md](../TEST_PLAN.md) (per-package coverage contracts). This file captures the **layer taxonomy**, the **review findings** that motivated the v12.x test-hardening effort, the **meaningful-floor coverage policy**, and the **runbook** for raising a package to the floor. Authored during the `feat/test-coverage-hardening` effort.

## Contents

- [Test layers in this codebase](#test-layers-in-this-codebase)
- [Coverage baseline & tiers](#coverage-baseline--tiers)
- [Review findings](#review-findings)
- [The 95% meaningful-floor policy](#the-95-meaningful-floor-policy)
- [Coverage runbook](#coverage-runbook)
- [Remaining campaign](#remaining-campaign)

## Test layers in this codebase

The user-facing request named four layers (unit/component/integration/e2e). "Component" is frontend/microservice terminology; in this Go backend it maps to **package-level tests**. The concrete mapping:

| Layer | Mechanism in this repo | Example |
|---|---|---|
| Unit | function/struct, table-driven, pure | `statefile_extra_test.go`, `guards/*_test.go` |
| Package / "component" | whole package via DI seams + fakes | `ship/native_test.go` (real-git harness), `bridge/driver_*_test.go` (`fakeTmux`) |
| Integration | `*_integration_test.go`, real `tmux` / `/bin/sh`, skips if unavailable | `bridge/tmux_repl_integration_test.go`, `verifyeval/*_integration_test.go` |
| E2E | live subprocess / CLI matrix in `cmd/evolve/` | `e2e_cycle_cli_matrix_test.go`, `cmd_loop_live_e2e_test.go` |

**Seam discipline** (the lever that makes layering testable): inject `Now`, `Env`, `Runner`/`CmdRunner`, `Tmux`, `Sleep`, `LookupEnv` as struct fields/func types; production wires the real impl, tests wire fakes. Never read the clock, env, or spawn a process without a seam.

## Coverage baseline & tiers

Measured at the start of the effort (CI-equivalent: `go test $(go list ./... | grep -v /acs/)`):

| Tier | Count | Action |
|---|---|---|
| ≥95% | 48 pkgs | hold (floor, not ceiling) |
| 85–94% | 16 pkgs | raise to floor (mechanical: error/edge branches) |
| <85% | 6 pkgs | priority: `phases/ship` 63%, `releasepipeline` 68%, `rollback` 77%, `cmd/evolve` 57%, `cmd/filter-stdout` 62% |

`acs/` (cycle predicate suites) are CI-excluded by design (read runtime artifacts absent in CI). `cmd/*` is excluded from the gate (thin CLI glue) per TEST_PLAN.md.

## Review findings

The review surfaced two **test-quality defects** masquerading as other problems — both fixed and committed:

| Finding | Root cause | Smell (testing-patterns) | Fix |
|---|---|---|---|
| `guards` suite red, denial tests "fail open" | tests read ambient `EVOLVE_BYPASS_*` via `os.Getenv`; operator exports them → every `Decide` short-circuits to Allow | Local Hero (#6); breaks F.I.R.S.T. Isolated/Repeatable | hermetic `TestMain` neutralizes all 5 bypass vars + `TestGuardsSuiteIsHermetic` invariant guard |
| `bridge/TestRealTmux_ArtifactTimeout` flaky under load | one `perTick` scales both boot-wait (60 polls) and artifact-wait; 600ms boot budget raced real-tmux boot → wrong exit code | arbitrary-timeout / condition-based-waiting | decouple: generous boot budget (9s) + short `ArtifactTimeoutS` |

**Lesson:** a test that fails for the wrong reason (ambient env, timing) is as dangerous as no test — it can also *pass* for the wrong reason and hide a real regression. Hermeticity first, then coverage.

## The 95% meaningful-floor policy

Target is **≥95% per-package statement coverage via behavior-probing tests** — not literal 100%. Rationale: the `testing-patterns` skill and `docs/TEST_PLAN.md` both reject 100%-as-goal ("produces tests that touch lines, not verify behavior"). Every added test must be able to fail for a real reason.

**Documented exclusion zone** (acceptable to leave uncovered, annotate in-package):

| Pattern | Why excluded | Example |
|---|---|---|
| Interactive TTY reads | needs a real PTY char-device; not worth the dep | `verifyManualConfirm` scanner (`verify.go:203-213`) |
| Filesystem-syscall errors after success | `tmp.Sync`/`Close`/`Rename` failures need fault-injecting FS | `writeStateMap` tail (`statefile.go:66-78`) |
| `os.Executable()` / `os.Getenv` fallbacks | environment-dependent, defensive | `repinPostCycle` exe fallback |

Closing these costs more (PTY harness, FS fault libs) than the regression risk they retire. Prefer a per-package gate with a justified exception list over chasing the last 3%.

## Coverage runbook

The repeatable loop, demonstrated raising `phases/ship` 63%→82.5% (`worktree_test.go`, `release_extra_test.go`, `dispatch_test.go`, `error_paths_test.go`, plus state/audit extras):

1. **Profile**: `go test -coverprofile=/tmp/p.cov ./internal/<pkg>/ && go tool cover -func=/tmp/p.cov | awk '$3!="100.0%"'`.
2. **Rank by uncovered behavior, not line count**: 0% functions on the critical path first (e.g. `shipFromWorktree`, `writeShipBinding`).
3. **Read the function + its seams** before writing the test (Rule 8). Identify how to drive each branch: real harness, `Runner` fault-injection, direct white-box call.
4. **Write behavior-probing tests** (one behavior per test, AAA). For error branches, inject faults via the seam (`faultRunner` delegates to real git but fails one subcommand).
5. **Verify with the operator's real env set** (`-race -count=1`) so hermeticity holds; confirm the % moved.
6. **Commit per concern** with `tests: <pkg> N/N PASS (race), no regression` and the before→after %.

## Remaining campaign

1. Finish `phases/ship` dry-run-path cluster (→ ~90%+; the floor minus the exclusion zone).
2. Apply the runbook to `releasepipeline`, `rollback`, then the 16 packages at 85–94% (parallelizable per-package — independent worktrees).
3. **Enforce the gate**: flip `.github/workflows/go.yml` coverage step from warning-only (`exit 0`) to failing at the per-package floor, with the exclusion list above.
4. **Mutation testing** on critical packages (`ship`, `guards`, `rollback`, `ledger`) to catch "The Liar" tests — reuse the existing `mutate-eval.sh` discipline; kill-rate < 0.8 flags a tautological test.
