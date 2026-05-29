# Retired regression predicates — v12 flag-day casualties

> These 83 ACS regression predicates were retired (moved here, **not deleted**) because every one invokes bash scripts that were removed in the **v12.0.0 flag day** (~220 bash scripts deleted when the pipeline was ported to Go; see `project_v12_prep` / commit `a5ee915`). They are **permanently unsatisfiable false-REDs**: the behaviour they once guarded now lives in Go, but the predicate still shells out to a `scripts/*.sh` path that no longer exists, so it can never go GREEN by any present or future code change.
>
> Retiring them is **v12 migration cleanup, not gate-gaming.** Files are preserved here as history; the behaviours are re-covered by Go unit tests (mapping below). The ACS suite globs `acs/regression-suite/cycle-*/` — moving a predicate into `_retired-v12/` excludes it from the live suite while keeping it auditable. See `go/internal/acssuite/acssuite.go`.

## Why these are false-REDs (evidence)

| Check | Result |
|---|---|
| Predicates **directly** referencing a `scripts/*.sh` path | 82 / 83 |
| Those referenced scripts present on disk | 0 (all MISSING — deleted in v12) |
| The 1 indirect predicate (`cycle-86/pred-test-lint-acs-passes.sh`) | invokes `tests/verification/test-lint-acs-predicates.sh` (which **does** still exist), but that harness in turn shells out to `scripts/verification/lint-acs-predicates.sh` — **deleted in v12** → every sub-test exits 127 → "0/11 passed" → predicate's `grep "all tests passed"` fails → RED. Transitively, the same v12 deletion. |

Verify at any time:

```bash
for f in acs/regression-suite/_retired-v12/cycle-*/*.sh; do
  grep -hoE '(legacy/)?scripts/[a-zA-Z0-9/_.-]+\.(sh|bats)' "$f"
done | sort -u | while read p; do [ -e "$p" ] || echo "MISSING $p"; done
```

## Behaviour-coverage map (no real coverage lost)

The guarded behaviours survived the Go port and are covered by Go unit tests:

| Retired predicate family | Behaviour now covered by |
|---|---|
| CLI-adapter capability / dispatch (`scripts/cli_adapters/*.sh`) | `go/internal/bridge/*_test.go` (manifest, model-clamp, driver tests) |
| Phase ordering / watchdog / dispatch (`scripts/dispatch/*.sh`) | `go/internal/core/orchestrator_test.go`, `go/internal/router/*_test.go` |
| Research quota / phase gates (`scripts/research/*`, gate scripts) | `go/internal/guards/quota_test.go`, `go/internal/guards/phase_test.go` |
| Subagent run / profile enforcement (`scripts/dispatch/subagent-run.sh`) | `go/internal/subagent/*_test.go` |
| ACS predicate lint harness (`tests/verification/test-lint-acs-predicates.sh`) | `evolve eval quality-check` / `evolve eval diversity-check` (Go) |

## Reinstatement

If a behaviour above is found to be genuinely uncovered, port the predicate to assert against the **Go** surface (binary subcommand or Go test) and move it back under `acs/regression-suite/cycle-<N>/`. Do not reinstate a predicate that still shells out to a deleted `scripts/*.sh` path.
