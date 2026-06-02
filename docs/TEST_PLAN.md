# Go-port test plan — Phase 1

> Test discipline for the Go rewrite under `go/`. Documents the contracts each package tests, the ACS predicate coexistence strategy with the bash originals, and the gaps deferred to later phases.
>
> Companion to the parent plan at `~/.claude/plans/this-is-a-big-parallel-locket.md` §3 and the Phase 1 sub-plan at `~/.claude/plans/drifting-drifting-otter.md`.

---

## Goals

1. Lock the **behavior** of every ported subsystem before Phase 2 wires it into the orchestrator — bash-Go drift gets caught in the test suite, not in production.
2. Run the test suite in <30 seconds on a developer laptop, in <2 minutes under `-race`.
3. Hold ≥95% per-package statement coverage (operator-mandated mid-session, raised from parent §3's 85%).
4. Treat coverage as the floor, not the ceiling — the suite exists to verify *intent*, not to game `go test -cover`.

`cmd/evolve/*` is excluded from the 95% gate (parent §8 risk #6): the CLI layer is thin glue over the internal packages, smoke-tested via `dispatch_test.go`.

---

## Coverage snapshot (cycle ending 2026-05-22 — c729307…f50ac0b…82b66aa)

| Package | Coverage | Test surface |
|---|---|---|
| `pkg/version` | 100.0% | ldflag stamp + `runtime/debug.BuildInfo` composition |
| `pkg/acsassert` | 96.6% | 6 helpers (`FileExists`, `FileContains`, `FileMatchesRegex`, `JSONFieldEquals`, `SubprocessOutput`, `AllOf`) + `SetupTempProject` |
| `internal/log` | 97.1% | sidecar `abnormal-events.jsonl` writer + slog phase emitter |
| `internal/projecthash` | 100.0% | byte-exact port of `printf %s "$PWD" \| shasum -a 256 \| head -c 8` |
| `internal/core` | 97.5% | sentinels, `Phase` JSON round-trip, state-machine transition table, orchestrator sequencer (fake ports) |
| `internal/adapters/storage` | 99.1% | `state.json` / `cycle-state.json` round-trip, atomic write, `flock` + same-process mutex |
| `internal/adapters/ledger` | 96.7% | append + SHA256 chain, tip file, verify (intact / truncated / mutated), iter |
| `internal/guards` | 97.0% | ship / phase / role / docdelete / quota / chain — per-rule table tests |
| `internal/acsrunner` | 98.8% | `go test -json` stream parser + atomic verdict write |
| `internal/doctor` | 98.2% | probe-tool port: `exec.LookPath` then `/usr/local/bin` → `/opt/homebrew/bin` → `~/.local/bin` → `~/bin` → `/usr/bin` |
| `cmd/evolve` | 79.5% | dispatch smoke tests for version / help / doctor / guard / ledger / acs subcommands (excluded from gate) |
| `acs/cycle42` | 100% | 4 file-grep predicates against `docs/architecture/token-reduction-roadmap.md` + KB dossier |
| `acs/cycle104` | 100% | 5 file-grep + regex + line-position invariants against `agents/*.md` and `docs/architecture/control-flags.md` |
| `acs/cycle53` | (no statements) | 3 integration tests shelling to `legacy/scripts/dispatch/subagent-run.sh --validate-profile` with sentinel adapters and dispatch-plan JSON checks |

12 of the 12 bash predicates in `acs/cycle-{42,53,104}/*.sh` are ported. Both bash and Go counterparts coexist (parent §4 Phase 4 cutover discipline).

---

## What each package's test suite **must** verify

### `pkg/version`
- Dev build (no ldflag) returns a build-info string that includes the module path.
- Release build (`-ldflags='-X .../pkg/version.Version=x.y.z'`) returns the supplied version.
- `shortSHA` strips to 7 chars and tolerates an empty input.

### `pkg/acsassert`
- Each helper has a **happy** path and at least one **sad** path that calls `Errorf` on the `TB`.
- `AllOf` short-circuits on the first false (verified via a counter-incrementing predicate).
- `SubprocessOutput` distinguishes "not found on PATH" (`ErrSubprocessNotFound`) from "ran with exit code N".
- `JSONFieldEquals` survives non-comparable result types (maps) via `reflect.DeepEqual`.

### `internal/log`
- `EmitAbnormal` writes one JSON line per event; cumulative tail is preserved.
- `EmitPhase` shapes the event correctly for the slog handler.
- Concurrent writers do not interleave bytes (test under `-race`).

### `internal/projecthash`
- Three golden vectors (different `$PWD` strings) produce SHA256[0:8] byte-equal to the canonical bash command.
- Empty input still returns 8 characters.

### `internal/core`
- `Phase` JSON round-trip: every field of `PhaseRequest` / `PhaseResponse` survives marshal+unmarshal.
- `CanTransition` accepts all valid edges and rejects every other pair (full transition-table table test).
- `Orchestrator` invokes guard hooks before each phase and appends a ledger entry per phase (verified via fake ports).

### `internal/adapters/storage`
- `WriteState` is atomic: a partial write (mid-stream `errors.New`) does not corrupt the on-disk file.
- `AcquireLock` blocks a second goroutine in the same process (mutex) AND a second OS process (flock); the test exercises both.
- Missing files → zero-value structs (no error) — matches bash `cycle-state.sh` bootstrap behaviour.

### `internal/adapters/ledger`
- First entry's `prev_hash` is the 64-zero seed; `entry_seq=0`.
- Subsequent entries chain — recomputed SHA256 of the previous line bytes matches.
- `Verify` returns `ErrLedgerChainBroken` on: prev_hash mismatch, duplicate prev_hash (fan-out anomaly), tip mismatch.
- Iteration yields entries in append order and is safe to call concurrently with read-only access.

### `internal/guards`
For every guard:
- The bypass env var (`EVOLVE_BYPASS_*`) returns Allow=true.
- The "out of scope" tool name (e.g. `Edit` for `ship` guard) returns Allow=true.
- The targeted deny rule fires with the bash-canonical reason text.
- A test that exercises the env-var bypass to keep that branch covered.

Per-guard specifics:
- **ship**: canonical script path (`legacy/scripts/lifecycle/ship.sh`) allowlists every ship-verb (`git commit`, `git push`, `gh release create|edit`).
- **phase**: denies the `Agent` tool when `cycle-state.json:cycle_id != 0`; allows otherwise.
- **role**: per-phase write allowlist (build: workspace + worktree; audit: workspace only); always-safe paths (`/tmp/`, `$HOME/.claude/`).
- **docdelete**: blocks `rm` against `docs/` or `knowledge-base/`; blocks `mv` from `docs/` unless dest is `knowledge-base/research/archived-YYYY-MM-DD/`.
- **quota**: per-agent + per-bucket counter, resets per process (Phase 1 in-memory); Phase 2 persists into `cycle-state.json:research_usage`.
- **chain**: thin wrapper over `ledger.Verify`; missing ledger reference returns Allow=false with a deterministic reason.

### `internal/acsrunner`
- `ParseTestJSON` ignores package-level (no-`Test`) events.
- Garbage lines (build-output preamble) are skipped without aborting the parse.
- `RedCount` matches the count of `fail` actions across distinct test names.
- `WriteVerdict` is atomic (tmp+rename); the verdict path is `runs/cycle-<N>/acs-verdict.json`.
- Hooks (`marshal`, `write`, `close`, `rename`) drive marshal+write+rename failure branches deterministically.

### `internal/doctor`
- `Probe` finds via `exec.LookPath` first; falls back to the documented 5 explicit paths.
- The `Checked` log enumerates **every** location attempted (operator-readable diagnostic trail).
- `home-dir error` does not crash — the home-prefixed candidates are skipped, system paths still walked.
- `EmitJSON` emits `path: null` (not `path: ""`) when `Found=false`; the marshal failure branch is hook-driven.

### `cmd/evolve` (smoke layer)
- `dispatch` returns the documented exit codes (0 success, 1 not-found, 2 deny, 10 usage error).
- `reorderArgs` accepts flag-after-positional invocations (operator muscle memory from bash predicates).
- Every subcommand reaches its respective `runX` entry point — verified by injecting `bytes.Buffer` writers and asserting payload shape.

### ACS predicate ports

Each `acs/cycleN/predicates_test.go` asserts the same invariants as the corresponding `acs/cycle-N/*.sh` predicate. The test name encodes the AC-ID (`TestC42_003_PNew13Done`, `TestC104_001_OrchestratorDefaultAdvisory`, …) so a verdict mismatch between bash and Go is immediately traceable.

Predicates that need subprocess scaffolding (cycle-53) shell to `legacy/scripts/dispatch/subagent-run.sh --validate-profile` with the same `EVOLVE_TESTING=1` + sentinel-adapter pattern the bash predicates use. Anti-tautology checks (e.g. claude has no `[adapter-cap] WARN`) are preserved in the Go ports.

---

## Coexistence strategy

Both bash and Go ACS predicates run during the coexistence period (parent §4 Phase 4):

```
bash:  bash acs/cycle-104/001-orchestrator-default-advisory.sh
go:    go test ./acs/cycle104/... -run TestC104_001_OrchestratorDefaultAdvisory
       evolve acs run --cycle 104 ./acs/cycle104/...
```

Any divergence between the two verdicts is an integrity bug. The retrospective hook (`legacy/scripts/observability/replay-egps.sh`) currently consumes the bash verdicts; once cycle 105+ rewires it to consume `evolve acs run`'s output, the bash files can be removed package-by-package.

---

## Race detection + property tests

Race detector is non-negotiable for `go test ./internal/adapters/storage/...` (concurrent `flock` + mutex) and `go test ./internal/adapters/ledger/...` (concurrent append).

Property testing via `pgregory.net/rapid` is wired for the ledger chain: 10K shrink iterations against the chain-integrity invariant. No counterexample has been found; the test file lives at `internal/adapters/ledger/ledger_test.go` (search for `rapid.T`).

---

## How to run

```
cd go
go build ./...                                              # 1. compile
go vet ./...                                                # 2. static analysis
go test ./... -race -coverprofile=/tmp/c.out               # 3. unit + race
go tool cover -func=/tmp/c.out | tail -1                    # 4. headline coverage
evolve doctor probe go                                      # 5. probe smoke
evolve ledger verify --evolve-dir .evolve                   # 6. ledger smoke
evolve acs run --cycle 104 ./acs/cycle104/...              # 7. predicate smoke
```

Steps 1-4 are non-negotiable per CLAUDE.md "Verification before claiming done". Steps 5-7 are end-to-end smoke that any subagent or operator can run before declaring a cycle done.

---

## Known parity findings (cycle ending 2026-05-22)

| Finding | Status |
|---|---|
| **Soft-start boundary** | `ledger.Verify` now matches bash semantics — pre-v8.37 entries (no `prev_hash` field) are skipped from chain validation but their SHA still anchors the first v8.37+ entry. Added in cycle ending 2026-05-22 (`decodeLedgerLine` + raw map key check). |
| **Live ledger has 18 chain breaks** | Both `bash legacy/scripts/observability/verify-ledger-chain.sh` AND `evolve ledger verify --evolve-dir .evolve` report 18 chain breaks on the live `.evolve/ledger.jsonl` (1846 entries, first break at entry_seq=0 in cycle 25). This is operational (likely concurrent fan-out anomalies from earlier cycles), not a port bug. Tracked separately. |
| **Verification step §6.5 not yet satisfiable** | Parent plan §6 item 5 — "`evolve ledger verify` returns exit 0 on the live repo ledger" — is blocked on the operational finding above. The verifier itself is correct (proved by `TestVerify_SoftBoundary_*` table). |

---

## Phase 2 — CLI scenario coverage (added 2026-05-30)

End-to-end coverage of the "any CLI × any phase" invariant and the adversarial
pipeline paths, layered on the `evolve-fake-cli` seam (`BRIDGE_TESTING=1` +
`BRIDGE_<CLI>_BINARY`). Offline-deterministic by default; live opt-in via
`EVOLVE_E2E_LIVE=1`. All E2E tests skip under `-short`.

| Test (`go/cmd/evolve/`) | Scenario | Drivers / mechanism |
|---|---|---|
| `e2e_cycle_cli_matrix_test.go` (pre-existing) | Happy-path full cycle scout→ship | headless: claude-p, codex, agy |
| `e2e_cli_tmux_matrix_test.go` | Happy-path full cycle through a **real tmux server** | interactive: claude-tmux, codex-tmux, agy-tmux. Fake serves a REPL printing every driver's boot marker; `HOME`/`EVOLVE_CODEX_CONFIG_PATH` redirected so preflights never touch the operator's real `~/.codex`. ~45s/CLI; timeout tunable via `EVOLVE_E2E_TMUX_TIMEOUT_S`. |
| `e2e_cli_degradation_test.go` | CLI fallback chain (ADR-0029) | Primary claude-p fails with each trigger code {80,81,124,127} → runner retries codex → ships. Non-trigger 99 → no fallback → no ship. Per-CLI exit injection via `FAKE_CLI_CLAUDE_EXIT`. |
| `e2e_pipeline_paths_test.go` | Audit FAIL→retro→no-ship; WARN fluent-ships vs strict-blocks; intent phase | headless; `FAKE_CLI_AUDIT_VERDICT`, `EVOLVE_STRICT_AUDIT`, `EVOLVE_REQUIRE_INTENT`. |
| `e2e_setup_validation_test.go` | `evolve setup detect/validate` floor | in-process `runSetup`: envelope (ERROR), allowed_clis (ERROR), cross-family (WARN / `--strict` ERROR), detect JSON shape. |

`evolve-fake-cli` gained a persistent REPL mode (auto-detected from a tmux launch:
no `-p`, no `exec`), per-CLI exit injection, and audit-verdict injection — all
backward-compatible. It also picked up a correctness fix: the tdd artifact is
`test-report.md`, not the stale `team-context.md` (the headless matrix masked this
because its completion is process-exit-based; the tmux artifact-poll exposed it).

**Documented gap — bash-adapter graceful degradation.** The bash adapters'
"missing binary → stub artifact → exit 0" degradation does NOT exist in the v11+
Go path. The Go path returns a fallback-trigger exit code (127 `ExitMissingBinary`)
and relies on the fallback chain; the `--require-full` hard-fail (exit 99) is
covered at the bridge level (`internal/bridge/launch_modes_test.go:TestLaunchArgs_RequireFull_Unmet`,
`coverage_batch2_test.go:TestRequireFull_ManifestMissing`). No E2E asserts the
bash stub behavior, because it is unreachable through `evolve cycle run`.

**Not yet covered (later pass):** live-injection (ADR-0023 send/nudge/keystroke),
recipe engine + capability catalog (ADR-0031), and system-prompt/interactive-policy
injection *content* — all have bridge-level unit coverage; full-cycle E2E is deferred.
The `ollama-tmux` driver (local inference) is out of provider-matrix scope.

---

## Gaps (Phase 2 owns these)

| Surface | Reason deferred |
|---|---|
| `internal/adapters/bridge/` | Now covered (v12): delegates to the in-process Go `bridge.Engine` (no subprocess); unit-tested with a fake `core.Bridge`. |
| `internal/phases/*` | Unit isolation still TBD, but every phase is now exercised end-to-end through the CLI-scenario E2E matrix above (headless + tmux + fallback + FAIL/retro). |
| `internal/adapters/sandbox/` | Phase 2 — `sandbox-exec` (macOS) and `bwrap` (Linux) profiles. Tests need OS-specific build tags. |
| `evolve loop` / `evolve cycle run` | `evolve cycle run` now covered E2E across all 6 shipped drivers + fallback + adversarial paths (see Phase 2 section). `evolve loop` multi-cycle + state-machine property tests still TBD. |
| Cross-CLI parity (Gemini / AGY) | agy-tmux now runs a full cycle in the tmux matrix; the cli/model floor (envelope + allowed_clis) is enforced via `policy.ValidatePin` and surfaced by `evolve setup detect` (`pin_violation`). Model-router unit tests under `internal/router/` still Phase 3. |
| Distribution | Phase 3 — GoReleaser config, brew formula smoke test, install.sh idempotency test. |
| v11.0 cutover | Phase 4 — final coexistence sunset; bash predicates removed after `evolve acs run` consumes their verdict surface for one full release. |

---

## Owners

- Phase 1 (this doc): single-author work, branch `go-rewrite-phase-1`.
- Phase 2+: TBD — the test-plan section per subsystem will be filled in by the author of that subsystem before its first GREEN commit.
