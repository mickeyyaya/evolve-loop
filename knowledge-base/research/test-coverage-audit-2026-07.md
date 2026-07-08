# Test & Code Coverage Audit — 2026-07-06

> Operator-requested review: "review the test coverage and code coverage — easy to maintain, strong and robust test plan/suites for each module."
> Method: full `go test -cover` sweep (176 pkgs), tagged re-runs (`integration acs`) on low scorers, per-function `go tool cover -func` on the gap zone, structural analysis of all 1,412 test files.

## Executive verdict

**Strong and unusually well-architected overall; the weak zone is the CLI-command layer, plus two systemic risks (tag-split coverage measurement, zero fuzzing).**

| Metric | Value | Verdict |
|---|---|---|
| Test:source ratio | 1,412 test files / 628 source (225K vs 115K LOC ≈ 2:1) | exceptional |
| Coverage distribution (n=170) | min 0%* · p10 87.5% · p25 92.9% · **median 96.6%** · p75 99.0% · max 100% | strong |
| Test failures in sweep | **0 / 170 packages** | green |
| Packages with no tests at all | 0 production packages (only test fixtures/harnesses) | green |
| Hermetic FS (`t.TempDir`) | 644 files | strong isolation |
| Parallel-safe (`t.Parallel`) | 276 files | good |
| Table-driven | 309 files | good |
| Fakes/DI injection | 252 files | strong (matches the repo's DI style) |
| Fuzz tests | **0** | gap |
| Property-based | 2 files (gc/rapid) | thin |
| `time.Sleep` in tests | 28 files | flake-risk watchlist |

*0% entries are cross-package-covered or test-only harnesses — see below.

## Layered suite architecture (the strength)

Six deliberate layers make the suites *robust* rather than merely large:

1. **Unit** — same-package `_test.go`, hermetic via `t.TempDir`, fake-injected via DI seams (`WithMaxPhaseIterations`, `shipRepinProvenanceFn`).
2. **Integration tier** — `//go:build integration`, 68 files; CI's "race + cover, incl. integration tier" lane.
3. **ACS-durable** — `//go:build acs`, 186 files; behavioral contracts surviving refactors; `-tags acs` in CI + cycle audit (ADR-0069).
4. **Per-cycle ACS suites** — `go/acs/cycle*/predicates_test.go`, 130 suites; every autonomous cycle ships its own durable predicates; regression accretes monotonically (append-only).
5. **Golden/fixture** — 11 `testdata/` dirs, 20 golden-file tests; real captured CLI panes (claude/codex/agy/ollama), routingeval scenario dirs.
6. **apicover gates (×2)** — every exported symbol needs same-package coverage AND a test naming it; enforced in CI + cycle audit. Untested public API is structurally impossible.

Maintainability: conventions consistent (AAA, table-driven, fakes over mocks, `TestXxx_Scenario` naming); apicover naming makes tests discoverable-by-symbol.

## Per-module coverage (tagged where relevant)

| Module | Coverage | Assessment |
|---|---|---|
| internal/core | ~93% (23.2K test LOC vs 11.1K src) | strong; orchestrator/chokepoint/recovery well-characterized |
| internal/bridge | ~92% (16.5K vs 7.8K) | strong; pane goldens for all 4 CLIs |
| internal/phases/ship | **47% plain → 90.6% tagged** | strong — but see R1 |
| internal/fleet | 96%+ | strong (partition/planner + new disjointness tests) |
| policy, config, triagecap, quotastate, fleetbudget, budgethistory | 92–100% | strong (recent TDD campaigns) |
| failure* family | 79–100% | good |
| **cmd/evolve** | **58.2%** (72 zero-covered funcs) | GAP — worktree cmds, swarm-reap, main wiring |
| **internal/cli/guardcmd** | **53.2%** | GAP — wiring + error paths |
| **internal/cli/opscmd** | **57.5%** | GAP |
| **cmd/evolve/cmdutil** | **53.6%** | GAP |
| **internal/commitgate** | **65.8%** | PARTIAL — non-Go lanes 0% (lanePython/laneNode/laneRust), writeAttestation 53.8% |
| bridge/clicontrol, ipcenv | 0% in-package | cross-package covered (usageprobe/core/storage); acceptable, attribution misleading |
| test/component, test/trustkernel | n/a | test-only harnesses |

## Risks (ranked)

- **R1 — Tag-split coverage measurement.** Plain `go test -cover` under-reports tagged packages by up to 43 points (ship 47%→90.6%). Any gate reading the PLAIN number (per-cycle coverage-gate, EGPS eval, ad-hoc audits) measures the wrong thing. → SSOT the tagged coverage command (ciparity extension, ADR-0069 pattern). **Inbox: coverage-gate-tag-parity (0.76).**
- **R2 — CLI-command layer soft belly.** guardcmd/opscmd/cmdutil/cmd_worktree/swarm-reap = the operator-facing incident-recovery path at ~50–58%. **Inbox: cli-command-layer-test-coverage (0.78).**
- **R3 — Zero fuzzing on parser surfaces.** Pane text, reset hints, quota panes, manifest/inbox JSON — the incident-generating class (trust-dialog regressions). **Inbox: fuzz-parser-surfaces (0.72), incl. rapid property tests for fleet.Partition.**
- **R4 — Sleep-based tests (28 files).** Latent flakes (macOS spill flake already known); fake-clock pattern exists, not universal. Opportunistic cleanup.
- **R5 — commitgate non-Go lanes untested.** Fixture-test or explicitly unsupport (folded into 0.78 item).
- **R6 — Giant test files.** 7 files >720 lines (max subagent/run_test.go 895). Split along scenario seams opportunistically.

## Bottom line

Median 96.6%, zero failures, layered architecture with structural API-coverage gates: **strong and maintainable**. To be robust in the adversarial sense: close the CLI-layer gap (0.78), make every coverage gate tag-aware (0.76), fuzz the parser surfaces (0.72). All three injected into the inbox 2026-07-06; triage arbitrates by weight.
