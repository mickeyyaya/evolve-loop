# ADR-0021: Agent-bridge bash → Go port

> Status: **Accepted (implementation complete; production cutover gated)**. Supersedes the
> "v2 bridge effort" placeholder in `go/internal/adapters/bridge/bridge.go`.

## Context

The agent-bridge — the multi-CLI dispatch layer that launches subagents through `claude`, `codex`,
and `agy` (headless + tmux-REPL variants) — was ~3,000 lines of bash under `tools/agent-bridge/`
(`bin/bridge` + 6 drivers + 10 `lib/` modules + 6 JSON manifests). The only Go was a thin subprocess
wrapper (`adapters/bridge`) that shelled out to it. This ADR records the native-Go reimplementation:
same CLI surface, same behavior, refactored onto clean patterns, fully unit-testable.

## Decision

| Aspect | Choice |
|---|---|
| Wiring | In-process Go library `go/internal/bridge/` + `evolve bridge` CLI shim. The `core.Bridge` adapter calls the library directly (no fork) behind `EVOLVE_BRIDGE_GO`. |
| Driver model | **Strategy + self-registering Registry** (`driver.go`), replacing bash's `drv_launch_${cli//-/_}` name-mangled file dispatch. All 6 drivers. |
| Launch flow | **Template Method**: `LaunchArgs` (parse → profile → validate → resolve → mode → dispatch) and `runTmuxREPL` (spawn → boot-wait → deliver → artifact-wait → scrollback → exit) — one shared REPL engine for all 3 tmux drivers. |
| Testability | Injectable seams on `Deps`: `Runner` (CmdRunner), `Tmux` (TmuxController), `Sleep`, `LookupEnv`, `LookPath`, `Now`, `NewChallengeToken`, plus `randRead` + a `manifestSource` interface over the `go:embed` manifests. No real subprocess/tmux/clock needed in unit tests. |
| Cutover | Incremental behind `EVOLVE_BRIDGE_GO` (default **off** → bash). Flip + bash deletion gated on shadow-parity (below). |

## What was ported (complete, 100% covered)

- 6 drivers: `claude-p`, `codex`, `agy` (headless) + `claude-tmux`, `codex-tmux`, `agy-tmux` (REPL).
- Full launch pipeline: flag/env/profile precedence, profile loader, embedded manifests, probe
  (tier resolution), credential-isolation cost-leak guards, `--validate-only`/`--dry-run`/`--require-full`.
- Interactive auto-respond engine (manifest `interactive_prompts`, loop guard, escalation reports).
- `core.Bridge` (`Launch` + `Probe`) and the `evolve bridge` CLI: `launch|probe|report|add-rule|doctor|validate|version|help`.
- Adapter cutover seam (`EVOLVE_BRIDGE_GO` → in-process engine).
- Exit-code contract preserved exactly (0/2/3/10/80/81/85/86/99/127).

**Coverage:** `internal/bridge` and `adapters/bridge` are each 100.0% of statements (`go test -race`).

Auxiliary modules also ported (100% covered): `add-rule` (manifest-patcher, with a writable
override-dir layer since manifests are `go:embed`), `doctor` (binary + file-based auth + env-warnings
+ optional deep probe), `human-input` (keystroke-plausibility cadence, double-gated OFF as in bash),
and `billing-snapshot` (credential-isolation snapshot + compare).

## Documented simplifications (not 1:1 with bash)

| Item | Simplification |
|---|---|
| `doctor` / `billing` auth detection | File + env signals only; the macOS **Keychain** + `claude usage` + statsig branches are platform/exec-specific and not portably testable. The credentials-file fallback covers the common case. |
| `bridge selftest` | Replaced by `go test ./internal/bridge/...`. |
| `human-input` timing | Delays go through the `Sleep` seam (Gaussian magnitude is cosmetic); behavior/artifact identical to the default path. |

## GATED — production cutover (requires human sign-off, NOT autonomous)

The default remains bash because deleting the production path while a background loop may use it is
unsafe. The remaining steps, in order, each gated on the prior:

1. **Shadow-parity:** run both bridges on identical `BridgeRequest` fixtures; diff exit codes +
   artifact + report JSON until clean over a verification window.
2. **Flip default:** make `EVOLVE_BRIDGE_GO=1` the default in the adapter.
3. **Delete bash:** remove `tools/agent-bridge/`, retain the `bridge` entrypoint as a Go shim, and
   update the `CLAUDE.md` env-var table.

Until step 3, `bridge launch`/`probe`/etc. continue to resolve to bash for any out-of-process caller.

## Consequences

- The hardest-to-test component (the tmux REPL state machine) became the cleanest to test — the
  seam design turned 6s-flaky BATS into microsecond deterministic unit tests.
- ~600 lines of near-identical bash tmux drivers collapsed to one engine + three ~35-line specs.
- The bash and Go implementations coexist behind one env flag, so the cutover carries no big-bang risk.
