# Plan: a correction names a verifier that the phase cell can run

- **Status:** proposed, 2026-11-21. Revised after review round 2 and round 3.
- **Item:** `phase-cell-has-no-evolve-on-path`, rank 35, console-owned.
- **Related:** [ADR-0154](../architecture/adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md), [llm-isolation](../architecture/llm-isolation.md), [internal-bridge](../architecture/packages/internal-bridge.md), [internal-phasecontract](../architecture/packages/internal-phasecontract.md), [internal-deliverable](../architecture/packages/internal-deliverable.md), [internal-ipcenv](../architecture/packages/internal-ipcenv.md).

## Table of contents

1. [Request](#1-request)
2. [Facts](#2-facts)
3. [Root cause](#3-root-cause)
4. [Candidate designs](#4-candidate-designs)
5. [Decisions](#5-decisions)
6. [Files](#6-files)
7. [Red tests](#7-red-tests)
8. [Rollout](#8-rollout)
9. [Risks](#9-risks)
10. [Open questions](#10-open-questions)

## 1. Request

The item says:

> "In cycle 2161 the triage cell got command not found: evolve, exit 127, for the commands in the correction text. The agent then stated that no effect was owed, with no verifier output."

The acceptance line is: "A test proves that each command a correction names runs inside a cell environment".

A worked example. The contract block of a phase ends with `<self-check>evolve phase verify triage --workspace <your workspace dir></self-check>`. The agent runs the line in its cell. The shell finds no `evolve` and returns 127. The agent has no verifier output, so it reports that no effect is owed.

The failure is systematic. In `runtime/.evolve/runs`, 368 of 368 Claude final-pane captures that hold a `Bash(evolve ...)` call also hold `command not found: evolve` (counted 2026-11-21). Cycle 2579 shows it in `cycle-2579/build-evolve-bridge-r01M7XKGM-c2579-build-pid87664-n6-1795216376-final-pane.txt:143`. There the agent falls back to `../../../go/bin/evolve`.

The gate has no signal that the agent skipped the check. This plan does not add that signal (section 10, N1).

## 2. Facts

| Id | Fact | Source |
|---|---|---|
| F1 | The contract block hard-codes the bare word `evolve` in the self-check line. | `go/internal/phasecontract/render.go:118-120` |
| F2 | The host-fault hint hard-codes `evolve signals tail`. The text is addressed to the console, not to the agent. | `go/internal/deliverable/route_effect.go:101-103` |
| F3 | `PATH` is on the cell environment allowlist. The cell receives the `PATH` of the host process. | `go/internal/bridge/hermetic_env.go:34`, `go/internal/bridge/hermetic_env.go:70-77` |
| F4 | `hermeticEnv` copies the parent value of each allowed name. It adds no directory to `PATH`. | `go/internal/bridge/hermetic_env.go:80-103` |
| F5 | A seat cell always uses the `enforce` env stage. Any other cell uses the stage of `deps.envStage()`. | `go/internal/bridge/hermetic_env.go:161-165` |
| F6 | A cycle worktree gets a symlink `go/bin/evolve` to the running binary. `linkGuardDeps` reads it with `os.Executable`. The symlink is not on `PATH`, and it exists only in a worktree. | `go/internal/core/worktree.go:182-187` |
| F7 | `EVOLVE_GO_BIN` is an entry of `retiredControls`. Its replacement text is "go/bin/evolve or evolve on PATH". No Go code reads the variable. | `go/internal/ipcenv/markers.go:50` |
| F8 | Other code resolves the binary in three ways: `go/bin/evolve`, then `exec.LookPath("evolve")`. Each site has its own copy of the rule. | `go/internal/consensusdispatch/resolve.go:25-35`, `go/internal/rollback/rollback.go:370-374`, `go/internal/releasepipeline/releasepipeline.go:405-411` |
| F9 | `effects.go` has no `evolve` call. The effect checks run in the host process. | `go/internal/deliverable/effects.go:20-50` |
| F10 | The command `evolve phase verify` runs the same checks as the host gate. The two cannot drift. | `go/internal/deliverable/deliverable.go:1-3` |
| F11 | The pane of the hermetic path starts as `exec /usr/bin/env -i TERM=... /bin/sh <cell-env-file>`. The script of the file sets each variable of the cell env. The tmux server supplies no variable. | `go/internal/bridge/tmux.go:279-288`, `go/internal/bridge/tmux.go:306-313` |
| F12 | Below `enforce`, `cellEnv` returns `isolatedDriverEnv`. It copies `os.Environ()` and adds no `PATH` rule. `channel.ResolveStage("")` returns `shadow`, so a project with no `isolation.i2` policy runs at `shadow`. | `go/internal/bridge/hermetic_env.go:168-175`, `go/internal/bridge/driver_common.go:26-42`, `go/internal/bridge/channel/enablement.go:5-8` |
| F13 | On the non-hermetic tmux path, `paneEnv` returns nil. `primePaneShell` exports the realization env, the markers and the credentials. It exports no `PATH`. | `go/internal/bridge/hermetic_env.go:289-296`, `go/internal/bridge/driver_tmux_boot.go:130-149` |
| F14 | The running lane `runtime/go/bin/evolve loop --relaunch` (pid 26667) was started by path. Its `PATH` (read with `ps eww`, 2026-11-21) holds no `go/bin` directory of the project. `hermeticEnv` passes it through unchanged. | `ps eww -p 26667`, F3, F4 |
| F15 | Claude Code sources a shell snapshot for its Bash tool. The snapshot holds a line `export PATH=...` built from the rc files. | `~/.claude/shell-snapshots/snapshot-zsh-1793979632876-j56l5g.sh:74` |
| F16 | The sandbox of a Linux cell binds only `/usr`, `/bin`, `/lib`, `/lib64`, `/etc`, `HOME`, `RepoRoot` and the write paths. A binary outside these is invisible in the cell. | `go/internal/adapters/sandbox/sandbox.go:322-350` |
| F17 | Agent-facing correction text names `evolve selfcheck build` in two more sites. | `go/internal/core/builder_selfcheck.go:117`, `go/internal/core/phase_bindings_graduation.go:36` |
| F18 | In cycle 2161 the correction named `evolve inbox-mover claim %q %d`. That verb changes state. Commit `0cbfd3f5f` (2026-11-05) replaced it with the host claim and a console hint. | `git show c529f7cdb:go/internal/deliverable/effects.go:64`, `git show -s 0cbfd3f5f` |
| F19 | The guard manifest protects `hermetic_env.go` and `read_isolation.go` as `SandboxGuarantee`. It names no other file for the cell `PATH`. | `go/internal/guards/integrity_surface.go:203`, `go/internal/guards/integrity_surface.go:207` |
| F20 | The `Bash(evolve phase verify:*)` tool allowlist is in 93 agent profiles. The sandbox cannot deny the `evolve` binary, and a cell can already run it by path. | `docs/architecture/llm-isolation.md:1928`, `docs/architecture/llm-isolation.md:492` |
| F21 | The binary already enforces a verb-class table: `agentclass.Of(args)` gives class A (allowed), R (request) or D (denied). `dispatchGate.refuse` applies it. On this host `~/.evolve/dispatch/stage` reads `shadow`, so the gate only logs `ISOLATION_I3_VIOLATION` and refuses nothing. | `go/internal/agentclass/agentclass.go:44`, `go/cmd/evolve/dispatch_gate.go:34-48`, `~/.evolve/dispatch/stage` |
| F22 | Agent-facing text names more commands than the self-check line. `agentclass.Brief` tells each phase prompt to run `evolve agent capabilities --json`. The build floor can tell the builder to run `evolve test audit --rebaseline <pkg>`. That verb is not in the class table, so its class is D. | `go/internal/agentclass/surface.go:65`, `go/internal/core/build_testaudit_check.go:62`, `go/internal/core/builder_selfcheck.go:116-117` |
| F23 | `selfsha.Running` already resolves the running executable with `os.Executable`. `defaultBootRefreshExecTarget` repeats the rule with `EvalSymlinks`. `go/internal/selfsha` is on the apicover list and has a package note. | `go/internal/selfsha/selfsha.go:26-31`, `go/cmd/evolve/cmd_loop_boot_refresh.go:65-69`, `go/.apicover-enforce:77` |
| F24 | `symlinkForce` removes the link and then creates it. Two concurrent launches can race, and an agent of another cell can see no link between the two calls. | `go/internal/core/worktree.go:198-203` |
| F25 | `dispatchcred.Default` takes the home from the passwd entry, not from `$HOME`. Bridge tests that reach `cellEnv` can write otherwise into the real home. `builtBinaries` exists only in package `main` (`go/cmd/evolve/builtbinaries_test.go:5`). Package `bridge` uses `gotooltest` only for `gotooltest.Env`, in a file with the tags `integration && darwin && !notmux`. `tmuxtest_main_test.go` is in package `bridge_test`. Package `bridge` has `RunTestMainCleanups` (`go/internal/bridge/testmain_cleanup_test.go:16`). | `go/internal/dispatchcred/dispatchcred.go:40-52`, `go/internal/bridge/tmuxtest_main_test.go`, `go/internal/bridge/pane_parity_integration_test.go:1`, `go/internal/bridge/pane_parity_integration_test.go:111` |
| F26 | The rc files of the user prepend to `$PATH`, so the inherited entries survive. The risk is an earlier `evolve` that shadows the entry, not a removed entry. | `~/.zshenv:5`, `~/.zprofile:16-94`, `~/.claude/shell-snapshots/snapshot-zsh-1793979632876-j56l5g.sh:74` |

*Implication of F3, F4 and F14:* the allowlist is not the defect. The value of `PATH` is the defect. The host decides it, and the host `PATH` has no `evolve` directory.

*Implication of F11:* on the hermetic path no tmux-server `PATH` reaches the cell.

*Implication of F12 and F13:* a `PATH` rule in `hermeticEnv` alone reaches the `enforce` stage only. Every other cell keeps the 127.

## 3. Root cause

The root cause is proven. The host process has no `evolve` directory on its `PATH` (F14). The cell `PATH` is a verbatim copy of the host `PATH` (F3, F4, F11). So the cell cannot find `evolve`. The 368 of 368 pane captures confirm that the failure is systematic and not intermittent (section 1).

The tmux-server hypothesis of the first draft does not apply to the hermetic path. The pane starts under `env -i`, and the cell env file sets each variable (F11). The tmux server `PATH` matters only on the non-hermetic path (F13). That path exports no `PATH` at all, so the server value stays.

One layer remains that no host test can prove. The CLI shell layer sets `PATH` again (F15). The rc files prepend to the inherited value, so the entry survives (F26). The remaining risk is an earlier `evolve` in a user directory that shadows the entry. The live proof of section 8 checks that `command -v evolve` resolves to the `cell-bin` entry.

The plan handles the unknown in two steps:

1. The host fix puts a bin directory first on the cell `PATH` at every env stage (D1, D3).
2. The rollout proves the result with one live check and one pane count (D11). If the count does not reach zero, the fallback is an absolute path in the self-check line only.

## 4. Candidate designs

Criteria:

- **C1** the acceptance: each named command runs in the cell.
- **C2** no feature flag.
- **C3** no control value from the environment.
- **C4** one home for each rule.
- **C5** protected surface cost.
- **C6** CI parity on Linux and Go 1.23.
- **C7** capability change of a cell.
- **C8** the rule survives the CLI shell layer (F15, F26).

### Approach A: the cell `PATH` gets a host-owned directory with one `evolve` symlink

`cellEnv` resolves the real path of the running binary. It makes the host-owned directory `<home>/.evolve/cell-bin/<digest of the real path>/`, which holds one symlink `evolve`. It puts this directory first on the cell `PATH` at each env stage. The directory is outside each workspace that an agent can write.

- **For:** one rule fixes every command in every correction and in the own calls of the agent. It changes no prompt text, so the prompt cache stays valid. A single-entry directory cannot shadow a system tool.
- **For:** the cell gains no capability. A cell can already run the binary by path (F20, cycle 2579).
- **Against:** the cell env is a protected surface (F19). The rule must reach three code paths (F12, F13). An earlier `evolve` on the user `PATH` can still shadow the entry (F26, C8).

### Approach B: each correction names an absolute path

`selfCheckCommand` takes the binary path as a parameter and prints `/abs/path/evolve phase verify ...`.

- **For:** no change to the cell env. It survives the snapshot (C8).
- **Against:** each new correction site must repeat the rule (C4). The path enters the prompt, so a cached prompt changes with each install path. The agent can still call bare `evolve` in its own commands. The 93 profiles that allow `Bash(evolve phase verify:*)` match a bare command word, so an absolute-path call can leave the tool allowlist (F20).

### Approach C: the host runs the verifier and hands the result to the cell

At the end of each phase turn, the host runs `deliverable.Verify` and writes `verify-report.md` into the workspace. The correction says "read this file".

- **For:** the cell needs no binary.
- **Against:** the agent loses the early self-check, which is the reason `evolve phase verify` exists (F10). The host must know when the agent is "done". It is the largest change, with the most protected-surface cost.

### Approach D: add an `evolve` shim file in the workspace

The host writes a script `<workspace>/bin/evolve` that calls the host binary.

- **For:** no change to `PATH` of the host.
- **Against:** a second copy of the binary rule (F8). A script in a writable workspace is a new executable that the fence and the audit binding must trace. It does not help bare `evolve` calls.

### Score

| Criterion | A: PATH dir | B: absolute path | C: host runs it | D: shim file |
|---|---|---|---|---|
| C1 acceptance | yes, all commands | yes, only the named commands | yes, but the command is gone | yes, only the named commands |
| C2 no flag | yes | yes | yes | yes |
| C3 no env control value | yes | yes | yes | yes |
| C4 one home | yes, one rule in the cell env | no, one per site | yes | no, two resolvers |
| C5 protected surface | edits in the protected cell env, with full review (D4) | none | loop and gate edit | new workspace executable |
| C6 CI parity | yes, with a seam | yes | yes | needs a script on Windows-like hosts |
| C7 capability | none new (F20) | none | none | one new executable |
| C8 survives the snapshot | not proven, checked by a live proof in rollout | yes | yes | no |

## 5. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | Approach A. `cellEnv` applies `prependPath` once, on the env that each stage returns. The `hermeticEnv` and `isolatedDriverEnv` signatures stay unchanged. `primePaneShell` gets the directory from a helper in `hermetic_env.go` and sends `export PATH=<dir>:"$PATH"`. The helper caches the directory on `cfg`, as `cfg.credentials` is cached. | F12, F13: a rule in `hermeticEnv` alone leaves the 127 at `shadow` and `off`. `isolatedDriverEnv` also serves `driverEnv`, which host git runs use (`completion.go:125`, `completion_worktree.go:58`). A change there puts the directory on host git. | B: one site per correction (C4), a changed prompt, and a break of the tool allowlist (F20). C: removes the early self-check (F10). D: a second resolver and a new executable. Scope to `enforce` only: rejected, because a project with no `isolation.i2` policy runs at `shadow` (F12). A parameter on `isolatedDriverEnv`: rejected, it reaches host git. |
| D2 | The directory is host-owned and holds only an `evolve` symlink to the real path of the running binary. The root is `<home>/.evolve/cell-bin`. The home comes from one function that `dispatchcred.Default` also uses (the passwd entry, F25). A zero `Deps.CellBinRoot` falls back to a package-level default resolver, as `dispatchcred.root` does (`go/internal/dispatchcred/dispatchcred.go:31`). A hook in a package-`bridge` `_test.go` file, in the pattern of `RunTestMainCleanups`, points that resolver at a temporary directory. Each test builds its own `Deps{...}` literal, so only the resolver default covers all of them. | The real directory can hold other executables and shadow system tools. A root under the real home makes each `go test` run add a directory (F25). | Prepend the real directory: shadows system tools when the directory is shared. `$HOME` as the home source: differs from the credential store (F25). |
| D3 | The write is idempotent and atomic. If the digest directory holds exactly one entry, `evolve`, linked to the target, do nothing. Otherwise stage a new directory with that one symlink in the `cell-bin` root (mode `0700`). Move the old directory aside, `os.Rename` the stage to `<digest>`, and remove the old one. A launch that finds the directory exact keeps it and removes its own stage. A foreign entry, such as a planted `git`, does not survive the rebuild, and the launch fails only if the rebuild fails. | F24: remove-then-create races between lanes (4 to 12) and leaves a window with exit 127. A temporary entry in the digest directory makes a concurrent launch see two entries. A killed lane (lanes are deadline-killed) or a file such as `.DS_Store` blocks every later launch until someone deletes the entry. | Reuse `symlinkForce`: racy (F24). Lock file: a second mechanism, and `Rename` is already atomic. |
| D4 | `selfsha.Path() (string, error)` returns `filepath.EvalSymlinks(os.Executable())`. `selfsha.Running`, `defaultBootRefreshExecTarget`, `linkGuardDeps` and `cellEnv` call it. `linkGuardDeps` (`go/internal/core/worktree.go:183`) changes from `os.Executable` to `EvalSymlinks`. On this host `runtime/go/bin/evolve` is a regular file, so nothing changes. The seam is one `Deps.ExecutablePath` field, default `selfsha.Path`. The F8 resolvers keep their own rule, because they use `LookPath`. | F23: one home for the rule, already enrolled in apicover and documented. A new package needs a package note, a docgo package doc and an apicover row, or the graduation correction fires (`phase_bindings_graduation.go:36`). If the binary became a symlink, the worktree link pins the resolved target. | New package `selfbin`: a second home for the same rule. |
| D5 | The PATH logic stays in protected files (F19, section 6). The review class is protected: full seats, plus the adversarial and the security reviewer. | F19: a file outside the manifest lets a later lane widen the cell `PATH` with no protected-path review. | A new unprotected `cell_bin_path.go`: rejected for this reason. If the size ratchet forces a split, the new file goes into `integrity_surface.go` as `SandboxGuarantee` in the same change. |
| D6 | `prependPath` replaces the single effective `PATH` entry and removes each duplicate. The last copy wins, because `isolatedDriverEnv` appends `default_env` after `os.Environ()`. An unset or empty parent `PATH` gives the directory plus the fallback `/usr/bin:/bin`. | `PATH` can appear twice in the env (F12). | Keep both copies: which one the shell uses depends on the order. |
| D7 | Tests and the acceptance are one chain, split by package. T3, in package `core`, shows that each named command is a class A verb. T1 shows that the `cell-bin` entry is first on `PATH` at every stage. T2, in package `bridge`, runs the reachable commands in a real cell env and shows that they resolve to that entry. Together they meet the acceptance line. The text of `testAuditLine` changes: it tells the agent to delete the baseline row, and no longer names `evolve test audit --rebaseline`. | F22: the acceptance says "each command a correction names". The class table is the verb control, so a hard-coded verb set is a second copy (C4). | A registry or a fixed verb set: a second copy of the table (F21). Add `test audit` to the class table: it changes state, so the class is D and a cell cannot run it. |
| D8 | The cell-bin root is not checked for readability at runtime. The Linux bind list in `BwrapPrefix` stays the only home of what a Linux cell sees. Linux is a CI host and macOS is the only production host (`adapters/sandbox/read_isolation.go` denies reads only of the runs directory, secrets and raw outputs). The check moves to section 10, N2. | The first draft added a second copy of the bind list in `bridge` and a new fail-closed refusal at `enforce` with no shadow evidence. The review round 2 named this as scope that the acceptance does not need. | A runtime check in `read_isolation.go`: second copy of the bind list. |
| D9 | `cellEnv` returns `([]string, bool, error)`. A failure of the resolver or of the directory write stops the launch. The launch error is the only new refusal. It needs no new signal code: the existing launch-failure path reports it. | A silent empty `PATH` entry repeats the 127. | Warn and continue: hides the failure. A new `ISOLATION_CELL_BIN_*` signal at each launch: noise, and `signalcenter` has no ERROR severity (`event.go:21-23`). |
| D10 | Agent-facing correction text never names a state-changing verb. T3 enforces this through class A of `agentclass.Of`. `phaseidentity.Authority` names no class D verb: "leave temp to the pipeline". | F18: cycle 2161 named `evolve inbox-mover claim`, which changes state. F22: `evolve test audit --rebaseline` is class D today. `Authority` (`go/internal/bridge/phaseidentity/identity.go:65`) goes into each tmux phase prompt and names `evolve gc`, class D (`go/internal/agentclass/table.go:87`). Exit 127 blocks it by accident today. The I3 gate is at `shadow`. | None. The rule costs nothing. |
| D11 | Rollout proof: one live check and one pane count. Before merge, a tested change of `evolve doctor live` makes the live check possible. The probe prompt writes the output of `command -v evolve` to the artifact. The verb checks that it equals the `cell-bin` entry, so this is also a standing check. After merge, count the final panes with `⏺ Bash(evolve` and `command not found: evolve`. Target: zero over 10 cycles, else the fallback of section 3. | F26: a nonzero exit code other than 127 does not prove that the right binary ran, because an earlier `evolve` can shadow the entry. Today `doctor_live.go:191-200` accepts only `--model` and `--json`. `LiveSmokeTest` uses a fixed prompt (`go/internal/bridge/livesmoke.go:44`). | Absolute path now: changes the cached prompt and breaks the allowlist match before the data shows a need (F20). |

Consequence of D1: a cell can run any `evolve` verb by bare name, as it can already run by path (F20). The verb control is `agentclass` with `dispatchGate` (F21). The dispatch credential stage on this host is `shadow`, so it logs `ISOLATION_I3_VIOLATION` and refuses nothing. The only enforcing control today is the sandbox write fence (`ReadOnlyRepo` and `WritePaths`). The condition that makes us change D1: a measured case where a cell calls a state-changing verb through the new `PATH`. Then the fix is the I3 enforce stage, not removal of the `PATH` entry.

## 6. Files

A file marked "protected" is on the guard manifest (`go/internal/guards/integrity_surface.go`).

| File | Change |
|---|---|
| `go/internal/selfsha/selfsha.go` | Add `Path()`. `Running` calls it. Update `docs/architecture/packages/internal-selfsha.md`. |
| `go/cmd/evolve/cmd_loop_boot_refresh.go` | `defaultBootRefreshExecTarget` calls `selfsha.Path`. |
| `go/internal/core/worktree.go` | `linkGuardDeps` calls `selfsha.Path`. |
| `go/internal/dispatchcred/dispatchcred.go` | Export the passwd-home function. `Default` and the cell-bin root use it. |
| `go/internal/bridge/hermetic_env.go` (protected, `:207`) | `Deps` gets `ExecutablePath` and `CellBinRoot`. A helper makes the directory (D3), caches it on `cfg` and returns it. `cellEnv` calls `prependPath` once and returns the error (D9). `prependPath` follows D6. |
| `go/internal/bridge/driver_tmux_boot.go` (protected, `:211`) | `primePaneShell` sends `export PATH=` from the helper. `paneEnv` callers pass the error up: `driver_tmux_boot.go:43`, `recipe_adapter.go:52`, `driver_claudecloud.go:151`. |
| `go/internal/bridge/dispatch_reap.go` (protected, `:225`) | `dispatchEnv` (`:54`, call at `:55`) passes the error up. Its callers: `driver_codex.go:82`, `driver_claudep.go:92` (protected, `:220`) and `driver_agy.go:56`. |
| `go/internal/bridge/process_env.go` | `probeEnv` (`:3-5`) passes the `cellEnv` error up (D9). |
| `go/internal/bridge/livesmoke.go`, `go/internal/cli/opscmd/doctor_live.go` | The probe prompt runs `command -v evolve`, and `doctor live` checks the answer against the `cell-bin` entry (D11). |
| `go/internal/bridge/launch_llm.go` (protected, `:210`) | `LaunchLLM` (`:49`) and `ModelQueryRunner` reach `cellEnv` through `probeEnv` (`process_env.go:4`). They pass the error up. |
| `go/internal/bridge/driver_common.go` | No change. |
| `go/internal/core/build_testaudit_check.go` | `testAuditLine` text changes (D7). |
| `go/internal/bridge/phaseidentity/identity.go` | `Authority` names no class D verb (D10). |
| `go/internal/ipcenv/markers.go` | The replacement text of `EVOLVE_GO_BIN` (`:50`) becomes: "the cell `PATH` holds `evolve` first, see llm-isolation §6.1". |
| Existing env tests that assert `PATH` | Update them, for example `TestProcessEnv_IsTheAllowlistPlusTheManifest`. The `_test.go` hook of package `bridge` sets the default `CellBinRoot` resolver to a temporary directory (D2). |
| `go/internal/bridge/hermetic_env_path_test.go` (new) | The tests T1, T4, T5 and T6 of section 7. |
| `go/internal/bridge/cell_correction_cuj_test.go` (new) | The test T2 of section 7. |
| `go/internal/core/agent_correction_class_test.go` (new) | The test T3 of section 7. |
| `docs/architecture/llm-isolation.md` | §6 Solution table (line 347): the `PATH` value is the allowlist entry with the `cell-bin` directory first. §6.1 As built: the same rule and the three env stages. |
| `docs/architecture/packages/internal-bridge.md` | One paragraph: the cell `PATH` rule, with a link to this plan. |
| gc check registry | Dependency: no registry exists on `origin/main` (`go/internal/gc` has none). The item `cycle_recycle_gc_check` owns it. Add the `cell-bin` root after that item merges. File an inbox item for this row. |
| `docs/research-index/2026-11-21-01-plan-phase-cell-has-no-evolve-on-path.md` | Index entry. Then run `evolve docs index generate`. |

## 7. Red tests

Each test fails on `origin/main` and passes after the build. Land the seam `Deps.ExecutablePath` first, alone, with no behavior change. Then T2 fails on behavior (exit 127 and no verifier output), with no test-only build variant.

| Test | Package | What it proves | Acceptance |
|---|---|---|---|
| T1 `TestCellEnv_PathHasEvolveAtEachStage` | `bridge` | Table over the stages `enforce`, `shadow` and `off`, and the non-hermetic pane. Parent `PATH` is `/usr/bin:/bin`. The resulting `PATH` starts with the `cell-bin` directory. For the pane path, the `primePaneShell` keys include `export PATH=`. A table row for each caller of section 6 shows that a resolver error stops the launch (D9). | One red per stage (D1). |
| T2 `TestCUJ_EachCorrectionCommandRunsInCell` | `bridge` | See below. | The acceptance line itself. |
| T3 `TestAgentCorrections_AreClassA` | `core` (internal test) | It enumerates the agent-facing text and checks that each command is class A. It is red on `origin/main` for `testAuditLine`. See "T3 in detail". | D7, D10. |
| T4 `TestSelfshaPath_FollowsSymlink` | `selfsha` | A symlinked binary gives the real path. A resolver error gives an error, not an empty entry. | Fail loud (D9). |
| T5 `TestCellBinDir_AtomicSingleEntry` | `bridge` | N goroutines launch at the same time, and none gets an error. The directory holds exactly one entry, a symlink to the target, outside the workspace. A new target gets a new directory, and the old directory is unchanged. A leftover temporary entry or a foreign entry in the digest directory does not stop the launch, and the host rebuilds the directory. A launch that finds a leftover temporary entry gets no error. A test with a symlinked home checks that the root is outside each agent-writable path on canonical paths. | D2, D3. |
| T6 `TestPrependPath_Cases` | `bridge` | The parent entries stay after the new entry, in order. An unset parent `PATH` gives the directory plus `/usr/bin:/bin`. A duplicate `PATH` entry gives one entry. | D6. |

T3 in detail. The test enumerates `agentclass.Brief`, `RenderContractBlock`, `RenderContractTail` and `selfCheckFindingsMessage`. It also takes `testAuditLine` on the stale-row branch (`ErrStaleBaselineRow`), `steLintRemedy`, the graduation message, `task_contract.go:58`, `audit_gate_remedy.go:25` and `phaseidentity.Authority`. The core texts are unexported, so the test lives in package `core`.

The extractor reads backtick spans, the `<self-check>` element and the "Run evolve ... first" sentence of `Brief`. Each command must give `agentclass.Allowed`. A negative control flags `evolve test audit --rebaseline x`. It ignores prose such as "The evolve verbs of this cell" (`agentclass/surface.go:63-64`). The console hints `hostFaultNext` and `blocker_outcome.go:65/74` are not in the list.

T2 in detail. T2 creates a package-local `gotooltest.NewCache(gotooltest.ExecRunner)` and closes it through `RunTestMainCleanups` (`go/internal/bridge/testmain_cleanup_test.go:16`). It builds the real `evolve` binary with that cache. This is the first internal-package test of `bridge` that builds `./cmd/evolve`, so check the added build time against the CI wall ratchet before merge. The test then:

1. Sets `Deps.ExecutablePath` to the built binary, and builds the env through the real `Deps.cellEnv`, with parent `PATH=/usr/bin:/bin`.
2. Writes the env with `writeCellEnvFile`.
3. Makes a real temp workspace that holds a minimal triage deliverable.
4. Takes its commands from the exported sources: the `<self-check>` line of `RenderContractBlock` and the `agent capabilities --json` command of `Brief`.
5. Replaces each placeholder (`<your workspace dir>`, `<base>`, `<id>`) first, because the shell reads `<` as input redirection. Runs `env -i /bin/sh <file>` with each command.
6. Asserts that `command -v evolve` in the same cell shell equals the `cell-bin` entry, and that `evolve` printed output.

The tests run on Linux and macOS. They use `sh` and a built binary, so they need no feature of Go 1.24.

## 8. Rollout

The `PATH` rule goes to `enforce` in the first merge, at each env stage, with no shadow phase for the `PATH` entry. Evidence: the rule adds only a host-owned directory with one symlink, and the cell can already run the binary by path (F20). The change adds no new refusal other than a launch error (D9).

Deploy at a boundary. A reused named session that predates the merge fails the env fingerprint check once, because `PATH` changes (`hermetic_env.go:310-311`).

The I3 shadow signal rate is expected to rise. Cells call bare `evolve` more often after the fix, so `ISOLATION_I3_VIOLATION` events for class R and D verbs can increase (F21). Record the pre-merge rate. Read the later I3 shadow-to-enforce decision against that rate and not against the raw count.

The proof is D11: the live check before merge, then the pane count. The pattern of the count is `⏺ Bash(evolve`, because `Bash(evolve` also matches the launch line `--allowedTools 'Bash(evolve phase verify:*)'`:

```
grep -l '⏺ Bash(evolve' <runs>/*/*final-pane.txt | xargs grep -l 'command not found: evolve' | wc -l
```

The baseline is 368 of 368 (2026-11-21, re-counted with this pattern). The target is zero over the next 10 cycles. A nonzero count names the shell layer (F26) or a shadowing `evolve`. In that case the fallback of D11 applies.

## 9. Risks

| Risk | Control |
|---|---|
| A cell calls a state-changing verb, such as `evolve inbox`. | The class table and `dispatchGate` (F21) name the verb control, but the credential stage is `shadow` on this host. Today only the sandbox write fence enforces. A cell can already run the binary by path (F20), so the `PATH` entry adds no capability. T3 keeps the correction text class A (D10). |
| The directory shadows a system tool. | D3: the directory holds one symlink, `evolve`. T5 proves it. |
| Two launches race on the directory. | D3: atomic rename. T5 proves it with concurrent launches. |
| An `evolve` earlier on the user `PATH` shadows the entry. | D11: the live proof asserts `command -v evolve`. |
| The binary is unreadable in a Linux cell, for example under `/opt`. | Not controlled here. Linux is a CI host. Follow-up N2. |
| The host runs from a `go test` binary. | The seams `Deps.ExecutablePath` and `Deps.CellBinRoot` replace the resolver and the root in each test (D2, F25). |
| A cached prompt changes. | D1 leaves the prompt text unchanged. This is a reason to prefer A over B. |
| A later lane widens the cell `PATH`. | D5: the logic stays in protected files. |

What a test cannot prove: the `PATH` that the agent shell sees after the CLI shell layer runs. The live proof and the pane count give that evidence.

## 10. Open questions and follow-ups

- **Q1:** Does the console want a launch stop or a warning for `probeEnv`, which has no cell? Default: the probe returns the error, as the other callers do (T1).
- **Q2:** Does a later refactor move the three F8 resolvers to `selfsha`? They use `LookPath` as a fallback, so the move needs its own design. This plan leaves them as they are, and tags them for cleanup.
- **N1 (follow-up, optional):** A bridge check of the final pane that emits WARN when the pane holds `command not found: evolve`. It closes the "no signal that the agent skipped the check" gap of section 1 and replaces the manual pane count. Use `SeverityWarn` with `KindIsolationEnv`, and regenerate the catalog with `evolve signals codes generate` (do not hand-edit it).
- **N2 (follow-up, optional):** A readability check of the `cell-bin` root for Linux cells. It must derive from the `BwrapPrefix` bind list (`adapters/sandbox/sandbox.go:322-350`) in the `sandbox` package, and emit WARN in shadow until there is evidence.
