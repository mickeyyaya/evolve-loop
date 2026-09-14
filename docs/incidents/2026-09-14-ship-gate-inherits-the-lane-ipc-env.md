# 2026-09-14 — the ship repo-contract gate ran `go test` in the lane's IPC environment

**Class:** pipeline (false RED at ship; a lane's own change was green).
**Surface:** `internal/phases/ship/repocontract.go` (the pack runner every gate layer shares).
**Found by:** wave 2 of the pipeline-health verification, Signal Center line
`SHIP_REPO_CONTRACT_GATE cycle=1677 … importer backstop RED in the lane worktree`.

## What happened

Lane 1677 (task `ledger-verify-seal-anchor`; changed `cmd/evolve/cmd_ledger.go` and
`internal/adapters/ledger`) reached ship at 17:38. The repo-contract gate's scanner pack
and both added-test backstops were green. The importer backstop (29 targets, 1m59s)
went RED on twenty-odd tests across four packages — `cmd/evolve`, `internal/guards`,
`internal/phases/ship`, `internal/core` — every one of them env-sensitive:

- `TestRunCycleReset_LeaseFencing/*` and `TestRunCycleReset_RelativeProjectRootAbsolutized`
  refused with a `workspace_path` that belonged to a DIFFERENT test's temp dir
  (`TestRunGuard_RoleNormalSourceEdit_NoAlarm…/.evolve/runs/cycle-20`);
- `TestRole_OutsideCyclePasses` and its siblings were told they were INSIDE a cycle whose
  workspace was, again, another test's fixture (`TestRole_BypassAllows…/cycle-1`);
- `TestWorktreeShipIntegrate_GoldenArgvLogsAndErrors`'s `fleet-off` rows produced
  `GIT_FLEET_REBASE_NEEDED` instead of the golden `GIT_FF_MERGE_*`;
- `TestSealCycle_*`, `TestLoadResumeState_*`, `TestAutosealStaleMarker_*` in core likewise.

The same tests, run in the same lane worktree from the console with
`env -i PATH HOME GOTOOLCHAIN=auto go test ./cmd/evolve/`, pass.

## Root cause

A fleet lane's process carries its runtime state in the environment: the supervisor
launches it with `EVOLVE_FLEET=1` (plus scope and width), and `cyclerun.go` sets
`EVOLVE_CYCLE_STATE_FILE=<run dir>/cycle-state.json` process-wide (`os.Setenv`) so the
orchestrator and its guard subprocesses agree on THIS lane's phase. The gate's pack
runner built its `go test` with `exec.CommandContext` and never set `cmd.Env`, so the
child inherited both. Under `EVOLVE_CYCLE_STATE_FILE` every test that reads or writes
"its own" cycle state resolved to ONE file, so parallel tests read each other's
fixtures; under `EVOLVE_FLEET=1` the ship's fleet path engaged in a test that had
declared fleet off.

The leak is older than the wave. It never bit because, until #612, the gate ran only
four scanner packages (`phasespec`, `profiles`, `phasecoherence`, `routingtest`), none of
which reads the environment. #612's importer backstop runs the packages that import
what a lane changed — `cmd/evolve`, `core`, `guards`, `ship` on the first lane that
touched the ledger adapter — and those do.

The repository already knew the shape. `internal/core` carried a private `sanitizeEnv`
(prefix-strip `EVOLVE_*`) in front of the phase-bindings self-check, the build floor's
`go test`/`go list`/`go tool cover`, and the task contract's `go test -list`, with a
comment citing the same symptom in `internal/bridge` (fleet-mode worktree guard, exit
10). `ciparitygate` keeps an allowlist for the integration tier. Three scrubbers, one
gap: the ship gate.

## Fix

`ipcenv.Scrub(environ)` — the package that OWNS the IPC keys projects the scrub: drop
every `EVOLVE_`-namespaced entry, keep everything else in order, inspect only the key.
`TestScrub_CoversEveryIPCKey` pins that every exported key lives in the namespace, so a
key added tomorrow is scrubbed the day it is added. The ship pack runner (shared by the
scanner pack, the added-test backstops and the importer backstop) now runs `go test`
with `ipcenv.Scrub(os.Environ())`; core's four sites call the same function and the
private copy is gone. `ciparitygate`'s allowlist stays — CI-shell parity is a
deliberately stricter contract for the integration tier.

Red first: `TestRunRepoContractPackages_ScrubsTheLaneIPCEnvFromGoTest` sets both keys
with `t.Setenv`, runs the real runner against a throwaway module whose one test fails
when either key is present, and failed with `failed=[envprobe.TestNoLaneIPCEnvLeaks]`
before the change.

## Cost and blast radius

- Lane 1677's ship aborted → `recovering via audit (attempt 1/2)`. Under the running
  plane the second ship hits the same RED, so the recovery audit and ship are spent
  tokens; the plane binary is never rebuilt mid-batch, the fix lands at the wave boundary.
- Every wave-2 lane whose change is imported by `cmd/evolve`, `core`, `guards` or `ship`
  is exposed the same way until the plane is reconciled.
- `not_verified`: whether the leaked tests transiently overwrote lane 1677's live
  `cycle-state.json`. After the run the file was intact (its own `workspace_path`, no
  fixture paths, mtime 17:40 — after the gate); the orchestrator rewrites it from memory
  at each phase transition, which would mask a transient clobber. A guard subprocess
  reading during the gate's two minutes could have seen a fixture's phase.

## Operator notes

- A ship RED whose failing tests are all about "outside a cycle", lease fencing, seal
  role or fleet-off, and which pass under `env -i PATH HOME`, is this class — an
  environment leak, not the lane's change.
- Any NEW `go test`/`go run` a cycle spawns to judge the repository must take
  `ipcenv.Scrub(os.Environ())`; the four core sites and the ship runner are the examples.

## Follow-ups

- Two scrub policies remain (`ipcenv.Scrub` prefix-strip; `ciparitygate` allowlist).
  A later slice can express the allowlist as a projection of the same package if the
  integration tier ever needs the namespace rule too.
- `EVOLVE_TMUX_SOCKET` is declared in `internal/bridge` (`TmuxSocketEnv`), not in
  `ipcenv`; it is inside the namespace and therefore scrubbed, but the key lives outside
  the SSOT that names the namespace.
- Fixed here as well: the pack runner pointed the child's stderr at the same
  `io.Writer` as its tee (`cmd.Stderr = out` and `classifyPackEvents(stdout, out)`).
  For the ship's `*os.File` scan log exec hands the fd to the child and nothing is
  lost; for an in-memory writer it is a data race (`-race` reports it) and
  `bytes.Buffer.ReadFrom` — what `io.Copy` picks for the stderr goroutine — re-slices
  to the length it captured before its blocking read, dropping every concurrent tee
  write at EOF. The runner now wraps `out` in a `lockedWriter` (Write only, so
  `io.Copy` takes the lock per chunk); the new test pins both the scrub and the tee.

## Second site (later the same day): the audit's CI-parity apicover step

Every wave-2 audit raised `AUDIT_CIPARITY_GATE_STEP_FAILED` (`gate=apicover_enforce
step=cover_run`): the scoped coverage run `go test -tags "integration acs"
-coverprofile=… ./internal/coherence ./internal/core` exited 1 in the lane worktree. Same
leak, second spawn site — `coverageProfile` called the gate's raw runner (`g.run`) while
the tier step beside it wraps the runner in `scrubbedRun` (the CI-shell allowlist).
Reproduced in a clean worktree: `EVOLVE_FLEET=1 EVOLVE_CYCLE_STATE_FILE=/tmp/x go test
-tags "integration acs" -run TestRunCycle_InterruptCheckpointSelectsActivePhaseOnResume
./internal/core/` fails; without the two variables it passes. The step is fail-open, so
the cost was a WARN on every audit and one ~7-minute core coverage run per audit, not a
blocked ship — and a coded line that cried wolf on every cycle since 1673 (wave 1, F4).
Fixed by running the coverage step's three `go` invocations under `scrubbedRun`, red
first (`TestCoverageProfile_RunsGoTestUnderTheScrubbedEnv`: the fake runner received a
nil env, i.e. inherit).
